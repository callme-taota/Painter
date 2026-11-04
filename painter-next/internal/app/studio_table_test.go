package app

import (
	"strings"
	"testing"
)

func TestStudioArticleSearchAndCursorScope(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	_, owner, csrf := testIdentity(t, a, "owner")
	_, author, authorCSRF := testIdentity(t, a, "author")
	for _, title := range []string{"跨页关键字 A", "跨页关键字 B", "完全不同"} {
		contentWant(t, contentCall(t, a, author, authorCSRF, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": title, "summary": "可查摘要", "markdown": "正文仅有的词"}, 0, ""), 201)
	}
	base := "/api/v1/me/posts?scope=site&q=跨页关键字&limit=1"
	first := contentWant(t, contentCall(t, a, owner, csrf, "GET", base, nil, 0, ""), 200)
	items := first["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["author_name"] != "Test User" {
		t.Fatal(first)
	}
	cursor := first["next_cursor"].(string)
	if cursor == "" {
		t.Fatal("all matching results must paginate")
	}
	second := contentWant(t, contentCall(t, a, owner, csrf, "GET", base+"&cursor="+cursor, nil, 0, ""), 200)
	if second["items"].([]any)[0].(map[string]any)["id"] == items[0].(map[string]any)["id"] {
		t.Fatal("cursor repeated row")
	}
	contentWant(t, contentCall(t, a, owner, csrf, "GET", strings.Replace(base, "跨页关键字", "不同", 1)+"&cursor="+cursor, nil, 0, ""), 400)
	own := contentWant(t, contentCall(t, a, owner, csrf, "GET", "/api/v1/me/posts?q=跨页关键字", nil, 0, ""), 200)
	if len(own["items"].([]any)) != 0 {
		t.Fatal("mine search leaked another author")
	}
	contentWant(t, contentCall(t, a, author, authorCSRF, "GET", base, nil, 0, ""), 403)
	summary := contentWant(t, contentCall(t, a, owner, csrf, "GET", "/api/v1/me/posts?scope=site&q=可查摘要", nil, 0, ""), 200)
	if len(summary["items"].([]any)) != 3 {
		t.Fatal("summary filter", summary)
	}
	body := contentWant(t, contentCall(t, a, owner, csrf, "GET", "/api/v1/me/posts?scope=site&q=正文仅有的词", nil, 0, ""), 200)
	if len(body["items"].([]any)) != 0 {
		t.Fatal("search contract is title/summary")
	}
}
func TestAuthorSSRUsesOwnSearchAndSavedBackground(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterSearch()
	a.RegisterWeb()
	p, c, csrf := testIdentity(t, a, "owner")
	_, other, otherCSRF := testIdentity(t, a, "author")
	reviewPublish(t, a, c, csrf, "作者同页关键字", "", "内容")
	reviewPublish(t, a, other, otherCSRF, "另一位作者同页关键字", "", "内容")
	page := testRequest(a, "GET", "/users/"+p.ID+"?q=同页关键字&author_id=999", "", nil, "")
	html := page.Body.String()
	if page.Code != 200 || !strings.Contains(html, `class="author-identity-row"`) || strings.Contains(html, "另一位作者同页关键字") || !strings.Contains(html, `action="/users/`+p.ID+`"`) {
		t.Fatal("author SSR search scope", page.Code)
	}
	if !strings.Contains(html, `name="robots" content="noindex,nofollow"`) {
		t.Fatal("author search must not index")
	}
}
