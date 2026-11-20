package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

type adminSocial struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}
type adminProfile struct {
	Version     int64         `json:"version"`
	DisplayName string        `json:"display_name"`
	Tagline     string        `json:"tagline"`
	About       string        `json:"about_markdown"`
	Avatar      *string       `json:"avatar_media_id"`
	Social      []adminSocial `json:"social_links"`
	Footer      string        `json:"footer_text"`
	ICP         string        `json:"icp_number"`
	SEO         string        `json:"seo_description"`
	IntroName   *string       `json:"intro_name,omitempty"`
	IntroRoles  *[]string     `json:"intro_roles,omitempty"`
}
type adminSettings struct {
	Version        int64           `json:"version"`
	Registration   bool            `json:"registration_enabled"`
	ExternalLogin  *bool           `json:"external_login_enabled,omitempty"`
	Comments       bool            `json:"comments_enabled"`
	Collections    bool            `json:"collections_enabled"`
	Follows        bool            `json:"follows_enabled"`
	Analytics      bool            `json:"analytics_enabled"`
	Mail           bool            `json:"mail_enabled"`
	SMTPHost       string          `json:"smtp_host"`
	SMTPPort       int64           `json:"smtp_port"`
	SMTPUsername   string          `json:"smtp_username"`
	SMTPPassword   *string         `json:"smtp_password,omitempty"`
	SMTPConfigured bool            `json:"smtp_password_configured"`
	MailFrom       string          `json:"mail_from"`
	Timezone       string          `json:"timezone"`
	RSS            *bool           `json:"rss_enabled,omitempty"`
	Floating       json.RawMessage `json:"floating_controls,omitempty"`
}

func (a *App) RegisterAdmin() {
	a.Handle("GET /api/v1/site", a.adminPublicSite)
	a.Handle("GET /api/v1/admin/site-profile", a.adminGetProfile)
	a.Handle("PUT /api/v1/admin/site-profile", a.adminPutProfile)
	a.Handle("GET /api/v1/admin/settings", a.adminGetSettings)
	a.Handle("PUT /api/v1/admin/settings", a.adminPutSettings)
	for _, plural := range []string{"categories", "tags"} {
		table := plural
		a.Handle("GET /api/v1/"+table, func(w http.ResponseWriter, r *http.Request) error { return a.adminTaxonomyList(w, r, table) })
		a.Handle("GET /api/v1/"+table+"/{id}", func(w http.ResponseWriter, r *http.Request) error { return a.adminTaxonomyGet(w, r, table) })
		a.Handle("GET /api/v1/admin/"+table, func(w http.ResponseWriter, r *http.Request) error { return a.adminTaxonomyList(w, r, table) })
		a.Handle("POST /api/v1/admin/"+table, func(w http.ResponseWriter, r *http.Request) error { return a.adminTaxonomyWrite(w, r, table, true) })
		a.Handle("PATCH /api/v1/admin/"+table+"/{id}", func(w http.ResponseWriter, r *http.Request) error { return a.adminTaxonomyWrite(w, r, table, false) })
	}
	a.Handle("GET /api/v1/admin/users", a.adminUsers)
	a.Handle("PATCH /api/v1/admin/users/{id}", a.adminUserUpdate)
	a.Handle("GET /api/v1/admin/comments", a.adminComments)
	a.Handle("PUT /api/v1/admin/comments/{id}/visibility", a.adminCommentVisibility)
	a.Handle("GET /api/v1/admin/storage", a.adminStorage)
	a.Handle("GET /api/v1/admin/jobs", a.adminJobs)
	a.Handle("GET /api/v1/admin/audit", a.adminAudit)
}
func (a *App) adminProfileRead(r *http.Request) (adminProfile, error) {
	var d adminProfile
	var social, roles []byte
	var name string
	e := a.Pool.QueryRow(r.Context(), `SELECT version,display_name,tagline,about_markdown,avatar_media_id,social_links_json,footer_text,icp_number,seo_description,intro_name,intro_roles_json FROM site_profile WHERE id=1`).Scan(&d.Version, &d.DisplayName, &d.Tagline, &d.About, &d.Avatar, &social, &d.Footer, &d.ICP, &d.SEO, &name, &roles)
	if e != nil {
		return d, e
	}
	d.IntroName = &name
	var roleList []string
	if e = json.Unmarshal(roles, &roleList); e != nil {
		return d, e
	}
	d.IntroRoles = &roleList
	e = json.Unmarshal(social, &d.Social)
	return d, e
}
func (a *App) adminPublicSite(w http.ResponseWriter, r *http.Request) error {
	d, e := a.adminProfileRead(r)
	if e != nil {
		return e
	}
	var registration, external, comments, collections, follows, analytics bool
	e = a.Pool.QueryRow(r.Context(), `SELECT registration_enabled,external_login_enabled,comments_enabled,collections_enabled,follows_enabled,analytics_enabled FROM site_settings WHERE id=1`).Scan(&registration, &external, &comments, &collections, &follows, &analytics)
	if e != nil {
		return e
	}
	avatar := ""
	if d.Avatar != nil {
		avatar = "/media/" + *d.Avatar
	}
	rss, controls, e := a.siteExperience(r.Context())
	if e != nil {
		return e
	}
	JSON(w, 200, map[string]any{"display_name": d.DisplayName, "tagline": d.Tagline, "intro_name": d.IntroName, "intro_roles": d.IntroRoles, "about_markdown": d.About, "avatar_url": avatar, "social_links": d.Social, "footer_text": d.Footer, "icp_number": d.ICP, "seo_description": d.SEO, "registration_enabled": registration, "external_login_enabled": external, "comments_enabled": comments, "collections_enabled": collections, "follows_enabled": follows, "analytics_enabled": analytics, "rss_enabled": rss, "floating_controls": controls})
	return nil
}
func (a *App) adminGetProfile(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	d, e := a.adminProfileRead(r)
	if e != nil {
		return e
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, d)
	return nil
}
func (a *App) adminPutProfile(w http.ResponseWriter, r *http.Request) error {
	var d adminProfile
	if e := Decode(r, &d); e != nil {
		return e
	}
	version, e := IfMatch(r)
	if e != nil {
		return e
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(d.DisplayName)); n < 1 || n > 100 || utf8.RuneCountInString(d.Tagline) > 300 || len(d.About) > 1<<20 || utf8.RuneCountInString(d.SEO) > 500 || len(d.Social) > 20 {
		return E(400, "VALIDATION_FAILED", "站点资料超出长度限制")
	}
	if d.IntroName != nil {
		name := strings.TrimSpace(*d.IntroName)
		d.IntroName = &name
		if utf8.RuneCountInString(name) > 80 {
			return E(400, "VALIDATION_FAILED", "介绍姓名最多80字")
		}
	}
	if d.IntroRoles != nil {
		if len(*d.IntroRoles) > 12 {
			return E(400, "VALIDATION_FAILED", "身份标签最多12个")
		}
		for i, role := range *d.IntroRoles {
			role = strings.TrimSpace(role)
			if utf8.RuneCountInString(role) < 1 || utf8.RuneCountInString(role) > 60 {
				return E(400, "VALIDATION_FAILED", "每个身份标签需为1到60字")
			}
			(*d.IntroRoles)[i] = role
		}
	}
	for _, s := range d.Social {
		u, e := url.Parse(s.URL)
		if e != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || utf8.RuneCountInString(s.Label) > 100 {
			return E(400, "VALIDATION_FAILED", "社交链接必须为有效HTTP链接")
		}
	}
	if d.Social == nil {
		d.Social = []adminSocial{}
	}
	b, _ := json.Marshal(d.Social)
	var roles any
	if d.IntroRoles != nil {
		encoded, _ := json.Marshal(*d.IntroRoles)
		roles = encoded
	}
	data, e := a.Write(r, []string{"owner"}, true, false, func(tx pgx.Tx, p Principal) (any, error) {
		var current int64
		if e := tx.QueryRow(r.Context(), `SELECT version FROM site_profile WHERE id=1 FOR UPDATE`).Scan(&current); e != nil {
			return nil, e
		}
		if current != version {
			return nil, E(409, "VERSION_CONFLICT", "站点资料已改变")
		}
		if d.Avatar != nil && *d.Avatar != "" {
			var state, owner string
			if e := tx.QueryRow(r.Context(), `SELECT state,owner_user_id::text FROM media_assets WHERE id=$1 FOR UPDATE`, *d.Avatar).Scan(&state, &owner); e == pgx.ErrNoRows {
				return nil, NotFound()
			} else if e != nil {
				return nil, e
			}
			if state != "ready" {
				return nil, E(409, "MEDIA_NOT_READY", "头像未就绪")
			}
		} else {
			d.Avatar = nil
		}
		var v int64
		if e := tx.QueryRow(r.Context(), `UPDATE site_profile SET version=version+1,display_name=$2,tagline=$3,about_markdown=$4,avatar_media_id=$5,social_links_json=$6,footer_text=$7,icp_number=$8,seo_description=$9,updated_at_ms=$10,intro_name=COALESCE($11,intro_name),intro_roles_json=COALESCE($12::jsonb,intro_roles_json) WHERE id=1 AND version=$1 RETURNING version`, version, strings.TrimSpace(d.DisplayName), d.Tagline, d.About, d.Avatar, b, d.Footer, d.ICP, d.SEO, NowMillis(), d.IntroName, roles).Scan(&v); e != nil {
			return nil, e
		}
		if e := interactionAudit(tx, r, p, "site.profile_update", "site", "1", ""); e != nil {
			return nil, e
		}
		d.Version = v
		return d, nil
	})
	if e == nil {
		JSON(w, 200, data)
	}
	return e
}
func (a *App) adminGetSettings(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	var d adminSettings
	e := a.Pool.QueryRow(r.Context(), `SELECT version,external_login_enabled,registration_enabled,comments_enabled,collections_enabled,follows_enabled,analytics_enabled,mail_enabled,smtp_host,smtp_port,smtp_username,smtp_password_ciphertext IS NOT NULL,mail_from,timezone,rss_enabled,floating_controls FROM site_settings WHERE id=1`).Scan(&d.Version, &d.ExternalLogin, &d.Registration, &d.Comments, &d.Collections, &d.Follows, &d.Analytics, &d.Mail, &d.SMTPHost, &d.SMTPPort, &d.SMTPUsername, &d.SMTPConfigured, &d.MailFrom, &d.Timezone, &d.RSS, &d.Floating)
	if e != nil {
		return e
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, d)
	return nil
}
func (a *App) adminEncryptSecret(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	block, e := aes.NewCipher(a.Config.EncryptionKey)
	if e != nil {
		return nil, e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = io.ReadFull(rand.Reader, nonce); e != nil {
		return nil, e
	}
	return g.Seal(nonce, nonce, []byte(value), []byte("painter-next:smtp:v1")), nil
}
func (a *App) adminPutSettings(w http.ResponseWriter, r *http.Request) error {
	var d adminSettings
	if e := Decode(r, &d); e != nil {
		return e
	}
	version, e := IfMatch(r)
	if e != nil {
		return e
	}
	if d.SMTPPort < 1 || d.SMTPPort > 65535 || (d.Timezone != "" && d.Timezone != "Asia/Shanghai") || len(d.SMTPHost) > 255 || len(d.SMTPUsername) > 255 || len(d.MailFrom) > 255 {
		return E(400, "VALIDATION_FAILED", "通知设置无效")
	}
	d.Floating, e = normalizeFloatingControls(d.Floating)
	if e != nil {
		return e
	}
	var secret []byte
	if d.SMTPPassword != nil {
		if len(*d.SMTPPassword) > 4096 {
			return E(400, "VALIDATION_FAILED", "秘密过长")
		}
		secret, e = a.adminEncryptSecret(*d.SMTPPassword)
		if e != nil {
			return e
		}
	}
	data, e := a.Write(r, []string{"owner"}, true, false, func(tx pgx.Tx, p Principal) (any, error) {
		var v int64
		var configured bool
		e := tx.QueryRow(r.Context(), `UPDATE site_settings SET version=version+1,registration_enabled=$2,comments_enabled=$3,collections_enabled=$4,follows_enabled=$5,analytics_enabled=$6,mail_enabled=$7,smtp_host=$8,smtp_port=$9,smtp_username=$10,smtp_password_ciphertext=CASE WHEN $11 THEN $12::bytea ELSE smtp_password_ciphertext END,mail_from=$13,updated_at_ms=$14,rss_enabled=COALESCE($15::boolean,rss_enabled),floating_controls=COALESCE($16::jsonb,floating_controls),external_login_enabled=COALESCE($17::boolean,external_login_enabled) WHERE id=1 AND version=$1 RETURNING version,smtp_password_ciphertext IS NOT NULL,rss_enabled,floating_controls,external_login_enabled`, version, d.Registration, d.Comments, d.Collections, d.Follows, d.Analytics, d.Mail, d.SMTPHost, d.SMTPPort, d.SMTPUsername, d.SMTPPassword != nil, secret, d.MailFrom, NowMillis(), d.RSS, d.Floating, d.ExternalLogin).Scan(&v, &configured, &d.RSS, &d.Floating, &d.ExternalLogin)
		if e == pgx.ErrNoRows {
			return nil, E(409, "VERSION_CONFLICT", "站点设置已改变")
		}
		if e != nil {
			return nil, e
		}
		if d.ExternalLogin != nil && !*d.ExternalLogin {
			now := NowMillis()
			if _, e = tx.Exec(r.Context(), `UPDATE sessions SET revoked_at_ms=$1 WHERE revoked_at_ms IS NULL AND user_id IN(SELECT id FROM users WHERE role<>'owner')`, now); e != nil {
				return nil, e
			}
			if _, e = tx.Exec(r.Context(), `UPDATE mfa_login_challenges SET consumed_at_ms=$1 WHERE consumed_at_ms IS NULL AND user_id IN(SELECT id FROM users WHERE role<>'owner')`, now); e != nil {
				return nil, e
			}
		}
		if e = interactionAudit(tx, r, p, "site.settings_update", "site", "1", ""); e != nil {
			return nil, e
		}
		d.Version = v
		d.SMTPPassword = nil
		d.SMTPConfigured = configured
		d.Timezone = "Asia/Shanghai"
		return d, nil
	})
	if e == nil {
		JSON(w, 200, data)
	}
	return e
}

const adminTaxonomyCountSQL = `(SELECT count(*) FROM effective_revision_tags rt JOIN public_posts pp ON pp.revision_id=rt.revision_id WHERE rt.tag_id=t.id)`

func (a *App) adminTaxonomyList(w http.ResponseWriter, r *http.Request, table string) error {
	// Legacy category reads are aliases of the one tag vocabulary.
	table = "tags"
	management := strings.Contains(r.URL.Path, "/admin/")
	if management {
		if _, e := a.Require(r, "owner"); e != nil {
			return e
		}
		w.Header().Set("Cache-Control", "private, no-store")
	}
	scope := table
	if management {
		scope = "admin:" + scope
	}
	c, e := a.interactionCursor(r, scope)
	if e != nil {
		return e
	}
	where := " AND t.archived_at_ms IS NULL"
	if management {
		where = ""
	}
	rows, e := a.Pool.Query(r.Context(), `SELECT t.id::text,t.name,t.description,t.archived_at_ms IS NOT NULL,`+adminTaxonomyCountSQL+` FROM `+table+` t WHERE t.id>$1::bigint AND NOT EXISTS(SELECT 1 FROM tag_aliases a WHERE a.tag_id=t.id AND a.canonical_tag_id<>t.id)`+where+` ORDER BY t.id LIMIT $2`, c.ID, Limit(r)+1)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []map[string]any{}
	next, last := "", ""
	for rows.Next() {
		var id, name, desc string
		var archived bool
		var count int64
		if e = rows.Scan(&id, &name, &desc, &archived, &count); e != nil {
			return e
		}
		if len(items) == Limit(r) {
			next = a.interactionNext(scope, last, 0)
			break
		}
		items = append(items, map[string]any{"id": id, "name": name, "description": desc, "archived": archived, "article_count": count})
		last = id
	}
	if e = rows.Err(); e != nil {
		return e
	}
	JSON(w, 200, interactionPage(items, next))
	return nil
}
func (a *App) adminTaxonomyGet(w http.ResponseWriter, r *http.Request, table string) error {
	id := r.PathValue("id")
	if e := interactionID(id); e != nil {
		return e
	}
	var e error
	if table == "categories" {
		e = a.Pool.QueryRow(r.Context(), "SELECT tag_id::text FROM category_tag_map WHERE category_id=$1", id).Scan(&id)
		if e == pgx.ErrNoRows {
			return NotFound()
		}
		if e != nil {
			return e
		}
	} else {
		e = a.Pool.QueryRow(r.Context(), "SELECT COALESCE((SELECT canonical_tag_id FROM tag_aliases WHERE tag_id=$1),$1::bigint)::text", id).Scan(&id)
		if e != nil {
			return e
		}
	}
	table = "tags"
	var name, desc string
	var count int64
	e = a.Pool.QueryRow(r.Context(), `SELECT t.name,t.description,`+adminTaxonomyCountSQL+` FROM `+table+` t WHERE t.id=$1 AND t.archived_at_ms IS NULL`, id).Scan(&name, &desc, &count)
	if e == pgx.ErrNoRows {
		return NotFound()
	}
	if e != nil {
		return e
	}
	JSON(w, 200, map[string]any{"id": id, "name": name, "description": desc, "article_count": count})
	return nil
}
func (a *App) adminTaxonomyWrite(w http.ResponseWriter, r *http.Request, table string, create bool) error {
	if table != "tags" {
		return E(http.StatusGone, "TAXONOMY_UNIFIED", "分类已合并到标签，请使用标签管理")
	}
	var d struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Archived    *bool   `json:"archived"`
	}
	if e := Decode(r, &d); e != nil {
		return e
	}
	if create && d.Name == nil {
		return E(400, "VALIDATION_FAILED", "名称必填")
	}
	if d.Name != nil {
		*d.Name = strings.TrimSpace(*d.Name)
		if n := utf8.RuneCountInString(*d.Name); n < 1 || n > 100 {
			return E(400, "VALIDATION_FAILED", "名称需1至100字")
		}
	}
	if d.Description != nil && utf8.RuneCountInString(*d.Description) > 2000 {
		return E(400, "VALIDATION_FAILED", "说明最多2000字")
	}
	data, e := a.Write(r, []string{"owner"}, true, create, func(tx pgx.Tx, p Principal) (any, error) {
		id := r.PathValue("id")
		if !create {
			if e := interactionID(id); e != nil {
				return nil, e
			}
			if e := tx.QueryRow(r.Context(), "SELECT COALESCE((SELECT canonical_tag_id FROM tag_aliases WHERE tag_id=$1),$1::bigint)::text", id).Scan(&id); e != nil {
				return nil, e
			}
		}
		if d.Name != nil {
			// Lock by normalized spelling so concurrent creates cannot introduce
			// a second visible tag with a Unicode/case-only name difference.
			if _, e := tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended('painter:tag:'||painter_taxonomy_key($1),0))", *d.Name); e != nil {
				return nil, e
			}
			var duplicate bool
			if e := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM tags t WHERE painter_taxonomy_key(t.name)=painter_taxonomy_key($1) AND ($2='' OR t.id<>NULLIF($2,'')::bigint) AND NOT EXISTS(SELECT 1 FROM tag_aliases a WHERE a.tag_id=t.id AND a.canonical_tag_id<>t.id))`, *d.Name, id).Scan(&duplicate); e != nil {
				return nil, e
			}
			if duplicate {
				return nil, E(409, "TAG_EXISTS", "同名标签已存在")
			}
		}
		now := NowMillis()
		action := strings.TrimSuffix(table, "s")
		if table == "categories" {
			action = "category"
		}
		if create {
			desc := ""
			if d.Description != nil {
				desc = *d.Description
			}
			if e := tx.QueryRow(r.Context(), `INSERT INTO `+table+`(name,description,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$3) RETURNING id::text`, *d.Name, desc, now).Scan(&id); e != nil {
				return nil, e
			}
			action += ".create"
		} else {
			if e := interactionID(id); e != nil {
				return nil, e
			}
			tag, err := tx.Exec(r.Context(), `UPDATE `+table+` SET name=COALESCE($2,name),description=COALESCE($3,description),archived_at_ms=CASE WHEN $4::boolean IS NULL THEN archived_at_ms WHEN $4 THEN COALESCE(archived_at_ms,$5) ELSE NULL END,updated_at_ms=$5 WHERE id=$1`, id, d.Name, d.Description, d.Archived, now)
			if err != nil {
				return nil, err
			}
			if tag.RowsAffected() == 0 {
				return nil, NotFound()
			}
			if d.Archived != nil && *d.Archived {
				action += ".archive"
			} else {
				action += ".update"
			}
		}
		kind := "tag"
		if table == "categories" {
			kind = "category"
		}
		if e := interactionAudit(tx, r, p, action, kind, id, ""); e != nil {
			return nil, e
		}
		var name, desc string
		var archived bool
		if e := tx.QueryRow(r.Context(), `SELECT name,description,archived_at_ms IS NOT NULL FROM `+table+` WHERE id=$1`, id).Scan(&name, &desc, &archived); e != nil {
			return nil, e
		}
		return map[string]any{"id": id, "name": name, "description": desc, "archived": archived}, nil
	})
	if e == nil {
		status := 200
		if create {
			status = 201
		}
		JSON(w, status, data)
	}
	return e
}
func (a *App) adminUsers(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	c, e := a.interactionCursor(r, "admin:users")
	if e != nil {
		return e
	}
	rows, e := a.Pool.Query(r.Context(), `SELECT id::text,username,display_name,avatar_url,role,disabled_at_ms,created_at_ms,version FROM users WHERE id>$1::bigint ORDER BY id LIMIT $2`, c.ID, Limit(r)+1)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []map[string]any{}
	next, last := "", ""
	for rows.Next() {
		var id, user, name, avatar, role string
		var disabled *int64
		var created, version int64
		if e = rows.Scan(&id, &user, &name, &avatar, &role, &disabled, &created, &version); e != nil {
			return e
		}
		if len(items) == Limit(r) {
			next = a.interactionNext("admin:users", last, 0)
			break
		}
		items = append(items, map[string]any{"id": id, "username": user, "display_name": name, "avatar_url": avatar, "role": role, "disabled": disabled != nil, "version": version, "created_at": interactionTime(created)})
		last = id
	}
	if e = rows.Err(); e != nil {
		return e
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, interactionPage(items, next))
	return nil
}
func (a *App) adminUserUpdate(w http.ResponseWriter, r *http.Request) error {
	var d struct {
		Role     *string `json:"role"`
		Disabled *bool   `json:"disabled"`
		Reason   string  `json:"reason"`
	}
	if e := Decode(r, &d); e != nil {
		return e
	}
	if d.Role != nil && *d.Role != "owner" && *d.Role != "author" && *d.Role != "reader" {
		return E(400, "VALIDATION_FAILED", "无效角色")
	}
	id := r.PathValue("id")
	if e := interactionID(id); e != nil {
		return e
	}
	data, e := a.Write(r, []string{"owner"}, true, true, func(tx pgx.Tx, p Principal) (any, error) {
		var role string
		var disabled *int64
		if e := tx.QueryRow(r.Context(), `SELECT role,disabled_at_ms FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&role, &disabled); e == pgx.ErrNoRows {
			return nil, NotFound()
		} else if e != nil {
			return nil, e
		}
		newrole := role
		if d.Role != nil {
			newrole = *d.Role
		}
		newdisabled := disabled
		if d.Disabled != nil {
			if *d.Disabled {
				now := NowMillis()
				newdisabled = &now
			} else {
				newdisabled = nil
			}
		}
		if role == "owner" && disabled == nil && (newrole != "owner" || newdisabled != nil) {
			var n int
			if e := tx.QueryRow(r.Context(), `SELECT count(*) FROM users WHERE role='owner' AND disabled_at_ms IS NULL AND id<>$1`, id).Scan(&n); e != nil {
				return nil, e
			}
			if n == 0 {
				return nil, E(409, "LAST_OWNER", "必须保留一个可用站长")
			}
		}
		changed := role != newrole || (disabled == nil) != (newdisabled == nil)
		if changed {
			if _, e := tx.Exec(r.Context(), `UPDATE users SET role=$2,disabled_at_ms=$3,updated_at_ms=$4 WHERE id=$1`, id, newrole, newdisabled, NowMillis()); e != nil {
				return nil, e
			}
			if _, e := tx.Exec(r.Context(), `UPDATE sessions SET revoked_at_ms=$2 WHERE user_id=$1 AND revoked_at_ms IS NULL`, id, NowMillis()); e != nil {
				return nil, e
			}
			action := "user.role_change"
			if role == newrole {
				action = "user.disable"
			}
			if e := interactionAudit(tx, r, p, action, "user", id, d.Reason); e != nil {
				return nil, e
			}
		}
		return map[string]any{"id": id, "role": newrole, "disabled": newdisabled != nil, "sessions_revoked": changed}, nil
	})
	if e == nil {
		JSON(w, 200, data)
	}
	return e
}
func (a *App) adminComments(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	state := r.URL.Query().Get("state")
	if state != "" && state != "visible" && state != "hidden" && state != "deleted" {
		return E(400, "VALIDATION_FAILED", "无效评论状态")
	}
	post := r.URL.Query().Get("post_id")
	if post != "" {
		if e := interactionID(post); e != nil {
			return e
		}
	} else {
		post = "0"
	}
	scope := "admin:comments:" + state + ":" + post
	c, e := a.interactionCursor(r, scope)
	if e != nil {
		return e
	}
	rows, e := a.Pool.Query(r.Context(), `SELECT c.id::text,c.post_id::text,c.body,c.state,c.created_at_ms,u.id::text,u.display_name,c.parent_id::text,c.reply_to_id::text FROM comments c JOIN users u ON u.id=c.user_id WHERE c.id>$1::bigint AND ($2='' OR c.state=$2) AND ($3::bigint=0 OR c.post_id=$3::bigint) ORDER BY c.id LIMIT $4`, c.ID, state, post, Limit(r)+1)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []map[string]any{}
	next, last := "", ""
	for rows.Next() {
		var id, pid, body, s, uid, name string
		var parent, replyTo *string
		var ms int64
		if e = rows.Scan(&id, &pid, &body, &s, &ms, &uid, &name, &parent, &replyTo); e != nil {
			return e
		}
		if len(items) == Limit(r) {
			next = a.interactionNext(scope, last, 0)
			break
		}
		items = append(items, map[string]any{"id": id, "post_id": pid, "body": body, "state": s, "created_at": interactionTime(ms), "user": map[string]any{"id": uid, "display_name": name}, "parent_id": parent, "reply_to_id": replyTo, "is_reply": parent != nil})
		last = id
	}
	if e = rows.Err(); e != nil {
		return e
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, interactionPage(items, next))
	return nil
}
func (a *App) adminCommentVisibility(w http.ResponseWriter, r *http.Request) error {
	var d struct {
		State  string `json:"state"`
		Reason string `json:"reason"`
	}
	if e := Decode(r, &d); e != nil {
		return e
	}
	if d.State != "visible" && d.State != "hidden" {
		return E(400, "VALIDATION_FAILED", "只可显示或隐藏评论")
	}
	id := r.PathValue("id")
	if e := interactionID(id); e != nil {
		return e
	}
	data, e := a.Write(r, []string{"owner"}, true, true, func(tx pgx.Tx, p Principal) (any, error) {
		var post string
		if e := tx.QueryRow(r.Context(), `SELECT post_id::text FROM comments WHERE id=$1`, id).Scan(&post); e == pgx.ErrNoRows {
			return nil, NotFound()
		} else if e != nil {
			return nil, e
		}
		if _, e := tx.Exec(r.Context(), `SELECT 1 FROM posts WHERE id=$1 FOR UPDATE`, post); e != nil {
			return nil, e
		}
		var state string
		if e := tx.QueryRow(r.Context(), `SELECT state FROM comments WHERE id=$1 FOR UPDATE`, id).Scan(&state); e != nil {
			return nil, e
		}
		if state == "deleted" {
			return nil, E(409, "COMMENT_DELETED", "已删除评论不能恢复显示")
		}
		if state != d.State {
			if _, e := tx.Exec(r.Context(), `UPDATE comments SET state=$2,updated_at_ms=$3 WHERE id=$1`, id, d.State, NowMillis()); e != nil {
				return nil, e
			}
			action := "comment.hide"
			if d.State == "visible" {
				action = "comment.show"
			}
			if e := interactionAudit(tx, r, p, action, "comment", id, d.Reason); e != nil {
				return nil, e
			}
		}
		return map[string]any{"id": id, "state": d.State}, nil
	})
	if e == nil {
		JSON(w, 200, data)
	}
	return e
}
func (a *App) adminStorage(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	var dbsize, bytes, objects int64
	if e := a.Pool.QueryRow(r.Context(), `SELECT pg_database_size(current_database()),COALESCE(sum(byte_size) FILTER(WHERE state='ready'),0),count(*) FILTER(WHERE state='ready') FROM media_assets`).Scan(&dbsize, &bytes, &objects); e != nil {
		return e
	}
	var diskbytes int64
	var files int64
	e := filepath.WalkDir(filepath.Join(a.Config.MediaDir, "objects"), func(path string, d os.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !d.IsDir() {
			s, e := d.Info()
			if e != nil {
				return e
			}
			diskbytes += s.Size()
			files++
		}
		return nil
	})
	if e != nil {
		return E(503, "STORAGE_UNAVAILABLE", "媒体存储检查失败")
	}
	backup, e := a.observedBackup(r.Context())
	if e != nil {
		return e
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, map[string]any{"database_bytes": dbsize, "ready_media_bytes": bytes, "ready_media_count": objects, "media_disk_bytes": diskbytes, "media_file_count": files, "backup": backup})
	return nil
}
func (a *App) adminJobs(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	rows, e := a.Pool.Query(r.Context(), `SELECT state,count(*),min(created_at_ms),max(attempts) FROM outbox GROUP BY state ORDER BY state`)
	if e != nil {
		return e
	}
	items := []map[string]any{}
	for rows.Next() {
		var state string
		var count, oldest, attempts int64
		if e = rows.Scan(&state, &count, &oldest, &attempts); e != nil {
			rows.Close()
			return e
		}
		items = append(items, map[string]any{"state": state, "count": count, "oldest_at": interactionTime(oldest), "max_attempts": attempts})
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return e
	}
	var pending int64
	var oldest *int64
	if e = a.Pool.QueryRow(r.Context(), `SELECT count(*),min(received_at_ms) FROM analytics_events WHERE aggregated_at_ms IS NULL`).Scan(&pending, &oldest); e != nil {
		return e
	}
	var last int64
	if e = a.Pool.QueryRow(r.Context(), `SELECT last_seq FROM analytics_rollup_state WHERE id=1`).Scan(&last); e != nil {
		return e
	}
	var oldestAt any
	var lag int64
	if oldest != nil {
		oldestAt = interactionTime(*oldest)
		lag = NowMillis() - *oldest
	}
	dead := []map[string]any{}
	dr, e := a.Pool.Query(r.Context(), `SELECT id,topic,attempts,last_error,created_at_ms FROM outbox WHERE state='dead' ORDER BY created_at_ms DESC LIMIT 20`)
	if e != nil {
		return e
	}
	for dr.Next() {
		var id, topic string
		var attempts, ms int64
		var message *string
		if e = dr.Scan(&id, &topic, &attempts, &message, &ms); e != nil {
			dr.Close()
			return e
		}
		dead = append(dead, map[string]any{"id": id, "topic": topic, "attempts": attempts, "error": message, "created_at": interactionTime(ms)})
	}
	dr.Close()
	if e = dr.Err(); e != nil {
		return e
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, map[string]any{"items": items, "dead_tasks": dead, "analytics": map[string]any{"pending_count": pending, "oldest_pending_at": oldestAt, "lag_ms": lag, "max_processed_seq_diagnostic": last}})
	return nil
}
func (a *App) adminAudit(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	c, e := a.interactionCursor(r, "admin:audit")
	if e != nil {
		return e
	}
	rows, e := a.Pool.Query(r.Context(), `SELECT id::text,actor_user_id::text,action,target_type,target_id,reason,created_at_ms FROM audit_logs WHERE ($1::bigint=0 OR id<$1::bigint) ORDER BY id DESC LIMIT $2`, c.ID, Limit(r)+1)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []map[string]any{}
	next, last := "", ""
	for rows.Next() {
		var id, action, kind, target, reason string
		var actor *string
		var ms int64
		if e = rows.Scan(&id, &actor, &action, &kind, &target, &reason, &ms); e != nil {
			return e
		}
		if len(items) == Limit(r) {
			next = a.interactionNext("admin:audit", last, 0)
			break
		}
		items = append(items, map[string]any{"id": id, "actor_id": actor, "action": action, "target_type": kind, "target_id": target, "reason": reason, "created_at": interactionTime(ms)})
		last = id
	}
	if e = rows.Err(); e != nil {
		return e
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, interactionPage(items, next))
	return nil
}
