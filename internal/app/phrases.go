package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const phrasePageLimit = 200
const phraseBodyLimit = 12 << 10
const phraseColumns = "id::text,version,text,kind,source,published,created_at_ms,updated_at_ms"

type phraseInput struct {
	Text      string `json:"text"`
	Kind      string `json:"kind"`
	Source    string `json:"source"`
	Published bool   `json:"published"`
}

func (a *App) RegisterPhrases() {
	a.Handle("GET /api/v1/phrases", func(w http.ResponseWriter, r *http.Request) error {
		value, err := a.PublicPhrases(r.Context(), r.URL.Query())
		if err == nil {
			JSON(w, 200, value)
		}
		return err
	})
	a.Handle("GET /api/v1/admin/phrases", a.phrasesAdminList)
	a.Handle("GET /api/v1/admin/phrases/{id}", a.phraseAdminGet)
	a.Handle("POST /api/v1/admin/phrases", a.phraseCreate)
	a.Handle("PUT /api/v1/admin/phrases/{id}", a.phraseUpdate)
	a.Handle("DELETE /api/v1/admin/phrases/{id}", a.phraseDelete)
}

func phraseText(raw string, max int, required bool) (string, error) {
	if !utf8.ValidString(raw) || strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsControl(r) || r == '\u2028' || r == '\u2029' }) {
		return "", E(400, "VALIDATION_FAILED", "片语与来源须为有效文字，不能包含控制字符或换行")
	}
	text := strings.TrimSpace(raw)
	if utf8.RuneCountInString(text) > max || (required && text == "") {
		return "", E(400, "VALIDATION_FAILED", "片语须为1至280个字符，来源最多120个字符")
	}
	return text, nil
}

func decodePhrase(w http.ResponseWriter, r *http.Request) (phraseInput, error) {
	var in phraseInput
	r.Body = http.MaxBytesReader(w, r.Body, phraseBodyLimit)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return in, E(413, "PAYLOAD_TOO_LARGE", "片语请求最多12 KiB")
		}
		return in, E(400, "INVALID_JSON", "无法读取片语请求")
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	if err = Decode(r, &in); err != nil {
		return in, err
	}
	if in.Text, err = phraseText(in.Text, 280, true); err != nil {
		return in, err
	}
	if in.Source, err = phraseText(in.Source, 120, false); err != nil {
		return in, err
	}
	if in.Kind != "sentence" && in.Kind != "word" {
		return in, E(400, "VALIDATION_FAILED", "片语类型须为句子或词语")
	}
	return in, nil
}

func scanPhrase(row interface{ Scan(...any) error }) (map[string]any, error) {
	var id, text, kind, source string
	var version, created, updated int64
	var published bool
	if err := row.Scan(&id, &version, &text, &kind, &source, &published, &created, &updated); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "version": version, "text": text, "kind": kind, "source": source, "published": published, "created_at": TimeString(created), "updated_at": TimeString(updated)}, nil
}

func phrasePage(q url.Values) (int, int64, error) {
	limit := phrasePageLimit
	if raw := q.Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return 0, 0, E(400, "VALIDATION_FAILED", "片语每页数量须为正整数，最多200项")
		}
		limit = min(value, phrasePageLimit)
	}
	var cursor int64
	if raw := q.Get("cursor"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 1 {
			return 0, 0, E(400, "INVALID_CURSOR", "片语分页位置无效")
		}
		cursor = value
	}
	return limit, cursor, nil
}

// PublicPhrases always filters in SQL. A caller's cookie or query parameters can
// never turn this SSR/API helper into the private owner collection.
func (a *App) PublicPhrases(ctx context.Context, q url.Values) (map[string]any, error) {
	return a.listPhrases(ctx, q, true)
}

func (a *App) listPhrases(ctx context.Context, q url.Values, public bool) (map[string]any, error) {
	limit, cursor, err := phrasePage(q)
	if err != nil {
		return nil, err
	}
	where := "WHERE ($1::bigint=0 OR id<$1)"
	if public {
		where += " AND published=true"
	}
	// Qualify the source ID: ORDER BY the selected id::text alias would sort
	// lexicographically and break numeric keyset pagination.
	rows, err := a.Pool.Query(ctx, "SELECT "+phraseColumns+" FROM phrases "+where+" ORDER BY phrases.id DESC LIMIT $2", cursor, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	more := false
	for rows.Next() {
		if len(items) == limit {
			more = true
			break
		}
		item, err := scanPhrase(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var next any
	if more {
		next = items[len(items)-1]["id"]
	}
	return map[string]any{"items": items, "next_cursor": next, "has_more": more, "limit": limit, "max_limit": phrasePageLimit}, nil
}

func (a *App) phrasesAdminList(w http.ResponseWriter, r *http.Request) error {
	if _, err := a.Require(r, "owner"); err != nil {
		return err
	}
	value, err := a.listPhrases(r.Context(), r.URL.Query(), false)
	if err == nil {
		JSON(w, 200, value)
	}
	return err
}

func (a *App) phraseAdminGet(w http.ResponseWriter, r *http.Request) error {
	if _, err := a.Require(r, "owner"); err != nil {
		return err
	}
	id := r.PathValue("id")
	if !positiveID(id) {
		return NotFound()
	}
	item, err := scanPhrase(a.Pool.QueryRow(r.Context(), "SELECT "+phraseColumns+" FROM phrases WHERE id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return NotFound()
	}
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, item)
	return nil
}

func (a *App) phraseAuthorize(r *http.Request) error {
	if _, err := a.Require(r, "owner"); err != nil {
		return err
	}
	return a.checkCSRF(r)
}

func (a *App) phraseCreate(w http.ResponseWriter, r *http.Request) error {
	if err := a.phraseAuthorize(r); err != nil {
		return err
	}
	in, err := decodePhrase(w, r)
	if err != nil {
		return err
	}
	value, err := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		now := NowMillis()
		item, err := scanPhrase(tx.QueryRow(r.Context(), "INSERT INTO phrases(text,kind,source,published,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$4,$5,$5) RETURNING "+phraseColumns, in.Text, in.Kind, in.Source, in.Published, now))
		if err != nil {
			return nil, err
		}
		if err = audit(r.Context(), tx, p, "phrase.create", "phrase", item["id"].(string), ""); err != nil {
			return nil, err
		}
		return item, nil
	})
	if err == nil {
		JSON(w, 201, value)
	}
	return err
}

func phraseVersion(ctx context.Context, tx pgx.Tx, id string, expected int64) error {
	var actual int64
	err := tx.QueryRow(ctx, "SELECT version FROM phrases WHERE id=$1 FOR UPDATE", id).Scan(&actual)
	if errors.Is(err, pgx.ErrNoRows) {
		return NotFound()
	}
	if err != nil {
		return err
	}
	if actual != expected {
		return E(409, "VERSION_CONFLICT", "片语已变化，请重新加载后再保存")
	}
	return nil
}

func (a *App) phraseUpdate(w http.ResponseWriter, r *http.Request) error {
	if err := a.phraseAuthorize(r); err != nil {
		return err
	}
	id := r.PathValue("id")
	if !positiveID(id) {
		return NotFound()
	}
	version, err := IfMatch(r)
	if err != nil {
		return err
	}
	in, err := decodePhrase(w, r)
	if err != nil {
		return err
	}
	value, err := a.Write(r, []string{"owner"}, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		if err := phraseVersion(r.Context(), tx, id, version); err != nil {
			return nil, err
		}
		item, err := scanPhrase(tx.QueryRow(r.Context(), "UPDATE phrases SET text=$2,kind=$3,source=$4,published=$5,version=version+1,updated_at_ms=$6 WHERE id=$1 RETURNING "+phraseColumns, id, in.Text, in.Kind, in.Source, in.Published, NowMillis()))
		if err != nil {
			return nil, err
		}
		if err = audit(r.Context(), tx, p, "phrase.update", "phrase", id, ""); err != nil {
			return nil, err
		}
		return item, nil
	})
	if err == nil {
		JSON(w, 200, value)
	}
	return err
}

func (a *App) phraseDelete(w http.ResponseWriter, r *http.Request) error {
	if err := a.phraseAuthorize(r); err != nil {
		return err
	}
	id := r.PathValue("id")
	if !positiveID(id) {
		return NotFound()
	}
	version, err := IfMatch(r)
	if err != nil {
		return err
	}
	_, err = a.Write(r, []string{"owner"}, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		if err := phraseVersion(r.Context(), tx, id, version); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(r.Context(), "DELETE FROM phrases WHERE id=$1", id); err != nil {
			return nil, err
		}
		return nil, audit(r.Context(), tx, p, "phrase.delete", "phrase", id, "")
	})
	if err == nil {
		w.WriteHeader(204)
	}
	return err
}
