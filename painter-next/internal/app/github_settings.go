package app

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

type githubSettingsQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// githubCredentials is server-only. Never encode it in an API response or log.
type githubCredentials struct {
	Version  int64
	Enabled  bool
	ClientID string
	Secret   string
	Source   string
}

func (g githubCredentials) fingerprint() string {
	return hashString("github-config:v1\x00" + g.Source + "\x00" + strconv.FormatInt(g.Version, 10) + "\x00" + strconv.FormatBool(g.Enabled) + "\x00" + g.ClientID + "\x00" + g.Secret)
}

func (a *App) effectiveGitHub(ctx context.Context, q githubSettingsQuery) (githubCredentials, error) {
	g := githubCredentials{Source: "database"}
	var encrypted []byte
	err := q.QueryRow(ctx, "SELECT version,enabled,client_id,client_secret_ciphertext FROM github_auth_settings WHERE id=1").Scan(&g.Version, &g.Enabled, &g.ClientID, &encrypted)
	if errors.Is(err, pgx.ErrNoRows) {
		return githubCredentials{Source: "environment", ClientID: a.Config.GitHubClientID, Secret: a.Config.GitHubClientSecret, Enabled: a.Config.GitHubClientID != "" && a.Config.GitHubClientSecret != ""}, nil
	}
	if err != nil {
		return g, err
	}
	if len(encrypted) > 0 {
		plain, err := a.decrypt(encrypted)
		if err != nil {
			return g, E(503, "OAUTH_CONFIG_UNAVAILABLE", "GitHub 登录密钥无法解密，请检查服务器应用加密密钥")
		}
		g.Secret = string(plain)
	}
	return g, nil
}

func (a *App) githubSettingsDTO(g githubCredentials) map[string]any {
	return map[string]any{"version": g.Version, "enabled": g.Enabled, "client_id": g.ClientID, "client_secret_configured": g.Secret != "", "source": g.Source, "callback_url": a.Config.PublicBaseURL + "/auth/github/callback"}
}

func (a *App) githubSettingsGet(w http.ResponseWriter, r *http.Request) error {
	if _, err := a.Require(r, "owner"); err != nil {
		return err
	}
	g, err := a.effectiveGitHub(r.Context(), a.Pool)
	if err != nil {
		return err
	}
	JSON(w, 200, a.githubSettingsDTO(g))
	return nil
}

func credentialTextValid(s string, max int) bool {
	return len(s) <= max && utf8.ValidString(s) && !strings.ContainsFunc(s, unicode.IsControl)
}

func (a *App) githubSettingsPut(w http.ResponseWriter, r *http.Request) error {
	// Authorize before decoding so non-owners cannot use this as a configuration
	// oracle. Write authorizes again under the exclusive policy barrier.
	if _, err := a.Require(r, "owner"); err != nil {
		return err
	}
	if err := a.checkCSRF(r); err != nil {
		return err
	}
	var in struct {
		Version      int64   `json:"version"`
		Enabled      bool    `json:"enabled"`
		ClientID     string  `json:"client_id"`
		ClientSecret *string `json:"client_secret"`
	}
	if err := decodeLocalAuth(w, r, &in); err != nil {
		return err
	}
	in.ClientID = strings.TrimSpace(in.ClientID)
	if in.Version < 0 || !credentialTextValid(in.ClientID, 256) || strings.ContainsFunc(in.ClientID, unicode.IsSpace) || (in.ClientSecret != nil && !credentialTextValid(*in.ClientSecret, 4096)) {
		return E(400, "VALIDATION_FAILED", "GitHub 应用配置格式无效")
	}
	value, err := a.Write(r, []string{"owner"}, true, false, func(tx pgx.Tx, p Principal) (any, error) {
		current, err := a.effectiveGitHub(r.Context(), tx)
		if err != nil {
			return nil, err
		}
		if current.Version != in.Version {
			return nil, E(409, "VERSION_CONFLICT", "GitHub 登录配置已变化，请重新加载后再保存")
		}
		secret := current.Secret
		if in.ClientSecret != nil {
			secret = *in.ClientSecret
		}
		if in.Enabled && (in.ClientID == "" || strings.TrimSpace(secret) == "") {
			return nil, E(400, "VALIDATION_FAILED", "启用 GitHub 登录需要 Client ID 和 Client Secret")
		}
		var encrypted []byte
		if secret != "" {
			encrypted, err = a.encrypt([]byte(secret))
			if err != nil {
				return nil, err
			}
		}
		g := githubCredentials{Version: current.Version + 1, Enabled: in.Enabled, ClientID: in.ClientID, Secret: secret, Source: "database"}
		_, err = tx.Exec(r.Context(), "INSERT INTO github_auth_settings(id,version,enabled,client_id,client_secret_ciphertext,updated_at_ms) VALUES(1,$1,$2,$3,$4,$5) ON CONFLICT(id) DO UPDATE SET version=EXCLUDED.version,enabled=EXCLUDED.enabled,client_id=EXCLUDED.client_id,client_secret_ciphertext=EXCLUDED.client_secret_ciphertext,updated_at_ms=EXCLUDED.updated_at_ms", g.Version, g.Enabled, g.ClientID, encrypted, NowMillis())
		if err != nil {
			return nil, err
		}
		if err := audit(r.Context(), tx, p, "site.settings_update", "site", "github-auth", "GitHub login configuration updated"); err != nil {
			return nil, err
		}
		return a.githubSettingsDTO(g), nil
	})
	if err != nil {
		return err
	}
	JSON(w, 200, value)
	return nil
}
