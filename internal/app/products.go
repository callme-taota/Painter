package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const productLimit = 200

// Only the migration-marked built-in Painter record receives these defaults.
// User uploads take precedence and no asset URL is accepted from API inputs.
const builtinPainterIconURL = "/assets/images/painter-logo.png"
const builtinPainterCoverURL = "/assets/images/products/painter-preview-20261115.webp"

type productRepository struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}
type productAction struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}
type productLinks struct {
	Main         productAction       `json:"main"`
	DocsURL      string              `json:"docs_url"`
	Repositories []productRepository `json:"repositories"`
}
type productInput struct {
	Slug            string       `json:"slug"`
	Name            string       `json:"name"`
	Summary         string       `json:"summary"`
	Markdown        string       `json:"markdown"`
	IconMediaID     *string      `json:"icon_media_id"`
	CoverMediaID    *string      `json:"cover_media_id"`
	GalleryMediaIDs []string     `json:"gallery_media_ids"`
	Platforms       []string     `json:"platforms"`
	State           string       `json:"state"`
	Published       bool         `json:"published"`
	Featured        bool         `json:"featured"`
	Position        int          `json:"position"`
	Links           productLinks `json:"links"`
}

func (a *App) RegisterProducts() {
	a.Handle("GET /api/v1/products", func(w http.ResponseWriter, r *http.Request) error {
		v, err := a.PublicProducts(r.Context(), r.URL.Query())
		if err == nil {
			JSON(w, 200, v)
		}
		return err
	})
	a.Handle("GET /api/v1/products/{slug}", func(w http.ResponseWriter, r *http.Request) error {
		v, err := a.PublicProduct(r.Context(), r.PathValue("slug"))
		if err == nil {
			JSON(w, 200, v)
		}
		return err
	})
	a.Handle("GET /api/v1/admin/products", a.productsAdminList)
	a.Handle("GET /api/v1/admin/products/{id}", a.productAdminGet)
	a.Handle("POST /api/v1/admin/products", a.productCreate)
	a.Handle("PUT /api/v1/admin/products/{id}", a.productUpdate)
	a.Handle("DELETE /api/v1/admin/products/{id}", a.productDelete)
}

// Explicit public projection: publication cannot be widened by a cookie or a
// query parameter. Repository links are inert destinations, never fetch jobs.
var productPublicJSON = `jsonb_build_object('id',p.id::text,'slug',p.slug,'name',p.name,'summary',p.summary,
 'route_url','/products/'||p.slug,'platforms',p.platforms,'state',p.state,'featured',p.featured,'links',p.links,
 'icon_url',COALESCE((SELECT '/media/'||m.media_id FROM product_media m WHERE m.product_id=p.id AND m.kind='icon'),CASE WHEN p.builtin_key='painter' THEN '` + builtinPainterIconURL + `' ELSE '' END),
 'cover_url',COALESCE((SELECT '/media/'||m.media_id FROM product_media m WHERE m.product_id=p.id AND m.kind='cover'),CASE WHEN p.builtin_key='painter' THEN '` + builtinPainterCoverURL + `' ELSE '' END),
 'gallery',COALESCE((SELECT jsonb_agg(jsonb_build_object('media_id',m.media_id,'url','/media/'||m.media_id,'alt',p.name||' 截图 '||(m.position+1)) ORDER BY m.position) FROM product_media m WHERE m.product_id=p.id AND m.kind='gallery'),'[]'::jsonb),
 'created_at',` + contentTimeSQL("p.created_at_ms") + `,'updated_at',` + contentTimeSQL("p.updated_at_ms") + `)`
var productAdminListJSON = productPublicJSON + `||jsonb_build_object('published',p.published,'position',p.position,'version',p.version,
 'icon_media_id',(SELECT m.media_id FROM product_media m WHERE m.product_id=p.id AND m.kind='icon'),
 'cover_media_id',(SELECT m.media_id FROM product_media m WHERE m.product_id=p.id AND m.kind='cover'),
 'gallery_media_ids',COALESCE((SELECT jsonb_agg(m.media_id ORDER BY m.position) FROM product_media m WHERE m.product_id=p.id AND m.kind='gallery'),'[]'::jsonb))`
var productPrivateJSON = productAdminListJSON + `||jsonb_build_object('markdown',p.markdown,'sanitized_html',p.sanitized_html)`

func productFilters(q url.Values) (string, string, string, error) {
	state, platform, query := q.Get("state"), q.Get("platform"), strings.TrimSpace(q.Get("q"))
	if state == "all" {
		state = ""
	}
	if state != "" && state != "development" && state != "available" && state != "archived" {
		return "", "", "", E(400, "VALIDATION_FAILED", "产品状态无效")
	}
	if homeText(platform, 40, false) != nil || utf8.RuneCountInString(query) > 64 || homeText(query, 64, false) != nil {
		return "", "", "", E(400, "VALIDATION_FAILED", "平台或搜索词无效")
	}
	return state, platform, query, nil
}

func (a *App) PublicProducts(ctx context.Context, q url.Values) (map[string]any, error) {
	state, platform, query, err := productFilters(q)
	if err != nil {
		return nil, err
	}
	// One SQL snapshot keeps counts and listing consistent during withdrawal.
	var raw []byte
	err = a.Pool.QueryRow(ctx, `WITH visible AS MATERIALIZED (SELECT id,slug,builtin_key,name,summary,platforms,state,featured,position,links,created_at_ms,updated_at_ms FROM products WHERE published AND deleted_at_ms IS NULL), matched AS (SELECT * FROM visible WHERE ($1='' OR state=$1) AND ($2='' OR $2=ANY(platforms)) AND ($3='' OR name ILIKE $3 ESCAPE '!' OR summary ILIKE $3 ESCAPE '!'))
 SELECT jsonb_build_object('items',COALESCE((SELECT jsonb_agg(`+productPublicJSON+` ORDER BY p.featured DESC,p.position,p.id) FROM matched p),'[]'::jsonb),
 'total',(SELECT count(*) FROM visible),'result_count',(SELECT count(*) FROM matched),
 'available_count',(SELECT count(*) FROM visible WHERE state='available'),'development_count',(SELECT count(*) FROM visible WHERE state='development'),'archived_count',(SELECT count(*) FROM visible WHERE state='archived'),
 'platforms',COALESCE((SELECT jsonb_agg(x.platform ORDER BY x.platform) FROM (SELECT DISTINCT unnest(platforms) platform FROM visible) x),'[]'::jsonb))`, state, platform, func() string {
		if query == "" {
			return ""
		}
		return searchPattern(query)
	}()).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	err = json.Unmarshal(raw, &out)
	return out, err
}

func (a *App) PublicProduct(ctx context.Context, slug string) (map[string]any, error) {
	if len(slug) > 96 || !seriesSlug.MatchString(slug) {
		return nil, NotFound()
	}
	v, err := contentObject(ctx, a.Pool, `SELECT `+productPublicJSON+`||jsonb_build_object('sanitized_html',p.sanitized_html) FROM products p WHERE p.slug=$1 AND p.published AND p.deleted_at_ms IS NULL`, slug)
	if err != nil {
		return nil, err
	}
	v["sanitized_html"], err = a.ExpandInternalContentCards(ctx, webString(v["sanitized_html"]))
	return v, err
}

func (a *App) productsAdminList(w http.ResponseWriter, r *http.Request) error {
	if _, err := a.Require(r, "owner"); err != nil {
		return err
	}
	v, err := contentObjects(r.Context(), a.Pool, `SELECT `+productAdminListJSON+` FROM products p WHERE p.deleted_at_ms IS NULL ORDER BY p.position,p.id`)
	if err == nil {
		JSON(w, 200, map[string]any{"items": v, "max_items": productLimit})
	}
	return err
}
func (a *App) productAdminGet(w http.ResponseWriter, r *http.Request) error {
	if _, err := a.Require(r, "owner"); err != nil {
		return err
	}
	if !positiveID(r.PathValue("id")) {
		return NotFound()
	}
	v, err := productPrivate(r.Context(), a.Pool, r.PathValue("id"))
	if err == nil {
		JSON(w, 200, v)
	}
	return err
}
func productPrivate(ctx context.Context, db contentQuerier, id string) (map[string]any, error) {
	return contentObject(ctx, db, `SELECT `+productPrivateJSON+` FROM products p WHERE p.id=$1 AND p.deleted_at_ms IS NULL`, id)
}

func decodeProduct(r *http.Request) (productInput, Rendered, error) {
	var in productInput
	if err := Decode(r, &in); err != nil {
		return in, Rendered{}, err
	}
	if len(in.Slug) > 96 || !seriesSlug.MatchString(in.Slug) || strings.TrimSpace(in.Name) == "" || homeText(in.Name, 100, false) != nil || homeText(in.Summary, 600, true) != nil || in.Position < 0 || in.Position > 1000000 {
		return in, Rendered{}, E(400, "VALIDATION_FAILED", "产品名称、标识、简介或排序无效")
	}
	if strings.ContainsRune(in.Markdown, '\x00') {
		return in, Rendered{}, E(400, "VALIDATION_FAILED", "产品正文不能包含空字符")
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.State != "development" && in.State != "available" && in.State != "archived" {
		return in, Rendered{}, E(400, "VALIDATION_FAILED", "请选择开发中、可用或已归档状态")
	}
	if len(in.Platforms) > 12 || len(in.GalleryMediaIDs) > 12 || len(in.Links.Repositories) > 12 {
		return in, Rendered{}, E(400, "VALIDATION_FAILED", "平台、截图和代码仓库分别最多12项")
	}
	seen := map[string]bool{}
	for i, s := range in.Platforms {
		s = strings.TrimSpace(s)
		if s == "" || homeText(s, 40, false) != nil || seen[Normalize(s)] {
			return in, Rendered{}, E(400, "VALIDATION_FAILED", "平台不能为空或重复")
		}
		seen[Normalize(s)] = true
		in.Platforms[i] = s
	}
	for _, p := range []*string{in.IconMediaID, in.CoverMediaID} {
		if p != nil && !mediaReference.MatchString("/media/"+*p) {
			return in, Rendered{}, E(400, "INVALID_MEDIA", "图片标识无效")
		}
	}
	seen = map[string]bool{}
	for _, id := range in.GalleryMediaIDs {
		if !mediaReference.MatchString("/media/"+id) || seen[id] {
			return in, Rendered{}, E(400, "INVALID_MEDIA", "截图图片标识无效或重复")
		}
		seen[id] = true
	}
	if in.Links.Main.Type == "" {
		in.Links.Main.Type = "web"
	}
	if in.Links.Main.Type != "web" && in.Links.Main.Type != "download" && in.Links.Main.Type != "store" && in.Links.Main.Type != "other" {
		return in, Rendered{}, E(400, "VALIDATION_FAILED", "产品主链接类型无效")
	}
	if err := homeURL(in.Links.Main.URL); err != nil {
		return in, Rendered{}, err
	}
	if err := homeURL(in.Links.DocsURL); err != nil {
		return in, Rendered{}, err
	}
	seen = map[string]bool{}
	for i, repo := range in.Links.Repositories {
		if strings.TrimSpace(repo.Label) == "" || homeText(repo.Label, 80, false) != nil {
			return in, Rendered{}, E(400, "VALIDATION_FAILED", "仓库名称无效")
		}
		target, err := friendURL(repo.URL, true)
		if err != nil {
			return in, Rendered{}, err
		}
		if seen[target] {
			return in, Rendered{}, E(400, "VALIDATION_FAILED", "代码仓库链接不能重复")
		}
		seen[target] = true
		in.Links.Repositories[i].Label = strings.TrimSpace(repo.Label)
		in.Links.Repositories[i].URL = target
	}
	if in.Platforms == nil {
		in.Platforms = []string{}
	}
	if in.GalleryMediaIDs == nil {
		in.GalleryMediaIDs = []string{}
	}
	if in.Links.Repositories == nil {
		in.Links.Repositories = []productRepository{}
	}
	rendered, err := RenderMarkdown(in.Markdown)
	if len(rendered.MediaIDs) > 100 {
		return in, Rendered{}, E(400, "INVALID_MEDIA", "正文最多100张图片")
	}
	return in, rendered, err
}

// Lock the sorted union before replacing references, sharing the same media
// lock with deletion and reconciliation. An owner may retain existing images
// uploaded by a former owner, but can only attach their own new images.
func productLockMedia(ctx context.Context, tx pgx.Tx, p Principal, id string, in productInput, rendered Rendered) error {
	old := map[string]bool{}
	if id != "" {
		rows, err := tx.Query(ctx, `SELECT media_id FROM product_media WHERE product_id=$1`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var m string
			if err = rows.Scan(&m); err != nil {
				rows.Close()
				return err
			}
			old[m] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	wanted := map[string]bool{}
	for _, v := range []*string{in.IconMediaID, in.CoverMediaID} {
		if v != nil {
			wanted[*v] = true
		}
	}
	for _, v := range append(append([]string{}, in.GalleryMediaIDs...), rendered.MediaIDs...) {
		wanted[v] = true
	}
	all := map[string]bool{}
	for v := range old {
		all[v] = true
	}
	for v := range wanted {
		all[v] = true
	}
	ids := make([]string, 0, len(all))
	for v := range all {
		ids = append(ids, v)
	}
	sort.Strings(ids)
	for _, v := range ids {
		var owner, state, mime string
		err := tx.QueryRow(ctx, `SELECT owner_user_id::text,state,mime_type FROM media_assets WHERE id=$1 FOR UPDATE`, v).Scan(&owner, &state, &mime)
		if errors.Is(err, pgx.ErrNoRows) {
			return E(400, "INVALID_MEDIA", "产品图片不存在")
		}
		if err != nil {
			return err
		}
		if wanted[v] && (state != "ready" || (!old[v] && owner != p.ID) || !(mime == "image/png" || mime == "image/jpeg" || mime == "image/gif" || mime == "image/webp")) {
			return E(400, "INVALID_MEDIA", "产品图片必须已上传完成且属于当前账号")
		}
	}
	return nil
}
func productReplaceMedia(ctx context.Context, tx pgx.Tx, id string, in productInput, rendered Rendered) error {
	if _, err := tx.Exec(ctx, `DELETE FROM product_media WHERE product_id=$1`, id); err != nil {
		return err
	}
	groups := map[string][]string{"gallery": in.GalleryMediaIDs, "body": rendered.MediaIDs}
	if in.IconMediaID != nil {
		groups["icon"] = []string{*in.IconMediaID}
	}
	if in.CoverMediaID != nil {
		groups["cover"] = []string{*in.CoverMediaID}
	}
	for _, kind := range []string{"icon", "cover", "gallery", "body"} {
		for pos, media := range groups[kind] {
			if _, err := tx.Exec(ctx, `INSERT INTO product_media(product_id,media_id,kind,position) VALUES($1,$2,$3,$4)`, id, media, kind, pos); err != nil {
				return err
			}
		}
	}
	return nil
}
func (a *App) productCreate(w http.ResponseWriter, r *http.Request) error {
	if err := homeAuthorize(a, r); err != nil {
		return err
	}
	in, rendered, err := decodeProduct(r)
	if err != nil {
		return err
	}
	v, err := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		// A fixed transaction lock makes the bounded collection limit race-safe.
		if _, err := tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(7284923151125)`); err != nil {
			return nil, err
		}
		var n int
		if err := tx.QueryRow(r.Context(), `SELECT count(*) FROM products WHERE deleted_at_ms IS NULL`).Scan(&n); err != nil {
			return nil, err
		}
		if n >= productLimit {
			return nil, E(409, "PRODUCT_LIMIT", "产品最多200项")
		}
		if err := productLockMedia(r.Context(), tx, p, "", in, rendered); err != nil {
			return nil, err
		}
		now := NowMillis()
		links, _ := json.Marshal(in.Links)
		var id string
		err := tx.QueryRow(r.Context(), `INSERT INTO products(slug,name,summary,markdown,sanitized_html,platforms,state,published,featured,position,links,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$12) RETURNING id::text`, in.Slug, in.Name, in.Summary, in.Markdown, rendered.HTML, in.Platforms, in.State, in.Published, in.Featured, in.Position, links, now).Scan(&id)
		if err != nil {
			return nil, err
		}
		if err = productReplaceMedia(r.Context(), tx, id, in, rendered); err != nil {
			return nil, err
		}
		if err = audit(r.Context(), tx, p, "product.create", "product", id, ""); err != nil {
			return nil, err
		}
		return productPrivate(r.Context(), tx, id)
	})
	if err == nil {
		JSON(w, 201, v)
	}
	return err
}
func (a *App) productUpdate(w http.ResponseWriter, r *http.Request) error {
	if err := homeAuthorize(a, r); err != nil {
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
	in, rendered, err := decodeProduct(r)
	if err != nil {
		return err
	}
	v, err := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		var actual int64
		var slug string
		if err := tx.QueryRow(r.Context(), `SELECT version,slug FROM products WHERE id=$1 AND deleted_at_ms IS NULL FOR UPDATE`, id).Scan(&actual, &slug); err != nil {
			return nil, err
		}
		if actual != version {
			return nil, E(409, "VERSION_CONFLICT", "产品已在其他页面修改，请重新载入后保存")
		}
		if slug != in.Slug {
			return nil, E(400, "IMMUTABLE_SLUG", "产品标识创建后保持不变，修改名称不会改变公开地址")
		}
		if err := productLockMedia(r.Context(), tx, p, id, in, rendered); err != nil {
			return nil, err
		}
		links, _ := json.Marshal(in.Links)
		if _, err := tx.Exec(r.Context(), `UPDATE products SET name=$2,summary=$3,markdown=$4,sanitized_html=$5,platforms=$6,state=$7,published=$8,featured=$9,position=$10,links=$11,version=version+1,updated_at_ms=$12 WHERE id=$1`, id, in.Name, in.Summary, in.Markdown, rendered.HTML, in.Platforms, in.State, in.Published, in.Featured, in.Position, links, NowMillis()); err != nil {
			return nil, err
		}
		if err := productReplaceMedia(r.Context(), tx, id, in, rendered); err != nil {
			return nil, err
		}
		if err := audit(r.Context(), tx, p, "product.update", "product", id, ""); err != nil {
			return nil, err
		}
		return productPrivate(r.Context(), tx, id)
	})
	if err == nil {
		JSON(w, 200, v)
	}
	return err
}
func (a *App) productDelete(w http.ResponseWriter, r *http.Request) error {
	if err := homeAuthorize(a, r); err != nil {
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
	v, err := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		var actual int64
		if err := tx.QueryRow(r.Context(), `SELECT version FROM products WHERE id=$1 AND deleted_at_ms IS NULL FOR UPDATE`, id).Scan(&actual); err != nil {
			return nil, err
		}
		if version != actual {
			return nil, E(409, "VERSION_CONFLICT", "产品已在其他页面修改，请重新载入后删除")
		}
		if err := productLockMedia(r.Context(), tx, p, id, productInput{}, Rendered{}); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(r.Context(), `DELETE FROM product_media WHERE product_id=$1`, id); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE products SET published=false,featured=false,version=version+1,deleted_at_ms=$2,updated_at_ms=$2 WHERE id=$1`, id, NowMillis()); err != nil {
			return nil, err
		}
		return map[string]any{"deleted": true, "id": id, "version": actual + 1}, audit(r.Context(), tx, p, "product.delete", "product", id, "")
	})
	if err == nil {
		JSON(w, 200, v)
	}
	return err
}
