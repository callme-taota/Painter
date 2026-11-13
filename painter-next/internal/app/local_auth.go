package app

import (
	"context"
	"crypto/hmac"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const localNonceCookie = "painter_auth_nonce"
const localAuthBodyLimit = 12 << 10

func normalizeLocalUsername(raw string) (string, error) {
	username := strings.ToLower(strings.TrimSpace(raw))
	if !usernamePattern.MatchString(username) {
		return "", E(400, "VALIDATION_FAILED", "用户名须为2至64位字母、数字、下划线或连字符，以字母或数字开头")
	}
	return username, nil
}
func localDisplayName(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 100 || strings.ContainsRune(value, '\x00') {
		return "", E(400, "VALIDATION_FAILED", "昵称须为1至100字符")
	}
	return value, nil
}
func (a *App) localAuthOptions(w http.ResponseWriter, r *http.Request) error {
	github, err := a.effectiveGitHub(r.Context(), a.Pool)
	if err != nil {
		var api *APIError
		if !errors.As(err, &api) || api.Code != "OAUTH_CONFIG_UNAVAILABLE" {
			return err
		}
		// A broken optional provider must not lock local users out of recovery.
		github.Enabled = false
	}
	var registration, external bool
	if err := a.Pool.QueryRow(r.Context(), "SELECT registration_enabled,external_login_enabled FROM site_settings WHERE id=1").Scan(&registration, &external); err != nil {
		return err
	}
	nonce := RandID()
	if c, err := r.Cookie(localNonceCookie); err == nil && len(c.Value) == 48 {
		nonce = c.Value
	}
	a.cookie(w, localNonceCookie, nonce, 600)
	JSON(w, 200, map[string]any{"csrf_token": macString(a.Config.EncryptionKey, "local-auth:v1:"+nonce), "registration_open": registration && external, "external_login_enabled": external, "mfa_available": true, "local_login_enabled": true, "github_login_enabled": github.Enabled, "password_min_length": passwordMinLength, "password_max_length": passwordMaxLength, "username_min_length": 2, "username_max_length": 64})
	return nil
}
func (a *App) checkLocalAuthCSRF(r *http.Request) error {
	if r.Header.Get("Origin") != a.Config.PublicBaseURL {
		return E(403, "ORIGIN_REJECTED", "请求来源无效")
	}
	c, err := r.Cookie(localNonceCookie)
	if err != nil || len(c.Value) != 48 || !hmac.Equal([]byte(r.Header.Get("X-CSRF-Token")), []byte(macString(a.Config.EncryptionKey, "local-auth:v1:"+c.Value))) {
		return E(403, "CSRF_FAILED", "登录校验已过期，请刷新后重试")
	}
	return nil
}
func decodeLocalAuth(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, localAuthBodyLimit)
	if r.ContentLength > localAuthBodyLimit {
		return E(413, "PAYLOAD_TOO_LARGE", "登录请求过大")
	}
	return Decode(r, v)
}

// Limits count every attempt, including successes, and expire automatically.
// Client IPs come from the direct peer unless an explicitly trusted Cloudflare
// connector is configured. Account buckets apply independently of the IP source.
func (a *App) allowLocalAuth(r *http.Request, username string, registration bool) bool {
	host := a.ClientIP(r)
	scope := "login"
	ipLimit := 60
	accountLimit := 12
	if registration {
		scope = "register"
		ipLimit = 8
		accountLimit = 5
	}
	keys := []string{"local-auth/" + scope + "/ip/" + host, "local-auth/" + scope + "/account/" + hashString(username)}
	limits := []int{ipLimit, accountLimit}
	now := NowMillis()
	a.rateMu.Lock()
	defer a.rateMu.Unlock()
	if a.rates == nil {
		a.rates = make(map[string]rateBucket)
	}
	if len(a.rates) >= 10000 {
		for k, v := range a.rates {
			if v.Reset <= now {
				delete(a.rates, k)
			}
		}
	}
	for i, key := range keys {
		b, exists := a.rates[key]
		if !exists && len(a.rates) >= 10000 {
			return false
		}
		if b.Reset <= now {
			b = rateBucket{Reset: now + 10*60*1000}
		}
		if b.Count >= limits[i] {
			return false
		}
		// Reserve each rate bucket before starting expensive password work.
		b.Count++
		a.rates[key] = b
	}
	return true
}
func invalidCredentials() error { return E(401, "INVALID_CREDENTIALS", "用户名或密码不正确") }
func (a *App) localLogin(w http.ResponseWriter, r *http.Request) error {
	if err := a.checkLocalAuthCSRF(r); err != nil {
		return err
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeLocalAuth(w, r, &in); err != nil {
		return err
	}
	username, nameErr := normalizeLocalUsername(in.Username)
	if !a.allowLocalAuth(r, strings.ToLower(strings.TrimSpace(in.Username)), false) {
		return E(429, "RATE_LIMITED", "尝试过于频繁，请稍后再试")
	}
	var id, storedHash string
	var disabled *int64
	lookupErr := a.Pool.QueryRow(r.Context(), "SELECT u.id::text,l.password_hash,u.disabled_at_ms FROM users u JOIN local_accounts l ON l.user_id=u.id WHERE lower(u.username)=$1", username).Scan(&id, &storedHash, &disabled)
	if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
		return lookupErr
	}
	candidate := storedHash
	if candidate == "" {
		candidate = dummyPasswordHash
	}
	matched, err := verifyPassword(r.Context(), in.Password, candidate)
	if err != nil {
		return err
	}
	if nameErr != nil || lookupErr != nil || !matched || disabled != nil {
		return invalidCredentials()
	}
	token := RandID()
	var pending *pendingMFA
	err = a.identityMutation(r.Context(), func(tx pgx.Tx) error {
		var latest string
		var disabled *int64
		err := tx.QueryRow(r.Context(), "SELECT l.password_hash,u.disabled_at_ms FROM users u JOIN local_accounts l ON l.user_id=u.id WHERE u.id=$1 AND lower(u.username)=$2 FOR UPDATE OF u", id, username).Scan(&latest, &disabled)
		if errors.Is(err, pgx.ErrNoRows) {
			return invalidCredentials()
		}
		if err != nil {
			return err
		}
		if latest != storedHash || disabled != nil {
			return invalidCredentials()
		}
		if err = identityAccessAllowed(r.Context(), tx, id); err != nil {
			return err
		}
		pending, err = a.beginMFA(r.Context(), tx, id, "password")
		if err != nil {
			return err
		}
		if pending != nil {
			return nil
		}
		now := NowMillis()
		if _, err = tx.Exec(r.Context(), "UPDATE users SET last_login_at_ms=$2 WHERE id=$1", id, now); err != nil {
			return err
		}
		return a.insertLocalSession(r.Context(), tx, r, id, token, now)
	})
	if err != nil {
		return err
	}
	if pending != nil {
		return a.mfaChallengeResponse(w, r, pending)
	}
	return a.localLoginResponse(w, r, token, 200)
}
func (a *App) insertLocalSession(ctx context.Context, tx pgx.Tx, r *http.Request, id, token string, now int64) error {
	return a.insertAuthenticatedSession(ctx, tx, r, id, token, "password", nil, now)
}
func (a *App) localLoginResponse(w http.ResponseWriter, r *http.Request, token string, status int) error {
	a.cookie(w, sessionCookie, token, int(sessionLifetime.Seconds()))
	a.cookie(w, localNonceCookie, "", -1)
	copy := r.Clone(r.Context())
	copy.Header = r.Header.Clone()
	copy.Header.Set("Cookie", (&http.Cookie{Name: sessionCookie, Value: token}).String())
	return a.writeSession(w, copy, status)
}
func (a *App) localRegister(w http.ResponseWriter, r *http.Request) error {
	if err := a.checkLocalAuthCSRF(r); err != nil {
		return err
	}
	var in struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	if err := decodeLocalAuth(w, r, &in); err != nil {
		return err
	}
	if !a.allowLocalAuth(r, strings.ToLower(strings.TrimSpace(in.Username)), true) {
		return E(429, "RATE_LIMITED", "注册尝试过于频繁，请稍后再试")
	}
	username, err := normalizeLocalUsername(in.Username)
	if err != nil {
		return err
	}
	display, err := localDisplayName(in.DisplayName)
	if err != nil {
		return err
	}
	if err = validatePassword(in.Password); err != nil {
		return err
	}
	var open bool
	if err = a.Pool.QueryRow(r.Context(), "SELECT registration_enabled AND external_login_enabled FROM site_settings WHERE id=1").Scan(&open); err != nil {
		return err
	}
	if !open {
		return E(403, "REGISTRATION_CLOSED", "站点暂未开放读者注册")
	}
	encoded, err := hashPassword(r.Context(), in.Password)
	if err != nil {
		return err
	}
	token := RandID()
	err = a.identityMutation(r.Context(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(r.Context(), "SELECT registration_enabled AND external_login_enabled FROM site_settings WHERE id=1").Scan(&open); err != nil {
			return err
		}
		if !open {
			return E(403, "REGISTRATION_CLOSED", "站点暂未开放读者注册")
		}
		now := NowMillis()
		var id string
		if err := tx.QueryRow(r.Context(), "INSERT INTO users(username,display_name,role,created_at_ms,updated_at_ms,last_login_at_ms) VALUES($1,$2,'reader',$3,$3,$3) RETURNING id::text", username, display, now).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), "INSERT INTO local_accounts(user_id,password_hash,password_changed_at_ms,created_at_ms) VALUES($1,$2,$3,$3)", id, encoded, now); err != nil {
			return err
		}
		if err := localRegisteredEvent(r.Context(), tx, id, now); err != nil {
			return err
		}
		return a.insertLocalSession(r.Context(), tx, r, id, token, now)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return E(400, "REGISTRATION_FAILED", "无法完成注册，请检查信息或改用其他用户名")
	}
	if err != nil {
		return err
	}
	return a.localLoginResponse(w, r, token, 201)
}
func (a *App) localPasswordChange(w http.ResponseWriter, r *http.Request) error {
	pre, err := a.Require(r)
	if err != nil {
		return err
	}
	if err = a.checkCSRF(r); err != nil {
		return err
	}
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err = decodeLocalAuth(w, r, &in); err != nil {
		return err
	}
	if !a.allowLocalAuth(r, pre.Username, false) {
		return E(429, "RATE_LIMITED", "尝试过于频繁，请稍后再试")
	}
	if err = validatePassword(in.NewPassword); err != nil {
		return err
	}
	var previous string
	err = a.Pool.QueryRow(r.Context(), "SELECT password_hash FROM local_accounts WHERE user_id=$1", pre.ID).Scan(&previous)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	candidate := previous
	if candidate == "" {
		candidate = dummyPasswordHash
	}
	matched, err := verifyPassword(r.Context(), in.CurrentPassword, candidate)
	if err != nil {
		return err
	}
	if !matched || previous == "" {
		return invalidCredentials()
	}
	encoded, err := hashPassword(r.Context(), in.NewPassword)
	if err != nil {
		return err
	}
	*r = *r.WithContext(context.WithValue(r.Context(), updateActorKey, true))
	// Password replacement and its global session revocation must exclude all
	// ordinary writes before any actor/session authorization snapshot is taken.
	_, err = a.Write(r, nil, true, false, func(tx pgx.Tx, p Principal) (any, error) {
		var current string
		if err := tx.QueryRow(r.Context(), "SELECT password_hash FROM local_accounts WHERE user_id=$1", p.ID).Scan(&current); err != nil {
			return nil, err
		}
		if current != previous {
			return nil, invalidCredentials()
		}
		now := NowMillis()
		if _, err := tx.Exec(r.Context(), "UPDATE local_accounts SET password_hash=$2,password_changed_at_ms=$3 WHERE user_id=$1", p.ID, encoded, now); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(r.Context(), "UPDATE sessions SET revoked_at_ms=$2 WHERE user_id=$1 AND revoked_at_ms IS NULL", p.ID, now); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(r.Context(), "UPDATE users SET version=version+1,updated_at_ms=$2 WHERE id=$1", p.ID, now); err != nil {
			return nil, err
		}
		return nil, audit(r.Context(), tx, p, "user.update", "user", p.ID, "local password changed; all sessions revoked")
	})
	if err != nil {
		return err
	}
	a.cookie(w, sessionCookie, "", -1)
	w.WriteHeader(204)
	return nil
}
