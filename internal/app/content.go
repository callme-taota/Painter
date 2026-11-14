package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Query helpers deliberately accept a transaction, never silently use the pool.
type contentQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type draftInput struct {
	UnknownEditedAt      bool                `json:"-"`
	PostID               string              `json:"-"`
	ClientDraftID        string              `json:"client_draft_id"`
	Title                string              `json:"title"`
	Summary              string              `json:"summary"`
	Markdown             string              `json:"markdown"`
	CategoryID           *string             `json:"category_id"`
	TagIDs               []string            `json:"tag_ids"`
	MediaIDs             []string            `json:"media_ids"`
	Cover                CoverConfig         `json:"cover"`
	DateDisplay          *ArticleDateDisplay `json:"date_display,omitempty"`
	ExpectedDraftVersion int64               `json:"expected_draft_version"`
}
type contentState struct {
	ID, Author string
	Version    int64
	Deleted    bool
}

func (a *App) RegisterContent() {
	a.Handle("GET /api/v1/posts", func(w http.ResponseWriter, r *http.Request) error {
		v, e := a.PublicPosts(r.Context(), r.URL.Query())
		if e == nil {
			JSON(w, 200, v)
		}
		return e
	})
	a.Handle("GET /api/v1/posts/{id}", func(w http.ResponseWriter, r *http.Request) error {
		v, e := a.PublicPost(r.Context(), r.PathValue("id"))
		if e == nil {
			JSON(w, 200, v)
		}
		return e
	})
	a.Handle("GET /api/v1/users/{id}/posts", func(w http.ResponseWriter, r *http.Request) error {
		v, e := a.AuthorPosts(r.Context(), r.PathValue("id"), r.URL.Query())
		if e == nil {
			JSON(w, 200, v)
		}
		return e
	})
	a.Handle("GET /api/v1/me/posts", a.myPosts)
	a.Handle("GET /api/v1/me/posts/by-client-id/{client_draft_id}", a.byClientDraft)
	a.Handle("POST /api/v1/posts", a.createPost)
	a.Handle("GET /api/v1/studio/posts/{id}", a.getStudioPost)
	a.Handle("PUT /api/v1/studio/posts/{id}/draft", a.saveDraft)
	a.Handle("POST /api/v1/studio/posts/{id}/checkpoints", func(w http.ResponseWriter, r *http.Request) error { return a.snapshotPost(w, r, false) })
	a.Handle("POST /api/v1/studio/posts/{id}/publish", func(w http.ResponseWriter, r *http.Request) error { return a.snapshotPost(w, r, true) })
	a.Handle("DELETE /api/v1/studio/posts/{id}/publication", func(w http.ResponseWriter, r *http.Request) error { return a.changePostState(w, r, "unpublish") })
	a.Handle("DELETE /api/v1/studio/posts/{id}", func(w http.ResponseWriter, r *http.Request) error { return a.changePostState(w, r, "trash") })
	a.Handle("POST /api/v1/studio/posts/{id}/recover", func(w http.ResponseWriter, r *http.Request) error { return a.changePostState(w, r, "recover") })
	a.Handle("GET /api/v1/studio/posts/{id}/revisions", a.listRevisions)
	a.Handle("GET /api/v1/studio/posts/{id}/revisions/{rid}", a.getRevision)
	a.Handle("POST /api/v1/studio/posts/{id}/restore", a.restoreRevision)
	a.Handle("POST /api/v1/editor/preview", a.previewMarkdown)
	a.Handle("POST /api/v1/editor/cards/resolve", a.resolveContentCard)
	a.Handle("GET /api/v1/content-cards/{kind}/{id}", a.publicContentCard)
	a.registerCovers()
}

func contentObjects(ctx context.Context, db contentQuerier, sql string, args ...any) ([]map[string]any, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var v map[string]any
		if err = json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		contentInlineMedia(v)
		out = append(out, v)
	}
	return out, rows.Err()
}
func contentObject(ctx context.Context, db contentQuerier, sql string, args ...any) (map[string]any, error) {
	var raw []byte
	if e := db.QueryRow(ctx, sql, args...).Scan(&raw); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return nil, NotFound()
		}
		return nil, e
	}
	var v map[string]any
	e := json.Unmarshal(raw, &v)
	if e == nil {
		contentInlineMedia(v)
	}
	return v, e
}

// Relation tables also protect uploaded covers. Full editable objects expose
// only source image IDs, so removing a cover cannot leave a stale inline ID.
func contentInlineMedia(v map[string]any) {
	markdown, ok := v["markdown"].(string)
	if !ok {
		return
	}
	if _, ok := v["media_ids"]; !ok {
		return
	}
	if rendered, e := RenderMarkdown(markdown); e == nil {
		sort.Strings(rendered.MediaIDs)
		v["media_ids"] = rendered.MediaIDs
	}
}
func contentTimeSQL(field string) string {
	return "to_char(to_timestamp(" + field + "/1000.0) AT TIME ZONE 'UTC','YYYY-MM-DD\"T\"HH24:MI:SS.MS\"Z\"')"
}

var publicPostJSON = `jsonb_build_object('id',v.post_id::text,'post_id',v.post_id::text,'title',v.title,'summary',v.summary,
 'revision_id',v.revision_id,'published_at',` + contentTimeSQL("v.published_at_ms") + `,
 'created_at',` + contentTimeSQL("original_post.created_at_ms") + `,'updated_at',` + contentTimeSQL("pr.content_updated_at_ms") + `,'date_display',pr.date_display,
 'author',jsonb_build_object('id',u.id::text,'username',u.username,'display_name',u.display_name,'avatar_url',CASE WHEN u.avatar_media_id IS NULL THEN '' ELSE '/media/'||u.avatar_media_id END),
 'cover',(SELECT cover FROM post_revisions WHERE id=v.revision_id),
 'cover_url',CASE WHEN (SELECT cover->>'mode' FROM post_revisions WHERE id=v.revision_id)='none' THEN '' ELSE '/api/v1/posts/'||v.post_id||'/cover?revision='||v.revision_id END,
 'tags',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',rt.tag_id::text,'name',t.name) ORDER BY t.name,rt.tag_id) FROM effective_revision_tags rt JOIN tags t ON t.id=rt.tag_id WHERE rt.revision_id=v.revision_id),'[]'::jsonb))`

const publicPostFrom = ` FROM public_posts v JOIN users u ON u.id=v.author_user_id JOIN posts original_post ON original_post.id=v.post_id JOIN post_revisions pr ON pr.id=v.revision_id `

func contentLimit(q url.Values) int {
	n, _ := strconv.Atoi(q.Get("limit"))
	if n < 1 {
		n = 20
	}
	if n > 50 {
		n = 50
	}
	return n
}
func contentID(value string) error {
	if value == "" {
		return E(400, "VALIDATION_FAILED", "资源ID无效")
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return E(400, "VALIDATION_FAILED", "资源ID无效")
	}
	return nil
}

// Selected tags are an OR filter; repeated tag_id values are canonicalized so
// reordering selections does not invalidate pagination. Legacy single IDs work.
func selectedTagIDs(q url.Values) ([]string, error) {
	ids := []string{}
	seen := map[string]bool{}
	if len(q["tag_id"]) > 100 {
		return nil, E(400, "VALIDATION_FAILED", "最多选择20个标签")
	}
	for _, value := range q["tag_id"] {
		if value == "" {
			continue
		}
		if e := contentID(value); e != nil {
			return nil, e
		}
		if !seen[value] {
			ids = append(ids, value)
			seen[value] = true
		}
	}
	if len(ids) > 20 {
		return nil, E(400, "VALIDATION_FAILED", "最多选择20个标签")
	}
	sort.Strings(ids)
	return ids, nil
}
func tagFilterScope(q url.Values) string {
	ids, _ := selectedTagIDs(q)
	return strings.Join(ids, ",")
}
func contentFilter(q url.Values, start int) (string, []any, error) {
	where := " WHERE true "
	args := []any{}
	for _, f := range []struct{ key, clause string }{{"author_id", "v.author_user_id="}, {"category_id", "EXISTS(SELECT 1 FROM effective_revision_tags ft JOIN category_tag_map cm ON cm.tag_id=ft.tag_id WHERE ft.revision_id=v.revision_id AND cm.category_id="}} {
		if value := q.Get(f.key); value != "" {
			if e := contentID(value); e != nil {
				return "", nil, e
			}
			args = append(args, value)
			where += fmt.Sprintf(" AND %s$%d", f.clause, start+len(args)-1)
			if f.key == "category_id" {
				where += ")"
			}
		}
	}
	ids, e := selectedTagIDs(q)
	if e != nil {
		return "", nil, e
	}
	if len(ids) > 0 {
		args = append(args, ids)
		where += fmt.Sprintf(` AND EXISTS(SELECT 1 FROM effective_revision_tags ft WHERE ft.revision_id=v.revision_id AND ft.tag_id IN (SELECT COALESCE(ta.canonical_tag_id, wanted.id::bigint) FROM unnest($%d::text[]) wanted(id) LEFT JOIN tag_aliases ta ON ta.tag_id=wanted.id::bigint))`, start+len(args)-1)
	}
	return where, args, nil
}
func (a *App) PublicPosts(ctx context.Context, q url.Values) (map[string]any, error) {
	limit := contentLimit(q)
	where, args, e := contentFilter(q, 1)
	if e != nil {
		return nil, e
	}
	scope := hashString(strings.Join([]string{"public-posts", q.Get("author_id"), q.Get("category_id"), tagFilterScope(q), strconv.Itoa(limit)}, "\x00"))
	if cursor := q.Get("cursor"); cursor != "" {
		var page contentListPage
		if e = a.decodePage(cursor, &page); e != nil {
			return nil, e
		}
		if page.Scope != scope || contentID(page.ID) != nil || page.Timestamp < 0 {
			return nil, E(400, "VALIDATION_FAILED", "分页条件已改变，请重新加载")
		}
		args = append(args, page.Timestamp, page.ID)
		where += fmt.Sprintf(" AND (v.first_published_at_ms,v.post_id)<($%d,$%d::bigint)", len(args)-1, len(args))
	}

	args = append(args, limit+1)
	sql := "SELECT " + publicPostJSON + ` || jsonb_build_object('_cursor',v.first_published_at_ms::text||'.'||v.post_id::text)` + publicPostFrom + where + fmt.Sprintf(" ORDER BY v.first_published_at_ms DESC,v.post_id DESC LIMIT $%d", len(args))
	items, e := contentObjects(ctx, a.Pool, sql, args...)
	if e != nil {
		return nil, e
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	next := ""
	if more {
		raw := items[len(items)-1]["_cursor"].(string)
		parts := strings.Split(raw, ".")
		stamp, _ := strconv.ParseInt(parts[0], 10, 64)
		next = a.encodePage(contentListPage{Scope: scope, ID: parts[1], Timestamp: stamp})
	}
	for _, item := range items {
		delete(item, "_cursor")
	}
	return map[string]any{"items": items, "next_cursor": next, "has_more": more}, nil
}
func (a *App) PublicPost(ctx context.Context, id string) (map[string]any, error) {
	if contentID(id) != nil {
		return nil, NotFound()
	}
	v, e := contentObject(ctx, a.Pool, "SELECT "+publicPostJSON+` || jsonb_build_object('sanitized_html',v.sanitized_html,'toc',v.toc_json,'renderer_version',v.renderer_version)`+publicPostFrom+" WHERE v.post_id=$1", id)
	if e == nil {
		htmlValue, _ := v["sanitized_html"].(string)
		expanded, err := a.ExpandInternalContentCards(ctx, htmlValue)
		if err != nil {
			return nil, err
		}
		v["sanitized_html"] = expanded
		v["read_receipt"] = a.ReadReceipt(id, v["revision_id"].(string))
	}
	return v, e
}

func (a *App) privatePost(ctx context.Context, db contentQuerier, p Principal, id string) (map[string]any, error) {
	if contentID(id) != nil {
		return nil, NotFound()
	}
	sql := contentPrivateSQL(true) + " WHERE p.id=$1 AND (p.author_user_id=$2 OR $3)"
	return contentObject(ctx, db, sql, id, p.ID, p.Role == "owner")
}
func (a *App) getStudioPost(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r, "author", "owner")
	if e != nil {
		return e
	}
	v, e := a.privatePost(r.Context(), a.Pool, p, r.PathValue("id"))
	if e == nil {
		JSON(w, 200, v)
	}
	return e
}
func (a *App) myPosts(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r, "author", "owner")
	if e != nil {
		return e
	}
	q := r.URL.Query()
	limit := contentLimit(q)
	status := q.Get("status")
	site := q.Get("scope") == "site"
	if site && p.Role != "owner" {
		return E(403, "PERMISSION_DENIED", "需要站长权限")
	}
	where := " WHERE true "
	args := []any{}
	if !site {
		args = append(args, p.ID)
		where += " AND p.author_user_id=$1"
	}
	switch status {
	case "", "all":
	case "draft":
		where += " AND p.deleted_at_ms IS NULL AND pub.post_id IS NULL"
	case "published":
		where += " AND p.deleted_at_ms IS NULL AND pub.post_id IS NOT NULL"
	case "deleted", "trashed":
		where += " AND p.deleted_at_ms IS NOT NULL"
	default:
		return E(400, "VALIDATION_FAILED", "状态筛选无效")
	}
	query := strings.TrimSpace(q.Get("q"))
	if utf8.RuneCountInString(query) > 64 {
		return E(400, "VALIDATION_FAILED", "搜索词最多64个字符")
	}
	if query != "" {
		args = append(args, searchPattern(query))
		n := len(args)
		where += fmt.Sprintf(" AND (d.title ILIKE $%d ESCAPE '!' OR d.summary ILIKE $%d ESCAPE '!')", n, n)
	}
	scope := hashString(strings.Join([]string{"studio-posts", p.ID, q.Get("scope"), status, query, strconv.Itoa(limit)}, "\x00"))
	if cursor := q.Get("cursor"); cursor != "" {
		var page contentListPage
		if e = a.decodePage(cursor, &page); e != nil {
			return e
		}
		if page.Scope != scope || contentID(page.ID) != nil {
			return E(400, "VALIDATION_FAILED", "分页条件已改变，请重新加载")
		}
		args = append(args, page.ID)
		where += fmt.Sprintf(" AND p.id<$%d::bigint", len(args))
	}
	args = append(args, limit+1)
	items, e := contentObjects(r.Context(), a.Pool, contentPrivateSQL(false)+where+fmt.Sprintf(" ORDER BY p.id DESC LIMIT $%d", len(args)), args...)
	if e != nil {
		return e
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	next := ""
	if more {
		next = a.encodePage(contentListPage{Scope: scope, ID: items[len(items)-1]["id"].(string)})
	}
	JSON(w, 200, map[string]any{"items": items, "next_cursor": next, "has_more": more})
	return nil
}
func (a *App) byClientDraft(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r, "author", "owner")
	if e != nil {
		return e
	}
	var id string
	e = a.Pool.QueryRow(r.Context(), "SELECT id::text FROM posts WHERE author_user_id=$1 AND client_draft_id=$2", p.ID, r.PathValue("client_draft_id")).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return NotFound()
	}
	if e != nil {
		return e
	}
	v, e := a.privatePost(r.Context(), a.Pool, p, id)
	if e == nil {
		JSON(w, 200, v)
	}
	return e
}

func contentExpected(r *http.Request) (int64, error) {
	s := strings.TrimSpace(r.Header.Get("If-Match"))
	if s == "" {
		return 0, E(428, "PRECONDITION_REQUIRED", "请先读取文章版本")
	}
	n, e := strconv.ParseInt(strings.Trim(s, "\""), 10, 64)
	if e != nil || n < 1 {
		return 0, E(400, "VALIDATION_FAILED", "If-Match版本无效")
	}
	return n, nil
}
func contentConflict() error {
	return E(409, "VERSION_CONFLICT", "文章已被其他操作修改，请保留本地内容并重新读取版本")
}
func contentLock(ctx context.Context, tx pgx.Tx, p Principal, id string, expected int64, deleted bool) (contentState, error) {
	var s contentState
	if contentID(id) != nil {
		return s, NotFound()
	}
	e := tx.QueryRow(ctx, "SELECT id::text,author_user_id::text,version,deleted_at_ms IS NOT NULL FROM posts WHERE id=$1 FOR UPDATE", id).Scan(&s.ID, &s.Author, &s.Version, &s.Deleted)
	if errors.Is(e, pgx.ErrNoRows) {
		return s, NotFound()
	}
	if e != nil {
		return s, e
	}
	if s.Author != p.ID && p.Role != "owner" {
		return s, NotFound()
	}
	if s.Deleted && !deleted {
		return s, NotFound()
	}
	if expected > 0 && s.Version != expected {
		return s, contentConflict()
	}
	return s, nil
}
func contentIDs(ids []string) ([]string, error) {
	v := append([]string{}, ids...)
	sort.Slice(v, func(i, j int) bool {
		a, _ := strconv.ParseInt(v[i], 10, 64)
		b, _ := strconv.ParseInt(v[j], 10, 64)
		return a < b
	})
	for i, x := range v {
		if contentID(x) != nil {
			return nil, E(400, "VALIDATION_FAILED", "标签ID无效")
		}
		if i > 0 && v[i-1] == x {
			return nil, E(400, "VALIDATION_FAILED", "标签不可重复")
		}
	}
	return v, nil
}

func contentCanonicalize(ctx context.Context, tx pgx.Tx, d *draftInput) error {
	ids := append([]string{}, d.TagIDs...)
	if d.CategoryID != nil {
		var tag string
		e := tx.QueryRow(ctx, "SELECT tag_id::text FROM category_tag_map WHERE category_id=$1", *d.CategoryID).Scan(&tag)
		if errors.Is(e, pgx.ErrNoRows) {
			return E(400, "VALIDATION_FAILED", "旧分类不存在，请选择标签")
		}
		if e != nil {
			return e
		}
		ids = append(ids, tag)
		d.CategoryID = nil
	}
	unique := map[string]bool{}
	for _, id := range ids {
		var canonical string
		if e := tx.QueryRow(ctx, "SELECT COALESCE((SELECT canonical_tag_id FROM tag_aliases WHERE tag_id=$1),$1::bigint)::text", id).Scan(&canonical); e != nil {
			return e
		}
		unique[canonical] = true
	}
	d.TagIDs = []string{}
	for id := range unique {
		d.TagIDs = append(d.TagIDs, id)
	}
	var e error
	d.TagIDs, e = contentIDs(d.TagIDs)
	return e
}

func draftContentHash(d draftInput) string {
	return hashString(contentHash(d.Title, d.Summary, d.Markdown, d.CategoryID, d.TagIDs, d.MediaIDs) + "\x00" + string(coverJSON(d.Cover)) + "\x00" + string(dateDisplayJSON(d.DateDisplay)))
}
func validateDraft(d *draftInput) (Rendered, error) {
	if e := normalizeCover(&d.Cover); e != nil {
		return Rendered{}, e
	}
	if utf8.RuneCountInString(d.Title) > 200 || utf8.RuneCountInString(d.Summary) > 500 {
		return Rendered{}, E(400, "VALIDATION_FAILED", "标题最多200字，摘要最多500字")
	}
	if len(d.TagIDs) > 50 {
		return Rendered{}, E(400, "VALIDATION_FAILED", "最多50个标签")
	}
	if d.CategoryID != nil {
		if *d.CategoryID == "" {
			d.CategoryID = nil
		} else if e := contentID(*d.CategoryID); e != nil {
			return Rendered{}, e
		}
	}
	tags, e := contentIDs(d.TagIDs)
	if e != nil {
		return Rendered{}, e
	}
	d.TagIDs = tags
	rendered, e := RenderMarkdown(d.Markdown)
	if e != nil {
		return rendered, e
	}
	sort.Strings(rendered.MediaIDs)
	sort.Strings(d.MediaIDs)
	// An omitted media_ids means derive from the source; an explicit set must be exact.
	if d.MediaIDs != nil && !reflect.DeepEqual(d.MediaIDs, rendered.MediaIDs) {
		return rendered, E(400, "VALIDATION_FAILED", "图片引用必须与正文一致")
	}
	d.MediaIDs = rendered.MediaIDs
	return rendered, nil
}
func contentLockRefs(ctx context.Context, tx pgx.Tx, p Principal, d draftInput, active ...bool) error {
	requireActive := len(active) == 0 || active[0]
	for _, id := range d.TagIDs {
		var archived bool
		e := tx.QueryRow(ctx, "SELECT archived_at_ms IS NOT NULL FROM tags WHERE id=$1 FOR SHARE", id).Scan(&archived)
		if errors.Is(e, pgx.ErrNoRows) || (archived && requireActive) {
			return E(400, "VALIDATION_FAILED", "标签不存在或已归档")
		}
		if e != nil {
			return e
		}
	}
	for _, id := range draftMediaIDs(d) {
		var owner, state string
		e := tx.QueryRow(ctx, "SELECT owner_user_id::text,state FROM media_assets WHERE id=$1 FOR UPDATE", id).Scan(&owner, &state)
		if errors.Is(e, pgx.ErrNoRows) {
			return E(409, "MEDIA_NOT_READY", "图片不可用")
		}
		if e != nil {
			return e
		}
		allowed := owner == p.ID || p.Role == "owner"
		if !allowed && d.PostID != "" {
			if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM posts p WHERE p.id=$1 AND p.author_user_id=$2 AND (EXISTS(SELECT 1 FROM post_draft_media_refs WHERE post_id=p.id AND media_id=$3) OR EXISTS(SELECT 1 FROM post_revisions r JOIN post_revision_media_refs mr ON mr.revision_id=r.id WHERE r.post_id=p.id AND mr.media_id=$3)))`, d.PostID, p.ID, id).Scan(&allowed); e != nil {
				return e
			}
		}
		if state != "ready" || !allowed {
			return E(409, "MEDIA_NOT_READY", "图片未完成上传或没有引用权限")
		}
	}
	return nil
}
func contentSetRelations(ctx context.Context, tx pgx.Tx, id string, d draftInput) error {
	if _, e := tx.Exec(ctx, "DELETE FROM post_draft_tags WHERE post_id=$1", id); e != nil {
		return e
	}
	for _, tag := range d.TagIDs {
		if _, e := tx.Exec(ctx, "INSERT INTO post_draft_tags(post_id,tag_id) VALUES($1,$2)", id, tag); e != nil {
			return e
		}
	}
	if _, e := tx.Exec(ctx, "DELETE FROM post_draft_media_refs WHERE post_id=$1", id); e != nil {
		return e
	}
	for _, media := range draftMediaIDs(d) {
		if _, e := tx.Exec(ctx, "INSERT INTO post_draft_media_refs(post_id,media_id) VALUES($1,$2)", id, media); e != nil {
			return e
		}
	}
	return nil
}
func contentAudit(ctx context.Context, tx pgx.Tx, p Principal, action, id string) error {
	_, e := tx.Exec(ctx, "INSERT INTO audit_logs(actor_user_id,action,target_type,target_id,request_id,created_at_ms) VALUES($1,$2,'post',$3,$4,$5)", p.ID, action, id, RequestID(ctx), NowMillis())
	return e
}
func contentEvent(ctx context.Context, tx pgx.Tx, p Principal, topic, id string, version int64, revision string) error {
	eventID := RandID()
	payload, _ := json.Marshal(map[string]any{"event_id": eventID, "schema_version": 1, "event_type": topic, "aggregate_type": "post", "aggregate_id": id, "aggregate_version": version, "actor_id": p.ID, "request_id": RequestID(ctx), "revision_id": revision, "occurred_at_ms": NowMillis()})
	_, e := tx.Exec(ctx, "INSERT INTO outbox(id,topic,aggregate_id,dedupe_key,payload_json,available_at_ms,created_at_ms) VALUES($1,$2,$3,$4,$5,$6,$6)", eventID, topic, id, fmt.Sprintf("%s:%s:%d", topic, id, version), payload, NowMillis())
	return e
}
func contentAdvance(ctx context.Context, tx pgx.Tx, id string) (int64, error) {
	var v int64
	e := tx.QueryRow(ctx, "UPDATE posts SET version=version+1,updated_at_ms=$2 WHERE id=$1 RETURNING version", id, NowMillis()).Scan(&v)
	return v, e
}
func contentAck(id string, v, dv int64, r *http.Request) map[string]any {
	return map[string]any{"id": id, "post_id": id, "version": v, "draft_version": dv, "ack_mutation_id": r.Header.Get("Idempotency-Key"), "saved_at": time.Now().UTC().Format(time.RFC3339Nano)}
}

func (a *App) createPost(w http.ResponseWriter, r *http.Request) error {
	var d draftInput
	if e := Decode(r, &d); e != nil {
		return e
	}
	if d.ClientDraftID == "" || len(d.ClientDraftID) > 128 {
		return E(400, "VALIDATION_FAILED", "client_draft_id必填且最多128字符")
	}
	if _, e := validateDraft(&d); e != nil {
		return e
	}
	result, e := a.Write(r, []string{"author", "owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		ctx := r.Context()
		var id string
		found := tx.QueryRow(ctx, "SELECT id::text FROM posts WHERE author_user_id=$1 AND client_draft_id=$2 FOR UPDATE", p.ID, d.ClientDraftID).Scan(&id)
		if found == nil {
			return a.privatePost(ctx, tx, p, id)
		}
		if !errors.Is(found, pgx.ErrNoRows) {
			return nil, found
		}
		e := tx.QueryRow(ctx, "INSERT INTO posts(author_user_id,client_draft_id,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$3) ON CONFLICT(author_user_id,client_draft_id) DO NOTHING RETURNING id::text", p.ID, d.ClientDraftID, NowMillis()).Scan(&id)
		if errors.Is(e, pgx.ErrNoRows) {
			if e = tx.QueryRow(ctx, "SELECT id::text FROM posts WHERE author_user_id=$1 AND client_draft_id=$2", p.ID, d.ClientDraftID).Scan(&id); e != nil {
				return nil, e
			}
			return a.privatePost(ctx, tx, p, id)
		}
		if e != nil {
			return nil, e
		}
		d.PostID = id
		if e := contentCanonicalize(ctx, tx, &d); e != nil {
			return nil, e
		}
		if e := contentLockRefs(ctx, tx, p, d); e != nil {
			return nil, e
		}
		_, e = tx.Exec(ctx, "INSERT INTO post_drafts(post_id,title,summary,markdown,category_id,updated_by,updated_at_ms,cover,date_display) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)", id, d.Title, d.Summary, d.Markdown, d.CategoryID, p.ID, NowMillis(), coverJSON(d.Cover), dateDisplayJSON(d.DateDisplay))
		if e != nil {
			return nil, e
		}
		if e = contentSetRelations(ctx, tx, id, d); e != nil {
			return nil, e
		}
		if e = contentAudit(ctx, tx, p, "post.create", id); e != nil {
			return nil, e
		}
		if e = contentEvent(ctx, tx, p, "post.created.v1", id, 1, ""); e != nil {
			return nil, e
		}
		return contentAck(id, 1, 1, r), nil
	})
	if e == nil {
		JSON(w, 201, result)
	}
	return e
}
func (a *App) saveDraft(w http.ResponseWriter, r *http.Request) error {
	var d draftInput
	if e := Decode(r, &d); e != nil {
		return e
	}
	if d.ExpectedDraftVersion < 1 {
		return E(400, "VALIDATION_FAILED", "expected_draft_version必填")
	}
	if _, e := validateDraft(&d); e != nil {
		return e
	}
	expected, e := contentExpected(r)
	if e != nil {
		return e
	}
	id := r.PathValue("id")
	d.PostID = id
	result, e := a.Write(r, []string{"author", "owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		ctx := r.Context()
		if _, e := contentLock(ctx, tx, p, id, expected, false); e != nil {
			return nil, e
		}
		var dv int64
		if e := tx.QueryRow(ctx, "SELECT draft_version FROM post_drafts WHERE post_id=$1 FOR UPDATE", id).Scan(&dv); e != nil {
			return nil, e
		}
		if dv != d.ExpectedDraftVersion {
			return nil, contentConflict()
		}
		if e := contentCanonicalize(ctx, tx, &d); e != nil {
			return nil, e
		}
		if e := contentLockRefs(ctx, tx, p, d); e != nil {
			return nil, e
		}
		e := tx.QueryRow(ctx, "UPDATE post_drafts SET title=$2,summary=$3,markdown=$4,category_id=$5,updated_by=$6,updated_at_ms=$7,draft_version=draft_version+1,cover=$9,date_display=COALESCE($10::jsonb,date_display) WHERE post_id=$1 AND draft_version=$8 RETURNING draft_version", id, d.Title, d.Summary, d.Markdown, d.CategoryID, p.ID, NowMillis(), dv, coverJSON(d.Cover), optionalDateDisplayJSON(d.DateDisplay)).Scan(&dv)
		if errors.Is(e, pgx.ErrNoRows) {
			return nil, contentConflict()
		}
		if e != nil {
			return nil, e
		}
		if e = contentSetRelations(ctx, tx, id, d); e != nil {
			return nil, e
		}
		v, e := contentAdvance(ctx, tx, id)
		if e != nil {
			return nil, e
		}
		if e = contentAudit(ctx, tx, p, "post.save", id); e != nil {
			return nil, e
		}
		return contentAck(id, v, dv, r), nil
	})
	if e == nil {
		JSON(w, 200, result)
	}
	return e
}

func draftFromMap(v map[string]any) (draftInput, error) {
	raw, e := json.Marshal(v)
	if e != nil {
		return draftInput{}, e
	}
	var d draftInput
	e = json.Unmarshal(raw, &d)
	return d, e
}
func (a *App) prepareSnapshot(r *http.Request, id string, requireTitle bool) (draftInput, Rendered, int64, error) {
	p, e := a.Require(r, "author", "owner")
	if e != nil {
		return draftInput{}, Rendered{}, 0, e
	}
	v, e := a.privatePost(r.Context(), a.Pool, p, id)
	if e != nil {
		return draftInput{}, Rendered{}, 0, e
	}
	d, e := draftFromMap(v)
	d.PostID = id
	if e != nil {
		return d, Rendered{}, 0, e
	}
	rendered, e := validateDraft(&d)
	if e != nil {
		return d, rendered, 0, e
	}
	if requireTitle && strings.TrimSpace(d.Title) == "" {
		return d, rendered, 0, E(400, "VALIDATION_FAILED", "创建修订前请填写标题")
	}
	return d, rendered, int64(v["draft_version"].(float64)), nil
}
func (a *App) verifyMediaFiles(ctx context.Context, d draftInput) error {
	for _, id := range draftMediaIDs(d) {
		var key, hash, state string
		e := a.Pool.QueryRow(ctx, "SELECT storage_key,COALESCE(sha256,''),state FROM media_assets WHERE id=$1", id).Scan(&key, &hash, &state)
		if e != nil {
			return E(409, "MEDIA_NOT_READY", "图片不可用")
		}
		if state != "ready" {
			return E(409, "MEDIA_NOT_READY", "图片尚未完成")
		}
		file, e := os.Open(filepath.Join(a.Config.MediaDir, "objects", key))
		if e != nil {
			return E(503, "STORAGE_UNAVAILABLE", "图片文件缺失，请恢复后再发布")
		}
		sum := sha256.New()
		_, e = io.Copy(sum, file)
		file.Close()
		if e != nil || hex.EncodeToString(sum.Sum(nil)) != hash {
			return E(503, "STORAGE_UNAVAILABLE", "图片完整性检查未通过")
		}
	}
	return nil
}
func contentMakeSnapshot(ctx context.Context, tx pgx.Tx, p Principal, id, reason string, d draftInput, rendered Rendered, dv int64) (string, error) {
	d.PostID = id
	if e := contentCanonicalize(ctx, tx, &d); e != nil {
		return "", e
	}
	if e := contentLockRefs(ctx, tx, p, d, reason == "publish"); e != nil {
		return "", e
	}
	rid := RandID()
	var editedAt int64
	if e := tx.QueryRow(ctx, "SELECT updated_at_ms FROM post_drafts WHERE post_id=$1", id).Scan(&editedAt); e != nil {
		return "", e
	}
	var revisionNo int64
	e := tx.QueryRow(ctx, "SELECT COALESCE(MAX(revision_no),0)+1 FROM post_revisions WHERE post_id=$1", id).Scan(&revisionNo)
	if e != nil {
		return "", e
	}
	for _, tag := range d.TagIDs {
		if _, e = tx.Exec(ctx, "INSERT INTO post_revision_tags(revision_id,tag_id,tag_name) SELECT $1,id,name FROM tags WHERE id=$2", rid, tag); e != nil {
			return "", e
		}
	}
	for _, media := range draftMediaIDs(d) {
		if _, e = tx.Exec(ctx, "INSERT INTO post_revision_media_refs(revision_id,media_id) VALUES($1,$2)", rid, media); e != nil {
			return "", e
		}
	}
	toc, _ := json.Marshal(rendered.TOC)
	var snapshotEdited any = editedAt
	if d.UnknownEditedAt {
		snapshotEdited = nil
	}
	_, e = tx.Exec(ctx, `INSERT INTO post_revisions(id,post_id,revision_no,source_draft_version,reason,title,summary,markdown,sanitized_html,toc_json,plain_text,renderer_version,content_hash,search_title,search_summary,search_body,category_id,category_name,created_by,created_at_ms,cover,date_display,content_updated_at_ms)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,NULL,NULL,$17,$18,$19,$20,$21)`, rid, id, revisionNo, dv, reason, d.Title, d.Summary, d.Markdown, rendered.HTML, toc, rendered.Text, RendererVersion, draftContentHash(d), Normalize(d.Title), Normalize(d.Summary), Normalize(rendered.Text), p.ID, NowMillis(), coverJSON(d.Cover), dateDisplayJSON(d.DateDisplay), snapshotEdited)
	return rid, e
}
func (a *App) snapshotPost(w http.ResponseWriter, r *http.Request, publish bool) error {
	var input struct {
		ExpectedDraftVersion int64 `json:"expected_draft_version"`
	}
	if e := Decode(r, &input); e != nil {
		return e
	}
	if input.ExpectedDraftVersion < 1 {
		return E(400, "VALIDATION_FAILED", "expected_draft_version必填")
	}
	expected, e := contentExpected(r)
	if e != nil {
		return e
	}
	id := r.PathValue("id")
	if cached, found, replayErr := a.Replay(r, "author", "owner"); replayErr != nil {
		return replayErr
	} else if found {
		JSON(w, 200, cached)
		return nil
	}
	d, rendered, dv, e := a.prepareSnapshot(r, id, true)
	if e != nil {
		return e
	}
	if dv != input.ExpectedDraftVersion {
		return contentConflict()
	}
	if publish {
		if e = a.verifyMediaFiles(r.Context(), d); e != nil {
			return e
		}
	}
	result, e := a.Write(r, []string{"author", "owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		ctx := r.Context()
		if _, e := contentLock(ctx, tx, p, id, expected, false); e != nil {
			return nil, e
		}
		var latest int64
		if e := tx.QueryRow(ctx, "SELECT draft_version FROM post_drafts WHERE post_id=$1 FOR UPDATE", id).Scan(&latest); e != nil {
			return nil, e
		}
		if latest != dv {
			return nil, contentConflict()
		}
		reason := "checkpoint"
		if publish {
			reason = "publish"
		}
		rid, e := contentMakeSnapshot(ctx, tx, p, id, reason, d, rendered, dv)
		if e != nil {
			return nil, e
		}
		var v int64
		var advanceErr error
		if publish {
			advanceErr = tx.QueryRow(ctx, "UPDATE posts SET version=version+1,updated_at_ms=$2,first_published_at_ms=COALESCE(first_published_at_ms,$2) WHERE id=$1 RETURNING version", id, NowMillis()).Scan(&v)
		} else {
			v, advanceErr = contentAdvance(ctx, tx, id)
		}
		if advanceErr != nil {
			return nil, advanceErr
		}
		action := "post.checkpoint"
		if publish {
			_, e = tx.Exec(ctx, "INSERT INTO post_publications(post_id,revision_id,published_by,published_at_ms) VALUES($1,$2,$3,$4) ON CONFLICT(post_id) DO UPDATE SET revision_id=EXCLUDED.revision_id,published_by=EXCLUDED.published_by,published_at_ms=EXCLUDED.published_at_ms", id, rid, p.ID, NowMillis())
			if e != nil {
				return nil, e
			}
			action = "post.publish"
			if e = contentEvent(ctx, tx, p, "post.published.v1", id, v, rid); e != nil {
				return nil, e
			}
		}
		if e = contentAudit(ctx, tx, p, action, id); e != nil {
			return nil, e
		}
		out := contentAck(id, v, dv, r)
		out["revision_id"] = rid
		if publish {
			out["published_url"] = "/posts/" + id
		}
		return out, nil
	})
	if e == nil {
		JSON(w, 200, result)
	}
	return e
}
func (a *App) changePostState(w http.ResponseWriter, r *http.Request, action string) error {
	expected, e := contentExpected(r)
	if e != nil {
		return e
	}
	id := r.PathValue("id")
	result, e := a.Write(r, []string{"author", "owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		ctx := r.Context()
		s, e := contentLock(ctx, tx, p, id, expected, action == "recover" || action == "trash")
		if e != nil {
			return nil, e
		}
		auditAction := "post.unpublish"
		var topic string
		switch action {
		case "unpublish":
			tag, e := tx.Exec(ctx, "DELETE FROM post_publications WHERE post_id=$1", id)
			if e != nil {
				return nil, e
			}
			if tag.RowsAffected() > 0 {
				topic = "post.unpublished.v1"
			}
		case "trash":
			if !s.Deleted {
				_, e = tx.Exec(ctx, "UPDATE posts SET deleted_at_ms=$2,updated_at_ms=$2,version=version+1 WHERE id=$1", id, NowMillis())
				if e != nil {
					return nil, e
				}
				s.Version++
				topic = "post.trashed.v1"
			}
			auditAction = "post.trash"
		case "recover":
			if !s.Deleted {
				return nil, E(409, "VERSION_CONFLICT", "文章不在回收站")
			}
			_, e = tx.Exec(ctx, "UPDATE posts SET deleted_at_ms=NULL,updated_at_ms=$2,version=version+1 WHERE id=$1", id, NowMillis())
			if e != nil {
				return nil, e
			}
			s.Version++
			auditAction = "post.restore"
		}
		if action == "unpublish" {
			s.Version, e = contentAdvance(ctx, tx, id)
			if e != nil {
				return nil, e
			}
		}
		if e = contentAudit(ctx, tx, p, auditAction, id); e != nil {
			return nil, e
		}
		if topic != "" {
			if e = contentEvent(ctx, tx, p, topic, id, s.Version, ""); e != nil {
				return nil, e
			}
		}
		var dv int64
		if e = tx.QueryRow(ctx, "SELECT draft_version FROM post_drafts WHERE post_id=$1", id).Scan(&dv); e != nil {
			return nil, e
		}
		return contentAck(id, s.Version, dv, r), nil
	})
	if e == nil {
		JSON(w, 200, result)
	}
	return e
}
func (a *App) listRevisions(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r, "author", "owner")
	if e != nil {
		return e
	}
	id := r.PathValue("id")
	if _, e = a.privatePost(r.Context(), a.Pool, p, id); e != nil {
		return e
	}
	limit := contentLimit(r.URL.Query())
	scope := hashString(strings.Join([]string{"revisions", p.ID, id, strconv.Itoa(limit)}, "\x00"))
	cursor := int64(1 << 62)
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		var page contentListPage
		if e = a.decodePage(raw, &page); e != nil {
			return e
		}
		if page.Scope != scope || page.Timestamp < 1 {
			return E(400, "VALIDATION_FAILED", "修订分页游标无效")
		}
		cursor = page.Timestamp
	}
	items, e := contentObjects(r.Context(), a.Pool, `SELECT jsonb_build_object('id',id,'revision_id',id,'revision_no',revision_no,'title',title,'reason',reason,'created_at',`+contentTimeSQL("created_at_ms")+`,'created_by',created_by::text,'renderer_version',renderer_version,'content_hash',content_hash) FROM post_revisions WHERE post_id=$1 AND revision_no<$2 ORDER BY revision_no DESC LIMIT $3`, id, cursor, limit+1)
	if e != nil {
		return e
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	next := ""
	if more {
		next = a.encodePage(contentListPage{Scope: scope, Timestamp: int64(items[len(items)-1]["revision_no"].(float64))})
	}
	JSON(w, 200, map[string]any{"items": items, "has_more": more, "next_cursor": next})
	return nil
}
func (a *App) getRevision(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r, "author", "owner")
	if e != nil {
		return e
	}
	id := r.PathValue("id")
	if _, e = a.privatePost(r.Context(), a.Pool, p, id); e != nil {
		return e
	}
	v, e := contentObject(r.Context(), a.Pool, `SELECT jsonb_build_object('id',id,'revision_id',id,'post_id',post_id::text,'revision_no',revision_no,'title',title,'summary',summary,'markdown',markdown,'sanitized_html',sanitized_html,'toc',toc_json,'reason',reason,'cover',cover,'date_display',date_display,'updated_at',`+contentTimeSQL("content_updated_at_ms")+`,'tag_ids',COALESCE((SELECT jsonb_agg(tag_id::text ORDER BY tag_id) FROM effective_revision_tags WHERE revision_id=r.id),'[]'::jsonb),'media_ids',COALESCE((SELECT jsonb_agg(media_id ORDER BY media_id) FROM post_revision_media_refs WHERE revision_id=r.id),'[]'::jsonb),'created_at',`+contentTimeSQL("created_at_ms")+`) FROM post_revisions r WHERE post_id=$1 AND id=$2`, id, r.PathValue("rid"))
	if e == nil {
		htmlValue, _ := v["sanitized_html"].(string)
		expanded, err := a.ExpandInternalContentCards(r.Context(), htmlValue)
		if err != nil {
			return err
		}
		v["sanitized_html"] = expanded
		JSON(w, 200, v)
	}
	return e
}
func (a *App) restoreRevision(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		RevisionID string `json:"revision_id"`
	}
	if e := Decode(r, &input); e != nil {
		return e
	}
	expected, e := contentExpected(r)
	if e != nil {
		return e
	}
	id := r.PathValue("id")
	if cached, found, replayErr := a.Replay(r, "author", "owner"); replayErr != nil {
		return replayErr
	} else if found {
		JSON(w, 200, cached)
		return nil
	}
	current, rendered, dv, e := a.prepareSnapshot(r, id, false)
	if e != nil {
		return e
	}
	result, e := a.Write(r, []string{"author", "owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		ctx := r.Context()
		if _, e := contentLock(ctx, tx, p, id, expected, false); e != nil {
			return nil, e
		}
		var latest int64
		if e := tx.QueryRow(ctx, "SELECT draft_version FROM post_drafts WHERE post_id=$1 FOR UPDATE", id).Scan(&latest); e != nil {
			return nil, e
		}
		if latest != dv {
			return nil, contentConflict()
		}
		restored, e := contentObject(ctx, tx, `SELECT jsonb_build_object('title',title,'summary',summary,'markdown',markdown,'cover',cover,'date_display',date_display,'tag_ids',COALESCE((SELECT jsonb_agg(tag_id::text ORDER BY tag_id) FROM effective_revision_tags WHERE revision_id=r.id),'[]'::jsonb),'media_ids',COALESCE((SELECT jsonb_agg(media_id ORDER BY media_id) FROM post_revision_media_refs WHERE revision_id=r.id),'[]'::jsonb)) FROM post_revisions r WHERE post_id=$1 AND id=$2`, id, input.RevisionID)
		if e != nil {
			return nil, e
		}
		d, e := draftFromMap(restored)
		if e != nil {
			return nil, e
		}
		d.PostID = id
		if e = contentRestoreLocks(ctx, tx, p, id, current, d); e != nil {
			return nil, e
		}
		if _, e = contentMakeSnapshot(ctx, tx, p, id, "before_restore", current, rendered, dv); e != nil {
			return nil, e
		}
		if e = contentLockRefs(ctx, tx, p, d, false); e != nil {
			return nil, e
		}
		e = tx.QueryRow(ctx, "UPDATE post_drafts SET title=$2,summary=$3,markdown=$4,category_id=$5,updated_by=$6,updated_at_ms=$7,draft_version=draft_version+1,cover=$8,date_display=$9 WHERE post_id=$1 RETURNING draft_version", id, d.Title, d.Summary, d.Markdown, d.CategoryID, p.ID, NowMillis(), coverJSON(d.Cover), dateDisplayJSON(d.DateDisplay)).Scan(&dv)
		if e != nil {
			return nil, e
		}
		if e = contentSetRelations(ctx, tx, id, d); e != nil {
			return nil, e
		}
		v, e := contentAdvance(ctx, tx, id)
		if e != nil {
			return nil, e
		}
		if e = contentAudit(ctx, tx, p, "post.restore", id); e != nil {
			return nil, e
		}
		return contentAck(id, v, dv, r), nil
	})
	if e == nil {
		JSON(w, 200, result)
	}
	return e
}
func (a *App) previewMarkdown(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Markdown string `json:"markdown"`
		Seq      int64  `json:"seq"`
	}
	if e := Decode(r, &in); e != nil {
		return e
	}
	rendered, e := RenderMarkdown(in.Markdown)
	if e != nil {
		return e
	}
	result, e := a.Write(r, []string{"author", "owner"}, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		// Reuse the writer transaction; a second pool acquisition can deadlock
		// concurrent previews when each already holds a transaction connection.
		expanded, err := a.expandInternalContentCards(r.Context(), tx, rendered.HTML)
		if err != nil {
			return nil, err
		}
		return map[string]any{"sanitized_html": expanded, "toc": rendered.TOC, "content_hash": rendered.Hash, "seq": in.Seq, "renderer_version": RendererVersion}, nil
	})
	if e == nil {
		JSON(w, 200, result)
	}
	return e
}

// Acquire the entire restore set before either snapshot/reference write, so
// cross-article owner restores cannot lock media in opposite orders.
func contentRestoreLocks(ctx context.Context, tx pgx.Tx, p Principal, id string, current, target draftInput) error {
	tags := map[string]bool{}
	media := map[string]bool{}
	for _, d := range []draftInput{current, target} {
		for _, v := range d.TagIDs {
			tags[v] = true
		}
		for _, v := range draftMediaIDs(d) {
			media[v] = true
		}
	}
	d := draftInput{PostID: id}
	for id := range tags {
		d.TagIDs = append(d.TagIDs, id)
	}
	d.TagIDs, _ = contentIDs(d.TagIDs)
	for id := range media {
		d.MediaIDs = append(d.MediaIDs, id)
	}
	sort.Strings(d.MediaIDs)
	return contentLockRefs(ctx, tx, p, d, false)
}

func contentPrivateSQL(body bool) string {
	sql := `SELECT jsonb_build_object('id',p.id::text,'post_id',p.id::text,'version',p.version,'draft_version',d.draft_version,
 'title',d.title,'summary',d.summary,'markdown',d.markdown,'cover',d.cover,'date_display',d.date_display,'last_edited_at',` + contentTimeSQL("d.updated_at_ms") + `,'cover_url',CASE WHEN d.cover->>'mode'='none' THEN '' ELSE '/api/v1/studio/posts/'||p.id||'/cover' END,
 'status',CASE WHEN p.deleted_at_ms IS NOT NULL THEN 'trashed' WHEN pub.revision_id IS NOT NULL THEN 'published' ELSE 'draft' END,
 'author_id',p.author_user_id::text,'author_name',(SELECT display_name FROM users WHERE id=p.author_user_id),'created_at',` + contentTimeSQL("p.created_at_ms") + `,'updated_at',` + contentTimeSQL("p.updated_at_ms") + `,
 'publication',CASE WHEN pub.revision_id IS NULL THEN NULL ELSE jsonb_build_object('revision_id',pub.revision_id,'published_at',` + contentTimeSQL("pub.published_at_ms") + `) END,
 'tag_ids',COALESCE((SELECT jsonb_agg(tag_id::text ORDER BY tag_id) FROM effective_draft_tags WHERE post_id=p.id),'[]'::jsonb),
 'media_ids',COALESCE((SELECT jsonb_agg(media_id ORDER BY media_id) FROM post_draft_media_refs WHERE post_id=p.id),'[]'::jsonb))
 FROM posts p JOIN post_drafts d ON d.post_id=p.id LEFT JOIN post_publications pub ON pub.post_id=p.id`
	if !body {
		sql = strings.Replace(sql, ",'markdown',d.markdown", "", 1)
	}
	return sql
}

type contentListPage struct {
	Scope     string `json:"s"`
	ID        string `json:"i"`
	Timestamp int64  `json:"t"`
}
