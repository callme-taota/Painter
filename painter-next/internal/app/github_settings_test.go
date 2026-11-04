package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func githubSettingsSave(t *testing.T, a *App, cookie *http.Cookie, csrf string, input map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return testRequest(a, "PUT", "/api/v1/admin/auth/github", string(raw), cookie, csrf)
}

func TestGitHubSettingsOwnerCASSecretsAndEnvironmentOverride(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	path := "/api/v1/admin/auth/github"
	for _, check := range []struct {
		cookie *http.Cookie
		status int
	}{{nil, 401}, {reader, 403}, {owner, 200}} {
		got := testRequest(a, "GET", path, "", check.cookie, "")
		if got.Code != check.status {
			t.Fatal("settings read authorization", got.Code, got.Body.String())
		}
		if strings.Contains(got.Body.String(), "test-secret") || strings.Contains(got.Body.String(), "ciphertext") {
			t.Fatal("settings exposed secret")
		}
	}
	initial := jsonMap(t, testRequest(a, "GET", path, "", owner, ""))
	if initial["version"] != float64(0) || initial["source"] != "environment" || initial["enabled"] != true || initial["client_secret_configured"] != true || initial["callback_url"] != a.Config.PublicBaseURL+"/auth/github/callback" {
		t.Fatal("environment fallback", initial)
	}
	in := map[string]any{"version": 0, "enabled": false, "client_id": "test-client"}
	for _, check := range []struct {
		cookie *http.Cookie
		csrf   string
		status int
	}{{nil, "", 401}, {reader, readerCSRF, 403}, {owner, "bad", 403}} {
		got := githubSettingsSave(t, a, check.cookie, check.csrf, in)
		if got.Code != check.status {
			t.Fatal("settings write authorization", got.Code, got.Body.String())
		}
	}
	r := httptest.NewRequest("PUT", path, strings.NewReader(`{"version":0,"enabled":false,"client_id":"test-client"}`))
	r.AddCookie(owner)
	r.Header.Set("Origin", "https://evil.test")
	r.Header.Set("X-CSRF-Token", csrf)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("foreign origin accepted", w.Code)
	}
	w = githubSettingsSave(t, a, owner, csrf, in)
	if w.Code != 200 {
		t.Fatal("save disabled environment config", w.Code, w.Body.String())
	}
	saved := jsonMap(t, w)
	if saved["version"] != float64(1) || saved["enabled"] != false || saved["source"] != "database" || saved["client_secret_configured"] != true || strings.Contains(w.Body.String(), "test-secret") {
		t.Fatal(saved)
	}
	var ciphertext []byte
	if err := a.Pool.QueryRow(context.Background(), "SELECT client_secret_ciphertext FROM github_auth_settings WHERE id=1").Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	plain, err := a.decrypt(ciphertext)
	if err != nil || string(plain) != "test-secret" || bytes.Contains(ciphertext, []byte("test-secret")) {
		t.Fatal("environment secret was not preserved encrypted", err)
	}
	// An explicit disabled row remains authoritative after environment changes.
	a.Config.GitHubClientSecret = "changed-environment-secret"
	_, _, options := localOptions(t, a)
	if options["github_login_enabled"] != false {
		t.Fatal("disabled DB row fell back to environment")
	}
	if got := testRequest(a, "GET", "/auth/github/start", "", nil, ""); got.Code != 503 {
		t.Fatal("disabled provider accepted login", got.Code)
	}
	if got := githubSettingsSave(t, a, owner, csrf, in); got.Code != 409 {
		t.Fatal("stale version accepted", got.Code, got.Body.String())
	}
	in["version"], in["enabled"] = 1, true
	in["client_secret"] = nil
	if got := githubSettingsSave(t, a, owner, csrf, in); got.Code != 200 {
		t.Fatal("null did not preserve saved secret", got.Code, got.Body.String())
	}
	credentials, err := a.effectiveGitHub(context.Background(), a.Pool)
	if err != nil || credentials.Secret != "test-secret" || !credentials.Enabled {
		t.Fatal("environment unexpectedly overrode DB secret", err)
	}
	in["version"], in["client_secret"] = 2, ""
	if got := githubSettingsSave(t, a, owner, csrf, in); got.Code != 400 {
		t.Fatal("enabled without secret", got.Code)
	}
	in["client_secret"] = "replacement-provider-secret"
	if got := githubSettingsSave(t, a, owner, csrf, in); got.Code != 200 || strings.Contains(got.Body.String(), "replacement-provider-secret") {
		t.Fatal("replacement/redaction", got.Code)
	}
	var auditPayload string
	if err := a.Pool.QueryRow(context.Background(), "SELECT string_agg(reason,' ') FROM audit_logs WHERE target_id='github-auth'").Scan(&auditPayload); err != nil || strings.Contains(auditPayload, "secret") {
		t.Fatal("credential in audit reason", err)
	}
}

func TestGitHubSettingsValidationAndClear(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	for _, in := range []map[string]any{
		{"version": 0, "enabled": true, "client_id": ""},
		{"version": -1, "enabled": false},
		{"version": 0, "enabled": true, "client_id": "client", "client_secret": "bad\nsecret"},
		{"version": 0, "enabled": false, "client_id": "client", "token_url": "http://127.0.0.1/private"},
	} {
		if got := githubSettingsSave(t, a, owner, csrf, in); got.Code != 400 {
			t.Fatal("bad configuration accepted", got.Code, got.Body.String())
		}
	}
	got := githubSettingsSave(t, a, owner, csrf, map[string]any{"version": 0, "enabled": false, "client_id": "", "client_secret": ""})
	if got.Code != 200 || jsonMap(t, got)["client_secret_configured"] != false {
		t.Fatal("explicit clearing did not work", got.Code, got.Body.String())
	}
}

func TestGitHubSettingsConcurrentCAS(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	start := make(chan struct{})
	done := make(chan *httptest.ResponseRecorder, 2)
	for _, client := range []string{"first-client", "second-client"} {
		go func(client string) {
			<-start
			done <- githubSettingsSave(t, a, owner, csrf, map[string]any{"version": 0, "enabled": true, "client_id": client, "client_secret": client + "-fixture-secret"})
		}(client)
	}
	close(start)
	statuses := map[int]int{}
	for range 2 {
		statuses[(<-done).Code]++
	}
	if statuses[200] != 1 || statuses[409] != 1 {
		t.Fatal("concurrent configuration writes lost CAS", statuses)
	}
	credentials, err := a.effectiveGitHub(context.Background(), a.Pool)
	if err != nil || credentials.Version != 1 || credentials.Secret != credentials.ClientID+"-fixture-secret" {
		t.Fatal("configuration was partially replaced", err)
	}
}

func TestGitHubUnreadableSecretFailsClosedWithoutBlockingLocalLogin(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "provider-recovery-owner", "owner")
	logged := loginTestLocal(t, a, "provider-recovery-owner", "correct horse battery staple 2026", nil)
	owner, csrf := responseSession(t, logged), jsonMap(t, logged)["csrf_token"].(string)
	if got := githubSettingsSave(t, a, owner, csrf, map[string]any{"version": 0, "enabled": true, "client_id": "test-client"}); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	if _, err := a.Pool.Exec(context.Background(), "UPDATE github_auth_settings SET client_secret_ciphertext=$1 WHERE id=1", []byte("unreadable-fixture-ciphertext")); err != nil {
		t.Fatal(err)
	}
	_, _, options := localOptions(t, a)
	if options["github_login_enabled"] != false || options["local_login_enabled"] != true {
		t.Fatal("optional provider blocked local login options", options)
	}
	if got := testRequest(a, "GET", "/auth/github/start", "", nil, ""); got.Code != 503 || !strings.Contains(got.Body.String(), "OAUTH_CONFIG_UNAVAILABLE") {
		t.Fatal("unreadable provider did not fail closed", got.Code)
	}
	if got := loginTestLocal(t, a, "provider-recovery-owner", "correct horse battery staple 2026", nil); got.Code != 200 {
		t.Fatal("unreadable optional provider blocked local authentication", got.Code, got.Body.String())
	}
}

func TestGitHubOwnerBootstrapSealsInstallation(t *testing.T) {
	a := testApp(t)
	a.Config.BootstrapGitHubID = "42"
	if _, err := a.completeLogin(context.Background(), githubProfile{ID: json.Number("42"), Login: "bootstrap-owner"}); err != nil {
		t.Fatal(err)
	}
	var sealed bool
	if err := a.Pool.QueryRow(context.Background(), "SELECT s.completed_at_ms IS NOT NULL AND u.role='owner' FROM installation_state s JOIN users u ON u.id=s.completed_by WHERE s.id=1").Scan(&sealed); err != nil || !sealed {
		t.Fatal("GitHub owner creation did not seal installation", sealed, err)
	}
}

func githubFlowStart(t *testing.T, a *App) (string, *http.Cookie) {
	t.Helper()
	w := testRequest(a, "GET", "/auth/github/start?return_to=/me", "", nil, "")
	if w.Code != 302 {
		t.Fatal("OAuth start", w.Code, w.Body.String())
	}
	u, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == bindingCookie {
			return u.Query().Get("state"), cookie
		}
	}
	t.Fatal("no OAuth browser binding")
	return "", nil
}

func githubFlowCallback(a *App, state string, binding, old *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "/auth/github/callback?state="+url.QueryEscape(state)+"&code=fixture-code", nil)
	r.AddCookie(binding)
	if old != nil {
		r.AddCookie(old)
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}

func TestGitHubDatabaseCredentialsAndPresentedSessionRotation(t *testing.T) {
	a := testApp(t)
	_, old, csrf := testIdentity(t, a, "owner")
	if got := githubSettingsSave(t, a, old, csrf, map[string]any{"version": 0, "enabled": true, "client_id": "database-client", "client_secret": "database-provider-secret"}); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	if _, err := a.Pool.Exec(context.Background(), "UPDATE site_settings SET registration_enabled=true WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	var exchanges atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			exchanges.Add(1)
			_ = r.ParseForm()
			if r.Form.Get("client_id") != "database-client" || r.Form.Get("client_secret") != "database-provider-secret" || r.Form.Get("redirect_uri") != a.Config.PublicBaseURL+"/auth/github/callback" {
				t.Error("exchange ignored effective database credentials")
			}
			JSON(w, 200, map[string]any{"access_token": "fixture-only-token", "token_type": "bearer"})
			return
		}
		JSON(w, 200, map[string]any{"id": 31415, "login": "github-reader", "type": "User"})
	}))
	defer provider.Close()
	a.Config.OAuthURL, a.Config.TokenURL, a.Config.ProfileURL = provider.URL+"/authorize", provider.URL+"/token", provider.URL+"/user"
	state, binding := githubFlowStart(t, a)
	w := githubFlowCallback(a, state, binding, old)
	if w.Code != 303 || w.Header().Get("Location") != "/me" {
		t.Fatal("callback", w.Code, w.Body.String())
	}
	newCookie := responseSession(t, w)
	if newCookie.Value == old.Value || jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", old, ""))["authenticated"] != false || jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", newCookie, ""))["authenticated"] != true {
		t.Fatal("GitHub sign-in did not retire old session and create new session")
	}
	if replay := githubFlowCallback(a, state, binding, old); replay.Code != 400 || exchanges.Load() != 1 {
		t.Fatal("callback replay exchanged another token", replay.Code, exchanges.Load())
	}
}

func TestGitHubConfigurationChangeRejectsInFlightOAuth(t *testing.T) {
	for _, when := range []string{"before-callback", "during-exchange"} {
		t.Run(when, func(t *testing.T) {
			a := testApp(t)
			_, owner, csrf := testIdentity(t, a, "owner")
			var exchanges atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					exchanges.Add(1)
					if when == "during-exchange" {
						got := githubSettingsSave(t, a, owner, csrf, map[string]any{"version": 0, "enabled": false, "client_id": "test-client"})
						if got.Code != 200 {
							t.Error("concurrent configuration change", got.Code, got.Body.String())
						}
					}
					JSON(w, 200, map[string]any{"access_token": "fixture-token", "token_type": "bearer"})
					return
				}
				JSON(w, 200, map[string]any{"id": 42, "login": "race-owner", "type": "User"})
			}))
			defer provider.Close()
			a.Config.OAuthURL, a.Config.TokenURL, a.Config.ProfileURL = provider.URL+"/authorize", provider.URL+"/token", provider.URL+"/user"
			state, binding := githubFlowStart(t, a)
			if when == "before-callback" {
				if got := githubSettingsSave(t, a, owner, csrf, map[string]any{"version": 0, "enabled": true, "client_id": "replacement-client"}); got.Code != 200 {
					t.Fatal(got.Code, got.Body.String())
				}
			}
			got := githubFlowCallback(a, state, binding, owner)
			if got.Code != 400 {
				t.Fatal("changed provider created a session", got.Code, got.Body.String())
			}
			var count int
			if err := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM oauth_accounts").Scan(&count); err != nil || count != 0 {
				t.Fatal("changed provider created identity", count, err)
			}
			if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", owner, ""))["authenticated"] != true {
				t.Fatal("failed callback retired original session")
			}
			if when == "before-callback" && exchanges.Load() != 0 {
				t.Fatal("configuration change reached provider")
			}
		})
	}
}
