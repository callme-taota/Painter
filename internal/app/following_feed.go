package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

const followingFeedSummaryLimit = 240

type followingFeedCursor struct {
	Scope string `json:"s"`
	Time  int64  `json:"t"`
	Kind  int    `json:"k"`
	ID    int64  `json:"i"`
}

// Read current public snapshots and relationships together. First publication
// controls ordering, so edits and republication update an item without bumping
// it. Phrases have no author ownership and series surface through their posts.
// The materialized follow set is shared by both branches; no item lookup or
// application-side fan-out is needed.
const followingFeedSQL = `WITH following AS MATERIALIZED (
 SELECT u.id,u.display_name,CASE WHEN u.avatar_media_id IS NULL THEN '' ELSE '/media/'||u.avatar_media_id END AS avatar_url
 FROM follows f JOIN users u ON u.id=f.followed_user_id
 WHERE f.follower_user_id=$1 AND u.disabled_at_ms IS NULL
), feed AS (
 SELECT p.post_id AS id,2 AS kind,'post'::text AS type,p.first_published_at_ms AS stamp,
 left(p.title,200) AS title,left(COALESCE(NULLIF(btrim(p.summary),''),p.plain_text),241) AS summary,
 '/posts/'||p.post_id AS url,u.id AS author_id,u.display_name,u.avatar_url,
 CASE WHEN pr.cover->>'mode'='none' THEN '' ELSE '/api/v1/posts/'||p.post_id||'/cover?revision='||p.revision_id END AS cover_url
 FROM public_posts p JOIN following u ON u.id=p.author_user_id JOIN post_revisions pr ON pr.id=p.revision_id
 WHERE $2 IN('all','post')
 UNION ALL
 SELECT m.id,1,'moment',m.first_published_at_ms,''::text,left(m.text,241),
 '/moments/'||m.id,u.id,u.display_name,u.avatar_url,NULL::text
 FROM public_moments m JOIN following u ON u.id=m.author_user_id WHERE $2 IN('all','moment')
), page AS MATERIALIZED (
 SELECT * FROM feed
WHERE stamp IS NOT NULL AND (NOT $3::boolean OR (stamp,kind,id)<($4::bigint,$5::integer,$6::bigint))
 ORDER BY stamp DESC,kind DESC,id DESC LIMIT $7
), moment_covers AS (
 SELECT DISTINCT ON(mi.moment_id) mi.moment_id,'/media/'||mi.media_id AS cover_url,
 count(*) OVER(PARTITION BY mi.moment_id) AS image_count
 FROM moment_images mi JOIN page p ON p.type='moment' AND p.id=mi.moment_id
 JOIN media_assets media ON media.id=mi.media_id AND media.state='ready' AND media.mime_type IN('image/jpeg','image/png','image/gif','image/webp')
 WHERE mi.scope='public' ORDER BY mi.moment_id,mi.position
)
SELECT (SELECT count(*) FROM following),COALESCE(jsonb_agg(jsonb_build_object(
 'id',p.type||':'||p.id,'content_id',p.id::text,'type',p.type,'url',p.url,'title',p.title,'summary',p.summary,
 'cover_url',COALESCE(p.cover_url,mc.cover_url,''),'image_count',COALESCE(mc.image_count,0),
 'author',jsonb_build_object('id',p.author_id::text,'display_name',p.display_name,'avatar_url',p.avatar_url),
 '_cursor_ms',p.stamp::text,'_cursor_kind',p.kind
) ORDER BY p.stamp DESC,p.kind DESC,p.id DESC),'[]'::jsonb)
FROM page p LEFT JOIN moment_covers mc ON mc.moment_id=p.id AND p.type='moment'`

func (a *App) followingFeed(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", "private, no-store")
	p, err := a.Require(r)
	if err != nil {
		return err
	}
	page, err := a.followingFeedPage(r, p)
	if err == nil {
		JSON(w, 200, page)
	}
	return err
}

// Both the private SSR route and the API call this only after Require. The
// caller owns response/cache policy; all data and cursor rules live here.
func (a *App) followingFeedPage(r *http.Request, p Principal) (map[string]any, error) {
	var err error
	q := r.URL.Query()
	kind := q.Get("type")
	if kind == "" {
		kind = "all"
	}
	if len(q["type"]) > 1 || (kind != "all" && kind != "post" && kind != "moment") {
		return nil, E(400, "VALIDATION_FAILED", "关注更新类型无效")
	}
	limit := 20
	if len(q["limit"]) > 1 || len(q["cursor"]) > 1 {
		return nil, E(400, "VALIDATION_FAILED", "分页参数无效")
	}
	if raw := q.Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 {
			return nil, E(400, "VALIDATION_FAILED", "每页数量须为正整数，最多50项")
		}
		limit = min(n, 50)
	}
	scope := hashString("following-feed:v1\x00" + p.ID + "\x00" + kind + "\x00" + strconv.Itoa(limit))
	cursor := followingFeedCursor{Scope: scope}
	raw := q.Get("cursor")
	if raw != "" {
		if len(raw) > 4096 {
			return nil, E(400, "VALIDATION_FAILED", "分页游标无效")
		}
		if err = a.decodePage(raw, &cursor); err != nil {
			return nil, err
		}
		if cursor.Scope != scope || cursor.Time < 0 || cursor.ID < 1 || (cursor.Kind != 1 && cursor.Kind != 2) || (kind == "post" && cursor.Kind != 2) || (kind == "moment" && cursor.Kind != 1) {
			return nil, E(400, "VALIDATION_FAILED", "分页游标不匹配")
		}
	}
	var count int64
	var payload []byte
	if err = a.Pool.QueryRow(r.Context(), followingFeedSQL, p.ID, kind, raw != "", cursor.Time, cursor.Kind, cursor.ID, limit+1).Scan(&count, &payload); err != nil {
		return nil, err
	}
	items := []map[string]any{}
	if err = json.Unmarshal(payload, &items); err != nil {
		return nil, err
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	last := cursor
	for _, item := range items {
		stamp, _ := strconv.ParseInt(item["_cursor_ms"].(string), 10, 64)
		id, _ := strconv.ParseInt(item["content_id"].(string), 10, 64)
		last = followingFeedCursor{Scope: scope, Time: stamp, Kind: int(item["_cursor_kind"].(float64)), ID: id}
		summary := item["summary"].(string)
		text := []rune(strings.TrimSpace(summary))
		if len(text) > followingFeedSummaryLimit {
			text = append(text[:followingFeedSummaryLimit-1], '…')
		}
		item["summary"], item["published_at"] = string(text), TimeString(stamp)
		delete(item, "_cursor_ms")
		delete(item, "_cursor_kind")
	}
	next := ""
	if more {
		next = a.encodePage(last)
	}
	page := interactionPage(items, next)
	page["type"] = kind
	page["available_types"] = []string{"post", "moment"}
	page["following_count"] = count
	page["viewer_id"] = p.ID
	return page, nil
}
