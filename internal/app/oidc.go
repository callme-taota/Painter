package app

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"painter-next/internal/identity"
	"painter-next/web"
)

// RegisterOIDC embeds protocol endpoints without giving OAuth tokens access to
// Painter administration APIs. The only trusted issuer is PublicBaseURL, never a
// Host or Forwarded header supplied by an HTTP client or a tunnel.
func (a *App) RegisterOIDC(ctx context.Context) error {
	store := identity.New(a.Pool, identity.Hooks{Encrypt: a.encrypt, Decrypt: a.decrypt, SessionAllowed: a.oidcSessionAllowed})
	if err := store.EnsureSigningKey(ctx); err != nil {
		return err
	}
	var cryptoKey [32]byte
	copy(cryptoKey[:], a.Config.EncryptionKey)
	opts := []op.Option{op.WithCORSOptions(nil)}
	// Only explicit development environments may run a local HTTP issuer. The
	// production tunnel always publishes HTTPS even when its upstream uses HTTP.
	if a.Config.Environment != "production" && strings.HasPrefix(a.Config.PublicBaseURL, "http://") {
		opts = append(opts, op.WithAllowInsecure())
	}
	provider, err := op.NewProvider(&op.Config{CryptoKey: cryptoKey, CodeMethodS256: true, GrantTypeRefreshToken: true, SupportedScopes: []string{"openid", "profile", "offline_access"}, SupportedClaims: []string{"sub", "iss", "aud", "iat", "exp", "auth_time", "nonce", "amr", "acr", "name", "preferred_username"}, DefaultLogoutRedirectURI: a.Config.PublicBaseURL}, store, op.StaticIssuer(a.Config.PublicBaseURL+"/oidc"), opts...)
	if err != nil {
		return err
	}
	protocolHandler := a.oidcProtocolHandler(store, http.StripPrefix("/oidc", provider))
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodOptions} {
		a.Mux.Handle(method+" /oidc/", protocolHandler)
	}
	a.Handle("GET /oidc/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) error {
		discovery := op.CreateDiscoveryConfig(op.ContextWithIssuer(r.Context(), a.Config.PublicBaseURL+"/oidc"), provider, store)
		discovery.ResponseTypesSupported = []string{"code"}
		discovery.ResponseModesSupported = []string{"query"}
		discovery.GrantTypesSupported = []oidc.GrantType{oidc.GrantTypeCode, oidc.GrantTypeRefreshToken}
		discovery.CodeChallengeMethodsSupported = []oidc.CodeChallengeMethod{oidc.CodeChallengeMethodS256}
		discovery.IntrospectionEndpointAuthMethodsSupported = []oidc.AuthMethod{oidc.AuthMethodBasic}
		discovery.IntrospectionEndpointAuthSigningAlgValuesSupported = nil
		discovery.RevocationEndpointAuthMethodsSupported = []oidc.AuthMethod{oidc.AuthMethodNone, oidc.AuthMethodBasic}
		discovery.RevocationEndpointAuthSigningAlgValuesSupported = nil
		discovery.DeviceAuthorizationEndpoint = ""
		w.Header().Set("Cache-Control", "public, max-age=300")
		JSON(w, 200, discovery)
		return nil
	})
	a.Handle("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) error {
		http.Redirect(w, r, "/oidc/.well-known/openid-configuration", http.StatusTemporaryRedirect)
		return nil
	})
	a.Handle("GET /oidc/consent", func(w http.ResponseWriter, r *http.Request) error { return a.oidcConsent(store, w, r) })
	a.Handle("POST /oidc/consent", func(w http.ResponseWriter, r *http.Request) error { return a.oidcConsentPost(store, w, r) })
	a.Handle("GET /api/v1/admin/identity/clients", a.identityClientsGet)
	a.Handle("POST /api/v1/admin/identity/clients", a.identityClientsCreate)
	a.Handle("DELETE /api/v1/admin/identity/clients/{id}", a.identityClientsDisable)
	a.Handle("POST /api/v1/admin/identity/clients/{id}/rotate-secret", a.identityClientsRotate)
	return nil
}

func (a *App) oidcSessionAllowed(ctx context.Context, q identity.Queryer, userID, sessionID string, mfaAt *time.Time) error {
	if err := identitySessionAllowed(ctx, q, userID, sessionID); err != nil {
		return err
	}
	var enabledAt *int64
	if err := q.QueryRow(ctx, "SELECT (SELECT enabled_at_ms FROM user_mfa WHERE user_id=$1)", userID).Scan(&enabledAt); err != nil {
		return err
	}
	if enabledAt != nil && (mfaAt == nil || mfaAt.UnixMilli() < *enabledAt) {
		return E(401, "UNAUTHENTICATED", "账号安全设置已更新，请重新授权登录")
	}
	return nil
}

func (a *App) oidcProtocolHandler(store *identity.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		// Token requests are form data, bounded before the library parses them.
		r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
		path := strings.TrimPrefix(r.URL.Path, "/oidc")
		if !a.allowOIDCRequest(r, path) {
			Error(w, E(429, "RATE_LIMITED", "登录请求过于频繁，请稍后再试"))
			return
		}
		if path == "/oauth/token" || path == "/revoke" || path == "/oauth/introspect" {
			if r.Method != "POST" && r.Method != "OPTIONS" {
				w.Header().Set("Allow", "POST, OPTIONS")
				w.WriteHeader(405)
				return
			}
			if r.Method == "POST" {
				if err := r.ParseForm(); err != nil {
					JSON(w, 400, map[string]string{"error": "invalid_request"})
					return
				}
				if path == "/oauth/token" && !slices.Contains([]string{"authorization_code", "refresh_token"}, r.PostForm.Get("grant_type")) {
					JSON(w, 400, map[string]string{"error": "unsupported_grant_type"})
					return
				}
				if r.PostForm.Get("client_assertion") != "" || r.PostForm.Get("client_assertion_type") != "" {
					JSON(w, 400, map[string]string{"error": "invalid_client"})
					return
				}
				id, _, basic := r.BasicAuth()
				if !basic {
					id = r.PostForm.Get("client_id")
				}
				client, err := store.GetClient(r.Context(), id)
				if err != nil || client.Kind == "web" && !basic {
					JSON(w, 401, map[string]string{"error": "invalid_client"})
					return
				}
			}
		}
		// Cross-origin XHR is available only to SPA redirect origins which the owner
		// registered. Cookies are never allowed cross-origin; browser login is a top
		// level redirect. The library's permissive default CORS policy is disabled.
		if origin := r.Header.Get("Origin"); origin != "" && origin != a.Config.PublicBaseURL {
			allowed := false
			rows, err := a.Pool.Query(r.Context(), "SELECT redirect_uris FROM identity_clients WHERE kind='spa' AND disabled_at_ms IS NULL")
			if err == nil {
				for rows.Next() {
					var raw []byte
					if rows.Scan(&raw) != nil {
						continue
					}
					var redirects []string
					if json.Unmarshal(raw, &redirects) != nil {
						continue
					}
					for _, rawURI := range redirects {
						u, e := url.Parse(rawURI)
						if e == nil && origin == u.Scheme+"://"+u.Host {
							allowed = true
						}
					}
				}
				rows.Close()
			}
			if !allowed {
				JSON(w, 403, map[string]string{"error": "access_denied"})
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) allowOIDCRequest(r *http.Request, path string) bool {
	if r.Method == "OPTIONS" {
		return true
	}
	scope := "oidc-read"
	limit := 300
	window := int64(60_000)
	if path == "/authorize" {
		scope = "oidc-authorize"
		limit = 30
		window = 600_000
	} else if r.Method == "POST" {
		scope = "oidc-token"
		limit = 60
	}
	key := scope + ":" + a.ClientIP(r)
	now := NowMillis()
	a.rateMu.Lock()
	defer a.rateMu.Unlock()
	if a.rates == nil {
		a.rates = make(map[string]rateBucket)
	}
	if len(a.rates) >= 10000 {
		for k, b := range a.rates {
			if b.Reset <= now {
				delete(a.rates, k)
			}
		}
		if _, exists := a.rates[key]; !exists && len(a.rates) >= 10000 {
			return false
		}
	}
	bucket := a.rates[key]
	if bucket.Reset <= now {
		bucket = rateBucket{Reset: now + window}
	}
	if bucket.Count >= limit {
		return false
	}
	bucket.Count++
	a.rates[key] = bucket
	return true
}

func (a *App) identityClientsGet(w http.ResponseWriter, r *http.Request) error {
	if _, err := a.Require(r, "owner"); err != nil {
		return err
	}
	rows, err := a.Pool.Query(r.Context(), "SELECT id,name,kind,redirect_uris,disabled_at_ms,created_at_ms FROM identity_clients ORDER BY created_at_ms DESC,id")
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []identity.Client{}
	for rows.Next() {
		var c identity.Client
		var raw []byte
		if err = rows.Scan(&c.ID, &c.Name, &c.Kind, &raw, &c.DisabledAt, &c.CreatedAt); err != nil {
			return err
		}
		if err = json.Unmarshal(raw, &c.Redirects); err != nil {
			return err
		}
		items = append(items, c)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	JSON(w, 200, map[string]any{"issuer": a.Config.PublicBaseURL + "/oidc", "items": items})
	return nil
}
func (a *App) identityClientsCreate(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name      string   `json:"name"`
		Kind      string   `json:"kind"`
		Redirects []string `json:"redirect_uris"`
	}
	if err := Decode(r, &in); err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 1 || len([]rune(in.Name)) > 100 {
		return E(400, "VALIDATION_FAILED", "应用名称需要 1 至 100 个字符")
	}
	if err := identity.ValidateRedirects(in.Kind, in.Redirects); err != nil {
		return E(400, "VALIDATION_FAILED", err.Error())
	}
	c := identity.Client{ID: identity.Random(), Name: in.Name, Kind: in.Kind, Redirects: in.Redirects, CreatedAt: NowMillis()}
	secret := ""
	secretHash := ""
	if c.Kind == "web" {
		secret = identity.Random()
		secretHash = identity.Hash(secret)
	}
	_, err := a.Write(r, []string{"owner"}, true, false, func(tx pgx.Tx, p Principal) (any, error) {
		raw, _ := json.Marshal(c.Redirects)
		_, err := tx.Exec(r.Context(), "INSERT INTO identity_clients(id,name,kind,secret_hash,redirect_uris,created_by,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$4,$5,$6,$7,$7)", c.ID, c.Name, c.Kind, secretHash, raw, p.ID, c.CreatedAt)
		if err != nil {
			return nil, err
		}
		return nil, audit(r.Context(), tx, p, "site.settings_update", "site", "1", "registered identity client "+c.ID)
	})
	if err != nil {
		return err
	}
	out := map[string]any{"client": c}
	if secret != "" {
		out["client_secret"] = secret
	}
	JSON(w, 201, out)
	return nil
}
func (a *App) identityClientsDisable(w http.ResponseWriter, r *http.Request) error {
	_, err := a.Write(r, []string{"owner"}, true, false, func(tx pgx.Tx, p Principal) (any, error) {
		now := NowMillis()
		tag, err := tx.Exec(r.Context(), "UPDATE identity_clients SET disabled_at_ms=COALESCE(disabled_at_ms,$2),updated_at_ms=$2 WHERE id=$1", r.PathValue("id"), now)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() != 1 {
			return nil, NotFound()
		}
		if _, err = tx.Exec(r.Context(), "UPDATE identity_grants SET revoked_at_ms=$2 WHERE client_id=$1 AND revoked_at_ms IS NULL", r.PathValue("id"), now); err != nil {
			return nil, err
		}
		return nil, audit(r.Context(), tx, p, "site.settings_update", "site", "1", "disabled identity client "+r.PathValue("id"))
	})
	if err != nil {
		return err
	}
	JSON(w, 204, nil)
	return nil
}
func (a *App) identityClientsRotate(w http.ResponseWriter, r *http.Request) error {
	secret := identity.Random()
	_, err := a.Write(r, []string{"owner"}, true, false, func(tx pgx.Tx, p Principal) (any, error) {
		now := NowMillis()
		tag, err := tx.Exec(r.Context(), "UPDATE identity_clients SET secret_hash=$2,updated_at_ms=$3 WHERE id=$1 AND kind='web' AND disabled_at_ms IS NULL", r.PathValue("id"), identity.Hash(secret), now)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() != 1 {
			return nil, E(400, "VALIDATION_FAILED", "只能轮换启用中的 Web 应用密钥")
		}
		if _, err = tx.Exec(r.Context(), "UPDATE identity_grants SET revoked_at_ms=$2 WHERE client_id=$1 AND revoked_at_ms IS NULL", r.PathValue("id"), now); err != nil {
			return nil, err
		}
		return nil, audit(r.Context(), tx, p, "site.settings_update", "site", "1", "rotated identity client secret "+r.PathValue("id"))
	})
	if err != nil {
		return err
	}
	JSON(w, 200, map[string]string{"client_secret": secret})
	return nil
}

type consentData struct {
	Name, Kind, Redirect, User, ID, CSRF string
	Offline, Reauth, Profile             bool
}

var identityConsentTemplate = template.Must(template.New("identity-consent").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex,nofollow"><title>确认登录 · Painter</title><style>` + web.ColorStyles() + `
*{box-sizing:border-box}body{margin:0;background:var(--canvas);color:var(--ink);font:16px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;min-height:100svh;display:grid;place-items:center;padding:32px 20px}main{width:min(100%,480px);background:var(--paper);border-radius:16px;padding:36px;box-shadow:0 12px 40px color-mix(in srgb,var(--shadow-color) 5%,transparent)}.brand{display:flex;align-items:center;gap:10px;font-weight:600}.brand img{width:32px;height:32px;object-fit:contain}h1{font-size:28px;line-height:1.25;letter-spacing:-.025em;margin:32px 0 14px;overflow-wrap:anywhere}p{margin:0 0 18px;color:var(--muted)}strong{color:var(--ink)}ul{padding-left:20px;margin:22px 0}li+li{margin-top:8px}.target{font-size:13px;color:var(--muted);overflow-wrap:anywhere;border-top:1px solid var(--line);padding-top:20px;margin-top:24px}.actions{display:flex;gap:12px;margin-top:28px}button{min-height:44px;flex:1;border:1px solid var(--line);border-radius:10px;padding:10px 16px;background:var(--paper);color:var(--ink);font:inherit;font-weight:600;cursor:pointer}button.primary{border-color:var(--link);background:var(--link);color:var(--link-ink)}button:hover{filter:brightness(.94)}button:focus-visible{outline:3px solid var(--link);outline-offset:3px}::selection{background:var(--link-soft)}a{color:var(--link);text-underline-offset:3px}@media(max-width:420px){main{padding:28px 24px}h1{font-size:25px}.actions{flex-wrap:wrap}}
</style></head><body><main><div class="brand"><img src="/assets/images/painter-logo.png" alt="">Painter 账号</div><h1>{{if .Reauth}}请重新验证身份{{else}}登录到 {{.Name}}{{end}}</h1><p>当前账号 <strong>{{.User}}</strong></p>{{if .Reauth}}<p><strong>{{.Name}}</strong> 要求最近完成的登录验证。重新登录后，你会回到此处确认授权。</p>{{else}}<p>允许 <strong>{{.Name}}</strong> 使用你的 Painter 身份登录</p><ul><li>识别你的账号与本次登录验证方式</li>{{if .Profile}}<li>读取昵称和公开用户名</li>{{end}}{{if .Offline}}<li>保持登录最长 30 天，注销或撤销后失效</li>{{end}}</ul><p>此授权不包含博客后台管理、文章编辑或私密联系方式</p>{{end}}<div class="target">应用类型：{{.Kind}}<br>登录完成后返回：{{.Redirect}}</div><form method="post" action="/oidc/consent"><input type="hidden" name="id" value="{{.ID}}"><input type="hidden" name="csrf" value="{{.CSRF}}"><div class="actions"><button type="submit" name="action" value="deny">拒绝</button>{{if .Reauth}}<button class="primary" type="submit" name="action" value="reauth">重新登录</button>{{else}}<button class="primary" type="submit" name="action" value="allow">允许并登录</button>{{end}}</div></form></main></body></html>`))

var identityContinueTemplate = template.Must(template.New("identity-continue").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex,nofollow"><title>返回应用 · Painter</title></head><body><p>正在返回应用</p><p><a href="{{.Link}}">继续返回应用</a></p><script nonce="{{.Nonce}}">location.replace({{.Target}})</script></body></html>`))

type identityContinueData struct {
	Target string
	Link   template.URL
	Nonce  string
}

// A native form's entire redirect chain remains subject to form-action 'self'
// in Chromium. Finish the POST with a same-origin document before starting an
// ordinary navigation. This preserves the global CSP for arbitrary clients.
func (a *App) oidcContinue(w http.ResponseWriter, r *http.Request, target string) error {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Location", target)
	// target is either our fixed provider callback or the exact validated client
	// redirect from storage. Native reverse-domain schemes are safe at this bound.
	return identityContinueTemplate.Execute(w, identityContinueData{Target: target, Link: template.URL(target), Nonce: requestScriptNonce(r.Context())})
}

func oidcNeedsReauth(req *identity.Request, authenticatedAt time.Time) bool {
	if req.MaxAge == nil {
		return false
	}
	if *req.MaxAge == 0 {
		return authenticatedAt.Before(req.CreatedAt)
	}
	// Do not convert arbitrary uints to a duration, which could overflow.
	return time.Since(authenticatedAt).Seconds() > float64(*req.MaxAge)
}
func (a *App) oidcConsent(store *identity.Store, w http.ResponseWriter, r *http.Request) error {
	req, err := store.RequestByID(r.Context(), r.URL.Query().Get("id"))
	if err != nil {
		return E(400, "OAUTH_STATE_INVALID", "授权请求已过期，请从应用重新登录")
	}
	client, err := store.GetClient(r.Context(), req.ClientID)
	if err != nil {
		return E(400, "OAUTH_STATE_INVALID", "应用已停用")
	}
	p, err := a.Principal(r)
	if err != nil {
		http.Redirect(w, r, "/login?return_to="+url.QueryEscape("/oidc/consent?id="+req.ID), 303)
		return nil
	}
	_, authenticatedAt, _, err := a.SessionAuthentication(r.Context(), p.SessionID)
	if err != nil {
		return err
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	// no-referrer makes Chromium serialize a native form POST's Origin as null,
	// which our strict same-origin CSRF check must reject. Keep same-origin form
	// metadata while suppressing the request identifier on cross-origin links.
	w.Header().Set("Referrer-Policy", "same-origin")
	user := p.DisplayName
	if user == "" {
		user = p.Username
	}
	kind := map[string]string{"web": "Web 服务端应用", "spa": "浏览器应用", "native": "原生应用"}[client.Kind]
	return identityConsentTemplate.Execute(w, consentData{Name: client.Name, Kind: kind, Redirect: req.Redirect, User: user, ID: req.ID, CSRF: a.csrf(c.Value), Offline: slices.Contains(req.Scopes, "offline_access"), Profile: slices.Contains(req.Scopes, "profile"), Reauth: oidcNeedsReauth(req, authenticatedAt)})
}
func (a *App) oidcConsentPost(store *identity.Store, w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if err := r.ParseForm(); err != nil {
		return E(400, "VALIDATION_FAILED", "授权表单无效")
	}
	// Use the ordinary management CSRF/Origin/session transaction checks for the
	// native HTML form. No Origin or cookie is synthesized from request data.
	r.Header.Set("X-CSRF-Token", r.PostForm.Get("csrf"))
	req, err := store.RequestByID(r.Context(), r.PostForm.Get("id"))
	if err != nil {
		return E(400, "OAUTH_STATE_INVALID", "授权请求已过期")
	}
	action := r.PostForm.Get("action")
	if !slices.Contains([]string{"allow", "deny", "reauth"}, action) {
		return E(400, "VALIDATION_FAILED", "请选择授权操作")
	}
	_, err = a.Write(r, nil, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		if action == "reauth" {
			_, e := tx.Exec(r.Context(), "UPDATE sessions SET revoked_at_ms=$2 WHERE management_id=$1", p.SessionID, NowMillis())
			return nil, e
		}
		if action == "deny" {
			return nil, nil
		}
		method, authenticatedAt, mfaAt, e := sessionAuthentication(r.Context(), tx, p.SessionID)
		if e != nil {
			return nil, e
		}
		if oidcNeedsReauth(req, authenticatedAt) {
			return nil, E(401, "REAUTHENTICATION_REQUIRED", "请重新登录后确认授权")
		}
		req.Subject = p.ID
		req.SessionID = p.SessionID
		req.AuthTime = authenticatedAt
		req.MFAAt = mfaAt
		req.Complete = true
		if strings.HasPrefix(method, "password") || method == "local" {
			req.AMR = []string{"pwd"}
		} else {
			req.AMR = []string{"urn:painter:amr:" + strings.TrimSuffix(method, "+mfa")}
		}
		if mfaAt != nil {
			req.AMR = append(req.AMR, "otp")
		}
		// Complete within this transaction: the policy lock and session checks stay
		// authoritative until the authorization has been recorded.
		if e = store.CompleteTx(r.Context(), tx, req); e != nil {
			return nil, E(409, "OAUTH_STATE_INVALID", "授权已完成、会话已失效或应用已停用，请重新开始")
		}
		return nil, audit(r.Context(), tx, p, "user.update", "user", p.ID, "authorized identity client "+req.ClientID)
	})
	if err != nil {
		return err
	}
	if action == "reauth" {
		a.cookie(w, sessionCookie, "", -1)
		http.Redirect(w, r, "/login?return_to="+url.QueryEscape("/oidc/consent?id="+req.ID), 303)
		return nil
	}
	if action == "deny" {
		if err = store.DeleteAuthRequest(r.Context(), req.ID); err != nil {
			return err
		}
		u, _ := url.Parse(req.Redirect)
		q := u.Query()
		q.Set("error", "access_denied")
		if req.State != "" {
			q.Set("state", req.State)
		}
		u.RawQuery = q.Encode()
		return a.oidcContinue(w, r, u.String())
	}
	return a.oidcContinue(w, r, a.Config.PublicBaseURL+"/oidc/authorize/callback?id="+url.QueryEscape(req.ID))
}
