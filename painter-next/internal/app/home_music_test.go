package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const homeMusicRealID = "pl.u-KVXBkAWuZqb1BXV"
const homeMusicRealSource = "https://music.apple.com/us/playlist/%E9%80%83%E5%A1%94%E5%96%9C%E6%AC%A2%E7%9A%84%E9%9F%B3%E4%B9%90/" + homeMusicRealID

// Synthetic deduplication fixture using two album identities sampled from the
// actual user playlist. Full live-page evidence is a separate opt-in test.
func homeMusicFixture(country string, ids ...string) string {
	if len(ids) == 0 {
		ids = []string{"1872514327", "1872514327", "1789430305"}
	}
	tracks := make([]map[string]any, 0, len(ids))
	for index, id := range ids {
		albumURL := "https://music.apple.com/" + country + "/album/fade/" + id
		tracks = append(tracks, map[string]any{
			"title": "FADE", "artistName": "Alesso & Pendulum",
			"contentDescriptor": map[string]any{"kind": "song", "identifiers": map[string]any{"storeAdamID": fmt.Sprint(1872514329 + index)}, "url": albumURL + "?i=1872514329"},
			"artwork":           map[string]any{"dictionary": map[string]any{"width": 3000, "height": 3000, "url": "https://is1-ssl.mzstatic.com/image/thumb/Music221/v4/cf/52/4b/cf524bc5-03a7-d151-213e-72d10813452f/26UMGIM00733.rgb.jpg/{w}x{h}bb.{f}"}},
			"tertiaryLinks":     []any{map[string]any{"title": "FADE - Single", "segue": map[string]any{"destination": map[string]any{"contentDescriptor": map[string]any{"kind": "album", "identifiers": map[string]any{"storeAdamID": id}, "url": albumURL}}}}},
		})
	}
	canonical := "https://music.apple.com/" + country + "/playlist/test/" + homeMusicRealID
	value := map[string]any{"data": []any{map[string]any{
		"intent": map[string]any{"$kind": "PlaylistDetailPageIntent", "contentDescriptor": map[string]any{"kind": "playlist", "identifiers": map[string]any{"storeAdamID": homeMusicRealID}, "locale": map[string]any{"storefront": country}}},
		"data": map[string]any{"canonicalURL": canonical, "nextIntent": map[string]any{"$kind": "PlaylistPaginationIntent", "url": "/v1/catalog/" + country + "/playlists/" + homeMusicRealID + "/tracks?offset=300"}, "sections": []any{
			map[string]any{"id": "header", "itemKind": "containerDetailHeaderLockup", "items": []any{map[string]any{"title": "逃塔喜欢的音乐", "trackCount": len(tracks), "contentDescriptor": map[string]any{"kind": "playlist", "identifiers": map[string]any{"storeAdamID": homeMusicRealID}}}}},
			map[string]any{"id": "track-list - " + homeMusicRealID, "itemKind": "trackLockup", "items": tracks},
		}},
	}}}
	raw, _ := json.Marshal(value)
	return `<html><script id="serialized-server-data" type="application/json">` + string(raw) + `</script></html>`
}
func homeMusicTestApp(t *testing.T) *App {
	t.Helper()
	a := homeTestApp(t)
	if _, pattern := a.Mux.Handler(httptest.NewRequest("GET", "/api/v1/admin/home/music-playlist", nil)); pattern == "" {
		a.Handle("GET /api/v1/admin/home/music-playlist", a.homeMusicGet)
		a.Handle("POST /api/v1/admin/home/music-playlist/preview", a.homeMusicPreview)
		a.Handle("POST /api/v1/admin/home/music-playlist/import", a.homeMusicImport)
	}
	return a
}
func homeMusicPreviewCall(t *testing.T, a *App, cookie *http.Cookie, csrf string) map[string]any {
	t.Helper()
	return momentResponse(t, momentCall(t, a, cookie, csrf, "POST", "/api/v1/admin/home/music-playlist/preview", map[string]any{"input": homeMusicRealSource}, 0, RandID()), 200)
}
func homeMusicImportCall(t *testing.T, a *App, cookie *http.Cookie, csrf, preview string, version int64, key string, status int) map[string]any {
	t.Helper()
	return momentResponse(t, momentCall(t, a, cookie, csrf, "POST", "/api/v1/admin/home/music-playlist/import", map[string]any{"preview_id": preview}, version, key), status)
}
func homeMusicMock(a *App, calls *atomic.Int32) {
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Host != "music.apple.com" {
			return nil, fmt.Errorf("wrong host")
		}
		if strings.HasPrefix(r.URL.Path, "/us/") {
			response := homeHTTP(302, "")
			response.Header.Set("Location", "/cn")
			return response, nil
		}
		return homeHTTP(200, homeMusicFixture("cn")), nil
	})
}
func TestHomeMusicStrictURLsArtworkAndIdentity(t *testing.T) {
	id, country, canonical, e := homeMusicParseURL(homeMusicRealSource+"?l=en-US", "playlist")
	if e != nil || id != homeMusicRealID || country != "us" || canonical != homeMusicRealSource {
		t.Fatal("valid public playlist rejected", id, country, canonical, e)
	}
	for _, raw := range []string{"http://music.apple.com/us/playlist/" + id, "https://music.apple.com.evil.test/us/playlist/" + id, "https://music.apple.com:443/us/playlist/" + id, "https://u:p@music.apple.com/us/playlist/" + id, "https://music.apple.com/us/playlist/123", "https://music.apple.com/us/album/123", homeMusicRealSource + "#x", " " + homeMusicRealSource, "https://music.apple.com/us/playlist/../../" + id, "https://music.apple.com/us/playlist/test%2fextra/" + id, "https://music.apple.com/us/playlist/test%5cextra/" + id} {
		if _, _, _, e := homeMusicParseURL(raw, "playlist"); e == nil {
			t.Fatal("unsafe playlist URL accepted", raw)
		}
	}
	parsed, e := homeMusicParsePage([]byte(homeMusicFixture("cn")), homeMusicRealSource, "", id, "cn")
	if e != nil || parsed.TrackCount != 3 || parsed.AlbumCount != 2 || !parsed.Partial || parsed.Title != "逃塔喜欢的音乐" || parsed.Items[0].ProviderID != "1872514327" || !strings.HasSuffix(parsed.Items[0].ImageURL, "/632x632bb.jpg") || !homeMusicValidateSnapshot(parsed) {
		t.Fatal("public snapshot/dedupe/artwork", parsed, e)
	}
	for _, body := range []string{`<html>no serialized data</html>`, strings.ReplaceAll(homeMusicFixture("cn"), homeMusicRealID, "pl.other"), strings.Replace(homeMusicFixture("cn"), `"storefront":"cn"`, `"storefront":"us"`, 1), strings.Replace(homeMusicFixture("cn"), `"kind":"playlist"`, `"kind":"album"`, 1)} {
		if _, e := homeMusicParsePage([]byte(body), homeMusicRealSource, "", id, "cn"); e == nil {
			t.Fatal("unverified page identity accepted")
		}
	}
	var artwork homeMusicArtwork
	artwork.Dictionary.URL = "https://is1-ssl.mzstatic.com/image/thumb/actual/100x100bb.jpg"
	if value := homeMusicArtworkURL(artwork); value != artwork.Dictionary.URL {
		t.Fatal("literal artwork URL silently upscaled", value)
	}
	artwork.Dictionary.URL = "https://is1-ssl.mzstatic.com/a/{w}x{h}bb.{f}"
	artwork.Dictionary.Width, artwork.Dictionary.Height = 240, 300
	if value := homeMusicArtworkURL(artwork); value != "https://is1-ssl.mzstatic.com/a/240x240bb.jpg" {
		t.Fatal("artwork maximum ignored", value)
	}
	artwork.Dictionary.URL = "https://evil.test/a/{w}x{h}bb.{f}"
	if homeMusicArtworkURL(artwork) != "" {
		t.Fatal("off-platform artwork accepted")
	}
}
func TestHomeMusicRedirectBoundsAndReadCap(t *testing.T) {
	a := &App{Config: Config{Environment: "test"}}
	var calls atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Host != "music.apple.com" {
			t.Error("redirect escaped Apple")
		}
		response := homeHTTP(302, "")
		response.Header.Set("Location", "https://127.0.0.1/private")
		return response, nil
	})
	if _, e := a.homeMusicFetch(context.Background(), homeMusicRealSource); e == nil || calls.Load() != 2 {
		t.Fatal("unsafe redirect followed or missing bounded regional retry", calls.Load(), e)
	}
	for _, location := range []string{"https://music.apple.com/cn/playlist/test/pl.different", "https://music.apple.com/cn", "http://music.apple.com/cn/playlist/" + homeMusicRealID, "https://u:p@music.apple.com/cn/playlist/" + homeMusicRealID} {
		calls.Store(0)
		homeFake(a, func(r *http.Request) (*http.Response, error) {
			calls.Add(1)
			response := homeHTTP(301, "")
			response.Header.Set("Location", location)
			return response, nil
		})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, e := a.homeMusicPageGET(ctx, "https://music.apple.com/cn/playlist/"+homeMusicRealID, "playlist", homeMusicRealID, "cn")
		cancel()
		if e == nil || calls.Load() != 1 {
			t.Fatal("wrong-identity redirect accepted", location, calls.Load(), e)
		}
	}
	calls.Store(0)
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if !strings.Contains(r.URL.Path, "/test/") {
			response := homeHTTP(301, "")
			response.Header.Set("Location", "https://music.apple.com/cn/playlist/test/"+homeMusicRealID)
			return response, nil
		}
		return homeHTTP(200, homeMusicFixture("cn")), nil
	})
	if _, e := a.homeMusicFetch(context.Background(), "https://music.apple.com/cn/playlist/"+homeMusicRealID); e != nil || calls.Load() != 2 {
		t.Fatal("same-identity canonical slug redirect rejected", calls.Load(), e)
	}
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		return homeHTTP(200, strings.Repeat("x", homeMusicHTMLLimit+1)), nil
	})
	if _, e := a.homeMusicFetch(context.Background(), "https://music.apple.com/cn/playlist/test/"+homeMusicRealID); e == nil {
		t.Fatal("oversized page accepted")
	}
	ids := make([]string, 301)
	for i := range ids {
		ids[i] = fmt.Sprint(1000 + i)
	}
	if _, e := homeMusicParsePage([]byte(homeMusicFixture("cn", ids...)), homeMusicRealSource, "", homeMusicRealID, "cn"); e == nil {
		t.Fatal("more than 300 page tracks accepted")
	}
}
func TestHomeMusicPreviewCacheExplicitImportAndReplay(t *testing.T) {
	a := homeMusicTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	var calls atomic.Int32
	homeMusicMock(a, &calls)
	initial := momentResponse(t, momentCall(t, a, owner, csrf, "GET", "/api/v1/admin/home/music-playlist", nil, 0, ""), 200)
	if initial["version"] != float64(1) || initial["last_success_at"] != nil {
		t.Fatal(initial)
	}
	preview := homeMusicPreviewCall(t, a, owner, csrf)
	if preview["track_count"] != float64(3) || preview["album_count"] != float64(2) || preview["partial"] != true || preview["source_url"] != homeMusicRealSource || calls.Load() != 2 {
		t.Fatal("bad preview", preview, calls.Load())
	}
	var total int
	if e := a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_items`).Scan(&total); e != nil || total != 0 {
		t.Fatal("preview auto-saved", total, e)
	}
	if public, e := a.PublicMusicPlaylist(context.Background()); e != nil || public != nil {
		t.Fatal("preview became public", public, e)
	}
	var expiry int64
	if e := a.Pool.QueryRow(context.Background(), `SELECT expires_at_ms FROM homepage_import_cache WHERE provider_id=$1`, "playlist:"+homeMusicRealID).Scan(&expiry); e != nil {
		t.Fatal(e)
	}
	preview = homeMusicPreviewCall(t, a, owner, csrf)
	var expiryAgain int64
	a.Pool.QueryRow(context.Background(), `SELECT expires_at_ms FROM homepage_import_cache WHERE provider_id=$1`, "playlist:"+homeMusicRealID).Scan(&expiryAgain)
	if calls.Load() != 2 || expiryAgain != expiry {
		t.Fatal("cache miss or sliding TTL", calls.Load(), expiry, expiryAgain)
	}
	key := RandID()
	result := homeMusicImportCall(t, a, owner, csrf, preview["preview_id"].(string), 1, key, 200)
	if result["version"] != float64(2) || result["new_count"] != float64(2) || result["imported_count"] != float64(2) || calls.Load() != 2 {
		t.Fatal("import", result, calls.Load())
	}
	replayed := homeMusicImportCall(t, a, owner, csrf, preview["preview_id"].(string), 1, key, 200)
	if !reflect.DeepEqual(result, replayed) {
		t.Fatal("not exact replay", result, replayed)
	}
	homeMusicImportCall(t, a, owner, csrf, RandID(), 1, key, 409)
	if e := a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_items WHERE source='music_playlist'`).Scan(&total); e != nil || total != 2 {
		t.Fatal("duplicate import", total, e)
	}
	public, e := a.PublicMusicPlaylist(context.Background())
	if e != nil || public["source_url"] != homeMusicRealSource || public["partial"] != true || public["version"] != nil || public["last_error"] != nil || public["last_attempt_source_url"] != nil {
		t.Fatal("unsafe/missing public projection", public, e)
	}
	if _, e = a.HomepageData(context.Background()); e != nil || calls.Load() != 2 {
		t.Fatal("visitor provider request", calls.Load(), e)
	}
}
func TestHomeMusicImportPreservesManualOverridesAndHiddenPositions(t *testing.T) {
	a := homeMusicTestApp(t)
	a.RegisterMedia()
	_, owner, csrf := testIdentity(t, a, "owner")
	var calls atomic.Int32
	homeMusicMock(a, &calls)
	manual := homeTestInput("music")
	manual["provider_id"] = "1789430305"
	manual["image_url"] = "https://is1-ssl.mzstatic.com/manual.jpg"
	manual["title"] = "手工保留"
	homeTestItem(t, a, owner, csrf, manual)
	preview := homeMusicPreviewCall(t, a, owner, csrf)
	result := homeMusicImportCall(t, a, owner, csrf, preview["preview_id"].(string), 1, RandID(), 200)
	if result["new_count"] != float64(1) || result["preserved_count"] != float64(1) {
		t.Fatal(result)
	}
	var id string
	var version int64
	if e := a.Pool.QueryRow(context.Background(), `SELECT id::text,version FROM homepage_items WHERE source='music_playlist'`).Scan(&id, &version); e != nil {
		t.Fatal(e)
	}
	change := homeTestInput("music")
	change["provider_id"] = "1872514327"
	change["image_url"] = "https://is1-ssl.mzstatic.com/custom.jpg"
	change["title"] = "我改的标题"
	change["visible"] = false
	change["position"] = 17
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, change, version, RandID()), 200)
	result = homeMusicImportCall(t, a, owner, csrf, preview["preview_id"].(string), 2, RandID(), 200)
	if result["new_count"] != float64(0) || result["preserved_count"] != float64(2) {
		t.Fatal(result)
	}
	var title, image string
	var visible, override bool
	var position int
	if e := a.Pool.QueryRow(context.Background(), `SELECT title,image_url,visible,position,local_override FROM homepage_items WHERE id=$1`, id).Scan(&title, &image, &visible, &position, &override); e != nil || title != "我改的标题" || image != "https://is1-ssl.mzstatic.com/custom.jpg" || visible || position != 17 || !override {
		t.Fatal(title, image, visible, position, override, e)
	}
	var total int
	a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_items`).Scan(&total)
	if total != 2 {
		t.Fatal("manual duplicated", total)
	}
	// Uploaded cover references survive another import and remain protected even
	// while this item is hidden. This is a real media upload, not a URL stand-in.
	mediaID := momentImageUpload(t, a, owner, csrf, 92)
	change["image_url"], change["media_id"] = "", mediaID
	change["alt"] = "我的封面说明"
	changed := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, change, version+1, RandID()), 200)
	homeMusicImportCall(t, a, owner, csrf, preview["preview_id"].(string), 3, RandID(), 200)
	var retained, alt string
	if e := a.Pool.QueryRow(context.Background(), `SELECT media_id,alt,visible,position,version FROM homepage_items WHERE id=$1`, id).Scan(&retained, &alt, &visible, &position, &version); e != nil || retained != mediaID || alt != "我的封面说明" || visible || position != 17 || float64(version) != changed["version"] {
		t.Fatal("upload/hidden/alt replaced", retained, alt, visible, position, version, e)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", "/api/v1/media/"+mediaID, nil, 0, RandID()), 409)
}

func TestHomeMusicAlbumEnhancementUsesVerifiedReturnedTemplate(t *testing.T) {
	a := &App{Config: Config{Environment: "test"}}
	item := homeItemInput{Kind: "music", ProviderID: "1872514327", URL: "https://music.apple.com/cn/album/fade/1872514327", ImageURL: "https://is1-ssl.mzstatic.com/image/thumb/Music221/actual/100x100bb.jpg"}
	art := map[string]any{"dictionary": map[string]any{"url": "https://is1-ssl.mzstatic.com/image/thumb/Music221/actual/{w}x{h}bb.{f}", "width": 3000, "height": 3000}}
	page := map[string]any{"data": []any{map[string]any{
		"intent": map[string]any{"$kind": "AlbumDetailPageIntent", "contentDescriptor": map[string]any{"kind": "album", "identifiers": map[string]any{"storeAdamID": "1872514327"}, "locale": map[string]any{"storefront": "cn"}}},
		"data":   map[string]any{"canonicalURL": item.URL, "sections": []any{map[string]any{"itemKind": "containerDetailHeaderLockup", "items": []any{map[string]any{"contentDescriptor": map[string]any{"kind": "album", "identifiers": map[string]any{"storeAdamID": "1872514327"}}, "artwork": art}}}}},
	}}}
	raw, _ := json.Marshal(page)
	body := `<script id="serialized-server-data" type="application/json">` + string(raw) + `</script>`
	homeFake(a, func(r *http.Request) (*http.Response, error) { return homeHTTP(200, body), nil })
	upgraded := a.homeMusicEnhanceAlbum(context.Background(), item)
	if upgraded.ImageURL != "https://is1-ssl.mzstatic.com/image/thumb/Music221/actual/632x632bb.jpg" || upgraded.URL != item.URL || upgraded.ProviderID != item.ProviderID {
		t.Fatal("album enhancement", upgraded)
	}
	for _, wrong := range []string{strings.ReplaceAll(body, "1872514327", "1872514328"), "<html>regional home</html>", strings.ReplaceAll(body, "is1-ssl.mzstatic.com", "evil.test")} {
		homeFake(a, func(r *http.Request) (*http.Response, error) { return homeHTTP(200, wrong), nil })
		if fallback := a.homeMusicEnhanceAlbum(context.Background(), item); fallback.ImageURL != item.ImageURL {
			t.Fatal("unverified art replaced fallback", fallback)
		}
	}
}

func TestHomeMusicAuthorizationOriginAndProviderQuota(t *testing.T) {
	a := homeMusicTestApp(t)
	actor, owner, csrf := testIdentity(t, a, "owner")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	for _, visitor := range []struct {
		cookie *http.Cookie
		status int
	}{{nil, 401}, {reader, 403}, {owner, 200}} {
		r := httptest.NewRequest("GET", "/api/v1/admin/home/music-playlist", nil)
		if visitor.cookie != nil {
			r.AddCookie(visitor.cookie)
		}
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		momentResponse(t, w, visitor.status)
	}
	for _, path := range []string{"/api/v1/admin/home/music-playlist/preview", "/api/v1/admin/home/music-playlist/import"} {
		momentResponse(t, momentCall(t, a, nil, "", "POST", path, map[string]any{}, 1, RandID()), 401)
		momentResponse(t, momentCall(t, a, reader, readerCSRF, "POST", path, map[string]any{}, 1, RandID()), 403)
		r := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
		r.AddCookie(owner)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "https://evil.test")
		r.Header.Set("X-CSRF-Token", csrf)
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		momentResponse(t, w, 403)
	}
	if _, e := a.Pool.Exec(context.Background(), `INSERT INTO homepage_provider_rates(owner_id,provider,window_at_ms,requests) VALUES($1,'music',$2,20)`, actor.ID, NowMillis()); e != nil {
		t.Fatal(e)
	}
	var calls atomic.Int32
	homeMusicMock(a, &calls)
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/music-playlist/preview", map[string]any{"input": homeMusicRealSource}, 0, RandID()), 429)
	if calls.Load() != 0 {
		t.Fatal("quota allowed provider request", calls.Load())
	}
}
func TestHomeMusicPreviewFailureRedactionAndAuthorization(t *testing.T) {
	a := homeMusicTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	var calls atomic.Int32
	homeMusicMock(a, &calls)
	preview := homeMusicPreviewCall(t, a, owner, csrf)
	result := homeMusicImportCall(t, a, owner, csrf, preview["preview_id"].(string), 1, RandID(), 200)
	key := RandID()
	for _, path := range []string{"/api/v1/admin/home/music-playlist/preview", "/api/v1/admin/home/music-playlist/import"} {
		momentResponse(t, momentCall(t, a, reader, readerCSRF, "POST", path, map[string]any{}, 1, key), 403)
	}
	homeFake(a, func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("private-upstream-secret") })
	failure := momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/music-playlist/preview", map[string]any{"input": homeMusicRealSource, "refresh": true}, 0, RandID()), 502)
	if strings.Contains(fmt.Sprint(failure), "private-upstream-secret") {
		t.Fatal("transport error leaked", failure)
	}
	status := momentResponse(t, momentCall(t, a, owner, csrf, "GET", "/api/v1/admin/home/music-playlist", nil, 0, ""), 200)
	if status["last_success_at"] != result["last_success_at"] || status["source_url"] != result["source_url"] || status["last_error"] == "" || strings.Contains(status["last_error"].(string), "private-upstream-secret") {
		t.Fatal("last success lost/error leaked", status)
	}
	var n int
	a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_items`).Scan(&n)
	if n != 2 {
		t.Fatal("failure changed items", n)
	}
	_, other, otherCSRF := testIdentity(t, a, "owner")
	homeMusicImportCall(t, a, other, otherCSRF, preview["preview_id"].(string), 2, RandID(), 409)
	if _, e := a.Pool.Exec(context.Background(), `UPDATE homepage_music_previews SET expires_at_ms=created_at_ms+1`); e != nil {
		t.Fatal(e)
	}
	homeMusicImportCall(t, a, owner, csrf, preview["preview_id"].(string), 2, RandID(), 409)
}
func TestHomeMusicPreviewDemotionDuringNetworkAndNoPoolReservation(t *testing.T) {
	a := homeMusicTestApp(t)
	actor, owner, csrf := testIdentity(t, a, "owner")
	testIdentity(t, a, "owner")
	config, e := pgxpool.ParseConfig(a.Config.DatabaseURL)
	if e != nil {
		t.Fatal(e)
	}
	config.MaxConns = 2
	small, e := pgxpool.NewWithConfig(context.Background(), config)
	if e != nil {
		t.Fatal(e)
	}
	previous := a.Pool
	a.Pool = small
	defer func() { a.Pool = previous; small.Close() }()
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			ctx, cancel := context.WithTimeout(r.Context(), time.Second)
			defer cancel()
			first, e := a.Pool.Acquire(ctx)
			if e != nil {
				return nil, e
			}
			defer first.Release()
			second, e := a.Pool.Acquire(ctx)
			if e != nil {
				return nil, e
			}
			second.Release()
			close(started)
			<-release
		}
		return homeHTTP(200, homeMusicFixture("us")), nil
	})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/music-playlist/preview", map[string]any{"input": homeMusicRealSource}, 0, RandID())
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("provider fetch held a transaction/pool reservation")
	}
	if _, e := a.Pool.Exec(context.Background(), `UPDATE users SET role='reader',version=version+1 WHERE id=$1`, actor.ID); e != nil {
		close(release)
		t.Fatal(e)
	}
	close(release)
	select {
	case response := <-done:
		momentResponse(t, response, 403)
	case <-time.After(3 * time.Second):
		t.Fatal("preview did not finish")
	}
	var previews, cache int
	a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_music_previews`).Scan(&previews)
	a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_import_cache`).Scan(&cache)
	if previews != 0 || cache != 0 {
		t.Fatal("demoted request saved snapshot", previews, cache)
	}
}
func TestHomeMusicImportCASCapacityAndRoleReplayAreAtomic(t *testing.T) {
	a := homeMusicTestApp(t)
	actor, owner, csrf := testIdentity(t, a, "owner")
	testIdentity(t, a, "owner")
	var calls atomic.Int32
	homeMusicMock(a, &calls)
	preview := homeMusicPreviewCall(t, a, owner, csrf)
	start := make(chan struct{})
	done := make(chan *httptest.ResponseRecorder, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			done <- momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/music-playlist/import", map[string]any{"preview_id": preview["preview_id"]}, 1, RandID())
		}()
	}
	close(start)
	statuses := map[int]int{}
	for i := 0; i < 2; i++ {
		select {
		case response := <-done:
			statuses[response.Code]++
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent import timed out")
		}
	}
	if statuses[200] != 1 || statuses[409] != 1 {
		t.Fatal("CAS lost", statuses)
	}
	key := RandID()
	homeMusicImportCall(t, a, owner, csrf, preview["preview_id"].(string), 2, key, 200)
	if _, e := a.Pool.Exec(context.Background(), `UPDATE users SET role='reader',version=version+1 WHERE id=$1`, actor.ID); e != nil {
		t.Fatal(e)
	}
	homeMusicImportCall(t, a, owner, csrf, preview["preview_id"].(string), 2, key, 403)
	_, second, secondCSRF := testIdentity(t, a, "owner")
	preview = homeMusicPreviewCall(t, a, second, secondCSRF)
	if _, e := a.Pool.Exec(context.Background(), `DELETE FROM homepage_items`); e != nil {
		t.Fatal(e)
	}
	if _, e := a.Pool.Exec(context.Background(), `INSERT INTO homepage_items(kind,title,image_url,created_at_ms,updated_at_ms) SELECT 'music','capacity','https://is1-ssl.mzstatic.com/capacity.jpg',1,1 FROM generate_series(1,4999)`); e != nil {
		t.Fatal(e)
	}
	homeMusicImportCall(t, a, second, secondCSRF, preview["preview_id"].(string), 3, RandID(), 409)
	var n int
	a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM homepage_items`).Scan(&n)
	if n != 4999 {
		t.Fatal("capacity failure partially saved", n)
	}
	var version int64
	a.Pool.QueryRow(context.Background(), `SELECT version FROM homepage_music_playlist WHERE id=1`).Scan(&version)
	if version != 3 {
		t.Fatal("capacity failure changed source", version)
	}
}

func TestHomeMusicLiveUserPublicPlaylist(t *testing.T) {
	if os.Getenv("PAINTER_TEST_APPLE_LIVE") != "1" {
		t.Skip("explicit public Apple sampling only")
	}
	a := &App{Config: Config{Environment: "development"}}
	snapshot, e := a.homeMusicFetch(context.Background(), homeMusicRealSource)
	if e != nil || snapshot.PlaylistID != homeMusicRealID || !homeMusicValidateSnapshot(snapshot) {
		t.Fatal("actual user playlist unreadable", e)
	}
	t.Logf("public user playlist: title=%q tracks=%d albums=%d partial=%v resolved=%s first_album=%s artwork=%s", snapshot.Title, snapshot.TrackCount, snapshot.AlbumCount, snapshot.Partial, snapshot.ResolvedURL, snapshot.Items[0].URL, snapshot.Items[0].ImageURL)
}
