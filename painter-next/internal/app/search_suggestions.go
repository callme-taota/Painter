package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Suggestions use only the current public projection. A draft title, private
// tag association or withdrawn revision must never become an autocomplete hint.
func (a *App) searchSuggestions(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query().Get("q")
	if !utf8.ValidString(query) || len(query) > 2048 || strings.ContainsRune(query, 0) {
		return E(400, "VALIDATION_FAILED", "搜索词无效")
	}
	query = Normalize(query)
	terms := []string{}
	if query != "" {
		var e error
		terms, query, e = searchTerms(query)
		if e != nil {
			return e
		}
	}
	select {
	case searchSlots <- struct{}{}:
		defer func() { <-searchSlots }()
	default:
		return E(503, "SEARCH_TIMEOUT", "搜索繁忙，请稍后重试")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 450*time.Millisecond)
	defer cancel()
	tx, e := a.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return searchError(e)
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, `SET LOCAL statement_timeout='350ms'`); e != nil {
		return searchError(e)
	}
	args := []any{}
	postConditions, tagConditions := []string{"TRUE"}, []string{"TRUE"}
	for _, term := range terms {
		args = append(args, term)
		postConditions = append(postConditions, fmt.Sprintf("strpos(sd.title,$%d)>0", len(args)))
		tagConditions = append(tagConditions, fmt.Sprintf("strpos(casefold(normalize(t.name,NFKC)),$%d)>0", len(args)))
	}
	rows, e := tx.Query(ctx, `SELECT pp.post_id::text,pp.title FROM public_posts pp JOIN search_documents sd ON sd.post_id=pp.post_id AND sd.revision_id=pp.revision_id WHERE `+strings.Join(postConditions, " AND ")+` ORDER BY pp.published_at_ms DESC,pp.post_id DESC LIMIT 6`, args...)
	if e != nil {
		return searchError(e)
	}
	items := []map[string]any{}
	for rows.Next() {
		var id, title string
		if e = rows.Scan(&id, &title); e != nil {
			rows.Close()
			return e
		}
		items = append(items, map[string]any{"kind": "post", "label": title, "url": "/posts/" + id})
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return searchError(e)
	}
	rows.Close()
	rows, e = tx.Query(ctx, `SELECT t.id::text,t.name FROM tags t JOIN effective_revision_tags rt ON rt.tag_id=t.id JOIN public_posts pp ON pp.revision_id=rt.revision_id WHERE t.archived_at_ms IS NULL AND `+strings.Join(tagConditions, " AND ")+` GROUP BY t.id,t.name ORDER BY count(DISTINCT pp.post_id) DESC,t.name,t.id LIMIT 4`, args...)
	if e != nil {
		return searchError(e)
	}
	for rows.Next() {
		var id, name string
		if e = rows.Scan(&id, &name); e != nil {
			rows.Close()
			return e
		}
		items = append(items, map[string]any{"kind": "tag", "label": name, "url": "/search?tag_id=" + id})
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return searchError(e)
	}
	rows.Close()
	if e = tx.Commit(ctx); e != nil {
		return searchError(e)
	}
	JSON(w, 200, map[string]any{"items": items, "query": query})
	return nil
}
