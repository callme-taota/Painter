package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
)

func setupTestOptions(t *testing.T, a *App) (*http.Cookie, string, map[string]any) {
	t.Helper()
	w := contentCall(t, a, nil, "", "GET", "/api/v1/setup/status", nil, 0, "")
	d := contentWant(t, w, 200)
	for _, c := range w.Result().Cookies() {
		if c.Name == setupNonceCookie {
			return c, d["csrf_token"].(string), d
		}
	}
	return nil, "", d
}
func setupTestBody(a *App, name string) map[string]any {
	return map[string]any{"setup_token": a.Config.SetupToken, "site_name": "书房 " + name, "tagline": "写下技术与日常", "admin_username": name, "admin_display_name": "站长 " + name, "password": "a private long passphrase for setup"}
}
func setupTestApp(t *testing.T) *App {
	a := testApp(t)
	a.Config.SetupToken = "private-test-install-token-0123456789abcdef"
	return a
}
func setupTestNoOwner(t *testing.T, a *App) {
	t.Helper()
	var n int
	if e := a.Pool.QueryRow(context.Background(), "SELECT (SELECT count(*) FROM users)+(SELECT count(*) FROM local_accounts)+(SELECT count(*) FROM installation_state WHERE completed_at_ms IS NOT NULL)").Scan(&n); e != nil || n != 0 {
		t.Fatal("failed installation left partial identities or completion", n, e)
	}
}

func TestSetupRequiresServerTokenBoundNonceAndAtomicCompletion(t *testing.T) {
	a := setupTestApp(t)
	c, csrf, status := setupTestOptions(t, a)
	if status["installed"] != false || status["available"] != true || status["postgres_ready"] != true || strings.Contains(contentCall(t, a, nil, "", "GET", "/api/v1/setup/status", nil, 0, "").Body.String(), a.Config.SetupToken) {
		t.Fatal("invalid public setup status", status)
	}
	body := setupTestBody(a, "first_admin")
	raw := httptest.NewRequest("POST", "/api/v1/setup/complete", strings.NewReader(`{}`))
	raw.Header.Set("Origin", "http://evil.test")
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, raw)
	contentWant(t, w, 403)
	contentWant(t, contentCall(t, a, nil, csrf, "POST", "/api/v1/setup/complete", body, 0, ""), 403)
	contentWant(t, contentCall(t, a, c, "wrong", "POST", "/api/v1/setup/complete", body, 0, ""), 403)
	wrong := setupTestBody(a, "intruder")
	wrong["setup_token"] = "not the server's installation token"
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/setup/complete", wrong, 0, ""), 403)
	weak := setupTestBody(a, "first_admin")
	weak["password"] = "short"
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/setup/complete", weak, 0, ""), 400)
	extra := setupTestBody(a, "first_admin")
	extra["role"] = "reader"
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/setup/complete", extra, 0, ""), 400)
	setupTestNoOwner(t, a)
	w = contentCall(t, a, c, csrf, "POST", "/api/v1/setup/complete", body, 0, "")
	result := contentWant(t, w, 201)
	if result["completed"] != true || result["login_url"] != "/login?installed=1" {
		t.Fatal(result)
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == sessionCookie {
			t.Fatal("installation issued an authentication session")
		}
	}
	var hash, role, site, username string
	var reg bool
	var owners, completed, audits int
	e := a.Pool.QueryRow(context.Background(), `SELECT l.password_hash,u.role,u.username,p.display_name,s.registration_enabled,(SELECT count(*) FROM users WHERE role='owner'),(SELECT count(*) FROM installation_state WHERE completed_by=u.id AND completed_at_ms IS NOT NULL),(SELECT count(*) FROM audit_logs WHERE action='system.bootstrap') FROM local_accounts l JOIN users u ON u.id=l.user_id CROSS JOIN site_profile p CROSS JOIN site_settings s`).Scan(&hash, &role, &username, &site, &reg, &owners, &completed, &audits)
	if e != nil || role != "owner" || username != "first_admin" || site != "书房 first_admin" || reg || owners != 1 || completed != 1 || audits != 1 {
		t.Fatal("installation was not atomic", e, role, site, reg, owners, completed, audits)
	}
	if ok, e := verifyPassword(context.Background(), body["password"].(string), hash); e != nil || !ok || hash == body["password"] {
		t.Fatal("password not securely stored", e)
	}
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/setup/complete", setupTestBody(a, "replacement_admin"), 0, ""), 409)
	_, _, status = setupTestOptions(t, a)
	if status["available"] != false || status["installed"] != true || status["csrf_token"] != "" {
		t.Fatal(status)
	}
	if _, e = a.InstallationToken(context.Background()); e == nil {
		t.Fatal("CLI exposed token after installation")
	}
	if _, e = a.Pool.Exec(context.Background(), "UPDATE installation_state SET completed_at_ms=NULL,completed_by=NULL"); e == nil {
		t.Fatal("installation reopened")
	}
	if _, e = a.Pool.Exec(context.Background(), "DELETE FROM installation_state"); e == nil {
		t.Fatal("installation sentinel deleted")
	}
}

func TestSetupConcurrentRequestsCreateOnlyOneOwnerAndOneSite(t *testing.T) {
	a := setupTestApp(t)
	c1, s1, _ := setupTestOptions(t, a)
	c2, s2, _ := setupTestOptions(t, a)
	responses := make(chan int, 2)
	var wg sync.WaitGroup
	for i, v := range []struct {
		c *http.Cookie
		s string
	}{{c1, s1}, {c2, s2}} {
		wg.Add(1)
		go func(i int, v struct {
			c *http.Cookie
			s string
		}) {
			defer wg.Done()
			name := []string{"alpha_admin", "beta_admin"}[i]
			responses <- contentCall(t, a, v.c, v.s, "POST", "/api/v1/setup/complete", setupTestBody(a, name), 0, "").Code
		}(i, v)
	}
	wg.Wait()
	close(responses)
	codes := []int{}
	for code := range responses {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	if len(codes) != 2 || codes[0] != 201 || codes[1] != 409 {
		t.Fatal("concurrent installation was not single-winner", codes)
	}
	var username, site string
	var owners, events int
	e := a.Pool.QueryRow(context.Background(), `SELECT u.username,p.display_name,(SELECT count(*) FROM users),(SELECT count(*) FROM outbox WHERE topic='user.registered.v1') FROM users u CROSS JOIN site_profile p WHERE u.role='owner'`).Scan(&username, &site, &owners, &events)
	if e != nil || site != "书房 "+username || owners != 1 || events != 1 {
		t.Fatal("mixed or duplicated bootstrap", username, site, owners, events, e)
	}
}

func TestSetupFailureRollsBackIdentityProfileAndSentinel(t *testing.T) {
	a := setupTestApp(t)
	c, csrf, _ := setupTestOptions(t, a)
	_, e := a.Pool.Exec(context.Background(), `CREATE FUNCTION fail_setup_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.reason LIKE 'web installation;%' THEN RAISE EXCEPTION 'fixture rejection'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_setup_audit BEFORE INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION fail_setup_audit()`)
	if e != nil {
		t.Fatal(e)
	}
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/setup/complete", setupTestBody(a, "rollback_admin"), 0, ""), 500)
	setupTestNoOwner(t, a)
	var site string
	if e = a.Pool.QueryRow(context.Background(), "SELECT display_name FROM site_profile WHERE id=1").Scan(&site); e != nil || site != "Painter" {
		t.Fatal("failed setup changed profile", site, e)
	}
	if _, e = a.Pool.Exec(context.Background(), "DROP TRIGGER fail_setup_audit ON audit_logs; DROP FUNCTION fail_setup_audit()"); e != nil {
		t.Fatal(e)
	}
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/setup/complete", setupTestBody(a, "rollback_admin"), 0, ""), 201)
}

func TestSetupDisabledWithoutOperatorTokenAndOfflineOwnerClosesIt(t *testing.T) {
	a := testApp(t)
	_, _, status := setupTestOptions(t, a)
	if status["installed"] != false || status["available"] != false {
		t.Fatal(status)
	}
	if _, e := a.InstallationToken(context.Background()); e == nil {
		t.Fatal("missing installation token accepted")
	}
	if _, e := a.CreateLocalUser(context.Background(), "offline_admin", "离线站长", "a private long offline passphrase", "owner"); e != nil {
		t.Fatal(e)
	}
	_, _, status = setupTestOptions(t, a)
	if status["installed"] != true {
		t.Fatal("offline owner did not close wizard", status)
	}
	var n int
	if e := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM installation_state WHERE completed_at_ms IS NOT NULL").Scan(&n); e != nil || n != 1 {
		t.Fatal("offline bootstrap was not durably sealed", n, e)
	}
}

func TestSetupWebEntryRedirectAndInstalledPage(t *testing.T) {
	a := setupTestApp(t)
	a.RegisterWeb()
	w := contentCall(t, a, nil, "", "GET", "/", nil, 0, "")
	if w.Code != 303 || w.Header().Get("Location") != "/setup" {
		t.Fatal("fresh homepage did not start wizard", w.Code, w.Header())
	}
	w = contentCall(t, a, nil, "", "GET", "/setup", nil, 0, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "data-install-wizard") || strings.Contains(w.Body.String(), "data-search-open") {
		t.Fatal("installer shell missing or contains irrelevant navigation", w.Code)
	}
	if _, e := a.CreateLocalUser(context.Background(), "existing_admin", "已有站长", "a private long existing passphrase", "owner"); e != nil {
		t.Fatal(e)
	}
	w = contentCall(t, a, nil, "", "GET", "/", nil, 0, "")
	if w.Code != 200 {
		t.Fatal("existing site was sent back to installation", w.Code)
	}
}
