package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func momentTestApp(t *testing.T) *App {
	t.Helper()
	a := testApp(t)
	// Root wires the domain into New. Keep these tests usable both before and
	// after that integration without registering ServeMux patterns twice.
	if _, pattern := a.Mux.Handler(httptest.NewRequest("GET", "/api/v1/moments", nil)); pattern == "" {
		a.RegisterMoments()
	}
	return a
}

func momentRawRequest(a *App, cookie *http.Cookie, csrf, method, path, raw string, version int64, key string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(raw))
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

func momentCall(t *testing.T, a *App, cookie *http.Cookie, csrf, method, path string, body any, version int64, key string) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	return momentRawRequest(a, cookie, csrf, method, path, string(raw), version, key)
}

func momentResponse(t *testing.T, w *httptest.ResponseRecorder, status int) map[string]any {
	t.Helper()
	if w.Code != status {
		t.Fatalf("moment status=%d want=%d: %s", w.Code, status, w.Body.String())
	}
	return jsonMap(t, w)
}

func momentFixture(t *testing.T, a *App, cookie *http.Cookie, csrf, text string, publish bool) map[string]any {
	t.Helper()
	item := momentResponse(t, momentCall(t, a, cookie, csrf, "POST", "/api/v1/moments", map[string]any{"text": text}, 0, RandID()), 201)
	if publish {
		item = momentResponse(t, momentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/moments/"+item["id"].(string)+"/publish", nil, 1, RandID()), 200)
	}
	return item
}

func momentItems(t *testing.T, a *App, cookie *http.Cookie, path string) []any {
	t.Helper()
	return momentResponse(t, momentCall(t, a, cookie, "", "GET", path, nil, 0, ""), 200)["items"].([]any)
}

func TestMomentTextUnicodeBoundaries(t *testing.T) {
	for _, text := range []string{"界", strings.Repeat("界", 500), strings.Repeat("🌿", 500), strings.Repeat("e\u0301", 250), " \n\t保留 内部与首尾空白\r\n ", `<script>alert("literal text")</script>`, strings.Repeat(" ", 498) + "\n字"} {
		if err := validateMomentText(text); err != nil {
			t.Errorf("valid %d-codepoint moment rejected: %v", utf8.RuneCountInString(text), err)
		}
	}
	for _, text := range []string{"", "\n\r\t　 ", strings.Repeat("界", 501), strings.Repeat("🌿", 501), strings.Repeat("e\u0301", 251), strings.Repeat(" ", 500) + "字", "null\x00byte", "bell\a", "invalid" + string([]byte{0xff})} {
		if err := validateMomentText(text); err == nil {
			t.Errorf("invalid %d-codepoint moment accepted", utf8.RuneCountInString(text))
		}
	}
}

func TestMomentsRequestValidationAndPayloadBound(t *testing.T) {
	a := momentTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	for _, body := range []any{
		map[string]any{"text": ""}, map[string]any{"text": nil}, map[string]any{},
		map[string]any{"text": strings.Repeat("🌿", 501)},
		map[string]any{"text": "\n\r\t　 "}, map[string]any{"text": "bad\x00text"},
		map[string]any{"text": "valid", "published": true}, map[string]any{"text": "valid", "author_user_id": "1"},
		map[string]any{"text": "valid", "public_text": "leaked"}, map[string]any{"text": "valid", "html": "<script>"},
	} {
		if w := momentCall(t, a, owner, csrf, "POST", "/api/v1/moments", body, 0, RandID()); w.Code != 400 {
			t.Fatal("invalid body accepted", w.Code, w.Body.String())
		}
	}
	for _, raw := range []string{`{"text":"one"}{"text":"two"}`, "{\"text\":\"bad" + string([]byte{0xff}) + "\"}", `{"text":2}`, `[]`} {
		if w := momentRawRequest(a, owner, csrf, "POST", "/api/v1/moments", raw, 0, RandID()); w.Code != 400 {
			t.Fatal("invalid JSON accepted", w.Code, w.Body.String())
		}
	}
	if w := momentRawRequest(a, owner, csrf, "POST", "/api/v1/moments", strings.Repeat(" ", momentBodyLimit+1), 0, RandID()); w.Code != 413 {
		t.Fatal("chunked oversized body accepted", w.Code)
	}
	text := "　 \n保留 内 部 空格\n　"
	item := momentFixture(t, a, owner, csrf, text, false)
	if item["text"] != text || item["max_text_length"] != float64(MomentTextLimit) {
		t.Fatal("text was trimmed/count rule omitted", item)
	}
	limit := momentFixture(t, a, owner, csrf, strings.Repeat("🌿", 500), false)
	if utf8.RuneCountInString(limit["text"].(string)) != 500 {
		t.Fatal("emoji limit is bytes instead of codepoints")
	}
	path := "/api/v1/studio/moments/" + item["id"].(string) + "/publish"
	if w := momentCall(t, a, owner, csrf, "POST", path, map[string]any{"text": "cannot override draft on publish"}, 1, RandID()); w.Code != 400 {
		t.Fatal("publish accepted content injection", w.Code)
	}
}

func TestMomentsAuthorizationOriginAndCSRF(t *testing.T) {
	a := momentTestApp(t)
	_, owner, ownerCSRF := testIdentity(t, a, "owner")
	_, author, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	item := momentFixture(t, a, author, csrf, "private-owned-moment", false)
	id := item["id"].(string)
	base := "/api/v1/studio/moments/" + id
	for _, actor := range []struct {
		name, csrf string
		cookie     *http.Cookie
		status     int
	}{{"anonymous", "", nil, 401}, {"reader", readerCSRF, reader, 403}} {
		for _, op := range []struct{ method, path string }{{"GET", "/api/v1/me/moments"}, {"GET", base}, {"POST", "/api/v1/moments"}, {"PUT", base + "/draft"}, {"POST", base + "/publish"}, {"DELETE", base + "/publication"}, {"DELETE", base}} {
			w := momentCall(t, a, actor.cookie, actor.csrf, op.method, op.path, map[string]any{"text": "must not write"}, 1, RandID())
			if w.Code != actor.status || strings.Contains(w.Body.String(), "private-owned-moment") {
				t.Fatal(actor.name, op, w.Code, w.Body.String())
			}
		}
	}
	for _, op := range []struct{ method, suffix string }{{"GET", ""}, {"PUT", "/draft"}, {"POST", "/publish"}, {"DELETE", "/publication"}, {"DELETE", ""}} {
		var body any
		if op.method == "PUT" {
			body = map[string]any{"text": "foreign overwrite"}
		}
		w := momentCall(t, a, other, otherCSRF, op.method, base+op.suffix, body, 999, RandID())
		if w.Code != 404 || strings.Contains(w.Body.String(), "private-owned-moment") || strings.Contains(w.Body.String(), "VERSION_CONFLICT") {
			t.Fatal("foreign author received content/version", op, w.Code, w.Body.String())
		}
	}
	if w := momentCall(t, a, other, otherCSRF, "GET", "/api/v1/me/moments?scope=site", nil, 0, ""); w.Code != 403 {
		t.Fatal("author site scope accepted", w.Code)
	}
	if w := momentCall(t, a, author, "wrong", "PUT", base+"/draft", map[string]any{"text": "CSRF overwrite"}, 1, RandID()); w.Code != 403 {
		t.Fatal("CSRF accepted", w.Code)
	}
	r := httptest.NewRequest("POST", "/api/v1/moments", strings.NewReader(`{"text":"wrong origin"}`))
	r.AddCookie(author)
	r.Header.Set("Origin", "https://attacker.example")
	r.Header.Set("X-CSRF-Token", csrf)
	r.Header.Set("Idempotency-Key", RandID())
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "ORIGIN_REJECTED") {
		t.Fatal("foreign origin accepted", w.Code)
	}
	if w := momentCall(t, a, author, csrf, "POST", "/api/v1/moments", map[string]any{"text": "missing key"}, 0, ""); w.Code != 400 {
		t.Fatal("create without idempotency accepted", w.Code)
	}
	if w := momentCall(t, a, author, csrf, "PUT", base+"/draft", map[string]any{"text": "missing version"}, 0, RandID()); w.Code != 428 {
		t.Fatal("missing version accepted", w.Code)
	}
	ownerEdit := momentResponse(t, momentCall(t, a, owner, ownerCSRF, "PUT", base+"/draft", map[string]any{"text": "owner-managed"}, 1, RandID()), 200)
	if ownerEdit["text"] != "owner-managed" || ownerEdit["version"] != float64(2) {
		t.Fatal("owner could not manage another author")
	}
}

func TestMomentsDraftPublicationWithdrawalAndDelete(t *testing.T) {
	a := momentTestApp(t)
	p, author, csrf := testIdentity(t, a, "author")
	if _, err := a.Pool.Exec(context.Background(), "UPDATE users SET email='private-moment-email@example.com',phone='private-moment-phone',bio='private-moment-bio',avatar_url='https://private-legacy.example/avatar' WHERE id=$1", p.ID); err != nil {
		t.Fatal(err)
	}
	text := `<script>alert("just plain text")</script>` + "\n公开最初内容"
	item := momentFixture(t, a, author, csrf, text, false)
	id := item["id"].(string)
	base := "/api/v1/studio/moments/" + id
	public := "/api/v1/moments/" + id
	if item["published"] != false || item["public_text"] != nil || item["first_published_at"] != nil {
		t.Fatal("draft created public data", item)
	}
	if w := momentCall(t, a, author, csrf, "GET", public, nil, 0, ""); w.Code != 404 {
		t.Fatal("owner cookie turned public detail private", w.Code)
	}
	if len(momentItems(t, a, author, "/api/v1/moments?scope=site&status=draft")) != 0 {
		t.Fatal("public scope query leaked drafts")
	}
	key := RandID()
	published := momentResponse(t, momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 1, key), 200)
	first, latest := published["first_published_at"], published["published_at"]
	w := momentCall(t, a, nil, "", "GET", public, nil, 0, "")
	visible := momentResponse(t, w, 200)
	if visible["text"] != text || visible["first_published_at"] != first || visible["published_at"] != latest {
		t.Fatal("public snapshot missing", visible)
	}
	wantKeys := map[string]bool{"id": true, "text": true, "images": true, "author": true, "first_published_at": true, "published_at": true}
	for key := range visible {
		if !wantKeys[key] {
			t.Fatal("extra public field", key)
		}
	}
	if len(visible["author"].(map[string]any)) != 4 || strings.Contains(w.Body.String(), "private-moment-") || strings.Contains(w.Body.String(), "private-legacy") || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("public DTO leaked author data or became cacheable", w.Body.String())
	}
	privateText := "未发布的新草稿-secret\n绝不可公开"
	saved := momentResponse(t, momentCall(t, a, author, csrf, "PUT", base+"/draft", map[string]any{"text": privateText}, 2, RandID()), 200)
	if saved["version"] != float64(3) || saved["public_text"] != text || saved["has_unpublished_changes"] != true {
		t.Fatal("saved draft broke old public snapshot", saved)
	}
	for _, path := range []string{public, "/api/v1/moments"} {
		w := momentCall(t, a, author, csrf, "GET", path, nil, 0, "")
		if w.Code != 200 || strings.Contains(w.Body.String(), "secret") {
			t.Fatal("unpublished edit leaked", path, w.Body.String())
		}
	}
	if got := momentResponse(t, momentCall(t, a, nil, "", "GET", public, nil, 0, ""), 200); got["published_at"] != latest {
		t.Fatal("saving draft changed public timestamp")
	}
	republished := momentResponse(t, momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 3, RandID()), 200)
	if republished["public_text"] != privateText || republished["first_published_at"] != first || republished["has_unpublished_changes"] != false {
		t.Fatal("explicit publish failed", republished)
	}
	withdrawn := momentResponse(t, momentCall(t, a, author, csrf, "DELETE", base+"/publication", nil, 4, RandID()), 200)
	if withdrawn["published"] != false || withdrawn["public_text"] != nil || withdrawn["first_published_at"] != first || withdrawn["published_at"] != republished["published_at"] {
		t.Fatal("withdraw timestamps/snapshot", withdrawn)
	}
	if got := momentCall(t, a, nil, "", "GET", public, nil, 0, ""); got.Code != 404 || len(momentItems(t, a, nil, "/api/v1/moments")) != 0 {
		t.Fatal("withdrawn content public", got.Code)
	}
	// A stale retry of the original publish is a ledger replay, not a new
	// publication. It must never resurrect an explicitly withdrawn snapshot.
	momentResponse(t, momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 1, key), 200)
	if got := momentCall(t, a, nil, "", "GET", public, nil, 0, ""); got.Code != 404 {
		t.Fatal("old publish retry resurrected withdrawn content", got.Code)
	}
	momentResponse(t, momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 5, RandID()), 200)
	delKey := RandID()
	deleted := momentResponse(t, momentCall(t, a, author, csrf, "DELETE", base, nil, 6, delKey), 200)
	if deleted["status"] != "deleted" || deleted["deleted_at"] == nil || deleted["version"] != float64(7) || deleted["public_text"] != nil {
		t.Fatal("delete not a soft state change", deleted)
	}
	momentResponse(t, momentCall(t, a, author, csrf, "DELETE", base, nil, 6, delKey), 200)
	if w := momentCall(t, a, nil, "", "GET", public, nil, 0, ""); w.Code != 404 || len(momentItems(t, a, nil, "/api/v1/moments")) != 0 {
		t.Fatal("deleted content public", w.Code)
	}
	for _, op := range []struct{ method, suffix string }{{"PUT", "/draft"}, {"POST", "/publish"}, {"DELETE", "/publication"}} {
		var body any
		if op.method == "PUT" {
			body = map[string]any{"text": "cannot resurrect"}
		}
		if w := momentCall(t, a, author, csrf, op.method, base+op.suffix, body, 7, RandID()); w.Code != 404 {
			t.Fatal("deleted moment was mutable", op, w.Code)
		}
	}
}

func TestMomentsIdempotencyAndConcurrentCAS(t *testing.T) {
	a := momentTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	body := map[string]any{"text": "exactly once"}
	key := RandID()
	first := momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/moments", body, 0, key), 201)
	replay := momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/moments", body, 0, key), 201)
	if !reflect.DeepEqual(first, replay) {
		t.Fatal("create retry changed result")
	}
	if w := momentCall(t, a, owner, csrf, "POST", "/api/v1/moments", map[string]any{"text": "different"}, 0, key); w.Code != 409 {
		t.Fatal("same key different body accepted", w.Code)
	}
	id := first["id"].(string)
	base := "/api/v1/studio/moments/" + id
	saveKey := RandID()
	save := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", base+"/draft", body, 1, saveKey), 200)
	saveReplay := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", base+"/draft", body, 1, saveKey), 200)
	if !reflect.DeepEqual(save, saveReplay) || save["version"] != float64(2) {
		t.Fatal("save retry advanced version")
	}
	if w := momentCall(t, a, owner, csrf, "PUT", base+"/draft", body, 2, saveKey); w.Code != 409 {
		t.Fatal("same save key different If-Match accepted", w.Code)
	}
	pubKey := RandID()
	pub := momentResponse(t, momentCall(t, a, owner, csrf, "POST", base+"/publish", nil, 2, pubKey), 200)
	pubReplay := momentResponse(t, momentCall(t, a, owner, csrf, "POST", base+"/publish", nil, 2, pubKey), 200)
	if !reflect.DeepEqual(pub, pubReplay) || pub["version"] != float64(3) {
		t.Fatal("publish retry advanced version")
	}
	start := make(chan struct{})
	done := make(chan *httptest.ResponseRecorder, 2)
	for _, value := range []string{"parallel-one", "parallel-two"} {
		go func(value string) {
			<-start
			done <- momentCall(t, a, owner, csrf, "PUT", base+"/draft", map[string]any{"text": value}, 3, RandID())
		}(value)
	}
	close(start)
	statuses := map[int]int{}
	for range 2 {
		statuses[(<-done).Code]++
	}
	if statuses[200] != 1 || statuses[409] != 1 {
		t.Fatal("CAS lost a write", statuses)
	}
	public := momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/moments/"+id, nil, 0, ""), 200)
	if public["text"] != "exactly once" {
		t.Fatal("parallel save altered public snapshot")
	}
	var count, creates, saves, publishes, version int
	if err := a.Pool.QueryRow(context.Background(), "SELECT (SELECT count(*) FROM moments),(SELECT count(*) FROM audit_logs WHERE action='moment.create'),(SELECT count(*) FROM audit_logs WHERE action='moment.save'),(SELECT count(*) FROM audit_logs WHERE action='moment.publish'),version FROM moments WHERE id=$1", id).Scan(&count, &creates, &saves, &publishes, &version); err != nil || count != 1 || creates != 1 || saves != 2 || publishes != 1 || version != 4 {
		t.Fatal("duplicates or partial transaction", count, creates, saves, publishes, version, err)
	}
}

func TestMomentsPublicPaginationOrderScopeAndIntegerIDs(t *testing.T) {
	a := momentTestApp(t)
	p, owner, csrf := testIdentity(t, a, "owner")
	other, _, _ := testIdentity(t, a, "author")
	ctx := context.Background()
	if _, err := a.Pool.Exec(ctx, `INSERT INTO moments(id,author_user_id,draft_text,public_text,first_published_at_ms,published_at_ms,created_at_ms,updated_at_ms) VALUES
 (2,$1,'private-draft-2','public-2',100,100,1,100),
 (9,$1,'private-draft-9','public-9',100,100,1,100),
 (10,$2,'private-draft-10','public-10',100,100,1,100),
 (9007199254740993,$1,'private-draft-large','public-large',200,200,1,200),
 (9007199254740994,$1,'highest-private',NULL,NULL,NULL,1,1)`, p.ID, other.ID); err != nil {
		t.Fatal(err)
	}
	page := momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/moments?limit=2", nil, 0, ""), 200)
	items := page["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["id"] != "9007199254740993" || items[1].(map[string]any)["id"] != "10" || page["has_more"] != true {
		t.Fatal("public order was lexicographic or leaked draft", page)
	}
	next := page["next_cursor"].(string)
	last := momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/moments?limit=2&cursor="+url.QueryEscape(next), nil, 0, ""), 200)
	items = last["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["id"] != "9" || items[1].(map[string]any)["id"] != "2" || last["has_more"] != false || last["next_cursor"] != "" {
		t.Fatal("keyset page lost rows", last)
	}
	for _, query := range []string{"limit=0", "limit=-1", "limit=no", "author_id=bad", "author_id=-1", "cursor=bad", "limit=3&cursor=" + next, "limit=2&author_id=" + p.ID + "&cursor=" + next} {
		if w := momentCall(t, a, nil, "", "GET", "/api/v1/moments?"+query, nil, 0, ""); w.Code != 400 {
			t.Fatal("invalid/unbound public pagination accepted", query, w.Code)
		}
	}
	if got := momentItems(t, a, nil, "/api/v1/moments?author_id="+other.ID); len(got) != 1 || got[0].(map[string]any)["id"] != "10" {
		t.Fatal("public author filter", got)
	}
	// Refreshing an existing publication preserves its first-publication order.
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/studio/moments/2/publish", nil, 1, RandID()), 200)
	if got := momentItems(t, a, nil, "/api/v1/moments"); got[len(got)-1].(map[string]any)["id"] != "2" {
		t.Fatal("republish jumped to the top")
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", "/api/v1/studio/moments/9007199254740993/publication", nil, 1, RandID()), 200)
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", "/api/v1/studio/moments/9", nil, 1, RandID()), 200)
	if got := momentItems(t, a, nil, "/api/v1/moments?scope=site&status=deleted"); len(got) != 2 {
		t.Fatal("withdraw/delete remained in public collection", got)
	}
	var count int
	if err := a.Pool.QueryRow(ctx, "SELECT count(*) FROM public_moments").Scan(&count); err != nil || count != 2 {
		t.Fatal("public view count includes private/deleted snapshots", count, err)
	}
	if _, err := a.Pool.Exec(ctx, "SELECT setval(pg_get_serial_sequence('moments','id'),(SELECT max(id) FROM moments),true)"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Pool.Exec(ctx, "INSERT INTO moments(author_user_id,draft_text,public_text,first_published_at_ms,published_at_ms,created_at_ms,updated_at_ms) SELECT $1,'public bulk '||n,'public bulk '||n,300,300,1,300 FROM generate_series(1,55) n", p.ID); err != nil {
		t.Fatal(err)
	}
	cap := momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/moments?limit=999", nil, 0, ""), 200)
	if len(cap["items"].([]any)) != 50 || cap["limit"] != float64(50) || cap["max_limit"] != float64(50) || cap["has_more"] != true {
		t.Fatal("public limit not capped", cap["limit"])
	}
}

func TestMomentsPrivateScopeAndStatusPagination(t *testing.T) {
	a := momentTestApp(t)
	_, owner, ownerCSRF := testIdentity(t, a, "owner")
	_, author, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	momentFixture(t, a, owner, ownerCSRF, "owner-only", false)
	draft := momentFixture(t, a, author, csrf, "author-draft", false)
	momentFixture(t, a, author, csrf, "author-public", true)
	del := momentFixture(t, a, author, csrf, "author-deleted", false)
	momentResponse(t, momentCall(t, a, author, csrf, "DELETE", "/api/v1/studio/moments/"+del["id"].(string), nil, 1, RandID()), 200)
	momentFixture(t, a, other, otherCSRF, "other-only", false)
	for _, c := range []struct {
		cookie *http.Cookie
		query  string
		count  int
	}{{owner, "", 1}, {owner, "?scope=site", 5}, {author, "", 3}, {author, "?status=draft", 1}, {author, "?status=published", 1}, {author, "?status=deleted", 1}, {other, "", 1}} {
		if got := momentItems(t, a, c.cookie, "/api/v1/me/moments"+c.query); len(got) != c.count {
			t.Fatal("private scope/status count", c.query, len(got), c.count)
		}
	}
	first := momentResponse(t, momentCall(t, a, author, csrf, "GET", "/api/v1/me/moments?limit=2", nil, 0, ""), 200)
	next := first["next_cursor"].(string)
	last := momentItems(t, a, author, "/api/v1/me/moments?limit=2&cursor="+url.QueryEscape(next))
	if len(last) != 1 || last[0].(map[string]any)["id"] != draft["id"] {
		t.Fatal("private cursor missing own oldest item", last)
	}
	for _, c := range []struct {
		cookie *http.Cookie
		path   string
	}{{other, "/api/v1/me/moments?limit=2&cursor=" + next}, {author, "/api/v1/me/moments?limit=2&status=draft&cursor=" + next}, {owner, "/api/v1/me/moments?limit=2&scope=site&cursor=" + next}, {nil, "/api/v1/moments?limit=2&cursor=" + next}, {author, "/api/v1/me/moments?scope=unknown"}, {owner, "/api/v1/me/moments?status=unknown"}} {
		if w := momentCall(t, a, c.cookie, "", "GET", c.path, nil, 0, ""); w.Code != 400 {
			t.Fatal("private cursor scope/status not bound", c.path, w.Code)
		}
	}
}

func TestMomentsRoleRevocationAndDisabledAuthorPublication(t *testing.T) {
	a := momentTestApp(t)
	testIdentity(t, a, "owner") // Required by the shared last-owner guard.
	p, author, csrf := testIdentity(t, a, "author")
	item := momentFixture(t, a, author, csrf, "public survives author disable", true)
	id := item["id"].(string)
	if _, err := a.Pool.Exec(context.Background(), "UPDATE users SET disabled_at_ms=$2 WHERE id=$1", p.ID, NowMillis()); err != nil {
		t.Fatal(err)
	}
	if w := momentCall(t, a, author, csrf, "PUT", "/api/v1/studio/moments/"+id+"/draft", map[string]any{"text": "disabled write"}, 2, RandID()); w.Code != 401 {
		t.Fatal("disabled account kept writing", w.Code)
	}
	if w := momentCall(t, a, nil, "", "GET", "/api/v1/moments/"+id, nil, 0, ""); w.Code != 200 {
		t.Fatal("disabled author policy differs from public_posts", w.Code)
	}
	if _, err := a.Pool.Exec(context.Background(), "UPDATE users SET disabled_at_ms=NULL,role='author' WHERE id=$1", p.ID); err != nil {
		t.Fatal(err)
	}
	// Re-authentication follows the same rule as the existing session service;
	// role changes must still reject replay, not just new callbacks.
	_, active, activeCSRF := testIdentity(t, a, "author")
	key := RandID()
	body := map[string]any{"text": "before role revoked"}
	momentResponse(t, momentCall(t, a, active, activeCSRF, "POST", "/api/v1/moments", body, 0, key), 201)
	var activeID string
	if err := a.Pool.QueryRow(context.Background(), "SELECT user_id::text FROM sessions WHERE token_hash=$1", hashString(active.Value)).Scan(&activeID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Pool.Exec(context.Background(), "UPDATE users SET role='reader' WHERE id=$1", activeID); err != nil {
		t.Fatal(err)
	}
	if w := momentCall(t, a, active, activeCSRF, "POST", "/api/v1/moments", body, 0, key); w.Code != 403 {
		t.Fatal("revoked author replay remained authorized", w.Code)
	}
}

func TestMomentsFormerOwnerCannotReplayForeignDraft(t *testing.T) {
	a := momentTestApp(t)
	testIdentity(t, a, "owner")
	p, owner, ownerCSRF := testIdentity(t, a, "owner")
	_, author, csrf := testIdentity(t, a, "author")
	item := momentFixture(t, a, author, csrf, "foreign-private-original", false)
	base := "/api/v1/studio/moments/" + item["id"].(string)
	body, key := map[string]any{"text": "foreign-private-managed"}, RandID()
	momentResponse(t, momentCall(t, a, owner, ownerCSRF, "PUT", base+"/draft", body, 1, key), 200)
	if _, err := a.Pool.Exec(context.Background(), "UPDATE users SET role='author' WHERE id=$1", p.ID); err != nil {
		t.Fatal(err)
	}
	w := momentCall(t, a, owner, ownerCSRF, "PUT", base+"/draft", body, 1, key)
	if w.Code != 404 || strings.Contains(w.Body.String(), "foreign-private-") {
		t.Fatal("former owner replayed foreign private DTO", w.Code, w.Body.String())
	}
}

func TestMomentsQueuedForeignReplayRechecksOwnerRole(t *testing.T) {
	a := momentTestApp(t)
	testIdentity(t, a, "owner")
	p, owner, ownerCSRF := testIdentity(t, a, "owner")
	_, author, csrf := testIdentity(t, a, "author")
	item := momentFixture(t, a, author, csrf, "queued-private-original", false)
	base := "/api/v1/studio/moments/" + item["id"].(string)
	body, key := map[string]any{"text": "queued-private-managed"}, RandID()
	momentResponse(t, momentCall(t, a, owner, ownerCSRF, "PUT", base+"/draft", body, 1, key), 200)
	ctx := context.Background()
	gate, err := a.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Rollback(ctx)
	if _, err = gate.Exec(ctx, "SELECT id FROM policy_barrier WHERE id=1 FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- momentCall(t, a, owner, ownerCSRF, "PUT", base+"/draft", body, 1, key)
	}()
	waiting := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err = a.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%policy_barrier%')").Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("foreign replay did not reach policy gate")
	}
	if _, err = gate.Exec(ctx, "UPDATE users SET role='author' WHERE id=$1", p.ID); err != nil {
		t.Fatal(err)
	}
	if err = gate.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case w := <-done:
		if w.Code != 403 || strings.Contains(w.Body.String(), "queued-private-") {
			t.Fatal("queued owner replay leaked after role change", w.Code, w.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued replay stuck after policy gate released")
	}
}

func TestMomentsMutationAuditFailureRollsBack(t *testing.T) {
	a := momentTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	ctx := context.Background()
	if _, err := a.Pool.Exec(ctx, `CREATE FUNCTION painter_test_reject_moment_audit() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN IF NEW.target_type='moment' THEN RAISE EXCEPTION 'test audit unavailable' USING ERRCODE='23514'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER painter_test_reject_moment_audit BEFORE INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION painter_test_reject_moment_audit()`); err != nil {
		t.Fatal(err)
	}
	if w := momentCall(t, a, owner, csrf, "POST", "/api/v1/moments", map[string]any{"text": "must rollback"}, 0, RandID()); w.Code == 201 {
		t.Fatal("write succeeded without audit")
	}
	var count, ledger int
	if err := a.Pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM moments),(SELECT count(*) FROM idempotency_keys)").Scan(&count, &ledger); err != nil || count != 0 || ledger != 0 {
		t.Fatal("partial content/response commit", count, ledger, err)
	}
}

func TestMomentsDatabasePublicationGuards(t *testing.T) {
	a := momentTestApp(t)
	p, _, _ := testIdentity(t, a, "owner")
	for _, statement := range []string{
		`INSERT INTO moments(author_user_id,draft_text,created_at_ms,updated_at_ms) VALUES($1,repeat('界',501),1,1)`,
		`INSERT INTO moments(author_user_id,draft_text,public_text,created_at_ms,updated_at_ms) VALUES($1,'draft','unsafe snapshot without publication dates',1,1)`,
		`INSERT INTO moments(author_user_id,draft_text,public_text,first_published_at_ms,published_at_ms,created_at_ms,updated_at_ms,deleted_at_ms) VALUES($1,'draft','deleted snapshot',1,1,1,1,1)`,
		`INSERT INTO moments(author_user_id,draft_text,first_published_at_ms,published_at_ms,created_at_ms,updated_at_ms) VALUES($1,'draft',5,4,1,1)`,
	} {
		if _, err := a.Pool.Exec(context.Background(), statement, p.ID); err == nil {
			t.Fatal("invalid database publication state accepted", statement)
		}
	}
	var runtimeRole bool
	if err := a.Pool.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='painter_app')").Scan(&runtimeRole); err != nil {
		t.Fatal(err)
	}
	if runtimeRole {
		for _, privilege := range []struct {
			name string
			want bool
		}{{"SELECT", true}, {"INSERT", true}, {"UPDATE", true}, {"DELETE", false}, {"TRUNCATE", false}} {
			var got bool
			if err := a.Pool.QueryRow(context.Background(), "SELECT has_table_privilege('painter_app','moments',$1)", privilege.name).Scan(&got); err != nil || got != privilege.want {
				t.Fatal("nonminimal runtime grant", privilege.name, got, err)
			}
		}
	}
	var applied int
	if err := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM schema_migrations WHERE version=15 AND name='0015_moments.sql'").Scan(&applied); err != nil || applied != 1 {
		t.Fatal(fmt.Sprintf("migration15 missing: %d %v", applied, err))
	}
	var id string
	if err := a.Pool.QueryRow(context.Background(), "INSERT INTO moments(author_user_id,draft_text,created_at_ms,updated_at_ms) VALUES($1,'immutable author',1,1) RETURNING id::text", p.ID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	other, _, _ := testIdentity(t, a, "author")
	if _, err := a.Pool.Exec(context.Background(), "UPDATE moments SET author_user_id=$2 WHERE id=$1", id, other.ID); err == nil {
		t.Fatal("moment author changed and bypassed replay role contract")
	}
}
