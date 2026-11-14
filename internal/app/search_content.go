package app

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Every branch reads current public content, never working copies. Keep the
// established article search/ranking contract for tags, authors and API clients.
const searchContentUnion = `
 SELECT v.post_id AS id,'post'::text AS kind,v.title,v.summary,sd.body,v.published_at_ms AS stamp,'/posts/'||v.post_id AS url
 FROM public_posts v JOIN search_documents sd ON sd.post_id=v.post_id AND sd.revision_id=v.revision_id
 UNION ALL
 SELECT s.id,'series',s.title,s.description,''::text,s.updated_at_ms,'/series/'||s.id
 FROM series s JOIN users u ON u.id=s.author_user_id WHERE s.status='published' AND s.deleted_at_ms IS NULL AND u.disabled_at_ms IS NULL
 UNION ALL
 SELECT m.id,'moment',left(m.text,80),m.text,''::text,m.published_at_ms,'/moments/'||m.id
 FROM public_moments m JOIN users u ON u.id=m.author_user_id WHERE u.disabled_at_ms IS NULL
 UNION ALL
 SELECT id,'phrase',text,source,''::text,updated_at_ms,'/phrases/'||id FROM phrases WHERE published`

func (a *App) SearchContent(parent context.Context, q url.Values) (map[string]any, error) {
	kind := q.Get("type")
	if kind == "" {
		kind = "all"
	}
	if len(q["type"]) > 1 || (kind != "all" && kind != "post" && kind != "series" && kind != "moment" && kind != "phrase") {
		return nil, E(400, "VALIDATION_FAILED", "搜索类型无效")
	}
	if len(q["tag_id"]) > 0 || q.Get("author_id") != "" || q.Get("category_id") != "" {
		kind = "post"
	}
	if kind == "post" {
		data, err := a.SearchPosts(parent, q)
		if err == nil {
			for _, item := range webMaps(data["items"]) {
				item["kind"] = "post"
				item["url"] = "/posts/" + webString(item["id"])
			}
		}
		return data, err
	}
	terms, normalized, err := searchTerms(q.Get("q"))
	if err != nil {
		return nil, err
	}
	order := q.Get("sort")
	if order != "" && order != "relevance" && order != "recent" && order != "newest" {
		return nil, E(400, "VALIDATION_FAILED", "排序无效")
	}
	limit := contentLimit(q)
	hash := hashString("content-search:v1\x00" + normalized + "\x00" + kind + "\x00" + order + fmt.Sprint(limit))
	page := searchPage{Hash: hash}
	if q.Get("cursor") != "" {
		if err = a.decodePage(q.Get("cursor"), &page); err != nil {
			return nil, err
		}
		if page.Hash != hash || page.Offset < 0 || page.Offset >= 1000 {
			return nil, E(409, "SEARCH_CHANGED", "查询已改变，请重新搜索")
		}
	}
	select {
	case searchSlots <- struct{}{}:
		defer func() { <-searchSlots }()
	default:
		return nil, E(503, "SEARCH_TIMEOUT", "搜索繁忙，请稍后重试")
	}
	ctx, cancel := context.WithTimeout(parent, 500*time.Millisecond)
	defer cancel()
	tx, err := a.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, searchError(err)
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SET LOCAL statement_timeout='500ms'"); err != nil {
		return nil, searchError(err)
	}
	args := []any{kind}
	where := []string{"($1::text='all' OR kind=$1)"}
	scores := []string{}
	for _, term := range terms {
		args = append(args, term)
		n := len(args)
		where = append(where, fmt.Sprintf("strpos(casefold(normalize(title||' '||summary||' '||body,NFKC)),$%d)>0", n))
		scores = append(scores, fmt.Sprintf("(strpos(casefold(normalize(title,NFKC)),$%d)>0)::int", n))
	}
	sorting := "stamp DESC,id DESC,kind"
	if order == "" || order == "relevance" {
		sorting = strings.Join(scores, "+") + " DESC," + sorting
	}
	args = append(args, min(limit+1, 1001-page.Offset), page.Offset)
	statement := `WITH content AS (` + searchContentUnion + `) SELECT jsonb_build_object('id',id::text,'kind',kind,'title',title,'summary',summary,'_body',body,'url',url) FROM content WHERE ` + strings.Join(where, " AND ") + " ORDER BY " + sorting + fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	items, err := contentObjects(ctx, tx, statement, args...)
	if err != nil {
		return nil, searchError(err)
	}
	more := len(items) > limit && page.Offset+limit < 1000
	if len(items) > limit {
		items = items[:limit]
	}
	next := ""
	if more {
		next = a.encodePage(searchPage{Hash: hash, Offset: page.Offset + len(items)})
	}
	for _, item := range items {
		item["highlight"] = map[string]any{"title": highlightSnippet(webString(item["title"]), terms, 200), "body": highlightSnippet(webString(item["summary"])+" "+webString(item["_body"]), terms, 180)}
		delete(item, "_body")
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, searchError(err)
	}
	return map[string]any{"items": items, "has_more": more, "next_cursor": next, "query": normalized, "search_instance_id": RandID()}, nil
}
