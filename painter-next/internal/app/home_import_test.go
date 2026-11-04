package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type homeRoundTripper func(*http.Request) (*http.Response, error)

func (f homeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func homeHTTP(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func homeFake(a *App, fn homeRoundTripper) { a.Config.HTTPClient = &http.Client{Transport: fn} }
func homeImportCall(t *testing.T, a *App, cookie *http.Cookie, csrf, kind, input string, status int) map[string]any {
	t.Helper()
	return momentResponse(t, momentCall(t, a, cookie, csrf, "POST", "/api/v1/admin/home/import", map[string]any{"kind": kind, "input": input}, 0, RandID()), status)
}
func TestHomeImportStrictParsingAndImageAllowlist(t *testing.T) {
	for _, input := range []struct{ kind, input, id, country string }{{"steam", "730", "730", "us"}, {"steam", "https://store.steampowered.com/app/730/CounterStrike_2/", "730", "us"}, {"music", "https://music.apple.com/us/album/example/123456", "123456", "us"}, {"music", "https://music.apple.com/cn/album/123456?i=111", "123456", "cn"}} {
		id, country, e := homeParseImport(input.kind, input.input)
		if e != nil || id != input.id || country != input.country {
			t.Fatal("valid import parse", input, id, country, e)
		}
	}
	for _, input := range []string{"http://music.apple.com/us/album/123", "https://music.apple.com.evil.test/us/album/123", "https://x:pass@music.apple.com/us/album/123", "https://music.apple.com:443/us/album/123", "https://127.0.0.1/us/album/123", "https://music.apple.com/us/playlist/123", "https://music.apple.com/us/album/0", "https://music.apple.com/us/album/123#x", "file:///etc/passwd", " https://music.apple.com/us/album/123"} {
		if _, _, e := homeParseImport("music", input); e == nil {
			t.Fatal("SSRF/invalid input accepted", input)
		}
	}
	for _, image := range []string{"http://is1-ssl.mzstatic.com/a.jpg", "https://is1-ssl.mzstatic.com.evil.test/a.jpg", "https://evil.test/a.jpg", "https://a:b@is1-ssl.mzstatic.com/a.jpg", "https://is1-ssl.mzstatic.com:443/a.jpg"} {
		if homeRemoteImage("music", image) {
			t.Fatal("unsafe artwork accepted", image)
		}
	}
	if !homeRemoteImage("music", "https://is1-ssl.mzstatic.com/image/thumb/foo/100x100bb.jpg") || !homeRemoteImage("steam", "https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/730/header.jpg") {
		t.Fatal("official image denied")
	}
}
func TestHomeImportAlbumFilterCacheExpiryAndNoAutosave(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	var calls atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Host != "itunes.apple.com" || r.URL.Path != "/lookup" || r.URL.Query().Get("id") != "123456" || r.URL.Query().Get("country") != "cn" || r.Header.Get("x-webapi-key") != "" {
			t.Error("unexpected provider request", r.URL)
		}
		return homeHTTP(200, `{"results":[{"wrapperType":"artist","artistName":"Do not select me"},{"wrapperType":"collection","collectionType":"Album","collectionId":999,"collectionName":"Wrong album"},{"wrapperType":"collection","collectionType":"Album","collectionId":123456,"collectionName":"正确唱片","artistName":"音乐人","collectionViewUrl":"https://music.apple.com/cn/album/a/123456","artworkUrl100":"https://is1-ssl.mzstatic.com/image/thumb/a/100x100bb.jpg"}]}`), nil
	})
	for range 2 {
		preview := homeImportCall(t, a, owner, csrf, "music", "https://music.apple.com/cn/album/a/123456", 200)
		if preview["title"] != "正确唱片" || preview["subtitle"] != "音乐人" || preview["image_url"] != "https://is1-ssl.mzstatic.com/image/thumb/a/100x100bb.jpg" {
			t.Fatal("album result/upsize fabricated", preview)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("cache miss", calls.Load())
	}
	var count int
	if e := a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_items`).Scan(&count); e != nil || count != 0 {
		t.Fatal("import auto-saved", count, e)
	}
	if _, e := a.Pool.Exec(context.Background(), `UPDATE homepage_import_cache SET expires_at_ms=0`); e != nil {
		t.Fatal(e)
	}
	homeImportCall(t, a, owner, csrf, "music", "https://music.apple.com/cn/album/a/123456", 200)
	if calls.Load() != 2 {
		t.Fatal("expired cache reused")
	}
	if _, e := a.HomepageData(context.Background()); e != nil {
		t.Fatal(e)
	}
	if calls.Load() != 2 {
		t.Fatal("visitor triggered provider HTTP")
	}
}
func TestHomeImportSteamRedirectBoundsRateAndErrorRedaction(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	var calls atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "IStoreBrowseService") {
			return homeHTTP(503, "optional artwork unavailable"), nil
		}
		calls.Add(1)
		id := r.URL.Query().Get("appids")
		if r.URL.Host != "store.steampowered.com" {
			t.Fatal("redirect followed", r.URL)
		}
		return homeHTTP(200, fmt.Sprintf(`{"%s":{"success":true,"data":{"steam_appid":%s,"name":"游戏","header_image":"https://shared.akamai.steamstatic.com/header.jpg"}}}`, id, id)), nil
	})
	preview := homeImportCall(t, a, owner, csrf, "steam", "730", 200)
	if preview["provider_id"] != "730" || preview["url"] != "https://store.steampowered.com/app/730/" {
		t.Fatal("steam preview", preview)
	}
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		response := homeHTTP(302, "")
		response.Header.Set("Location", "http://127.0.0.1/private")
		return response, nil
	})
	homeImportCall(t, a, owner, csrf, "steam", "731", 502)
	if calls.Load() != 2 {
		t.Fatal("redirect followed")
	}
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		return homeHTTP(200, strings.Repeat("x", (2<<20)+1)), nil
	})
	homeImportCall(t, a, owner, csrf, "steam", "732", 502)
	secret := "secret-should-not-reach-error"
	homeFake(a, func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("transport failed %s", secret) })
	// 3 provider attempts have consumed the bounded owner/provider window.
	for i := 0; i < 17; i++ {
		result := homeImportCall(t, a, owner, csrf, "steam", "733", 502)
		raw, _ := json.Marshal(result)
		if strings.Contains(string(raw), secret) {
			t.Fatal("provider error leaked", string(raw))
		}
	}
	homeImportCall(t, a, owner, csrf, "steam", "733", 429)
	var items, cache int
	a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_items`).Scan(&items)
	a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_import_cache`).Scan(&cache)
	if items != 0 || cache != 1 {
		t.Fatal("failed import persisted unsafe metadata", items, cache)
	}
}
