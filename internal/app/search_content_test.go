package app

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestContentSearchPublicTypesLiteralTermsAndCursorBinding(t *testing.T) {
	a := seriesTestApp(t)
	a.RegisterContent()
	a.RegisterSearch()
	_, owner, csrf := testIdentity(t, a, "owner")
	seriesTestPost(t, a, owner, csrf, "Attention 文章", true)
	seriesTestPost(t, a, owner, csrf, "Attention 私人文章", false)
	phrase := phraseCreateFixture(t, a, owner, csrf, "ＡＴＴＥＮＴＩＯＮ %_!", "来源", true)
	phraseCreateFixture(t, a, owner, csrf, "Attention private phrase", "", false)
	moment := momentFixture(t, a, owner, csrf, "Attention 公开动态", true)
	momentFixture(t, a, owner, csrf, "Attention private moment", false)
	in := seriesTestInput()
	in["title"] = "Attention 连载"
	series := contentWant(t, contentCall(t, a, owner, csrf, "POST", "/api/v1/series", in, 0, ""), 201)
	search := func(query string) map[string]any {
		t.Helper()
		v, e := a.SearchContent(context.Background(), url.Values{"q": {query}, "type": {"all"}, "limit": {"10"}})
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	items := webMaps(search("attention")["items"])
	if len(items) != 4 {
		t.Fatal("expected one public result per type, no private drafts", items)
	}
	kinds := map[string]bool{}
	for _, item := range items {
		kinds[webString(item["kind"])] = true
		if !strings.HasPrefix(webString(item["url"]), "/") {
			t.Fatal("invalid public target", item)
		}
	}
	for _, kind := range []string{"post", "series", "moment", "phrase"} {
		if !kinds[kind] {
			t.Fatal("missing content type", kind)
		}
	}
	if got := webMaps(search("%_!")["items"]); len(got) != 1 || got[0]["id"] != phrase["id"] {
		t.Fatal("wildcards must be literal", got)
	}
	first, e := a.SearchContent(context.Background(), url.Values{"q": {"attention"}, "type": {"all"}, "limit": {"1"}})
	if e != nil {
		t.Fatal(e)
	}
	cursor := webString(first["next_cursor"])
	if cursor == "" {
		t.Fatal("missing continuation")
	}
	if _, e = a.SearchContent(context.Background(), url.Values{"q": {"other"}, "type": {"all"}, "limit": {"1"}, "cursor": {cursor}}); e == nil {
		t.Fatal("cursor accepted another query")
	}
	if _, e = a.SearchContent(context.Background(), url.Values{"q": {"attention"}, "type": {"phrase"}, "limit": {"1"}, "cursor": {cursor}}); e == nil {
		t.Fatal("cursor accepted another type")
	}
	if _, e = a.SearchContent(context.Background(), url.Values{"q": {"attention"}, "type": {"invalid"}}); e == nil {
		t.Fatal("invalid type accepted")
	}
	// Withdrawal is reflected immediately, including suggestions.
	contentWant(t, contentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/phrases/"+webString(phrase["id"]), map[string]any{"text": "ＡＴＴＥＮＴＩＯＮ %_!", "kind": "sentence", "source": "来源", "published": false}, 1, ""), 200)
	in["status"] = "draft"
	contentWant(t, contentCall(t, a, owner, csrf, "PUT", "/api/v1/studio/series/"+webString(series["id"]), in, 1, ""), 200)
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", "/api/v1/studio/moments/"+webString(moment["id"])+"/publication", nil, 2, RandID()), 200)
	if got := webMaps(search("attention")["items"]); len(got) != 1 || got[0]["kind"] != "post" {
		t.Fatal("withdrawn content leaked", got)
	}
	suggestions := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/search/suggestions?q=attention", nil, 0, ""), 200)
	if len(webMaps(suggestions["items"])) != 1 {
		t.Fatal("withdrawn suggestion leaked", suggestions)
	}
}

func TestGameNameSearchCountsVisibilityAndCursorScope(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	for i, title := range []string{"ＡＬＰＨＡ %_!", "Alpha two", "Other", "Alpha hidden"} {
		in := homeTestInput("steam")
		in["title"] = title
		in["position"] = i
		in["visible"] = i != 3
		homeTestItem(t, a, owner, csrf, in)
	}
	homeGameEnable(t, a, owner, csrf, 1, true, 5)
	q := url.Values{"q": {"alpha"}, "limit": {"1"}}
	data, e := a.PublicGames(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	if data["total"] != 2 || data["has_more"] != true {
		t.Fatal("wrong filtered total", data)
	}
	q.Set("cursor", webString(data["next_cursor"]))
	q.Set("q", "other")
	if _, e = a.PublicGames(context.Background(), q); e == nil {
		t.Fatal("query changed but cursor accepted")
	}
	data, e = a.PublicGames(context.Background(), url.Values{"q": {"%_!"}})
	if e != nil || data["total"] != 1 {
		t.Fatal("game wildcards not literal", data, e)
	}
}
