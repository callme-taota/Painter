package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const homeItemLimit = 5000
const homeBodyLimit = 64 << 10

var homeSectionKeys = []string{"intro", "articles", "steam", "music", "photos", "closing", "activity", "projects"}

type HomeSection struct {
	Enabled     bool   `json:"enabled"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Limit       int    `json:"limit"`
	LinkLabel   string `json:"link_label"`
	LinkURL     string `json:"link_url"`
}
type HomeConfig struct {
	Activity           HomeActivityOptions `json:"activity"`
	activityPresent    bool
	Order              []string               `json:"order"`
	Sections           map[string]HomeSection `json:"sections"`
	UseSiteIntro       bool                   `json:"use_site_intro"`
	IntroName          string                 `json:"intro_name"`
	IntroRoles         []string               `json:"intro_roles"`
	WritingTitle       string                 `json:"writing_title"`
	WritingDescription string                 `json:"writing_description"`
	ScrollLabel        string                 `json:"scroll_label"`
	MotionEnabled      bool                   `json:"motion_enabled"`
	WarmthEnabled      bool                   `json:"warmth_enabled"`
}

func (c *HomeConfig) UnmarshalJSON(raw []byte) error {
	type plain HomeConfig
	var v plain
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&v); e != nil {
		return e
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		return e
	}
	*c = HomeConfig(v)
	_, c.activityPresent = fields["activity"]
	return nil
}
func homeAdditionSection(key string) HomeSection {
	if key == "activity" {
		return HomeSection{Title: "写下来的日子。"}
	}
	return HomeSection{Title: "正在做的项目。", Limit: 6}
}

// Read normalization is deliberately non-persistent. Migration never rewrites
// an existing owner's six-section configuration or choices.
func normalizeHomeConfig(c *HomeConfig) {
	if c.Sections == nil {
		return
	}
	for _, key := range []string{"activity", "projects"} {
		if _, ok := c.Sections[key]; !ok {
			c.Sections[key] = homeAdditionSection(key)
			c.Order = append(c.Order, key)
		}
	}
	if !c.activityPresent {
		c.Activity = defaultHomeActivity()
	}
}
func mergeHomeAdditions(c *HomeConfig, old HomeConfig) {
	if c.Sections == nil {
		return
	}
	legacyOrder := len(c.Order) == 6
	seen := map[string]bool{}
	for _, key := range c.Order {
		seen[key] = true
	}
	for _, key := range []string{"intro", "articles", "steam", "music", "photos", "closing"} {
		legacyOrder = legacyOrder && seen[key]
	}
	for _, key := range []string{"activity", "projects"} {
		if _, ok := c.Sections[key]; !ok {
			c.Sections[key] = old.Sections[key]
		}
		if legacyOrder {
			c.Order = append(c.Order, key)
		}
	}
	if !c.activityPresent {
		c.Activity = old.Activity
	}
}

type homeSettings struct {
	Version       int64
	Config        HomeConfig
	SteamID, Mode string
	Auto          bool
	Cipher        []byte
	LastSync      *int64
	LastError     string
}
type homeItemInput struct {
	Kind            string `json:"kind"`
	Title           string `json:"title"`
	Subtitle        string `json:"subtitle"`
	Alt             string `json:"alt"`
	URL             string `json:"url"`
	ImageURL        string `json:"image_url"`
	MediaID         string `json:"media_id"`
	ProviderID      string `json:"provider_id"`
	Position        int    `json:"position"`
	Visible         bool   `json:"visible"`
	HeroURL         string `json:"hero_url"`
	LogoURL         string `json:"logo_url"`
	PlaytimeMinutes *int64 `json:"playtime_minutes"`
	heroPresent     bool
	logoPresent     bool
}

// New presentation fields are optional on old clients' full draft writes.
// Explicit empty strings restore automatic artwork; omission retains overrides.
func (in *homeItemInput) UnmarshalJSON(raw []byte) error {
	type plain homeItemInput
	var value plain
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&value); e != nil {
		return e
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		return e
	}
	*in = homeItemInput(value)
	_, in.heroPresent = fields["hero_url"]
	_, in.logoPresent = fields["logo_url"]
	for _, key := range []string{"hero_url", "logo_url"} {
		if raw, ok := fields[key]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return E(400, "VALIDATION_FAILED", "素材网址必须为字符串，自动素材请留空")
		}
	}
	return nil
}

type homeQuery interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

const homeItemColumns = `id::text,kind,title,subtitle,alt,url,image_url,COALESCE(media_id,''),provider_id,source,position,visible,version,created_at_ms,updated_at_ms,local_override,hero_url,logo_url,playtime_minutes`

func (a *App) RegisterHome() {
	a.Handle("GET /api/v1/admin/home/activity", a.homeActivityPreview)
	a.registerHomeProjects()
	a.Handle("GET /api/v1/admin/home", a.homeGet)
	a.Handle("PUT /api/v1/admin/home", a.homePut)
	a.Handle("GET /api/v1/admin/home/items", a.homeItems)
	a.Handle("GET /api/v1/admin/home/items/{id}", a.homeItemGet)
	a.Handle("POST /api/v1/admin/home/items", a.homeItemCreate)
	a.Handle("PUT /api/v1/admin/home/items/{id}", a.homeItemUpdate)
	a.Handle("DELETE /api/v1/admin/home/items/{id}", a.homeItemDelete)
	a.Handle("POST /api/v1/admin/home/import", a.homeImport)
	a.Handle("POST /api/v1/admin/home/steam-sync", a.homeSteamSync)
	a.Handle("GET /api/v1/games", a.gamesList)
	a.Handle("GET /api/v1/admin/home/music-playlist", a.homeMusicGet)
	a.Handle("POST /api/v1/admin/home/music-playlist/preview", a.homeMusicPreview)
	a.Handle("POST /api/v1/admin/home/music-playlist/import", a.homeMusicImport)
}
func homeRead(ctx context.Context, q homeQuery, lock bool) (homeSettings, error) {
	var s homeSettings
	var raw []byte
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRow(ctx, `SELECT version,config,steam_id,steam_mode,steam_auto_sync,steam_key_ciphertext,last_sync_at_ms,last_error FROM homepage_settings WHERE id=1`+suffix).Scan(&s.Version, &raw, &s.SteamID, &s.Mode, &s.Auto, &s.Cipher, &s.LastSync, &s.LastError)
	if err == nil {
		err = json.Unmarshal(raw, &s.Config)
		if err == nil {
			normalizeHomeConfig(&s.Config)
		}
	}
	return s, err
}
func (a *App) homeSettingsDTO(s homeSettings) map[string]any {
	var last any
	if s.LastSync != nil {
		last = TimeString(*s.LastSync)
	}
	limit := a.Config.UploadLimit
	if limit <= 0 {
		limit = 10 << 20
	}
	return map[string]any{"version": s.Version, "config": s.Config, "max_image_bytes": limit, "steam": map[string]any{"steam_id": s.SteamID, "mode": s.Mode, "auto_sync_enabled": s.Auto, "api_key_configured": len(s.Cipher) > 0, "last_sync_at": last, "last_error": s.LastError}}
}
func (a *App) OwnerHome(ctx context.Context) (map[string]any, error) {
	s, e := homeRead(ctx, a.Pool, false)
	if e != nil {
		return nil, e
	}
	return a.homeSettingsDTO(s), nil
}
func homeText(s string, max int, multiline bool) error {
	if !utf8.ValidString(s) || utf8.RuneCountInString(s) > max || strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) && !(multiline && r == '\n') }) {
		return E(400, "VALIDATION_FAILED", "首页文字过长或包含无效控制字符")
	}
	return nil
}
func homeURL(s string) error {
	if s == "" {
		return nil
	}
	if len(s) > 2048 || !utf8.ValidString(s) || strings.ContainsRune(s, '\\') || strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return E(400, "VALIDATION_FAILED", "首页链接格式无效")
	}
	decoded, e := url.PathUnescape(s)
	if e != nil || strings.ContainsFunc(decoded, unicode.IsControl) {
		return E(400, "VALIDATION_FAILED", "首页链接格式无效")
	}
	if strings.HasPrefix(s, "#") || (strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") && !strings.HasPrefix(decoded, "//")) {
		u, e := url.Parse(s)
		if e == nil && u.Host == "" && u.Scheme == "" && u.User == nil {
			return nil
		}
		return E(400, "VALIDATION_FAILED", "首页相对链接格式无效")
	}
	_, e = friendURL(s, true)
	return e
}
func validateHome(c HomeConfig) error {
	invalid := func() error {
		return E(400, "VALIDATION_FAILED", "首页分区必须完整且顺序不可重复；数量超出范围")
	}
	if len(c.Order) != len(homeSectionKeys) || len(c.Sections) != len(homeSectionKeys) || c.IntroRoles == nil || len(c.IntroRoles) > 12 {
		return invalid()
	}
	if e := validateHomeActivity(c.Activity); e != nil {
		return e
	}
	seen := map[string]bool{}
	for _, key := range c.Order {
		if seen[key] {
			return invalid()
		}
		seen[key] = true
	}
	for _, key := range homeSectionKeys {
		section, ok := c.Sections[key]
		if !ok || !seen[key] {
			return invalid()
		}
		max := 24
		if key == "articles" {
			max = 12
		}
		if key == "intro" || key == "closing" || key == "activity" {
			max = 0
		}
		if section.Limit < 0 || section.Limit > max {
			return invalid()
		}
		for _, v := range []struct {
			s     string
			n     int
			multi bool
		}{{section.Title, 80, true}, {section.Description, 600, true}, {section.LinkLabel, 60, false}} {
			if e := homeText(v.s, v.n, v.multi); e != nil {
				return e
			}
		}
		if e := homeURL(section.LinkURL); e != nil {
			return e
		}
	}
	for _, v := range []struct {
		s     string
		n     int
		multi bool
	}{{c.IntroName, 80, false}, {c.WritingTitle, 80, true}, {c.WritingDescription, 600, true}, {c.ScrollLabel, 60, false}} {
		if e := homeText(v.s, v.n, v.multi); e != nil {
			return e
		}
	}
	for _, role := range c.IntroRoles {
		if e := homeText(role, 60, false); e != nil {
			return e
		}
	}
	return nil
}
func decodeHome(w http.ResponseWriter, r *http.Request, in any) error {
	r.Body = http.MaxBytesReader(w, r.Body, homeBodyLimit)
	raw, e := io.ReadAll(r.Body)
	if e != nil {
		var too *http.MaxBytesError
		if errors.As(e, &too) {
			return E(413, "PAYLOAD_TOO_LARGE", "首页请求最多64 KiB")
		}
		return E(400, "INVALID_JSON", "无法读取首页请求")
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	return Decode(r, in)
}
func homeAuthorize(a *App, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	return a.checkCSRF(r)
}
func (a *App) homeGet(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	v, e := a.OwnerHome(r.Context())
	if e == nil {
		JSON(w, 200, v)
	}
	return e
}
func (a *App) homePut(w http.ResponseWriter, r *http.Request) error {
	if e := homeAuthorize(a, r); e != nil {
		return e
	}
	version, e := IfMatch(r)
	if e != nil {
		return e
	}
	var in struct {
		Config HomeConfig `json:"config"`
		Steam  struct {
			SteamID string  `json:"steam_id"`
			Mode    string  `json:"mode"`
			Auto    bool    `json:"auto_sync_enabled"`
			Key     *string `json:"api_key"`
			Clear   bool    `json:"clear_api_key"`
		} `json:"steam"`
	}
	if e := decodeHome(w, r, &in); e != nil {
		return e
	}
	if (in.Steam.SteamID != "" && !homeDigits(in.Steam.SteamID, 17, 17)) || (in.Steam.Mode != "owned" && in.Steam.Mode != "recent") || (in.Steam.Key != nil && (!credentialTextValid(*in.Steam.Key, 256) || strings.TrimSpace(*in.Steam.Key) == "" || strings.ContainsFunc(*in.Steam.Key, unicode.IsSpace))) || (in.Steam.Clear && in.Steam.Key != nil) {
		return E(400, "VALIDATION_FAILED", "Steam ID、模式或密钥格式无效")
	}
	value, e := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		s, e := homeRead(r.Context(), tx, true)
		if e != nil {
			return nil, e
		}
		mergeHomeAdditions(&in.Config, s.Config)
		if e := validateHome(in.Config); e != nil {
			return nil, e
		}
		if s.Version != version {
			return nil, E(409, "VERSION_CONFLICT", "首页配置已变化，请重新加载")
		}
		if in.Steam.Clear {
			s.Cipher = nil
		}
		if in.Steam.Key != nil {
			s.Cipher, e = a.encrypt([]byte(*in.Steam.Key))
			if e != nil {
				return nil, e
			}
		}
		if in.Steam.Auto && (in.Steam.SteamID == "" || len(s.Cipher) == 0) {
			return nil, E(400, "VALIDATION_FAILED", "自动同步需要 Steam ID 和 API Key")
		}
		raw, _ := json.Marshal(in.Config)
		changedSource := s.SteamID != in.Steam.SteamID || s.Mode != in.Steam.Mode || in.Steam.Key != nil || in.Steam.Clear
		_, e = tx.Exec(r.Context(), `UPDATE homepage_settings SET version=version+1,config=$1,steam_id=$2,steam_mode=$3,steam_auto_sync=$4,steam_key_ciphertext=$5,lease_token=NULL,lease_until_ms=NULL,next_sync_at_ms=CASE WHEN $6 THEN 0 ELSE next_sync_at_ms END,last_error=CASE WHEN $6 THEN '' ELSE last_error END WHERE id=1`, raw, in.Steam.SteamID, in.Steam.Mode, in.Steam.Auto, s.Cipher, changedSource)
		if e != nil {
			return nil, e
		}
		s, e = homeRead(r.Context(), tx, false)
		if e != nil {
			return nil, e
		}
		if e = audit(r.Context(), tx, p, "site.settings_update", "site", "homepage", ""); e != nil {
			return nil, e
		}
		return a.homeSettingsDTO(s), nil
	})
	if e == nil {
		JSON(w, 200, value)
	}
	return e
}
func homeKind(s string) bool { return s == "steam" || s == "music" || s == "photos" }
func homeDigits(s string, min, max int) bool {
	if len(s) < min || len(s) > max || s[0] == '0' {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func validateHomeItem(in homeItemInput) error {
	if !homeKind(in.Kind) || in.Position < 0 || in.Position > 1000000 {
		return E(400, "VALIDATION_FAILED", "首页条目类型或排序无效")
	}
	for _, v := range []struct {
		s string
		n int
	}{{in.Title, 80}, {in.Subtitle, 600}, {in.Alt, 200}, {in.ProviderID, 64}} {
		if e := homeText(v.s, v.n, false); e != nil {
			return e
		}
	}
	if e := homeURL(in.URL); e != nil {
		return e
	}
	if in.ProviderID != "" && !homeDigits(in.ProviderID, 1, 20) {
		return E(400, "VALIDATION_FAILED", "供应商 ID 无效")
	}
	if in.MediaID != "" {
		if len(in.MediaID) > 128 || strings.ContainsFunc(in.MediaID, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
		}) {
			return E(400, "VALIDATION_FAILED", "图片 ID 无效")
		}
	} else if in.Kind == "photos" || in.ImageURL == "" {
		return E(400, "VALIDATION_FAILED", "照片需要上传图片；其它条目需要图片或官方封面")
	}
	if in.ImageURL != "" && !homeRemoteImage(in.Kind, in.ImageURL) {
		return E(400, "VALIDATION_FAILED", "请上传图片或使用官方封面地址")
	}
	if in.Kind != "steam" && (in.HeroURL != "" || in.LogoURL != "" || in.PlaytimeMinutes != nil) {
		return E(400, "VALIDATION_FAILED", "游戏素材与游玩时长仅适用于 Steam 游戏")
	}
	for _, art := range []string{in.HeroURL, in.LogoURL} {
		if art != "" && !homeRemoteImage("steam", art) {
			return E(400, "VALIDATION_FAILED", "游戏素材必须使用官方 Steam HTTPS 图片地址")
		}
	}
	if in.PlaytimeMinutes != nil && (*in.PlaytimeMinutes < 0 || *in.PlaytimeMinutes > 52704000) {
		return E(400, "VALIDATION_FAILED", "游玩分钟数超出有效范围")
	}
	return nil
}
func scanHomeItem(row interface{ Scan(...any) error }) (map[string]any, error) {
	var id, kind, title, subtitle, alt, target, image, media, provider, source string
	var position int
	var visible bool
	var localOverride bool
	var version, created, updated int64
	var hero, logo string
	var minutes *int64
	e := row.Scan(&id, &kind, &title, &subtitle, &alt, &target, &image, &media, &provider, &source, &position, &visible, &version, &created, &updated, &localOverride, &hero, &logo, &minutes)
	if e != nil {
		return nil, e
	}
	if media != "" {
		image = "/media/" + media
	}
	var played any
	if minutes != nil {
		played = *minutes
	}
	return map[string]any{"id": id, "kind": kind, "title": title, "subtitle": subtitle, "alt": alt, "url": target, "image_url": image, "media_id": media, "provider_id": provider, "source": source, "position": position, "visible": visible, "version": version, "created_at": TimeString(created), "updated_at": TimeString(updated), "local_override": localOverride, "hero_url": hero, "logo_url": logo, "playtime_minutes": played}, nil
}

// HomepageData uses one repeatable-read snapshot, no network access and no secret
// projection. Disabled walls never contribute entries; visible entries beyond a
// display limit remain explicitly public media, but not present in this payload.
func (a *App) HomepageData(ctx context.Context) (map[string]any, error) {
	tx, e := a.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(context.Background())
	s, e := homeRead(ctx, tx, false)
	if e != nil {
		return nil, e
	}
	walls := map[string][]map[string]any{"steam": {}, "music": {}, "photos": {}, "projects": {}}
	for _, kind := range []string{"steam", "music", "photos"} {
		section := s.Config.Sections[kind]
		if !section.Enabled || section.Limit == 0 {
			continue
		}
		limit := section.Limit
		if kind == "steam" {
			limit = min(limit, 5)
		}
		rows, e := tx.Query(ctx, `SELECT `+homeItemColumns+` FROM homepage_items WHERE kind=$1 AND visible ORDER BY position,id LIMIT $2`, kind, limit)
		if e != nil {
			return nil, e
		}
		for rows.Next() {
			item, e := scanHomeItem(rows)
			if e != nil {
				rows.Close()
				return nil, e
			}
			homePublicItem(item)
			walls[kind] = append(walls[kind], item)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
	}
	activity, e := homeActivityAt(ctx, tx, s.Config, time.Now())
	if e != nil {
		return nil, e
	}
	walls["projects"], e = homePublicProjects(ctx, tx, s.Config)
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return map[string]any{"config": s.Config, "walls": walls, "activity": activity}, nil
}
func (a *App) homeItems(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r, "owner")
	if e != nil {
		return e
	}
	kind := r.URL.Query().Get("kind")
	if !homeKind(kind) {
		return E(400, "VALIDATION_FAILED", "请选择首页条目类型")
	}
	limit, e := momentLimit(r.URL.Query())
	if e != nil {
		return e
	}
	scope := "home:" + p.ID + ":" + kind + ":" + strconv.Itoa(limit)
	if len(r.URL.Query().Get("cursor")) > 2048 {
		return E(400, "VALIDATION_FAILED", "分页游标过长")
	}
	cursor, e := a.interactionCursor(r, scope)
	if e != nil {
		return e
	}
	if !positiveID(cursor.ID) && cursor.ID != "0" || cursor.Time < 0 || cursor.Time > 1000000 {
		return E(400, "VALIDATION_FAILED", "分页游标无效")
	}
	rows, e := a.Pool.Query(r.Context(), `SELECT `+homeItemColumns+` FROM homepage_items WHERE kind=$1 AND ($2='0' OR (position,id)>($3,$2::bigint)) ORDER BY position,id LIMIT $4`, kind, cursor.ID, cursor.Time, limit+1)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		item, e := scanHomeItem(rows)
		if e != nil {
			return e
		}
		items = append(items, item)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	more := len(items) > limit
	next := ""
	if more {
		items = items[:limit]
		last := items[len(items)-1]
		next = a.interactionNext(scope, last["id"].(string), int64(last["position"].(int)))
	}
	JSON(w, 200, map[string]any{"items": items, "cursor": next, "next_cursor": next, "has_more": more, "limit": limit, "max_limit": 50, "max_items": homeItemLimit})
	return nil
}
func (a *App) homeItemGet(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	id := r.PathValue("id")
	if !positiveID(id) {
		return NotFound()
	}
	item, e := scanHomeItem(a.Pool.QueryRow(r.Context(), `SELECT `+homeItemColumns+` FROM homepage_items WHERE id=$1`, id))
	if e == nil {
		JSON(w, 200, item)
	}
	return e
}
func homeLockMedia(ctx context.Context, tx pgx.Tx, p Principal, id, oldID string) error {
	if id == "" {
		return nil
	}
	var owner, state, mime string
	e := tx.QueryRow(ctx, `SELECT owner_user_id::text,state,mime_type FROM media_assets WHERE id=$1 FOR UPDATE`, id).Scan(&owner, &state, &mime)
	if errors.Is(e, pgx.ErrNoRows) {
		return E(400, "INVALID_MEDIA", "图片不存在或不可用")
	}
	if e != nil {
		return e
	}
	if state != "ready" || !(mime == "image/png" || mime == "image/jpeg" || mime == "image/gif" || mime == "image/webp") || (owner != p.ID && id != oldID) {
		return E(400, "INVALID_MEDIA", "只能使用自己的就绪图片或保留已有封面")
	}
	return nil
}
func (a *App) homeItemCreate(w http.ResponseWriter, r *http.Request) error {
	if e := homeAuthorize(a, r); e != nil {
		return e
	}
	var in homeItemInput
	if e := decodeHome(w, r, &in); e != nil {
		return e
	}
	if in.PlaytimeMinutes != nil {
		return E(400, "READ_ONLY_FIELD", "游玩时长仅从 Steam 同步，不支持手工设置")
	}
	if e := validateHomeItem(in); e != nil {
		return e
	}
	value, e := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		// Serialize collection capacity and coordinate provider replacement.
		if _, e := homeRead(r.Context(), tx, true); e != nil {
			return nil, e
		}
		var n int
		if e := tx.QueryRow(r.Context(), `SELECT count(*) FROM homepage_items`).Scan(&n); e != nil {
			return nil, e
		}
		if n >= homeItemLimit {
			return nil, E(409, "HOME_ITEM_LIMIT", "首页条目最多5000项")
		}
		if e := homeLockMedia(r.Context(), tx, p, in.MediaID, ""); e != nil {
			return nil, e
		}
		now := NowMillis()
		item, e := scanHomeItem(tx.QueryRow(r.Context(), `INSERT INTO homepage_items(kind,title,subtitle,alt,url,image_url,media_id,provider_id,position,visible,created_at_ms,updated_at_ms,hero_url,logo_url) VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$10,$11,$11,$12,$13) RETURNING `+homeItemColumns, in.Kind, in.Title, in.Subtitle, in.Alt, in.URL, in.ImageURL, in.MediaID, in.ProviderID, in.Position, in.Visible, now, in.HeroURL, in.LogoURL))
		if e != nil {
			return nil, e
		}
		return item, audit(r.Context(), tx, p, "home.item_create", "site", item["id"].(string), "")
	})
	if e == nil {
		JSON(w, 201, value)
	}
	return e
}
func (a *App) homeItemUpdate(w http.ResponseWriter, r *http.Request) error {
	if e := homeAuthorize(a, r); e != nil {
		return e
	}
	id := r.PathValue("id")
	if !positiveID(id) {
		return NotFound()
	}
	version, e := IfMatch(r)
	if e != nil {
		return e
	}
	var in homeItemInput
	if e := decodeHome(w, r, &in); e != nil {
		return e
	}
	if in.PlaytimeMinutes != nil {
		return E(400, "READ_ONLY_FIELD", "游玩时长仅从 Steam 同步，不支持手工设置")
	}
	// A read DTO's effective /media URL can be round-tripped with its media_id.
	if in.MediaID != "" && in.ImageURL == "/media/"+in.MediaID {
		in.ImageURL = ""
	}
	if e := validateHomeItem(in); e != nil {
		return e
	}
	value, e := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		if _, e := homeRead(r.Context(), tx, true); e != nil {
			return nil, e
		}
		old, e := scanHomeItem(tx.QueryRow(r.Context(), `SELECT `+homeItemColumns+` FROM homepage_items WHERE id=$1 FOR UPDATE`, id))
		if e != nil {
			return nil, e
		}
		if old["version"] != version {
			return nil, E(409, "VERSION_CONFLICT", "首页条目已变化，请重新加载")
		}
		if old["kind"] != in.Kind || (old["source"] != "manual" && old["provider_id"] != in.ProviderID) {
			return nil, E(400, "VALIDATION_FAILED", "不能改变条目类型或同步游戏 ID")
		}
		if e = homeLockMedia(r.Context(), tx, p, in.MediaID, old["media_id"].(string)); e != nil {
			return nil, e
		}
		if !in.heroPresent {
			in.HeroURL = old["hero_url"].(string)
		}
		if !in.logoPresent {
			in.LogoURL = old["logo_url"].(string)
		}
		item, e := scanHomeItem(tx.QueryRow(r.Context(), `UPDATE homepage_items SET title=$2,subtitle=$3,alt=$4,url=$5,image_url=$6,media_id=NULLIF($7,''),provider_id=$8,position=$9,visible=$10,local_override=(source<>'manual' OR local_override),version=version+1,updated_at_ms=$11,hero_url=$12,logo_url=$13 WHERE id=$1 RETURNING `+homeItemColumns, id, in.Title, in.Subtitle, in.Alt, in.URL, in.ImageURL, in.MediaID, in.ProviderID, in.Position, in.Visible, NowMillis(), in.HeroURL, in.LogoURL))
		if e != nil {
			return nil, e
		}
		return item, audit(r.Context(), tx, p, "home.item_update", "site", id, "")
	})
	if e == nil {
		JSON(w, 200, value)
	}
	return e
}
func (a *App) homeItemDelete(w http.ResponseWriter, r *http.Request) error {
	if e := homeAuthorize(a, r); e != nil {
		return e
	}
	id := r.PathValue("id")
	if !positiveID(id) {
		return NotFound()
	}
	version, e := IfMatch(r)
	if e != nil {
		return e
	}
	_, e = a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		if _, e := homeRead(r.Context(), tx, true); e != nil {
			return nil, e
		}
		var v int64
		if e := tx.QueryRow(r.Context(), `SELECT version FROM homepage_items WHERE id=$1 FOR UPDATE`, id).Scan(&v); e != nil {
			return nil, e
		}
		if v != version {
			return nil, E(409, "VERSION_CONFLICT", "首页条目已变化，请重新加载")
		}
		if _, e := tx.Exec(r.Context(), `DELETE FROM homepage_items WHERE id=$1`, id); e != nil {
			return nil, e
		}
		return nil, audit(r.Context(), tx, p, "home.item_delete", "site", id, "")
	})
	if e == nil {
		JSON(w, 204, nil)
	}
	return e
}

// Keep diagnostic strings stable and credential-free; raw provider errors/URLs
// must not enter API errors, audit text, worker logs or last_error.
func homeUpstreamError() error {
	return E(502, "HOME_PROVIDER_UNAVAILABLE", "供应商暂不可用，请稍后重试或手动填写")
}
func homeVersionError() error {
	return E(409, "VERSION_CONFLICT", "同步期间首页配置已变化，请重新加载后重试")
}
