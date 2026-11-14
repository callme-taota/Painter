package app

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
)

type homeSteamGame struct {
	ID      int64  `json:"appid"`
	Name    string `json:"name"`
	Minutes *int64 `json:"playtime_forever"`
	Recent  int64  `json:"playtime_2weeks"`
	Icon    string `json:"img_icon_url"`
}
type homeSteamLease struct {
	Settings homeSettings
	Token    string
}

func (a *App) homeSteamFetch(ctx context.Context, s homeSettings) ([]homeItemInput, error) {
	key, e := a.decrypt(s.Cipher)
	if e != nil {
		return nil, E(503, "HOME_KEY_UNAVAILABLE", "Steam 密钥无法解密，请检查服务器加密配置")
	}
	method := "GetOwnedGames"
	if s.Mode == "recent" {
		method = "GetRecentlyPlayedGames"
	}
	target := "https://api.steampowered.com/IPlayerService/" + method + "/v0001/?steamid=" + s.SteamID + "&include_appinfo=true&include_played_free_games=true&format=json"
	if s.Mode == "recent" {
		target += "&count=0"
	}
	raw, e := a.homeProviderGET(ctx, target, string(key), 8<<20)
	if e != nil {
		return nil, e
	}
	var response struct {
		Response struct {
			Games []homeSteamGame `json:"games"`
			Count *int            `json:"game_count"`
			Total *int            `json:"total_count"`
		} `json:"response"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return nil, homeUpstreamError()
	}
	if len(response.Response.Games) == 0 {
		return nil, E(409, "HOME_STEAM_EMPTY", "Steam 未返回可见游戏，可能为空库或隐私限制；保留已有游戏")
	}
	count := response.Response.Count
	if s.Mode == "recent" {
		count = response.Response.Total
	}
	if len(response.Response.Games) > homeItemLimit || (count != nil && *count > homeItemLimit) {
		return nil, E(409, "HOME_ITEM_LIMIT", "Steam 游戏超过5000项，未修改已有内容")
	}
	if count == nil || *count != len(response.Response.Games) {
		return nil, homeUpstreamError()
	}
	games := response.Response.Games
	sort.Slice(games, func(i, j int) bool {
		l, r := int64(-1), int64(-1)
		if games[i].Minutes != nil {
			l = *games[i].Minutes
		}
		if games[j].Minutes != nil {
			r = *games[j].Minutes
		}
		if s.Mode == "recent" {
			l, r = games[i].Recent, games[j].Recent
		}
		if l == r {
			return games[i].ID < games[j].ID
		}
		return l > r
	})
	items := make([]homeItemInput, 0, len(games))
	seen := map[string]bool{}
	for i, g := range games {
		id := strconv.FormatInt(g.ID, 10)
		if !homeDigits(id, 1, 10) || seen[id] || strings.TrimSpace(g.Name) == "" || (g.Minutes != nil && (*g.Minutes < 0 || *g.Minutes > 52704000)) || g.Recent < 0 {
			return nil, homeUpstreamError()
		}
		seen[id] = true
		name := []rune(strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, g.Name))
		if len(name) > 80 {
			name = name[:80]
		}
		image := "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/" + id + "/header.jpg"
		subtitle := ""
		if g.Minutes != nil {
			subtitle = strconv.FormatInt(*g.Minutes, 10) + " 分钟"
		}
		item := homeItemInput{Kind: "steam", Title: string(name), Subtitle: subtitle, Alt: string(name), URL: "https://store.steampowered.com/app/" + id + "/", ImageURL: image, ProviderID: id, Position: i, Visible: true, PlaytimeMinutes: g.Minutes}
		if validateHomeItem(item) != nil {
			return nil, homeUpstreamError()
		}
		items = append(items, item)
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ProviderID)
	}
	art := a.steamLibraryArtwork(ctx, ids)
	for i := range items {
		items[i].HeroURL = art[items[i].ProviderID].Hero
		items[i].LogoURL = art[items[i].ProviderID].Logo
	}
	return items, nil
}
func homeLeaseClaim(ctx context.Context, tx pgx.Tx, expected int64, automatic bool) (homeSteamLease, error) {
	s, e := homeRead(ctx, tx, true)
	if e != nil {
		return homeSteamLease{}, e
	}
	now := NowMillis()
	if expected > 0 && s.Version != expected {
		return homeSteamLease{}, homeVersionError()
	}
	if automatic && !s.Auto {
		return homeSteamLease{}, nil
	}
	if s.SteamID == "" || len(s.Cipher) == 0 {
		return homeSteamLease{}, E(400, "HOME_STEAM_NOT_CONFIGURED", "请先填写 Steam ID 和 API Key")
	}
	var next int64
	var until *int64
	if e = tx.QueryRow(ctx, `SELECT next_sync_at_ms,lease_until_ms FROM homepage_settings WHERE id=1`).Scan(&next, &until); e != nil {
		return homeSteamLease{}, e
	}
	if automatic && (!s.Auto || next > now) {
		return homeSteamLease{}, nil
	}
	if until != nil && *until > now {
		if automatic {
			return homeSteamLease{}, nil
		}
		return homeSteamLease{}, E(409, "HOME_SYNC_BUSY", "Steam 同步正在进行，请稍后查看")
	}
	token := RandID()
	_, e = tx.Exec(ctx, `UPDATE homepage_settings SET lease_token=$1,lease_until_ms=$2 WHERE id=1`, token, now+60000)
	return homeSteamLease{s, token}, e
}
func homeSteamApply(ctx context.Context, tx pgx.Tx, lease homeSteamLease, items []homeItemInput) error {
	s, e := homeRead(ctx, tx, true)
	if e != nil {
		return e
	}
	var token *string
	var until *int64
	if e = tx.QueryRow(ctx, `SELECT lease_token,lease_until_ms FROM homepage_settings WHERE id=1`).Scan(&token, &until); e != nil {
		return e
	}
	if s.Version != lease.Settings.Version || token == nil || *token != lease.Token || until == nil || *until <= NowMillis() {
		return homeVersionError()
	}
	var manual int
	if e = tx.QueryRow(ctx, `SELECT count(*) FROM homepage_items WHERE source<>'steam_sync'`).Scan(&manual); e != nil {
		return e
	}
	if manual+len(items) > homeItemLimit {
		return E(409, "HOME_ITEM_LIMIT", "游戏与手动条目合计超过5000项，未修改已有内容")
	}
	// All home writes use settings -> item -> sorted media lock order. Reused
	// overrides stay referenced and are locked before the set-based upsert.
	if _, e = tx.Exec(ctx, `SELECT id FROM homepage_items WHERE source='steam_sync' ORDER BY id FOR UPDATE`); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, `SELECT m.id FROM media_assets m WHERE EXISTS(SELECT 1 FROM homepage_items h WHERE h.media_id=m.id AND h.source='steam_sync') ORDER BY m.id FOR UPDATE`); e != nil {
		return e
	}
	raw, _ := json.Marshal(items)
	now := NowMillis()
	if _, e = tx.Exec(ctx, `DELETE FROM homepage_items WHERE source='steam_sync' AND provider_id NOT IN(SELECT x.provider_id FROM jsonb_to_recordset($1::jsonb) AS x(provider_id text))`, raw); e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO homepage_items(kind,title,subtitle,alt,url,image_url,provider_id,source,position,visible,created_at_ms,updated_at_ms,hero_url,logo_url,playtime_minutes)
 SELECT kind,title,subtitle,alt,url,image_url,provider_id,'steam_sync',position,true,$2,$2,COALESCE(hero_url,''),COALESCE(logo_url,''),playtime_minutes FROM jsonb_to_recordset($1::jsonb) AS x(kind text,title text,subtitle text,alt text,url text,image_url text,provider_id text,position int,hero_url text,logo_url text,playtime_minutes bigint)
 ON CONFLICT(kind,source,provider_id) WHERE source='steam_sync' DO UPDATE SET title=CASE WHEN homepage_items.local_override THEN homepage_items.title ELSE EXCLUDED.title END,subtitle=CASE WHEN homepage_items.local_override THEN homepage_items.subtitle ELSE EXCLUDED.subtitle END,url=CASE WHEN homepage_items.local_override THEN homepage_items.url ELSE EXCLUDED.url END,image_url=CASE WHEN homepage_items.local_override THEN homepage_items.image_url ELSE EXCLUDED.image_url END,hero_url=CASE WHEN homepage_items.local_override AND homepage_items.hero_url<>'' THEN homepage_items.hero_url ELSE COALESCE(NULLIF(EXCLUDED.hero_url,''),homepage_items.hero_url) END,logo_url=CASE WHEN homepage_items.local_override AND homepage_items.logo_url<>'' THEN homepage_items.logo_url ELSE COALESCE(NULLIF(EXCLUDED.logo_url,''),homepage_items.logo_url) END,playtime_minutes=EXCLUDED.playtime_minutes,version=homepage_items.version+1,updated_at_ms=EXCLUDED.updated_at_ms`, raw, now)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `UPDATE homepage_settings SET last_sync_at_ms=$1,last_error='',sync_failures=0,next_sync_at_ms=$2,lease_token=NULL,lease_until_ms=NULL WHERE id=1`, now, now+6*time.Hour.Milliseconds())
	return e
}
func (a *App) homeSteamFailure(ctx context.Context, lease homeSteamLease) error {
	// The constant status is deliberate: transport errors can include secret data.
	return a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE homepage_settings SET last_error='Steam 同步未完成；保留已有游戏，请检查配置、隐私或稍后重试',sync_failures=LEAST(sync_failures+1,20),next_sync_at_ms=$1+LEAST(21600000,900000*(1::bigint<<LEAST(sync_failures,4))),lease_token=NULL,lease_until_ms=NULL WHERE id=1 AND version=$2 AND lease_token=$3`, NowMillis(), lease.Settings.Version, lease.Token)
		return e
	})
}
func (a *App) homeSteamSync(w http.ResponseWriter, r *http.Request) error {
	if e := homeAuthorize(a, r); e != nil {
		return e
	}
	version, e := IfMatch(r)
	if e != nil {
		return e
	}
	if e = momentCommandBody(w, r); e != nil {
		return e
	}
	value, found, e := a.Replay(r, "owner")
	if e != nil {
		return e
	}
	if found {
		JSON(w, 200, value)
		return nil
	}
	claim, e := a.Write(r, []string{"owner"}, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		lease, e := homeLeaseClaim(r.Context(), tx, version, false)
		if e != nil {
			return nil, e
		}
		if e = homeRateTx(r.Context(), tx, p.ID, "steam"); e != nil {
			return nil, e
		}
		return lease, nil
	})
	if e != nil {
		return e
	}
	lease := claim.(homeSteamLease)
	items, e := a.homeSteamFetch(r.Context(), lease.Settings)
	if e != nil {
		_ = a.homeSteamFailure(r.Context(), lease)
		return e
	}
	value, e = a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		if e := homeSteamApply(r.Context(), tx, lease, items); e != nil {
			return nil, e
		}
		s, e := homeRead(r.Context(), tx, false)
		if e != nil {
			return nil, e
		}
		if e = audit(r.Context(), tx, p, "home.steam_sync", "site", "homepage", ""); e != nil {
			return nil, e
		}
		v := a.homeSettingsDTO(s)
		v["synced_count"] = len(items)
		return v, nil
	})
	if e != nil {
		_ = a.homeSteamFailure(r.Context(), lease)
	} else {
		JSON(w, 200, value)
	}
	return e
}

// SyncHomeSteam is called from the existing worker cadence. The DB lease makes
// overlapping processes safe. Provider I/O never holds a DB connection or lock.
func (a *App) SyncHomeSteam(ctx context.Context) error {
	var lease homeSteamLease
	e := a.moduleWriteTx(ctx, func(tx pgx.Tx) error { var e error; lease, e = homeLeaseClaim(ctx, tx, 0, true); return e })
	if e != nil {
		return e
	}
	if lease.Token == "" {
		return nil
	}
	items, e := a.homeSteamFetch(ctx, lease.Settings)
	if e != nil {
		_ = a.homeSteamFailure(ctx, lease)
		return e
	}
	e = a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		if e := homeSteamApply(ctx, tx, lease, items); e != nil {
			return e
		}
		_, e := tx.Exec(ctx, `INSERT INTO audit_logs(actor_user_id,action,target_type,target_id,request_id,reason,created_at_ms) VALUES(NULL,'home.steam_sync','site','homepage',$1,'自动同步 Steam 游戏快照',$2)`, requestID(ctx), NowMillis())
		return e
	})
	if e != nil {
		_ = a.homeSteamFailure(ctx, lease)
	}
	return e
}
