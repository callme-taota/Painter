package app

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestSearchMultiTagORAndCursorBinding(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterSearch()
	a.RegisterWeb()
	_, cookie, csrf := testIdentity(t, a, "owner")
	ctx := context.Background()
	ids := []string{}
	for _, name := range []string{"alpha", "beta", "gamma"} {
		var id string
		if e := a.Pool.QueryRow(ctx, `INSERT INTO tags(name,created_at_ms,updated_at_ms) VALUES($1,1,1) RETURNING id::text`, name).Scan(&id); e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
	}
	for i, tags := range [][]string{{ids[0]}, {ids[1]}, {ids[2]}, {ids[0], ids[1]}} {
		created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": fmt.Sprintf("筛选测试 %d", i), "markdown": "公开筛选正文", "tag_ids": tags}, 0, ""), 201)
		contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+created["id"].(string)+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	}
	base := url.Values{"tag_id": {ids[0], ids[1]}, "limit": {"1"}}
	first, e := a.SearchPosts(ctx, base)
	if e != nil {
		t.Fatal(e)
	}
	if len(webMaps(first["items"])) != 1 || first["effective_sort"] != "published_at_desc_v1" {
		t.Fatal(first)
	}
	cursor := first["next_cursor"].(string)
	if cursor == "" {
		t.Fatal("expected cursor")
	}
	next := url.Values{"tag_id": {ids[1], ids[0], ids[0]}, "limit": {"1"}, "cursor": {cursor}}
	if _, e = a.SearchPosts(ctx, next); e != nil {
		t.Fatalf("same tag set reordered: %v", e)
	}
	next["tag_id"] = []string{ids[2]}
	if _, e = a.SearchPosts(ctx, next); e == nil {
		t.Fatal("cursor crossed filter")
	}
	all, e := a.SearchPosts(ctx, url.Values{"q": {"筛选"}, "tag_id": {ids[0], ids[1]}})
	if e != nil || len(webMaps(all["items"])) != 3 {
		t.Fatal(all, e)
	}
	for _, order := range []string{"recent", "newest"} {
		sorted, err := a.SearchPosts(ctx, url.Values{"q": {"筛选测试"}, "sort": {order}, "tag_id": {ids[0], ids[1]}})
		if err != nil || len(webMaps(sorted["items"])) != 3 {
			t.Fatal("long keyword chronological search", order, sorted, err)
		}
	}
	browse, e := a.PublicPosts(ctx, url.Values{"tag_id": {ids[0], ids[1]}})
	if e != nil || len(webMaps(browse["items"])) != 3 {
		t.Fatal(browse, e)
	}
	for _, q := range []url.Values{{"q": {"筛选"}, "tag_id": {ids[0], "bad"}}, {"q": {"筛选"}, "tag_id": {strings.Repeat("9", 30)}}, {}} {
		if _, e = a.SearchPosts(ctx, q); e == nil {
			t.Fatal("invalid query accepted", q)
		}
	}
	many := url.Values{"q": {"筛选"}}
	for i := 1; i <= 21; i++ {
		many.Add("tag_id", fmt.Sprint(i))
	}
	if _, e = a.SearchPosts(ctx, many); e == nil {
		t.Fatal("tag limit ignored")
	}
	w := testRequest(a, "GET", "/search?tag_id="+ids[0]+"&tag_id="+ids[1], "", nil, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "搜索结果") || strings.Contains(w.Body.String(), "<select") {
		t.Fatalf("SSR filter failed %d", w.Code)
	}
	tagPage := testRequest(a, "GET", "/tags", "", nil, "")
	if tagPage.Code != 301 || tagPage.Header().Get("Location") != "/search" {
		t.Fatal("legacy tag directory")
	}
}
