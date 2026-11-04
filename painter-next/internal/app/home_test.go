package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func homeTestApp(t *testing.T) *App {
	t.Helper()
	a := testApp(t)
	if _, pattern := a.Mux.Handler(httptest.NewRequest("GET", "/api/v1/admin/home", nil)); pattern == "" {
		a.RegisterHome()
	}
	return a
}
func homeTestGet(t *testing.T, a *App, cookie *http.Cookie) map[string]any {
	t.Helper()
	return momentResponse(t, momentCall(t, a, cookie, "", "GET", "/api/v1/admin/home", nil, 0, ""), 200)
}
func homeTestConfig(t *testing.T, a *App, cookie *http.Cookie) (map[string]any, map[string]any) {
	t.Helper()
	home := homeTestGet(t, a, cookie)
	return home["config"].(map[string]any), home["steam"].(map[string]any)
}
func homeTestPut(t *testing.T, a *App, cookie *http.Cookie, csrf string, config, steam map[string]any, version int64) map[string]any {
	t.Helper()
	return momentResponse(t, momentCall(t, a, cookie, csrf, "PUT", "/api/v1/admin/home", map[string]any{"config": config, "steam": steam}, version, RandID()), 200)
}
func homeTestInput(kind string) map[string]any {
	return map[string]any{"kind": kind, "title": "A title", "subtitle": "Subtitle", "alt": "Alternative", "url": "https://example.org/content", "image_url": "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/730/header.jpg", "media_id": "", "provider_id": "730", "position": 0, "visible": true}
}
func homeTestItem(t *testing.T, a *App, cookie *http.Cookie, csrf string, in map[string]any) map[string]any {
	t.Helper()
	return momentResponse(t, momentCall(t, a, cookie, csrf, "POST", "/api/v1/admin/home/items", in, 0, RandID()), 201)
}
func homeTestPhoto(t *testing.T, a *App, cookie *http.Cookie, csrf string, media string, visible bool) map[string]any {
	t.Helper()
	in := homeTestInput("photos")
	in["media_id"] = media
	in["image_url"] = ""
	in["provider_id"] = ""
	in["visible"] = visible
	return homeTestItem(t, a, cookie, csrf, in)
}

func TestHomeDefaultsFullValidationAndAuthorization(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	_, author, authorCSRF := testIdentity(t, a, "author")
	c, s := homeTestConfig(t, a, owner)
	if c["use_site_intro"] != true || c["motion_enabled"] != true || c["warmth_enabled"] != true || c["writing_title"] != "把想法，\n变成文字。" || !reflect.DeepEqual(c["order"], []any{"intro", "articles", "steam", "music", "photos", "closing", "activity", "projects"}) {
		t.Fatal("migration changed existing home defaults", c)
	}
	sections := c["sections"].(map[string]any)
	if sections["articles"].(map[string]any)["limit"] != float64(6) || sections["photos"].(map[string]any)["enabled"] != false || s["api_key_configured"] != false {
		t.Fatal("bad defaults", c, s)
	}
	for _, actor := range []struct {
		cookie *http.Cookie
		csrf   string
		status int
	}{{nil, "", 401}, {reader, readerCSRF, 403}, {author, authorCSRF, 403}} {
		for _, path := range []string{"/api/v1/admin/home", "/api/v1/admin/home/items?kind=steam"} {
			momentResponse(t, momentCall(t, a, actor.cookie, actor.csrf, "GET", path, nil, 0, ""), actor.status)
		}
		for _, path := range []string{"/api/v1/admin/home/items", "/api/v1/admin/home/import", "/api/v1/admin/home/steam-sync"} {
			momentResponse(t, momentCall(t, a, actor.cookie, actor.csrf, "POST", path, map[string]any{}, 1, RandID()), actor.status)
		}
	}
	var typed HomeConfig
	raw, _ := json.Marshal(c)
	if json.Unmarshal(raw, &typed) != nil || validateHome(typed) != nil {
		t.Fatal("defaults rejected")
	}
	for _, bad := range []func(*HomeConfig){func(c *HomeConfig) { c.Order[1] = "intro" }, func(c *HomeConfig) { delete(c.Sections, "music") }, func(c *HomeConfig) { v := c.Sections["articles"]; v.Limit = 13; c.Sections["articles"] = v }, func(c *HomeConfig) { v := c.Sections["photos"]; v.Limit = 25; c.Sections["photos"] = v }, func(c *HomeConfig) { c.IntroRoles = nil }, func(c *HomeConfig) { c.IntroName = strings.Repeat("界", 81) }, func(c *HomeConfig) { c.IntroRoles = []string{strings.Repeat("界", 61)} }, func(c *HomeConfig) {
		v := c.Sections["intro"]
		v.LinkURL = "javascript:alert(1)"
		c.Sections["intro"] = v
	}} {
		var candidate HomeConfig
		json.Unmarshal(raw, &candidate)
		bad(&candidate)
		if validateHome(candidate) == nil {
			t.Fatal("invalid full config accepted", candidate)
		}
	}
	for _, target := range []string{"javascript:alert(1)", "//evil.test/a", "/%2fexample.org", "https://a:b@example.org", "http://example.org/%0d%0a", "/bad\\path", "https://example.org/a b", "/#bad\n"} {
		if homeURL(target) == nil {
			t.Fatal("unsafe URL", target)
		}
	}
	for _, target := range []string{"", "/posts", "/#home-writing", "#a", "https://example.org/article?x=1", "http://192.168.1.2/posts"} {
		if homeURL(target) != nil {
			t.Fatal("safe destination rejected", target)
		}
	}
	momentResponse(t, momentCall(t, a, owner, "bad", "PUT", "/api/v1/admin/home", map[string]any{"config": c, "steam": s}, 1, RandID()), 403)
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home", map[string]any{"config": c, "steam": map[string]any{"steam_id": "", "mode": "owned", "auto_sync_enabled": false}}, 0, RandID()), 428)
	a.Config.UploadLimit = 2 << 20
	if current := homeTestGet(t, a, owner); current["max_image_bytes"] != float64(2<<20) {
		t.Fatal("private GET advertises incorrect configured upload limit", current["max_image_bytes"])
	}
	updated := homeTestPut(t, a, owner, csrf, c, map[string]any{"steam_id": "", "mode": "owned", "auto_sync_enabled": false}, 1)
	if updated["max_image_bytes"] != float64(2<<20) {
		t.Fatal("private PUT advertises incorrect configured upload limit", updated["max_image_bytes"])
	}
	public, e := a.HomepageData(context.Background())
	if e != nil || public["max_image_bytes"] != nil {
		t.Fatal("private upload limit entered public DTO", e, public)
	}
	a.Config.UploadLimit = 0
	if current := homeTestGet(t, a, owner); current["max_image_bytes"] != float64(10<<20) {
		t.Fatal("upload limit fallback is incorrect", current["max_image_bytes"])
	}
}
func TestHomeConfigCASIdempotencyEncryptedSecretsAndClear(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	c, _ := homeTestConfig(t, a, owner)
	secret := "private-steam-key-do-not-render"
	steam := map[string]any{"steam_id": "76561198000000000", "mode": "owned", "auto_sync_enabled": true, "api_key": secret}
	body := map[string]any{"config": c, "steam": steam}
	key := RandID()
	first := momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home", body, 1, key)
	result := momentResponse(t, first, 200)
	if result["version"] != float64(2) || strings.Contains(first.Body.String(), secret) || strings.Contains(first.Body.String(), "ciphertext") {
		t.Fatal("secret in response", first.Body.String())
	}
	again := momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home", body, 1, key)
	if !reflect.DeepEqual(result, momentResponse(t, again, 200)) {
		t.Fatal("configuration replay changed")
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home", body, 1, RandID()), 409)
	var cipher []byte
	if e := a.Pool.QueryRow(context.Background(), `SELECT steam_key_ciphertext FROM homepage_settings`).Scan(&cipher); e != nil || strings.Contains(string(cipher), secret) {
		t.Fatal("secret not encrypted", e)
	}
	plain, e := a.decrypt(cipher)
	if e != nil || string(plain) != secret {
		t.Fatal("encryption roundtrip", e)
	}
	delete(steam, "api_key")
	result = homeTestPut(t, a, owner, csrf, c, steam, 2)
	if result["steam"].(map[string]any)["api_key_configured"] != true {
		t.Fatal("omission cleared secret")
	}
	steam["api_key"] = ""
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home", map[string]any{"config": c, "steam": steam}, 3, RandID()), 400)
	delete(steam, "api_key")
	steam["clear_api_key"] = true
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home", map[string]any{"config": c, "steam": steam}, 3, RandID()), 400)
	steam["auto_sync_enabled"] = false
	result = homeTestPut(t, a, owner, csrf, c, steam, 3)
	if result["steam"].(map[string]any)["api_key_configured"] != false {
		t.Fatal("clear not applied")
	}
	public, e := a.HomepageData(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(public)
	if strings.Contains(string(raw), "steam_id") || strings.Contains(string(raw), "api_key") || strings.Contains(string(raw), secret) {
		t.Fatal("private settings entered public home", string(raw))
	}
}
func TestHomePhotoVisibilityOwnershipAndMediaProtection(t *testing.T) {
	a := homeTestApp(t)
	a.RegisterMedia()
	_, owner, csrf := testIdentity(t, a, "owner")
	_, author, authorCSRF := testIdentity(t, a, "author")
	first := momentImageUpload(t, a, owner, csrf, 1)
	second := momentImageUpload(t, a, owner, csrf, 2)
	foreign := momentImageUpload(t, a, author, authorCSRF, 3)
	in := homeTestInput("photos")
	in["media_id"] = foreign
	in["image_url"] = ""
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/items", in, 0, RandID()), 400)
	item := homeTestPhoto(t, a, owner, csrf, first, false)
	id := item["id"].(string)
	if item["image_url"] != "/media/"+first {
		t.Fatal("effective cover absent")
	}
	momentImageRead(t, a, nil, first, 404)
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", "/api/v1/media/"+first, nil, 0, RandID()), 409)
	c, s := homeTestConfig(t, a, owner)
	delete(s, "api_key_configured")
	delete(s, "last_sync_at")
	delete(s, "last_error")
	photos := c["sections"].(map[string]any)["photos"].(map[string]any)
	photos["enabled"] = true
	photos["limit"] = 1
	homeTestPut(t, a, owner, csrf, c, s, 1)
	momentImageRead(t, a, nil, first, 404)
	in["media_id"] = first
	in["provider_id"] = ""
	in["visible"] = true
	item = momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+id, in, 1, RandID()), 200)
	momentImageRead(t, a, nil, first, 200)
	homeTestPhoto(t, a, owner, csrf, second, true)
	momentImageRead(t, a, nil, second, 200) // visible is explicit public intent, even past display cap.
	data, e := a.HomepageData(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	wall := data["walls"].(map[string][]map[string]any)["photos"]
	if len(wall) != 1 || wall[0]["id"] != id || wall[0]["version"] != nil || wall[0]["visible"] != nil {
		t.Fatal("public cap/order/private fields", wall)
	}
	photos["limit"] = 0
	homeTestPut(t, a, owner, csrf, c, s, 2)
	momentImageRead(t, a, nil, first, 404)
	photos["limit"] = 1
	photos["enabled"] = false
	homeTestPut(t, a, owner, csrf, c, s, 3)
	momentImageRead(t, a, nil, second, 404)
	if _, e = a.Pool.Exec(context.Background(), `UPDATE media_assets SET created_at_ms=$1 WHERE id=$2`, NowMillis()-8*24*time.Hour.Milliseconds(), first); e != nil {
		t.Fatal(e)
	}
	if e = a.ReconcileMedia(context.Background()); e != nil {
		t.Fatal(e)
	}
	momentImageRead(t, a, owner, first, 200)
	if _, e = a.Pool.Exec(context.Background(), `UPDATE media_assets SET state='deleting' WHERE id=$1`, first); e == nil {
		t.Fatal("DB allowed referenced home media deletion")
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", "/api/v1/admin/home/items/"+id, nil, 1, RandID()), 409)
	if w := momentCall(t, a, owner, csrf, "DELETE", "/api/v1/admin/home/items/"+id, nil, 2, RandID()); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := momentCall(t, a, owner, csrf, "DELETE", "/api/v1/media/"+first, nil, 0, RandID()); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestHomeItemsPagingCASAndValidation(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	ids := []string{}
	for _, position := range []int{3, 0, 1} {
		in := homeTestInput("steam")
		in["position"] = position
		item := homeTestItem(t, a, owner, csrf, in)
		ids = append(ids, item["id"].(string))
	}
	path := "/api/v1/admin/home/items?kind=steam&limit=2"
	page := momentResponse(t, momentCall(t, a, owner, csrf, "GET", path, nil, 0, ""), 200)
	if !page["has_more"].(bool) || page["items"].([]any)[0].(map[string]any)["id"] != ids[1] {
		t.Fatal("position order/cursor", page)
	}
	next := page["next_cursor"].(string)
	page = momentResponse(t, momentCall(t, a, owner, csrf, "GET", path+"&cursor="+next, nil, 0, ""), 200)
	if len(page["items"].([]any)) != 1 || page["items"].([]any)[0].(map[string]any)["id"] != ids[0] {
		t.Fatal("cursor skipped", page)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "GET", "/api/v1/admin/home/items?kind=music&limit=2&cursor="+next, nil, 0, ""), 400)
	in := homeTestInput("steam")
	key := RandID()
	path = "/api/v1/admin/home/items/" + ids[0]
	one := momentCall(t, a, owner, csrf, "PUT", path, in, 1, key)
	momentResponse(t, one, 200)
	two := momentCall(t, a, owner, csrf, "PUT", path, in, 1, key)
	momentResponse(t, two, 200)
	if one.Body.String() != two.Body.String() {
		t.Fatal("item replay differs")
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, in, 1, RandID()), 409)
	current := momentResponse(t, momentCall(t, a, owner, csrf, "GET", path, nil, 0, ""), 200)
	if current["version"] != float64(2) || current["id"] != ids[0] {
		t.Fatal("single-item refresh did not return current CAS state", current)
	}
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	momentResponse(t, momentCall(t, a, reader, readerCSRF, "GET", path, nil, 0, ""), 403)
	for _, bad := range []struct {
		key   string
		value any
	}{{"image_url", "http://shared.akamai.steamstatic.com/header.jpg"}, {"image_url", "https://example.org/pixel"}, {"kind", "invalid"}, {"position", -1}, {"title", strings.Repeat("界", 81)}, {"url", "//example.org"}, {"provider_id", "abc"}} {
		candidate := homeTestInput("steam")
		candidate[bad.key] = bad.value
		momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/items", candidate, 0, RandID()), 400)
	}
	// Read DTO effective local-cover URLs are accepted on full-field update.
	a.RegisterMedia()
	media := momentImageUpload(t, a, owner, csrf, 7)
	photo := homeTestPhoto(t, a, owner, csrf, media, true)
	update := homeTestInput("photos")
	update["media_id"] = media
	update["image_url"] = "/media/" + media
	update["provider_id"] = ""
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+photo["id"].(string), update, 1, RandID()), 200)
}

func TestHomeAttachedCoverRetentionAcrossOwners(t *testing.T) {
	a := homeTestApp(t)
	a.RegisterMedia()
	_, owner1, csrf1 := testIdentity(t, a, "owner")
	_, owner2, csrf2 := testIdentity(t, a, "owner")
	media := momentImageUpload(t, a, owner1, csrf1, 1)
	item := homeTestPhoto(t, a, owner1, csrf1, media, false)
	in := homeTestInput("photos")
	in["media_id"] = media
	in["image_url"] = ""
	in["provider_id"] = ""
	in["alt"] = "Other owner retains already attached image"
	in["visible"] = false
	momentResponse(t, momentCall(t, a, owner2, csrf2, "PUT", "/api/v1/admin/home/items/"+item["id"].(string), in, 1, RandID()), 200)
	other := momentImageUpload(t, a, owner1, csrf1, 2)
	in["media_id"] = other
	momentResponse(t, momentCall(t, a, owner2, csrf2, "PUT", "/api/v1/admin/home/items/"+item["id"].(string), in, 2, RandID()), 400)
}
func TestHomeAttachWinsMediaDeleteRace(t *testing.T) {
	a := homeTestApp(t)
	a.RegisterMedia()
	_, owner, csrf := testIdentity(t, a, "owner")
	media := momentImageUpload(t, a, owner, csrf, 4)
	ctx := context.Background()
	if _, e := a.Pool.Exec(ctx, `CREATE FUNCTION painter_test_pause_home_image() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(943219020);RETURN NEW;END $$; CREATE TRIGGER zz_test_home_pause BEFORE INSERT ON homepage_items FOR EACH ROW EXECUTE FUNCTION painter_test_pause_home_image()`); e != nil {
		t.Fatal(e)
	}
	gate, e := a.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer gate.Rollback(ctx)
	if _, e = gate.Exec(ctx, `SELECT pg_advisory_xact_lock(943219020)`); e != nil {
		t.Fatal(e)
	}
	in := homeTestInput("photos")
	in["media_id"] = media
	in["image_url"] = ""
	in["provider_id"] = ""
	created, deleted := make(chan *httptest.ResponseRecorder, 1), make(chan *httptest.ResponseRecorder, 1)
	go func() { created <- momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/items", in, 0, RandID()) }()
	momentImageWaitForLock(t, a, "INSERT INTO homepage_items")
	go func() { deleted <- momentCall(t, a, owner, csrf, "DELETE", "/api/v1/media/"+media, nil, 0, "") }()
	momentImageWaitForLock(t, a, "owner_user_id::text,state FROM media_assets")
	if e = gate.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case w := <-created:
		momentResponse(t, w, 201)
	case <-time.After(5 * time.Second):
		t.Fatal("attach stuck")
	}
	select {
	case w := <-deleted:
		momentResponse(t, w, 409)
	case <-time.After(5 * time.Second):
		t.Fatal("delete stuck")
	}
}
func TestHomeDeleteWinsAttachRollsBack(t *testing.T) {
	a := homeTestApp(t)
	a.RegisterMedia()
	_, owner, csrf := testIdentity(t, a, "owner")
	media := momentImageUpload(t, a, owner, csrf, 5)
	ctx := context.Background()
	gate, e := a.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer gate.Rollback(ctx)
	if _, e = gate.Exec(ctx, `SELECT id FROM media_assets WHERE id=$1 FOR UPDATE`, media); e != nil {
		t.Fatal(e)
	}
	in := homeTestInput("photos")
	in["media_id"] = media
	in["image_url"] = ""
	in["provider_id"] = ""
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/items", in, 0, RandID()) }()
	momentImageWaitForLock(t, a, "owner_user_id::text,state,mime_type FROM media_assets")
	if _, e = gate.Exec(ctx, `UPDATE media_assets SET state='deleting' WHERE id=$1`, media); e != nil {
		t.Fatal(e)
	}
	if e = gate.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	select {
	case w := <-done:
		momentResponse(t, w, 400)
	case <-time.After(5 * time.Second):
		t.Fatal("attach stuck")
	}
	var count int
	if e = a.Pool.QueryRow(ctx, `SELECT count(*) FROM homepage_items`).Scan(&count); e != nil || count != 0 {
		t.Fatal("failed attach partially persisted", count, e)
	}
}
