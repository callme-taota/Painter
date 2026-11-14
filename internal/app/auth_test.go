package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestGitHubPKCEStateOnceAndStableSubject(t *testing.T) {
	a := testApp(t)
	a.Config.BootstrapGitHubID = "42"
	var exchanges atomic.Int32
	var expectedChallenge string
	var login = "octocat"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			exchanges.Add(1)
			if e := r.ParseForm(); e != nil {
				t.Error(e)
			}
			sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(sum[:]) != expectedChallenge {
				t.Error("PKCE verifier mismatch")
			}
			if r.Form.Get("redirect_uri") != a.Config.GitHubCallbackURL || r.Form.Get("client_secret") != "test-secret" {
				t.Error("wrong trusted OAuth configuration")
			}
			JSON(w, 200, map[string]any{"access_token": "provider-token-only-in-memory", "token_type": "bearer", "refresh_token": "must-not-store"})
		case "/user":
			if r.Header.Get("Authorization") != "Bearer provider-token-only-in-memory" || r.Header.Get("X-GitHub-Api-Version") != "2026-03-10" {
				t.Error("missing authenticated identity revalidation")
			}
			JSON(w, 200, map[string]any{"id": 42, "login": login, "name": "Octo", "type": "User", "email": "never-merge@example.com"})
		default:
			w.WriteHeader(404)
		}
	}))
	defer provider.Close()
	a.Config.OAuthURL = provider.URL + "/authorize"
	a.Config.TokenURL = provider.URL + "/token"
	a.Config.ProfileURL = provider.URL + "/user"
	flow := func() (*httptest.ResponseRecorder, string, *http.Cookie) {
		t.Helper()
		start := testRequest(a, "GET", "/auth/github/start?return_to=//evil.test", "", nil, "")
		if start.Code != 302 {
			t.Fatalf("start: %d %s", start.Code, start.Body.String())
		}
		u, e := url.Parse(start.Header().Get("Location"))
		if e != nil {
			t.Fatal(e)
		}
		if u.Query().Get("code_challenge_method") != "S256" {
			t.Fatal("no PKCE")
		}
		expectedChallenge = u.Query().Get("code_challenge")
		state := u.Query().Get("state")
		var binding *http.Cookie
		for _, c := range start.Result().Cookies() {
			if c.Name == bindingCookie {
				binding = c
			}
		}
		if binding == nil {
			t.Fatal("missing browser binding")
		}
		r := httptest.NewRequest("GET", "/auth/github/callback?state="+state+"&code=code", nil)
		r.AddCookie(binding)
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		return w, state, binding
	}
	w, state, binding := flow()
	if w.Code != 303 || w.Header().Get("Location") != "/studio" {
		t.Fatalf("callback %d %s", w.Code, w.Body.String())
	}
	var cookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("missing safe session cookie")
	}
	session := jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", cookie, ""))
	if session["authenticated"] != true {
		t.Fatal(session)
	}
	user := session["user"].(map[string]any)
	if user["role"] != "owner" {
		t.Fatal(user)
	}
	if _, ok := user["email"]; ok {
		t.Fatal("session leaked email")
	}
	id := user["id"]
	r := httptest.NewRequest("GET", "/auth/github/callback?state="+state+"&code=code", nil)
	r.AddCookie(binding)
	replay := httptest.NewRecorder()
	a.Handler().ServeHTTP(replay, r)
	if replay.Code != 400 || exchanges.Load() != 1 {
		t.Fatal("state replay performed another exchange", replay.Code, exchanges.Load())
	}
	login = "renamed-account"
	w, _, _ = flow()
	if w.Code != 303 {
		t.Fatalf("renamed callback %d %s", w.Code, w.Body.String())
	}
	var count int
	if e := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&count); e != nil || count != 1 {
		t.Fatal("mutable GitHub login created duplicate", count, e)
	}
	var storedID, storedLogin string
	var email *string
	if e := a.Pool.QueryRow(context.Background(), "SELECT o.user_id::text,o.provider_login,u.email FROM oauth_accounts o JOIN users u ON u.id=o.user_id WHERE subject='42'").Scan(&storedID, &storedLogin, &email); e != nil || storedID != id || storedLogin != login || email != nil {
		t.Fatal(storedID, storedLogin, email, e)
	}
	var leaked bool
	if e := a.Pool.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash=$1)", cookie.Value).Scan(&leaked); e != nil || leaked {
		t.Fatal("raw session stored", e)
	}
}
func TestOAuthWrongBrowserDoesNotConsumeAndClosedRegistration(t *testing.T) {
	a := testApp(t)
	start := testRequest(a, "GET", "/auth/github/start", "", nil, "")
	u, _ := url.Parse(start.Header().Get("Location"))
	state := u.Query().Get("state")
	r := httptest.NewRequest("GET", "/auth/github/callback?state="+state+"&code=x", nil)
	r.AddCookie(&http.Cookie{Name: bindingCookie, Value: RandID()})
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	var consumed *int64
	if e := a.Pool.QueryRow(context.Background(), "SELECT consumed_at_ms FROM oauth_states WHERE state_hash=$1", hashString(state)).Scan(&consumed); e != nil || consumed != nil {
		t.Fatal(consumed, e)
	}
	_, e := a.completeLogin(context.Background(), githubProfile{ID: json.Number("99"), Login: "closed"})
	var api *APIError
	if !asAPI(e, &api) || api.Code != "REGISTRATION_CLOSED" {
		t.Fatal(e)
	}
}
func asAPI(e error, p **APIError) bool { v, ok := e.(*APIError); *p = v; return ok }
func TestSessionCSRFRevokeAndPrivateProfile(t *testing.T) {
	a := testApp(t)
	p, cookie, csrf := testIdentity(t, a, "owner")
	session := testRequest(a, "GET", "/api/v1/session", "", cookie, "")
	if session.Code != 200 {
		t.Fatal(session.Code, session.Body.String())
	}
	if jsonMap(t, session)["csrf_token"] != csrf {
		t.Fatal("csrf not recoverable")
	}
	patch := func(body, key, ifmatch, origin, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PATCH", "/api/v1/me", strings.NewReader(body))
		r.AddCookie(cookie)
		r.Header.Set("Origin", origin)
		r.Header.Set("X-CSRF-Token", token)
		r.Header.Set("If-Match", ifmatch)
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		return w
	}
	key := RandID()
	body := `{"display_name":"Updated","email":"private@example.com","phone":"123"}`
	w := patch(body, key, `"1"`, a.Config.PublicBaseURL, "bad")
	if w.Code != 403 {
		t.Fatal("missing CSRF", w.Code)
	}
	w = patch(body, key, `"1"`, "https://evil.test", csrf)
	if w.Code != 403 {
		t.Fatal("wrong Origin", w.Code)
	}
	w = patch(body, key, "", a.Config.PublicBaseURL, csrf)
	if w.Code != 428 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = patch(body, key, `"1"`, a.Config.PublicBaseURL, csrf)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if jsonMap(t, w)["version"] != float64(2) {
		t.Fatal(w.Body.String())
	}
	replay := patch(body, key, `"1"`, a.Config.PublicBaseURL, csrf)
	if replay.Code != 200 || !bytes.Equal(w.Body.Bytes(), replay.Body.Bytes()) {
		t.Fatal("idempotency replay changed", replay.Code, replay.Body.String())
	}
	w = patch(`{"display_name":"Other"}`, key, `"1"`, a.Config.PublicBaseURL, csrf)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "IDEMPOTENCY_KEY_REUSED") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = patch(body, RandID(), `"1"`, a.Config.PublicBaseURL, csrf)
	if w.Code != 409 {
		t.Fatal("stale CAS accepted", w.Code)
	}
	public := testRequest(a, "GET", "/api/v1/users/"+p.ID, "", nil, "")
	if public.Code != 200 || strings.Contains(public.Body.String(), "private@example.com") || strings.Contains(public.Body.String(), `"phone"`) {
		t.Fatal("public profile leaked", public.Code, public.Body.String())
	}
	w = testRequest(a, "DELETE", "/api/v1/session", "", cookie, csrf)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = testRequest(a, "GET", "/api/v1/session", "", cookie, "")
	if jsonMap(t, w)["authenticated"] != false {
		t.Fatal(w.Body.String())
	}
	w = testRequest(a, "DELETE", "/api/v1/session", "", cookie, csrf)
	if w.Code != 204 {
		t.Fatal("logout not repeatable", w.Code)
	}
}
func TestWriteFreshPolicyAndLastOwner(t *testing.T) {
	a := testApp(t)
	p, cookie, csrf := testIdentity(t, a, "owner")
	a.Handle("POST /test-write", func(w http.ResponseWriter, r *http.Request) error {
		v, e := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) { return map[string]any{"actor": p.ID}, nil })
		if e != nil {
			return e
		}
		JSON(w, 200, v)
		return nil
	})
	if w := testRequest(a, "POST", "/test-write", "{}", cookie, csrf); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	tx, e := a.Pool.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	tx.Exec(context.Background(), "SELECT id FROM policy_barrier WHERE id=1 FOR UPDATE")
	_, e = tx.Exec(context.Background(), "UPDATE users SET role='reader' WHERE id=$1", p.ID)
	if e == nil {
		t.Fatal("last active owner demoted")
	}
	tx.Rollback(context.Background())
	other, _, _ := testIdentity(t, a, "owner")
	_ = other
	tx, e = a.Pool.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	tx.Exec(context.Background(), "SELECT id FROM policy_barrier WHERE id=1 FOR UPDATE")
	if _, e = tx.Exec(context.Background(), "UPDATE users SET role='reader' WHERE id=$1", p.ID); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	w := testRequest(a, "POST", "/test-write", "{}", cookie, csrf)
	if w.Code != 403 {
		t.Fatal("stale role authorized", w.Code, w.Body.String())
	}
	_, e = a.Pool.Exec(context.Background(), "UPDATE users SET disabled_at_ms=$2 WHERE id=$1", p.ID, NowMillis())
	if e != nil {
		t.Fatal(e)
	}
	w = testRequest(a, "POST", "/test-write", "{}", cookie, csrf)
	if w.Code != 401 {
		t.Fatal("disabled actor authorized", w.Code, w.Body.String())
	}
}

func TestConcurrentProfileCASAndIdempotentWrite(t *testing.T) {
	a := testApp(t)
	_, cookie, csrf := testIdentity(t, a, "owner")
	start := make(chan struct{})
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			r := httptest.NewRequest("PATCH", "/api/v1/me", strings.NewReader(`{"display_name":"Concurrent"}`))
			r.AddCookie(cookie)
			r.Header.Set("Origin", a.Config.PublicBaseURL)
			r.Header.Set("X-CSRF-Token", csrf)
			r.Header.Set("If-Match", `"1"`)
			r.Header.Set("Idempotency-Key", RandID())
			w := httptest.NewRecorder()
			a.Handler().ServeHTTP(w, r)
			codes <- w.Code
		}()
	}
	close(start)
	one, two := <-codes, <-codes
	if !((one == 200 && two == 409) || (one == 409 && two == 200)) {
		t.Fatalf("CAS outcomes %d %d", one, two)
	}
	var invocations atomic.Int32
	a.Handle("POST /test-once", func(w http.ResponseWriter, r *http.Request) error {
		v, e := a.Write(r, nil, false, true, func(tx pgx.Tx, p Principal) (any, error) {
			invocations.Add(1)
			_, e := tx.Exec(r.Context(), "SELECT pg_sleep(0.03)")
			return map[string]any{"ok": true}, e
		})
		if e != nil {
			return e
		}
		JSON(w, 200, v)
		return nil
	})
	start = make(chan struct{})
	key := RandID()
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			r := httptest.NewRequest("POST", "/test-once", strings.NewReader(`{}`))
			r.AddCookie(cookie)
			r.Header.Set("Origin", a.Config.PublicBaseURL)
			r.Header.Set("X-CSRF-Token", csrf)
			r.Header.Set("Idempotency-Key", key)
			w := httptest.NewRecorder()
			a.Handler().ServeHTTP(w, r)
			codes <- w.Code
		}()
	}
	close(start)
	one, two = <-codes, <-codes
	if one != 200 || two != 200 || invocations.Load() != 1 {
		t.Fatalf("idempotent outcomes %d %d, runs %d", one, two, invocations.Load())
	}
}
func TestSessionManagementOwnershipAndRevocation(t *testing.T) {
	a := testApp(t)
	p, cookie, csrf := testIdentity(t, a, "owner")
	other, _, _ := testIdentity(t, a, "reader")
	w := testRequest(a, "GET", "/api/v1/me/sessions", "", cookie, "")
	if w.Code != 200 || strings.Contains(w.Body.String(), cookie.Value) || strings.Contains(w.Body.String(), hashString(cookie.Value)) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = testRequest(a, "DELETE", "/api/v1/me/sessions/"+other.SessionID, "", cookie, csrf)
	if w.Code != 404 {
		t.Fatal("revoked someone else's session", w.Code, w.Body.String())
	}
	w = testRequest(a, "DELETE", "/api/v1/me/sessions/"+p.SessionID, "", cookie, csrf)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = testRequest(a, "GET", "/api/v1/me", "", cookie, "")
	if w.Code != 401 {
		t.Fatal("revoked cookie still works", w.Code)
	}
}

func TestBootstrapExistingImportedIdentity(t *testing.T) {
	a := testApp(t)
	a.Config.BootstrapGitHubID = "42"
	_, e := a.Pool.Exec(context.Background(), "INSERT INTO users(id,username,display_name,role,created_at_ms,updated_at_ms) VALUES(10,'existing-author','Historical Author','author',1,1); INSERT INTO oauth_accounts(provider,issuer,subject,user_id,provider_login,created_at_ms,updated_at_ms) VALUES('github','https://github.com','42',10,'existing-author',1,1)")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.completeLogin(context.Background(), githubProfile{ID: json.Number("42"), Login: "renamed"}); e != nil {
		t.Fatal(e)
	}
	var role string
	if e = a.Pool.QueryRow(context.Background(), "SELECT role FROM users WHERE id=10").Scan(&role); e != nil || role != "owner" {
		t.Fatal(role, e)
	}
	var count int
	if e = a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&count); e != nil || count != 1 {
		t.Fatal("bootstrap recreated historical user", count, e)
	}
}
