package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const mfaChallengeCookie = "painter_mfa_challenge"
const mfaSetupLifetime = 10 * time.Minute
const mfaChallengeLifetime = 5 * time.Minute

// TOTP uses RFC 6238 / RFC 4226: SHA-1, six digits and a 30-second step.
// A previous/next step tolerates clock drift; last_totp_step makes every accepted
// step single-use across all challenges, sessions and recovery operations.
func totpAt(secret string, step int64, digits int) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil || len(key) < 20 || step < 0 || (digits != 6 && digits != 8) {
		return "", errors.New("invalid TOTP parameters")
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(counter[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 15
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	modulus := uint32(1000000)
	if digits == 8 {
		modulus = 100000000
	}
	return fmt.Sprintf("%0*d", digits, n%modulus), nil
}
func matchTOTPStep(secret, code string, now, after int64) (int64, bool) {
	if len(code) != 6 || strings.IndexFunc(code, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return 0, false
	}
	current := now / 30000
	// Prefer the current step, then a previous step, then a future step.
	for _, step := range []int64{current, current - 1, current + 1} {
		candidate, err := totpAt(secret, step, 6)
		if err == nil && step > after && subtle.ConstantTimeCompare([]byte(candidate), []byte(code)) == 1 {
			return step, true
		}
	}
	return 0, false
}
func newMFASecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}
func (a *App) mfaCipher(userID string, value []byte, decrypt bool) ([]byte, error) {
	block, err := aes.NewCipher(a.Config.EncryptionKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	aad := []byte("painter:mfa:v1:" + userID)
	if decrypt {
		if len(value) < gcm.NonceSize() {
			return nil, errors.New("invalid MFA ciphertext")
		}
		return gcm.Open(nil, value[:gcm.NonceSize()], value[gcm.NonceSize():], aad)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, value, aad), nil
}

// IdentityAccessAllowed is the shared account policy for federated clients too.
// Owners retain local recovery access; closing external login denies every other
// role, including already issued sessions and refresh tokens.
func (a *App) IdentityAccessAllowed(ctx context.Context, userID string) error {
	return identityAccessAllowed(ctx, a.Pool, userID)
}
func identityAccessAllowed(ctx context.Context, q githubSettingsQuery, userID string) error {
	var allowed bool
	err := q.QueryRow(ctx, `SELECT u.disabled_at_ms IS NULL AND (u.role='owner' OR s.external_login_enabled) FROM users u CROSS JOIN site_settings s WHERE u.id=$1 AND s.id=1`, userID).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !allowed) {
		return E(403, "ACCOUNT_LOGIN_CLOSED", "此账号暂时不能登录，请联系站长")
	}
	return err
}
func (a *App) IdentitySessionAllowed(ctx context.Context, userID, sessionID string) error {
	return identitySessionAllowed(ctx, a.Pool, userID, sessionID)
}

// Identity transactions must reuse their existing connection, both for policy
// visibility and to avoid acquiring another connection from a bounded pool.
func identitySessionAllowed(ctx context.Context, q githubSettingsQuery, userID, sessionID string) error {
	var allowed bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions se JOIN users u ON u.id=se.user_id CROSS JOIN site_settings s LEFT JOIN user_mfa m ON m.user_id=u.id WHERE se.user_id=$1 AND se.management_id=$2 AND se.revoked_at_ms IS NULL AND se.expires_at_ms>$3 AND u.disabled_at_ms IS NULL AND (u.role='owner' OR s.external_login_enabled) AND (m.enabled_at_ms IS NULL OR se.mfa_authenticated_at_ms>=m.enabled_at_ms) AND s.id=1)`, userID, sessionID, NowMillis()).Scan(&allowed)
	if err == nil && !allowed {
		return E(401, "UNAUTHENTICATED", "登录已过期，请重新验证身份")
	}
	return err
}
func (a *App) SessionAuthentication(ctx context.Context, sessionID string) (method string, authenticatedAt time.Time, mfaAt *time.Time, err error) {
	return sessionAuthentication(ctx, a.Pool, sessionID)
}
func sessionAuthentication(ctx context.Context, q githubSettingsQuery, sessionID string) (method string, authenticatedAt time.Time, mfaAt *time.Time, err error) {
	var created int64
	var mfa *int64
	err = q.QueryRow(ctx, `SELECT authentication_method,COALESCE(authenticated_at_ms,created_at_ms),mfa_authenticated_at_ms FROM sessions WHERE management_id=$1 AND revoked_at_ms IS NULL AND expires_at_ms>$2`, sessionID, NowMillis()).Scan(&method, &created, &mfa)
	authenticatedAt = time.UnixMilli(created)
	if mfa != nil {
		value := time.UnixMilli(*mfa)
		mfaAt = &value
	}
	return
}
func (a *App) RegisterMFA() {
	a.Handle("GET /api/v1/me/security", a.mfaSecurity)
	a.Handle("POST /api/v1/me/mfa/setup", a.mfaSetup)
	a.Handle("POST /api/v1/me/mfa/enable", a.mfaEnable)
	a.Handle("POST /api/v1/me/mfa/disable", a.mfaDisable)
	a.Handle("POST /api/v1/me/mfa/recovery-codes", a.mfaRegenerateCodes)
	a.Handle("GET /api/v1/auth/mfa/challenge", a.mfaChallengeGet)
	a.Handle("POST /api/v1/auth/mfa/verify", a.mfaVerify)
}
func (a *App) mfaSecurity(w http.ResponseWriter, r *http.Request) error {
	p, err := a.Require(r)
	if err != nil {
		return err
	}
	var enabled, local bool
	var remaining int
	var enabledAt *int64
	err = a.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM user_mfa WHERE user_id=$1 AND enabled_at_ms IS NOT NULL),EXISTS(SELECT 1 FROM local_accounts WHERE user_id=$1),(SELECT count(*) FROM mfa_recovery_codes WHERE user_id=$1 AND consumed_at_ms IS NULL),(SELECT enabled_at_ms FROM user_mfa WHERE user_id=$1)`, p.ID).Scan(&enabled, &local, &remaining, &enabledAt)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, map[string]any{"mfa_enabled": enabled, "mfa_enabled_at_ms": enabledAt, "recovery_codes_remaining": remaining, "has_local_password": local})
	return nil
}

// Returns the verified hash for an in-transaction race check. OAuth-only users
// must have completed a fresh GitHub authentication within five minutes.
func (a *App) mfaReauthenticate(r *http.Request, p Principal, password string) (string, error) {
	if err := a.checkCSRF(r); err != nil {
		return "", err
	}
	if !a.allowLocalAuth(r, "mfa-settings/"+p.ID, false) {
		return "", E(429, "RATE_LIMITED", "尝试过于频繁，请稍后再试")
	}
	var encoded string
	err := a.Pool.QueryRow(r.Context(), "SELECT password_hash FROM local_accounts WHERE user_id=$1", p.ID).Scan(&encoded)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	matched, err := verifyPassword(r.Context(), password, encoded)
	if err != nil {
		return "", err
	}
	if !matched {
		return "", invalidCredentials()
	}
	return encoded, nil
}
func mfaReauthTx(ctx context.Context, tx pgx.Tx, p Principal, expected string) error {
	if expected != "" {
		var actual string
		if err := tx.QueryRow(ctx, "SELECT password_hash FROM local_accounts WHERE user_id=$1", p.ID).Scan(&actual); err != nil {
			return err
		}
		if actual != expected {
			return E(401, "REAUTHENTICATION_REQUIRED", "密码已变化，请重新验证身份")
		}
		return nil
	}
	var recent bool
	err := tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM local_accounts WHERE user_id=$1) AND EXISTS(SELECT 1 FROM sessions WHERE management_id=$2 AND user_id=$1 AND authentication_method IN('github','github+mfa') AND COALESCE(authenticated_at_ms,created_at_ms)>$3 AND revoked_at_ms IS NULL AND expires_at_ms>$4)`, p.ID, p.SessionID, NowMillis()-int64(mfaChallengeLifetime/time.Millisecond), NowMillis()).Scan(&recent)
	if err != nil {
		return err
	}
	if !recent {
		return E(401, "REAUTHENTICATION_REQUIRED", "请先重新通过 GitHub 登录，再管理两步验证")
	}
	return nil
}
func (a *App) mfaSetup(w http.ResponseWriter, r *http.Request) error {
	pre, err := a.Require(r)
	if err != nil {
		return err
	}
	var in struct {
		CurrentPassword string `json:"current_password"`
	}
	if err = decodeLocalAuth(w, r, &in); err != nil {
		return err
	}
	expected, err := a.mfaReauthenticate(r, pre, in.CurrentPassword)
	if err != nil {
		return err
	}
	secret, err := newMFASecret()
	if err != nil {
		return err
	}
	encrypted, err := a.mfaCipher(pre.ID, []byte(secret), false)
	if err != nil {
		return err
	}
	expires := NowMillis() + mfaSetupLifetime.Milliseconds()
	*r = *r.WithContext(context.WithValue(r.Context(), updateActorKey, true))
	_, err = a.Write(r, nil, true, false, func(tx pgx.Tx, p Principal) (any, error) {
		if err := mfaReauthTx(r.Context(), tx, p, expected); err != nil {
			return nil, err
		}
		var enabled bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM user_mfa WHERE user_id=$1 AND enabled_at_ms IS NOT NULL)`, p.ID).Scan(&enabled); err != nil {
			return nil, err
		}
		if enabled {
			return nil, E(409, "MFA_ALREADY_ENABLED", "已启用两步验证，请先关闭后再更换验证器")
		}
		_, err := tx.Exec(r.Context(), `INSERT INTO user_mfa(user_id,pending_secret_ciphertext,pending_expires_at_ms,updated_at_ms) VALUES($1,$2,$3,$4) ON CONFLICT(user_id) DO UPDATE SET pending_secret_ciphertext=EXCLUDED.pending_secret_ciphertext,pending_expires_at_ms=EXCLUDED.pending_expires_at_ms,pending_attempts=0,updated_at_ms=EXCLUDED.updated_at_ms`, p.ID, encrypted, expires, NowMillis())
		return nil, err
	})
	if err != nil {
		return err
	}
	issuer := "Painter"
	u := url.URL{Scheme: "otpauth", Host: "totp", Path: "/" + issuer + ":" + pre.Username}
	q := url.Values{"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	u.RawQuery = q.Encode()
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, map[string]any{"secret": secret, "otpauth_uri": u.String(), "expires_at": TimeString(expires)})
	return nil
}
func (a *App) newRecoveryCodes(ctx context.Context, tx pgx.Tx, userID string) ([]string, error) {
	if _, err := tx.Exec(ctx, "DELETE FROM mfa_recovery_codes WHERE user_id=$1", userID); err != nil {
		return nil, err
	}
	codes := make([]string, 10)
	for i := range codes {
		raw := strings.ToUpper(RandID()[:20])
		code := raw[:5] + "-" + raw[5:10] + "-" + raw[10:15] + "-" + raw[15:]
		if _, err := tx.Exec(ctx, `INSERT INTO mfa_recovery_codes(user_id,code_hash,created_at_ms) VALUES($1,$2,$3)`, userID, a.recoveryHash(userID, code), NowMillis()); err != nil {
			return nil, err
		}
		codes[i] = code
	}
	return codes, nil
}
func (a *App) recoveryHash(userID, code string) string {
	canonical := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
	return macString(a.Config.EncryptionKey, "mfa-recovery:v1:"+userID+":"+canonical)
}
func (a *App) mfaEnable(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Code string `json:"code"`
	}
	if err := decodeLocalAuth(w, r, &in); err != nil {
		return err
	}
	*r = *r.WithContext(context.WithValue(r.Context(), updateActorKey, true))
	var rejected error
	var codes []string
	sessionToken := RandID()
	_, err := a.Write(r, nil, true, false, func(tx pgx.Tx, p Principal) (any, error) {
		var encrypted []byte
		var expires *int64
		var enabled *int64
		var attempts int
		err := tx.QueryRow(r.Context(), `SELECT pending_secret_ciphertext,pending_expires_at_ms,enabled_at_ms,pending_attempts FROM user_mfa WHERE user_id=$1 FOR UPDATE`, p.ID).Scan(&encrypted, &expires, &enabled, &attempts)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, E(400, "MFA_SETUP_EXPIRED", "请重新开始设置两步验证")
		}
		if err != nil {
			return nil, err
		}
		if enabled != nil {
			return nil, E(409, "MFA_ALREADY_ENABLED", "已启用两步验证")
		}
		if expires == nil || *expires <= NowMillis() || attempts >= 5 {
			return nil, E(400, "MFA_SETUP_EXPIRED", "设置已过期或尝试次数过多，请重新开始")
		}
		secret, err := a.mfaCipher(p.ID, encrypted, true)
		if err != nil {
			return nil, err
		}
		step, ok := matchTOTPStep(string(secret), strings.TrimSpace(in.Code), NowMillis(), -1)
		if !ok {
			_, err = tx.Exec(r.Context(), `UPDATE user_mfa SET pending_attempts=pending_attempts+1 WHERE user_id=$1`, p.ID)
			rejected = E(401, "MFA_INVALID_CODE", "验证码不正确，请检查验证器时间")
			return nil, err
		}
		now := NowMillis()
		if _, err = tx.Exec(r.Context(), `UPDATE user_mfa SET secret_ciphertext=pending_secret_ciphertext,enabled_at_ms=$2,last_totp_step=$3,pending_secret_ciphertext=NULL,pending_expires_at_ms=NULL,pending_attempts=0,failed_attempts=0,locked_until_ms=0,updated_at_ms=$2 WHERE user_id=$1`, p.ID, now, step); err != nil {
			return nil, err
		}
		codes, err = a.newRecoveryCodes(r.Context(), tx, p.ID)
		if err != nil {
			return nil, err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE sessions SET revoked_at_ms=$3 WHERE user_id=$1 AND management_id<>$2 AND revoked_at_ms IS NULL`, p.ID, p.SessionID, now); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE sessions SET token_hash=$3,csrf_secret_hash=$4,authenticated_at_ms=$2,mfa_authenticated_at_ms=$2,authentication_method=CASE WHEN authentication_method IN('github','github+mfa') THEN 'github+mfa' ELSE 'password+mfa' END WHERE management_id=$1`, p.SessionID, now, hashString(sessionToken), hashString(a.csrf(sessionToken))); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE users SET version=version+1,updated_at_ms=$2 WHERE id=$1`, p.ID, now); err != nil {
			return nil, err
		}
		return nil, audit(r.Context(), tx, p, "user.update", "user", p.ID, "two-factor authentication enabled; other sessions revoked")
	})
	if err != nil {
		return err
	}
	if rejected != nil {
		return rejected
	}
	a.cookie(w, sessionCookie, sessionToken, int(sessionLifetime.Seconds()))
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, map[string]any{"mfa_enabled": true, "recovery_codes": codes, "csrf_token": a.csrf(sessionToken)})
	return nil
}

// Called only while the user's row is exclusively locked. Failed checks are
// committed by callers; counters therefore survive restart and challenge rotation.
func (a *App) consumeMFA(ctx context.Context, tx pgx.Tx, userID, code string, now int64) (bool, error) {
	var encrypted []byte
	var enabled *int64
	var last, window, locked int64
	var failures int
	err := tx.QueryRow(ctx, `SELECT secret_ciphertext,enabled_at_ms,last_totp_step,failure_window_start_ms,failed_attempts,locked_until_ms FROM user_mfa WHERE user_id=$1 FOR UPDATE`, userID).Scan(&encrypted, &enabled, &last, &window, &failures, &locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if enabled == nil || locked > now {
		return false, nil
	}
	if now-window >= 10*60*1000 {
		window = now
		failures = 0
	}
	secret, err := a.mfaCipher(userID, encrypted, true)
	if err != nil {
		return false, err
	}
	step, accepted := matchTOTPStep(string(secret), strings.TrimSpace(code), now, last)
	recovery := false
	if !accepted && len(code) <= 64 {
		tag, err := tx.Exec(ctx, `UPDATE mfa_recovery_codes SET consumed_at_ms=$3 WHERE user_id=$1 AND code_hash=$2 AND consumed_at_ms IS NULL`, userID, a.recoveryHash(userID, code), now)
		if err != nil {
			return false, err
		}
		recovery = tag.RowsAffected() == 1
		accepted = recovery
	}
	if accepted {
		if recovery {
			step = last
		}
		_, err = tx.Exec(ctx, `UPDATE user_mfa SET last_totp_step=$2,failure_window_start_ms=$3,failed_attempts=0,locked_until_ms=0,updated_at_ms=$3 WHERE user_id=$1`, userID, step, now)
		return true, err
	}
	failures++
	if failures >= 10 {
		locked = now + 10*60*1000
	}
	_, err = tx.Exec(ctx, `UPDATE user_mfa SET failure_window_start_ms=$2,failed_attempts=$3,locked_until_ms=$4 WHERE user_id=$1`, userID, window, failures, locked)
	return false, err
}
func (a *App) mfaDisable(w http.ResponseWriter, r *http.Request) error {
	return a.mfaManage(w, r, true)
}
func (a *App) mfaRegenerateCodes(w http.ResponseWriter, r *http.Request) error {
	return a.mfaManage(w, r, false)
}
func (a *App) mfaManage(w http.ResponseWriter, r *http.Request, disable bool) error {
	pre, err := a.Require(r)
	if err != nil {
		return err
	}
	var in struct {
		CurrentPassword string `json:"current_password"`
		Code            string `json:"code"`
	}
	if err = decodeLocalAuth(w, r, &in); err != nil {
		return err
	}
	expected, err := a.mfaReauthenticate(r, pre, in.CurrentPassword)
	if err != nil {
		return err
	}
	*r = *r.WithContext(context.WithValue(r.Context(), updateActorKey, true))
	var rejected error
	var codes []string
	_, err = a.Write(r, nil, true, false, func(tx pgx.Tx, p Principal) (any, error) {
		if err := mfaReauthTx(r.Context(), tx, p, expected); err != nil {
			return nil, err
		}
		now := NowMillis()
		ok, err := a.consumeMFA(r.Context(), tx, p.ID, in.Code, now)
		if err != nil {
			return nil, err
		}
		if !ok {
			rejected = E(401, "MFA_INVALID_CODE", "验证码无效、已使用或尝试过于频繁")
			return nil, nil
		}
		reason := "two-factor recovery codes regenerated; other sessions revoked"
		if disable {
			reason = "two-factor authentication disabled; all sessions revoked"
			if _, err = tx.Exec(r.Context(), "DELETE FROM user_mfa WHERE user_id=$1", p.ID); err != nil {
				return nil, err
			}
			if _, err = tx.Exec(r.Context(), "DELETE FROM mfa_recovery_codes WHERE user_id=$1", p.ID); err != nil {
				return nil, err
			}
		} else {
			codes, err = a.newRecoveryCodes(r.Context(), tx, p.ID)
			if err != nil {
				return nil, err
			}
		}
		if _, err = tx.Exec(r.Context(), `UPDATE sessions SET revoked_at_ms=$3 WHERE user_id=$1 AND ($4 OR management_id<>$2) AND revoked_at_ms IS NULL`, p.ID, p.SessionID, now, disable); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE mfa_login_challenges SET consumed_at_ms=$2 WHERE user_id=$1 AND consumed_at_ms IS NULL`, p.ID, now); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE users SET version=version+1,updated_at_ms=$2 WHERE id=$1`, p.ID, now); err != nil {
			return nil, err
		}
		return nil, audit(r.Context(), tx, p, "user.update", "user", p.ID, reason)
	})
	if err != nil {
		return err
	}
	if rejected != nil {
		return rejected
	}
	if disable {
		a.cookie(w, sessionCookie, "", -1)
		w.WriteHeader(204)
	} else {
		JSON(w, 200, map[string]any{"recovery_codes": codes})
	}
	return nil
}

type pendingMFA struct {
	ID, Token string
	Expires   int64
}

func (a *App) beginMFA(ctx context.Context, tx pgx.Tx, userID, method string) (*pendingMFA, error) {
	var enabled bool
	var version int64
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_mfa WHERE user_id=$1 AND enabled_at_ms IS NOT NULL),(SELECT version FROM users WHERE id=$1)`, userID).Scan(&enabled, &version); err != nil {
		return nil, err
	}
	if !enabled {
		return nil, nil
	}
	value := &pendingMFA{ID: RandID(), Token: RandID(), Expires: NowMillis() + mfaChallengeLifetime.Milliseconds()}
	_, err := tx.Exec(ctx, `INSERT INTO mfa_login_challenges(id,token_hash,user_id,authentication_method,user_version,created_at_ms,expires_at_ms) VALUES($1,$2,$3,$4,$5,$6,$7)`, value.ID, hashString(value.Token), userID, method, version, NowMillis(), value.Expires)
	return value, err
}
func (a *App) mfaChallengeResponse(w http.ResponseWriter, r *http.Request, pending *pendingMFA) error {
	a.cookie(w, mfaChallengeCookie, pending.Token, int(mfaChallengeLifetime.Seconds()))
	nonce := RandID()
	if c, err := r.Cookie(localNonceCookie); err == nil && len(c.Value) == 48 {
		nonce = c.Value
	}
	a.cookie(w, localNonceCookie, nonce, int(mfaChallengeLifetime.Seconds()))
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 202, map[string]any{"authenticated": false, "mfa_required": true, "challenge_id": pending.ID, "expires_at": TimeString(pending.Expires), "csrf_token": macString(a.Config.EncryptionKey, "local-auth:v1:"+nonce)})
	return nil
}
func (a *App) mfaChallengeGet(w http.ResponseWriter, r *http.Request) error {
	c, err := r.Cookie(mfaChallengeCookie)
	if err != nil || len(c.Value) != 48 {
		return E(401, "MFA_CHALLENGE_EXPIRED", "请重新登录")
	}
	var id string
	var expires int64
	err = a.Pool.QueryRow(r.Context(), `SELECT c.id,c.expires_at_ms FROM mfa_login_challenges c JOIN users u ON u.id=c.user_id JOIN user_mfa m ON m.user_id=u.id CROSS JOIN site_settings s WHERE c.token_hash=$1 AND c.consumed_at_ms IS NULL AND c.expires_at_ms>$2 AND c.attempts<5 AND c.user_version=u.version AND u.disabled_at_ms IS NULL AND m.enabled_at_ms IS NOT NULL AND (u.role='owner' OR s.external_login_enabled) AND s.id=1`, hashString(c.Value), NowMillis()).Scan(&id, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return E(401, "MFA_CHALLENGE_EXPIRED", "验证已过期，请重新登录")
	}
	if err != nil {
		return err
	}
	return a.mfaChallengeResponse(w, r, &pendingMFA{ID: id, Token: c.Value, Expires: expires})
}
func (a *App) mfaVerify(w http.ResponseWriter, r *http.Request) error {
	if err := a.checkLocalAuthCSRF(r); err != nil {
		return err
	}
	var in struct {
		ChallengeID string `json:"challenge_id"`
		Code        string `json:"code"`
	}
	if err := decodeLocalAuth(w, r, &in); err != nil {
		return err
	}
	c, err := r.Cookie(mfaChallengeCookie)
	if err != nil || len(c.Value) != 48 || len(in.ChallengeID) != 48 {
		return E(401, "MFA_CHALLENGE_EXPIRED", "请重新登录")
	}
	if !a.allowLocalAuth(r, "mfa-challenge/"+in.ChallengeID, false) {
		return E(429, "RATE_LIMITED", "尝试过于频繁，请稍后再试")
	}
	token := RandID()
	var rejected error
	err = a.identityMutation(r.Context(), func(tx pgx.Tx) error {
		var userID string
		err := tx.QueryRow(r.Context(), `SELECT user_id::text FROM mfa_login_challenges WHERE id=$1 AND token_hash=$2`, in.ChallengeID, hashString(c.Value)).Scan(&userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return E(401, "MFA_CHALLENGE_EXPIRED", "请重新登录")
		}
		if err != nil {
			return err
		}
		// All identity-changing flows take the user lock before the challenge/MFA row.
		var version int64
		if err = tx.QueryRow(r.Context(), "SELECT version FROM users WHERE id=$1 FOR UPDATE", userID).Scan(&version); err != nil {
			return err
		}
		if err = identityAccessAllowed(r.Context(), tx, userID); err != nil {
			return err
		}
		var method string
		var expires, expected int64
		var consumed *int64
		var attempts int
		err = tx.QueryRow(r.Context(), `SELECT authentication_method,expires_at_ms,user_version,consumed_at_ms,attempts FROM mfa_login_challenges WHERE id=$1 AND token_hash=$2 FOR UPDATE`, in.ChallengeID, hashString(c.Value)).Scan(&method, &expires, &expected, &consumed, &attempts)
		if err != nil {
			return err
		}
		now := NowMillis()
		if consumed != nil || expires <= now || attempts >= 5 || expected != version {
			return E(401, "MFA_CHALLENGE_EXPIRED", "验证已过期，请重新登录")
		}
		ok, err := a.consumeMFA(r.Context(), tx, userID, in.Code, now)
		if err != nil {
			return err
		}
		if !ok {
			if _, err = tx.Exec(r.Context(), `UPDATE mfa_login_challenges SET attempts=attempts+1 WHERE id=$1`, in.ChallengeID); err != nil {
				return err
			}
			rejected = E(401, "MFA_INVALID_CODE", "验证码无效、已使用或尝试过于频繁")
			return nil
		}
		if _, err = tx.Exec(r.Context(), `UPDATE mfa_login_challenges SET consumed_at_ms=$2 WHERE id=$1`, in.ChallengeID, now); err != nil {
			return err
		}
		if err = a.insertAuthenticatedSession(r.Context(), tx, r, userID, token, method+"+mfa", &now, now); err != nil {
			return err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE users SET last_login_at_ms=$2 WHERE id=$1`, userID, now); err != nil {
			return err
		}
		var role string
		if err = tx.QueryRow(r.Context(), "SELECT role FROM users WHERE id=$1", userID).Scan(&role); err != nil {
			return err
		}
		if role == "owner" {
			if err = markInstallation(r.Context(), tx, userID, now); err != nil {
				return err
			}
		}
		return audit(r.Context(), tx, Principal{ID: userID}, "user.update", "user", userID, "two-factor login completed")
	})
	if err != nil {
		return err
	}
	if rejected != nil {
		return rejected
	}
	a.cookie(w, mfaChallengeCookie, "", -1)
	return a.localLoginResponse(w, r, token, 200)
}
func (a *App) insertAuthenticatedSession(ctx context.Context, tx pgx.Tx, r *http.Request, userID, token, method string, mfaAt *int64, now int64) error {
	if r != nil {
		if old, err := r.Cookie(sessionCookie); err == nil {
			if _, err = tx.Exec(ctx, `UPDATE sessions SET revoked_at_ms=$2 WHERE token_hash=$1 AND revoked_at_ms IS NULL`, hashString(old.Value), now); err != nil {
				return err
			}
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO sessions(token_hash,management_id,user_id,csrf_secret_hash,created_at_ms,expires_at_ms,last_seen_at_ms,authentication_method,mfa_authenticated_at_ms,authenticated_at_ms) VALUES($1,$2,$3,$4,$5,$6,$5,$7,$8,$5)`, hashString(token), RandID(), userID, hashString(a.csrf(token)), now, now+sessionLifetime.Milliseconds(), method, mfaAt)
	return err
}

// DisableLocalMFA is an explicit offline recovery action for the OS operator.
// There is intentionally no HTTP admin reset endpoint. Recovery revokes every
// cookie session, challenge and recovery code before allowing a fresh login.
func (a *App) DisableLocalMFA(ctx context.Context, username string) (map[string]any, error) {
	username, err := normalizeLocalUsername(username)
	if err != nil {
		return nil, err
	}
	tx, err := a.maintenance(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	var id string
	if err = tx.QueryRow(ctx, "SELECT id::text FROM users WHERE lower(username)=$1 FOR UPDATE", username).Scan(&id); err != nil {
		return nil, err
	}
	now := NowMillis()
	if _, err = tx.Exec(ctx, "DELETE FROM user_mfa WHERE user_id=$1", id); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM mfa_recovery_codes WHERE user_id=$1", id); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "UPDATE mfa_login_challenges SET consumed_at_ms=$2 WHERE user_id=$1 AND consumed_at_ms IS NULL", id, now); err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, "UPDATE sessions SET revoked_at_ms=$2 WHERE user_id=$1 AND revoked_at_ms IS NULL", id, now)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "UPDATE users SET version=version+1,updated_at_ms=$2 WHERE id=$1", id, now); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_user_id,action,target_type,target_id,request_id,reason,created_at_ms) VALUES(NULL,'user.update','user',$1,$2,'offline two-factor reset; all sessions revoked',$3)`, id, requestID(ctx), now); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "username": username, "mfa_disabled": true, "sessions_revoked": tag.RowsAffected()}, nil
}
