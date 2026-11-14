package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"golang.org/x/net/idna"
)

const friendLimit = 500
const friendBodyLimit = 16 << 10
const friendColumns = "id::text,name,url,description,avatar_url,sort_order,visible,version,created_at_ms,updated_at_ms"

type friendInput struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatar_url"`
	SortOrder   int    `json:"sort_order"`
	Visible     bool   `json:"visible"`
}

func (a *App) RegisterFriends() {
	a.Handle("GET /api/v1/studio/friends", a.friendsList)
	a.Handle("POST /api/v1/studio/friends", a.friendCreate)
	a.Handle("PUT /api/v1/studio/friends/{id}", a.friendUpdate)
	a.Handle("DELETE /api/v1/studio/friends/{id}", a.friendDelete)
	a.Handle("GET /api/v1/studio/friend-settings", a.friendSettingsGet)
	a.Handle("PUT /api/v1/studio/friend-settings", a.friendSettingsPut)
}

func friendText(raw string, max int, required bool) (string, error) {
	if !utf8.ValidString(raw) || strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsControl(r) || r == '\u2028' || r == '\u2029' }) {
		return "", E(400, "VALIDATION_FAILED", "友链文字不能包含控制字符或换行")
	}
	value := strings.TrimSpace(raw)
	if utf8.RuneCountInString(value) > max || (required && value == "") {
		return "", E(400, "VALIDATION_FAILED", "友链名称或简介长度不符合要求")
	}
	return value, nil
}

// Descriptions are plain text and may span lines. Store one newline convention
// so browser textareas and API clients have the same character count and value.
func friendDescription(raw string, max int) (string, error) {
	value := strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	if !utf8.ValidString(value) || strings.ContainsFunc(value, func(r rune) bool {
		return (unicode.IsControl(r) && r != '\n') || r == '\u2028' || r == '\u2029'
	}) {
		return "", E(400, "VALIDATION_FAILED", "友链简介不能包含换行以外的控制字符")
	}
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > max {
		return "", E(400, "VALIDATION_FAILED", "友链简介长度不符合要求")
	}
	return value, nil
}

// These are destination URLs, never server-side fetch instructions. Reject
// browser-ambiguous authority syntax and encoded controls before storing them.
func friendURL(raw string, required bool) (string, error) {
	invalid := func() (string, error) {
		return "", E(400, "VALIDATION_FAILED", "网址须为有效的 HTTP 或 HTTPS 地址，不能包含账号、空白或控制字符")
	}
	if raw == "" && !required {
		return "", nil
	}
	if raw == "" || len(raw) > 2048 || !utf8.ValidString(raw) || strings.ContainsRune(raw, '\\') || strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) {
		return invalid()
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil || strings.ContainsFunc(decoded, unicode.IsControl) {
		return invalid()
	}
	u, err := url.Parse(raw)
	if err != nil || (strings.ToLower(u.Scheme) != "http" && strings.ToLower(u.Scheme) != "https") || u.Opaque != "" || u.Host == "" || u.User != nil {
		return invalid()
	}
	u.Scheme = strings.ToLower(u.Scheme)
	host := u.Hostname()
	if host == "" || strings.Contains(host, "%") || strings.HasSuffix(u.Host, ":") || (strings.HasPrefix(u.Host, "[") && !strings.Contains(host, ":")) {
		return invalid()
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return invalid()
		}
	}
	if net.ParseIP(host) == nil {
		ascii, err := idna.Lookup.ToASCII(host)
		if err != nil || len(ascii) > 253 {
			return invalid()
		}
		for _, label := range strings.Split(strings.TrimSuffix(ascii, "."), ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return invalid()
			}
			for _, ch := range label {
				if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-') {
					return invalid()
				}
			}
		}
	}
	return u.String(), nil
}

func decodeFriendJSON(w http.ResponseWriter, r *http.Request, input any) error {
	r.Body = http.MaxBytesReader(w, r.Body, friendBodyLimit)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return E(413, "PAYLOAD_TOO_LARGE", "友链请求最多16 KiB")
		}
		return E(400, "INVALID_JSON", "无法读取友链请求")
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	return Decode(r, input)
}

func decodeFriend(w http.ResponseWriter, r *http.Request) (friendInput, error) {
	var in friendInput
	if err := decodeFriendJSON(w, r, &in); err != nil {
		return in, err
	}
	var err error
	if in.Name, err = friendText(in.Name, 80, true); err != nil {
		return in, err
	}
	if in.Description, err = friendDescription(in.Description, 240); err != nil {
		return in, err
	}
	if in.URL, err = friendURL(in.URL, true); err != nil {
		return in, err
	}
	if in.AvatarURL, err = friendURL(in.AvatarURL, false); err != nil {
		return in, err
	}
	if in.SortOrder < -100000 || in.SortOrder > 100000 {
		return in, E(400, "VALIDATION_FAILED", "友链排序须为 -100000 至 100000 的整数")
	}
	return in, nil
}

func scanFriend(row interface{ Scan(...any) error }) (map[string]any, error) {
	var id, name, target, description, avatar string
	var order int
	var visible bool
	var version, created, updated int64
	if err := row.Scan(&id, &name, &target, &description, &avatar, &order, &visible, &version, &created, &updated); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "name": name, "url": target, "description": description, "avatar_url": avatar, "sort_order": order, "visible": visible, "version": version, "created_at": TimeString(created), "updated_at": TimeString(updated)}, nil
}

// FriendSettings is data-only: public rendering uses only the enabled flag and
// presentation copy. Administration still authorizes before calling this helper.
func (a *App) FriendSettings(ctx context.Context) (map[string]any, error) {
	var enabled bool
	var title, description string
	var version int64
	if err := a.Pool.QueryRow(ctx, "SELECT enabled,title,description,version FROM friend_link_settings WHERE id=1").Scan(&enabled, &title, &description, &version); err != nil {
		return nil, err
	}
	return map[string]any{"enabled": enabled, "title": title, "description": description, "version": version}, nil
}

// PublicFriends takes settings and visible links from one statement snapshot.
// Cookies and query parameters cannot widen this public projection. Turning the
// page off returns 404 without exposing any link or private setting data.
func (a *App) PublicFriends(ctx context.Context) (map[string]any, error) {
	var enabled bool
	var title, description string
	var version int64
	var raw []byte
	err := a.Pool.QueryRow(ctx, `SELECT s.enabled,s.title,s.description,s.version,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',f.id::text,'name',f.name,'url',f.url,
 'description',f.description,'avatar_url',f.avatar_url,'sort_order',f.sort_order,'visible',f.visible,
 'version',f.version,'created_at',`+contentTimeSQL("f.created_at_ms")+`,'updated_at',`+contentTimeSQL("f.updated_at_ms")+`)
 ORDER BY f.sort_order,f.id) FROM friend_links f WHERE s.enabled AND f.visible),'[]'::jsonb)
 FROM friend_link_settings s WHERE s.id=1`).Scan(&enabled, &title, &description, &version, &raw)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, NotFound()
	}
	items := []map[string]any{}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return map[string]any{"settings": map[string]any{"enabled": true, "title": title, "description": description, "version": version}, "items": items}, nil
}

func (a *App) friendsList(w http.ResponseWriter, r *http.Request) error {
	if _, err := a.Require(r, "owner"); err != nil {
		return err
	}
	rows, err := a.Pool.Query(r.Context(), "SELECT "+friendColumns+" FROM friend_links ORDER BY sort_order,friend_links.id LIMIT $1", friendLimit)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		item, err := scanFriend(rows)
		if err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	JSON(w, 200, map[string]any{"items": items, "limit": friendLimit, "max_limit": friendLimit})
	return nil
}

func (a *App) friendAuthorize(r *http.Request) error {
	if _, err := a.Require(r, "owner"); err != nil {
		return err
	}
	return a.checkCSRF(r)
}

func (a *App) friendCreate(w http.ResponseWriter, r *http.Request) error {
	if err := a.friendAuthorize(r); err != nil {
		return err
	}
	in, err := decodeFriend(w, r)
	if err != nil {
		return err
	}
	value, err := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		// This singleton also serializes concurrent creates at the collection cap.
		var settingsID int
		if err := tx.QueryRow(r.Context(), "SELECT id FROM friend_link_settings WHERE id=1 FOR UPDATE").Scan(&settingsID); err != nil {
			return nil, err
		}
		var count int
		if err := tx.QueryRow(r.Context(), "SELECT count(*) FROM friend_links").Scan(&count); err != nil {
			return nil, err
		}
		if count >= friendLimit {
			return nil, E(409, "FRIEND_LIMIT_REACHED", "友链最多500项，请先移除不再展示的友链")
		}
		now := NowMillis()
		item, err := scanFriend(tx.QueryRow(r.Context(), "INSERT INTO friend_links(name,url,description,avatar_url,sort_order,visible,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$4,$5,$6,$7,$7) RETURNING "+friendColumns, in.Name, in.URL, in.Description, in.AvatarURL, in.SortOrder, in.Visible, now))
		if err != nil {
			return nil, err
		}
		if err := audit(r.Context(), tx, p, "friend.create", "friend", item["id"].(string), ""); err != nil {
			return nil, err
		}
		return item, nil
	})
	if err == nil {
		JSON(w, 201, value)
	}
	return err
}

func friendVersion(ctx context.Context, tx pgx.Tx, id string, expected int64) error {
	var actual int64
	err := tx.QueryRow(ctx, "SELECT version FROM friend_links WHERE id=$1 FOR UPDATE", id).Scan(&actual)
	if errors.Is(err, pgx.ErrNoRows) {
		return NotFound()
	}
	if err != nil {
		return err
	}
	if actual != expected {
		return E(409, "VERSION_CONFLICT", "友链已变化，请重新加载后再保存")
	}
	return nil
}

func (a *App) friendUpdate(w http.ResponseWriter, r *http.Request) error {
	if err := a.friendAuthorize(r); err != nil {
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
	in, err := decodeFriend(w, r)
	if err != nil {
		return err
	}
	value, err := a.Write(r, []string{"owner"}, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		if err := friendVersion(r.Context(), tx, id, version); err != nil {
			return nil, err
		}
		item, err := scanFriend(tx.QueryRow(r.Context(), "UPDATE friend_links SET name=$2,url=$3,description=$4,avatar_url=$5,sort_order=$6,visible=$7,version=version+1,updated_at_ms=$8 WHERE id=$1 RETURNING "+friendColumns, id, in.Name, in.URL, in.Description, in.AvatarURL, in.SortOrder, in.Visible, NowMillis()))
		if err != nil {
			return nil, err
		}
		if err := audit(r.Context(), tx, p, "friend.update", "friend", id, ""); err != nil {
			return nil, err
		}
		return item, nil
	})
	if err == nil {
		JSON(w, 200, value)
	}
	return err
}

func (a *App) friendDelete(w http.ResponseWriter, r *http.Request) error {
	if err := a.friendAuthorize(r); err != nil {
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
		if err := friendVersion(r.Context(), tx, id, version); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(r.Context(), "DELETE FROM friend_links WHERE id=$1", id); err != nil {
			return nil, err
		}
		return nil, audit(r.Context(), tx, p, "friend.delete", "friend", id, "")
	})
	if err == nil {
		JSON(w, 204, nil)
	}
	return err
}

func (a *App) friendSettingsGet(w http.ResponseWriter, r *http.Request) error {
	if _, err := a.Require(r, "owner"); err != nil {
		return err
	}
	settings, err := a.FriendSettings(r.Context())
	if err == nil {
		JSON(w, 200, settings)
	}
	return err
}

func (a *App) friendSettingsPut(w http.ResponseWriter, r *http.Request) error {
	if err := a.friendAuthorize(r); err != nil {
		return err
	}
	version, err := IfMatch(r)
	if err != nil {
		return err
	}
	var in struct {
		Enabled     *bool  `json:"enabled"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := decodeFriendJSON(w, r, &in); err != nil {
		return err
	}
	if in.Enabled == nil {
		return E(400, "VALIDATION_FAILED", "请明确设置是否启用友链页面")
	}
	if in.Title, err = friendText(in.Title, 80, true); err != nil {
		return err
	}
	if in.Description, err = friendDescription(in.Description, 400); err != nil {
		return err
	}
	value, err := a.Write(r, []string{"owner"}, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		var actual int64
		if err := tx.QueryRow(r.Context(), "SELECT version FROM friend_link_settings WHERE id=1 FOR UPDATE").Scan(&actual); err != nil {
			return nil, err
		}
		if actual != version {
			return nil, E(409, "VERSION_CONFLICT", "友链页面设置已变化，请重新加载后再保存")
		}
		if _, err := tx.Exec(r.Context(), "UPDATE friend_link_settings SET enabled=$1,title=$2,description=$3,version=version+1 WHERE id=1", *in.Enabled, in.Title, in.Description); err != nil {
			return nil, err
		}
		if err := audit(r.Context(), tx, p, "site.settings_update", "site", "friend-links", ""); err != nil {
			return nil, err
		}
		return map[string]any{"enabled": *in.Enabled, "title": in.Title, "description": in.Description, "version": actual + 1}, nil
	})
	if err == nil {
		JSON(w, 200, value)
	}
	return err
}
