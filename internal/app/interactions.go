package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Private collection cursors are authenticated and bound to their route/filter.
type interactionCursor struct {
	Scope string `json:"s"`
	Time  int64  `json:"t"`
	ID    string `json:"i"`
}

func (a *App) interactionCursor(r *http.Request, scope string) (interactionCursor, error) {
	c := interactionCursor{Scope: scope, ID: "0"}
	raw := r.URL.Query().Get("cursor")
	if raw == "" {
		return c, nil
	}
	p := strings.Split(raw, ".")
	if len(p) != 2 {
		return c, E(400, "VALIDATION_FAILED", "无效分页游标")
	}
	b, e := base64.RawURLEncoding.DecodeString(p[0])
	if e != nil {
		return c, E(400, "VALIDATION_FAILED", "无效分页游标")
	}
	sig, e := hex.DecodeString(p[1])
	if e != nil {
		return c, E(400, "VALIDATION_FAILED", "无效分页游标")
	}
	h := hmac.New(sha256.New, a.Config.EncryptionKey)
	h.Write(b)
	if !hmac.Equal(sig, h.Sum(nil)) || json.Unmarshal(b, &c) != nil || c.Scope != scope {
		return c, E(400, "VALIDATION_FAILED", "分页游标不匹配")
	}
	return c, nil
}
func (a *App) interactionNext(scope, id string, t int64) string {
	b, _ := json.Marshal(interactionCursor{scope, t, id})
	h := hmac.New(sha256.New, a.Config.EncryptionKey)
	h.Write(b)
	return base64.RawURLEncoding.EncodeToString(b) + "." + hex.EncodeToString(h.Sum(nil))
}
func interactionID(s string) error {
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil || n < 1 {
		return NotFound()
	}
	return nil
}
func interactionTime(ms int64) string { return time.UnixMilli(ms).UTC().Format(time.RFC3339Nano) }
func interactionPage(items []map[string]any, next string) map[string]any {
	return map[string]any{"items": items, "next_cursor": next, "has_more": next != ""}
}
func interactionAudit(tx pgx.Tx, r *http.Request, p Principal, action, kind, id, reason string) error {
	if utf8.RuneCountInString(reason) > 500 {
		return E(400, "VALIDATION_FAILED", "原因最多500字")
	}
	_, e := tx.Exec(r.Context(), `INSERT INTO audit_logs(actor_user_id,action,target_type,target_id,request_id,reason,created_at_ms) VALUES($1,$2,$3,$4,$5,$6,$7)`, p.ID, action, kind, id, RandID(), reason, NowMillis())
	return e
}
func interactionFact(tx pgx.Tx, r *http.Request, p Principal, topic, kind, id, dedupe string, payload map[string]any) error {
	eid := RandID()
	now := NowMillis()
	body := map[string]any{"event_id": eid, "schema_version": 1, "event_type": topic, "aggregate_type": kind, "aggregate_id": id, "aggregate_version": 0, "actor_id": p.ID, "occurred_at_ms": now, "payload": payload}
	b, _ := json.Marshal(body)
	_, e := tx.Exec(r.Context(), `INSERT INTO outbox(id,topic,aggregate_id,dedupe_key,payload_json,available_at_ms,created_at_ms) VALUES($1,$2,$3,$4,$5,$6,$6) ON CONFLICT(dedupe_key) DO NOTHING`, eid, topic, id, dedupe, b, now)
	return e
}
func interactionPublicPost(tx pgx.Tx, r *http.Request, id string) (string, error) {
	if e := interactionID(id); e != nil {
		return "", e
	}
	var author string
	var deleted *int64
	e := tx.QueryRow(r.Context(), `SELECT author_user_id::text,deleted_at_ms FROM posts WHERE id=$1 FOR UPDATE`, id).Scan(&author, &deleted)
	if e == pgx.ErrNoRows {
		return "", NotFound()
	}
	if e != nil {
		return "", e
	}
	var ok bool
	e = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM post_publications WHERE post_id=$1)`, id).Scan(&ok)
	if e != nil {
		return "", e
	}
	if !ok || deleted != nil {
		return "", NotFound()
	}
	return author, nil
}

func (a *App) RegisterInteractions() {
	a.Handle("GET /api/v1/users/{id}/follow-state", a.followState)
	a.Handle("GET /api/v1/me/post-states", a.interactionStates)
	for _, kind := range []string{"like", "bookmark"} {
		k := kind
		a.Handle("PUT /api/v1/posts/{id}/"+k, func(w http.ResponseWriter, r *http.Request) error { return a.interactionPostRelation(w, r, k, true) })
		a.Handle("DELETE /api/v1/posts/{id}/"+k, func(w http.ResponseWriter, r *http.Request) error { return a.interactionPostRelation(w, r, k, false) })
	}
	a.Handle("GET /api/v1/posts/{id}/comments", a.interactionComments)
	a.Handle("POST /api/v1/posts/{id}/comments", a.interactionCreateComment)
	a.Handle("DELETE /api/v1/comments/{id}", a.interactionDeleteComment)
	a.Handle("PUT /api/v1/comments/{id}/like", func(w http.ResponseWriter, r *http.Request) error { return a.interactionCommentLike(w, r, true) })
	a.Handle("DELETE /api/v1/comments/{id}/like", func(w http.ResponseWriter, r *http.Request) error { return a.interactionCommentLike(w, r, false) })
	a.Handle("PUT /api/v1/users/{id}/follow", func(w http.ResponseWriter, r *http.Request) error { return a.interactionFollow(w, r, true) })
	a.Handle("DELETE /api/v1/users/{id}/follow", func(w http.ResponseWriter, r *http.Request) error { return a.interactionFollow(w, r, false) })
	a.Handle("GET /api/v1/me/following", func(w http.ResponseWriter, r *http.Request) error { return a.interactionFollows(w, r, false) })
	a.Handle("GET /api/v1/me/followers", func(w http.ResponseWriter, r *http.Request) error { return a.interactionFollows(w, r, true) })
	a.Handle("GET /api/v1/me/bookmarks", func(w http.ResponseWriter, r *http.Request) error { return a.interactionSaved(w, r, false) })
	a.Handle("GET /api/v1/me/history", func(w http.ResponseWriter, r *http.Request) error { return a.interactionSaved(w, r, true) })
	a.Handle("DELETE /api/v1/me/history", func(w http.ResponseWriter, r *http.Request) error {
		_, e := a.Write(r, nil, false, false, func(tx pgx.Tx, p Principal) (any, error) {
			_, e := tx.Exec(r.Context(), `DELETE FROM reading_history WHERE user_id=$1`, p.ID)
			return nil, e
		})
		if e == nil {
			w.WriteHeader(204)
		}
		return e
	})
	a.Handle("GET /api/v1/posts/{id}/metrics", a.interactionMetrics)
}

// A follow button needs one relationship, never the reader's entire follow list.
func (a *App) followState(w http.ResponseWriter, r *http.Request) error {
	p, err := a.Require(r)
	if err != nil {
		return err
	}
	id := r.PathValue("id")
	if !positiveID(id) {
		return NotFound()
	}
	var followed, enabled bool
	err = a.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM follows WHERE follower_user_id=$1 AND followed_user_id=u.id),s.follows_enabled FROM users u CROSS JOIN site_settings s WHERE u.id=$2 AND u.disabled_at_ms IS NULL AND s.id=1`, p.ID, id).Scan(&followed, &enabled)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, map[string]any{"followed": followed, "follows_enabled": enabled})
	return nil
}
func (a *App) interactionStates(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r)
	if e != nil {
		return e
	}
	ids := strings.Split(r.URL.Query().Get("ids"), ",")
	if len(ids) > 50 {
		return E(400, "VALIDATION_FAILED", "最多查询50篇文章")
	}
	items := []map[string]any{}
	for _, id := range ids {
		if id == "" {
			continue
		}
		if e := interactionID(id); e != nil {
			return E(400, "VALIDATION_FAILED", "无效文章ID")
		}
		var liked, bookmarked bool
		e = a.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM post_likes WHERE post_id=$1 AND user_id=$2),EXISTS(SELECT 1 FROM bookmarks WHERE post_id=$1 AND user_id=$2) WHERE EXISTS(SELECT 1 FROM public_posts WHERE post_id=$1)`, id, p.ID).Scan(&liked, &bookmarked)
		if e == pgx.ErrNoRows {
			continue
		}
		if e != nil {
			return e
		}
		items = append(items, map[string]any{"post_id": id, "liked": liked, "bookmarked": bookmarked})
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, map[string]any{"items": items})
	return nil
}
func (a *App) interactionPostRelation(w http.ResponseWriter, r *http.Request, kind string, put bool) error {
	id := r.PathValue("id")
	_, e := a.Write(r, nil, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		if _, e := interactionPublicPost(tx, r, id); e != nil {
			return nil, e
		}
		table := "post_likes"
		if kind == "bookmark" {
			table = "bookmarks"
			var enabled bool
			if e := tx.QueryRow(r.Context(), `SELECT collections_enabled FROM site_settings WHERE id=1`).Scan(&enabled); e != nil {
				return nil, e
			}
			if !enabled && put {
				return nil, E(403, "FEATURE_DISABLED", "收藏功能已关闭")
			}
		}
		var err error
		if put {
			_, err = tx.Exec(r.Context(), `INSERT INTO `+table+`(post_id,user_id,created_at_ms) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, id, p.ID, NowMillis())
		} else {
			_, err = tx.Exec(r.Context(), `DELETE FROM `+table+` WHERE post_id=$1 AND user_id=$2`, id, p.ID)
		}
		return nil, err
	})
	if e == nil {
		w.WriteHeader(204)
	}
	return e
}
func (a *App) interactionComments(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if e := interactionID(id); e != nil {
		return e
	}
	var ok bool
	if e := a.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM public_posts WHERE post_id=$1)`, id).Scan(&ok); e != nil {
		return e
	}
	if !ok {
		return NotFound()
	}
	scope := "comments:" + id
	c, e := a.interactionCursor(r, scope)
	if e != nil {
		return e
	}
	p, _ := a.Principal(r)
	uid := p.ID
	if uid == "" {
		uid = "0"
	}
	rows, e := a.Pool.Query(r.Context(), `SELECT c.id::text,c.body,c.created_at_ms,u.id::text,u.display_name,u.avatar_url,(SELECT count(*) FROM comment_likes l WHERE l.comment_id=c.id),EXISTS(SELECT 1 FROM comment_likes l WHERE l.comment_id=c.id AND l.user_id=$4) FROM comments c JOIN users u ON u.id=c.user_id JOIN public_posts pp ON pp.post_id=c.post_id WHERE c.post_id=$1 AND c.state='visible' AND (c.created_at_ms,c.id)>($2,$3::bigint) ORDER BY c.created_at_ms,c.id LIMIT $5`, id, c.Time, c.ID, uid, Limit(r)+1)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []map[string]any{}
	var lastID string
	var lastTime int64
	next := ""
	for rows.Next() {
		var cid, body, userid, name, avatar string
		var ms, count int64
		var liked bool
		if e = rows.Scan(&cid, &body, &ms, &userid, &name, &avatar, &count, &liked); e != nil {
			return e
		}
		if len(items) == Limit(r) {
			next = a.interactionNext(scope, lastID, lastTime)
			break
		}
		items = append(items, map[string]any{"id": cid, "body": body, "user": map[string]any{"id": userid, "display_name": name, "avatar_url": avatar}, "created_at": interactionTime(ms), "like_count": count, "liked": liked, "is_self": p.ID == userid})
		lastID = cid
		lastTime = ms
	}
	if e = rows.Err(); e != nil {
		return e
	}
	JSON(w, 200, interactionPage(items, next))
	return nil
}
func (a *App) interactionCreateComment(w http.ResponseWriter, r *http.Request) error {
	var d struct {
		Body string `json:"body"`
	}
	if e := Decode(r, &d); e != nil {
		return e
	}
	d.Body = strings.TrimSpace(d.Body)
	if n := utf8.RuneCountInString(d.Body); n < 1 || n > 5000 {
		return E(400, "VALIDATION_FAILED", "评论需1至5000字")
	}
	id := r.PathValue("id")
	data, e := a.Write(r, nil, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		if _, e := interactionPublicPost(tx, r, id); e != nil {
			return nil, e
		}
		var enabled bool
		if e := tx.QueryRow(r.Context(), `SELECT comments_enabled FROM site_settings WHERE id=1`).Scan(&enabled); e != nil {
			return nil, e
		}
		if !enabled {
			return nil, E(403, "FEATURE_DISABLED", "评论已关闭")
		}
		var cid string
		now := NowMillis()
		if e := tx.QueryRow(r.Context(), `INSERT INTO comments(post_id,user_id,body,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$4,$4) RETURNING id::text`, id, p.ID, d.Body, now).Scan(&cid); e != nil {
			return nil, e
		}
		if e := interactionFact(tx, r, p, "comment.created.v1", "comment", cid, "comment.created:"+cid, map[string]any{"post_id": id}); e != nil {
			return nil, e
		}
		return map[string]any{"id": cid, "body": d.Body, "user": map[string]any{"id": p.ID, "display_name": p.DisplayName}, "created_at": interactionTime(now), "like_count": 0, "liked": false, "is_self": true}, nil
	})
	if e == nil {
		JSON(w, 201, data)
	}
	return e
}
func (a *App) interactionDeleteComment(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if e := interactionID(id); e != nil {
		return e
	}
	_, e := a.Write(r, nil, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		var post string
		if e := tx.QueryRow(r.Context(), `SELECT post_id::text FROM comments WHERE id=$1`, id).Scan(&post); e == pgx.ErrNoRows {
			return nil, NotFound()
		} else if e != nil {
			return nil, e
		}
		if _, e := tx.Exec(r.Context(), `SELECT 1 FROM posts WHERE id=$1 FOR UPDATE`, post); e != nil {
			return nil, e
		}
		var uid, state string
		if e := tx.QueryRow(r.Context(), `SELECT user_id::text,state FROM comments WHERE id=$1 AND post_id=$2 FOR UPDATE`, id, post).Scan(&uid, &state); e != nil {
			return nil, e
		}
		if uid != p.ID && p.Role != "owner" {
			return nil, NotFound()
		}
		if state == "deleted" {
			return nil, nil
		}
		if _, e := tx.Exec(r.Context(), `UPDATE comments SET state='deleted',updated_at_ms=$2 WHERE id=$1`, id, NowMillis()); e != nil {
			return nil, e
		}
		return nil, interactionAudit(tx, r, p, "comment.delete", "comment", id, "")
	})
	if e == nil {
		w.WriteHeader(204)
	}
	return e
}
func (a *App) interactionCommentLike(w http.ResponseWriter, r *http.Request, put bool) error {
	id := r.PathValue("id")
	if e := interactionID(id); e != nil {
		return e
	}
	_, e := a.Write(r, nil, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		var post string
		if e := tx.QueryRow(r.Context(), `SELECT post_id::text FROM comments WHERE id=$1`, id).Scan(&post); e == pgx.ErrNoRows {
			return nil, NotFound()
		} else if e != nil {
			return nil, e
		}
		if _, e := interactionPublicPost(tx, r, post); e != nil {
			return nil, e
		}
		var state string
		if e := tx.QueryRow(r.Context(), `SELECT state FROM comments WHERE id=$1 FOR UPDATE`, id).Scan(&state); e != nil {
			return nil, e
		}
		if state != "visible" {
			return nil, NotFound()
		}
		var err error
		if put {
			_, err = tx.Exec(r.Context(), `INSERT INTO comment_likes(comment_id,user_id,created_at_ms) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, id, p.ID, NowMillis())
		} else {
			_, err = tx.Exec(r.Context(), `DELETE FROM comment_likes WHERE comment_id=$1 AND user_id=$2`, id, p.ID)
		}
		return nil, err
	})
	if e == nil {
		w.WriteHeader(204)
	}
	return e
}
func (a *App) interactionFollow(w http.ResponseWriter, r *http.Request, put bool) error {
	id := r.PathValue("id")
	if e := interactionID(id); e != nil {
		return e
	}
	_, e := a.Write(r, nil, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		if id == p.ID {
			return nil, E(400, "VALIDATION_FAILED", "不能关注本人")
		}
		var disabled *int64
		if e := tx.QueryRow(r.Context(), `SELECT disabled_at_ms FROM users WHERE id=$1 FOR SHARE`, id).Scan(&disabled); e == pgx.ErrNoRows {
			return nil, NotFound()
		} else if e != nil {
			return nil, e
		}
		if disabled != nil && put {
			return nil, NotFound()
		}
		var enabled bool
		if e := tx.QueryRow(r.Context(), `SELECT follows_enabled FROM site_settings WHERE id=1`).Scan(&enabled); e != nil {
			return nil, e
		}
		if !enabled && put {
			return nil, E(403, "FEATURE_DISABLED", "关注已关闭")
		}
		var err error
		if put {
			_, err = tx.Exec(r.Context(), `INSERT INTO follows(follower_user_id,followed_user_id,created_at_ms) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, p.ID, id, NowMillis())
		} else {
			_, err = tx.Exec(r.Context(), `DELETE FROM follows WHERE follower_user_id=$1 AND followed_user_id=$2`, p.ID, id)
		}
		return nil, err
	})
	if e == nil {
		w.WriteHeader(204)
	}
	return e
}
func (a *App) interactionFollows(w http.ResponseWriter, r *http.Request, followers bool) error {
	p, e := a.Require(r)
	if e != nil {
		return e
	}
	scope := "following:" + p.ID
	target, filter := "followed_user_id", "follower_user_id"
	if followers {
		scope = "followers:" + p.ID
		target, filter = "follower_user_id", "followed_user_id"
	}
	c, e := a.interactionCursor(r, scope)
	if e != nil {
		return e
	}
	rows, e := a.Pool.Query(r.Context(), `SELECT u.id::text,u.display_name,u.avatar_url,f.created_at_ms,EXISTS(SELECT 1 FROM follows ff WHERE ff.follower_user_id=$1 AND ff.followed_user_id=u.id) FROM follows f JOIN users u ON u.id=f.`+target+` WHERE f.`+filter+`=$1 AND u.disabled_at_ms IS NULL AND ($2::bigint=0 OR (f.created_at_ms,u.id)<($2,$3::bigint)) ORDER BY f.created_at_ms DESC,u.id DESC LIMIT $4`, p.ID, c.Time, c.ID, Limit(r)+1)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []map[string]any{}
	next, last := "", ""
	var lastms int64
	for rows.Next() {
		var id, name, avatar string
		var ms int64
		var followed bool
		if e = rows.Scan(&id, &name, &avatar, &ms, &followed); e != nil {
			return e
		}
		if len(items) == Limit(r) {
			next = a.interactionNext(scope, last, lastms)
			break
		}
		items = append(items, map[string]any{"id": id, "display_name": name, "avatar_url": avatar, "followed": followed})
		last = id
		lastms = ms
	}
	if e = rows.Err(); e != nil {
		return e
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, interactionPage(items, next))
	return nil
}
func (a *App) interactionSaved(w http.ResponseWriter, r *http.Request, history bool) error {
	p, e := a.Require(r)
	if e != nil {
		return e
	}
	table, col, scope := "bookmarks", "created_at_ms", "bookmarks:"+p.ID
	if history {
		table, col, scope = "reading_history", "last_read_at_ms", "history:"+p.ID
	}
	c, e := a.interactionCursor(r, scope)
	if e != nil {
		return e
	}
	rows, e := a.Pool.Query(r.Context(), `SELECT pp.post_id::text,pp.title,pp.summary,pp.author_user_id::text,pp.published_at_ms,b.`+col+` FROM `+table+` b JOIN public_posts pp ON pp.post_id=b.post_id WHERE b.user_id=$1 AND ($2::bigint=0 OR (b.`+col+`,b.post_id)<($2,$3::bigint)) ORDER BY b.`+col+` DESC,b.post_id DESC LIMIT $4`, p.ID, c.Time, c.ID, Limit(r)+1)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []map[string]any{}
	next, last := "", ""
	var lastms int64
	for rows.Next() {
		var id, title, summary, author string
		var published, ms int64
		if e = rows.Scan(&id, &title, &summary, &author, &published, &ms); e != nil {
			return e
		}
		if len(items) == Limit(r) {
			next = a.interactionNext(scope, last, lastms)
			break
		}
		item := map[string]any{"post_id": id, "id": id, "title": title, "summary": summary, "author_id": author, "published_at": interactionTime(published), "created_at": interactionTime(ms)}
		if history {
			item["last_read_at"] = interactionTime(ms)
		}
		items = append(items, item)
		last = id
		lastms = ms
	}
	if e = rows.Err(); e != nil {
		return e
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, interactionPage(items, next))
	return nil
}
func (a *App) interactionMetrics(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if e := interactionID(id); e != nil {
		return e
	}
	var likes, comments, reads, legacy int64
	e := a.Pool.QueryRow(r.Context(), `SELECT (SELECT count(*) FROM post_likes WHERE post_id=pp.post_id),(SELECT count(*) FROM comments WHERE post_id=pp.post_id AND state='visible'),COALESCE((SELECT sum(post_reads) FROM analytics_daily WHERE post_id=pp.post_id),0),COALESCE((SELECT sum(count_value) FROM legacy_counters WHERE post_id=pp.post_id AND metric='article_read_count'),0) FROM public_posts pp WHERE pp.post_id=$1`, id).Scan(&likes, &comments, &reads, &legacy)
	if e == pgx.ErrNoRows {
		return NotFound()
	}
	if e != nil {
		return e
	}
	JSON(w, 200, map[string]any{"post_id": id, "like_count": likes, "comment_count": comments, "read_count": reads, "legacy_read_count": legacy, "read_count_source": "accepted_client_post_read_v1", "read_count_window_days": 400})
	return nil
}

// moduleWriteTx shares the maintenance gate used by HTTP commands and backup.
func (a *App) moduleWriteTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, `SELECT id FROM policy_barrier WHERE id=1 FOR SHARE`); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
