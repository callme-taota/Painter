package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func friendCall(t *testing.T, a *App, cookie *http.Cookie, csrf, method, path string, input any, version int64, key string) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	var err error
	if input != nil {
		raw, err = json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
	}
	return friendRawCall(a, cookie, csrf, method, path, string(raw), version, key)
}

func friendRawCall(a *App, cookie *http.Cookie, csrf, method, path, raw string, version int64, key string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", a.Config.PublicBaseURL)
	r.Header.Set("X-CSRF-Token", csrf)
	if version > 0 {
		r.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	r.ContentLength = -1
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}

func friendFixture(t *testing.T, a *App, cookie *http.Cookie, csrf, name string, order int, visible bool) map[string]any {
	t.Helper()
	w := friendCall(t, a, cookie, csrf, "POST", "/api/v1/studio/friends", friendInput{Name: name, URL: "https://example.com/" + strconv.Itoa(order), Description: name + "-description", SortOrder: order, Visible: visible}, 0, RandID())
	if w.Code != 201 {
		t.Fatal("create friend", w.Code, w.Body.String())
	}
	return jsonMap(t, w)
}

func TestFriendsOwnerPermissionsAndCSRF(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	item := friendFixture(t, a, owner, csrf, "owner-private-link", 0, false)
	id := item["id"].(string)
	_, author, authorCSRF := testIdentity(t, a, "author")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	for _, actor := range []struct {
		name   string
		cookie *http.Cookie
		csrf   string
		status int
	}{{"anonymous", nil, "", 401}, {"reader", reader, readerCSRF, 403}, {"author", author, authorCSRF, 403}} {
		for _, op := range []struct{ method, path string }{
			{"GET", "/api/v1/studio/friends"}, {"POST", "/api/v1/studio/friends"},
			{"PUT", "/api/v1/studio/friends/" + id}, {"DELETE", "/api/v1/studio/friends/" + id},
			{"GET", "/api/v1/studio/friend-settings"}, {"PUT", "/api/v1/studio/friend-settings"},
		} {
			// Even malformed bodies and missing versions must authorize first.
			w := friendRawCall(a, actor.cookie, actor.csrf, op.method, op.path, "invalid", 0, RandID())
			if w.Code != actor.status || strings.Contains(w.Body.String(), "owner-private-link") {
				t.Fatal(actor.name, op, w.Code, w.Body.String())
			}
		}
	}
	for _, op := range []struct{ method, path string }{
		{"POST", "/api/v1/studio/friends"}, {"PUT", "/api/v1/studio/friends/" + id},
		{"DELETE", "/api/v1/studio/friends/" + id}, {"PUT", "/api/v1/studio/friend-settings"},
	} {
		if w := friendRawCall(a, owner, "bad-csrf", op.method, op.path, "invalid", 1, RandID()); w.Code != 403 {
			t.Fatal("missing csrf accepted", op, w.Code)
		}
		r := httptest.NewRequest(op.method, op.path, strings.NewReader("invalid"))
		r.AddCookie(owner)
		r.Header.Set("Origin", "https://foreign.test")
		r.Header.Set("X-CSRF-Token", csrf)
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("foreign origin accepted", op, w.Code)
		}
	}
	var count, version int
	if err := a.Pool.QueryRow(context.Background(), "SELECT count(*),max(version) FROM friend_links").Scan(&count, &version); err != nil || count != 1 || version != 1 {
		t.Fatal("rejected operations changed data", count, version, err)
	}
}

func TestFriendsCRUDOrderingVisibilityCASAndAudit(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	empty, err := a.PublicFriends(context.Background())
	if err != nil || len(empty["items"].([]map[string]any)) != 0 || empty["settings"].(map[string]any)["title"] != "友链" {
		t.Fatal("default empty page", empty, err)
	}
	private := friendFixture(t, a, owner, csrf, "private-sentinel", -100, false)
	late := friendFixture(t, a, owner, csrf, "Late", 10, true)
	first := friendFixture(t, a, owner, csrf, "First", -2, true)
	tied := friendFixture(t, a, owner, csrf, "Tied", 10, true)
	if first["version"] != float64(1) || first["created_at"] == "" || first["updated_at"] == "" {
		t.Fatal("incomplete friend DTO", first)
	}
	public, err := a.PublicFriends(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	items := public["items"].([]map[string]any)
	if len(items) != 3 || items[0]["id"] != first["id"] || items[1]["id"] != late["id"] || items[2]["id"] != tied["id"] {
		t.Fatal("numeric order / stable tie / private filtering", items)
	}
	w := friendCall(t, a, owner, csrf, "GET", "/api/v1/studio/friends?visible=true", nil, 0, "")
	if w.Code != 200 || len(jsonMap(t, w)["items"].([]any)) != 4 || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("owner collection should contain all links", w.Code, w.Body.String())
	}
	id := private["id"].(string)
	input := friendInput{Name: "Now public", URL: "https://example.org/new", Description: "Updated atomically", AvatarURL: "https://example.org/avatar.png", SortOrder: -3, Visible: true}
	if w := friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friends/"+id, input, 0, ""); w.Code != 428 {
		t.Fatal("update without version accepted", w.Code)
	}
	w = friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friends/"+id, input, 1, "")
	if w.Code != 200 || jsonMap(t, w)["version"] != float64(2) || jsonMap(t, w)["url"] != input.URL {
		t.Fatal("update link", w.Code, w.Body.String())
	}
	if w := friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friends/"+id, input, 1, ""); w.Code != 409 || !strings.Contains(w.Body.String(), "VERSION_CONFLICT") {
		t.Fatal("stale update accepted", w.Code)
	}
	input.Visible = false
	if w := friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friends/"+id, input, 2, ""); w.Code != 200 {
		t.Fatal("hide link", w.Code, w.Body.String())
	}
	public, err = a.PublicFriends(context.Background())
	if err != nil || len(public["items"].([]map[string]any)) != 3 {
		t.Fatal("hidden link remained public", public, err)
	}
	if w := friendCall(t, a, owner, csrf, "DELETE", "/api/v1/studio/friends/"+id, nil, 0, ""); w.Code != 428 {
		t.Fatal("delete without version accepted", w.Code)
	}
	if w := friendCall(t, a, owner, csrf, "DELETE", "/api/v1/studio/friends/"+id, nil, 2, ""); w.Code != 409 {
		t.Fatal("stale delete accepted", w.Code)
	}
	if w := friendCall(t, a, owner, csrf, "DELETE", "/api/v1/studio/friends/"+id, nil, 3, ""); w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal("delete response", w.Code, w.Body.String())
	}
	for _, method := range []string{"PUT", "DELETE"} {
		if w := friendCall(t, a, owner, csrf, method, "/api/v1/studio/friends/"+id, input, 3, ""); w.Code != 404 {
			t.Fatal("deleted link mutable", method, w.Code)
		}
	}
	var creates, updates, deletes int
	if err := a.Pool.QueryRow(context.Background(), "SELECT count(*) FILTER(WHERE action='friend.create'),count(*) FILTER(WHERE action='friend.update'),count(*) FILTER(WHERE action='friend.delete') FROM audit_logs WHERE target_type='friend'").Scan(&creates, &updates, &deletes); err != nil || creates != 4 || updates != 2 || deletes != 1 {
		t.Fatal("audit transaction mismatch", creates, updates, deletes, err)
	}
}

func TestFriendsPageSettingsCASAndClose(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	friendFixture(t, a, owner, csrf, "visible-before-close", 0, true)
	settings := jsonMap(t, friendCall(t, a, owner, csrf, "GET", "/api/v1/studio/friend-settings", nil, 0, ""))
	if settings["enabled"] != true || settings["title"] != "友链" || settings["description"] != "" || settings["version"] != float64(1) {
		t.Fatal("settings defaults", settings)
	}
	input := map[string]any{"enabled": false, "title": "网上的朋友", "description": "慢慢认识，常常来往。"}
	if w := friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friend-settings", input, 0, ""); w.Code != 428 {
		t.Fatal("settings without version accepted", w.Code)
	}
	w := friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friend-settings", input, 1, "")
	if w.Code != 200 || jsonMap(t, w)["version"] != float64(2) {
		t.Fatal("disable page", w.Code, w.Body.String())
	}
	public, err := a.PublicFriends(context.Background())
	var apiErr *APIError
	if public != nil || !errors.As(err, &apiErr) || apiErr.Status != 404 {
		t.Fatal("disabled page leaked public data", public, err)
	}
	admin := friendCall(t, a, owner, csrf, "GET", "/api/v1/studio/friends", nil, 0, "")
	if admin.Code != 200 || len(jsonMap(t, admin)["items"].([]any)) != 1 {
		t.Fatal("disabled page deleted admin data", admin.Code)
	}
	if w := friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friend-settings", input, 1, ""); w.Code != 409 {
		t.Fatal("stale settings accepted", w.Code)
	}
	input["enabled"] = true
	if w := friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friend-settings", input, 2, ""); w.Code != 200 {
		t.Fatal("enable page", w.Code, w.Body.String())
	}
	public, err = a.PublicFriends(context.Background())
	if err != nil || len(public["items"].([]map[string]any)) != 1 || public["settings"].(map[string]any)["title"] != input["title"] {
		t.Fatal("enabled page failed to restore existing links", public, err)
	}
	for _, invalid := range []map[string]any{
		{"title": "missing enabled"}, {"enabled": true, "title": " "}, {"enabled": true, "title": strings.Repeat("界", 81)},
		{"enabled": true, "title": "valid", "description": strings.Repeat("界", 401)},
		{"enabled": true, "title": "line\nbreak"}, {"enabled": true, "title": "valid", "version": 3},
	} {
		if w := friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friend-settings", invalid, 3, ""); w.Code != 400 {
			t.Fatal("invalid settings accepted", invalid, w.Code)
		}
	}
	var audits, version int
	if err := a.Pool.QueryRow(context.Background(), "SELECT (SELECT count(*) FROM audit_logs WHERE action='site.settings_update' AND target_id='friend-links'),version FROM friend_link_settings WHERE id=1").Scan(&audits, &version); err != nil || audits != 2 || version != 3 {
		t.Fatal("settings rejected changes / audits", audits, version, err)
	}
}

func TestFriendsValidationBoundsAndNeverFetchRemote(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("friend destination must never be fetched by server")
	}))
	defer remote.Close()
	valid := friendInput{Name: strings.Repeat("界", 79) + "🌿", URL: remote.URL + "/destination", Description: strings.Repeat("界", 240), AvatarURL: remote.URL + "/avatar", SortOrder: -100000, Visible: true}
	if w := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", valid, 0, RandID()); w.Code != 201 {
		t.Fatal("valid boundary / no remote fetch", w.Code, w.Body.String())
	}
	if _, err := a.PublicFriends(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, badURL := range []string{"javascript:alert(1)", "data:image/png;base64,eA==", "//example.com", "/relative", "https://user:secret@example.com", "https://user@example.com", "https://", "https://example.com:0", "https://example.com:65536", "https://example.com:bad", "https://example.com:", "https://[example.com]", "https://exa mple.com", "https://example.com/\n", "https://example.com/%0A", "https://example.com/%00", "https://example.com\\@evil.test", "https://-invalid.test", "https://example..com", "https://example.com/%xx"} {
		for _, field := range []string{"url", "avatar_url"} {
			input := map[string]any{"name": "valid", "url": "https://example.com", field: badURL}
			if w := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, RandID()); w.Code != 400 {
				t.Fatal("unsafe friend URL accepted", field, badURL, w.Code)
			}
		}
	}
	for _, input := range []map[string]any{
		{"name": " ", "url": "https://example.com"}, {"name": strings.Repeat("界", 81), "url": "https://example.com"},
		{"name": "valid", "url": "https://example.com", "description": strings.Repeat("界", 241)},
		{"name": "line\nbreak", "url": "https://example.com"}, {"name": "valid", "url": "https://example.com", "description": "\u2028"},
		{"name": "valid", "url": "https://example.com", "sort_order": 100001}, {"name": "valid", "url": "https://example.com", "sort_order": -100001},
		{"name": "valid", "url": "https://example.com", "sort_order": 1.5}, {"name": "valid", "url": "https://example.com", "visible": "true"},
		{"name": "valid", "url": "https://example.com", "version": 8}, {"name": "valid", "url": "https://example.com", "id": "99"},
	} {
		if w := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, RandID()); w.Code != 400 {
			t.Fatal("invalid friend accepted", input, w.Code, w.Body.String())
		}
	}
	for _, raw := range []string{`{"name":"valid","url":"https://example.com"}{}`, "{\"name\":\"bad" + string([]byte{0xff}) + "\",\"url\":\"https://example.com\"}"} {
		if w := friendRawCall(a, owner, csrf, "POST", "/api/v1/studio/friends", raw, 0, RandID()); w.Code != 400 {
			t.Fatal("invalid JSON accepted", w.Code)
		}
	}
	if w := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", map[string]any{"name": strings.Repeat("a", friendBodyLimit), "url": "https://example.com"}, 0, RandID()); w.Code != 413 {
		t.Fatal("unbounded body accepted", w.Code)
	}
	for _, raw := range []string{"https://example.com/a%20b?q=one+two#section", "HTTPS://example.com", "http://localhost:8080", "http://127.0.0.1", "https://[::1]:8080/", "https://例子.测试/文章"} {
		if _, err := friendURL(raw, true); err != nil {
			t.Fatal("valid URL rejected", raw, err)
		}
	}
}

func TestFriendsDescriptionsNormalizeNewlinesAndRejectOtherControls(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	raw := "  第一行\r\n第二行\r第三行\n第四行  "
	expected := "第一行\n第二行\n第三行\n第四行"
	input := friendInput{Name: "多行简介", URL: "https://example.com", Description: raw, Visible: true}
	w := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, RandID())
	if w.Code != 201 || jsonMap(t, w)["description"] != expected {
		t.Fatal("friend description newline normalization", w.Code, w.Body.String())
	}
	settings := map[string]any{"enabled": true, "title": "友链", "description": raw}
	w = friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friend-settings", settings, 1, "")
	if w.Code != 200 || jsonMap(t, w)["description"] != expected {
		t.Fatal("page description newline normalization", w.Code, w.Body.String())
	}
	public, err := a.PublicFriends(context.Background())
	if err != nil || public["settings"].(map[string]any)["description"] != expected || public["items"].([]map[string]any)[0]["description"] != expected {
		t.Fatal("normalized descriptions not preserved in public projection", public, err)
	}
	for _, control := range []string{"\t", "\x00", "\x1b", "\u2028", "\u2029"} {
		input.Description = "before" + control + "after"
		if w := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, RandID()); w.Code != 400 {
			t.Fatal("friend description accepted control", strconv.Quote(control), w.Code)
		}
		settings["description"] = input.Description
		if w := friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friend-settings", settings, 2, ""); w.Code != 400 {
			t.Fatal("page description accepted control", strconv.Quote(control), w.Code)
		}
	}
	input.Description = "allowed\nline"
	input.Name = "not\nallowed"
	if w := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, RandID()); w.Code != 400 {
		t.Fatal("name newline restriction changed", w.Code)
	}
	input.Name = "valid"
	input.URL = "https://example.com/\n"
	if w := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, RandID()); w.Code != 400 {
		t.Fatal("URL newline restriction changed", w.Code)
	}
}

func TestFriendsCreateIdempotencyConcurrentCASAndCollectionCap(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	input := friendInput{Name: "Idempotent", URL: "https://example.com", Visible: true}
	key := RandID()
	first := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, key)
	replay := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, key)
	if first.Code != 201 || replay.Code != 201 || jsonMap(t, first)["id"] != jsonMap(t, replay)["id"] {
		t.Fatal("idempotent create duplicated", first.Code, replay.Code)
	}
	input.Name = "Different"
	if w := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, key); w.Code != 409 {
		t.Fatal("idempotency key reused", w.Code)
	}
	if w := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, ""); w.Code != 400 {
		t.Fatal("create without idempotency accepted", w.Code)
	}
	id := jsonMap(t, first)["id"].(string)
	start := make(chan struct{})
	done := make(chan int, 2)
	for _, name := range []string{"Concurrent first", "Concurrent second"} {
		go func(name string) {
			<-start
			done <- friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friends/"+id, friendInput{Name: name, URL: "https://example.com/" + strings.ReplaceAll(name, " ", "-"), Visible: true}, 1, "").Code
		}(name)
	}
	close(start)
	statuses := map[int]int{}
	for range 2 {
		statuses[<-done]++
	}
	if statuses[200] != 1 || statuses[409] != 1 {
		t.Fatal("concurrent update lost CAS", statuses)
	}
	now := NowMillis()
	if _, err := a.Pool.Exec(context.Background(), "INSERT INTO friend_links(name,url,created_at_ms,updated_at_ms) SELECT 'Cap fixture '||n,'https://example.com/'||n,$1,$1 FROM generate_series(1,498) n", now); err != nil {
		t.Fatal(err)
	}
	start = make(chan struct{})
	for range 2 {
		go func() {
			<-start
			done <- friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, RandID()).Code
		}()
	}
	close(start)
	statuses = map[int]int{}
	for range 2 {
		statuses[<-done]++
	}
	if statuses[201] != 1 || statuses[409] != 1 {
		t.Fatal("concurrent creates exceeded cap", statuses)
	}
	w := friendCall(t, a, owner, csrf, "GET", "/api/v1/studio/friends", nil, 0, "")
	if w.Code != 200 || len(jsonMap(t, w)["items"].([]any)) != friendLimit {
		t.Fatal("owner list silently truncates below cap", w.Code)
	}
	// A completed create remains replayable even when the collection is full.
	input.Name = "Idempotent"
	if w := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, key); w.Code != 201 || jsonMap(t, w)["id"] != id {
		t.Fatal("full collection broke completed idempotency replay", w.Code)
	}
}
