package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"painter-next/migrations"
)

func homePlatformFixture() homeItemInput {
	return homeItemInput{Kind: "steam", Platform: "psn", ProviderID: "CUSA_12345:00", Title: "Saved PlayStation game", Platforms: []string{"PS5"}}
}

func TestHomePlatformValidationPreservesUnknownAndLegacyGames(t *testing.T) {
	psn := homePlatformFixture()
	zero := int64(0)
	steam := homeItemInput{Kind: "steam", ProviderID: "730", Title: "Saved Steam game", ImageURL: "https://cdn.akamai.steamstatic.com/steam/apps/730/header.jpg"}
	for _, sample := range []struct {
		name string
		in   homeItemInput
	}{
		{"legacy Steam request", steam},
		{"PSN unknown time and missing artwork", psn},
		{"PSN observed zero time", func() homeItemInput { in := psn; in.PlaytimeMinutes = &zero; return in }()},
		{"PSN official artwork", func() homeItemInput {
			in := psn
			in.ImageURL = "https://image.api.playstation.com/actual/portrait.png"
			return in
		}()},
		{"PSN numeric identity", func() homeItemInput { in := psn; in.ProviderID = "730"; return in }()},
		{"PSN both host platforms", func() homeItemInput { in := psn; in.Platforms = []string{"PS4", "PS5"}; return in }()},
	} {
		t.Run(sample.name, func(t *testing.T) {
			if err := validateHomeItem(sample.in); err != nil {
				t.Fatal(err)
			}
		})
	}
	if psn.PlaytimeMinutes != nil || steam.Platform != "" {
		t.Fatal("validation mutated input or fabricated unknown statistics")
	}
	for _, mutate := range []func(*homeItemInput){
		func(in *homeItemInput) { in.Platform = "xbox" },
		func(in *homeItemInput) { in.Platforms = []string{"PS3"} },
		func(in *homeItemInput) { in.Platforms = []string{"PS5", "PS5"} },
		func(in *homeItemInput) { in.Platforms = []string{"ps5"} },
		func(in *homeItemInput) { in.Platform = "steam" },
		func(in *homeItemInput) { in.Kind = "music" },
		func(in *homeItemInput) { in.ProviderID = "" },
		func(in *homeItemInput) { in.ProviderID = "CUSA/123" },
		func(in *homeItemInput) { in.ProviderID = strings.Repeat("A", 65) },
		func(in *homeItemInput) { in.ProviderID = "CUSA\n123" },
		func(in *homeItemInput) { in.ProviderID = "游戏123" },
		func(in *homeItemInput) { in.ImageURL = steam.ImageURL },
		func(in *homeItemInput) { in.HeroURL = steam.ImageURL },
		func(in *homeItemInput) { in.LogoURL = "https://image.api.playstation.com.evil.test/logo.png" },
		func(in *homeItemInput) { minutes := int64(-1); in.PlaytimeMinutes = &minutes },
		func(in *homeItemInput) { minutes := int64(52704001); in.PlaytimeMinutes = &minutes },
	} {
		in := homePlatformFixture()
		mutate(&in)
		if err := validateHomeItem(in); err == nil {
			t.Fatalf("invalid platform item accepted: %+v", in)
		}
	}
}

func TestHomePlatformEditsRetainOmittedIdentityAndDeviceList(t *testing.T) {
	var legacy homeItemInput
	if err := json.Unmarshal([]byte(`{"kind":"steam","title":"My saved title","provider_id":"CUSA_12345:00"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	old := []string{"PS4", "PS5"}
	if err := inheritHomeItemPlatform(&legacy, "psn", old); err != nil {
		t.Fatal(err)
	}
	if legacy.Platform != "psn" || !reflect.DeepEqual(legacy.Platforms, old) || validateHomeItem(legacy) != nil {
		t.Fatal("legacy edit changed an existing PSN identity or device list", legacy)
	}
	legacy.Platforms[0] = "PS5"
	if old[0] != "PS4" {
		t.Fatal("request device edits alias the saved slice")
	}
	var cleared homeItemInput
	if err := json.Unmarshal([]byte(`{"kind":"steam","platform":"psn","platforms":[],"provider_id":"730"}`), &cleared); err != nil {
		t.Fatal(err)
	}
	if err := inheritHomeItemPlatform(&cleared, "psn", old); err != nil || len(cleared.Platforms) != 0 {
		t.Fatal("explicit empty platforms failed to clear the optional list", cleared, err)
	}
	for _, raw := range []string{`{"kind":"steam","platform":"steam"}`, `{"kind":"steam","platform":""}`} {
		var changed homeItemInput
		if err := json.Unmarshal([]byte(raw), &changed); err != nil {
			t.Fatal(err)
		}
		if err := inheritHomeItemPlatform(&changed, "psn", old); err == nil {
			t.Fatal("explicit cross-platform identity change accepted", raw)
		}
	}
	for _, raw := range []string{`{"platform":null}`, `{"platforms":"PS5"}`, `{"unknown_platform_field":true}`} {
		if json.Unmarshal([]byte(raw), &homeItemInput{}) == nil {
			t.Fatal("invalid platform JSON accepted", raw)
		}
	}
	legacy = homeItemInput{Kind: "steam"}
	normalizeHomeItemPlatform(&legacy)
	if legacy.Platform != "steam" || legacy.Platforms == nil || len(legacy.Platforms) != 0 {
		t.Fatal("legacy creation did not default to Steam with an empty device list", legacy)
	}
}

func TestHomePlatformPublicArtworkNeverUsesSteamForPSN(t *testing.T) {
	for _, image := range []string{"", "https://image.api.np.km.playstation.net/actual/portrait.png", "/media/custom-cover"} {
		item := map[string]any{
			"kind": "steam", "platform": "psn", "platforms": []string{"PS5"},
			"provider_id": "730", "source": "psn_sync", "image_url": image,
			"hero_url": "", "logo_url": "", "playtime_minutes": nil,
			"version": int64(3), "visible": false, "local_override": true,
		}
		homePublicItem(item)
		if item["poster_url"] != image || item["hero_url"] != "" || item["logo_url"] != "" || item["playtime_minutes"] != nil {
			t.Fatal("PSN artwork or unknown playtime was fabricated", item)
		}
		if item["platform"] != "psn" || item["version"] != nil || item["visible"] != nil || item["local_override"] != nil {
			t.Fatal("platform projection lost its identity or leaked private fields", item)
		}
	}
	legacy := map[string]any{"kind": "steam", "provider_id": "730", "image_url": "saved-header", "hero_url": "", "logo_url": ""}
	homePublicItem(legacy)
	if legacy["platform"] != "steam" || legacy["poster_url"] != "https://cdn.akamai.steamstatic.com/steam/apps/730/library_600x900.jpg" || legacy["hero_url"] != "https://cdn.akamai.steamstatic.com/steam/apps/730/library_hero.jpg" {
		t.Fatal("legacy Steam automatic artwork changed", legacy)
	}
}

func TestHomePlatformSonyImagesUseOnlyConfirmedHTTPSHosts(t *testing.T) {
	for _, image := range []string{
		"https://image.api.playstation.com/actual/game.png",
		"https://image.api.np.km.playstation.net/actual/game.jpg?size=large",
	} {
		if !homeRemoteImage("psn", image) {
			t.Fatal("confirmed Sony artwork rejected", image)
		}
	}
	for _, image := range []string{
		"http://image.api.playstation.com/a.png", "https://image.api.playstation.com.evil.test/a.png",
		"https://evil.image.api.playstation.com/a.png", "https://image.api.playstation.com:443/a.png",
		"https://user:secret@image.api.playstation.com/a.png", "https://127.0.0.1/a.png",
		"https://image.api.playstation.com/a.png#fragment", "https://cdn.akamai.steamstatic.com/a.png",
		"https://image.api.playstation.com/a.png?" + strings.Repeat("x", 513),
	} {
		if homeRemoteImage("psn", image) {
			t.Fatal("unapproved Sony image URL accepted", image)
		}
	}
}

func TestHomePlatformFiltersBindSignedCursorBeforeDatabaseAccess(t *testing.T) {
	for _, value := range []string{"", "all", "steam", "psn"} {
		got, err := homeGameFilter(url.Values{"platform": {value}})
		want := value
		if want == "" {
			want = "all"
		}
		if err != nil || got != want {
			t.Fatal("valid platform filter rejected", value, got, err)
		}
	}
	a := &App{Config: Config{EncryptionKey: bytes.Repeat([]byte{1}, 32)}}
	steamCursor := a.encodePage(homeGamePage{Scope: homeGamesScope("position", "steam", 2), ID: "1", Value: 0})
	for _, query := range []url.Values{
		{"platform": {"psn"}, "limit": {"2"}, "cursor": {steamCursor}},
		{"platform": {"all"}, "limit": {"2"}, "cursor": {steamCursor}},
		{"platform": {"steam"}, "limit": {"3"}, "cursor": {steamCursor}},
		{"platform": {"steam"}, "limit": {"2"}, "sort": {"title"}, "cursor": {steamCursor}},
		{"platform": {"steam", "psn"}}, {"platform": {"sony"}},
	} {
		// A nil pool is deliberate: mismatched filters/cursors must be rejected
		// before any transaction starts, without a DB or a SQL implementation mock.
		_, err := a.PublicGames(context.Background(), query)
		var api *APIError
		if !errors.As(err, &api) || api.Status != 400 {
			t.Fatal("invalid platform/cursor reached storage or lost its validation status", query, err)
		}
	}
}

func TestHomePlatformMetadataKeepsNullAndZeroDistinct(t *testing.T) {
	in := homePlatformFixture()
	in.Platforms = nil
	raw, err := json.Marshal(homeMetadata(in))
	if err != nil || !bytes.Contains(raw, []byte(`"playtime_minutes":null`)) || !bytes.Contains(raw, []byte(`"platforms":[]`)) || !bytes.Contains(raw, []byte(`"platform":"psn"`)) {
		t.Fatal("metadata invented statistics or emitted a null device list", string(raw), err)
	}
	zero := int64(0)
	in.PlaytimeMinutes = &zero
	raw, err = json.Marshal(homeMetadata(in))
	if err != nil || !bytes.Contains(raw, []byte(`"playtime_minutes":0`)) {
		t.Fatal("observed zero playtime became unknown", string(raw), err)
	}
}

// Construct the actual released schema 20 only inside testApp's disposable DB.
// No maintenance/business database or provider account is used as a fixture.
func homePlatformSchema20(t *testing.T, a *App) {
	t.Helper()
	ctx := context.Background()
	var database string
	if err := a.Pool.QueryRow(ctx, `SELECT current_database()`).Scan(&database); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(database, "painter_test_") {
		t.Fatal("legacy fixture requires testApp's isolated database")
	}
	if _, err := a.Pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	a.Pool.Reset()
	files, err := migrations.Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	version := func(name string) int {
		n, _ := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		return n
	}
	sort.Slice(files, func(i, j int) bool { return version(files[i].Name()) < version(files[j].Name()) })
	for _, file := range files {
		name := file.Name()
		n := version(name)
		if !strings.HasSuffix(name, ".sql") || n < 1 || n > 20 {
			continue
		}
		raw, err := migrations.Files.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.Pool.Exec(ctx, string(raw)); err != nil {
			t.Fatalf("released schema-20 fixture %s: %v", name, err)
		}
		if _, err := a.Pool.Exec(ctx, `INSERT INTO schema_migrations(version,name,checksum,applied_at_ms) VALUES($1,$2,$3,1)`, n, name, fmt.Sprintf("%x", sha256.Sum256(raw))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.Pool.Exec(ctx, `INSERT INTO site_profile(id,display_name,updated_at_ms) VALUES(1,'Migration fixture',1); INSERT INTO site_settings(id,updated_at_ms) VALUES(1,1)`); err != nil {
		t.Fatal(err)
	}
	a.Pool.Reset()
}

func TestHomePlatformMigration21PreservesReleasedRowsAndConnection(t *testing.T) {
	a := homeTestApp(t)
	homePlatformSchema20(t, a)
	ctx := context.Background()
	// Build released-schema metadata directly: current HTTP authentication and
	// upload handlers require the latest schema and are not a schema-20 fixture.
	var ownerID string
	if err := a.Pool.QueryRow(ctx, `INSERT INTO users(username,display_name,role,created_at_ms,updated_at_ms) VALUES('migration20-owner','Migration owner','owner',1,1) RETURNING id::text`).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	media := RandID()
	if _, err := a.Pool.Exec(ctx, `INSERT INTO media_assets(id,owner_user_id,storage_key,original_name,mime_type,byte_size,sha256,state,created_at_ms,updated_at_ms,ready_at_ms) VALUES($1,$2,$3,'saved-photo.png','image/png',31,$4,'ready',1,1,1)`, media, ownerID, "migration20/"+media+".png", strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	cipher, err := a.encrypt([]byte("isolated-old-steam-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Pool.Exec(ctx, `UPDATE homepage_settings SET version=7,steam_id='76561198000000000',steam_key_ciphertext=$1,steam_auto_sync=true,last_sync_at_ms=100,last_error='saved failure',next_sync_at_ms=200 WHERE id=1`, cipher); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Pool.Exec(ctx, `INSERT INTO homepage_items(kind,title,subtitle,alt,url,image_url,media_id,provider_id,source,local_override,position,visible,version,created_at_ms,updated_at_ms,hero_url,logo_url,playtime_minutes) VALUES
 ('steam','Shared title','owner subtitle','owner alt','/posts','https://cdn.akamai.steamstatic.com/730/header.jpg',NULL,'730','steam_sync',true,37,false,9,101,202,'https://cdn.akamai.steamstatic.com/730/hero.jpg','https://cdn.akamai.steamstatic.com/730/logo.png',0),
 ('steam','Unknown lifetime','','','','https://cdn.akamai.steamstatic.com/570/header.jpg',NULL,'570','manual',false,4,true,3,103,204,'','',NULL),
 ('music','Saved record','','','','https://is1-ssl.mzstatic.com/album.jpg',NULL,'1234','music_playlist',true,8,false,2,105,206,'','',NULL),
 ('photos','Saved photo','','photo alt','', '',$1,'','manual',false,3,true,4,107,208,'','',NULL)`, media); err != nil {
		t.Fatal(err)
	}
	var beforeItems, beforeSettings string
	if err = a.Pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(h) ORDER BY id)::text FROM homepage_items h`).Scan(&beforeItems); err != nil {
		t.Fatal(err)
	}
	if err = a.Pool.QueryRow(ctx, `SELECT to_jsonb(h)::text FROM homepage_settings h`).Scan(&beforeSettings); err != nil {
		t.Fatal(err)
	}
	if err = Migrate(ctx, a.Pool); err != nil {
		t.Fatal("actual released-schema20-to-current migration", err)
	}
	var afterItems, afterSettings string
	if err = a.Pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(h)-'platform'-'platforms' ORDER BY id)::text FROM homepage_items h`).Scan(&afterItems); err != nil || beforeItems != afterItems {
		t.Fatal("migration changed a saved field, time, visibility, order, artwork, NULL, or observed zero", err)
	}
	if err = a.Pool.QueryRow(ctx, `SELECT to_jsonb(h)::text FROM homepage_settings h`).Scan(&afterSettings); err != nil || beforeSettings != afterSettings {
		t.Fatal("migration changed the existing config, Steam secret, or sync state", err)
	}
	var labelled, other, emptyDevices, schema int
	if err = a.Pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE kind='steam' AND platform='steam'),count(*) FILTER(WHERE kind<>'steam' AND platform=''),count(*) FILTER(WHERE platforms='{}') FROM homepage_items`).Scan(&labelled, &other, &emptyDevices); err != nil || labelled != 2 || other != 2 || emptyDevices != 4 {
		t.Fatal("incorrect legacy platform backfill", labelled, other, emptyDevices, err)
	}
	latestSchema := 0
	migrationFiles, err := migrations.Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range migrationFiles {
		if !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}
		version, err := strconv.Atoi(strings.SplitN(file.Name(), "_", 2)[0])
		if err != nil {
			t.Fatal("invalid embedded migration filename", file.Name(), err)
		}
		if version > latestSchema {
			latestSchema = version
		}
	}
	if err = a.Pool.QueryRow(ctx, `SELECT max(version) FROM schema_migrations`).Scan(&schema); err != nil || schema != latestSchema {
		t.Fatal("migration ledger did not reach latest embedded schema", schema, latestSchema, err)
	}
	var connectionVersion, credentialVersion int64
	var auto, requested bool
	var secret []byte
	if err = a.Pool.QueryRow(ctx, `SELECT version,credential_version,auto_sync,sync_requested,token_ciphertext FROM homepage_game_connections WHERE provider='psn'`).Scan(&connectionVersion, &credentialVersion, &auto, &requested, &secret); err != nil || connectionVersion != 1 || credentialVersion != 1 || auto || requested || secret != nil {
		t.Fatal("new PSN connection was not disconnected by default", err)
	}
	for _, statement := range []string{
		`UPDATE homepage_items SET platform='psn' WHERE provider_id='730'`,
		`UPDATE homepage_items SET platforms=ARRAY['PS5'] WHERE provider_id='570'`,
		`UPDATE homepage_items SET media_id=NULL WHERE kind='photos'`,
		`UPDATE homepage_items SET image_url='' WHERE kind='music'`,
	} {
		if _, err := a.Pool.Exec(ctx, statement); err == nil {
			t.Fatal("migration removed a required identity/media/platform guard", statement)
		}
	}
}

func homePlatformPageItems(t *testing.T, a *App, cookie *http.Cookie, target, platform string, total int) []map[string]any {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("platform", platform)
	q.Set("limit", "2")
	items := []map[string]any{}
	seen := map[string]bool{}
	for range 10 {
		page := momentResponse(t, momentCall(t, a, cookie, "", "GET", u.Path+"?"+q.Encode(), nil, 0, ""), 200)
		if page["platform"] != platform || page["total"] != float64(total) {
			t.Fatal("platform filter and total disagree", platform, page)
		}
		for _, raw := range page["items"].([]any) {
			item := raw.(map[string]any)
			id := item["id"].(string)
			if seen[id] || (platform != "all" && item["platform"] != platform) {
				t.Fatal("platform paging duplicated or widened a result", platform, item)
			}
			seen[id] = true
			items = append(items, item)
		}
		next := page["next_cursor"].(string)
		if !page["has_more"].(bool) {
			if next != "" || len(items) != total {
				t.Fatal("platform paging ended with missing rows or a terminal cursor", len(items), total, next)
			}
			return items
		}
		if next == "" {
			t.Fatal("platform paging lacks its continuation")
		}
		q.Set("cursor", next)
	}
	t.Fatal("platform paging did not terminate")
	return nil
}

func TestHomePlatformManualCRUDKeepsLegacyEditsAndUploadedArt(t *testing.T) {
	a := homeTestApp(t)
	a.RegisterMedia()
	_, owner, csrf := testIdentity(t, a, "owner")
	in := homeTestInput("steam")
	in["platform"], in["platforms"], in["provider_id"] = "psn", []string{"PS4", "PS5"}, "CUSA_12345:00"
	in["title"], in["url"], in["image_url"] = "My saved game", "", ""
	item := homeTestItem(t, a, owner, csrf, in)
	id := item["id"].(string)
	if item["platform"] != "psn" || item["image_url"] != "" || item["playtime_minutes"] != nil {
		t.Fatal("manual PSN creation fabricated missing data", item)
	}
	media := momentImageUpload(t, a, owner, csrf, 32)
	in["media_id"], in["hero_url"], in["logo_url"] = media, "https://image.api.playstation.com/custom/hero.png", "https://image.api.playstation.com/custom/logo.png"
	in["visible"], in["position"] = false, 31
	item = momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, in, int64(item["version"].(float64)), RandID()), 200)
	delete(in, "platform")
	delete(in, "platforms")
	delete(in, "hero_url")
	delete(in, "logo_url")
	in["title"] = "Legacy client edit"
	item = momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, in, int64(item["version"].(float64)), RandID()), 200)
	if item["platform"] != "psn" || !reflect.DeepEqual(item["platforms"], []any{"PS4", "PS5"}) || item["media_id"] != media || item["image_url"] != "/media/"+media || item["hero_url"] != "https://image.api.playstation.com/custom/hero.png" || item["logo_url"] != "https://image.api.playstation.com/custom/logo.png" || item["visible"] != false || item["position"] != float64(31) {
		t.Fatal("legacy edit lost platform, device, visibility, order, or saved art", item)
	}
	momentImageRead(t, a, nil, media, 404)
	version := int64(item["version"].(float64))
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, in, version-1, RandID()), 409)
	in["platform"] = "steam"
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, in, version, RandID()), 400)
	delete(in, "platform")
	in["playtime_minutes"] = 0
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, in, version, RandID()), 400)
	delete(in, "playtime_minutes")
	in["platforms"] = []string{}
	item = momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, in, version, RandID()), 200)
	if item["platform"] != "psn" || len(item["platforms"].([]any)) != 0 {
		t.Fatal("explicit device reset changed the game identity", item)
	}
	momentResponse(t, momentCall(t, a, owner, "", "GET", "/api/v1/admin/home/items/"+id, nil, 0, ""), 200)
	deleted := momentCall(t, a, owner, csrf, "DELETE", "/api/v1/admin/home/items/"+id, nil, int64(item["version"].(float64)), RandID())
	if deleted.Code != http.StatusNoContent {
		t.Fatal("manual PSN delete failed", deleted.Code, deleted.Body.String())
	}
	momentResponse(t, momentCall(t, a, owner, "", "GET", "/api/v1/admin/home/items/"+id, nil, 0, ""), 404)
}

func TestHomePlatformMixedFilteringPaginationAndGlobalHomeCap(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	var calls atomic.Int32
	homeFake(a, func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, fmt.Errorf("public reads must stay local")
	})
	ids := []string{}
	for i, title := range []string{"Same", "Same", "Alpha", "Beta", "Same", "Omega", "Zulu", "Hidden"} {
		in := homeTestInput("steam")
		in["title"], in["provider_id"], in["position"], in["visible"] = title, strconv.Itoa(100+i), i/2, i != 7
		if i%2 == 1 {
			in["platform"], in["platforms"], in["image_url"], in["url"] = "psn", []string{"PS5"}, "", ""
		}
		item := homeTestItem(t, a, owner, csrf, in)
		ids = append(ids, item["id"].(string))
	}
	for i, minutes := range []any{nil, 0, 60, 60, 120, nil, 0, nil} {
		if _, err := a.Pool.Exec(context.Background(), `UPDATE homepage_items SET playtime_minutes=$1 WHERE id=$2`, minutes, ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	homeGameEnable(t, a, owner, csrf, 1, true, 24)
	for _, sample := range []struct {
		platform, order string
		want            []int
	}{
		{"all", "position", []int{0, 1, 2, 3, 4, 5, 6}},
		{"all", "playtime", []int{4, 2, 3, 1, 6, 0, 5}},
		{"all", "title", []int{2, 3, 5, 0, 1, 4, 6}},
		{"steam", "position", []int{0, 2, 4, 6}},
		{"steam", "playtime", []int{4, 2, 6, 0}},
		{"psn", "position", []int{1, 3, 5}},
		{"psn", "playtime", []int{3, 1, 5}},
	} {
		items := homePlatformPageItems(t, a, nil, "/api/v1/games?sort="+sample.order, sample.platform, len(sample.want))
		for i, index := range sample.want {
			if items[i]["id"] != ids[index] {
				t.Fatal("mixed game ordering lost stable ties or NULL/zero distinction", sample.platform, sample.order, i, items[i])
			}
			if items[i]["platform"] == "psn" && (items[i]["poster_url"] != "" || items[i]["hero_url"] != "" || items[i]["logo_url"] != "") {
				t.Fatal("numeric PSN ID produced fabricated Steam assets", items[i])
			}
		}
	}
	for _, sample := range []struct {
		platform string
		total    int
	}{{"all", 8}, {"steam", 4}, {"psn", 4}} {
		homePlatformPageItems(t, a, owner, "/api/v1/admin/home/items?kind=steam", sample.platform, sample.total)
	}
	for _, target := range []string{"/api/v1/games?platform=psn&limit=2", "/api/v1/admin/home/items?kind=steam&platform=psn&limit=2"} {
		page := momentResponse(t, momentCall(t, a, owner, "", "GET", target, nil, 0, ""), 200)
		cursor := url.QueryEscape(page["next_cursor"].(string))
		for _, replacement := range []string{"steam", "all"} {
			changed := strings.Replace(target, "platform=psn", "platform="+replacement, 1) + "&cursor=" + cursor
			momentResponse(t, momentCall(t, a, owner, "", "GET", changed, nil, 0, ""), 400)
		}
	}
	data, err := a.HomepageData(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wall := data["walls"].(map[string][]map[string]any)["steam"]
	if len(wall) != 5 || data["config"].(HomeConfig).Sections["steam"].Limit != 24 {
		t.Fatal("home did not apply one shared cap without rewriting the saved limit", data)
	}
	for i, item := range wall {
		if item["id"] != ids[i] {
			t.Fatal("platforms no longer share the owner's one ordered home wall", wall)
		}
	}
	if wall[0]["playtime_minutes"] != nil || wall[1]["playtime_minutes"] != int64(0) {
		t.Fatal("home snapshot conflates unknown lifetime and an observed zero", wall)
	}
	homeGameEnable(t, a, owner, csrf, 2, true, 0)
	data, err = a.HomepageData(context.Background())
	if err != nil || len(data["walls"].(map[string][]map[string]any)["steam"]) != 0 {
		t.Fatal("zero home limit failed to hide the mixed shelf", err)
	}
	homePlatformPageItems(t, a, nil, "/api/v1/games", "all", 7)
	homePlatformPageItems(t, a, nil, "/api/v1/games", "psn", 3)
	homeGameEnable(t, a, owner, csrf, 3, false, 5)
	momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/games?platform=psn", nil, 0, ""), 404)
	if calls.Load() != 0 {
		t.Fatal("public mixed game rendering fetched provider data", calls.Load())
	}
}

func TestHomePlatformLegacySteamUpsertPreservesPSNNeighborAndOverrides(t *testing.T) {
	a := homeTestApp(t)
	a.RegisterMedia()
	_, owner, csrf := testIdentity(t, a, "owner")
	homeSteamConfigure(t, a, owner, csrf, false)
	in := homeTestInput("steam")
	in["platform"], in["platforms"], in["provider_id"], in["title"] = "psn", []string{"PS5"}, "730", "Counter Game"
	in["image_url"], in["url"] = "", ""
	psn := homeTestItem(t, a, owner, csrf, in)
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			return homeHTTP(503, "optional artwork unavailable"), nil
		}
		return homeHTTP(200, homeGames), nil
	})
	homeSyncCall(t, a, owner, csrf, 2, 200)
	items := homePlatformPageItems(t, a, owner, "/api/v1/admin/home/items?kind=steam", "all", 3)
	var steam map[string]any
	for _, item := range items {
		if item["platform"] == "steam" && item["provider_id"] == "730" {
			steam = item
		}
	}
	if steam == nil || steam["id"] == psn["id"] || steam["title"] != psn["title"] || len(steam["platforms"].([]any)) != 0 {
		t.Fatal("legacy Steam INSERT failed to default platform or collided with PSN", steam, psn)
	}
	media := momentImageUpload(t, a, owner, csrf, 33)
	edit := homeTestInput("steam")
	edit["provider_id"], edit["title"], edit["position"], edit["visible"] = "730", "Owner title", 77, false
	edit["media_id"], edit["image_url"] = media, ""
	edit["hero_url"], edit["logo_url"] = "https://cdn.akamai.steamstatic.com/custom/hero.jpg", "https://cdn.akamai.steamstatic.com/custom/logo.png"
	saved := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+steam["id"].(string), edit, int64(steam["version"].(float64)), RandID()), 200)
	homeSyncCall(t, a, owner, csrf, 2, 200)
	updated := momentResponse(t, momentCall(t, a, owner, "", "GET", "/api/v1/admin/home/items/"+steam["id"].(string), nil, 0, ""), 200)
	for _, field := range []string{"platform", "platforms", "title", "visible", "position", "media_id", "image_url", "hero_url", "logo_url", "local_override"} {
		if !reflect.DeepEqual(updated[field], saved[field]) {
			t.Fatal("legacy Steam upsert lost saved manual override", field, updated[field], saved[field])
		}
	}
	if updated["local_override"] != true || updated["playtime_minutes"] != float64(1) || updated["version"].(float64) <= saved["version"].(float64) {
		t.Fatal("legacy Steam upsert did not refresh its own real statistics/version", updated)
	}
	neighbor := momentResponse(t, momentCall(t, a, owner, "", "GET", "/api/v1/admin/home/items/"+psn["id"].(string), nil, 0, ""), 200)
	if !reflect.DeepEqual(neighbor, psn) {
		t.Fatal("Steam sync changed the same-ID same-title PSN neighbor", neighbor, psn)
	}
	public := homePlatformPageItems(t, a, nil, "/api/v1/games", "psn", 1)
	if public[0]["id"] != psn["id"] || public[0]["poster_url"] != "" || public[0]["playtime_minutes"] != nil {
		t.Fatal("PSN neighbor acquired Steam artwork or synthetic time", public)
	}
}
