package app

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

type homeGamePage struct {
	Scope string `json:"s"`
	ID    string `json:"i"`
	Value int64  `json:"v"`
	Title string `json:"t"`
}

func homeGameFilter(q url.Values) (string, error) {
	platform := q.Get("platform")
	if platform == "" {
		platform = "all"
	}
	if len(q["platform"]) > 1 || (platform != "all" && platform != "steam" && platform != "psn") {
		return "", E(400, "VALIDATION_FAILED", "游戏平台筛选可选 all、steam 或 psn")
	}
	return platform, nil
}
func homeGamesScope(order, platform string, limit int) string {
	return "games:v2:" + platform + ":" + order + ":" + strconv.Itoa(limit)
}

func homePublicItem(item map[string]any) {
	for _, field := range []string{"version", "created_at", "updated_at", "visible", "local_override"} {
		delete(item, field)
	}
	if item["kind"] == "steam" {
		platform, _ := item["platform"].(string)
		platform = homeItemPlatform("steam", platform)
		item["platform"] = platform
		item["poster_url"] = item["image_url"]
		id, _ := item["provider_id"].(string)
		if platform == "steam" && homeDigits(id, 1, 10) {
			// A ready upload is the owner's selected cover. Automatic Steam
			// portrait art is only the default, never a replacement for that upload.
			if media, _ := item["media_id"].(string); media == "" {
				item["poster_url"] = "https://cdn.akamai.steamstatic.com/steam/apps/" + id + "/library_600x900.jpg"
			}
			if item["hero_url"] == "" {
				item["hero_url"] = "https://cdn.akamai.steamstatic.com/steam/apps/" + id + "/library_hero.jpg"
			}
			if item["logo_url"] == "" {
				item["logo_url"] = "https://cdn.akamai.steamstatic.com/steam/apps/" + id + "/logo.png"
			}
		}
	}
}
func (a *App) gamesList(w http.ResponseWriter, r *http.Request) error {
	data, e := a.PublicGames(r.Context(), r.URL.Query())
	if e == nil {
		JSON(w, 200, data)
	}
	return e
}

// PublicGames is a local snapshot: explicit visibility and Steam's enabled flag
// apply to all readers, including owners. Home's display cap does not constrain
// this collection. A zero homepage count hides that shelf, not the full library.
func (a *App) PublicGames(ctx context.Context, q url.Values) (map[string]any, error) {
	platform, e := homeGameFilter(q)
	if e != nil {
		return nil, e
	}
	order := q.Get("sort")
	if order == "" {
		order = "position"
	}
	if order != "position" && order != "playtime" && order != "title" {
		return nil, E(400, "VALIDATION_FAILED", "游戏排序可选 position、playtime 或 title")
	}
	limit, e := momentLimit(q)
	if e != nil {
		return nil, e
	}
	query := Normalize(q.Get("q"))
	if len(q["q"]) > 1 || !utf8.ValidString(query) || utf8.RuneCountInString(query) > 64 {
		return nil, E(400, "VALIDATION_FAILED", "游戏名称最多64个字符")
	}
	scope := homeGamesScope(order, platform, limit)
	if query != "" {
		scope += ":q:" + hashString(query)
	}
	page := homeGamePage{Scope: scope, ID: "0", Value: -1}
	cursor := q.Get("cursor")
	if cursor != "" {
		if len(cursor) > 2048 {
			return nil, E(400, "VALIDATION_FAILED", "游戏分页游标过长")
		}
		if e = a.decodePage(cursor, &page); e != nil {
			return nil, e
		}
		if page.Scope != scope || !positiveID(page.ID) || page.Value < -1 || page.Value > 52704000 || len(page.Title) > 320 {
			return nil, E(400, "VALIDATION_FAILED", "游戏分页条件已改变，请重新加载")
		}
	}
	tx, e := a.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(context.Background())
	settings, e := homeRead(ctx, tx, false)
	if e != nil {
		return nil, e
	}
	if !settings.Config.Sections["steam"].Enabled {
		return nil, NotFound()
	}
	var total int
	if e = tx.QueryRow(ctx, `SELECT count(*) FROM homepage_items WHERE kind='steam' AND visible AND ($1::text='all' OR platform=$1) AND strpos(casefold(normalize(title,NFKC)),$2)>0`, platform, query).Scan(&total); e != nil {
		return nil, e
	}
	condition, sorting := `(position,id)>($2::bigint,$3::bigint)`, `position,id`
	if order == "playtime" {
		condition = `(COALESCE(playtime_minutes,-1)<$2::bigint OR (COALESCE(playtime_minutes,-1)=$2::bigint AND id>$3::bigint))`
		sorting = `playtime_minutes DESC NULLS LAST,id`
	}
	if order == "title" {
		condition = `(title COLLATE "C",id)>($4::text COLLATE "C",$3::bigint)`
		sorting = `title COLLATE "C",id`
	}
	// Every branch references all five typed placeholders; order is a fixed enum,
	// never a user-supplied SQL expression. No query can widen public visibility.
	statement := `SELECT ` + homeItemColumns + ` FROM homepage_items WHERE kind='steam' AND visible AND ($5::text='all' OR platform=$5) AND ($3::bigint=0 OR ` + condition + `) AND $4::text IS NOT NULL AND $2::bigint IS NOT NULL AND strpos(casefold(normalize(title,NFKC)),$6)>0 ORDER BY ` + sorting + ` LIMIT $1`
	rows, e := tx.Query(ctx, statement, limit+1, page.Value, page.ID, page.Title, platform, query)
	if e != nil {
		return nil, e
	}
	items := []map[string]any{}
	for rows.Next() {
		item, e := scanHomeItem(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		items = append(items, item)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	more := len(items) > limit
	next := ""
	if more {
		items = items[:limit]
		last := items[len(items)-1]
		p := homeGamePage{Scope: scope, ID: last["id"].(string), Title: last["title"].(string)}
		switch order {
		case "position":
			p.Value = int64(last["position"].(int))
		case "playtime":
			p.Value = -1
			if value, ok := last["playtime_minutes"].(int64); ok {
				p.Value = value
			}
		}
		next = a.encodePage(p)
	}
	for _, item := range items {
		homePublicItem(item)
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return map[string]any{"items": items, "next_cursor": next, "cursor": next, "has_more": more, "limit": limit, "max_limit": 50, "total": total, "sort": order, "platform": platform}, nil
}

func steamAssetURL(id, format, filename string) string {
	if !homeDigits(id, 1, 10) || !strings.HasPrefix(format, "steam/apps/"+id+"/") || strings.Count(format, "${FILENAME}") != 1 || filename == "" || len(format) > 512 || len(filename) > 256 || strings.Contains(format, "..") || strings.Contains(filename, "..") || strings.ContainsAny(format+filename, "\\\r\n\t") || strings.HasPrefix(filename, "/") {
		return ""
	}
	target := "https://shared.akamai.steamstatic.com/store_item_assets/" + strings.Replace(format, "${FILENAME}", filename, 1)
	if !homeRemoteImage("steam", target) {
		return ""
	}
	u, e := url.Parse(target)
	if e != nil || u.Host != "shared.akamai.steamstatic.com" || u.Fragment != "" {
		return ""
	}
	return target
}
