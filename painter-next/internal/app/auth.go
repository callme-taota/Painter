package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const sessionCookie = "painter_session"
const bindingCookie = "painter_oauth_binding"
const githubIssuer = "https://github.com"
const sessionLifetime = 30 * 24 * time.Hour

func (a *App) RegisterIdentity() {
	a.Handle("GET /api/v1/auth/options", a.localAuthOptions)
	a.Handle("POST /api/v1/auth/login", a.localLogin)
	a.Handle("POST /api/v1/auth/register", a.localRegister)
	a.Handle("POST /api/v1/me/password", a.localPasswordChange)
	a.Handle("GET /api/v1/admin/auth/github", a.githubSettingsGet)
	a.Handle("PUT /api/v1/admin/auth/github", a.githubSettingsPut)
	a.Handle("GET /auth/github/start", a.oauthStart)
	a.Handle("GET /auth/github/callback", a.oauthCallback)
	a.Handle("GET /api/v1/session", a.sessionGet)
	a.Handle("DELETE /api/v1/session", a.sessionDelete)
	a.Handle("GET /api/v1/me/sessions", a.sessionsGet)
	a.Handle("DELETE /api/v1/me/sessions/{id}", a.sessionRevoke)
	a.Handle("GET /api/v1/me", a.meGet)
	a.Handle("PATCH /api/v1/me", a.mePatch)
	a.Handle("GET /api/v1/users/{id}", a.userGet)
}
func (a *App) cookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: age, HttpOnly: true, Secure: strings.HasPrefix(a.Config.PublicBaseURL, "https://"), SameSite: http.SameSiteLaxMode})
}
func (a *App) Principal(r *http.Request) (Principal, error) {
	c, e := r.Cookie(sessionCookie)
	if e != nil || len(c.Value) != 48 {
		return Principal{}, E(401, "UNAUTHENTICATED", "请先登录")
	}
	var p Principal
	e = a.Pool.QueryRow(r.Context(), "SELECT u.id::text,u.role,u.username,u.display_name,s.management_id FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at_ms IS NULL AND s.expires_at_ms>$2 AND u.disabled_at_ms IS NULL", hashString(c.Value), NowMillis()).Scan(&p.ID, &p.Role, &p.Username, &p.DisplayName, &p.SessionID)
	if errors.Is(e, pgx.ErrNoRows) {
		return p, E(401, "UNAUTHENTICATED", "登录已过期")
	}
	return p, e
}
func (a *App) txPrincipal(ctx context.Context, tx pgx.Tx, r *http.Request, lock string) (Principal, error) {
	c, e := r.Cookie(sessionCookie)
	if e != nil {
		return Principal{}, E(401, "UNAUTHENTICATED", "请先登录")
	}
	var p Principal
	var csrfHash string
	e = tx.QueryRow(ctx, "SELECT u.id::text,u.role,u.username,u.display_name,s.management_id,s.csrf_secret_hash FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.revoked_at_ms IS NULL AND s.expires_at_ms>$2 AND u.disabled_at_ms IS NULL FOR "+lock+" OF u", hashString(c.Value), NowMillis()).Scan(&p.ID, &p.Role, &p.Username, &p.DisplayName, &p.SessionID, &csrfHash)
	if errors.Is(e, pgx.ErrNoRows) {
		return p, E(401, "UNAUTHENTICATED", "登录已过期")
	}
	if e != nil {
		return p, e
	}
	// A locking JOIN can wait for an updated users row while PostgreSQL keeps
	// the joined sessions row from the statement's old snapshot. Re-read session
	// validity in a new READ COMMITTED statement after the user lock is acquired.
	// Role/disable and ordinary user updates remain protected by that user lock.
	e = tx.QueryRow(ctx, "SELECT management_id,csrf_secret_hash FROM sessions WHERE token_hash=$1 AND user_id=$2 AND revoked_at_ms IS NULL AND expires_at_ms>$3", hashString(c.Value), p.ID, NowMillis()).Scan(&p.SessionID, &csrfHash)
	if errors.Is(e, pgx.ErrNoRows) {
		return Principal{}, E(401, "UNAUTHENTICATED", "登录已过期")
	}
	if e != nil {
		return Principal{}, e
	}
	if csrfHash != hashString(a.csrf(c.Value)) {
		return p, E(403, "CSRF_FAILED", "CSRF校验失败")
	}
	return p, nil
}
func (a *App) encrypt(plain []byte) ([]byte, error) {
	block, e := aes.NewCipher(a.Config.EncryptionKey)
	if e != nil {
		return nil, e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, e
	}
	return g.Seal(nonce, nonce, plain, []byte("painter:v1")), nil
}
func (a *App) decrypt(raw []byte) ([]byte, error) {
	block, e := aes.NewCipher(a.Config.EncryptionKey)
	if e != nil {
		return nil, e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	if len(raw) < g.NonceSize() {
		return nil, errors.New("invalid ciphertext")
	}
	return g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], []byte("painter:v1"))
}
func internalReturn(s string) string {
	if s == "" {
		return "/studio"
	}
	if len(s) > 2048 {
		return "/studio"
	}
	u, e := url.Parse(s)
	if e != nil || !strings.HasPrefix(s, "/") || strings.HasPrefix(s, "//") || strings.ContainsAny(s, "\\\r\n") || u.IsAbs() || u.Host != "" || strings.HasPrefix(u.Path, "//") || strings.ContainsAny(u.Path, "\\\r\n") || strings.HasPrefix(u.Path, "/auth/") {
		return "/studio"
	}
	return u.RequestURI()
}
func (a *App) oauthStart(w http.ResponseWriter, r *http.Request) error {
	var credentials githubCredentials
	state := RandID()
	binding := RandID()
	if c, e := r.Cookie(bindingCookie); e == nil && len(c.Value) == 48 {
		binding = c.Value
	}
	verifier := RandID()
	digest := sha256.Sum256([]byte(verifier))
	encrypted, e := a.encrypt([]byte(verifier))
	if e != nil {
		return e
	}
	now := NowMillis()
	e = a.identityMutation(r.Context(), func(tx pgx.Tx) error {
		credentials, e = a.effectiveGitHub(r.Context(), tx)
		if e != nil {
			return e
		}
		if !credentials.Enabled {
			return E(503, "OAUTH_NOT_CONFIGURED", "尚未启用 GitHub 登录")
		}
		_, e = tx.Exec(r.Context(), "INSERT INTO oauth_states(state_hash,browser_binding_hash,provider,issuer,flow,pkce_verifier_ciphertext,return_path,created_at_ms,expires_at_ms,provider_config_hash) VALUES($1,$2,'github',$3,'login',$4,$5,$6,$7,$8)", hashString(state), hashString(binding), githubIssuer, encrypted, internalReturn(r.URL.Query().Get("return_to")), now, now+10*60*1000, credentials.fingerprint())
		return e
	})
	if e != nil {
		return e
	}
	a.cookie(w, bindingCookie, binding, 600)
	u, e := url.Parse(a.Config.OAuthURL)
	if e != nil {
		return e
	}
	q := u.Query()
	q.Set("client_id", credentials.ClientID)
	q.Set("redirect_uri", a.Config.PublicBaseURL+"/auth/github/callback")
	q.Set("state", state)
	q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(digest[:]))
	q.Set("code_challenge_method", "S256")
	q.Set("allow_signup", "false")
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), 302)
	return nil
}

type githubProfile struct {
	ID        json.Number `json:"id"`
	Login     string      `json:"login"`
	Name      string      `json:"name"`
	AvatarURL string      `json:"avatar_url"`
	Type      string      `json:"type"`
}

func (a *App) oauthCallback(w http.ResponseWriter, r *http.Request) error {
	binding, e := r.Cookie(bindingCookie)
	if e != nil || len(r.URL.Query().Get("state")) != 48 {
		return E(400, "OAUTH_STATE_INVALID", "登录状态无效或已过期")
	}
	var encrypted []byte
	var path string
	var credentials githubCredentials
	now := NowMillis()
	// Consume before calling the network. Failed exchanges require a fresh flow;
	// no concurrent callback can exchange this state a second time.
	e = a.identityMutation(r.Context(), func(tx pgx.Tx) error {
		credentials, e = a.effectiveGitHub(r.Context(), tx)
		if e != nil {
			return e
		}
		if !credentials.Enabled {
			return E(400, "OAUTH_CONFIG_CHANGED", "GitHub 登录配置已变化，请重新开始登录")
		}
		e = tx.QueryRow(r.Context(), "UPDATE oauth_states SET consumed_at_ms=$1 WHERE state_hash=$2 AND browser_binding_hash=$3 AND provider='github' AND issuer=$4 AND flow='login' AND consumed_at_ms IS NULL AND expires_at_ms>$1 AND provider_config_hash=$5 RETURNING pkce_verifier_ciphertext,return_path", now, hashString(r.URL.Query().Get("state")), hashString(binding.Value), githubIssuer, credentials.fingerprint()).Scan(&encrypted, &path)
		return e
	})
	if errors.Is(e, pgx.ErrNoRows) {
		return E(400, "OAUTH_STATE_INVALID", "登录状态无效或已使用")
	}
	if e != nil {
		return e
	}
	if r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
		return E(400, "OAUTH_DENIED", "GitHub 授权未完成")
	}
	verifier, e := a.decrypt(encrypted)
	if e != nil {
		return E(400, "OAUTH_STATE_INVALID", "登录状态不可解密，请重新登录")
	}
	profile, e := a.githubIdentity(r.Context(), r.URL.Query().Get("code"), string(verifier), credentials)
	if e != nil {
		return e
	}
	token, e := a.completeGitHubLogin(r.Context(), profile, r, credentials.fingerprint())
	if e != nil {
		return e
	}
	a.cookie(w, sessionCookie, token, int(sessionLifetime.Seconds()))
	http.Redirect(w, r, internalReturn(path), 303)
	return nil
}
func (a *App) githubIdentity(ctx context.Context, code, verifier string, credentials githubCredentials) (githubProfile, error) {
	var profile githubProfile
	form := url.Values{"client_id": {credentials.ClientID}, "client_secret": {credentials.Secret}, "redirect_uri": {a.Config.PublicBaseURL + "/auth/github/callback"}, "code": {code}, "code_verifier": {verifier}}
	req, e := http.NewRequestWithContext(ctx, "POST", a.Config.TokenURL, strings.NewReader(form.Encode()))
	if e != nil {
		return profile, e
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, e := a.Config.HTTPClient.Do(req)
	if e != nil {
		return profile, E(502, "OAUTH_PROVIDER_ERROR", "无法连接 GitHub，请重新登录")
	}
	defer resp.Body.Close()
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&token) != nil || token.AccessToken == "" || token.Error != "" || !strings.EqualFold(token.TokenType, "bearer") {
		return profile, E(502, "OAUTH_PROVIDER_ERROR", "GitHub 授权交换失败")
	}
	req, e = http.NewRequestWithContext(ctx, "GET", a.Config.ProfileURL, nil)
	if e != nil {
		return profile, e
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("User-Agent", "Painter-Next")
	resp2, e := a.Config.HTTPClient.Do(req)
	if e != nil {
		return profile, E(502, "OAUTH_PROVIDER_ERROR", "无法读取 GitHub 身份")
	}
	defer resp2.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(resp2.Body, 64<<10))
	decoder.UseNumber()
	if resp2.StatusCode != 200 || decoder.Decode(&profile) != nil {
		return profile, E(502, "OAUTH_PROVIDER_ERROR", "GitHub 身份读取失败")
	}
	id, e := strconv.ParseInt(string(profile.ID), 10, 64)
	if e != nil || id <= 0 || profile.Login == "" || (profile.Type != "" && profile.Type != "User") {
		return profile, E(502, "OAUTH_PROVIDER_ERROR", "GitHub 身份无效")
	}
	return profile, nil
	// Provider token and optional refresh token are intentionally not persisted.
}
func (a *App) completeLogin(ctx context.Context, g githubProfile) (string, error) {
	return a.completeGitHubLogin(ctx, g, nil, "")
}

func (a *App) completeGitHubLogin(ctx context.Context, g githubProfile, r *http.Request, expectedConfig string) (string, error) {
	tx, e := a.Pool.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return "", e
	}
	defer tx.Rollback(context.Background())
	subject := string(g.ID)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "identity/github/"+subject); e != nil {
		return "", e
	}
	// Exclusive policy barrier serializes membership creation and first-owner bootstrap.
	if _, e = tx.Exec(ctx, "SELECT id FROM policy_barrier WHERE id=1 FOR UPDATE"); e != nil {
		return "", e
	}
	// Configuration changes may race the network exchange. Recheck after the
	// policy lock so a disabled/replaced provider cannot establish a session.
	if expectedConfig != "" {
		credentials, err := a.effectiveGitHub(ctx, tx)
		if err != nil {
			return "", err
		}
		if !credentials.Enabled || credentials.fingerprint() != expectedConfig {
			return "", E(400, "OAUTH_CONFIG_CHANGED", "GitHub 登录配置已变化，请重新开始登录")
		}
	}
	var id string
	var disabled *int64
	e = tx.QueryRow(ctx, "SELECT u.id::text,u.disabled_at_ms FROM oauth_accounts o JOIN users u ON u.id=o.user_id WHERE o.provider='github' AND o.issuer=$1 AND o.subject=$2 FOR UPDATE OF u", githubIssuer, subject).Scan(&id, &disabled)
	now := NowMillis()
	if e == pgx.ErrNoRows {
		var enabled bool
		var owners int
		e = tx.QueryRow(ctx, "SELECT registration_enabled,(SELECT count(*) FROM users WHERE role='owner' AND disabled_at_ms IS NULL) FROM site_settings WHERE id=1").Scan(&enabled, &owners)
		if e != nil {
			return "", e
		}
		role := "reader"
		if owners == 0 && a.Config.BootstrapGitHubID == subject {
			role = "owner"
		} else if !enabled {
			return "", E(403, "REGISTRATION_CLOSED", "站点暂未开放读者注册")
		}
		login := strings.ToLower(g.Login)
		if len(login) > 60 {
			login = login[:60]
		}
		display := strings.TrimSpace(g.Name)
		if display == "" {
			display = g.Login
		}
		if len([]rune(display)) > 100 {
			display = string([]rune(display)[:100])
		}
		avatar := ""
		if u, err := url.Parse(g.AvatarURL); err == nil && u.Scheme == "https" && u.Host != "" {
			avatar = u.String()
		}
		for attempt := 0; attempt < 3; attempt++ {
			name := login
			if attempt == 1 {
				name = login + "-" + subject
			}
			if attempt == 2 {
				name = "github-" + subject + "-" + RandID()[:8]
			}
			e = tx.QueryRow(ctx, "INSERT INTO users(username,display_name,avatar_url,role,created_at_ms,updated_at_ms,last_login_at_ms) VALUES($1,$2,$3,$4,$5,$5,$5) ON CONFLICT DO NOTHING RETURNING id::text", name, display, avatar, role, now).Scan(&id)
			if e == nil {
				break
			}
			if e != pgx.ErrNoRows {
				return "", e
			}
		}
		if e != nil {
			return "", e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO oauth_accounts(provider,issuer,subject,user_id,provider_login,created_at_ms,updated_at_ms) VALUES('github',$1,$2,$3,$4,$5,$5)", githubIssuer, subject, id, g.Login, now); e != nil {
			return "", e
		}
		eventID := RandID()
		envelope, _ := json.Marshal(map[string]any{"event_id": eventID, "schema_version": 1, "event_type": "user.registered.v1", "aggregate_type": "user", "aggregate_id": id, "aggregate_version": 1, "actor_id": id, "occurred_at_ms": now, "payload": map[string]any{"provider": "github"}})
		if _, e = tx.Exec(ctx, "INSERT INTO outbox(id,topic,aggregate_id,dedupe_key,payload_json,available_at_ms,created_at_ms) VALUES($1,'user.registered.v1',$2,$3,$4,$5,$5)", eventID, id, "user.registered/"+id, envelope, now); e != nil {
			return "", e
		}
		if role == "owner" {
			if e = audit(ctx, tx, Principal{ID: id}, "system.bootstrap", "system", "owner", "configured GitHub subject"); e != nil {
				return "", e
			}
		}
	} else if e != nil {
		return "", e
	} else {
		if disabled != nil {
			return "", E(403, "ACCOUNT_DISABLED", "账号已停用")
		}
		if a.Config.BootstrapGitHubID == subject {
			var owners int
			if e = tx.QueryRow(ctx, "SELECT count(*) FROM users WHERE role='owner' AND disabled_at_ms IS NULL").Scan(&owners); e != nil {
				return "", e
			}
			if owners == 0 {
				if _, e = tx.Exec(ctx, "UPDATE users SET role='owner',updated_at_ms=$2 WHERE id=$1", id, now); e != nil {
					return "", e
				}
				if e = audit(ctx, tx, Principal{ID: id}, "system.bootstrap", "system", "owner", "configured existing GitHub subject"); e != nil {
					return "", e
				}
			}
		}
		if _, e = tx.Exec(ctx, "UPDATE users SET last_login_at_ms=$2 WHERE id=$1", id, now); e != nil {
			return "", e
		}
		if _, e = tx.Exec(ctx, "UPDATE oauth_accounts SET provider_login=$3,updated_at_ms=$4 WHERE provider='github' AND issuer=$1 AND subject=$2", githubIssuer, subject, g.Login, now); e != nil {
			return "", e
		}
	}
	token := RandID()
	if r != nil {
		if old, err := r.Cookie(sessionCookie); err == nil {
			if _, e = tx.Exec(ctx, "UPDATE sessions SET revoked_at_ms=$2 WHERE token_hash=$1 AND revoked_at_ms IS NULL", hashString(old.Value), now); e != nil {
				return "", e
			}
		}
	}
	_, e = tx.Exec(ctx, "INSERT INTO sessions(token_hash,management_id,user_id,csrf_secret_hash,created_at_ms,expires_at_ms,last_seen_at_ms) VALUES($1,$2,$3,$4,$5,$6,$5)", hashString(token), RandID(), id, hashString(a.csrf(token)), now, now+sessionLifetime.Milliseconds())
	if e != nil {
		return "", e
	}
	var role string
	if e = tx.QueryRow(ctx, "SELECT role FROM users WHERE id=$1", id).Scan(&role); e != nil {
		return "", e
	}
	if role == "owner" {
		if e = markInstallation(ctx, tx, id, now); e != nil {
			return "", e
		}
	}
	return token, tx.Commit(ctx)
}
func (a *App) sessionGet(w http.ResponseWriter, r *http.Request) error {
	return a.writeSession(w, r, 200)
}
func (a *App) writeSession(w http.ResponseWriter, r *http.Request, status int) error {
	p, e := a.Principal(r)
	if e != nil {
		var api *APIError
		if errors.As(e, &api) && api.Status == 401 {
			JSON(w, 200, map[string]any{"authenticated": false})
			return nil
		}
		return e
	}
	user, e := a.userDTO(r.Context(), p.ID, true)
	if e != nil {
		return e
	}
	delete(user, "email")
	delete(user, "phone")
	var expires int64
	if e = a.Pool.QueryRow(r.Context(), "SELECT expires_at_ms FROM sessions WHERE management_id=$1", p.SessionID).Scan(&expires); e != nil {
		return e
	}
	c, _ := r.Cookie(sessionCookie)
	JSON(w, status, map[string]any{"admin_tools_version": 1, "authenticated": true, "user": user, "csrf_token": a.csrf(c.Value), "expires_at": TimeString(expires), "capabilities": map[string]bool{"manage_site": p.Role == "owner", "write_posts": p.Role == "owner" || p.Role == "author", "interact": true}})
	return nil
}
func (a *App) sessionDelete(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Principal(r); e != nil {
		var api *APIError
		if errors.As(e, &api) && api.Status == 401 {
			if r.Header.Get("Origin") != a.Config.PublicBaseURL {
				return E(403, "ORIGIN_REJECTED", "请求来源无效")
			}
			a.cookie(w, sessionCookie, "", -1)
			w.WriteHeader(204)
			return nil
		}
		return e
	}
	_, e := a.Write(r, nil, true, false, func(tx pgx.Tx, p Principal) (any, error) {
		_, e := tx.Exec(r.Context(), "UPDATE sessions SET revoked_at_ms=$2 WHERE management_id=$1", p.SessionID, NowMillis())
		return nil, e
	})
	if e != nil {
		return e
	}
	a.cookie(w, sessionCookie, "", -1)
	w.WriteHeader(204)
	return nil
}
func (a *App) sessionsGet(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r)
	if e != nil {
		return e
	}
	rows, e := a.Pool.Query(r.Context(), "SELECT management_id,created_at_ms,last_seen_at_ms,expires_at_ms FROM sessions WHERE user_id=$1 AND revoked_at_ms IS NULL AND expires_at_ms>$2 ORDER BY created_at_ms DESC LIMIT 100", p.ID, NowMillis())
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		var id string
		var created, seen, expires int64
		if e = rows.Scan(&id, &created, &seen, &expires); e != nil {
			return e
		}
		items = append(items, map[string]any{"id": id, "management_id": id, "created_at": TimeString(created), "last_seen_at": TimeString(seen), "expires_at": TimeString(expires), "current": id == p.SessionID})
	}
	if e = rows.Err(); e != nil {
		return e
	}
	JSON(w, 200, map[string]any{"items": items, "next_cursor": nil, "has_more": false})
	return nil
}
func (a *App) sessionRevoke(w http.ResponseWriter, r *http.Request) error {
	current := false
	_, e := a.Write(r, nil, true, false, func(tx pgx.Tx, p Principal) (any, error) {
		tag, e := tx.Exec(r.Context(), "UPDATE sessions SET revoked_at_ms=COALESCE(revoked_at_ms,$3) WHERE management_id=$1 AND user_id=$2", r.PathValue("id"), p.ID, NowMillis())
		if e != nil {
			return nil, e
		}
		if tag.RowsAffected() == 0 {
			return nil, NotFound()
		}
		current = r.PathValue("id") == p.SessionID
		return nil, audit(r.Context(), tx, p, "session.revoke", "session", r.PathValue("id"), "")
	})
	if e != nil {
		return e
	}
	if current {
		a.cookie(w, sessionCookie, "", -1)
	}
	w.WriteHeader(204)
	return nil
}

func (a *App) identityMutation(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, "SELECT id FROM policy_barrier WHERE id=1 FOR SHARE"); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// PruneIdentity is a bounded periodic housekeeping command, called by the worker.
// Expired OAuth state and dedupe records never grant access after expiry.
func (a *App) PruneIdentity(ctx context.Context) error {
	return a.identityMutation(ctx, func(tx pgx.Tx) error {
		now := NowMillis()
		for _, statement := range []struct {
			SQL    string
			Cutoff int64
		}{{"DELETE FROM oauth_states WHERE state_hash IN(SELECT state_hash FROM oauth_states WHERE expires_at_ms<$1 LIMIT 5000)", now - 60*60*1000}, {"DELETE FROM sessions WHERE token_hash IN(SELECT token_hash FROM sessions WHERE COALESCE(revoked_at_ms,expires_at_ms)<$1 LIMIT 5000)", now - 30*24*60*60*1000}, {"DELETE FROM idempotency_keys WHERE (actor_user_id,operation,idempotency_key) IN(SELECT actor_user_id,operation,idempotency_key FROM idempotency_keys WHERE expires_at_ms<$1 LIMIT 5000)", now}} {
			if _, e := tx.Exec(ctx, statement.SQL, statement.Cutoff); e != nil {
				return e
			}
		}
		return nil
	})
}
