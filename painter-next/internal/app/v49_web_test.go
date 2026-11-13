package app

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

func v49WebApp(t *testing.T) *App {
	t.Helper()
	a := seriesTestApp(t)
	a.RegisterContent()
	a.RegisterWeb()
	return a
}

func TestV49ReaderPublicProfilePrivacyAndCreatorAffordances(t *testing.T) {
	a := v49WebApp(t)
	reader, readerCookie, _ := testIdentity(t, a, "reader")
	author, authorCookie, authorCSRF := testIdentity(t, a, "author")
	fixtures := []struct {
		principal               Principal
		cookie                  *http.Cookie
		name, bio, email, phone string
	}{
		{reader, readerCookie, "V49 Reader", "读者愿意公开的介绍", "v49-reader-private@example.invalid", "v49-reader-private-phone"},
		{author, authorCookie, "V49 Author", "作者愿意公开的介绍", "v49-author-private@example.invalid", "v49-author-private-phone"},
	}
	for _, fixture := range fixtures {
		if _, err := a.Pool.Exec(context.Background(), `UPDATE users SET display_name=$2,bio=$3,email=$4,phone=$5 WHERE id=$1`, fixture.principal.ID, fixture.name, fixture.bio, fixture.email, fixture.phone); err != nil {
			t.Fatal(err)
		}
	}
	postID := seriesTestPost(t, a, authorCookie, authorCSRF, "V49 作者公开写作", true)
	for _, fixture := range fixtures {
		path := "/users/" + fixture.principal.ID
		// A visitor and the profile owner receive the same public privacy boundary.
		for _, visitor := range []*http.Cookie{nil, fixture.cookie} {
			response := testRequest(a, "GET", path, "", visitor, "")
			page := response.Body.String()
			if response.Code != 200 || !strings.Contains(page, fixture.name) || !strings.Contains(page, fixture.bio) {
				t.Fatal("public profile lost its readable identity", path, response.Code)
			}
			if strings.Contains(page, fixture.email) || strings.Contains(page, fixture.phone) {
				t.Fatal("contact details leaked into public HTML", path)
			}
			dtoResponse := testRequest(a, "GET", "/api/v1"+path, "", visitor, "")
			dto := contentWant(t, dtoResponse, 200)
			if _, exists := dto["email"]; exists {
				t.Fatal("public DTO included email", path)
			}
			if _, exists := dto["phone"]; exists {
				t.Fatal("public DTO included phone", path)
			}
			if dto["display_name"] != fixture.name || strings.Contains(dtoResponse.Body.String(), fixture.email) || strings.Contains(dtoResponse.Body.String(), fixture.phone) {
				t.Fatal("public DTO lost identity or exposed private values", path)
			}
		}
	}
	readerPage := testRequest(a, "GET", "/users/"+reader.ID+"?q=anything&cursor=invalid-cursor", "", nil, "")
	if readerPage.Code != 200 {
		t.Fatal("reader profile incorrectly executed author article pagination", readerPage.Code)
	}
	readerDoc := friendWebDocument(t, readerPage.Body.String())
	readerSearch := friendWebNodes(readerDoc, func(node *nethtml.Node) bool {
		return node.Type == nethtml.ElementNode && node.Data == "form" && friendWebAttr(node, "action") == "/users/"+reader.ID
	})
	if len(readerSearch) != 0 || len(friendWebClass(readerDoc, "author-writing")) != 0 || strings.Contains(readerPage.Body.String(), "还没有公开文章") {
		t.Fatal("reader profile pretended to be an empty author profile")
	}
	if !strings.Contains(readerPage.Body.String(), "收藏、阅读足迹和联系方式仅自己可见") {
		t.Fatal("reader profile omitted the boundary between public identity and private reading")
	}
	authorPage := testRequest(a, "GET", "/users/"+author.ID, "", nil, "")
	authorDoc := friendWebDocument(t, authorPage.Body.String())
	authorSearch := friendWebNodes(authorDoc, func(node *nethtml.Node) bool {
		return node.Type == nethtml.ElementNode && node.Data == "form" && friendWebAttr(node, "action") == "/users/"+author.ID
	})
	authorArticle := friendWebNodes(authorDoc, func(node *nethtml.Node) bool {
		return node.Type == nethtml.ElementNode && node.Data == "a" && friendWebAttr(node, "href") == "/posts/"+postID
	})
	if authorPage.Code != 200 || len(authorSearch) != 1 || len(authorArticle) == 0 {
		t.Fatal("creator profile lost author search or its published article", authorPage.Code)
	}
	for _, fixture := range fixtures {
		if _, err := a.Pool.Exec(context.Background(), `UPDATE users SET disabled_at_ms=$2 WHERE id=$1`, fixture.principal.ID, NowMillis()); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"/users/" + fixture.principal.ID, "/api/v1/users/" + fixture.principal.ID} {
			response := testRequest(a, "GET", path, "", nil, "")
			if response.Code != 404 || strings.Contains(response.Body.String(), fixture.bio) || strings.Contains(response.Body.String(), fixture.email) || strings.Contains(response.Body.String(), fixture.phone) {
				t.Fatal("disabled profile remained a public identity or leaked private data", path, response.Code)
			}
		}
	}
}

func TestV49SeriesSSRReadingPaginationPermissionsAndSitemap(t *testing.T) {
	a := v49WebApp(t)
	author, cookie, csrf := testIdentity(t, a, "author")
	_, readerCookie, _ := testIdentity(t, a, "reader")
	first := seriesTestPost(t, a, cookie, csrf, "V49 第一篇公开章节", true)
	second := seriesTestPost(t, a, cookie, csrf, "V49 第二篇公开章节", true)
	third := seriesTestPost(t, a, cookie, csrf, "V49 排在最前的章节", true)
	draft := seriesTestPost(t, a, cookie, csrf, "V49 绝不可公开的章节标题", false)
	create := func(title, status string, chapters ...string) (string, map[string]any) {
		t.Helper()
		in := seriesTestInput(chapters...)
		in["title"] = title
		in["status"] = status
		value := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/series", in, 0, ""), 201)
		return value["id"].(string), in
	}
	serial, in := create("V49 连续阅读", "published", third, draft, first, second)
	empty, _ := create("V49 另一组连载", "published")
	newest, _ := create("V49 最新连载", "published")
	private, _ := create("V49 私人专栏标题", "draft", draft)
	deleted, _ := create("V49 已删除专栏标题", "published")
	contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/studio/series/"+deleted, nil, 1, ""), 200)
	for _, visitor := range []*http.Cookie{nil, cookie} {
		page := testRequest(a, "GET", "/series/"+serial, "", visitor, "")
		if page.Code != 200 || strings.Contains(page.Body.String(), "V49 绝不可公开的章节标题") {
			t.Fatal("public SSR leaked a draft chapter", page.Code)
		}
		doc := friendWebDocument(t, page.Body.String())
		contents := friendWebClass(doc, "series-contents")
		if len(contents) != 1 {
			t.Fatal("serial cannot be read as an ordered chapter directory")
		}
		links := friendWebNodes(contents[0], func(node *nethtml.Node) bool { return node.Type == nethtml.ElementNode && node.Data == "a" })
		want := []string{third, first, second}
		if len(links) != len(want) {
			t.Fatal("wrong public chapter count", len(links))
		}
		for index, id := range want {
			if friendWebAttr(links[index], "href") != "/posts/"+id {
				t.Fatal("directory lost author chapter order", index, friendWebAttr(links[index], "href"))
			}
		}
		for _, id := range []string{private, deleted, "9223372036854775807"} {
			if response := testRequest(a, "GET", "/series/"+id, "", visitor, ""); response.Code != 404 {
				t.Fatal("private/deleted/unknown serial was exposed", id, response.Code)
			}
		}
	}
	article := testRequest(a, "GET", "/posts/"+first, "", nil, "")
	articleDoc := friendWebDocument(t, article.Body.String())
	previous := friendWebNodes(articleDoc, func(node *nethtml.Node) bool {
		return node.Type == nethtml.ElementNode && node.Data == "a" && friendWebAttr(node, "rel") == "prev"
	})
	next := friendWebNodes(articleDoc, func(node *nethtml.Node) bool {
		return node.Type == nethtml.ElementNode && node.Data == "a" && friendWebAttr(node, "rel") == "next"
	})
	if article.Code != 200 || len(previous) != 1 || friendWebAttr(previous[0], "href") != "/posts/"+third || len(next) != 1 || friendWebAttr(next[0], "href") != "/posts/"+second {
		t.Fatal("article lost serial previous/next navigation", article.Code)
	}
	// Follow the actual SSR next link, preserving filters rather than deriving
	// a cursor independently in the test.
	pagePath := "/series?limit=1&q=V49&state=ongoing"
	for index, wantID := range []string{newest, empty, serial} {
		response := testRequest(a, "GET", pagePath, "", nil, "")
		if response.Code != 200 {
			t.Fatal("public serial pagination failed", pagePath, response.Code)
		}
		doc := friendWebDocument(t, response.Body.String())
		cards := friendWebClass(doc, "series-public-card")
		if len(cards) != 1 {
			t.Fatal("serial page size ignored", len(cards))
		}
		cardLinks := friendWebNodes(cards[0], func(node *nethtml.Node) bool {
			return node.Type == nethtml.ElementNode && node.Data == "a" && friendWebAttr(node, "href") == "/series/"+wantID
		})
		if len(cardLinks) == 0 {
			t.Fatal("serial pagination repeated/skipped an item", index, wantID)
		}
		if strings.Contains(response.Body.String(), "V49 私人专栏标题") || strings.Contains(response.Body.String(), "V49 已删除专栏标题") {
			t.Fatal("private or deleted serial appeared in public pagination")
		}
		pagination := friendWebClass(doc, "pagination")
		if index == 2 {
			if len(pagination) != 0 {
				t.Fatal("last serial page linked to an empty next page")
			}
			break
		}
		if len(pagination) != 1 {
			t.Fatal("serial page omitted next navigation", index)
		}
		links := friendWebNodes(pagination[0], func(node *nethtml.Node) bool { return node.Type == nethtml.ElementNode && node.Data == "a" })
		if len(links) != 1 {
			t.Fatal("ambiguous next page link")
		}
		pagePath = friendWebAttr(links[0], "href")
		target, err := url.Parse(pagePath)
		if err != nil || target.Path != "/series" || target.Query().Get("limit") != "1" || target.Query().Get("q") != "V49" || target.Query().Get("state") != "ongoing" || target.Query().Get("cursor") == "" {
			t.Fatal("next page lost its public search filters", pagePath, err)
		}
	}
	for _, path := range []string{"/studio/series", "/studio/series/"} {
		if response := testRequest(a, "GET", path, "", readerCookie, ""); response.Code != 403 {
			t.Fatal("reader entered creator serial management", path, response.Code)
		}
		if response := testRequest(a, "GET", path, "", nil, ""); response.Code != 303 {
			t.Fatal("anonymous serial management did not require login", path, response.Code)
		}
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/studio/posts/"+third+"/publication", nil, 2, ""), 200)
	for _, path := range []string{"/series/" + serial, "/posts/" + first} {
		page := testRequest(a, "GET", path, "", nil, "")
		if page.Code != 200 || strings.Contains(page.Body.String(), "V49 排在最前的章节") || strings.Contains(page.Body.String(), "V49 绝不可公开的章节标题") {
			t.Fatal("withdrawn/draft neighbour remained in SSR reading navigation", path, page.Code)
		}
	}
	article = testRequest(a, "GET", "/posts/"+first, "", nil, "")
	articleDoc = friendWebDocument(t, article.Body.String())
	previous = friendWebNodes(articleDoc, func(node *nethtml.Node) bool {
		return node.Type == nethtml.ElementNode && node.Data == "a" && friendWebAttr(node, "rel") == "prev"
	})
	if len(previous) != 0 {
		t.Fatal("withdrawn previous chapter remained navigable")
	}
	in["status"] = "draft"
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", "/api/v1/studio/series/"+serial, in, 1, ""), 200)
	if response := testRequest(a, "GET", "/series/"+serial, "", cookie, ""); response.Code != 404 {
		t.Fatal("withdrawn serial exposed by creator cookie", response.Code)
	}
	// More than one sitemap batch proves new public serials are not silently
	// omitted once the site's content exceeds the public API page size.
	expected := map[string]bool{a.Config.PublicBaseURL + "/series/" + newest: true, a.Config.PublicBaseURL + "/series/" + empty: true}
	rows, err := a.Pool.Query(context.Background(), `INSERT INTO series(author_user_id,title,slug,status,state,created_at_ms,updated_at_ms) SELECT $1,'V49 Sitemap '||n,$2||n,'published','ongoing',$3,$3 FROM generate_series(1,55) n RETURNING id::text`, author.ID, "v49-sitemap-"+RandID()[:12]+"-", NowMillis())
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		expected[a.Config.PublicBaseURL+"/series/"+id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	sitemap := testRequest(a, "GET", "/sitemap.xml", "", nil, "")
	var locations struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if sitemap.Code != 200 {
		t.Fatal("sitemap failed", sitemap.Code, sitemap.Body.String())
	}
	if err = xml.Unmarshal(sitemap.Body.Bytes(), &locations); err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, entry := range locations.URLs {
		if strings.HasPrefix(entry.Loc, a.Config.PublicBaseURL+"/series/") {
			seen[entry.Loc]++
			if !expected[entry.Loc] {
				t.Fatal("sitemap exposed a private/withdrawn/deleted serial", entry.Loc)
			}
		}
	}
	if len(seen) != len(expected) {
		t.Fatal("sitemap omitted serials beyond one batch", len(seen), len(expected))
	}
	for location := range expected {
		if seen[location] != 1 {
			t.Fatal("sitemap omitted or duplicated serial", location, seen[location])
		}
	}
}
