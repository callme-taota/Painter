package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"painter-next/migrations"
)

func homeGameEnable(t *testing.T, a *App, owner *http.Cookie, csrf string, version int64, enabled bool, count int) {
	t.Helper()
	c, _ := homeTestConfig(t, a, owner)
	steam := c["sections"].(map[string]any)["steam"].(map[string]any)
	steam["enabled"], steam["limit"] = enabled, count
	homeTestPut(t, a, owner, csrf, c, map[string]any{"steam_id": "", "mode": "owned", "auto_sync_enabled": false}, version)
}

func homeGameAll(t *testing.T, a *App, order string, limit int) []map[string]any {
	t.Helper()
	items := []map[string]any{}
	cursor := ""
	for range 20 {
		q := url.Values{"sort": {order}, "limit": {strconv.Itoa(limit)}, "cursor": {cursor}}
		page := momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/games?"+q.Encode(), nil, 0, ""), 200)
		for _, raw := range page["items"].([]any) {
			items = append(items, raw.(map[string]any))
		}
		if page["total"] != float64(7) || page["max_limit"] != float64(50) {
			t.Fatal("full library count/cap", page)
		}
		cursor = page["next_cursor"].(string)
		if !page["has_more"].(bool) {
			if cursor != "" {
				t.Fatal("terminal cursor", page)
			}
			return items
		}
		if cursor == "" {
			t.Fatal("missing continuation")
		}
	}
	t.Fatal("unbounded game pagination")
	return nil
}

func TestHomeGamesPublicPagingSortingVisibilityAndHomeCap(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	var requests atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, fmt.Errorf("visitor must not call providers")
	})
	ids := []string{}
	for i, title := range []string{"Zulu", "Alpha", "Beta", "Alpha", "Delta", "Echo", "Foxtrot", "Hidden"} {
		in := homeTestInput("steam")
		in["title"], in["position"], in["visible"], in["provider_id"] = title, i/2, i != 7, strconv.Itoa(700+i)
		item := homeTestItem(t, a, owner, csrf, in)
		ids = append(ids, item["id"].(string))
	}
	for i, minutes := range []any{nil, 0, 120, 120, 3600, nil, 60} {
		if _, e := a.Pool.Exec(context.Background(), `UPDATE homepage_items SET playtime_minutes=$1 WHERE id=$2`, minutes, ids[i]); e != nil {
			t.Fatal(e)
		}
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "GET", "/api/v1/games", nil, 0, ""), 404)
	homeGameEnable(t, a, owner, csrf, 1, true, 24) // Older settings remain valid.
	for order, expected := range map[string][]int{"position": {0, 1, 2, 3, 4, 5, 6}, "title": {1, 3, 2, 4, 5, 6, 0}, "playtime": {4, 2, 3, 6, 1, 0, 5}} {
		items := homeGameAll(t, a, order, 2)
		if len(items) != 7 {
			t.Fatal(order, "missing/duplicate games", items)
		}
		for n, index := range expected {
			if items[n]["id"] != ids[index] || items[n]["version"] != nil || items[n]["visible"] != nil || items[n]["local_override"] != nil {
				t.Fatal(order, "sort or private projection", items)
			}
			if items[n]["hero_url"] != "https://cdn.akamai.steamstatic.com/steam/apps/"+strconv.Itoa(700+index)+"/library_hero.jpg" {
				t.Fatal("legacy official hero fallback", items[n])
			}
		}
	}
	data, e := a.HomepageData(context.Background())
	if e != nil || len(data["walls"].(map[string][]map[string]any)["steam"]) != 5 || data["config"].(HomeConfig).Sections["steam"].Limit != 24 {
		t.Fatal("home cap changed saved config or full library", data, e)
	}
	first := momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/games?limit=2", nil, 0, ""), 200)
	cursor := url.QueryEscape(first["next_cursor"].(string))
	for _, path := range []string{"/api/v1/games?limit=3&cursor=" + cursor, "/api/v1/games?limit=2&sort=title&cursor=" + cursor, "/api/v1/games?cursor=invalid", "/api/v1/games?sort=evil", "/api/v1/games?limit=0"} {
		momentResponse(t, momentCall(t, a, nil, "", "GET", path, nil, 0, ""), 400)
	}
	if page := momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/games?limit=51", nil, 0, ""), 200); page["limit"] != float64(50) {
		t.Fatal("page cap did not clamp to existing collection contract", page)
	}
	homeGameEnable(t, a, owner, csrf, 2, true, 0)
	if len(homeGameAll(t, a, "position", 3)) != 7 {
		t.Fatal("home count zero incorrectly hides complete library")
	}
	data, e = a.HomepageData(context.Background())
	if e != nil || len(data["walls"].(map[string][]map[string]any)["steam"]) != 0 {
		t.Fatal("zero shelf cap", data, e)
	}
	homeGameEnable(t, a, owner, csrf, 3, false, 5)
	momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/games", nil, 0, ""), 404)
	if requests.Load() != 0 {
		t.Fatal("public rendering requested a provider", requests.Load())
	}
}

func TestHomeGamesArtValidationReadOnlyStatisticsAndLegacyEdits(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	_, author, authorCSRF := testIdentity(t, a, "author")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	in := homeTestInput("steam")
	hero, logo := "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/730/hash/library_hero.jpg?t=123", "https://cdn.akamai.steamstatic.com/steam/apps/730/logo.png"
	in["hero_url"], in["logo_url"], in["playtime_minutes"] = hero, logo, nil
	key := RandID()
	created := momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/items", in, 0, key), 201)
	if created["hero_url"] != hero || created["logo_url"] != logo || created["playtime_minutes"] != nil {
		t.Fatal("create art/stat projection", created)
	}
	if replay := momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/items", in, 0, key), 201); !reflect.DeepEqual(created, replay) {
		t.Fatal("create replay changed")
	}
	id := created["id"].(string)
	legacy := homeTestInput("steam")
	updated := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, legacy, 1, RandID()), 200)
	if updated["hero_url"] != hero || updated["logo_url"] != logo {
		t.Fatal("old client omission erased art", updated)
	}
	legacy["hero_url"], legacy["logo_url"] = "", ""
	reset := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, legacy, 2, RandID()), 200)
	if reset["hero_url"] != "" || reset["logo_url"] != "" {
		t.Fatal("explicit reset is not raw automatic", reset)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, legacy, 2, RandID()), 409)
	for _, field := range []string{"hero_url", "logo_url"} {
		for _, value := range []any{nil, "http://cdn.akamai.steamstatic.com/a.jpg", "https://evil.test/a.jpg", "https://x:secret@cdn.akamai.steamstatic.com/a.png"} {
			bad := homeTestInput("steam")
			bad[field] = value
			momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/items", bad, 0, RandID()), 400)
		}
	}
	for _, minutes := range []int64{0, 1, -1, 52704001} {
		bad := homeTestInput("steam")
		bad["playtime_minutes"] = minutes
		momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/items", bad, 0, RandID()), 400)
		momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, bad, 3, RandID()), 400)
	}
	music := homeTestInput("music")
	music["image_url"], music["hero_url"] = "https://is1-ssl.mzstatic.com/a.jpg", hero
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/items", music, 0, RandID()), 400)
	for _, actor := range []struct {
		cookie *http.Cookie
		csrf   string
	}{{author, authorCSRF}, {reader, readerCSRF}} {
		momentResponse(t, momentCall(t, a, actor.cookie, actor.csrf, "PUT", "/api/v1/admin/home/items/"+id, legacy, 3, RandID()), 403)
	}
	momentResponse(t, momentCall(t, a, owner, "bad", "PUT", "/api/v1/admin/home/items/"+id, legacy, 3, RandID()), 403)
}

func homeGameArtResponse(ids []string) string {
	items := []map[string]any{}
	for _, id := range ids {
		n, _ := strconv.ParseInt(id, 10, 64)
		items = append(items, map[string]any{"id": n, "appid": n, "item_type": 0, "success": 1, "assets": map[string]any{"asset_url_format": "steam/apps/" + id + "/${FILENAME}?t=123", "library_hero": "hash/library_hero.jpg", "library_hero_2x": "hash/library_hero_2x.jpg", "library_logo": "hash/logo.png"}})
	}
	b, _ := json.Marshal(map[string]any{"response": map[string]any{"store_items": items}})
	return string(b)
}

func homeGameArtIDs(t *testing.T, r *http.Request) []string {
	t.Helper()
	if r.URL.Scheme != "https" || r.URL.Host != "api.steampowered.com" || r.URL.Path != "/IStoreBrowseService/GetItems/v1/" || r.Header.Get("x-webapi-key") != "" || r.URL.Query().Get("key") != "" {
		t.Error("public artwork endpoint/credentials", r.URL)
	}
	var input struct {
		IDs []struct {
			AppID int64 `json:"appid"`
		} `json:"ids"`
	}
	if e := json.Unmarshal([]byte(r.URL.Query().Get("input_json")), &input); e != nil || len(input.IDs) < 1 || len(input.IDs) > 50 {
		t.Fatal("art batching", e, len(input.IDs))
	}
	ids := []string{}
	for _, id := range input.IDs {
		ids = append(ids, strconv.FormatInt(id.AppID, 10))
	}
	return ids
}

func TestHomeGamesRecentLifetimeUnknownZeroAndArtCache(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	homeSteamConfigure(t, a, owner, csrf, false)
	c, _ := homeTestConfig(t, a, owner)
	homeTestPut(t, a, owner, csrf, c, map[string]any{"steam_id": "76561198000000000", "mode": "recent", "auto_sync_enabled": false}, 2)
	var artCalls, gameCalls atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			artCalls.Add(1)
			return homeHTTP(200, homeGameArtResponse(homeGameArtIDs(t, r))), nil
		}
		gameCalls.Add(1)
		if !strings.Contains(r.URL.Path, "GetRecentlyPlayedGames") || r.Header.Get("x-webapi-key") != "private-steam-key" {
			t.Error("recent endpoint/key")
		}
		return homeHTTP(200, `{"response":{"total_count":4,"games":[{"appid":101,"name":"Known","playtime_forever":120,"playtime_2weeks":9},{"appid":102,"name":"Zero","playtime_forever":0,"playtime_2weeks":8},{"appid":103,"name":"Unknown","playtime_2weeks":7},{"appid":104,"name":"Long","playtime_forever":3600,"playtime_2weeks":6}]}}`), nil
	})
	for range 2 {
		homeSyncCall(t, a, owner, csrf, 3, 200)
	}
	items := momentItems(t, a, owner, "/api/v1/admin/home/items?kind=steam")
	if len(items) != 4 || artCalls.Load() != 1 || gameCalls.Load() != 2 {
		t.Fatal("art cache or snapshot", items, artCalls.Load(), gameCalls.Load())
	}
	for i, raw := range items {
		item := raw.(map[string]any)
		id := strconv.Itoa(101 + i)
		var expected any
		if i != 2 {
			expected = []float64{120, 0, 0, 3600}[i]
		}
		if item["provider_id"] != id || item["playtime_minutes"] != expected || !strings.Contains(item["hero_url"].(string), "/hash/library_hero.jpg?t=123") || strings.Contains(item["hero_url"].(string), "2x") {
			t.Fatal("lifetime/zero/NULL or normal hero", item, expected)
		}
	}
}

func TestHomeGamesInFlightArtOverrideRetainsMetadataRefreshesStatistics(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	homeSteamConfigure(t, a, owner, csrf, false)
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			return homeHTTP(200, homeGameArtResponse(homeGameArtIDs(t, r))), nil
		}
		return homeHTTP(200, homeGames), nil
	})
	homeSyncCall(t, a, owner, csrf, 2, 200)
	item := momentItems(t, a, owner, "/api/v1/admin/home/items?kind=steam")[0].(map[string]any)
	entered, release := make(chan struct{}), make(chan struct{})
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			return homeHTTP(200, homeGameArtResponse(homeGameArtIDs(t, r))), nil
		}
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		return homeHTTP(200, strings.Replace(homeGames, `"playtime_forever":7`, `"playtime_forever":900`, 1)), nil
	})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/steam-sync", nil, 2, RandID())
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("sync did not reach upstream")
	}
	in := homeTestInput("steam")
	in["provider_id"], in["title"], in["position"] = item["provider_id"], "My custom title", 99
	in["hero_url"], in["logo_url"] = "https://cdn.akamai.steamstatic.com/steam/apps/570/custom_hero.jpg", "https://cdn.akamai.steamstatic.com/steam/apps/570/custom_logo.png"
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+item["id"].(string), in, 1, RandID()), 200)
	close(release)
	select {
	case w := <-done:
		momentResponse(t, w, 200)
	case <-time.After(3 * time.Second):
		t.Fatal("sync did not finish")
	}
	after := momentResponse(t, momentCall(t, a, owner, csrf, "GET", "/api/v1/admin/home/items/"+item["id"].(string), nil, 0, ""), 200)
	if after["hero_url"] != in["hero_url"] || after["logo_url"] != in["logo_url"] || after["title"] != in["title"] || after["position"] != float64(99) || after["playtime_minutes"] != float64(900) || after["local_override"] != true {
		t.Fatal("in-flight sync lost override or actual updated statistics", after)
	}
}

func TestHomeGamesArtworkBoundedBatchingCacheAndCancellation(t *testing.T) {
	a := homeTestApp(t)
	if _, e := a.Pool.Exec(context.Background(), `INSERT INTO homepage_import_cache(kind,provider_id,country,metadata,expires_at_ms) SELECT 'steam',n::text,'seed','{}'::jsonb,$1 FROM generate_series(10000,14999)n`, NowMillis()+time.Hour.Milliseconds()); e != nil {
		t.Fatal(e)
	}
	ids := []string{}
	for n := 1; n <= 230; n++ {
		ids = append(ids, strconv.Itoa(n))
	}
	var calls atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if n := a.Pool.Stat().AcquiredConns(); n != 0 {
			t.Error("provider fetch holds DB connection", n)
		}
		return homeHTTP(200, homeGameArtResponse(homeGameArtIDs(t, r))), nil
	})
	if art := a.steamLibraryArtwork(context.Background(), ids); len(art) != 200 || calls.Load() != 4 {
		t.Fatal("one-call art cap", len(art), calls.Load())
	}
	if art := a.steamLibraryArtwork(context.Background(), ids); len(art) != 230 || calls.Load() != 5 {
		t.Fatal("art cache did not resume remaining IDs", len(art), calls.Load())
	}
	if art := a.steamLibraryArtwork(context.Background(), ids); len(art) != 230 || calls.Load() != 5 {
		t.Fatal("art cache missed", len(art), calls.Load())
	}
	var cached int
	if e := a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_import_cache`).Scan(&cached); e != nil || cached != 5000 {
		t.Fatal("shared metadata cache exceeds storage cap", cached, e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if n := a.Pool.Stat().AcquiredConns(); n != 0 {
			t.Error("bounded fetch holds DB connection", n)
		}
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	start := time.Now()
	if art := a.steamLibraryArtwork(ctx, []string{"999"}); len(art) != 0 || time.Since(start) > time.Second {
		t.Fatal("cancelled enhancement was not optional/bounded", art)
	}
	for _, bad := range []struct{ format, name string }{{"steam/apps/2/${FILENAME}", "hero.jpg"}, {"steam/apps/1/${FILENAME}", "../hero.jpg"}, {"https://evil.test/${FILENAME}", "hero.jpg"}, {"steam/apps/1/${FILENAME}#x", "hero.jpg"}, {"steam/apps/1/${FILENAME}", "/hero.jpg"}} {
		if steamAssetURL("1", bad.format, bad.name) != "" {
			t.Fatal("invalid provider path accepted", bad)
		}
	}
}

func TestHomeGamesManualImportEnhancesOfficialArtWithoutInventingStatistics(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	var calls atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Host == "store.steampowered.com" {
			return homeHTTP(200, `{"730":{"success":true,"data":{"steam_appid":730,"name":"Authoritative title","header_image":"https://cdn.akamai.steamstatic.com/steam/apps/730/header.jpg"}}}`), nil
		}
		homeGameArtIDs(t, r)
		return homeHTTP(200, `{"response":{"store_items":[{"id":730,"appid":730,"item_type":0,"success":1,"assets":{"asset_url_format":"steam/apps/730/${FILENAME}?t=7","library_hero_2x":"hash/library_hero_2x.jpg","library_logo":"hash/logo.png"}},{"id":999,"appid":999,"item_type":0,"success":1,"assets":{"asset_url_format":"steam/apps/999/${FILENAME}","library_hero":"bad.jpg"}}]}}`), nil
	})
	for range 2 {
		preview := homeImportCall(t, a, owner, csrf, "steam", "730", 200)
		if preview["title"] != "Authoritative title" || preview["hero_url"] != "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/730/hash/library_hero_2x.jpg?t=7" || preview["logo_url"] != "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/730/hash/logo.png?t=7" || preview["playtime_minutes"] != nil {
			t.Fatal("manual import replaced metadata, lost actual optional art or fabricated playtime", preview)
		}
	}
	var items, unknown int
	if e := a.Pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM homepage_items),(SELECT count(*) FROM homepage_import_cache WHERE provider_id='999')`).Scan(&items, &unknown); e != nil || items != 0 || unknown != 0 || calls.Load() != 2 {
		t.Fatal("preview autosave, unrequested ID cache or cache miss", items, unknown, calls.Load(), e)
	}
}

func TestHomeGamesMigration18BackfillsOnlyExactProviderStatistics(t *testing.T) {
	a := homeTestApp(t)
	ctx := context.Background()
	if _, e := a.Pool.Exec(ctx, `DROP INDEX homepage_games_playtime; DROP INDEX homepage_games_title; ALTER TABLE homepage_items DROP COLUMN hero_url CASCADE,DROP COLUMN logo_url,DROP COLUMN playtime_minutes`); e != nil {
		t.Fatal(e)
	}
	fixtures := []struct {
		source, subtitle string
		override         bool
		expected         any
	}{
		{"steam_sync", "120 分钟", false, int64(120)}, {"steam_sync", "0 分钟", false, nil}, {"manual", "120 分钟", false, nil}, {"steam_sync", "120 分钟", true, nil}, {"steam_sync", "1.5 小时", false, nil}, {"steam_sync", "-1 分钟", false, nil}, {"steam_sync", "999999999999999999999999 分钟", false, nil}, {"steam_sync", "52704001 分钟", false, nil}, {"steam_sync", "52704000 分钟", false, int64(52704000)},
	}
	ids := []string{}
	for n, f := range fixtures {
		var id string
		if e := a.Pool.QueryRow(ctx, `INSERT INTO homepage_items(kind,title,subtitle,image_url,source,provider_id,local_override,created_at_ms,updated_at_ms) VALUES('steam','old title',$1,'https://cdn.akamai.steamstatic.com/header.jpg',$2,$3,$4,10,20) RETURNING id::text`, f.subtitle, f.source, strconv.Itoa(n+1), f.override).Scan(&id); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	var before string
	if e := a.Pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(h) ORDER BY id)::text FROM homepage_items h`).Scan(&before); e != nil {
		t.Fatal(e)
	}
	raw, e := migrations.Files.ReadFile("0018_game_showcase.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.Pool.Exec(ctx, string(raw)); e != nil {
		t.Fatal("upgrade from V22 shape", e)
	}
	var after string
	if e = a.Pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(h)-'hero_url'-'logo_url'-'playtime_minutes' ORDER BY id)::text FROM homepage_items h`).Scan(&after); e != nil || before != after {
		t.Fatal("migration mutated existing fields", e, before, after)
	}
	for i, id := range ids {
		var minutes *int64
		if e = a.Pool.QueryRow(ctx, `SELECT playtime_minutes FROM homepage_items WHERE id=$1`, id).Scan(&minutes); e != nil {
			t.Fatal(e)
		}
		var value any
		if minutes != nil {
			value = *minutes
		}
		if value != fixtures[i].expected {
			t.Fatal("unsafe/missing backfill", fixtures[i], value)
		}
	}
	for _, statement := range []string{`UPDATE homepage_items SET playtime_minutes=-1 WHERE id=1`, `UPDATE homepage_items SET playtime_minutes=52704001 WHERE id=1`, `UPDATE homepage_items SET kind='music',hero_url='https://cdn.akamai.steamstatic.com/hero.jpg' WHERE id=3`} {
		if _, e = a.Pool.Exec(ctx, statement); e == nil {
			t.Fatal("database allowed invalid showcase data", statement)
		}
	}
}

func TestHomeGamesSteamCapacityIncludesPlaylistRows(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	homeSteamConfigure(t, a, owner, csrf, false)
	if _, e := a.Pool.Exec(context.Background(), `INSERT INTO homepage_items(kind,title,image_url,source,provider_id,created_at_ms,updated_at_ms) SELECT 'music','playlist album','https://is1-ssl.mzstatic.com/a.jpg','music_playlist',n::text,0,0 FROM generate_series(1,4999)n`); e != nil {
		t.Fatal(e)
	}
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			return homeHTTP(503, "optional art unavailable"), nil
		}
		return homeHTTP(200, homeGames), nil
	})
	homeSyncCall(t, a, owner, csrf, 2, 409)
	var total, synced int
	if e := a.Pool.QueryRow(context.Background(), `SELECT count(*),count(*) FILTER(WHERE source='steam_sync') FROM homepage_items`).Scan(&total, &synced); e != nil || total != 4999 || synced != 0 {
		t.Fatal("playlist+Steam cap rollback", total, synced, e)
	}
}
