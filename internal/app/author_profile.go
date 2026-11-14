package app

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

type authorPageInput struct {
	BackgroundPreset  string  `json:"background_preset"`
	BackgroundMediaID *string `json:"background_media_id"`
}

var authorMediaID = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

func (a *App) RegisterAuthorProfiles() {
	a.Handle("GET /api/v1/me/author-page", a.authorPageGet)
	a.Handle("PUT /api/v1/me/author-page", a.authorPagePut)
}

func authorPageDTO(preset string, media *string, version int64, private bool) map[string]any {
	image := ""
	if media != nil {
		image = "/media/" + *media
	}
	out := map[string]any{"background_preset": preset, "background_url": image}
	if private {
		out["background_media_id"] = media
		out["version"] = version
	}
	return out
}

// AuthorPage returns only saved presentation choices. Backgrounds are local
// media references, never arbitrary CSS or remote URLs.
func (a *App) AuthorPage(ctx context.Context, id string) (map[string]any, error) {
	if !positiveID(id) {
		return nil, NotFound()
	}
	var preset string
	var media *string
	e := a.Pool.QueryRow(ctx, `SELECT COALESCE(p.background_preset,'paper'),CASE WHEN m.state='ready' THEN p.background_media_id END FROM users u LEFT JOIN author_page_profiles p ON p.user_id=u.id LEFT JOIN media_assets m ON m.id=p.background_media_id WHERE u.id=$1 AND u.disabled_at_ms IS NULL`, id).Scan(&preset, &media)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, NotFound()
	}
	if e != nil {
		return nil, e
	}
	return authorPageDTO(preset, media, 0, false), nil
}

// AuthorPosts copies the query and binds scope to the route identity, including
// text searches and their signed pagination cursors.
func (a *App) AuthorPosts(ctx context.Context, id string, query url.Values) (map[string]any, error) {
	if !positiveID(id) {
		return nil, NotFound()
	}
	q := url.Values{}
	for key, values := range query {
		q[key] = append([]string(nil), values...)
	}
	q.Set("author_id", id)
	if strings.TrimSpace(q.Get("q")) != "" {
		return a.SearchPosts(ctx, q)
	}
	return a.PublicPosts(ctx, q)
}

func (a *App) authorPageGet(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r)
	if e != nil {
		return e
	}
	var preset string
	var media *string
	var version int64
	e = a.Pool.QueryRow(r.Context(), `SELECT COALESCE(p.background_preset,'paper'),p.background_media_id,COALESCE(p.version,1) FROM users u LEFT JOIN author_page_profiles p ON p.user_id=u.id WHERE u.id=$1`, p.ID).Scan(&preset, &media, &version)
	if e != nil {
		return e
	}
	w.Header().Set("ETag", `"`+intString(version)+`"`)
	JSON(w, 200, authorPageDTO(preset, media, version, true))
	return nil
}

func (a *App) authorPagePut(w http.ResponseWriter, r *http.Request) error {
	version, e := IfMatch(r)
	if e != nil {
		return e
	}
	var in authorPageInput
	if e = Decode(r, &in); e != nil {
		return e
	}
	if in.BackgroundPreset != "paper" && in.BackgroundPreset != "mist" && in.BackgroundPreset != "ink" && in.BackgroundPreset != "rose" {
		return E(400, "VALIDATION_FAILED", "请选择可用的主页背景配色")
	}
	if in.BackgroundMediaID != nil && !authorMediaID.MatchString(*in.BackgroundMediaID) {
		return E(400, "VALIDATION_FAILED", "背景图片ID无效，请重新上传图片")
	}
	value, e := a.Write(r, nil, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		if _, e := tx.Exec(r.Context(), `INSERT INTO author_page_profiles(user_id,updated_at_ms) VALUES($1,$2) ON CONFLICT DO NOTHING`, p.ID, NowMillis()); e != nil {
			return nil, e
		}
		var actual int64
		if e := tx.QueryRow(r.Context(), `SELECT version FROM author_page_profiles WHERE user_id=$1 FOR UPDATE`, p.ID).Scan(&actual); e != nil {
			return nil, e
		}
		if actual != version {
			return nil, E(409, "VERSION_CONFLICT", "主页背景已在其他页面修改，请重新读取后再保存")
		}
		if in.BackgroundMediaID != nil {
			var owner, state string
			e := tx.QueryRow(r.Context(), `SELECT owner_user_id::text,state FROM media_assets WHERE id=$1 FOR UPDATE`, *in.BackgroundMediaID).Scan(&owner, &state)
			if errors.Is(e, pgx.ErrNoRows) || (e == nil && (owner != p.ID || state != "ready")) {
				return nil, E(400, "VALIDATION_FAILED", "背景必须使用自己已完成上传的图片")
			}
			if e != nil {
				return nil, e
			}
		}
		if _, e := tx.Exec(r.Context(), `UPDATE author_page_profiles SET background_preset=$2,background_media_id=$3,version=version+1,updated_at_ms=$4 WHERE user_id=$1`, p.ID, in.BackgroundPreset, in.BackgroundMediaID, NowMillis()); e != nil {
			return nil, e
		}
		if e := interactionAudit(tx, r, p, "user.update", "user", p.ID, "作者主页背景"); e != nil {
			return nil, e
		}
		return authorPageDTO(in.BackgroundPreset, in.BackgroundMediaID, version+1, true), nil
	})
	if e != nil {
		return e
	}
	JSON(w, 200, value)
	return nil
}
