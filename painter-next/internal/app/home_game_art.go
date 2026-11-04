package app

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type steamLibraryArt struct {
	Hero string `json:"hero_url"`
	Logo string `json:"logo_url"`
}

// Steam's own public store metadata returns actual asset paths, including hash
// directories and cache revisions. Enhancement is bounded and best-effort;
// visitor rendering only uses this local cache or official legacy fallbacks.
func (a *App) steamLibraryArtwork(ctx context.Context, ids []string) map[string]steamLibraryArt {
	result := map[string]steamLibraryArt{}
	unique := []string{}
	seen := map[string]bool{}
	for _, id := range ids {
		if homeDigits(id, 1, 10) && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	rows, e := a.Pool.Query(ctx, `SELECT provider_id,metadata FROM homepage_import_cache WHERE kind='steam' AND country='library-v1' AND provider_id=ANY($1::text[]) AND expires_at_ms>$2`, unique, NowMillis())
	if e == nil {
		for rows.Next() {
			var id string
			var raw []byte
			if rows.Scan(&id, &raw) != nil {
				continue
			}
			var art steamLibraryArt
			if json.Unmarshal(raw, &art) == nil && (art.Hero == "" || homeRemoteImage("steam", art.Hero)) && (art.Logo == "" || homeRemoteImage("steam", art.Logo)) {
				result[id] = art
			}
		}
		rows.Close()
	}
	missing := []string{}
	for _, id := range unique {
		if _, ok := result[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 200 {
		missing = missing[:200]
	}
	for start := 0; start < len(missing) && ctx.Err() == nil; start += 50 {
		batch := missing[start:min(start+50, len(missing))]
		requested := map[string]bool{}
		params := []map[string]int64{}
		for _, id := range batch {
			n, _ := strconv.ParseInt(id, 10, 64)
			params = append(params, map[string]int64{"appid": n})
			requested[id] = true
		}
		payload, _ := json.Marshal(map[string]any{"ids": params, "context": map[string]string{"language": "english", "country_code": "US"}, "data_request": map[string]bool{"include_assets": true}})
		raw, e := a.homeProviderGET(ctx, "https://api.steampowered.com/IStoreBrowseService/GetItems/v1/?input_json="+url.QueryEscape(string(payload)), "", 2<<20)
		if e != nil {
			continue
		}
		var response struct {
			Response struct {
				Items []struct {
					ID       int64 `json:"id"`
					AppID    int64 `json:"appid"`
					ItemType int   `json:"item_type"`
					Success  int   `json:"success"`
					Assets   struct {
						Format string `json:"asset_url_format"`
						Hero   string `json:"library_hero"`
						Hero2x string `json:"library_hero_2x"`
						Logo   string `json:"library_logo"`
					} `json:"assets"`
				} `json:"store_items"`
			} `json:"response"`
		}
		if json.Unmarshal(raw, &response) != nil {
			continue
		}
		fetched := map[string]steamLibraryArt{}
		for _, item := range response.Response.Items {
			id := strconv.FormatInt(item.ID, 10)
			if !requested[id] || item.ID != item.AppID || item.ItemType != 0 || item.Success != 1 {
				continue
			}
			hero := item.Assets.Hero
			if hero == "" {
				hero = item.Assets.Hero2x
			}
			art := steamLibraryArt{steamAssetURL(id, item.Assets.Format, hero), steamAssetURL(id, item.Assets.Format, item.Assets.Logo)}
			fetched[id] = art
			result[id] = art
		}
		if len(fetched) > 0 {
			_ = a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
				// Share the home write barrier with single-item/playlist imports.
				// Cache maintenance is brief and never encloses provider I/O.
				if _, e := homeRead(ctx, tx, true); e != nil {
					return e
				}
				if _, e := tx.Exec(ctx, `DELETE FROM homepage_import_cache WHERE expires_at_ms<=$1`, NowMillis()); e != nil {
					return e
				}
				for id, art := range fetched {
					body, _ := json.Marshal(art)
					ttl := 7 * 24 * time.Hour
					if art.Hero == "" && art.Logo == "" {
						ttl = 24 * time.Hour
					}
					if _, e := tx.Exec(ctx, `INSERT INTO homepage_import_cache(kind,provider_id,country,metadata,expires_at_ms) VALUES('steam',$1,'library-v1',$2,$3) ON CONFLICT(kind,provider_id,country) DO UPDATE SET metadata=EXCLUDED.metadata,expires_at_ms=EXCLUDED.expires_at_ms`, id, body, NowMillis()+ttl.Milliseconds()); e != nil {
						return e
					}
				}
				_, e := tx.Exec(ctx, `DELETE FROM homepage_import_cache WHERE (kind,provider_id,country) IN(SELECT kind,provider_id,country FROM homepage_import_cache ORDER BY expires_at_ms DESC,kind,provider_id,country OFFSET 5000)`)
				return e
			})
		}
	}
	return result
}

// Called by home_import.go after the authoritative AppID/title/header lookup.
// This helper never invents playtime or requires the account's private API key.
func (a *App) enhanceSteamImport(ctx context.Context, item homeItemInput) homeItemInput {
	if item.Kind != "steam" || !homeDigits(item.ProviderID, 1, 10) {
		return item
	}
	art := a.steamLibraryArtwork(ctx, []string{item.ProviderID})[item.ProviderID]
	if art.Hero != "" {
		item.HeroURL = art.Hero
	}
	if art.Logo != "" {
		item.LogoURL = art.Logo
	}
	return item
}
