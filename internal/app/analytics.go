package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type analyticsEvent struct {
	EventID                                 string                     `json:"event_id"`
	Schema                                  int                        `json:"schema_version"`
	Kind                                    string                     `json:"event_name"`
	Page                                    string                     `json:"page_instance_id"`
	Search                                  *string                    `json:"search_instance_id,omitempty"`
	Operation                               *string                    `json:"operation_id,omitempty"`
	Post                                    *string                    `json:"post_id,omitempty"`
	Revision                                *string                    `json:"revision_id,omitempty"`
	ClientAt                                int64                      `json:"client_at_ms"`
	Receipt                                 string                     `json:"read_receipt,omitempty"`
	Props                                   map[string]json.RawMessage `json:"props"`
	semantic, path                          string
	queryCount, resultCount, rank, duration *int64
	editorOperation, errorCode              *string
}

var analyticsErrorCodes = map[string]bool{"NETWORK_ERROR": true, "FETCH_FAILED": true, "REQUEST_FAILED": true, "VERSION_CONFLICT": true, "VALIDATION_FAILED": true, "SESSION_EXPIRED": true, "UNAUTHENTICATED": true, "PERMISSION_DENIED": true, "CSRF_FAILED": true, "NOT_FOUND": true, "IDEMPOTENCY_KEY_REUSED": true, "PAYLOAD_TOO_LARGE": true, "MEDIA_NOT_READY": true, "STORAGE_UNAVAILABLE": true, "RATE_LIMITED": true, "WRITE_BUSY": true, "INTERNAL_ERROR": true, "PRECONDITION_REQUIRED": true, "UNKNOWN": true}

func analyticsToken(s string) bool {
	return len(s) >= 16 && len(s) <= 128 && !strings.ContainsAny(s, "\r\n\x00")
}
func analyticsHash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func analyticsPropInt(e *analyticsEvent, key string, min, max int64) (*int64, error) {
	b, ok := e.Props[key]
	if !ok {
		return nil, E(400, "VALIDATION_FAILED", "事件缺少属性 "+key)
	}
	var n int64
	if json.Unmarshal(b, &n) != nil || n < min || n > max {
		return nil, E(400, "VALIDATION_FAILED", "事件属性无效 "+key)
	}
	return &n, nil
}
func (e *analyticsEvent) validate(now int64) error {
	if e.Schema != 1 || !analyticsToken(e.EventID) || !analyticsToken(e.Page) {
		return E(400, "VALIDATION_FAILED", "事件版本或标识无效")
	}
	if e.ClientAt < now-24*60*60*1000 || e.ClientAt > now+60*1000 {
		return E(400, "STALE_EVENT", "事件超出24小时补传窗口")
	}
	if e.Search != nil && !analyticsToken(*e.Search) || e.Operation != nil && !analyticsToken(*e.Operation) {
		return E(400, "VALIDATION_FAILED", "事件实例标识无效")
	}
	if e.Post != nil {
		if err := interactionID(*e.Post); err != nil {
			return E(400, "VALIDATION_FAILED", "无效文章ID")
		}
	}
	if e.Revision != nil && (e.Post == nil || len(*e.Revision) > 128 || *e.Revision == "") {
		return E(400, "VALIDATION_FAILED", "修订必须属于文章")
	}
	e.path = "/"
	if e.Post != nil {
		e.path = "/posts/" + *e.Post
	}
	allowed := map[string]bool{}
	var err error
	semantic := ""
	switch e.Kind {
	case "page_view.v1":
		semantic = e.Kind + ":" + e.Page
	case "post_read.v1":
		if e.Post == nil || e.Revision == nil {
			return E(400, "VALIDATION_FAILED", "有效阅读必须带文章修订")
		}
		allowed["duration_ms"] = true
		e.duration, err = analyticsPropInt(e, "duration_ms", 10000, 600000)
		semantic = e.Kind + ":" + e.Page
	case "search_submit.v1":
		if e.Search == nil || e.Post != nil || e.Revision != nil {
			return E(400, "VALIDATION_FAILED", "搜索事件实例无效")
		}
		e.path = "/search"
		allowed = map[string]bool{"query_rune_count": true, "result_count": true, "duration_ms": true}
		e.queryCount, err = analyticsPropInt(e, "query_rune_count", 1, 64)
		if err == nil {
			e.resultCount, err = analyticsPropInt(e, "result_count", 0, 50)
		}
		if err == nil {
			e.duration, err = analyticsPropInt(e, "duration_ms", 0, 600000)
		}
		semantic = e.Kind + ":" + *e.Search
	case "search_result_open.v1":
		if e.Search == nil || e.Post == nil || e.Revision == nil {
			return E(400, "VALIDATION_FAILED", "搜索点击必须带文章修订和搜索实例")
		}
		allowed["result_rank"] = true
		e.rank, err = analyticsPropInt(e, "result_rank", 1, 1000)
		semantic = e.Kind + ":" + *e.Search + ":" + *e.Post
	case "editor_open.v1":
		e.path = "/studio"
		if e.Post != nil {
			e.path = "/studio/posts/" + *e.Post
		}
		semantic = e.Kind + ":" + e.Page
	case "editor_error.v1":
		if e.Operation == nil {
			return E(400, "VALIDATION_FAILED", "缺少编辑操作实例")
		}
		allowed = map[string]bool{"editor_operation": true, "error_code": true}
		var op, code string
		if json.Unmarshal(e.Props["editor_operation"], &op) != nil || json.Unmarshal(e.Props["error_code"], &code) != nil || !analyticsErrorCodes[code] {
			return E(400, "VALIDATION_FAILED", "编辑错误属性无效")
		}
		if !map[string]bool{"load": true, "save": true, "publish": true, "unpublish": true, "upload": true, "preview": true, "delete": true}[op] {
			return E(400, "VALIDATION_FAILED", "编辑操作无效")
		}
		e.editorOperation = &op
		e.errorCode = &code
		e.path = "/studio"
		if e.Post != nil {
			e.path = "/studio/posts/" + *e.Post
		}
		semantic = e.Kind + ":" + *e.Operation + ":" + code
	default:
		return E(400, "VALIDATION_FAILED", "不支持的事件类型")
	}
	if err != nil {
		return err
	}
	for key := range e.Props {
		if !allowed[key] {
			return E(400, "VALIDATION_FAILED", "事件包含未知属性 "+key)
		}
	}
	if e.Kind == "page_view.v1" && e.Post != nil && e.Revision == nil {
		return E(400, "VALIDATION_FAILED", "文章浏览必须带修订")
	}
	if !strings.HasPrefix(e.Kind, "editor_") && e.Kind != "search_submit.v1" && e.Operation != nil {
		return E(400, "VALIDATION_FAILED", "事件操作实例不适用")
	}
	e.semantic = analyticsHash(semantic)
	return nil
}
func (a *App) RegisterAnalytics() {
	a.Handle("POST /api/v1/analytics/events", a.analyticsIngest)
	a.Handle("GET /api/v1/admin/analytics", func(w http.ResponseWriter, r *http.Request) error { return a.analyticsReport(w, r, true) })
	a.Handle("GET /api/v1/me/analytics", func(w http.ResponseWriter, r *http.Request) error { return a.analyticsReport(w, r, false) })
}
func (a *App) analyticsVisitor(r *http.Request, day string) (*string, *http.Cookie) {
	if r.Header.Get("DNT") == "1" || r.Header.Get("Sec-GPC") == "1" {
		return nil, nil
	}
	id := ""
	if c, e := r.Cookie("painter_visitor"); e == nil {
		p := strings.Split(c.Value, ".")
		if len(p) == 3 && analyticsToken(p[0]) {
			expiry, err := strconv.ParseInt(p[1], 10, 64)
			want := macString(a.Config.EncryptionKey, "visitor:v1:"+p[0]+"."+p[1])
			if err == nil && expiry > NowMillis() && hmac.Equal([]byte(p[2]), []byte(want)) {
				id = p[0]
			}
		}
	}
	var cookie *http.Cookie
	if id == "" {
		id = RandID()
		expiry := strconv.FormatInt(NowMillis()+30*24*60*60*1000, 10)
		value := id + "." + expiry
		cookie = &http.Cookie{Name: "painter_visitor", Value: value + "." + macString(a.Config.EncryptionKey, "visitor:v1:"+value), Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.Config.PublicBaseURL, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 60 * 60}
	}
	h := hmac.New(sha256.New, a.Config.AnalyticsHMACKey)
	h.Write([]byte("visitor-day:v1:" + day + ":" + id))
	key := hex.EncodeToString(h.Sum(nil))
	return &key, cookie
}
func (a *App) analyticsIngest(w http.ResponseWriter, r *http.Request) error {
	if r.Header.Get("Origin") != a.Config.PublicBaseURL {
		return E(403, "ORIGIN_DENIED", "事件仅接受同源请求")
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, (64<<10)+1))
	if e != nil {
		return E(400, "INVALID_JSON", "事件读取失败")
	}
	if len(b) > 64<<10 {
		return E(413, "PAYLOAD_TOO_LARGE", "事件批次最多64KiB")
	}
	r.Body = io.NopCloser(bytes.NewReader(b))
	var input struct {
		Events []json.RawMessage `json:"events"`
	}
	if e = Decode(r, &input); e != nil {
		return e
	}
	if len(input.Events) < 1 || len(input.Events) > 50 {
		return E(400, "VALIDATION_FAILED", "每批需1至50条事件")
	}
	var d struct{ Events []analyticsEvent }
	for i, raw := range input.Events {
		var v analyticsEvent
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if e = decoder.Decode(&v); e != nil {
			return &APIError{Status: 400, Code: "VALIDATION_FAILED", Message: "事件格式或字段无效", Details: map[string]any{"index": i}}
		}
		d.Events = append(d.Events, v)
	}
	now := NowMillis()
	for i := range d.Events {
		if e = d.Events[i].validate(now); e != nil {
			if ae, ok := e.(*APIError); ok {
				ae.Details = map[string]any{"index": i}
			}
			return e
		}
		v := &d.Events[i]
		if (v.Kind == "page_view.v1" || v.Kind == "post_read.v1") && v.Post != nil {
			post, revision, err := a.VerifyReadReceipt(v.Receipt)
			if err != nil {
				if ae, ok := err.(*APIError); ok {
					ae.Details = map[string]any{"index": i}
				}
				return err
			}
			if post != *v.Post || v.Revision == nil || revision != *v.Revision {
				return &APIError{Status: 400, Code: "INVALID_RECEIPT", Message: "阅读凭据与文章不匹配", Details: map[string]any{"index": i}}
			}
		}
	}
	// Consistent insert order also keeps overlapping retry batches from taking
	// their semantic uniqueness locks in opposite order.
	sort.SliceStable(d.Events, func(i, j int) bool {
		if d.Events[i].semantic == d.Events[j].semantic {
			return d.Events[i].EventID < d.Events[j].EventID
		}
		return d.Events[i].semantic < d.Events[j].semantic
	})
	tx, e := a.Pool.Begin(r.Context())
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(r.Context(), `SELECT id FROM policy_barrier WHERE id=1 FOR SHARE`); e != nil {
		return e
	}
	var enabled bool
	if e = tx.QueryRow(r.Context(), `SELECT analytics_enabled FROM site_settings WHERE id=1`).Scan(&enabled); e != nil {
		return e
	}
	if !enabled {
		if e = tx.Commit(r.Context()); e != nil {
			return e
		}
		JSON(w, 200, map[string]any{"accepted": 0, "duplicate": 0, "disabled": true})
		return nil
	}
	var p Principal
	if pre, err := a.Principal(r); err == nil && pre.ID != "" {
		p, e = a.txPrincipal(r.Context(), tx, r, "SHARE")
		if e != nil {
			return e
		}
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, v := range d.Events {
		if v.Post != nil && !seen[*v.Post] {
			seen[*v.Post] = true
			ids = append(ids, *v.Post)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		x, _ := strconv.ParseInt(ids[i], 10, 64)
		y, _ := strconv.ParseInt(ids[j], 10, 64)
		return x < y
	})
	authors := map[string]string{}
	for _, id := range ids {
		var author string
		e = tx.QueryRow(r.Context(), `SELECT author_user_id::text FROM posts WHERE id=$1 FOR SHARE`, id).Scan(&author)
		if e == pgx.ErrNoRows {
			return E(400, "VALIDATION_FAILED", "文章事件不可接受")
		}
		if e != nil {
			return e
		}
		authors[id] = author
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	day := time.UnixMilli(now).In(loc).Format("2006-01-02")
	visitor, cookie := a.analyticsVisitor(r, day)
	accepted, duplicate := 0, 0
	ua := strings.ToLower(r.UserAgent())
	bot := strings.Contains(ua, "bot") || strings.Contains(ua, "crawler") || strings.Contains(ua, "spider")
	for i := range d.Events {
		v := &d.Events[i]
		var exists bool
		if e = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM analytics_events WHERE event_id=$1 OR semantic_key=$2)`, v.EventID, v.semantic).Scan(&exists); e != nil {
			return e
		}
		if exists {
			duplicate++
			continue
		}
		editor := strings.HasPrefix(v.Kind, "editor_")
		if editor {
			if p.Role != "author" && p.Role != "owner" {
				return E(403, "PERMISSION_DENIED", "编辑事件需要作者登录")
			}
			if v.Post != nil && authors[*v.Post] != p.ID && p.Role != "owner" {
				return NotFound()
			}
		} else if v.Post != nil {
			var revision string
			e = tx.QueryRow(r.Context(), `SELECT revision_id FROM public_posts WHERE post_id=$1`, *v.Post).Scan(&revision)
			if e == pgx.ErrNoRows {
				return E(400, "VALIDATION_FAILED", "文章已撤回或删除")
			}
			if e != nil {
				return e
			}
			if v.Kind == "search_result_open.v1" && revision != *v.Revision {
				return E(400, "VALIDATION_FAILED", "搜索结果修订已改变")
			}
		}
		traffic := "public"
		if p.Role == "owner" {
			traffic = "owner"
		} else if editor || (v.Post != nil && p.ID != "" && authors[*v.Post] == p.ID) {
			traffic = "author"
		} else if bot {
			traffic = "suspected_bot"
		}
		tag, e := tx.Exec(r.Context(), `INSERT INTO analytics_events(event_id,semantic_key,kind,page_instance_id,search_instance_id,operation_id,post_id,revision_id,client_occurred_at_ms,received_at_ms,day_key,visitor_day_key,traffic_class,normalized_path,query_rune_count,result_count,result_rank,duration_ms,editor_operation,error_code) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::date,$12,$13,$14,$15,$16,$17,$18,$19,$20) ON CONFLICT DO NOTHING`, v.EventID, v.semantic, v.Kind, v.Page, v.Search, v.Operation, v.Post, v.Revision, v.ClientAt, now, day, visitor, traffic, v.path, v.queryCount, v.resultCount, v.rank, v.duration, v.editorOperation, v.errorCode)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 0 {
			duplicate++
			continue
		}
		accepted++
		if p.ID != "" && v.Post != nil && (v.Kind == "page_view.v1" || v.Kind == "post_read.v1") {
			if _, e = tx.Exec(r.Context(), `INSERT INTO reading_history(user_id,post_id,last_read_at_ms) VALUES($1,$2,$3) ON CONFLICT(user_id,post_id) DO UPDATE SET last_read_at_ms=GREATEST(reading_history.last_read_at_ms,EXCLUDED.last_read_at_ms)`, p.ID, *v.Post, now); e != nil {
				return e
			}
		}
	}
	if e = tx.Commit(r.Context()); e != nil {
		return e
	}
	if cookie != nil {
		http.SetCookie(w, cookie)
	}
	JSON(w, 200, map[string]any{"accepted": accepted, "duplicate": duplicate})
	return nil
}

type analyticsRaw struct {
	seq                int64
	kind, day, traffic string
	post, visitor      *string
}
type analyticsDelta struct {
	day, resource                                                  string
	post                                                           *string
	pv, identified, reads, searches, opens, editor, errors, maxseq int64
	visitors                                                       map[string]int64
}

// RollupAnalytics processes durable pending rows, never a sequence high-water mark.
func (a *App) RollupAnalytics(ctx context.Context) error {
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, `SELECT id FROM policy_barrier WHERE id=1 FOR SHARE`); e != nil {
		return e
	}
	rows, e := tx.Query(ctx, `SELECT seq,kind,day_key::text,traffic_class,post_id::text,visitor_day_key FROM analytics_events WHERE aggregated_at_ms IS NULL ORDER BY received_at_ms,seq LIMIT 100 FOR UPDATE SKIP LOCKED`)
	if e != nil {
		return e
	}
	raw := []analyticsRaw{}
	for rows.Next() {
		var v analyticsRaw
		if e = rows.Scan(&v.seq, &v.kind, &v.day, &v.traffic, &v.post, &v.visitor); e != nil {
			rows.Close()
			return e
		}
		raw = append(raw, v)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return e
	}
	if len(raw) == 0 {
		return tx.Commit(ctx)
	}
	deltas := map[string]*analyticsDelta{}
	var maxseq int64
	for _, v := range raw {
		if v.seq > maxseq {
			maxseq = v.seq
		}
		editor := strings.HasPrefix(v.kind, "editor_")
		if !editor && v.traffic != "public" {
			continue
		}
		resources := []string{"site"}
		if v.post != nil {
			resources = append(resources, "post:"+*v.post)
		}
		for _, resource := range resources {
			key := v.day + ":" + resource
			d := deltas[key]
			if d == nil {
				d = &analyticsDelta{day: v.day, resource: resource, visitors: map[string]int64{}}
				if resource != "site" {
					d.post = v.post
				}
				deltas[key] = d
			}
			if v.seq > d.maxseq {
				d.maxseq = v.seq
			}
			switch v.kind {
			case "page_view.v1":
				d.pv++
				if v.visitor != nil {
					d.identified++
					old, ok := d.visitors[*v.visitor]
					if !ok || v.seq < old {
						d.visitors[*v.visitor] = v.seq
					}
				}
			case "post_read.v1":
				d.reads++
			case "search_submit.v1":
				if resource == "site" {
					d.searches++
				}
			case "search_result_open.v1":
				d.opens++
			case "editor_open.v1":
				d.editor++
			case "editor_error.v1":
				d.errors++
			}
		}
	}
	keys := make([]string, 0, len(deltas))
	for k := range deltas {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	now := NowMillis()
	for _, key := range keys {
		d := deltas[key]
		if _, e = tx.Exec(ctx, `INSERT INTO analytics_daily(day_key,resource_key,post_id,updated_at_ms) VALUES($1::date,$2,$3,$4) ON CONFLICT DO NOTHING`, d.day, d.resource, d.post, now); e != nil {
			return e
		}
		visitors := make([]string, 0, len(d.visitors))
		for visitor := range d.visitors {
			visitors = append(visitors, visitor)
		}
		sort.Strings(visitors)
		var uv int64
		for _, visitor := range visitors {
			tag, err := tx.Exec(ctx, `INSERT INTO analytics_scope_visitors(day_key,resource_key,visitor_day_key,first_event_seq) VALUES($1::date,$2,$3,$4) ON CONFLICT DO NOTHING`, d.day, d.resource, visitor, d.visitors[visitor])
			if err != nil {
				return err
			}
			uv += tag.RowsAffected()
		}
		if _, e = tx.Exec(ctx, `UPDATE analytics_daily SET page_views=page_views+$3,identified_page_views=identified_page_views+$4,unique_visitors=unique_visitors+$5,post_reads=post_reads+$6,search_submits=search_submits+$7,search_result_opens=search_result_opens+$8,editor_opens=editor_opens+$9,editor_errors=editor_errors+$10,computed_through_seq=GREATEST(computed_through_seq,$11),updated_at_ms=$12 WHERE day_key=$1::date AND resource_key=$2`, d.day, d.resource, d.pv, d.identified, uv, d.reads, d.searches, d.opens, d.editor, d.errors, d.maxseq, now); e != nil {
			return e
		}
	}
	seqs := make([]int64, len(raw))
	for i, v := range raw {
		seqs[i] = v.seq
	}
	if _, e = tx.Exec(ctx, `UPDATE analytics_events SET aggregated_at_ms=$2 WHERE seq=ANY($1::bigint[])`, seqs, now); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `UPDATE analytics_rollup_state SET last_seq=GREATEST(last_seq,$1),updated_at_ms=$2 WHERE id=1`, maxseq, now); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

type analyticsTask struct {
	id, topic, token string
	payload          []byte
	attempts         int64
}

func (a *App) ConsumeOutbox(ctx context.Context) error {
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, `SELECT id FROM policy_barrier WHERE id=1 FOR SHARE`); e != nil {
		return e
	}
	rows, e := tx.Query(ctx, `SELECT id,topic,payload_json,attempts FROM outbox WHERE (state='pending' AND available_at_ms<=(extract(epoch FROM clock_timestamp())*1000)::bigint) OR (state='processing' AND lease_until_ms<=(extract(epoch FROM clock_timestamp())*1000)::bigint) ORDER BY available_at_ms,id LIMIT 20 FOR UPDATE SKIP LOCKED`)
	if e != nil {
		return e
	}
	tasks := []analyticsTask{}
	exhausted := []string{}
	for rows.Next() {
		var t analyticsTask
		if e = rows.Scan(&t.id, &t.topic, &t.payload, &t.attempts); e != nil {
			rows.Close()
			return e
		}
		if t.attempts >= 5 {
			exhausted = append(exhausted, t.id)
			continue
		}
		t.token = RandID()
		t.attempts++
		tasks = append(tasks, t)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return e
	}
	for _, id := range exhausted {
		if _, e = tx.Exec(ctx, `UPDATE outbox SET state='dead',lease_token=NULL,lease_until_ms=NULL,last_error='delivery attempts exhausted after lease expiry' WHERE id=$1`, id); e != nil {
			return e
		}
	}
	for _, t := range tasks {
		if _, e = tx.Exec(ctx, `UPDATE outbox SET state='processing',attempts=attempts+1,lease_token=$2,lease_until_ms=(extract(epoch FROM clock_timestamp())*1000)::bigint+30000 WHERE id=$1`, t.id, t.token); e != nil {
			return e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	for _, t := range tasks {
		if e = a.analyticsConsumeFact(ctx, t); e != nil {
			state := "pending"
			if t.attempts >= 5 {
				state = "dead"
			}
			backoff := int64(1000) * (1 << min(t.attempts, 8))
			err := a.moduleWriteTx(ctx, func(failtx pgx.Tx) error {
				_, err := failtx.Exec(ctx, `UPDATE outbox SET state=$3,last_error=$4,available_at_ms=(extract(epoch FROM clock_timestamp())*1000)::bigint+$5,lease_token=NULL,lease_until_ms=NULL WHERE id=$1 AND state='processing' AND lease_token=$2 AND lease_until_ms>(extract(epoch FROM clock_timestamp())*1000)::bigint`, t.id, t.token, state, "fact consumer rejected schema or database transaction; see server log", backoff)
				return err
			})
			if err != nil {
				return err
			}
			slog.Error("outbox fact failed", "event_id", t.id, "error", e)
		}
	}
	return nil
}
func (a *App) analyticsConsumeFact(ctx context.Context, t analyticsTask) error {
	var payload struct {
		EventID   string `json:"event_id"`
		Schema    int    `json:"schema_version"`
		EventType string `json:"event_type"`
		Occurred  int64  `json:"occurred_at_ms"`
	}
	if e := json.Unmarshal(t.payload, &payload); e != nil || payload.Schema != 1 || payload.EventType != t.topic || payload.EventID != t.id || payload.Occurred <= 0 {
		return fmt.Errorf("invalid outbox envelope")
	}
	column := map[string]string{"post.created.v1": "posts_created", "post.published.v1": "posts_published", "post.unpublished.v1": "posts_unpublished", "post.trashed.v1": "posts_trashed", "comment.created.v1": "comments_created", "user.registered.v1": "users_registered"}[t.topic]
	if column == "" {
		return fmt.Errorf("unsupported fact topic %s", t.topic)
	}
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, `SELECT id FROM policy_barrier WHERE id=1 FOR SHARE`); e != nil {
		return e
	}
	var valid bool
	if e = tx.QueryRow(ctx, `SELECT lease_token=$2 AND lease_until_ms>(extract(epoch FROM clock_timestamp())*1000)::bigint FROM outbox WHERE id=$1 AND state='processing' FOR UPDATE`, t.id, t.token).Scan(&valid); e == pgx.ErrNoRows {
		return nil
	}
	if e != nil {
		return e
	}
	if !valid {
		return nil
	}
	now := NowMillis()
	tag, e := tx.Exec(ctx, `INSERT INTO outbox_consumptions(consumer,event_id,consumed_at_ms) VALUES('business_daily.v1',$1,$2) ON CONFLICT DO NOTHING`, t.id, now)
	if e != nil {
		return e
	}
	if tag.RowsAffected() > 0 {
		loc, _ := time.LoadLocation("Asia/Shanghai")
		day := time.UnixMilli(payload.Occurred).In(loc).Format("2006-01-02")
		if _, e = tx.Exec(ctx, `INSERT INTO business_daily(day_key,`+column+`,updated_at_ms) VALUES($1::date,1,$2) ON CONFLICT(day_key) DO UPDATE SET `+column+`=business_daily.`+column+`+1,updated_at_ms=EXCLUDED.updated_at_ms`, day, now); e != nil {
			return e
		}
	}
	tag, e = tx.Exec(ctx, `UPDATE outbox SET state='done',completed_at_ms=$3,lease_token=NULL,lease_until_ms=NULL,last_error=NULL WHERE id=$1 AND state='processing' AND lease_token=$2 AND lease_until_ms>(extract(epoch FROM clock_timestamp())*1000)::bigint`, t.id, t.token, now)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("lease expired before commit")
	}
	return tx.Commit(ctx)
}
func (a *App) RunWorkers(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	count := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count++
			jobctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			if e := a.RollupAnalytics(jobctx); e != nil && ctx.Err() == nil {
				slog.Error("analytics rollup failed", "error", e)
			}
			if e := a.ConsumeOutbox(jobctx); e != nil && ctx.Err() == nil {
				slog.Error("outbox worker failed", "error", e)
			}
			if count%60 == 0 {
				if e := a.ReconcileMedia(jobctx); e != nil && ctx.Err() == nil {
					slog.Error("media reconciliation failed", "error", e)
				}
				if e := a.SyncHomeSteam(jobctx); e != nil && ctx.Err() == nil {
					// Provider errors/URLs must never be logged with credentials.
					slog.Warn("homepage Steam sync did not complete")
				}
			}
			if count%3600 == 0 {
				if e := a.analyticsRetain(jobctx); e != nil && ctx.Err() == nil {
					slog.Error("analytics retention failed", "error", e)
				}
				if e := a.PruneIdentity(jobctx); e != nil && ctx.Err() == nil {
					slog.Error("identity retention failed", "error", e)
				}
			}
			cancel()
		}
	}
}
func (a *App) analyticsRetain(ctx context.Context) error {
	return a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		now := NowMillis()
		if _, e := tx.Exec(ctx, `DELETE FROM analytics_events WHERE seq IN(SELECT seq FROM analytics_events WHERE aggregated_at_ms IS NOT NULL AND received_at_ms<$1 ORDER BY received_at_ms LIMIT 5000)`, now-30*24*60*60*1000); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `DELETE FROM analytics_scope_visitors WHERE day_key<(CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai')::date-35`); e != nil {
			return e
		}
		_, e := tx.Exec(ctx, `DELETE FROM analytics_daily WHERE day_key<(CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai')::date-400`)
		return e
	})
}

func (a *App) analyticsReport(w http.ResponseWriter, r *http.Request, admin bool) error {
	roles := []string{"author", "owner"}
	if admin {
		roles = []string{"owner"}
	}
	p, e := a.Require(r, roles...)
	if e != nil {
		return e
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	today := time.Now().In(loc)
	from := r.URL.Query().Get("day_from")
	to := r.URL.Query().Get("day_to")
	if from == "" {
		from = today.AddDate(0, 0, -29).Format("2006-01-02")
	}
	if to == "" {
		to = today.Format("2006-01-02")
	}
	f, err := time.Parse("2006-01-02", from)
	if err != nil {
		return E(400, "VALIDATION_FAILED", "起始日期无效")
	}
	t, err := time.Parse("2006-01-02", to)
	if err != nil || t.Before(f) || t.Sub(f) > 399*24*time.Hour {
		return E(400, "VALIDATION_FAILED", "日期范围最多400天")
	}
	resource := r.URL.Query().Get("resource")
	if resource == "" && admin {
		resource = "site"
	}
	if resource != "" && resource != "site" {
		if !strings.HasPrefix(resource, "post:") {
			return E(400, "VALIDATION_FAILED", "统计资源无效")
		}
		if e = interactionID(strings.TrimPrefix(resource, "post:")); e != nil {
			return E(400, "VALIDATION_FAILED", "统计资源无效")
		}
	}
	if !admin && resource == "site" {
		return E(403, "PERMISSION_DENIED", "作者只能读取本人文章统计")
	}
	metric := r.URL.Query().Get("metric")
	if metric != "" && !map[string]bool{"page_views": true, "unique_visitors": true, "post_reads": true, "search_submits": true, "search_result_opens": true, "editor_opens": true, "editor_errors": true}[metric] {
		return E(400, "VALIDATION_FAILED", "统计指标无效")
	}
	rows, e := a.Pool.Query(r.Context(), `SELECT d.day_key::text,d.resource_key,d.post_id::text,d.page_views,d.identified_page_views,d.unique_visitors,d.post_reads,d.search_submits,d.search_result_opens,d.editor_opens,d.editor_errors,d.updated_at_ms FROM analytics_daily d LEFT JOIN posts p ON p.id=d.post_id WHERE d.day_key BETWEEN $1::date AND $2::date AND ($3='' OR d.resource_key=$3) AND ($4::boolean OR p.author_user_id=$5::bigint) ORDER BY d.day_key,d.resource_key LIMIT 20001`, from, to, resource, admin, p.ID)
	if e != nil {
		return e
	}
	items := []map[string]any{}
	var pv, identified, asof int64
	for rows.Next() {
		var day, key string
		var post *string
		var views, marked, uv, reads, searches, opens, editor, errors, updated int64
		if e = rows.Scan(&day, &key, &post, &views, &marked, &uv, &reads, &searches, &opens, &editor, &errors, &updated); e != nil {
			rows.Close()
			return e
		}
		if len(items) == 20000 {
			rows.Close()
			return E(400, "RANGE_TOO_LARGE", "请缩小日期范围或指定文章")
		}
		items = append(items, map[string]any{"day": day, "day_key": day, "resource": key, "resource_key": key, "post_id": post, "page_views": views, "identified_page_views": marked, "unique_visitors": uv, "post_reads": reads, "search_submits": searches, "search_result_opens": opens, "editor_opens": editor, "editor_errors": errors})
		pv += views
		identified += marked
		if updated > asof {
			asof = updated
		}
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return e
	}
	var pending int64
	var oldest *int64
	if e = a.Pool.QueryRow(r.Context(), `SELECT count(*),min(e.received_at_ms) FROM analytics_events e LEFT JOIN posts p ON p.id=e.post_id WHERE e.aggregated_at_ms IS NULL AND ($1::boolean OR p.author_user_id=$2::bigint) AND ($3='' OR $3='site' OR 'post:'||e.post_id::text=$3)`, admin, p.ID, resource).Scan(&pending, &oldest); e != nil {
		return e
	}
	legacy := []map[string]any{}
	business := []map[string]any{}
	if admin {
		lr, err := a.Pool.Query(r.Context(), `SELECT metric,day_key::text,post_id::text,sum(count_value) FROM legacy_counters WHERE (day_key BETWEEN $1::date AND $2::date OR day_key IS NULL) AND ($3='site' OR $3='' OR 'post:'||post_id::text=$3) GROUP BY metric,day_key,post_id ORDER BY day_key,post_id`, from, to, resource)
		if err != nil {
			return err
		}
		for lr.Next() {
			var metric string
			var day, post *string
			var n int64
			if err = lr.Scan(&metric, &day, &post, &n); err != nil {
				lr.Close()
				return err
			}
			legacy = append(legacy, map[string]any{"metric": metric, "day": day, "post_id": post, "value": n})
		}
		lr.Close()
		if err = lr.Err(); err != nil {
			return err
		}
		br, err := a.Pool.Query(r.Context(), `SELECT day_key::text,posts_created,posts_published,posts_unpublished,posts_trashed,comments_created,users_registered FROM business_daily WHERE day_key BETWEEN $1::date AND $2::date ORDER BY day_key`, from, to)
		if err != nil {
			return err
		}
		for br.Next() {
			var day string
			var created, published, unpublished, trashed, comments, users int64
			if err = br.Scan(&day, &created, &published, &unpublished, &trashed, &comments, &users); err != nil {
				br.Close()
				return err
			}
			business = append(business, map[string]any{"day": day, "posts_created": created, "posts_published": published, "posts_unpublished": unpublished, "posts_trashed": trashed, "comments_created": comments, "users_registered": users})
		}
		br.Close()
		if err = br.Err(); err != nil {
			return err
		}
	}
	var coverage any
	if pv > 0 {
		coverage = float64(identified) / float64(pv)
	}
	var dataAt, oldestAt any
	if asof > 0 {
		dataAt = interactionTime(asof)
	}
	if oldest != nil {
		oldestAt = interactionTime(*oldest)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, map[string]any{"items": items, "next_cursor": "", "has_more": false, "coverage": coverage, "data_as_of": dataAt, "pending_count": pending, "oldest_pending_at": oldestAt, "legacy": legacy, "business": business, "uv_scope": "daily_per_resource; daily UV cannot be summed", "metric": metric})
	return nil
}
