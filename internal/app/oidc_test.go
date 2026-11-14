package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"painter-next/internal/identity"
)

type oidcFixture struct {
	a                          *App
	user                       Principal
	cookie                     *http.Cookie
	csrf                       string
	client                     identity.Client
	secret, verifier, redirect string
}

func TestOIDCAndWebsiteRoutesShareServeMux(t *testing.T) {
	a := testApp(t)
	testIdentity(t, a, "owner")
	// Production registers the public website after New. Go's ServeMux forbids a
	// methodless /oidc/ subtree alongside GET / even though their paths differ.
	a.RegisterWeb()
	for _, path := range []string{"/", "/login", "/oidc/.well-known/openid-configuration", "/oidc/keys"} {
		w := testRequest(a, "GET", path, "", nil, "")
		if w.Code != 200 {
			t.Fatalf("integrated route %s: %d %s", path, w.Code, w.Body.String())
		}
	}
}

func newOIDCFixture(t *testing.T, kind string) *oidcFixture {
	t.Helper()
	a := testApp(t)
	p, cookie, csrf := testIdentity(t, a, "owner")
	if _, err := a.Pool.Exec(context.Background(), "UPDATE sessions SET authentication_method='password' WHERE management_id=$1", p.SessionID); err != nil {
		t.Fatal(err)
	}
	redirect := "https://client.example/callback"
	if kind == "native" {
		redirect = "http://127.0.0.1/callback"
	}
	raw, _ := json.Marshal(map[string]any{"name": "测试应用 <安全>", "kind": kind, "redirect_uris": []string{redirect}})
	w := testRequest(a, "POST", "/api/v1/admin/identity/clients", string(raw), cookie, csrf)
	if w.Code != 201 {
		t.Fatalf("client create: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Client identity.Client `json:"client"`
		Secret string          `json:"client_secret"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return &oidcFixture{a: a, user: p, cookie: cookie, csrf: csrf, client: out.Client, secret: out.Secret, verifier: identity.Random(), redirect: redirect}
}
func oidcForm(a *App, path string, values url.Values, cookie *http.Cookie, csrf, basicID, basicSecret string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", a.Config.PublicBaseURL)
	r.Header.Set("X-CSRF-Token", csrf)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if basicID != "" {
		r.SetBasicAuth(basicID, basicSecret)
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}
func (f *oidcFixture) authorize(t *testing.T, offline bool) string {
	t.Helper()
	sum := sha256.Sum256([]byte(f.verifier))
	scope := "openid profile"
	if offline {
		scope += " offline_access"
	}
	q := url.Values{"response_type": {"code"}, "client_id": {f.client.ID}, "redirect_uri": {f.redirect}, "scope": {scope}, "state": {"keep-client-state"}, "nonce": {"client-nonce"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}}
	start := testRequest(f.a, "GET", "/oidc/authorize?"+q.Encode(), "", nil, "")
	if start.Code != 302 {
		t.Fatalf("authorize start %d %s", start.Code, start.Body.String())
	}
	login := start.Header().Get("Location")
	u, err := url.Parse(login)
	if err != nil {
		t.Fatal(err)
	}
	id := u.Query().Get("id")
	if id == "" {
		t.Fatalf("no request id: %s", login)
	}
	anonymous := testRequest(f.a, "GET", login, "", nil, "")
	if anonymous.Code != 303 || !strings.Contains(anonymous.Header().Get("Location"), "/login?return_to=") {
		t.Fatalf("anonymous consent %d %s", anonymous.Code, anonymous.Body.String())
	}
	consent := testRequest(f.a, "GET", login, "", f.cookie, f.csrf)
	if consent.Code != 200 || !strings.Contains(consent.Body.String(), "测试应用 &lt;安全&gt;") || !strings.Contains(consent.Body.String(), f.redirect) {
		t.Fatalf("consent %d %s", consent.Code, consent.Body.String())
	}
	if consent.Header().Get("Referrer-Policy") != "same-origin" {
		t.Fatal("native consent POST must retain its same-origin Origin header")
	}
	invalid := oidcForm(f.a, "/oidc/consent", url.Values{"id": {id}, "action": {"allow"}, "csrf": {"bad"}}, f.cookie, "", "", "")
	if invalid.Code != 403 {
		t.Fatalf("consent must validate CSRF: %d", invalid.Code)
	}
	approved := oidcForm(f.a, "/oidc/consent", url.Values{"id": {id}, "action": {"allow"}, "csrf": {f.csrf}}, f.cookie, "", "", "")
	if approved.Code != 200 {
		t.Fatalf("approve %d %s", approved.Code, approved.Body.String())
	}
	if !strings.Contains(approved.Body.String(), `<script nonce="`) || !strings.Contains(approved.Body.String(), "location.replace(") || !strings.Contains(approved.Header().Get("Content-Security-Policy"), "form-action 'self'") {
		t.Fatal("CSP-safe continuation missing")
	}
	callback := testRequest(f.a, "GET", approved.Header().Get("Location"), "", f.cookie, f.csrf)
	if callback.Code != 302 {
		t.Fatalf("callback %d %s", callback.Code, callback.Body.String())
	}
	u, err = url.Parse(callback.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("state") != "keep-client-state" {
		t.Fatal("state lost")
	}
	code := u.Query().Get("code")
	if code == "" {
		t.Fatalf("no code: %s", callback.Header().Get("Location"))
	}
	return code
}
func (f *oidcFixture) token(values url.Values) *httptest.ResponseRecorder {
	values.Set("client_id", f.client.ID)
	id, secret := "", ""
	if f.secret != "" {
		id, secret = f.client.ID, f.secret
		values.Del("client_id")
	}
	return oidcForm(f.a, "/oidc/oauth/token", values, nil, "", id, secret)
}
func (f *oidcFixture) exchange(code string) *httptest.ResponseRecorder {
	return f.token(url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {f.redirect}, "code_verifier": {f.verifier}})
}
func (f *oidcFixture) userinfo(token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "/oidc/userinfo", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	f.a.Handler().ServeHTTP(w, r)
	return w
}

func TestOIDCCodePKCEUserinfoRotationAndRevocation(t *testing.T) {
	f := newOIDCFixture(t, "native")
	discovery := testRequest(f.a, "GET", "/oidc/.well-known/openid-configuration", "", nil, "")
	if discovery.Code != 200 {
		t.Fatalf("discovery %d %s", discovery.Code, discovery.Body.String())
	}
	d := jsonMap(t, discovery)
	if d["issuer"] != f.a.Config.PublicBaseURL+"/oidc" || strings.Contains(discovery.Body.String(), "implicit") || strings.Contains(discovery.Body.String(), "plain") || d["device_authorization_endpoint"] != nil {
		t.Fatalf("unsupported discovery: %s", discovery.Body.String())
	}
	code := f.authorize(t, true)
	bad := f.token(url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {f.redirect}, "code_verifier": {identity.Random()}})
	if bad.Code != 400 {
		t.Fatalf("bad PKCE %d %s", bad.Code, bad.Body.String())
	}
	w := f.exchange(code)
	if w.Code != 200 {
		t.Fatalf("token %d %s", w.Code, w.Body.String())
	}
	out := jsonMap(t, w)
	if out["expires_in"] != float64(300) && out["expires_in"] != float64(299) {
		t.Fatalf("wrong short lifetime %v", out["expires_in"])
	}
	access, refresh := out["access_token"].(string), out["refresh_token"].(string)
	if refresh == "" {
		t.Fatal("offline refresh missing")
	}
	keys := testRequest(f.a, "GET", "/oidc/keys", "", nil, "")
	if keys.Code != 200 {
		t.Fatal("JWKS unavailable")
	}
	var jwks jose.JSONWebKeySet
	if err := json.Unmarshal(keys.Body.Bytes(), &jwks); err != nil || len(jwks.Keys) != 1 {
		t.Fatalf("JWKS: %v", err)
	}
	signed, err := jose.ParseSigned(out["id_token"].(string), []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := signed.Verify(jwks.Keys[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	var claims map[string]any
	if err = json.Unmarshal(payload, &claims); err != nil {
		t.Fatal(err)
	}
	if claims["sub"] != f.user.ID || claims["iss"] != f.a.Config.PublicBaseURL+"/oidc" || claims["nonce"] != "client-nonce" {
		t.Fatalf("wrong ID claims: %s", payload)
	}
	info := f.userinfo(access)
	if info.Code != 200 {
		t.Fatalf("userinfo %d %s", info.Code, info.Body.String())
	}
	user := jsonMap(t, info)
	if user["sub"] != f.user.ID || user["email"] != nil || user["phone"] != nil || user["role"] != nil {
		t.Fatal("claims leaked or wrong subject")
	}
	replay := f.exchange(code)
	if replay.Code != 400 {
		t.Fatalf("code replay %d %s", replay.Code, replay.Body.String())
	}
	narrowed := f.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "scope": {"openid"}})
	if narrowed.Code != 200 {
		t.Fatalf("narrow refresh %d %s", narrowed.Code, narrowed.Body.String())
	}
	next := jsonMap(t, narrowed)
	if next["refresh_token"] == refresh {
		t.Fatal("refresh did not rotate")
	}
	info = f.userinfo(next["access_token"].(string))
	if info.Code != 200 || jsonMap(t, info)["name"] != nil {
		t.Fatal("scope narrowing not respected")
	}
	replay = f.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
	if replay.Code != 400 {
		t.Fatalf("refresh replay %d", replay.Code)
	}
	if w = f.userinfo(next["access_token"].(string)); w.Code == 200 {
		t.Fatal("replayed refresh family still active")
	}
	if w = f.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {next["refresh_token"].(string)}}); w.Code == 200 {
		t.Fatal("refresh replay did not revoke descendants")
	}
}

func TestOIDCCodeAndRefreshAreSingleUseUnderConcurrency(t *testing.T) {
	f := newOIDCFixture(t, "web")
	code := f.authorize(t, true)
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- f.exchange(code) }()
	}
	wg.Wait()
	close(results)
	var refresh string
	success := 0
	for w := range results {
		if w.Code == 200 {
			success++
			refresh = jsonMap(t, w)["refresh_token"].(string)
		} else if w.Code != 400 {
			t.Fatalf("concurrent code unexpected %d %s", w.Code, w.Body.String())
		}
	}
	if success != 1 {
		t.Fatalf("code was consumed %d times", success)
	}
	results = make(chan *httptest.ResponseRecorder, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- f.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
		}()
	}
	wg.Wait()
	close(results)
	success = 0
	var access string
	for w := range results {
		if w.Code == 200 {
			success++
			access = jsonMap(t, w)["access_token"].(string)
		} else if w.Code != 400 {
			t.Fatalf("concurrent refresh unexpected %d %s", w.Code, w.Body.String())
		}
	}
	if success != 1 {
		t.Fatalf("refresh was consumed %d times", success)
	}
	if f.userinfo(access).Code == 200 {
		t.Fatal("reuse must revoke the winning family too")
	}
}

func TestOIDCClientSecretSessionAndPolicyBoundaries(t *testing.T) {
	f := newOIDCFixture(t, "web")
	code := f.authorize(t, true)
	plain := oidcForm(f.a, "/oidc/oauth/token", url.Values{"grant_type": {"authorization_code"}, "client_id": {f.client.ID}, "client_secret": {f.secret}, "code": {code}, "redirect_uri": {f.redirect}, "code_verifier": {f.verifier}}, nil, "", "", "")
	if plain.Code != 401 {
		t.Fatalf("confidential client must use Basic %d", plain.Code)
	}
	w := f.exchange(code)
	if w.Code != 200 {
		t.Fatalf("token: %d %s", w.Code, w.Body.String())
	}
	out := jsonMap(t, w)
	if _, err := f.a.Pool.Exec(context.Background(), "UPDATE sessions SET revoked_at_ms=$2 WHERE management_id=$1", f.user.SessionID, NowMillis()); err != nil {
		t.Fatal(err)
	}
	if f.userinfo(out["access_token"].(string)).Code == 200 {
		t.Fatal("revoked Painter session still trusted")
	}
	if f.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {out["refresh_token"].(string)}}).Code == 200 {
		t.Fatal("revoked session can refresh")
	}
	_, cookie, csrf := testIdentity(t, f.a, "owner")
	read := testRequest(f.a, "GET", "/api/v1/admin/identity/clients", "", cookie, csrf)
	if read.Code != 200 || bytes.Contains(read.Body.Bytes(), []byte(f.secret)) || bytes.Contains(read.Body.Bytes(), []byte(identity.Hash(f.secret))) {
		t.Fatal("client credential leaked in listing")
	}
	_, reader, readerCSRF := testIdentity(t, f.a, "reader")
	if testRequest(f.a, "GET", "/api/v1/admin/identity/clients", "", reader, readerCSRF).Code == 200 {
		t.Fatal("reader can manage clients")
	}
	disabled := testRequest(f.a, "DELETE", "/api/v1/admin/identity/clients/"+f.client.ID, "", cookie, csrf)
	if disabled.Code != 204 {
		t.Fatalf("disable %d %s", disabled.Code, disabled.Body.String())
	}
	if f.exchange(code).Code == 200 {
		t.Fatal("disabled client accepted")
	}
}

func TestOIDCRejectsUnsafePKCEAndRedirects(t *testing.T) {
	f := newOIDCFixture(t, "native")
	sum := sha256.Sum256([]byte(f.verifier))
	q := url.Values{"response_type": {"code"}, "client_id": {f.client.ID}, "redirect_uri": {f.redirect}, "scope": {"openid"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}}
	for _, tc := range []struct{ field, value string }{{"code_challenge_method", "plain"}, {"code_challenge", ""}, {"redirect_uri", "http://127.0.0.2/callback"}, {"redirect_uri", "http://localhost/callback"}, {"scope", "openid email"}, {"response_type", "id_token token"}} {
		altered := url.Values{}
		for key, values := range q {
			altered[key] = append([]string(nil), values...)
		}
		altered.Set(tc.field, tc.value)
		w := testRequest(f.a, "GET", "/oidc/authorize?"+altered.Encode(), "", nil, "")
		if w.Code == 302 && strings.HasPrefix(w.Header().Get("Location"), "/oidc/consent") {
			t.Fatalf("unsafe authorize accepted %s=%s", tc.field, tc.value)
		}
	}
	q.Set("redirect_uri", "http://127.0.0.1:52143/callback")
	w := testRequest(f.a, "GET", "/oidc/authorize?"+q.Encode(), "", nil, "")
	if w.Code != 302 || !strings.HasPrefix(w.Header().Get("Location"), "/oidc/consent") {
		t.Fatalf("native dynamic loopback port rejected %d %s", w.Code, w.Body.String())
	}
}

func TestOIDCUnsupportedGrantsAndCORS(t *testing.T) {
	f := newOIDCFixture(t, "spa")
	for _, grant := range []string{"client_credentials", "password", "urn:ietf:params:oauth:grant-type:device_code", "urn:ietf:params:oauth:grant-type:token-exchange", "urn:ietf:params:oauth:grant-type:jwt-bearer"} {
		w := f.token(url.Values{"grant_type": {grant}, "username": {"owner"}, "password": {"irrelevant"}})
		if w.Code != 400 || jsonMap(t, w)["error"] != "unsupported_grant_type" {
			t.Fatalf("grant %s accepted: %d %s", grant, w.Code, w.Body.String())
		}
	}
	device := oidcForm(f.a, "/oidc/device_authorization", url.Values{"client_id": {f.client.ID}}, nil, "", "", "")
	if device.Code == 200 {
		t.Fatalf("unadvertised device endpoint enabled: %s", device.Body.String())
	}
	for _, tc := range []struct {
		origin string
		want   int
	}{{"https://client.example", 204}, {"https://evil.example", 403}} {
		r := httptest.NewRequest("OPTIONS", "/oidc/oauth/token", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Access-Control-Request-Method", "POST")
		r.Header.Set("Access-Control-Request-Headers", "Content-Type")
		w := httptest.NewRecorder()
		f.a.Handler().ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("CORS %s %d", tc.origin, w.Code)
		}
		if tc.want == 204 && w.Header().Get("Access-Control-Allow-Origin") != tc.origin {
			t.Fatal("registered SPA CORS missing")
		}
		if w.Header().Get("Access-Control-Allow-Credentials") != "" {
			t.Fatal("cross-origin cookie credentials must not be allowed")
		}
	}
	method := testRequest(f.a, "GET", "/oidc/oauth/token", "", nil, "")
	if method.Code != 405 {
		t.Fatal("token endpoint accepted GET")
	}
}

func TestOIDCSigningKeySurvivesRestartAndPolicyRevokes(t *testing.T) {
	f := newOIDCFixture(t, "native")
	code := f.authorize(t, true)
	w := f.exchange(code)
	if w.Code != 200 {
		t.Fatalf("exchange %d %s", w.Code, w.Body.String())
	}
	out := jsonMap(t, w)
	keyBefore := testRequest(f.a, "GET", "/oidc/keys", "", nil, "")
	restarted, err := New(context.Background(), f.a.Config)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	keyAfter := testRequest(restarted, "GET", "/oidc/keys", "", nil, "")
	if !bytes.Equal(keyBefore.Body.Bytes(), keyAfter.Body.Bytes()) {
		t.Fatal("provider signing identity changed after restart")
	}
	// MFA policy changes invalidate existing OAuth credentials even before they
	// expire, without disclosing or decrypting the TOTP secret on the token path.
	now := NowMillis()
	if _, err = f.a.Pool.Exec(context.Background(), "INSERT INTO user_mfa(user_id,secret_ciphertext,enabled_at_ms,updated_at_ms) VALUES($1,$2,$3,$3)", f.user.ID, []byte("test encrypted secret placeholder"), now); err != nil {
		t.Fatal(err)
	}
	if f.userinfo(out["access_token"].(string)).Code == 200 {
		t.Fatal("pre-MFA token still accepted")
	}
	if f.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {out["refresh_token"].(string)}}).Code == 200 {
		t.Fatal("pre-MFA refresh still accepted")
	}
	if _, err = f.a.Pool.Exec(context.Background(), "UPDATE sessions SET mfa_authenticated_at_ms=$2,authentication_method='password+mfa' WHERE management_id=$1", f.user.SessionID, now); err != nil {
		t.Fatal(err)
	}
	w = f.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {out["refresh_token"].(string)}})
	if w.Code == 200 {
		t.Fatal("session MFA upgrade resurrected a pre-MFA grant")
	}
	// A fresh explicit consent creates a grant with the new MFA assurance.
	w = f.exchange(f.authorize(t, true))
	if w.Code != 200 {
		t.Fatalf("new MFA grant %d %s", w.Code, w.Body.String())
	}
	verified := jsonMap(t, w)
	_, newOwner, newCSRF := testIdentity(t, f.a, "owner")
	if _, err = f.a.Pool.Exec(context.Background(), "UPDATE users SET role='reader',version=version+1,updated_at_ms=$2 WHERE id=$1", f.user.ID, NowMillis()); err != nil {
		t.Fatal(err)
	}
	if _, err = f.a.Pool.Exec(context.Background(), "UPDATE site_settings SET external_login_enabled=false,version=version+1,updated_at_ms=$1 WHERE id=1", NowMillis()); err != nil {
		t.Fatal(err)
	}
	if f.userinfo(verified["access_token"].(string)).Code == 200 {
		t.Fatal("closed external login still permits userinfo")
	}
	rotated := testRequest(f.a, "POST", "/api/v1/admin/identity/clients/"+f.client.ID+"/rotate-secret", "{}", newOwner, newCSRF)
	if rotated.Code != 400 {
		t.Fatal("native app should not receive a client secret")
	}
}

func TestOIDCConsentReauthenticationAndDeny(t *testing.T) {
	f := newOIDCFixture(t, "native")
	sum := sha256.Sum256([]byte(f.verifier))
	q := url.Values{"response_type": {"code"}, "client_id": {f.client.ID}, "redirect_uri": {f.redirect}, "scope": {"openid"}, "state": {"client-state"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}, "prompt": {"login"}}
	start := testRequest(f.a, "GET", "/oidc/authorize?"+q.Encode(), "", nil, "")
	if start.Code != 302 {
		t.Fatalf("prompt login %d %s", start.Code, start.Body.String())
	}
	location := start.Header().Get("Location")
	u, _ := url.Parse(location)
	id := u.Query().Get("id")
	page := testRequest(f.a, "GET", location, "", f.cookie, f.csrf)
	if page.Code != 200 || !strings.Contains(page.Body.String(), "请重新验证身份") {
		t.Fatal("prompt login did not request fresh authentication")
	}
	allow := oidcForm(f.a, "/oidc/consent", url.Values{"id": {id}, "action": {"allow"}, "csrf": {f.csrf}}, f.cookie, "", "", "")
	if allow.Code != 401 {
		t.Fatal("stale authentication accepted for prompt login")
	}
	denied := oidcForm(f.a, "/oidc/consent", url.Values{"id": {id}, "action": {"deny"}, "csrf": {f.csrf}}, f.cookie, "", "", "")
	if denied.Code != 200 {
		t.Fatalf("deny %d %s", denied.Code, denied.Body.String())
	}
	u, _ = url.Parse(denied.Header().Get("Location"))
	if u.Query().Get("error") != "access_denied" || u.Query().Get("state") != "client-state" {
		t.Fatal("deny response lost state")
	}
	// Duration conversion must not overflow for very large max_age.
	huge := ^uint(0)
	if oidcNeedsReauth(&identity.Request{MaxAge: &huge}, time.Now().Add(-time.Hour)) {
		t.Fatal("max_age overflow")
	}
}

func TestOIDCSingleConnectionCompletesConcurrentGrants(t *testing.T) {
	f := newOIDCFixture(t, "native")
	config := f.a.Pool.Config()
	config.MaxConns = 1
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	limited := &App{Pool: pool, Config: f.a.Config, Mux: http.NewServeMux()}
	limited.RegisterIdentity()
	if err = limited.RegisterOIDC(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.a = limited
	codes := make([]string, 8)
	for i := range codes {
		codes[i] = f.authorize(t, true)
	}
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, len(codes))
	start := time.Now()
	for _, code := range codes {
		wg.Add(1)
		go func(code string) { defer wg.Done(); results <- f.exchange(code) }(code)
	}
	wg.Wait()
	close(results)
	for w := range results {
		if w.Code != 200 {
			t.Fatalf("single-connection grant deadlocked or failed: %d %s", w.Code, w.Body.String())
		}
	}
	if time.Since(start) > 8*time.Second {
		t.Fatal("single-connection requests stalled")
	}
}

func TestOIDCAuthorizationRequestsAreRateLimited(t *testing.T) {
	f := newOIDCFixture(t, "native")
	sum := sha256.Sum256([]byte(f.verifier))
	q := url.Values{"response_type": {"code"}, "client_id": {f.client.ID}, "redirect_uri": {f.redirect}, "scope": {"openid"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}}
	for i := range 30 {
		w := testRequest(f.a, "GET", "/oidc/authorize?"+q.Encode(), "", nil, "")
		if w.Code != 302 {
			t.Fatalf("legitimate authorization %d: %d", i, w.Code)
		}
	}
	w := testRequest(f.a, "GET", "/oidc/authorize?"+q.Encode(), "", nil, "")
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("authorization write flood not bounded %d", w.Code)
	}
	var count int
	if err := f.a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM identity_auth_requests").Scan(&count); err != nil || count != 30 {
		t.Fatalf("unexpected pending request count %d: %v", count, err)
	}
}

func TestOIDCAuthorizationPendingCapacityIsBounded(t *testing.T) {
	f := newOIDCFixture(t, "native")
	now := NowMillis()
	if _, err := f.a.Pool.Exec(context.Background(), "INSERT INTO identity_auth_requests(id,client_id,request_json,created_at_ms,expires_at_ms) SELECT 'capacity-test-'||generate_series(1,1000),$1,'{}'::jsonb,$2,$3", f.client.ID, now, now+600_000); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(f.verifier))
	q := url.Values{"response_type": {"code"}, "client_id": {f.client.ID}, "redirect_uri": {f.redirect}, "scope": {"openid"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"}}
	w := testRequest(f.a, "GET", "/oidc/authorize?"+q.Encode(), "", nil, "")
	if strings.HasPrefix(w.Header().Get("Location"), "/oidc/consent") {
		t.Fatal("authorization pending hard cap bypassed")
	}
	var count int
	if err := f.a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM identity_auth_requests").Scan(&count); err != nil || count != 1000 {
		t.Fatalf("request table exceeded cap: %d, %v", count, err)
	}
}

func TestOIDCConsentTemplateEscapesMetadata(t *testing.T) {
	data := consentData{Name: `<script>alert("app")</script>`, Kind: "web", Redirect: `https://app.example/callback?value=<script>`, User: "Taota", ID: "preview-only-request", CSRF: "preview-only-csrf", Offline: true, Profile: true}
	var rendered bytes.Buffer
	if err := identityConsentTemplate.Execute(&rendered, data); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered.String(), "<script>") || !strings.Contains(rendered.String(), "&lt;script&gt;") || !strings.Contains(rendered.String(), `action="/oidc/consent"`) {
		t.Fatal("client metadata escaped incorrectly")
	}
	// Opt-in visual QA fixtures contain synthetic metadata, never a real session,
	// request identifier, CSRF token, private key, account password, or client key.
	if dir := os.Getenv("PAINTER_IDENTITY_PREVIEW_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		data.Name = "我的创作空间"
		data.Redirect = "https://workspace.example/auth/callback"
		data.Kind = "Web 服务端应用"
		for _, state := range []string{"allow", "reauth"} {
			data.Reauth = state == "reauth"
			var buf bytes.Buffer
			if err := identityConsentTemplate.Execute(&buf, data); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "identity-consent-"+state+".html"), buf.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
		}
		for _, state := range []string{"allow", "deny"} {
			target := "http://identity-preview.test/oidc/authorize/callback?id=preview-only-request"
			if state == "deny" {
				target = "http://client-preview.test/callback?error=access_denied"
			}
			var buf bytes.Buffer
			if err := identityContinueTemplate.Execute(&buf, identityContinueData{Target: target, Link: template.URL(target), Nonce: "preview-only-nonce"}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "identity-continue-"+state+".html"), buf.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
