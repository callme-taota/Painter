package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const SeriesChapterLimit = 300

type seriesInput struct {
	Title          string   `json:"title"`
	Slug           string   `json:"slug"`
	Description    string   `json:"description"`
	CoverMediaID   *string  `json:"cover_media_id"`
	Status         string   `json:"status"`
	State          string   `json:"state"`
	ChapterPostIDs []string `json:"chapter_post_ids"`
}

var seriesSlug = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func (a *App) RegisterSeries() {
	a.Handle("GET /api/v1/series", func(w http.ResponseWriter, r *http.Request) error {
		value, err := a.PublicSeries(r.Context(), r.URL.Query())
		if err == nil {
			JSON(w, 200, value)
		}
		return err
	})
	a.Handle("GET /api/v1/series/{id}", func(w http.ResponseWriter, r *http.Request) error {
		value, err := a.PublicSeriesDetail(r.Context(), r.PathValue("id"))
		if err == nil {
			JSON(w, 200, value)
		}
		return err
	})
	a.Handle("GET /api/v1/me/series", a.mySeries)
	a.Handle("GET /api/v1/studio/series/{id}", a.studioSeries)
	a.Handle("POST /api/v1/series", a.createSeries)
	a.Handle("PUT /api/v1/studio/series/{id}", a.updateSeries)
	a.Handle("DELETE /api/v1/studio/series/{id}", a.deleteSeries)
}

var seriesPublicJSON = `jsonb_build_object('id',s.id::text,'title',s.title,'slug',s.slug,'description',s.description,
 'state',s.state,'cover_url',CASE WHEN s.cover_media_id IS NULL THEN '' ELSE '/media/'||s.cover_media_id END,
 'author',jsonb_build_object('id',u.id::text,'username',u.username,'display_name',u.display_name,'avatar_url',CASE WHEN u.avatar_media_id IS NULL THEN '' ELSE '/media/'||u.avatar_media_id END),
 'chapter_count',(SELECT count(*) FROM series_chapters sc JOIN public_posts pp ON pp.post_id=sc.post_id WHERE sc.series_id=s.id),
 'created_at',` + contentTimeSQL("s.created_at_ms") + `,'updated_at',` + contentTimeSQL("s.updated_at_ms") + `)`

var seriesPrivateJSON = seriesPublicJSON + ` || jsonb_build_object('version',s.version,'status',s.status,'cover_media_id',s.cover_media_id,
 'total_chapter_count',(SELECT count(*) FROM series_chapters sc WHERE sc.series_id=s.id))`

const seriesFrom = ` FROM series s JOIN users u ON u.id=s.author_user_id `

// PublicSeries never accepts a viewer or a draft visibility flag. Its signed
// cursors bind author/filter/limit, and cannot be reused for a private listing.
func (a *App) PublicSeries(ctx context.Context, q url.Values) (map[string]any, error) {
	return a.listSeries(ctx, q, Principal{}, true)
}

// PublicSeriesSummary avoids loading all chapter metadata when resolving a
// share target or another compact public reference.
func (a *App) PublicSeriesSummary(ctx context.Context, id string) (map[string]any, error) {
	if contentID(id) != nil {
		return nil, NotFound()
	}
	return contentObject(ctx, a.Pool, "SELECT "+seriesPublicJSON+seriesFrom+` WHERE s.id=$1 AND s.status='published' AND s.deleted_at_ms IS NULL`, id)
}

func (a *App) listSeries(ctx context.Context, q url.Values, p Principal, public bool) (map[string]any, error) {
	limit := contentLimit(q)
	where := " WHERE s.deleted_at_ms IS NULL"
	args := []any{}
	scope := "series-public"
	projection := seriesPublicJSON
	if public {
		where += " AND s.status='published'"
	} else {
		projection = seriesPrivateJSON
		scope = "series-private/" + p.ID
		if q.Get("scope") == "site" {
			if p.Role != "owner" {
				return nil, E(403, "PERMISSION_DENIED", "需要站长权限")
			}
			scope += "/site"
		} else {
			args = append(args, p.ID)
			where += fmt.Sprintf(" AND s.author_user_id=$%d", len(args))
		}
		switch q.Get("status") {
		case "", "all":
		case "draft", "published":
			args = append(args, q.Get("status"))
			where += fmt.Sprintf(" AND s.status=$%d", len(args))
		default:
			return nil, E(400, "VALIDATION_FAILED", "专栏状态无效")
		}
	}
	if state := q.Get("state"); state != "" && state != "all" {
		if state != "ongoing" && state != "completed" {
			return nil, E(400, "VALIDATION_FAILED", "连载状态无效")
		}
		args = append(args, state)
		where += fmt.Sprintf(" AND s.state=$%d", len(args))
	}
	if author := q.Get("author_id"); author != "" {
		if contentID(author) != nil {
			return nil, E(400, "VALIDATION_FAILED", "作者ID无效")
		}
		args = append(args, author)
		where += fmt.Sprintf(" AND s.author_user_id=$%d", len(args))
	}
	query := strings.TrimSpace(q.Get("q"))
	if utf8.RuneCountInString(query) > 64 {
		return nil, E(400, "VALIDATION_FAILED", "搜索词最多64个字符")
	}
	if query != "" {
		args = append(args, searchPattern(query))
		where += fmt.Sprintf(" AND s.title ILIKE $%d ESCAPE '!'", len(args))
	}
	scope = hashString(strings.Join([]string{scope, q.Get("status"), q.Get("state"), q.Get("author_id"), query, strconv.Itoa(limit)}, "\x00"))
	if cursor := q.Get("cursor"); cursor != "" {
		var page contentListPage
		if err := a.decodePage(cursor, &page); err != nil {
			return nil, err
		}
		if page.Scope != scope || contentID(page.ID) != nil {
			return nil, E(400, "VALIDATION_FAILED", "分页条件已改变，请重新加载")
		}
		args = append(args, page.ID)
		where += fmt.Sprintf(" AND s.id<$%d::bigint", len(args))
	}
	args = append(args, limit+1)
	items, err := contentObjects(ctx, a.Pool, "SELECT "+projection+seriesFrom+where+fmt.Sprintf(" ORDER BY s.id DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		return nil, err
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	next := ""
	if more {
		next = a.encodePage(contentListPage{Scope: scope, ID: items[len(items)-1]["id"].(string)})
	}
	return map[string]any{"items": items, "has_more": more, "next_cursor": next}, nil
}

// PublicSeriesDetail uses one MVCC statement for metadata, count and chapters;
// draft titles never enter the result, including when an article is withdrawn.
func (a *App) PublicSeriesDetail(ctx context.Context, id string) (map[string]any, error) {
	if contentID(id) != nil {
		return nil, NotFound()
	}
	chapters := `COALESCE((SELECT jsonb_agg(jsonb_build_object('post_id',pp.post_id::text,'id',pp.post_id::text,'position',sc.position,'title',pp.title,'summary',pp.summary,
 'published_at',` + contentTimeSQL("pp.published_at_ms") + `,'cover_url',CASE WHEN pr.cover->>'mode'='none' THEN '' ELSE '/api/v1/posts/'||pp.post_id||'/cover?revision='||pp.revision_id END) ORDER BY sc.position)
 FROM series_chapters sc JOIN public_posts pp ON pp.post_id=sc.post_id JOIN post_revisions pr ON pr.id=pp.revision_id WHERE sc.series_id=s.id),'[]'::jsonb)`
	return contentObject(ctx, a.Pool, "SELECT "+seriesPublicJSON+` || jsonb_build_object('chapters',`+chapters+`)`+seriesFrom+` WHERE s.id=$1 AND s.status='published' AND s.deleted_at_ms IS NULL`, id)
}

// PostSeries supplies the article's serial navigation in one query. Both the
// current article and all neighbours must still be in the live public view.
func (a *App) PostSeries(ctx context.Context, postID string) ([]map[string]any, error) {
	if contentID(postID) != nil {
		return nil, NotFound()
	}
	return contentObjects(ctx, a.Pool, `SELECT `+seriesPublicJSON+` || jsonb_build_object('position',current.position,
 'previous',(SELECT jsonb_build_object('id',pp.post_id::text,'title',pp.title,'position',sc.position) FROM series_chapters sc JOIN public_posts pp ON pp.post_id=sc.post_id WHERE sc.series_id=s.id AND sc.position<current.position ORDER BY sc.position DESC LIMIT 1),
 'next',(SELECT jsonb_build_object('id',pp.post_id::text,'title',pp.title,'position',sc.position) FROM series_chapters sc JOIN public_posts pp ON pp.post_id=sc.post_id WHERE sc.series_id=s.id AND sc.position>current.position ORDER BY sc.position LIMIT 1))`+seriesFrom+` JOIN series_chapters current ON current.series_id=s.id JOIN public_posts current_post ON current_post.post_id=current.post_id WHERE current.post_id=$1 AND s.status='published' AND s.deleted_at_ms IS NULL ORDER BY s.id`, postID)
}

func (a *App) privateSeries(ctx context.Context, db contentQuerier, p Principal, id string) (map[string]any, error) {
	if contentID(id) != nil {
		return nil, NotFound()
	}
	chapters := `COALESCE((SELECT jsonb_agg(jsonb_build_object('post_id',p.id::text,'id',p.id::text,'position',sc.position,'title',d.title,
 'status',CASE WHEN p.deleted_at_ms IS NOT NULL THEN 'deleted' WHEN pp.post_id IS NULL THEN 'draft' ELSE 'published' END) ORDER BY sc.position)
 FROM series_chapters sc JOIN posts p ON p.id=sc.post_id JOIN post_drafts d ON d.post_id=p.id LEFT JOIN public_posts pp ON pp.post_id=p.id WHERE sc.series_id=s.id),'[]'::jsonb)`
	return contentObject(ctx, db, "SELECT "+seriesPrivateJSON+` || jsonb_build_object('chapters',`+chapters+`)`+seriesFrom+` WHERE s.id=$1 AND s.deleted_at_ms IS NULL AND (s.author_user_id=$2 OR $3)`, id, p.ID, p.Role == "owner")
}

func (a *App) mySeries(w http.ResponseWriter, r *http.Request) error {
	p, err := a.Require(r, "author", "owner")
	if err != nil {
		return err
	}
	value, err := a.listSeries(r.Context(), r.URL.Query(), p, false)
	if err == nil {
		JSON(w, 200, value)
	}
	return err
}

func (a *App) studioSeries(w http.ResponseWriter, r *http.Request) error {
	p, err := a.Require(r, "author", "owner")
	if err != nil {
		return err
	}
	value, err := a.privateSeries(r.Context(), a.Pool, p, r.PathValue("id"))
	if err == nil {
		w.Header().Set("ETag", `"`+webString(value["version"])+`"`)
		JSON(w, 200, value)
	}
	return err
}

func decodeSeries(r *http.Request) (seriesInput, error) {
	var in seriesInput
	if err := Decode(r, &in); err != nil {
		return in, err
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	if in.Title == "" || utf8.RuneCountInString(in.Title) > 120 || utf8.RuneCountInString(in.Description) > 1200 || strings.ContainsFunc(in.Title, unicode.IsControl) || strings.ContainsFunc(in.Description, func(c rune) bool { return unicode.IsControl(c) && c != '\n' && c != '\t' }) {
		return in, E(400, "VALIDATION_FAILED", "专栏标题须为1至120字，简介最多1200字")
	}
	if in.Slug != "" && (!seriesSlug.MatchString(in.Slug) || len(in.Slug) > 96) {
		return in, E(400, "VALIDATION_FAILED", "专栏地址仅支持小写字母、数字和连字符，最多96字符")
	}
	if in.Status == "" {
		in.Status = "draft"
	}
	if in.State == "" {
		in.State = "ongoing"
	}
	if (in.Status != "draft" && in.Status != "published") || (in.State != "ongoing" && in.State != "completed") {
		return in, E(400, "VALIDATION_FAILED", "请选择专栏发布状态与连载状态")
	}
	if in.CoverMediaID != nil && !authorMediaID.MatchString(*in.CoverMediaID) {
		return in, E(400, "VALIDATION_FAILED", "封面图片ID无效")
	}
	if len(in.ChapterPostIDs) > SeriesChapterLimit {
		return in, E(400, "VALIDATION_FAILED", "每个专栏最多300章")
	}
	seen := map[string]bool{}
	for index, id := range in.ChapterPostIDs {
		if contentID(id) != nil {
			return in, E(400, "VALIDATION_FAILED", "章节ID无效或重复")
		}
		number, _ := strconv.ParseInt(id, 10, 64)
		id = strconv.FormatInt(number, 10)
		if seen[id] {
			return in, E(400, "VALIDATION_FAILED", "章节ID无效或重复")
		}
		seen[id] = true
		in.ChapterPostIDs[index] = id
	}
	return in, nil
}

// The parent row is locked before replacing any children. Post locks are sorted
// to keep concurrent publications/reorders from taking article locks backwards.
func seriesValidateReferences(ctx context.Context, tx pgx.Tx, author string, in seriesInput) error {
	ids := append([]string(nil), in.ChapterPostIDs...)
	sort.Slice(ids, func(i, j int) bool {
		a, _ := strconv.ParseInt(ids[i], 10, 64)
		b, _ := strconv.ParseInt(ids[j], 10, 64)
		return a < b
	})
	if len(ids) > 0 {
		rows, err := tx.Query(ctx, `SELECT id::text,author_user_id::text,deleted_at_ms IS NOT NULL FROM posts WHERE id IN (SELECT wanted.id::bigint FROM unnest($1::text[]) wanted(id)) ORDER BY posts.id FOR KEY SHARE`, ids)
		if err != nil {
			return err
		}
		count := 0
		invalid := false
		for rows.Next() {
			var id, owner string
			var deleted bool
			if err = rows.Scan(&id, &owner, &deleted); err != nil {
				rows.Close()
				return err
			}
			count++
			invalid = invalid || owner != author || deleted
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if count != len(ids) || invalid {
			return E(400, "VALIDATION_FAILED", "章节必须选择专栏作者未删除的文章")
		}
	}
	if in.CoverMediaID != nil {
		var owner, state string
		err := tx.QueryRow(ctx, `SELECT owner_user_id::text,state FROM media_assets WHERE id=$1 FOR UPDATE`, *in.CoverMediaID).Scan(&owner, &state)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (owner != author || state != "ready")) {
			return E(400, "VALIDATION_FAILED", "封面必须使用专栏作者已完成上传的图片")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func seriesReplaceChapters(ctx context.Context, tx pgx.Tx, id string, chapters []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM series_chapters WHERE series_id=$1`, id); err != nil {
		return err
	}
	if len(chapters) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO series_chapters(series_id,post_id,position) SELECT $1,p.id::bigint,p.ordinality::integer FROM unnest($2::text[]) WITH ORDINALITY p(id,ordinality)`, id, chapters)
	return err
}

func (a *App) createSeries(w http.ResponseWriter, r *http.Request) error {
	in, err := decodeSeries(r)
	if err != nil {
		return err
	}
	value, err := a.Write(r, []string{"author", "owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		if err := seriesValidateReferences(r.Context(), tx, p.ID, in); err != nil {
			return nil, err
		}
		if in.Slug == "" {
			in.Slug = "series-" + RandID()[:12]
		}
		now := NowMillis()
		var id string
		err := tx.QueryRow(r.Context(), `INSERT INTO series(author_user_id,title,slug,description,cover_media_id,status,state,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8) RETURNING id::text`, p.ID, in.Title, in.Slug, in.Description, in.CoverMediaID, in.Status, in.State, now).Scan(&id)
		if err != nil {
			return nil, err
		}
		if err = seriesReplaceChapters(r.Context(), tx, id, in.ChapterPostIDs); err != nil {
			return nil, err
		}
		if err = interactionAudit(tx, r, p, "series.create", "series", id, ""); err != nil {
			return nil, err
		}
		return a.privateSeries(r.Context(), tx, p, id)
	})
	if err == nil {
		JSON(w, 201, value)
	}
	return err
}

func (a *App) updateSeries(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if contentID(id) != nil {
		return NotFound()
	}
	version, err := IfMatch(r)
	if err != nil {
		return err
	}
	in, err := decodeSeries(r)
	if err != nil {
		return err
	}
	value, err := a.Write(r, []string{"author", "owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		var author, slug string
		var actual int64
		err := tx.QueryRow(r.Context(), `SELECT author_user_id::text,version,slug FROM series WHERE id=$1 AND deleted_at_ms IS NULL AND (author_user_id=$2 OR $3) FOR UPDATE`, id, p.ID, p.Role == "owner").Scan(&author, &actual, &slug)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, NotFound()
		}
		if err != nil {
			return nil, err
		}
		if actual != version {
			return nil, E(409, "VERSION_CONFLICT", "专栏已在其他页面修改，请重新读取后再保存")
		}
		if err = seriesValidateReferences(r.Context(), tx, author, in); err != nil {
			return nil, err
		}
		if in.Slug == "" {
			in.Slug = slug
		}
		if _, err = tx.Exec(r.Context(), `UPDATE series SET title=$2,slug=$3,description=$4,cover_media_id=$5,status=$6,state=$7,version=version+1,updated_at_ms=$8 WHERE id=$1`, id, in.Title, in.Slug, in.Description, in.CoverMediaID, in.Status, in.State, NowMillis()); err != nil {
			return nil, err
		}
		if err = seriesReplaceChapters(r.Context(), tx, id, in.ChapterPostIDs); err != nil {
			return nil, err
		}
		if err = interactionAudit(tx, r, p, "series.update", "series", id, ""); err != nil {
			return nil, err
		}
		return a.privateSeries(r.Context(), tx, p, id)
	})
	if err == nil {
		JSON(w, 200, value)
	}
	return err
}

func (a *App) deleteSeries(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if contentID(id) != nil {
		return NotFound()
	}
	version, err := IfMatch(r)
	if err != nil {
		return err
	}
	value, err := a.Write(r, []string{"author", "owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		var actual int64
		err := tx.QueryRow(r.Context(), `SELECT version FROM series WHERE id=$1 AND deleted_at_ms IS NULL AND (author_user_id=$2 OR $3) FOR UPDATE`, id, p.ID, p.Role == "owner").Scan(&actual)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, NotFound()
		}
		if err != nil {
			return nil, err
		}
		if actual != version {
			return nil, E(409, "VERSION_CONFLICT", "专栏已在其他页面修改，请重新读取后再删除")
		}
		if _, err = tx.Exec(r.Context(), `UPDATE series SET status='draft',cover_media_id=NULL,deleted_at_ms=$2,updated_at_ms=$2,version=version+1 WHERE id=$1`, id, NowMillis()); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(r.Context(), `DELETE FROM series_chapters WHERE series_id=$1`, id); err != nil {
			return nil, err
		}
		if err = interactionAudit(tx, r, p, "series.delete", "series", id, ""); err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "deleted": true, "version": version + 1}, nil
	})
	if err == nil {
		JSON(w, 200, value)
	}
	return err
}
