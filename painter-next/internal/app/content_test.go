package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func contentCall(t *testing.T, a *App, cookie *http.Cookie, csrf, method, path string, body any, version int64, key string) *httptest.ResponseRecorder {
	t.Helper()
	raw := []byte{}
	if body != nil {
		var e error
		raw, e = json.Marshal(body)
		if e != nil {
			t.Fatal(e)
		}
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", a.Config.PublicBaseURL)
	r.Header.Set("X-CSRF-Token", csrf)
	if key == "" {
		key = RandID()
	}
	r.Header.Set("Idempotency-Key", key)
	if version > 0 {
		r.Header.Set("If-Match", fmt.Sprintf("\"%d\"", version))
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}
func contentWant(t *testing.T, w *httptest.ResponseRecorder, status int) map[string]any {
	t.Helper()
	if w.Code != status {
		t.Fatalf("expected HTTP %d got %d: %s", status, w.Code, w.Body.String())
	}
	return jsonMap(t, w)
}
func contentNumber(v map[string]any, key string) int64 { return int64(v[key].(float64)) }
func TestContentPublicationDraftIsolationAndReplay(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterSearch()
	_, cookie, csrf := testIdentity(t, a, "owner")
	d := map[string]any{"client_draft_id": "draft-durable-1", "title": "中文搜索：设计与写作", "summary": "稳定发布正文", "markdown": "# 中文搜索\n\n公开正文和Go语言，百分号 %_! 按字面。", "tag_ids": []string{}, "media_ids": []string{}}
	key := RandID()
	created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", d, 0, key), 201)
	id := created["id"].(string)
	again := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", d, 0, key), 201)
	if again["id"] != id {
		t.Fatal("idempotent create duplicated")
	}
	d["title"] = "different"
	contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", d, 0, key), 409)
	d["title"] = "中文搜索：设计与写作"
	contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 404)
	search := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/search?q=中文搜索", nil, 0, ""), 200)
	if len(search["items"].([]any)) != 0 {
		t.Fatal("draft leaked into search")
	}
	publishBody := map[string]any{"expected_draft_version": 1}
	publishKey := RandID()
	published := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", publishBody, 1, publishKey), 200)
	version := contentNumber(published, "version")
	if version != 2 {
		t.Fatalf("one publish advanced version to %d", version)
	}
	public := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	if public["title"] != d["title"] || !strings.Contains(public["sanitized_html"].(string), "section-1") {
		t.Fatal(public)
	}
	for _, q := range []string{"中文搜索", "中文", "Go语言", "%25_!"} {
		s := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/search?q="+q, nil, 0, ""), 200)
		if len(s["items"].([]any)) != 1 {
			t.Fatalf("search %q: %#v", q, s)
		}
	}
	d["title"] = "草稿独有词"
	d["markdown"] = "这段尚未上线"
	d["expected_draft_version"] = 1
	delete(d, "client_draft_id")
	saved := contentWant(t, contentCall(t, a, cookie, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", d, version, ""), 200)
	version = contentNumber(saved, "version")
	old := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	if old["title"] != public["title"] {
		t.Fatal("autosave changed publication")
	}
	repeated := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", publishBody, 1, publishKey), 200)
	if repeated["revision_id"] != published["revision_id"] {
		t.Fatal("replayed publish changed revision")
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", d, 1, ""), 409)
	contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/checkpoints", map[string]any{"expected_draft_version": 2}, version, ""), 200)
	version++
	unpublished := contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/studio/posts/"+id+"/publication", nil, version, ""), 200)
	version = contentNumber(unpublished, "version")
	contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 404)
	s := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/search?q=中文搜索", nil, 0, ""), 200)
	if len(s["items"].([]any)) != 0 {
		t.Fatal("withdrawn article still searchable")
	}
	trashed := contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/studio/posts/"+id, nil, version, ""), 200)
	version = contentNumber(trashed, "version")
	recovered := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/recover", nil, version, ""), 200)
	version = contentNumber(recovered, "version")
	contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 404)
	restored := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/restore", map[string]any{"revision_id": published["revision_id"]}, version, ""), 200)
	private := contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/studio/posts/"+id, nil, 0, ""), 200)
	if private["title"] != public["title"] || private["publication"] != nil || contentNumber(private, "version") != contentNumber(restored, "version") {
		t.Fatal("restore did not create private working copy", private)
	}
	if _, e := a.Pool.Exec(context.Background(), "UPDATE post_revisions SET title='changed' WHERE id=$1", published["revision_id"]); e == nil {
		t.Fatal("immutable revision changed")
	}
}
func TestContentCASPermissionsAndAtomicPublishFailure(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterSearch()
	_, cookie, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "并发测试", "markdown": "稳定内容"}, 0, ""), 201)
	id := created["id"].(string)
	contentWant(t, contentCall(t, a, other, otherCSRF, "GET", "/api/v1/studio/posts/"+id, nil, 0, ""), 404)
	contentWant(t, contentCall(t, a, cookie, "wrong", "PUT", "/api/v1/studio/posts/"+id+"/draft", map[string]any{"title": "bad", "expected_draft_version": 1}, 1, ""), 403)
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := contentCall(t, a, cookie, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", map[string]any{"title": "并发" + strconv.Itoa(i), "markdown": "保存正文", "expected_draft_version": 1}, 1, "")
			codes <- w.Code
		}(i)
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal("CAS race", counts)
	}
	_, e := a.Pool.Exec(context.Background(), `CREATE FUNCTION test_fail_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.topic='post.published.v1' THEN RAISE EXCEPTION 'injected failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER test_outbox BEFORE INSERT ON outbox FOR EACH ROW EXECUTE FUNCTION test_fail_outbox()`)
	if e != nil {
		t.Fatal(e)
	}
	failed := contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 2}, 2, "")
	if failed.Code < 400 {
		t.Fatal("failed transaction succeeded")
	}
	var publications, revisions int
	if e = a.Pool.QueryRow(context.Background(), "SELECT (SELECT count(*) FROM post_publications),(SELECT count(*) FROM post_revisions)").Scan(&publications, &revisions); e != nil {
		t.Fatal(e)
	}
	if publications != 0 || revisions != 0 {
		t.Fatal("publication not atomic", publications, revisions)
	}
}
func TestMarkdownSanitizationAndHighlight(t *testing.T) {
	rendered, e := RenderMarkdown("# 中文标题\n\n<script>alert(1)</script>\n\n[恶意](javascript:alert(1))\n\n```go\nfunc main() {}\n```\n\n- [x] 完成")
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(rendered.HTML, "<script") || strings.Contains(rendered.HTML, "javascript:") {
		t.Fatal("unsafe rendering", rendered.HTML)
	}
	if len(rendered.TOC) != 1 || rendered.TOC[0].Text != "中文标题" || !strings.Contains(rendered.HTML, "chroma") {
		t.Fatal(rendered)
	}
	if _, e = RenderMarkdown("![x](https://untrusted.example/image.png)"); e == nil {
		t.Fatal("untracked remote image allowed")
	}
}

func TestStudioListSignedPaginationAndEmptyDraftRestore(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterSearch()
	owner, ownerCookie, ownerCSRF := testIdentity(t, a, "owner")
	_, cookie, csrf := testIdentity(t, a, "author")
	ids := []string{}
	for i := 0; i < 3; i++ {
		v := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": fmt.Sprintf("条目%d", i), "markdown": "正文"}, 0, ""), 201)
		ids = append(ids, v["id"].(string))
	}
	page := contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/me/posts?limit=1", nil, 0, ""), 200)
	if !page["has_more"].(bool) || len(page["items"].([]any)) != 1 {
		t.Fatal(page)
	}
	cursor := page["next_cursor"].(string)
	next := contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/me/posts?limit=1&cursor="+cursor, nil, 0, ""), 200)
	if next["items"].([]any)[0].(map[string]any)["id"] == page["items"].([]any)[0].(map[string]any)["id"] {
		t.Fatal("page repeated")
	}
	if _, ok := next["items"].([]any)[0].(map[string]any)["markdown"]; ok {
		t.Fatal("list includes full markdown")
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/me/posts?limit=1&cursor="+cursor+"x", nil, 0, ""), 400)
	contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/me/posts?limit=1&status=draft&cursor="+cursor, nil, 0, ""), 400)
	contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/me/posts?scope=site", nil, 0, ""), 403)
	own := contentWant(t, contentCall(t, a, ownerCookie, ownerCSRF, "GET", "/api/v1/me/posts", nil, 0, ""), 200)
	if len(own["items"].([]any)) != 0 {
		t.Fatal(owner, own)
	}
	all := contentWant(t, contentCall(t, a, ownerCookie, ownerCSRF, "GET", "/api/v1/me/posts?scope=site", nil, 0, ""), 200)
	if len(all["items"].([]any)) != 3 {
		t.Fatal(all)
	}
	id := ids[0]
	published := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	saved := contentWant(t, contentCall(t, a, cookie, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", map[string]any{"title": "", "summary": "还没写完的摘要", "markdown": "这份未完成的工作副本也必须保留", "expected_draft_version": 1}, contentNumber(published, "version"), ""), 200)
	contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/restore", map[string]any{"revision_id": published["revision_id"]}, contentNumber(saved, "version"), ""), 200)
	var title, body string
	if e := a.Pool.QueryRow(context.Background(), "SELECT title,markdown FROM post_revisions WHERE post_id=$1 AND reason='before_restore'", id).Scan(&title, &body); e != nil {
		t.Fatal(e)
	}
	if title != "" || !strings.Contains(body, "未完成") {
		t.Fatal("unfinished draft restore point lost")
	}
}
