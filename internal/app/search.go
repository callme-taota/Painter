package app

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

var searchSlots = make(chan struct{}, 2)

type searchPage struct {
	Hash       string `json:"q"`
	Generation int64  `json:"g"`
	Offset     int    `json:"o"`
}

func (a *App) encodePage(v any) string {
	b, _ := json.Marshal(v)
	raw := base64.RawURLEncoding.EncodeToString(b)
	return raw + "." + macString(a.Config.EncryptionKey, "cursor:v1:"+raw)
}
func (a *App) decodePage(cursor string, v any) error {
	p := strings.Split(cursor, ".")
	if len(p) != 2 || !hmac.Equal([]byte(p[1]), []byte(macString(a.Config.EncryptionKey, "cursor:v1:"+p[0]))) {
		return E(400, "VALIDATION_FAILED", "分页游标无效")
	}
	raw, e := base64.RawURLEncoding.DecodeString(p[0])
	if e != nil || json.Unmarshal(raw, v) != nil {
		return E(400, "VALIDATION_FAILED", "分页游标无效")
	}
	return nil
}
func (a *App) RegisterSearch() {
	a.Handle("GET /api/v1/search/suggestions", a.searchSuggestions)
	a.Handle("GET /api/v1/search", func(w http.ResponseWriter, r *http.Request) error {
		var v map[string]any
		var e error
		if r.URL.Query().Get("type") == "" {
			v, e = a.SearchPosts(r.Context(), r.URL.Query())
		} else {
			v, e = a.SearchContent(r.Context(), r.URL.Query())
		}
		if e == nil {
			JSON(w, 200, v)
		}
		return e
	})
}
func searchTerms(query string) ([]string, string, error) {
	q := Normalize(query)
	if q == "" {
		return nil, q, E(400, "EMPTY_QUERY", "请输入搜索词")
	}
	if utf8.RuneCountInString(q) > 64 {
		return nil, q, E(400, "VALIDATION_FAILED", "搜索词最多64个字符")
	}
	terms := []string{}
	seen := map[string]bool{}
	for _, term := range strings.Fields(q) {
		if !seen[term] {
			terms = append(terms, term)
			seen[term] = true
		}
	}
	if len(terms) > 8 {
		return nil, q, E(400, "VALIDATION_FAILED", "最多8个搜索词")
	}
	return terms, q, nil
}
func searchPattern(term string) string {
	return "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(term) + "%"
}
func (a *App) SearchPosts(parent context.Context, q url.Values) (map[string]any, error) {
	select {
	case searchSlots <- struct{}{}:
		defer func() { <-searchSlots }()
	default:
		return nil, E(503, "SEARCH_TIMEOUT", "搜索繁忙，请稍后重试")
	}
	tags, e := selectedTagIDs(q)
	if e != nil {
		return nil, e
	}
	terms, normalized, e := searchTerms(q.Get("q"))
	if e != nil && !(normalized == "" && (len(tags) > 0 || q.Get("author_id") != "")) {
		return nil, e
	}
	limit := contentLimit(q)
	sortOrder := q.Get("sort")
	if sortOrder == "" {
		sortOrder = "relevance"
	}
	if sortOrder != "relevance" && sortOrder != "recent" && sortOrder != "newest" {
		return nil, E(400, "VALIDATION_FAILED", "排序无效")
	}
	hash := hashString(RendererVersion + "\x00field_hits_v1\x00" + normalized + "\x00" + q.Get("author_id") + "\x00" + q.Get("category_id") + "\x00" + tagFilterScope(q) + "\x00" + sortOrder + fmt.Sprint(limit))
	page := searchPage{Hash: hash}
	if cursor := q.Get("cursor"); cursor != "" {
		if e = a.decodePage(cursor, &page); e != nil {
			return nil, e
		}
		if page.Hash != hash || page.Offset < 0 || page.Offset >= 1000 {
			return nil, E(409, "SEARCH_CHANGED", "筛选或查询已改变，请重新搜索")
		}
	}
	ctx, cancel := context.WithTimeout(parent, 500*time.Millisecond)
	defer cancel()
	tx, e := a.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return nil, searchError(e)
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, "SET LOCAL statement_timeout='500ms'"); e != nil {
		return nil, searchError(e)
	}
	var generation int64
	if e = tx.QueryRow(ctx, "SELECT generation FROM search_state WHERE id=1").Scan(&generation); e != nil {
		return nil, searchError(e)
	}
	if q.Get("cursor") != "" && page.Generation != generation {
		return nil, E(409, "SEARCH_CHANGED", "内容已更新，请重新搜索")
	}
	args := []any{}
	conditions := []string{}
	scores := map[string][]string{"title": {}, "summary": {}, "body": {}}
	for _, term := range terms {
		args = append(args, term)
		raw := len(args)
		arg := raw
		if utf8.RuneCountInString(term) >= 3 {
			if sortOrder == "relevance" {
				args = append(args, searchPattern(term))
				arg = len(args)
			} else {
				args[raw-1] = searchPattern(term)
			}
		}
		or := []string{}
		for _, field := range []string{"title", "summary", "body"} {
			expr := fmt.Sprintf("strpos(sd.%s,$%d)>0", field, raw)
			if utf8.RuneCountInString(term) >= 3 {
				expr = fmt.Sprintf("sd.%s LIKE $%d ESCAPE '!'", field, arg)
			}
			or = append(or, expr)
			scores[field] = append(scores[field], fmt.Sprintf("(strpos(sd.%s,$%d)>0)::int", field, raw))
		}
		conditions = append(conditions, "("+strings.Join(or, " OR ")+")")
	}
	filters, filterArgs, e := contentFilter(q, len(args)+1)
	if e != nil {
		return nil, e
	}
	args = append(args, filterArgs...)
	order := "v.published_at_ms DESC,v.post_id DESC"
	if sortOrder == "relevance" && len(terms) > 0 {
		order = strings.Join(scores["title"], "+") + " DESC," + strings.Join(scores["summary"], "+") + " DESC," + strings.Join(scores["body"], "+") + " DESC," + order
	}
	requested := limit + 1
	if page.Offset+requested > 1001 {
		requested = 1001 - page.Offset
	}
	args = append(args, requested, page.Offset)
	sql := "SELECT " + publicPostJSON + ` || jsonb_build_object('_body',sd.body)` + publicPostFrom + " JOIN search_documents sd ON sd.post_id=v.post_id AND sd.revision_id=v.revision_id " + filters + " AND true " + func() string {
		if len(conditions) == 0 {
			return ""
		}
		return " AND " + strings.Join(conditions, " AND ")
	}() + " ORDER BY " + order + fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	items, e := contentObjects(ctx, tx, sql, args...)
	if e != nil {
		return nil, searchError(e)
	}
	more := len(items) > limit
	capReached := page.Offset+limit >= 1000
	if more {
		items = items[:limit]
	}
	if capReached {
		more = false
	}
	next := ""
	if more {
		next = a.encodePage(searchPage{Hash: hash, Generation: generation, Offset: page.Offset + len(items)})
	}
	for _, item := range items {
		title, _ := item["title"].(string)
		summary, _ := item["summary"].(string)
		body, _ := item["_body"].(string)
		item["highlight"] = map[string]any{"title": highlightSnippet(title, terms, 200), "summary": highlightSnippet(summary, terms, 180), "body": highlightSnippet(body, terms, 180)}
		item["snippet"] = snippetText(body, terms, 180)
		delete(item, "_body")
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, searchError(e)
	}
	effective := "field_hits_v1"
	if sortOrder != "relevance" || len(terms) == 0 {
		effective = "published_at_desc_v1"
	}
	return map[string]any{"items": items, "next_cursor": next, "has_more": more, "query": normalized, "query_hash": hash, "generation": generation, "effective_sort": effective, "search_instance_id": RandID(), "result_limit_reached": capReached}, nil
}
func searchError(err error) error {
	var pgerr *pgconn.PgError
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || (errors.As(err, &pgerr) && pgerr.Code == "57014") {
		return E(503, "SEARCH_TIMEOUT", "搜索超时或繁忙，请缩短关键词后重试")
	}
	return err
}
func snippetText(text string, terms []string, max int) string {
	s := Normalize(text)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	first := len(r)
	for _, term := range terms {
		if i := strings.Index(s, term); i >= 0 {
			pos := utf8.RuneCountInString(s[:i])
			if pos < first {
				first = pos
			}
		}
	}
	start := 0
	if first < len(r) && first > 40 {
		start = first - 40
	}
	end := start + max
	if end > len(r) {
		end = len(r)
	}
	prefix, suffix := "", ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(r) {
		suffix = "…"
	}
	return prefix + string(r[start:end]) + suffix
}
func highlightSnippet(text string, terms []string, max int) []map[string]any {
	s := snippetText(text, terms, max)
	sorted := append([]string{}, terms...)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	out := []map[string]any{}
	for len(s) > 0 {
		first := len(s)
		chosen := ""
		for _, term := range sorted {
			if i := strings.Index(s, term); i >= 0 && i < first {
				first = i
				chosen = term
			}
		}
		if chosen == "" {
			out = append(out, map[string]any{"text": s, "match": false})
			break
		}
		if first > 0 {
			out = append(out, map[string]any{"text": s[:first], "match": false})
		}
		out = append(out, map[string]any{"text": chosen, "match": true})
		s = s[first+len(chosen):]
	}
	return out
}
