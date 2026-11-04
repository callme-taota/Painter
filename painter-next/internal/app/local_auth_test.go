package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func localOptions(t *testing.T, a *App) (*http.Cookie, string, map[string]any) {
	t.Helper()
	w := testRequest(a, "GET", "/api/v1/auth/options", "", nil, "")
	if w.Code != 200 {
		t.Fatalf("options %d %s", w.Code, w.Body.String())
	}
	options := jsonMap(t, w)
	for _, c := range w.Result().Cookies() {
		if c.Name == localNonceCookie {
			return c, options["csrf_token"].(string), options
		}
	}
	t.Fatal("missing anonymous browser binding")
	return nil, "", nil
}
func localRequest(t *testing.T, a *App, path string, body any, nonce *http.Cookie, csrf string, session *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", path, bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", a.Config.PublicBaseURL)
	r.Header.Set("X-CSRF-Token", csrf)
	if nonce != nil {
		r.AddCookie(nonce)
	}
	if session != nil {
		r.AddCookie(session)
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}
func responseSession(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return c
		}
	}
	t.Fatal("missing session cookie", w.Code, w.Body.String())
	return nil
}
func createTestLocal(t *testing.T, a *App, username, role string) {
	t.Helper()
	if _, err := a.CreateLocalUser(context.Background(), username, "Test "+username, "correct horse battery staple 2026", role); err != nil {
		t.Fatal(err)
	}
}
func loginTestLocal(t *testing.T, a *App, username, password string, previous *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	nonce, csrf, _ := localOptions(t, a)
	return localRequest(t, a, "/api/v1/auth/login", map[string]any{"username": username, "password": password}, nonce, csrf, previous)
}
func TestLocalRegistrationCSRFPolicyAndUniqueIdentity(t *testing.T) {
	a := testApp(t)
	nonce, csrf, options := localOptions(t, a)
	if options["registration_open"] != false || options["password_min_length"] != float64(15) || !nonce.HttpOnly || nonce.SameSite != http.SameSiteLaxMode {
		t.Fatal(options, nonce)
	}
	body := map[string]any{"username": " New_Reader ", "display_name": "新读者", "password": "long passphrase with spaces 2026"}
	if w := localRequest(t, a, "/api/v1/auth/register", body, nonce, csrf, nil); w.Code != 403 {
		t.Fatalf("closed: %d %s", w.Code, w.Body.String())
	}
	if _, err := a.Pool.Exec(context.Background(), "UPDATE site_settings SET registration_enabled=true WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if w := localRequest(t, a, "/api/v1/auth/register", body, nil, csrf, nil); w.Code != 403 {
		t.Fatal("unbound registration accepted", w.Code)
	}
	if w := localRequest(t, a, "/api/v1/auth/register", body, nonce, "bad", nil); w.Code != 403 {
		t.Fatal("bad csrf accepted", w.Code)
	}
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewReader(raw))
	r.AddCookie(nonce)
	r.Header.Set("Origin", "https://evil.example")
	r.Header.Set("X-CSRF-Token", csrf)
	rejected := httptest.NewRecorder()
	a.Handler().ServeHTTP(rejected, r)
	if rejected.Code != 403 {
		t.Fatal("cross origin registration accepted")
	}
	body["role"] = "owner"
	if w := localRequest(t, a, "/api/v1/auth/register", body, nonce, csrf, nil); w.Code != 400 {
		t.Fatal("role mass assignment", w.Code, w.Body.String())
	}
	delete(body, "role")
	w := localRequest(t, a, "/api/v1/auth/register", body, nonce, csrf, nil)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	session := jsonMap(t, w)
	user := session["user"].(map[string]any)
	if session["authenticated"] != true || user["role"] != "reader" || user["username"] != "new_reader" {
		t.Fatal(session)
	}
	cookie := responseSession(t, w)
	if !cookie.HttpOnly || cookie.Value == nonce.Value {
		t.Fatal("unsafe session", cookie)
	}
	var hash, storedToken string
	if err := a.Pool.QueryRow(context.Background(), "SELECT l.password_hash,s.token_hash FROM local_accounts l JOIN sessions s ON s.user_id=l.user_id WHERE s.management_id=$1", userSessionID(t, a, cookie)).Scan(&hash, &storedToken); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") || strings.Contains(hash, body["password"].(string)) || storedToken == cookie.Value {
		t.Fatal("credential storage unsafe")
	}
	body["username"] = "NEW_READER"
	if w = localRequest(t, a, "/api/v1/auth/register", body, nonce, csrf, nil); w.Code != 400 || !strings.Contains(w.Body.String(), "REGISTRATION_FAILED") {
		t.Fatal("case variant duplicate", w.Code, w.Body.String())
	}
	// Body validation never reflects secret values.
	bad := localRequest(t, a, "/api/v1/auth/login", map[string]any{"username": "x", "password": "do-not-log-this-secret", "unexpected": "field"}, nonce, csrf, nil)
	if strings.Contains(bad.Body.String(), "do-not-log-this-secret") {
		t.Fatal("error reflected password")
	}
}
func userSessionID(t *testing.T, a *App, c *http.Cookie) string {
	t.Helper()
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(c)
	p, err := a.Principal(r)
	if err != nil {
		t.Fatal(err)
	}
	return p.SessionID
}
func TestLocalLoginRotationUniformFailuresAndDisabled(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "owner-login", "owner")
	w := loginTestLocal(t, a, " OWNER-LOGIN ", "correct horse battery staple 2026", nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	first := responseSession(t, w)
	w = loginTestLocal(t, a, "owner-login", "correct horse battery staple 2026", first)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	second := responseSession(t, w)
	if first.Value == second.Value {
		t.Fatal("session fixation")
	}
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", first, ""))["authenticated"] != false {
		t.Fatal("old presented session survived")
	}
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", second, ""))["authenticated"] != true {
		t.Fatal("new session missing")
	}
	failures := []struct{ name, password string }{{"owner-login", "wrong password value"}, {"absent-user", "wrong password value"}, {"invalid name!", "wrong password value"}}
	for _, failure := range failures {
		got := loginTestLocal(t, a, failure.name, failure.password, nil)
		if got.Code != 401 || !strings.Contains(got.Body.String(), `"code":"INVALID_CREDENTIALS"`) {
			t.Fatal(got.Code, got.Body.String())
		}
	}
	createTestLocal(t, a, "disabled-reader", "reader")
	if _, err := a.Pool.Exec(context.Background(), "UPDATE users SET disabled_at_ms=$1 WHERE username='disabled-reader'", NowMillis()); err != nil {
		t.Fatal(err)
	}
	got := loginTestLocal(t, a, "disabled-reader", "correct horse battery staple 2026", nil)
	if got.Code != 401 || !strings.Contains(got.Body.String(), "INVALID_CREDENTIALS") {
		t.Fatal("disabled state leaked or accepted", got.Code, got.Body.String())
	}
}
func TestLocalPasswordChangeAndOfflineResetRevokeAllSessions(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "password-owner", "owner")
	one := loginTestLocal(t, a, "password-owner", "correct horse battery staple 2026", nil)
	first := responseSession(t, one)
	two := loginTestLocal(t, a, "password-owner", "correct horse battery staple 2026", nil)
	second := responseSession(t, two)
	csrf := jsonMap(t, two)["csrf_token"].(string)
	body := map[string]any{"current_password": "wrong password", "new_password": "a brand new long password 2026"}
	failed := localRequest(t, a, "/api/v1/me/password", body, nil, csrf, second)
	if failed.Code != 401 {
		t.Fatal(failed.Code, failed.Body.String())
	}
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", first, ""))["authenticated"] != true {
		t.Fatal("failed password change revoked session")
	}
	body["current_password"] = "correct horse battery staple 2026"
	changed := localRequest(t, a, "/api/v1/me/password", body, nil, csrf, second)
	if changed.Code != 204 {
		t.Fatal(changed.Code, changed.Body.String())
	}
	for _, c := range []*http.Cookie{first, second} {
		if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", c, ""))["authenticated"] != false {
			t.Fatal("password change left a live session")
		}
	}
	if got := loginTestLocal(t, a, "password-owner", "correct horse battery staple 2026", nil); got.Code != 401 {
		t.Fatal("old password accepted")
	}
	logged := loginTestLocal(t, a, "password-owner", body["new_password"].(string), nil)
	if logged.Code != 200 {
		t.Fatal(logged.Code, logged.Body.String())
	}
	cookie := responseSession(t, logged)
	report, err := a.ResetLocalPassword(context.Background(), "PASSWORD-OWNER", "offline recovery passphrase 2026")
	if err != nil || report["sessions_revoked"] != int64(1) {
		t.Fatal(report, err)
	}
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", cookie, ""))["authenticated"] != false {
		t.Fatal("offline reset left live session")
	}
	if got := loginTestLocal(t, a, "password-owner", "offline recovery passphrase 2026", nil); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	var role string
	if err := a.Pool.QueryRow(context.Background(), "SELECT role FROM users WHERE username='password-owner'").Scan(&role); err != nil || role != "owner" {
		t.Fatal("reset changed role", role, err)
	}
	// Reset cannot silently re-enable a disabled account.
	createTestLocal(t, a, "disabled-reset", "reader")
	a.Pool.Exec(context.Background(), "UPDATE users SET disabled_at_ms=$1 WHERE username='disabled-reset'", NowMillis())
	if _, err = a.ResetLocalPassword(context.Background(), "disabled-reset", "offline recovery passphrase 2026"); err == nil {
		t.Fatal("disabled reset permitted")
	}
}
func TestLocalLimitsExpireAndRequestSizeBound(t *testing.T) {
	a := testApp(t)
	nonce, csrf, _ := localOptions(t, a)
	for i := 0; i < 12; i++ {
		w := localRequest(t, a, "/api/v1/auth/login", map[string]any{"username": "same-absent-user", "password": "wrong password value"}, nonce, csrf, nil)
		if w.Code != 401 {
			t.Fatal(i, w.Code, w.Body.String())
		}
	}
	limited := localRequest(t, a, "/api/v1/auth/login", map[string]any{"username": "same-absent-user", "password": "wrong password value"}, nonce, csrf, nil)
	if limited.Code != 429 {
		t.Fatal("account limiter missing", limited.Code)
	}
	a.rateMu.Lock()
	for k, b := range a.rates {
		if strings.HasPrefix(k, "local-auth/") {
			b.Reset = NowMillis() - 1
			a.rates[k] = b
		}
	}
	a.rateMu.Unlock()
	if w := localRequest(t, a, "/api/v1/auth/login", map[string]any{"username": "same-absent-user", "password": "wrong password value"}, nonce, csrf, nil); w.Code != 401 {
		t.Fatal("temporary lock did not expire", w.Code)
	}
	large := localRequest(t, a, "/api/v1/auth/login", map[string]any{"username": "absent", "password": strings.Repeat("p", localAuthBodyLimit+1)}, nonce, csrf, nil)
	if large.Code != 413 {
		t.Fatal("oversize request accepted", large.Code, large.Body.String())
	}
	// A single direct peer cannot bypass limits by cycling account names.
	peer := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	peer.RemoteAddr = "198.51.100.2:1234"
	for i := 0; i < 60; i++ {
		if !a.allowLocalAuth(peer, RandID(), false) {
			t.Fatal("IP limit early", i)
		}
	}
	peer.Header.Set("X-Forwarded-For", "198.51.100.99")
	if a.allowLocalAuth(peer, RandID(), false) {
		t.Fatal("forwarding header bypassed direct IP limit")
	}
}
func TestLocalLoginRechecksPasswordAfterPolicyBarrier(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "racing-reader", "reader")
	nonce, csrf, _ := localOptions(t, a)
	tx, err := a.maintenance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- localRequest(t, a, "/api/v1/auth/login", map[string]any{"username": "racing-reader", "password": "correct horse battery staple 2026"}, nonce, csrf, nil)
	}()
	deadline := time.Now().Add(5 * time.Second)
	waiting := false
	for time.Now().Before(deadline) {
		err = a.Pool.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%policy_barrier%')").Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("login did not reach maintenance gate")
	}
	encoded, err := hashPassword(context.Background(), "the concurrent reset password 2026")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), "UPDATE local_accounts SET password_hash=$1 WHERE user_id=(SELECT id FROM users WHERE username='racing-reader')", encoded); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if got.Code != 401 {
			t.Fatal("stale verified password created session", got.Code, got.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("login stuck")
	}
}

// The pause trigger holds password replacement after its user/policy locks have
// been taken. A normal write must queue behind the revocation and fail after the
// password transaction commits, never use the sessions row from an old snapshot.
func TestLocalPasswordChangeRejectsQueuedOldSessionWrite(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "queued-password-reader", "reader")
	logged := loginTestLocal(t, a, "queued-password-reader", "correct horse battery staple 2026", nil)
	cookie := responseSession(t, logged)
	csrf := jsonMap(t, logged)["csrf_token"].(string)
	if _, err := a.Pool.Exec(context.Background(), `
 CREATE FUNCTION test_pause_local_password() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN PERFORM pg_advisory_xact_lock(981639425); RETURN NEW; END $$;
 CREATE TRIGGER test_pause_local_password BEFORE UPDATE ON local_accounts
 FOR EACH ROW EXECUTE FUNCTION test_pause_local_password();`); err != nil {
		t.Fatal(err)
	}
	blocker, err := a.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(context.Background())
	if _, err = blocker.Exec(context.Background(), "SELECT pg_advisory_xact_lock(981639425)"); err != nil {
		t.Fatal(err)
	}
	passwordDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		passwordDone <- localRequest(t, a, "/api/v1/me/password", map[string]any{"current_password": "correct horse battery staple 2026", "new_password": "replacement long password 2026"}, nil, csrf, cookie)
	}()
	waitForLocks := func(want int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			var count int
			if err := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock'").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count >= want {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("expected %d requests waiting for controlled locks", want)
	}
	waitForLocks(1)
	var writes atomic.Int32
	a.Handle("POST /test-queued-password-write", func(w http.ResponseWriter, r *http.Request) error {
		_, err := a.Write(r, nil, false, false, func(tx pgx.Tx, p Principal) (any, error) { writes.Add(1); return nil, nil })
		if err != nil {
			return err
		}
		w.WriteHeader(204)
		return nil
	})
	writeDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { writeDone <- testRequest(a, "POST", "/test-queued-password-write", "{}", cookie, csrf) }()
	waitForLocks(2)
	if err = blocker.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case changed := <-passwordDone:
		if changed.Code != 204 {
			t.Fatal("password change failed", changed.Code, changed.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("password change stuck")
	}
	select {
	case queued := <-writeDone:
		if queued.Code != 401 || writes.Load() != 0 {
			t.Fatal("revoked old session authorized queued write", queued.Code, writes.Load(), queued.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued write stuck")
	}
	var live int
	if err = a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM sessions WHERE revoked_at_ms IS NULL").Scan(&live); err != nil || live != 0 {
		t.Fatal("password change did not revoke all sessions", live, err)
	}
}

// Session rotation can also update a user and revoke a token under the shared
// identity gate. PostgreSQL EPQ refreshes the locked users row after a wait, but
// does not refresh an unlocked joined sessions row from the original snapshot.
func TestTxPrincipalRechecksSessionAfterWaitingForUserLock(t *testing.T) {
	a := testApp(t)
	p, cookie, csrf := testIdentity(t, a, "reader")
	blocker, err := a.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(context.Background())
	if _, err = blocker.Exec(context.Background(), "SELECT id FROM policy_barrier WHERE id=1 FOR SHARE"); err != nil {
		t.Fatal(err)
	}
	if _, err = blocker.Exec(context.Background(), "UPDATE users SET version=version+1 WHERE id=$1", p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = blocker.Exec(context.Background(), "UPDATE sessions SET revoked_at_ms=$2 WHERE user_id=$1", p.ID, NowMillis()); err != nil {
		t.Fatal(err)
	}
	var writes atomic.Int32
	a.Handle("POST /test-stale-session-write", func(w http.ResponseWriter, r *http.Request) error {
		_, err := a.Write(r, nil, false, false, func(tx pgx.Tx, p Principal) (any, error) { writes.Add(1); return nil, nil })
		if err != nil {
			return err
		}
		w.WriteHeader(204)
		return nil
	})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- testRequest(a, "POST", "/test-stale-session-write", "{}", cookie, csrf) }()
	waiting := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err = a.Pool.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%FROM sessions s JOIN users%')").Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("authorization did not wait for user row lock")
	}
	if err = blocker.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if got.Code != 401 || writes.Load() != 0 {
			t.Fatal("joined stale session authorized write after user lock", got.Code, writes.Load(), got.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("authorization stuck")
	}
}
