package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type App struct {
	Pool   *pgxpool.Pool
	Config Config
	Mux    *http.ServeMux
	rateMu sync.Mutex
	rates  map[string]rateBucket
}
type Principal struct{ ID, Role, Username, DisplayName, SessionID string }
type APIError struct {
	Status    int    `json:"-"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Details   any    `json:"details,omitempty"`
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }
func E(status int, code, message string) error {
	return &APIError{Status: status, Code: code, Message: message}
}
func NotFound() error  { return E(404, "NOT_FOUND", "资源不存在") }
func NowMillis() int64 { return time.Now().UnixMilli() }
func RandID() string {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Normalize(s string) string {
	return strings.Join(strings.Fields(norm.NFKC.String(cases.Fold().String(norm.NFKC.String(s)))), " ")
}
func TimeString(ms int64) string { return time.UnixMilli(ms).UTC().Format(time.RFC3339Nano) }
func Limit(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if n <= 0 {
		n = 20
	}
	if n > 50 {
		n = 50
	}
	return n
}
func IfMatch(r *http.Request) (int64, error) {
	s := r.Header.Get("If-Match")
	if s == "" {
		return 0, E(428, "PRECONDITION_REQUIRED", "需要 If-Match 版本")
	}
	if len(s) < 3 || s[0] != '"' || s[len(s)-1] != '"' {
		return 0, E(400, "VALIDATION_FAILED", "If-Match 必须是带引号的版本")
	}
	v, e := strconv.ParseInt(s[1:len(s)-1], 10, 64)
	if e != nil || v < 1 {
		return 0, E(400, "VALIDATION_FAILED", "版本无效")
	}
	return v, nil
}
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if status != 204 {
		_ = json.NewEncoder(w).Encode(v)
	}
}
func Error(w http.ResponseWriter, err error) {
	var ae *APIError
	if !errors.As(err, &ae) {
		var pe *pgconn.PgError
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			ae = &APIError{Status: 404, Code: "NOT_FOUND", Message: "资源不存在"}
		case errors.As(err, &pe):
			switch pe.Code {
			case "23505":
				ae = &APIError{Status: 409, Code: "CONFLICT", Message: "资源已存在"}
			case "23503", "23514", "22P02":
				ae = &APIError{Status: 400, Code: "VALIDATION_FAILED", Message: "数据未满足约束"}
			case "40001", "40P01", "55P03", "57014":
				ae = &APIError{Status: 503, Code: "WRITE_BUSY", Message: "服务繁忙，请稍后重试"}
			default:
				slog.Error("database request failed", "sqlstate", pe.Code)
				ae = &APIError{Status: 500, Code: "INTERNAL_ERROR", Message: "服务暂不可用"}
			}
		case errors.Is(err, context.DeadlineExceeded):
			ae = &APIError{Status: 503, Code: "WRITE_BUSY", Message: "请求超时，请稍后重试"}
		default:
			slog.Error("request failed", "error", err)
			ae = &APIError{Status: 500, Code: "INTERNAL_ERROR", Message: "服务暂不可用"}
		}
	}
	if ae.Status == 429 || ae.Status == 503 {
		w.Header().Set("Retry-After", "2")
	}
	copy := *ae
	copy.RequestID = w.Header().Get("X-Request-ID")
	if copy.RequestID == "" {
		copy.RequestID = RandID()
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, copy.Status, map[string]any{"error": copy})
}

type contextKey string

const bodyKey contextKey = "body"
const updateActorKey contextKey = "updateActor"

func RequestID(ctx context.Context) string { return requestID(ctx) }

const requestIDKey contextKey = "requestID"

func bodyBytes(r *http.Request) ([]byte, error) {
	if b, ok := r.Context().Value(bodyKey).([]byte); ok {
		return b, nil
	}
	if r.Body == nil {
		return []byte{}, nil
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if e != nil {
		return nil, E(400, "INVALID_JSON", "无法读取请求")
	}
	if len(b) > 1<<20 {
		return nil, E(413, "PAYLOAD_TOO_LARGE", "JSON最多1 MiB")
	}
	r.Body = io.NopCloser(bytes.NewReader(b))
	*r = *r.WithContext(context.WithValue(r.Context(), bodyKey, b))
	return b, nil
}
func Decode(r *http.Request, v any) error {
	b, e := bodyBytes(r)
	if e != nil {
		return e
	}
	if !utf8.Valid(b) {
		return E(400, "INVALID_JSON", "需要有效UTF-8 JSON")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(v); e != nil {
		return E(400, "INVALID_JSON", "JSON格式或字段无效")
	}
	if e = d.Decode(new(any)); e != io.EOF {
		return E(400, "INVALID_JSON", "仅允许一个JSON对象")
	}
	return nil
}
func (a *App) Handle(pattern string, fn func(http.ResponseWriter, *http.Request) error) {
	a.Mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			Error(w, err)
		}
	})
}
func New(ctx context.Context, c Config) (*App, error) {
	if err := c.defaults(); err != nil {
		return nil, err
	}
	pool, err := openDB(ctx, c)
	if err != nil {
		return nil, err
	}
	if err = Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	if err = os.MkdirAll(c.MediaDir, 0700); err != nil {
		pool.Close()
		return nil, err
	}
	a := &App{Pool: pool, Config: c, Mux: http.NewServeMux()}
	now := NowMillis()
	_, err = pool.Exec(ctx, "INSERT INTO site_profile(id,display_name,updated_at_ms) VALUES(1,'Painter', $1) ON CONFLICT DO NOTHING;", now)
	if err == nil {
		_, err = pool.Exec(ctx, "INSERT INTO site_settings(id,updated_at_ms) VALUES(1,$1) ON CONFLICT DO NOTHING", now)
	}
	if err != nil {
		pool.Close()
		return nil, err
	}
	a.RegisterIdentity()
	a.RegisterSetup()
	a.RegisterPhrases()
	a.RegisterMoments()
	a.RegisterHome()
	a.Handle("GET /api/v1/admin/home/psn", a.homePSNGet)
	a.Handle("PUT /api/v1/admin/home/psn", a.homePSNPut)
	a.Handle("POST /api/v1/admin/home/psn/sync", a.homePSNSync)
	a.RegisterFriends()
	a.RegisterAuthorProfiles()
	return a, nil
}
func (a *App) Close() { a.Pool.Close() }
func (a *App) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := RandID()
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/auth/") {
			w.Header().Set("Cache-Control", "private, no-store")
		}
		defer func() {
			if v := recover(); v != nil {
				slog.Error("handler panic", "request_id", w.Header().Get("X-Request-ID"))
				Error(w, E(500, "INTERNAL_ERROR", "服务暂不可用"))
			}
		}()
		r = a.securityHeaders(w, r)
		if a.redirectPublicHTTP(w, r) {
			return
		}
		r.Header.Set("X-Request-ID", w.Header().Get("X-Request-ID"))
		*r = *r.WithContext(context.WithValue(r.Context(), requestIDKey, w.Header().Get("X-Request-ID")))
		if !a.allowRequest(r) {
			Error(w, E(429, "RATE_LIMITED", "请求过于频繁，请稍后再试"))
			return
		}
		a.Mux.ServeHTTP(w, r)
	})
}
func hashString(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func macString(key []byte, s string) string {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(s))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
func (a *App) csrf(token string) string { return macString(a.Config.EncryptionKey, "csrf:v1:"+token) }
func (a *App) checkCSRF(r *http.Request) error {
	if r.Header.Get("Origin") != a.Config.PublicBaseURL {
		return E(403, "ORIGIN_REJECTED", "请求来源无效")
	}
	c, e := r.Cookie(sessionCookie)
	if e != nil {
		return E(401, "UNAUTHENTICATED", "请先登录")
	}
	supplied := r.Header.Get("X-CSRF-Token")
	if !hmac.Equal([]byte(supplied), []byte(a.csrf(c.Value))) {
		return E(403, "CSRF_FAILED", "CSRF校验失败")
	}
	return nil
}
func (a *App) Write(r *http.Request, roles []string, management, idempotent bool, fn func(pgx.Tx, Principal) (any, error)) (any, error) {
	pre, err := a.Principal(r)
	if err != nil {
		return nil, err
	}
	if err := a.checkCSRF(r); err != nil {
		return nil, err
	}
	operation := r.Method + " " + r.URL.Path
	var key, requestHash string
	if idempotent {
		key = r.Header.Get("Idempotency-Key")
		if len(key) < 8 || len(key) > 128 || strings.ContainsAny(key, "\r\n") {
			return nil, E(400, "IDEMPOTENCY_REQUIRED", "需要8至128字符的Idempotency-Key")
		}
		body, e := bodyBytes(r)
		if e != nil {
			return nil, e
		}
		requestHash = hashString(operation + "?" + r.URL.RawQuery + "\x00" + r.Header.Get("If-Match") + "\x00" + string(body))
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, err := a.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if idempotent {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", pre.ID+"/"+operation+"/"+key); err != nil {
			return nil, err
		}
	}
	lock := "SHARE"
	if management {
		lock = "UPDATE"
	}
	if _, err = tx.Exec(ctx, "SELECT id FROM policy_barrier WHERE id=1 FOR "+lock); err != nil {
		return nil, err
	}
	actorLock := "SHARE"
	if r.Context().Value(updateActorKey) == true {
		actorLock = "UPDATE"
	}
	p, err := a.txPrincipal(ctx, tx, r, actorLock)
	if err != nil {
		return nil, err
	}
	if (r.Header.Get("X-Painter-Admin-Tool") != "" && p.Role != "owner") || (len(roles) > 0 && !hasRole(p.Role, roles)) {
		return nil, E(403, "PERMISSION_DENIED", "没有执行此操作的权限")
	}
	if idempotent {
		var savedHash string
		var response []byte
		err = tx.QueryRow(ctx, "SELECT request_hash,response_json FROM idempotency_keys WHERE actor_user_id=$1 AND operation=$2 AND idempotency_key=$3 AND expires_at_ms>$4", p.ID, operation, key, NowMillis()).Scan(&savedHash, &response)
		if err == nil {
			if savedHash != requestHash {
				return nil, E(409, "IDEMPOTENCY_KEY_REUSED", "相同幂等键对应不同请求")
			}
			var value any
			if err = json.Unmarshal(response, &value); err != nil {
				return nil, err
			}
			return value, tx.Commit(ctx)
		}
		if err != pgx.ErrNoRows {
			return nil, err
		}
	}
	value, err := fn(tx, p)
	if err != nil {
		return nil, err
	}
	if idempotent {
		encoded, e := json.Marshal(value)
		if e != nil {
			return nil, e
		}
		now := NowMillis()
		_, err = tx.Exec(ctx, "INSERT INTO idempotency_keys(actor_user_id,operation,idempotency_key,request_hash,response_status,response_json,created_at_ms,expires_at_ms) VALUES($1,$2,$3,$4,200,$5,$6,$7) ON CONFLICT(actor_user_id,operation,idempotency_key) DO UPDATE SET request_hash=EXCLUDED.request_hash,response_status=EXCLUDED.response_status,response_json=EXCLUDED.response_json,created_at_ms=EXCLUDED.created_at_ms,expires_at_ms=EXCLUDED.expires_at_ms", p.ID, operation, key, requestHash, encoded, now, now+7*24*60*60*1000)
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return value, nil
}
func hasRole(role string, roles []string) bool {
	for _, r := range roles {
		if role == r {
			return true
		}
	}
	return false
}
func (a *App) Require(r *http.Request, roles ...string) (Principal, error) {
	p, e := a.Principal(r)
	if e != nil {
		return p, e
	}
	if (r.Header.Get("X-Painter-Admin-Tool") != "" && p.Role != "owner") || (len(roles) > 0 && !hasRole(p.Role, roles)) {
		return p, E(403, "PERMISSION_DENIED", "没有执行此操作的权限")
	}
	return p, nil
}
func (a *App) ReadReceipt(postID, revisionID string) string {
	payload := postID + "|" + revisionID + "|" + strconv.FormatInt(NowMillis(), 10)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + macString(a.Config.AnalyticsHMACKey, "read:v1:"+payload)
}
func (a *App) VerifyReadReceipt(token string) (string, string, error) {
	pieces := strings.Split(token, ".")
	if len(pieces) != 2 {
		return "", "", E(400, "INVALID_RECEIPT", "阅读凭据无效")
	}
	raw, e := base64.RawURLEncoding.DecodeString(pieces[0])
	if e != nil || !hmac.Equal([]byte(pieces[1]), []byte(macString(a.Config.AnalyticsHMACKey, "read:v1:"+string(raw)))) {
		return "", "", E(400, "INVALID_RECEIPT", "阅读凭据无效")
	}
	p := strings.Split(string(raw), "|")
	if len(p) != 3 {
		return "", "", E(400, "INVALID_RECEIPT", "阅读凭据无效")
	}
	at, e := strconv.ParseInt(p[2], 10, 64)
	if e != nil || at > NowMillis()+30000 || NowMillis()-at > 24*60*60*1000 {
		return "", "", E(400, "INVALID_RECEIPT", "阅读凭据已过期")
	}
	return p[0], p[1], nil
}
func audit(ctx context.Context, tx pgx.Tx, p Principal, action, target, targetID, reason string) error {
	_, e := tx.Exec(ctx, "INSERT INTO audit_logs(actor_user_id,action,target_type,target_id,request_id,reason,created_at_ms) VALUES($1,$2,$3,$4,$5,$6,$7)", p.ID, action, target, targetID, requestID(ctx), reason, NowMillis())
	return e
}

// Replay is an optimization for commands rendering outside the transaction.
// It authorizes a completed idempotent result before inspecting mutable drafts.
// A miss grants no write permission: Write must still re-authorize and lock.
func (a *App) Replay(r *http.Request, roles ...string) (any, bool, error) {
	pre, e := a.Principal(r)
	if e != nil {
		return nil, false, e
	}
	if e := a.checkCSRF(r); e != nil {
		return nil, false, e
	}
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 8 || len(key) > 128 {
		return nil, false, E(400, "IDEMPOTENCY_REQUIRED", "需要8至128字符的Idempotency-Key")
	}
	body, e := bodyBytes(r)
	if e != nil {
		return nil, false, e
	}
	operation := r.Method + " " + r.URL.Path
	requestHash := hashString(operation + "?" + r.URL.RawQuery + "\x00" + r.Header.Get("If-Match") + "\x00" + string(body))
	tx, e := a.Pool.BeginTx(r.Context(), pgx.TxOptions{})
	if e != nil {
		return nil, false, e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", pre.ID+"/"+operation+"/"+key); e != nil {
		return nil, false, e
	}
	if _, e = tx.Exec(r.Context(), "SELECT id FROM policy_barrier WHERE id=1 FOR SHARE"); e != nil {
		return nil, false, e
	}
	p, e := a.txPrincipal(r.Context(), tx, r, "SHARE")
	if e != nil {
		return nil, false, e
	}
	if (r.Header.Get("X-Painter-Admin-Tool") != "" && p.Role != "owner") || (len(roles) > 0 && !hasRole(p.Role, roles)) {
		return nil, false, E(403, "PERMISSION_DENIED", "没有执行此操作的权限")
	}
	var saved string
	var raw []byte
	e = tx.QueryRow(r.Context(), "SELECT request_hash,response_json FROM idempotency_keys WHERE actor_user_id=$1 AND operation=$2 AND idempotency_key=$3 AND expires_at_ms>$4", p.ID, operation, key, NowMillis()).Scan(&saved, &raw)
	if e == pgx.ErrNoRows {
		return nil, false, nil
	}
	if e != nil {
		return nil, false, e
	}
	if saved != requestHash {
		return nil, false, E(409, "IDEMPOTENCY_KEY_REUSED", "相同幂等键对应不同请求")
	}
	var value any
	if e = json.Unmarshal(raw, &value); e != nil {
		return nil, false, e
	}
	return value, true, tx.Commit(r.Context())
}

func requestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return RandID()
}

type rateBucket struct {
	Count int
	Reset int64
}

// Bounded local limiter intentionally uses the direct peer address. A reverse
// proxy must enforce its own edge limits; untrusted forwarding headers are ignored.
func (a *App) allowRequest(r *http.Request) bool {
	limit := 0
	window := int64(60000)
	scope := ""
	switch {
	case r.URL.Path == "/api/v1/setup/complete":
		limit = 10
		window = 600000
		scope = "setup"
	case r.URL.Path == "/auth/github/start":
		limit = 10
		window = 600000
		scope = "oauth-start"
	case r.URL.Path == "/auth/github/callback":
		limit = 30
		window = 600000
		scope = "oauth-callback"
	case r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS":
		limit = 180
		scope = "write"
	}
	if limit == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	key := scope + ":" + host
	now := NowMillis()
	a.rateMu.Lock()
	defer a.rateMu.Unlock()
	if a.rates == nil {
		a.rates = make(map[string]rateBucket)
	}
	b := a.rates[key]
	if b.Reset <= now {
		b = rateBucket{Reset: now + window}
	}
	if b.Count >= limit {
		return false
	}
	if len(a.rates) >= 10000 {
		for k, v := range a.rates {
			if v.Reset <= now {
				delete(a.rates, k)
			}
		}
		if _, ok := a.rates[key]; !ok && len(a.rates) >= 10000 {
			return false
		}
	}
	b.Count++
	a.rates[key] = b
	return true
}
