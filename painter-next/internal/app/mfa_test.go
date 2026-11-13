package app

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRFC6238SHA1VectorsAndReplayWindow(t *testing.T) {
	// RFC 6238 Appendix B SHA-1 vectors, using the 20-byte RFC secret.
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	vectors := []struct {
		timestamp int64
		code      string
	}{{59, "94287082"}, {1111111109, "07081804"}, {1111111111, "14050471"}, {1234567890, "89005924"}, {2000000000, "69279037"}, {20000000000, "65353130"}}
	for _, v := range vectors {
		got, err := totpAt(secret, v.timestamp/30, 8)
		if err != nil || got != v.code {
			t.Fatalf("%d got %q %v", v.timestamp, got, err)
		}
	}
	now := int64(1234567890000)
	current := now / 30000
	for _, offset := range []int64{-1, 0, 1} {
		code, _ := totpAt(secret, current+offset, 6)
		step, ok := matchTOTPStep(secret, code, now, current-2)
		if !ok || step != current+offset {
			t.Fatal("drift", offset)
		}
		if _, ok := matchTOTPStep(secret, code, now, step); ok {
			t.Fatal("accepted replay")
		}
	}
	for _, code := range []string{"12345", "1234567", " 123456", "１２３４５６", "abcdef"} {
		if _, ok := matchTOTPStep(secret, code, now, -1); ok {
			t.Fatal("invalid OTP accepted")
		}
	}
}
func TestMFAEncryptionBoundToIdentity(t *testing.T) {
	a := &App{Config: Config{EncryptionKey: bytes.Repeat([]byte{1}, 32)}}
	raw, err := a.mfaCipher("12", []byte("top secret"), false)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("top secret")) {
		t.Fatal("stored plaintext")
	}
	plain, err := a.mfaCipher("12", raw, true)
	if err != nil || string(plain) != "top secret" {
		t.Fatal(err)
	}
	if _, err = a.mfaCipher("13", raw, true); err == nil {
		t.Fatal("ciphertext swapped across users")
	}
	if a.recoveryHash("12", "abcde-fghij-klmno-pqrst") != a.recoveryHash("12", "ABCDEFGHIJKLMNOPQRST") {
		t.Fatal("recovery formatting")
	}
	if a.recoveryHash("12", "ABCDE-FGHIJ-KLMNO-PQRST") == a.recoveryHash("13", "ABCDE-FGHIJ-KLMNO-PQRST") {
		t.Fatal("recovery hashes cross identity")
	}
}
func mfaSessionRequest(t *testing.T, a *App, path string, body any, session *http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	return localRequest(t, a, path, body, nil, csrf, session)
}
func cookieNamed(t *testing.T, w *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == name && c.Value != "" {
			return c
		}
	}
	t.Fatalf("missing cookie %s: %d %s", name, w.Code, w.Body.String())
	return nil
}
func enableTestMFA(t *testing.T, a *App, username string) (*http.Cookie, string, string, []string) {
	t.Helper()
	login := loginTestLocal(t, a, username, "correct horse battery staple 2026", nil)
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	cookie := responseSession(t, login)
	csrf := jsonMap(t, login)["csrf_token"].(string)
	setup := mfaSessionRequest(t, a, "/api/v1/me/mfa/setup", map[string]any{"current_password": "correct horse battery staple 2026"}, cookie, csrf)
	if setup.Code != 200 {
		t.Fatal(setup.Code, setup.Body.String())
	}
	secret := jsonMap(t, setup)["secret"].(string)
	// Verify previous accepted step during enrollment so current step can be used
	// for the first login immediately, without sleeping or disabling replay checks.
	code, _ := totpAt(secret, NowMillis()/30000-1, 6)
	enabled := mfaSessionRequest(t, a, "/api/v1/me/mfa/enable", map[string]any{"code": code}, cookie, csrf)
	if enabled.Code != 200 {
		t.Fatal(enabled.Code, enabled.Body.String())
	}
	raw := jsonMap(t, enabled)["recovery_codes"].([]any)
	codes := make([]string, len(raw))
	for i, value := range raw {
		codes[i] = value.(string)
	}
	return responseSession(t, enabled), jsonMap(t, enabled)["csrf_token"].(string), secret, codes
}
func challengeVerify(t *testing.T, a *App, challenge *httptest.ResponseRecorder, code string) *httptest.ResponseRecorder {
	t.Helper()
	payload := jsonMap(t, challenge)
	nonce := cookieNamed(t, challenge, localNonceCookie)
	binding := cookieNamed(t, challenge, mfaChallengeCookie)
	raw, _ := json.Marshal(map[string]any{"challenge_id": payload["challenge_id"], "code": code})
	r := httptest.NewRequest("POST", "/api/v1/auth/mfa/verify", bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", a.Config.PublicBaseURL)
	r.Header.Set("X-CSRF-Token", payload["csrf_token"].(string))
	r.AddCookie(nonce)
	r.AddCookie(binding)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}
func TestMFASetupProofAndEnableRevokesOtherSessions(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "mfa-owner", "owner")
	other := responseSession(t, loginTestLocal(t, a, "mfa-owner", "correct horse battery staple 2026", nil))
	login := loginTestLocal(t, a, "mfa-owner", "correct horse battery staple 2026", nil)
	cookie := responseSession(t, login)
	csrf := jsonMap(t, login)["csrf_token"].(string)
	for _, input := range []struct {
		password, csrf string
		status         int
	}{{"wrong password", csrf, 401}, {"correct horse battery staple 2026", "bad", 403}} {
		got := mfaSessionRequest(t, a, "/api/v1/me/mfa/setup", map[string]any{"current_password": input.password}, cookie, input.csrf)
		if got.Code != input.status {
			t.Fatal(got.Code, got.Body.String())
		}
	}
	setup := mfaSessionRequest(t, a, "/api/v1/me/mfa/setup", map[string]any{"current_password": "correct horse battery staple 2026"}, cookie, csrf)
	if setup.Code != 200 {
		t.Fatal(setup.Code, setup.Body.String())
	}
	secret := jsonMap(t, setup)["secret"].(string)
	if !strings.HasPrefix(jsonMap(t, setup)["otpauth_uri"].(string), "otpauth://totp/") {
		t.Fatal("missing authenticator URI")
	}
	var ciphertext []byte
	if err := a.Pool.QueryRow(context.Background(), "SELECT pending_secret_ciphertext FROM user_mfa").Scan(&ciphertext); err != nil || bytes.Contains(ciphertext, []byte(secret)) {
		t.Fatal("plaintext secret", err)
	}
	code, _ := totpAt(secret, NowMillis()/30000, 6)
	enabled := mfaSessionRequest(t, a, "/api/v1/me/mfa/enable", map[string]any{"code": code}, cookie, csrf)
	if enabled.Code != 200 {
		t.Fatal(enabled.Code, enabled.Body.String())
	}
	codes := jsonMap(t, enabled)["recovery_codes"].([]any)
	if len(codes) != 10 {
		t.Fatal("recovery count")
	}
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", other, ""))["authenticated"] != false {
		t.Fatal("other session survived MFA enrollment")
	}
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", cookie, ""))["authenticated"] != false {
		t.Fatal("pre-enrollment cloned cookie gained MFA assurance")
	}
	cookie = responseSession(t, enabled)
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", cookie, ""))["authenticated"] != true {
		t.Fatal("enrolling session lost assurance")
	}
	security := testRequest(a, "GET", "/api/v1/me/security", "", cookie, "")
	if security.Code != 200 || jsonMap(t, security)["mfa_enabled"] != true || jsonMap(t, security)["recovery_codes_remaining"] != float64(10) {
		t.Fatal(security.Code, security.Body.String())
	}
	var leaked int
	if err := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM mfa_recovery_codes WHERE code_hash=$1", codes[0]).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatal("plaintext recovery code")
	}
}
func TestMFALoginNeedsChallengeAndRejectsReplay(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "mfa-login", "owner")
	_, _, secret, _ := enableTestMFA(t, a, "mfa-login")
	first := loginTestLocal(t, a, "mfa-login", "correct horse battery staple 2026", nil)
	if first.Code != 202 || jsonMap(t, first)["authenticated"] != false {
		t.Fatal(first.Code, first.Body.String())
	}
	for _, c := range first.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			t.Fatal("full session before second factor")
		}
	}
	code, _ := totpAt(secret, NowMillis()/30000, 6)
	success := challengeVerify(t, a, first, code)
	if success.Code != 200 || jsonMap(t, success)["authenticated"] != true {
		t.Fatal(success.Code, success.Body.String())
	}
	cookie := responseSession(t, success)
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", cookie, ""))["authenticated"] != true {
		t.Fatal("MFA session invalid")
	}
	if replay := challengeVerify(t, a, first, code); replay.Code != 401 {
		t.Fatal("challenge reused", replay.Code)
	}
	second := loginTestLocal(t, a, "mfa-login", "correct horse battery staple 2026", nil)
	if second.Code != 202 {
		t.Fatal(second.Code, second.Body.String())
	}
	replay := challengeVerify(t, a, second, code)
	if replay.Code != 401 || !strings.Contains(replay.Body.String(), "MFA_INVALID_CODE") {
		t.Fatal("TOTP reused across challenges", replay.Code, replay.Body.String())
	}
}
func TestMFARecoveryCodeConsumedAtomically(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "mfa-race", "owner")
	_, _, _, codes := enableTestMFA(t, a, "mfa-race")
	first := loginTestLocal(t, a, "mfa-race", "correct horse battery staple 2026", nil)
	second := loginTestLocal(t, a, "mfa-race", "correct horse battery staple 2026", nil)
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for _, challenge := range []*httptest.ResponseRecorder{first, second} {
		wg.Add(1)
		go func(c *httptest.ResponseRecorder) {
			defer wg.Done()
			results <- challengeVerify(t, a, c, codes[0]).Code
		}(challenge)
	}
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for status := range results {
		counts[status]++
	}
	if counts[200] != 1 || counts[401] != 1 {
		t.Fatal("recovery code used twice", counts)
	}
	var remaining int
	if err := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM mfa_recovery_codes WHERE consumed_at_ms IS NULL").Scan(&remaining); err != nil || remaining != 9 {
		t.Fatal(remaining, err)
	}
}
func TestMFAChallengeBindingAttemptsAndPasswordReset(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "mfa-limits", "owner")
	_, _, _, codes := enableTestMFA(t, a, "mfa-limits")
	first := loginTestLocal(t, a, "mfa-limits", "correct horse battery staple 2026", nil)
	payload := jsonMap(t, first)
	missing := localRequest(t, a, "/api/v1/auth/mfa/verify", map[string]any{"challenge_id": payload["challenge_id"], "code": codes[0]}, cookieNamed(t, first, localNonceCookie), payload["csrf_token"].(string), nil)
	if missing.Code != 401 {
		t.Fatal("challenge browser binding ignored", missing.Code)
	}
	// Distinct malformed codes can never collide with a real TOTP.
	for range 5 {
		got := challengeVerify(t, a, first, "invalid")
		if got.Code != 401 {
			t.Fatal(got.Code, got.Body.String())
		}
	}
	if got := challengeVerify(t, a, first, codes[0]); got.Code != 401 || !strings.Contains(got.Body.String(), "MFA_CHALLENGE_EXPIRED") {
		t.Fatal("challenge attempts not persisted", got.Code, got.Body.String())
	}
	second := loginTestLocal(t, a, "mfa-limits", "correct horse battery staple 2026", nil)
	if _, err := a.ResetLocalPassword(context.Background(), "mfa-limits", "a replacement password for 2026"); err != nil {
		t.Fatal(err)
	}
	if got := challengeVerify(t, a, second, codes[0]); got.Code != 401 {
		t.Fatal("old first factor survived password reset", got.Code)
	}
}
func TestMFASetupExpiresAndAttemptsPersist(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "mfa-setup-limit", "owner")
	login := loginTestLocal(t, a, "mfa-setup-limit", "correct horse battery staple 2026", nil)
	cookie := responseSession(t, login)
	csrf := jsonMap(t, login)["csrf_token"].(string)
	setup := mfaSessionRequest(t, a, "/api/v1/me/mfa/setup", map[string]any{"current_password": "correct horse battery staple 2026"}, cookie, csrf)
	if setup.Code != 200 {
		t.Fatal(setup.Code, setup.Body.String())
	}
	for range 5 {
		got := mfaSessionRequest(t, a, "/api/v1/me/mfa/enable", map[string]any{"code": "invalid"}, cookie, csrf)
		if got.Code != 401 {
			t.Fatal(got.Code, got.Body.String())
		}
	}
	secret := jsonMap(t, setup)["secret"].(string)
	code, _ := totpAt(secret, NowMillis()/30000, 6)
	if got := mfaSessionRequest(t, a, "/api/v1/me/mfa/enable", map[string]any{"code": code}, cookie, csrf); got.Code != 400 {
		t.Fatal("setup retry bypass", got.Code, got.Body.String())
	}
}
func TestMFADisableRegenerateAndOfflineRecovery(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "mfa-manage", "owner")
	cookie, csrf, _, codes := enableTestMFA(t, a, "mfa-manage")
	regenerated := mfaSessionRequest(t, a, "/api/v1/me/mfa/recovery-codes", map[string]any{"current_password": "correct horse battery staple 2026", "code": codes[0]}, cookie, csrf)
	if regenerated.Code != 200 {
		t.Fatal(regenerated.Code, regenerated.Body.String())
	}
	first := loginTestLocal(t, a, "mfa-manage", "correct horse battery staple 2026", nil)
	if got := challengeVerify(t, a, first, codes[1]); got.Code != 401 {
		t.Fatal("old recovery code survived rotation", got.Code)
	}
	newCodes := jsonMap(t, regenerated)["recovery_codes"].([]any)
	disabled := mfaSessionRequest(t, a, "/api/v1/me/mfa/disable", map[string]any{"current_password": "correct horse battery staple 2026", "code": newCodes[0]}, cookie, csrf)
	if disabled.Code != 204 {
		t.Fatal(disabled.Code, disabled.Body.String())
	}
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", cookie, ""))["authenticated"] != false {
		t.Fatal("disable retained sessions")
	}
	if got := loginTestLocal(t, a, "mfa-manage", "correct horse battery staple 2026", nil); got.Code != 200 {
		t.Fatal("disabled MFA still required", got.Code)
	}
	_, _, _, _ = enableTestMFA(t, a, "mfa-manage")
	if _, err := a.DisableLocalMFA(context.Background(), "mfa-manage"); err != nil {
		t.Fatal(err)
	}
	if got := loginTestLocal(t, a, "mfa-manage", "correct horse battery staple 2026", nil); got.Code != 200 {
		t.Fatal("offline recovery unavailable", got.Code)
	}
}
func TestExternalLoginPolicyClosesLocalAndGitHubWithoutLockingOwner(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "policy-owner", "owner")
	createTestLocal(t, a, "policy-reader", "reader")
	readerLogin := loginTestLocal(t, a, "policy-reader", "correct horse battery staple 2026", nil)
	reader := responseSession(t, readerLogin)
	ctx := context.Background()
	if _, err := a.Pool.Exec(ctx, `UPDATE site_settings SET registration_enabled=true,external_login_enabled=false WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	_, _, options := localOptions(t, a)
	if options["registration_open"] != false || options["external_login_enabled"] != false {
		t.Fatal(options)
	}
	if got := loginTestLocal(t, a, "policy-reader", "correct horse battery staple 2026", nil); got.Code != 403 {
		t.Fatal("reader bypass", got.Code, got.Body.String())
	}
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", reader, ""))["authenticated"] != false {
		t.Fatal("existing reader session bypass")
	}
	if got := loginTestLocal(t, a, "policy-owner", "correct horse battery staple 2026", nil); got.Code != 200 {
		t.Fatal("owner locked out", got.Code)
	}
	nonce, csrf, _ := localOptions(t, a)
	register := localRequest(t, a, "/api/v1/auth/register", map[string]any{"username": "blocked-new", "display_name": "New", "password": "a new reader password for 2026"}, nonce, csrf, nil)
	if register.Code != 403 {
		t.Fatal("registration auto-login bypass", register.Code)
	}
	if _, err := a.completeLogin(ctx, githubProfile{ID: json.Number("84"), Login: "blocked-github"}); err == nil {
		t.Fatal("GitHub registration bypass")
	}
	var id string
	if err := a.Pool.QueryRow(ctx, "SELECT id::text FROM users WHERE username='policy-reader'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Pool.Exec(ctx, `INSERT INTO oauth_accounts(provider,issuer,subject,user_id,provider_login,created_at_ms,updated_at_ms) VALUES('github',$1,'85',$2,'reader',1,1)`, githubIssuer, id); err != nil {
		t.Fatal(err)
	}
	if _, err := a.completeLogin(ctx, githubProfile{ID: json.Number("85"), Login: "reader"}); err == nil {
		t.Fatal("existing GitHub policy bypass")
	}
}
func TestGitHubLoginCannotBypassMFA(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "github-mfa", "owner")
	_, _, _, _ = enableTestMFA(t, a, "github-mfa")
	ctx := context.Background()
	var id string
	if err := a.Pool.QueryRow(ctx, "SELECT id::text FROM users WHERE username='github-mfa'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Pool.Exec(ctx, `INSERT INTO oauth_accounts(provider,issuer,subject,user_id,provider_login,created_at_ms,updated_at_ms) VALUES('github',$1,'86',$2,'mfa',1,1)`, githubIssuer, id); err != nil {
		t.Fatal(err)
	}
	token, pending, err := a.completeGitHubAuthentication(ctx, githubProfile{ID: json.Number("86"), Login: "mfa"}, nil, "")
	if err != nil || token != "" || pending == nil {
		t.Fatal("OAuth bypassed second factor", err)
	}
	if pending.Expires <= time.Now().UnixMilli() {
		t.Fatal("challenge missing expiry")
	}
	var sessions int
	if err := a.Pool.QueryRow(ctx, `SELECT count(*) FROM sessions WHERE user_id=$1 AND authentication_method='github'`, id).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatal("OAuth created full session before MFA", err, sessions)
	}
}

func TestMFADurableFailureLimitAcrossChallenges(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "mfa-durable", "owner")
	_, _, _, codes := enableTestMFA(t, a, "mfa-durable")
	for range 2 {
		challenge := loginTestLocal(t, a, "mfa-durable", "correct horse battery staple 2026", nil)
		for range 5 {
			if got := challengeVerify(t, a, challenge, "invalid"); got.Code != 401 {
				t.Fatal(got.Code, got.Body.String())
			}
		}
	}
	// Simulate resetting process-local rate buckets. Database limits still apply.
	a.rateMu.Lock()
	a.rates = nil
	a.rateMu.Unlock()
	challenge := loginTestLocal(t, a, "mfa-durable", "correct horse battery staple 2026", nil)
	if got := challengeVerify(t, a, challenge, codes[0]); got.Code != 401 {
		t.Fatal("durable user limit bypassed", got.Code)
	}
	var failures int
	var locked int64
	if err := a.Pool.QueryRow(context.Background(), "SELECT failed_attempts,locked_until_ms FROM user_mfa").Scan(&failures, &locked); err != nil || failures != 10 || locked <= NowMillis() {
		t.Fatal("missing durable backoff", err, failures, locked)
	}
}
func TestMFAExpiredChallengeAndSessionAssurance(t *testing.T) {
	a := testApp(t)
	createTestLocal(t, a, "mfa-assurance", "owner")
	cookie, _, _, codes := enableTestMFA(t, a, "mfa-assurance")
	ctx := context.Background()
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(cookie)
	p, err := a.Principal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.IdentitySessionAllowed(ctx, p.ID, p.SessionID); err != nil {
		t.Fatal(err)
	}
	method, _, mfaAt, err := a.SessionAuthentication(ctx, p.SessionID)
	if err != nil || mfaAt == nil || method != "password+mfa" {
		t.Fatal("missing session assurance", method, mfaAt, err)
	}
	// A row created through an outdated integration has no MFA assurance and must
	// never be accepted merely because it carries a syntactically valid cookie.
	forged := RandID()
	if _, err = a.Pool.Exec(ctx, `INSERT INTO sessions(token_hash,management_id,user_id,csrf_secret_hash,created_at_ms,expires_at_ms,last_seen_at_ms) VALUES($1,$2,$3,$4,$5,$6,$5)`, hashString(forged), RandID(), p.ID, hashString(a.csrf(forged)), NowMillis(), NowMillis()+60000); err != nil {
		t.Fatal(err)
	}
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", &http.Cookie{Name: sessionCookie, Value: forged}, ""))["authenticated"] != false {
		t.Fatal("legacy cookie bypassed second factor")
	}
	challenge := loginTestLocal(t, a, "mfa-assurance", "correct horse battery staple 2026", nil)
	if _, err = a.Pool.Exec(ctx, "UPDATE mfa_login_challenges SET expires_at_ms=$1", NowMillis()-1); err != nil {
		t.Fatal(err)
	}
	if got := challengeVerify(t, a, challenge, codes[0]); got.Code != 401 || !strings.Contains(got.Body.String(), "MFA_CHALLENGE_EXPIRED") {
		t.Fatal("expired challenge accepted", got.Code)
	}
	if _, err = a.Pool.Exec(ctx, "UPDATE site_settings SET external_login_enabled=false WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if err = a.IdentitySessionAllowed(ctx, p.ID, p.SessionID); err != nil {
		t.Fatal("owner SSO recovery locked out", err)
	}
}

func TestExternalLoginAdminSwitchRevokesAndPreservesOwner(t *testing.T) {
	a := testApp(t)
	a.RegisterAdmin()
	_, owner, csrf := testIdentity(t, a, "owner")
	reader, readerCookie, _ := testIdentity(t, a, "reader")
	settings := contentWant(t, contentCall(t, a, owner, csrf, "GET", "/api/v1/admin/settings", nil, 0, ""), 200)
	if settings["external_login_enabled"] != true {
		t.Fatal("backward-compatible default missing", settings)
	}
	settings["external_login_enabled"] = false
	saved := contentWant(t, contentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/settings", settings, 1, ""), 200)
	if saved["external_login_enabled"] != false {
		t.Fatal("login switch not stored", saved)
	}
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", owner, ""))["authenticated"] != true {
		t.Fatal("owner lockout")
	}
	if jsonMap(t, testRequest(a, "GET", "/api/v1/session", "", readerCookie, ""))["authenticated"] != false {
		t.Fatal("external session survived")
	}
	var revoked bool
	if err := a.Pool.QueryRow(context.Background(), "SELECT revoked_at_ms IS NOT NULL FROM sessions WHERE management_id=$1", reader.SessionID).Scan(&revoked); err != nil || !revoked {
		t.Fatal("external cookie could revive after reopening", err)
	}
	// Older clients omit the new field; they must not silently reopen login.
	delete(saved, "external_login_enabled")
	next := contentWant(t, contentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/settings", saved, 2, ""), 200)
	if next["external_login_enabled"] != false {
		t.Fatal("omitted policy reopened external login")
	}
}

func TestMFASessionHelpersUseTransactionConnectionAndSnapshot(t *testing.T) {
	a := testApp(t)
	principal, _, _ := testIdentity(t, a, "reader")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	config := a.Pool.Config()
	config.MaxConns = 1
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	now := NowMillis()
	encrypted, err := a.mfaCipher(principal.ID, []byte("transaction test secret"), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO user_mfa(user_id,secret_ciphertext,enabled_at_ms,updated_at_ms) VALUES($1,$2,$3,$3)`, principal.ID, encrypted, now); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE sessions SET authentication_method='password+mfa',authenticated_at_ms=$2,mfa_authenticated_at_ms=$2 WHERE management_id=$1`, principal.SessionID, now); err != nil {
		t.Fatal(err)
	}
	if err = identitySessionAllowed(ctx, tx, principal.ID, principal.SessionID); err != nil {
		t.Fatal("transaction-local MFA assurance rejected", err)
	}
	method, authenticatedAt, mfaAt, err := sessionAuthentication(ctx, tx, principal.SessionID)
	if err != nil || method != "password+mfa" || authenticatedAt.UnixMilli() != now || mfaAt == nil || mfaAt.UnixMilli() != now {
		t.Fatal("helper missed transaction-local authentication", method, authenticatedAt, mfaAt, err)
	}
	if _, err = tx.Exec(ctx, `UPDATE sessions SET mfa_authenticated_at_ms=NULL WHERE management_id=$1`, principal.SessionID); err != nil {
		t.Fatal(err)
	}
	if err = identitySessionAllowed(ctx, tx, principal.ID, principal.SessionID); err == nil {
		t.Fatal("helper ignored transaction-local MFA requirement")
	}
	if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at_ms=$2 WHERE management_id=$1`, principal.SessionID, now); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = sessionAuthentication(ctx, tx, principal.SessionID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("helper ignored transaction-local revocation", err)
	}
	if stats := pool.Stat(); stats.AcquiredConns() != 1 || stats.TotalConns() != 1 {
		t.Fatal("helper allocated another pooled connection")
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	// Public compatibility methods still use the pool, after the transaction's
	// uncommitted policy and assurance changes have been rolled back.
	if err = a.IdentitySessionAllowed(ctx, principal.ID, principal.SessionID); err != nil {
		t.Fatal(err)
	}
	method, _, mfaAt, err = a.SessionAuthentication(ctx, principal.SessionID)
	if err != nil || method != "legacy" || mfaAt != nil {
		t.Fatal("public helper compatibility", method, mfaAt, err)
	}
}
