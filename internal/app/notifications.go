package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// The comment, recipient deduplication and inbox delivery commit together.
// When the article author is also the reply target, the more specific reply wins.
func commentNotifications(ctx context.Context, tx pgx.Tx, comment string) error {
	_, err := tx.Exec(ctx, `INSERT INTO notifications(recipient_user_id,comment_id,kind,created_at_ms)
 SELECT recipient,c.id,CASE WHEN bool_or(is_reply) THEN 'reply' ELSE 'comment' END,c.created_at_ms
 FROM comments c JOIN posts p ON p.id=c.post_id
 CROSS JOIN LATERAL (SELECT p.author_user_id AS recipient,false AS is_reply
 UNION ALL SELECT target.user_id,true FROM comments target WHERE target.id=c.reply_to_id) targets
 JOIN users recipient_user ON recipient_user.id=recipient AND recipient_user.disabled_at_ms IS NULL
 WHERE c.id=$1 AND recipient<>c.user_id GROUP BY recipient,c.id,c.created_at_ms
 ON CONFLICT(recipient_user_id,comment_id) DO NOTHING`, comment)
	return err
}

// A source or its reply context becoming unavailable removes every identifying
// field from the projection. The recipient can still dismiss the generic record.
const notificationJoins = ` FROM notifications n
 JOIN comments c ON c.id=n.comment_id
 JOIN users actor ON actor.id=c.user_id
 LEFT JOIN public_posts pp ON pp.post_id=c.post_id
 LEFT JOIN comments root ON root.id=c.parent_id AND root.post_id=c.post_id
 LEFT JOIN comments target ON target.id=c.reply_to_id AND target.post_id=c.post_id`
const notificationAvailable = `pp.post_id IS NOT NULL AND c.state='visible'
 AND actor.disabled_at_ms IS NULL AND (c.parent_id IS NULL OR root.state='visible')
 AND (c.reply_to_id IS NULL OR target.state='visible')`

func (a *App) notificationList(w http.ResponseWriter, r *http.Request) error {
	p, err := a.Require(r)
	if err != nil {
		return err
	}
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}
	if filter != "all" && filter != "unread" {
		return E(400, "VALIDATION_FAILED", "无效通知筛选")
	}
	limit := Limit(r)
	scope := fmt.Sprintf("notifications:%s:%s:%d", p.ID, filter, limit)
	cursor, err := a.interactionCursor(r, scope)
	if err != nil {
		return err
	}
	if cursor.ID != "0" && (!positiveID(cursor.ID) || cursor.Time < 0) {
		return E(400, "VALIDATION_FAILED", "无效分页游标")
	}
	tx, err := a.Pool.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var unread int64
	if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM notifications WHERE recipient_user_id=$1 AND read_at_ms IS NULL`, p.ID).Scan(&unread); err != nil {
		return err
	}
	where := ` WHERE n.recipient_user_id=$1`
	if filter == "unread" {
		where += ` AND n.read_at_ms IS NULL`
	}
	if cursor.ID != "0" {
		where += ` AND (n.created_at_ms,n.id)<($2,$3::bigint)`
	} else {
		where += ` AND ($2::bigint IS NOT NULL AND $3::bigint IS NOT NULL)`
	}
	rows, err := tx.Query(r.Context(), `SELECT n.id::text,n.kind,n.created_at_ms,n.read_at_ms,
 CASE WHEN (`+notificationAvailable+`) THEN jsonb_build_object(
 'actor',jsonb_build_object('id',actor.id::text,'display_name',actor.display_name),
 'post_title',pp.title,'body',left(c.body,240),'post_id',c.post_id::text,'comment_id',c.id::text) ELSE NULL END`+notificationJoins+where+` ORDER BY n.created_at_ms DESC,n.id DESC LIMIT $4`, p.ID, cursor.Time, cursor.ID, limit+1)
	if err != nil {
		return err
	}
	items := []map[string]any{}
	for rows.Next() {
		var id, kind string
		var created int64
		var read *int64
		var source map[string]any
		if err = rows.Scan(&id, &kind, &created, &read, &source); err != nil {
			rows.Close()
			return err
		}
		item := map[string]any{"id": id, "kind": kind, "created_at": interactionTime(created), "unread": read == nil, "available": source != nil}
		if source != nil {
			for k, v := range source {
				item[k] = v
			}
			item["url"] = "/posts/" + source["post_id"].(string) + "?comment=" + source["comment_id"].(string) + "#comment-" + source["comment_id"].(string)
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	next := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		next = a.interactionNext(scope, last["id"].(string), notificationTime(last["created_at"].(string)))
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, map[string]any{"viewer_id": p.ID, "filter": filter, "items": items, "unread_count": unread, "has_more": next != "", "next_cursor": next})
	return nil
}

func (a *App) notificationRead(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if !positiveID(id) {
		return NotFound()
	}
	_, err := a.Write(r, nil, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		tag, err := tx.Exec(r.Context(), `UPDATE notifications SET read_at_ms=COALESCE(read_at_ms,$3) WHERE id=$1 AND recipient_user_id=$2`, id, p.ID, NowMillis())
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, NotFound()
		}
		return nil, nil
	})
	if err == nil {
		w.WriteHeader(http.StatusNoContent)
	}
	return err
}

// Public, bounded context for a notification deep link, including comments that
// are beyond the first page. It never bypasses the ordinary discussion policy.
func (a *App) interactionCommentContext(w http.ResponseWriter, r *http.Request) error {
	post, id := r.PathValue("postID"), r.PathValue("commentID")
	if !positiveID(post) || !positiveID(id) {
		return NotFound()
	}
	p, _ := a.Principal(r)
	viewer := p.ID
	if viewer == "" {
		viewer = "0"
	}
	tx, err := a.Pool.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var rootID string
	err = tx.QueryRow(r.Context(), `SELECT COALESCE(c.parent_id,c.id)::text FROM comments c JOIN public_posts pp ON pp.post_id=c.post_id LEFT JOIN comments root ON root.id=c.parent_id WHERE c.id=$1 AND c.post_id=$2 AND c.state='visible' AND (c.parent_id IS NULL OR root.state='visible')`, id, post).Scan(&rootID)
	if err == pgx.ErrNoRows {
		return NotFound()
	}
	if err != nil {
		return err
	}
	target, err := interactionCommentObject(r.Context(), tx, id, viewer)
	if err != nil {
		return err
	}
	root := target
	if rootID != id {
		root, err = interactionCommentObject(r.Context(), tx, rootID, viewer)
		if err != nil {
			return err
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, map[string]any{"root": root, "target": target})
	return nil
}

func notificationTime(value string) int64 {
	stamp, _ := time.Parse(time.RFC3339Nano, value)
	return stamp.UnixMilli()
}
