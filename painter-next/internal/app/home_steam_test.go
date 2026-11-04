package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func homeSteamConfigure(t *testing.T, a *App, cookie *http.Cookie, csrf string, auto bool) {
	t.Helper()
	c, _ := homeTestConfig(t, a, cookie)
	c["sections"].(map[string]any)["steam"].(map[string]any)["enabled"] = true
	homeTestPut(t, a, cookie, csrf, c, map[string]any{"steam_id": "76561198000000000", "mode": "owned", "auto_sync_enabled": auto, "api_key": "private-steam-key"}, 1)
}
func homeSyncCall(t *testing.T, a *App, cookie *http.Cookie, csrf string, version int64, status int) map[string]any {
	t.Helper()
	return momentResponse(t, momentCall(t, a, cookie, csrf, "POST", "/api/v1/admin/home/steam-sync", nil, version, RandID()), status)
}

const homeGames = `{"response":{"game_count":2,"games":[{"appid":730,"name":"Counter Game","playtime_forever":1},{"appid":570,"name":"Another Game","playtime_forever":7}]}}`

func TestHomeSteamSnapshotManualOverridePrivacyAndReplay(t *testing.T) {
	a := homeTestApp(t)
	a.RegisterMedia()
	_, owner, csrf := testIdentity(t, a, "owner")
	homeSteamConfigure(t, a, owner, csrf, false)
	manual := homeTestItem(t, a, owner, csrf, homeTestInput("steam"))
	var calls atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			return homeHTTP(503, "optional artwork unavailable"), nil
		}

		calls.Add(1)
		if r.URL.Host != "api.steampowered.com" || !strings.Contains(r.URL.Path, "GetOwnedGames") || r.Header.Get("x-webapi-key") != "private-steam-key" || r.URL.Query().Get("key") != "" {
			t.Error("credential/endpoint contract", r.URL)
		}
		return homeHTTP(200, homeGames), nil
	})
	key := RandID()
	first := momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/steam-sync", nil, 2, key)
	result := momentResponse(t, first, 200)
	if result["synced_count"] != float64(2) || result["version"] != float64(2) || result["steam"].(map[string]any)["last_sync_at"] == nil || strings.Contains(first.Body.String(), "private-steam-key") {
		t.Fatal("sync result", result)
	}
	again := momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/steam-sync", nil, 2, key)
	if calls.Load() != 1 || !reflect.DeepEqual(result, momentResponse(t, again, 200)) {
		t.Fatal("sync replay fetched/changed")
	}
	items := momentItems(t, a, owner, "/api/v1/admin/home/items?kind=steam")
	if len(items) != 3 {
		t.Fatal("manual row removed", items)
	}
	var game map[string]any
	for _, v := range items {
		item := v.(map[string]any)
		if item["provider_id"] == "570" {
			game = item
		}
	}
	if game == nil {
		t.Fatal("synced game absent")
	}
	if game["position"] != float64(0) {
		t.Fatal("playtime order", game)
	}
	image := momentImageUpload(t, a, owner, csrf, 9)
	in := homeTestInput("steam")
	in["provider_id"] = "570"
	in["title"] = "My own game title"
	in["media_id"] = image
	in["image_url"] = ""
	in["position"] = 99
	in["visible"] = false
	edited := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+game["id"].(string), in, 1, RandID()), 200)
	if edited["version"] != float64(2) {
		t.Fatal("edit version")
	}
	homeSyncCall(t, a, owner, csrf, 2, 200)
	items = momentItems(t, a, owner, "/api/v1/admin/home/items?kind=steam")
	for _, v := range items {
		item := v.(map[string]any)
		if item["id"] == game["id"] && (item["title"] != "My own game title" || item["visible"] != false || item["position"] != float64(99) || item["image_url"] != "/media/"+image) {
			t.Fatal("sync overwrote local edit", item)
		}
	}
	momentImageRead(t, a, nil, image, 404)
	for _, body := range []string{`{"response":{}}`, `{"response":{"game_count":0,"games":[]}}`, `{"response":{"game_count":3,"games":[{"appid":730,"name":"partial","playtime_forever":1}]}}`} {
		homeFake(a, func(r *http.Request) (*http.Response, error) {
			if strings.Contains(r.URL.Path, "IStoreBrowseService") {
				return homeHTTP(503, "optional artwork unavailable"), nil
			}
			return homeHTTP(200, body), nil
		})
		status := 409
		if strings.Contains(body, "partial") {
			status = 502
		}
		homeSyncCall(t, a, owner, csrf, 2, status)
		if len(momentItems(t, a, owner, "/api/v1/admin/home/items?kind=steam")) != 3 {
			t.Fatal("empty/private/truncated erased snapshot")
		}
	}
	home := homeTestGet(t, a, owner)
	raw, _ := json.Marshal(home)
	if strings.Contains(string(raw), "private-steam-key") || home["steam"].(map[string]any)["last_error"] == "" {
		t.Fatal("error redaction/status", home)
	}
	public, e := a.HomepageData(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	raw, _ = json.Marshal(public)
	if strings.Contains(string(raw), "My own game title") || strings.Contains(string(raw), image) {
		t.Fatal("hidden game leaked", string(raw))
	}
	_ = manual
}
func TestHomeSteamLateConfigurationAndConcurrentLease(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	homeSteamConfigure(t, a, owner, csrf, false)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var calls atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			return homeHTTP(503, "optional artwork unavailable"), nil
		}

		calls.Add(1)
		once.Do(func() { close(entered) })
		select {
		case <-release:
			return homeHTTP(200, homeGames), nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/steam-sync", nil, 2, RandID())
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("fetch not started")
	}
	homeSyncCall(t, a, owner, csrf, 2, 409)
	if calls.Load() != 1 {
		t.Fatal("concurrent fetch bypassed lease")
	}
	c, _ := homeTestConfig(t, a, owner)
	homeTestPut(t, a, owner, csrf, c, map[string]any{"steam_id": "76561198000000001", "mode": "owned", "auto_sync_enabled": false}, 2)
	close(release)
	select {
	case w := <-done:
		momentResponse(t, w, 409)
	case <-time.After(3 * time.Second):
		t.Fatal("stale sync blocked")
	}
	if len(momentItems(t, a, owner, "/api/v1/admin/home/items?kind=steam")) != 0 {
		t.Fatal("old ID snapshot applied")
	}
	home := homeTestGet(t, a, owner)
	if home["steam"].(map[string]any)["last_sync_at"] != nil || home["steam"].(map[string]any)["last_error"] != "" {
		t.Fatal("stale request altered new config", home)
	}
}
func TestHomeSteamRoleRevocationBeforeApply(t *testing.T) {
	a := homeTestApp(t)
	p, owner, csrf := testIdentity(t, a, "owner")
	testIdentity(t, a, "owner") // Revocation must preserve another active owner.
	homeSteamConfigure(t, a, owner, csrf, false)
	entered, release := make(chan struct{}), make(chan struct{})
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			return homeHTTP(503, "optional artwork unavailable"), nil
		}

		close(entered)
		<-release
		return homeHTTP(200, homeGames), nil
	})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/steam-sync", nil, 2, RandID())
	}()
	<-entered
	if _, e := a.Pool.Exec(context.Background(), `UPDATE users SET role='author' WHERE id=$1`, p.ID); e != nil {
		t.Fatal(e)
	}
	close(release)
	select {
	case w := <-done:
		momentResponse(t, w, 403)
	case <-time.After(3 * time.Second):
		t.Fatal("revoked role did not finish")
	}
	var n int
	a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_items`).Scan(&n)
	if n != 0 {
		t.Fatal("revoked role wrote provider snapshot")
	}
}
func TestHomeSteamPeriodicLeaseDueAndBackoff(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	if e := a.SyncHomeSteam(context.Background()); e != nil {
		t.Fatal("default disabled worker should do nothing", e)
	}
	homeSteamConfigure(t, a, owner, csrf, true)
	var calls atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			return homeHTTP(503, "optional artwork unavailable"), nil
		}
		calls.Add(1)
		return homeHTTP(200, homeGames), nil
	})
	if e := a.SyncHomeSteam(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := a.SyncHomeSteam(context.Background()); e != nil {
		t.Fatal(e)
	}
	if calls.Load() != 1 {
		t.Fatal("periodic ignored six-hour due", calls.Load())
	}
	if _, e := a.Pool.Exec(context.Background(), `UPDATE homepage_settings SET next_sync_at_ms=0`); e != nil {
		t.Fatal(e)
	}
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			return homeHTTP(503, "optional artwork unavailable"), nil
		}

		calls.Add(1)
		return homeHTTP(503, "provider down"), nil
	})
	if e := a.SyncHomeSteam(context.Background()); e == nil {
		t.Fatal("expected upstream error")
	}
	if e := a.SyncHomeSteam(context.Background()); e != nil {
		t.Fatal("backoff should skip", e)
	}
	if calls.Load() != 2 {
		t.Fatal("backoff ignored")
	}
	var next int64
	var token *string
	var failures int
	if e := a.Pool.QueryRow(context.Background(), `SELECT next_sync_at_ms,lease_token,sync_failures FROM homepage_settings`).Scan(&next, &token, &failures); e != nil || next <= NowMillis() || token != nil || failures != 1 {
		t.Fatal("lease/backoff cleanup", next, token, failures, e)
	}
}
func TestHomeCollectionCapConcurrentCreate(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	if _, e := a.Pool.Exec(context.Background(), `INSERT INTO homepage_items(kind,title,image_url,created_at_ms,updated_at_ms) SELECT 'steam','seed','https://shared.akamai.steamstatic.com/header.jpg',0,0 FROM generate_series(1,4999)`); e != nil {
		t.Fatal(e)
	}
	done := make(chan int, 2)
	for range 2 {
		go func() {
			done <- momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/items", homeTestInput("steam"), 0, RandID()).Code
		}()
	}
	codes := []int{<-done, <-done}
	if !(codes[0] == 201 && codes[1] == 409 || codes[0] == 409 && codes[1] == 201) {
		t.Fatal("cap race", codes)
	}
	var n int
	a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_items`).Scan(&n)
	if n != 5000 {
		t.Fatal("cap exceeded", n)
	}
}

func TestHomeSteamInFlightPreservesConcurrentManualOverride(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	homeSteamConfigure(t, a, owner, csrf, false)
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			return homeHTTP(503, "optional artwork unavailable"), nil
		}
		return homeHTTP(200, homeGames), nil
	})
	homeSyncCall(t, a, owner, csrf, 2, 200)
	list := momentItems(t, a, owner, "/api/v1/admin/home/items?kind=steam")
	item := list[0].(map[string]any)
	if item["local_override"] != false {
		t.Fatal("new provider row marked overridden")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			return homeHTTP(503, "optional artwork unavailable"), nil
		}

		close(entered)
		<-release
		return homeHTTP(200, homeGames), nil
	})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/steam-sync", nil, 2, RandID())
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("fetch not started")
	}
	input := homeTestInput("steam")
	input["provider_id"] = item["provider_id"]
	input["title"] = "Concurrent owner title"
	input["subtitle"] = "Concurrent owner subtitle"
	input["url"] = "/posts"
	input["position"] = 42
	input["visible"] = false
	saved := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+item["id"].(string), input, 1, RandID()), 200)
	if saved["local_override"] != true {
		t.Fatal("override not acknowledged", saved)
	}
	close(release)
	select {
	case w := <-done:
		momentResponse(t, w, 200)
	case <-time.After(3 * time.Second):
		t.Fatal("sync did not finish")
	}
	list = momentItems(t, a, owner, "/api/v1/admin/home/items?kind=steam")
	for _, value := range list {
		current := value.(map[string]any)
		if current["id"] == item["id"] && (current["title"] != "Concurrent owner title" || current["subtitle"] != "Concurrent owner subtitle" || current["url"] != "/posts" || current["visible"] != false || current["position"] != float64(42) || current["local_override"] != true) {
			t.Fatal("in-flight sync clobbered saved local override", current)
		}
	}
	data, e := a.HomepageData(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, public := range data["walls"].(map[string][]map[string]any)["steam"] {
		if public["local_override"] != nil {
			t.Fatal("private override leaked")
		}
	}
}
