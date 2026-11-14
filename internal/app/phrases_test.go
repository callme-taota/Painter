package app

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func phraseRawRequest(a *App, cookie *http.Cookie, csrf, method, path, raw string, version int64, key string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", a.Config.PublicBaseURL)
	r.Header.Set("X-CSRF-Token", csrf)
	if version != 0 {
		r.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	// Cover bounded decoding without relying on a declared Content-Length.
	r.ContentLength = -1
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}

func phraseCall(t *testing.T, a *App, cookie *http.Cookie, csrf, method, path string, input any, version int64, key string) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	var err error
	if input != nil {
		raw, err = json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
	}
	return phraseRawRequest(a, cookie, csrf, method, path, string(raw), version, key)
}

func phraseCreateFixture(t *testing.T, a *App, cookie *http.Cookie, csrf, text, source string, published bool) map[string]any {
	t.Helper()
	w := phraseCall(t, a, cookie, csrf, "POST", "/api/v1/admin/phrases", map[string]any{"text": text, "kind": "sentence", "source": source, "published": published}, 0, RandID())
	if w.Code != 201 {
		t.Fatal("create phrase", w.Code, w.Body.String())
	}
	return jsonMap(t, w)
}

func TestPhrasesOwnerAuthorizationAndCSRF(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	item := phraseCreateFixture(t, a, owner, csrf, "private-owner-text", "private-source", false)
	id := item["id"].(string)
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	_, author, authorCSRF := testIdentity(t, a, "author")
	body := map[string]any{"text": "must-not-write", "kind": "word", "published": true}
	for _, actor := range []struct {
		name   string
		cookie *http.Cookie
		csrf   string
		status int
	}{{"anonymous", nil, "", 401}, {"reader", reader, readerCSRF, 403}, {"author", author, authorCSRF, 403}} {
		for _, operation := range []struct{ method, path string }{{"GET", "/api/v1/admin/phrases"}, {"POST", "/api/v1/admin/phrases"}, {"PUT", "/api/v1/admin/phrases/" + id}, {"DELETE", "/api/v1/admin/phrases/" + id}} {
			w := phraseCall(t, a, actor.cookie, actor.csrf, operation.method, operation.path, body, 1, RandID())
			if w.Code != actor.status || strings.Contains(w.Body.String(), "private-owner-text") || strings.Contains(w.Body.String(), "private-source") {
				t.Fatal(actor.name, operation.method, "authorization", w.Code, w.Body.String())
			}
		}
	}
	if w := phraseCall(t, a, owner, "bad-csrf", "POST", "/api/v1/admin/phrases", body, 0, RandID()); w.Code != 403 {
		t.Fatal("invalid csrf accepted", w.Code)
	}
	r := httptest.NewRequest("POST", "/api/v1/admin/phrases", strings.NewReader(`{"text":"foreign origin","kind":"word"}`))
	r.AddCookie(owner)
	r.Header.Set("Origin", "https://foreign.test")
	r.Header.Set("X-CSRF-Token", csrf)
	r.Header.Set("Idempotency-Key", RandID())
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("foreign origin accepted", w.Code)
	}
	var count, version int
	if err := a.Pool.QueryRow(context.Background(), "SELECT count(*),max(version) FROM phrases").Scan(&count, &version); err != nil || count != 1 || version != 1 {
		t.Fatal("rejected request changed phrase", count, version, err)
	}
}

func TestPhrasesPrivateVisibilityCASAndDelete(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	private := phraseCreateFixture(t, a, owner, csrf, "草稿里的月亮-private", "private-source", false)
	public := phraseCreateFixture(t, a, owner, csrf, "抬头看见星星🌌", "公开来源", true)
	privateID, publicID := private["id"].(string), public["id"].(string)
	if public["version"] != float64(1) || public["created_at"] == "" || public["updated_at"] == "" {
		t.Fatal("incomplete phrase DTO", public)
	}
	for _, cookie := range []*http.Cookie{nil, owner} {
		w := testRequest(a, "GET", "/api/v1/phrases?published=false&scope=admin", "", cookie, "")
		v := jsonMap(t, w)
		items := v["items"].([]any)
		if w.Code != 200 || len(items) != 1 || items[0].(map[string]any)["id"] != publicID || strings.Contains(w.Body.String(), "private") {
			t.Fatal("public collection leaked draft", w.Code, w.Body.String())
		}
	}
	shared, err := a.PublicPhrases(context.Background(), url.Values{"published": {"false"}})
	if err != nil || len(shared["items"].([]map[string]any)) != 1 {
		t.Fatal("SSR helper expanded private visibility", shared, err)
	}
	admin := jsonMap(t, testRequest(a, "GET", "/api/v1/admin/phrases", "", owner, ""))
	if len(admin["items"].([]any)) != 2 {
		t.Fatal("owner cannot manage draft")
	}
	update := map[string]any{"text": "草稿已公开", "kind": "word", "source": "公开来源", "published": true}
	w := phraseCall(t, a, owner, csrf, "PUT", "/api/v1/admin/phrases/"+privateID, update, 1, "")
	if w.Code != 200 || jsonMap(t, w)["version"] != float64(2) {
		t.Fatal("publish draft", w.Code, w.Body.String())
	}
	update["text"], update["source"] = "stale-secret", "stale-source"
	if w := phraseCall(t, a, owner, csrf, "PUT", "/api/v1/admin/phrases/"+privateID, update, 1, ""); w.Code != 409 || !strings.Contains(w.Body.String(), "VERSION_CONFLICT") {
		t.Fatal("stale update accepted", w.Code)
	}
	update["text"], update["source"], update["published"] = "重新收回的草稿-private", "private-source-new", false
	if w := phraseCall(t, a, owner, csrf, "PUT", "/api/v1/admin/phrases/"+privateID, update, 2, ""); w.Code != 200 {
		t.Fatal("unpublish", w.Code, w.Body.String())
	}
	if w := testRequest(a, "GET", "/api/v1/phrases", "", nil, ""); strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "stale") || len(jsonMap(t, w)["items"].([]any)) != 1 {
		t.Fatal("withdrawn content remained public", w.Body.String())
	}
	if w := phraseCall(t, a, owner, csrf, "DELETE", "/api/v1/admin/phrases/"+privateID, nil, 0, ""); w.Code != 428 {
		t.Fatal("delete without If-Match accepted", w.Code)
	}
	if w := phraseCall(t, a, owner, csrf, "DELETE", "/api/v1/admin/phrases/"+privateID, nil, 2, ""); w.Code != 409 {
		t.Fatal("stale delete accepted", w.Code)
	}
	if w := phraseCall(t, a, owner, csrf, "DELETE", "/api/v1/admin/phrases/"+publicID, nil, 1, ""); w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal("delete must return empty 204", w.Code, w.Body.String())
	}
	if w := testRequest(a, "GET", "/api/v1/phrases", "", nil, ""); len(jsonMap(t, w)["items"].([]any)) != 0 {
		t.Fatal("deleted phrase remained public", w.Body.String())
	}
	for _, method := range []string{"PUT", "DELETE"} {
		if w := phraseCall(t, a, owner, csrf, method, "/api/v1/admin/phrases/"+publicID, update, 1, ""); w.Code != 404 {
			t.Fatal("deleted phrase remained mutable", method, w.Code)
		}
	}
	var creates, updates, deletes int
	if err := a.Pool.QueryRow(context.Background(), "SELECT count(*) FILTER(WHERE action='phrase.create'),count(*) FILTER(WHERE action='phrase.update'),count(*) FILTER(WHERE action='phrase.delete') FROM audit_logs WHERE target_type='phrase'").Scan(&creates, &updates, &deletes); err != nil || creates != 2 || updates != 2 || deletes != 1 {
		t.Fatal("phrase audit vocabulary/transaction mismatch", creates, updates, deletes, err)
	}
}

func TestPhrasesUnicodeStrictJSONAndSizeLimits(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	boundary := map[string]any{"text": strings.Repeat("界", 279) + "🌿", "kind": "word", "source": strings.Repeat("🌿", 120)}
	w := phraseCall(t, a, owner, csrf, "POST", "/api/v1/admin/phrases", boundary, 0, RandID())
	if w.Code != 201 || jsonMap(t, w)["text"] != boundary["text"] || jsonMap(t, w)["published"] != false {
		t.Fatal("Unicode boundary/default draft", w.Code, w.Body.String())
	}
	for _, input := range []map[string]any{
		{"text": "  ", "kind": "sentence"},
		{"text": strings.Repeat("界", 281), "kind": "sentence"},
		{"text": "valid", "kind": "sentence", "source": strings.Repeat("🌿", 121)},
		{"text": "valid", "kind": "unsupported"},
		{"text": "valid", "kind": "word", "id": "99"},
		{"text": "valid", "kind": "word", "version": 99},
		{"text": "valid", "kind": "word", "published": "true"},
		{"text": "contains\x00nul", "kind": "word"},
		{"text": "contains\nnewline", "kind": "sentence"},
		{"text": "contains\u2028separator", "kind": "sentence"},
		{"text": "valid", "kind": "word", "source": "tab\tcontrol"},
	} {
		if w := phraseCall(t, a, owner, csrf, "POST", "/api/v1/admin/phrases", input, 0, RandID()); w.Code != 400 {
			t.Fatal("invalid phrase accepted", w.Code, w.Body.String())
		}
	}
	for _, raw := range []string{
		"{\"text\":\"invalid" + string([]byte{0xff}) + "\",\"kind\":\"word\"}",
		`{"text":"one","kind":"word"}{"text":"two","kind":"word"}`,
	} {
		if w := phraseRawRequest(a, owner, csrf, "POST", "/api/v1/admin/phrases", raw, 0, RandID()); w.Code != 400 {
			t.Fatal("invalid JSON accepted", w.Code, w.Body.String())
		}
	}
	if w := phraseCall(t, a, owner, csrf, "POST", "/api/v1/admin/phrases", map[string]any{"text": strings.Repeat("x", phraseBodyLimit), "kind": "word"}, 0, RandID()); w.Code != 413 {
		t.Fatal("unbounded phrase request", w.Code, w.Body.String())
	}
	trimmed := phraseCreateFixture(t, a, owner, csrf, "　 保留内 部文字 　", "　 来源 　", false)
	if trimmed["text"] != "保留内 部文字" || trimmed["source"] != "来源" {
		t.Fatal("unexpected whitespace normalization", trimmed)
	}
}

func TestPhrasesCreateIdempotencyAndConcurrentCAS(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	input := map[string]any{"text": "一次创建", "kind": "sentence", "published": true}
	key := RandID()
	first := phraseCall(t, a, owner, csrf, "POST", "/api/v1/admin/phrases", input, 0, key)
	replay := phraseCall(t, a, owner, csrf, "POST", "/api/v1/admin/phrases", input, 0, key)
	if first.Code != 201 || replay.Code != 201 || jsonMap(t, first)["id"] != jsonMap(t, replay)["id"] {
		t.Fatal("create replay changed identity", first.Code, replay.Code)
	}
	input["text"] = "another request"
	if got := phraseCall(t, a, owner, csrf, "POST", "/api/v1/admin/phrases", input, 0, key); got.Code != 409 {
		t.Fatal("idempotency key accepted a different request", got.Code)
	}
	if got := phraseCall(t, a, owner, csrf, "POST", "/api/v1/admin/phrases", input, 0, ""); got.Code != 400 {
		t.Fatal("create without idempotency key accepted", got.Code)
	}
	id := jsonMap(t, first)["id"].(string)
	start := make(chan struct{})
	done := make(chan *httptest.ResponseRecorder, 2)
	for _, text := range []string{"first-version", "second-version"} {
		go func(text string) {
			<-start
			done <- phraseCall(t, a, owner, csrf, "PUT", "/api/v1/admin/phrases/"+id, map[string]any{"text": text, "kind": "word", "source": text + "-source", "published": true}, 1, "")
		}(text)
	}
	close(start)
	statuses := map[int]int{}
	for range 2 {
		statuses[(<-done).Code]++
	}
	if statuses[200] != 1 || statuses[409] != 1 {
		t.Fatal("concurrent edit lost CAS", statuses)
	}
	var count, version int
	var text, source string
	if err := a.Pool.QueryRow(context.Background(), "SELECT count(*),max(version),min(text),min(source) FROM phrases").Scan(&count, &version, &text, &source); err != nil || count != 1 || version != 2 || source != text+"-source" {
		t.Fatal("partial/duplicated phrase write", count, version, err)
	}
}

func TestPhrasesPaginationCapsAndIntegerIDs(t *testing.T) {
	a := testApp(t)
	_, owner, _ := testIdentity(t, a, "owner")
	now := NowMillis()
	if _, err := a.Pool.Exec(context.Background(), "INSERT INTO phrases(text,kind,published,created_at_ms,updated_at_ms) SELECT 'public phrase '||n,'word',true,$1,$1 FROM generate_series(1,205) n", now); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Pool.Exec(context.Background(), "INSERT INTO phrases(id,text,kind,source,published,created_at_ms,updated_at_ms) VALUES(9007199254740993,'large integer identity','word','',true,$1,$1),(9007199254740994,'private-high-id','word','private-page-source',false,$1,$1)", now); err != nil {
		t.Fatal(err)
	}
	w := testRequest(a, "GET", "/api/v1/phrases?limit=999", "", nil, "")
	page := jsonMap(t, w)
	items := page["items"].([]any)
	if w.Code != 200 || len(items) != 200 || page["limit"] != float64(200) || page["max_limit"] != float64(200) || page["has_more"] != true || items[0].(map[string]any)["id"] != "9007199254740993" || strings.Contains(w.Body.String(), "private-") {
		t.Fatal("pagination bound/private/integer identity", w.Code, len(items), page["limit"], page["max_limit"], page["has_more"], items[0].(map[string]any)["id"], strings.Contains(w.Body.String(), "private-"))
	}
	next := page["next_cursor"].(string)
	w = testRequest(a, "GET", "/api/v1/phrases?cursor="+next, "", nil, "")
	last := jsonMap(t, w)
	if len(last["items"].([]any)) != 6 || last["has_more"] != false || last["next_cursor"] != nil {
		t.Fatal("cursor lost rows or continued beyond collection", w.Body.String())
	}
	admin := jsonMap(t, testRequest(a, "GET", "/api/v1/admin/phrases?limit=1", "", owner, ""))
	if admin["items"].([]any)[0].(map[string]any)["id"] != "9007199254740994" {
		t.Fatal("owner collection omitted draft")
	}
	for _, query := range []string{"limit=0", "limit=-1", "limit=abc", "cursor=0", "cursor=-1", "cursor=1%20OR%20true", "cursor=9223372036854775808"} {
		if got := testRequest(a, "GET", "/api/v1/phrases?"+query, "", nil, ""); got.Code != 400 {
			t.Fatal("invalid pagination accepted", query, got.Code)
		}
	}
}

func TestPhrasesPublicSSRVisibilityWithdrawalDeletionAndEscaping(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	a.RegisterWeb()
	empty := testRequest(a, "GET", "/phrases", "", nil, "")
	if empty.Code != 200 || !strings.Contains(empty.Body.String(), "还没有公开的片语") {
		t.Fatal("empty public phrase page", empty.Code)
	}
	phraseCreateFixture(t, a, owner, csrf, "SSR-private-draft", "SSR-private-source", false)
	text, source := `<script>alert("phrase")</script> SSR-visible-original-星图`, `<img src=x onerror="alert('source')">`
	public := phraseCreateFixture(t, a, owner, csrf, text, source, true)
	id := public["id"].(string)
	for _, cookie := range []*http.Cookie{nil, owner} {
		page := testRequest(a, "GET", "/phrases?published=false&scope=admin", "", cookie, "")
		body := page.Body.String()
		if page.Code != 200 || !strings.Contains(page.Header().Get("Content-Type"), "text/html") || !strings.Contains(page.Header().Get("Cache-Control"), "no-store") {
			t.Fatal("public SSR response", page.Code)
		}
		// The HTTP response already contains the reading text and source; no
		// JavaScript execution or client-side API request is needed to see them.
		if !strings.Contains(body, `data-phrase-text`) || !strings.Contains(body, html.EscapeString(text)) || !strings.Contains(body, html.EscapeString(source)) {
			t.Fatal("public phrase/source absent from no-JS HTML")
		}
		if strings.Contains(body, "SSR-private-") || strings.Contains(body, text) || strings.Contains(body, source) {
			t.Fatal("SSR leaked draft or trusted HTML from phrase input")
		}
	}
	private := map[string]any{"text": "SSR-withdrawn-private", "kind": "sentence", "source": "SSR-withdrawn-source", "published": false}
	if got := phraseCall(t, a, owner, csrf, "PUT", "/api/v1/admin/phrases/"+id, private, 1, ""); got.Code != 200 {
		t.Fatal("withdraw phrase", got.Code, got.Body.String())
	}
	withdrawn := testRequest(a, "GET", "/phrases", "", nil, "")
	if withdrawn.Code != 200 || strings.Contains(withdrawn.Body.String(), "SSR-withdrawn-") || strings.Contains(withdrawn.Body.String(), "SSR-visible-original") || !strings.Contains(withdrawn.Body.String(), "还没有公开的片语") {
		t.Fatal("withdrawn phrase remained in SSR")
	}
	private["text"], private["source"], private["published"] = "SSR-published-again", "SSR-public-source", true
	if got := phraseCall(t, a, owner, csrf, "PUT", "/api/v1/admin/phrases/"+id, private, 2, ""); got.Code != 200 {
		t.Fatal("republish phrase", got.Code)
	}
	if page := testRequest(a, "GET", "/phrases", "", nil, ""); !strings.Contains(page.Body.String(), "SSR-published-again") {
		t.Fatal("republished phrase absent from SSR")
	}
	if got := phraseCall(t, a, owner, csrf, "DELETE", "/api/v1/admin/phrases/"+id, nil, 3, ""); got.Code != 204 || got.Body.Len() != 0 {
		t.Fatal("delete SSR phrase", got.Code)
	}
	deleted := testRequest(a, "GET", "/phrases", "", nil, "")
	if deleted.Code != 200 || strings.Contains(deleted.Body.String(), "SSR-published-again") || strings.Contains(deleted.Body.String(), "SSR-public-source") || strings.Contains(deleted.Body.String(), "SSR-private-") {
		t.Fatal("deleted/private phrase remained in SSR")
	}
}

func TestPhrasesWorkbenchServerPermissions(t *testing.T) {
	a := testApp(t)
	_, owner, _ := testIdentity(t, a, "owner")
	_, reader, _ := testIdentity(t, a, "reader")
	_, author, _ := testIdentity(t, a, "author")
	a.RegisterWeb()
	for _, path := range []string{"/studio/phrases", "/studio/phrases/"} {
		for _, actor := range []struct {
			name   string
			cookie *http.Cookie
			status int
		}{{"anonymous", nil, 303}, {"reader", reader, 403}, {"author", author, 403}, {"owner", owner, 200}} {
			got := testRequest(a, "GET", path, "", actor.cookie, "")
			if got.Code != actor.status {
				t.Fatal("workbench route permission", actor.name, path, got.Code)
			}
			if actor.cookie == nil && !strings.HasPrefix(got.Header().Get("Location"), "/login?return_to=") {
				t.Fatal("anonymous workbench did not use local login redirect")
			}
		}
	}
}
