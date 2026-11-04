package app

import (
	"context"
	"crypto/hmac"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const setupNonceCookie = "painter_setup_nonce"

// InstallationToken is deliberately CLI-only: the web status never exposes it.
func (a *App) InstallationToken(ctx context.Context) (string, error) {
	installed, e := a.installationComplete(ctx, a.Pool)
	if e != nil {
		return "", e
	}
	if installed {
		return "", errors.New("installation is complete; setup token is permanently disabled")
	}
	if a.Config.SetupToken == "" {
		return "", errors.New("SETUP_TOKEN is not configured; generate a private random token and restart the app")
	}
	return a.Config.SetupToken, nil
}

func (a *App) installationComplete(ctx context.Context, db contentQuerier) (bool, error) {
	var installed bool
	e := db.QueryRow(ctx, `SELECT completed_at_ms IS NOT NULL OR EXISTS(SELECT 1 FROM users WHERE role='owner') OR EXISTS(SELECT 1 FROM local_accounts) OR EXISTS(SELECT 1 FROM oauth_accounts) FROM installation_state WHERE id=1`).Scan(&installed)
	return installed, e
}

// The caller already holds the exclusive policy/maintenance barrier. Completion
// is a one-way database transition, shared by web and offline owner bootstrap.
func markInstallation(ctx context.Context, tx pgx.Tx, owner string, now int64) error {
	_, e := tx.Exec(ctx, `UPDATE installation_state SET completed_at_ms=$1,completed_by=$2 WHERE id=1 AND completed_at_ms IS NULL`, now, owner)
	return e
}

func (a *App) RegisterSetup() {
	a.Handle("GET /api/v1/setup/status", a.setupStatus)
	a.Handle("POST /api/v1/setup/complete", a.setupComplete)
}

func (a *App) setupStatus(w http.ResponseWriter, r *http.Request) error {
	installed, e := a.installationComplete(r.Context(), a.Pool)
	if e != nil {
		return e
	}
	var version int
	if e = a.Pool.QueryRow(r.Context(), "SELECT max(version) FROM schema_migrations").Scan(&version); e != nil {
		return e
	}
	available := !installed && a.Config.SetupToken != ""
	csrf := ""
	if available {
		nonce := RandID()
		if c, err := r.Cookie(setupNonceCookie); err == nil && len(c.Value) == 48 {
			nonce = c.Value
		}
		a.cookie(w, setupNonceCookie, nonce, 600)
		csrf = macString(a.Config.EncryptionKey, "setup:v1:"+nonce)
	}
	JSON(w, 200, map[string]any{"available": available, "installed": installed, "csrf_token": csrf, "postgres_ready": true, "schema_version": version, "public_base_url": a.Config.PublicBaseURL, "github_configured": a.Config.GitHubClientID != "" && a.Config.GitHubClientSecret != ""})
	return nil
}

func (a *App) setupComplete(w http.ResponseWriter, r *http.Request) error {
	if r.Header.Get("Origin") != a.Config.PublicBaseURL {
		return E(403, "ORIGIN_REJECTED", "请求来源无效")
	}
	installed, e := a.installationComplete(r.Context(), a.Pool)
	if e != nil {
		return e
	}
	if installed {
		return E(409, "SETUP_COMPLETED", "网站已完成安装，请使用已有管理员账号登录")
	}
	if a.Config.SetupToken == "" {
		return E(403, "SETUP_DISABLED", "请先在服务器配置安装码并重启服务")
	}
	nonce, e := r.Cookie(setupNonceCookie)
	if e != nil || len(nonce.Value) != 48 || !hmac.Equal([]byte(r.Header.Get("X-CSRF-Token")), []byte(macString(a.Config.EncryptionKey, "setup:v1:"+nonce.Value))) {
		return E(403, "CSRF_FAILED", "安装校验已过期，请刷新后重试")
	}
	var in struct {
		Token    string `json:"setup_token"`
		SiteName string `json:"site_name"`
		Tagline  string `json:"tagline"`
		Username string `json:"admin_username"`
		Display  string `json:"admin_display_name"`
		Password string `json:"password"`
	}
	if e = decodeLocalAuth(w, r, &in); e != nil {
		return e
	}
	if len(in.Token) > 256 || !hmac.Equal([]byte(hashString(in.Token)), []byte(hashString(a.Config.SetupToken))) {
		return E(403, "SETUP_TOKEN_INVALID", "安装码不正确，请使用服务器生成的安装码")
	}
	in.SiteName = strings.TrimSpace(in.SiteName)
	in.Tagline = strings.TrimSpace(in.Tagline)
	if !utf8.ValidString(in.SiteName) || strings.ContainsRune(in.SiteName, '\x00') || utf8.RuneCountInString(in.SiteName) < 1 || utf8.RuneCountInString(in.SiteName) > 100 || !utf8.ValidString(in.Tagline) || strings.ContainsRune(in.Tagline, '\x00') || utf8.RuneCountInString(in.Tagline) > 300 {
		return E(400, "VALIDATION_FAILED", "站点名称须为1至100字，简介最多300字")
	}
	if in.Username, e = normalizeLocalUsername(in.Username); e != nil {
		return e
	}
	if in.Display, e = localDisplayName(in.Display); e != nil {
		return e
	}
	encoded, e := hashPassword(r.Context(), in.Password)
	if e != nil {
		return e
	}
	// Serializes the one-time operation with offline creation, OAuth bootstrap,
	// ordinary account writes, imports and backup. No locks are held during Argon2.
	tx, e := a.maintenance(r.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if installed, e = a.installationComplete(r.Context(), tx); e != nil {
		return e
	}
	if installed {
		return E(409, "SETUP_COMPLETED", "网站已完成安装，请使用已有管理员账号登录")
	}
	now := NowMillis()
	var id string
	if e = tx.QueryRow(r.Context(), `INSERT INTO users(username,display_name,role,created_at_ms,updated_at_ms) VALUES($1,$2,'owner',$3,$3) RETURNING id::text`, in.Username, in.Display, now).Scan(&id); e != nil {
		return e
	}
	if _, e = tx.Exec(r.Context(), `INSERT INTO local_accounts(user_id,password_hash,password_changed_at_ms,created_at_ms) VALUES($1,$2,$3,$3)`, id, encoded, now); e != nil {
		return e
	}
	if _, e = tx.Exec(r.Context(), `UPDATE site_profile SET display_name=$1,tagline=$2,intro_name=$3,version=version+1,updated_at_ms=$4 WHERE id=1`, in.SiteName, in.Tagline, in.Display, now); e != nil {
		return e
	}
	if _, e = tx.Exec(r.Context(), `UPDATE site_settings SET registration_enabled=false,version=version+1,updated_at_ms=$1 WHERE id=1`, now); e != nil {
		return e
	}
	if e = localRegisteredEvent(r.Context(), tx, id, now); e != nil {
		return e
	}
	if e = markInstallation(r.Context(), tx, id, now); e != nil {
		return e
	}
	if e = audit(r.Context(), tx, Principal{ID: id}, "system.bootstrap", "system", "installation", "web installation; first owner and site created atomically"); e != nil {
		return e
	}
	if e = tx.Commit(r.Context()); e != nil {
		return e
	}
	a.cookie(w, setupNonceCookie, "", -1)
	JSON(w, 201, map[string]any{"completed": true, "login_url": "/login?installed=1"})
	return nil
}
