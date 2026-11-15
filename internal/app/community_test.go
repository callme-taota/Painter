package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	nethtml "golang.org/x/net/html"
)

func communitySeriesFixture(t *testing.T, a *App, cookie *http.Cookie, csrf, title, status string) map[string]any {
	t.Helper()
	in := seriesTestInput()
	in["title"], in["status"] = title, status
	return contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/series", in, 0, ""), 201)
}

func communityAssertPublic(t *testing.T, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"version":`, `"draft_version":`, `"markdown":`, `"password_hash":`, `"email":`, `"phone":`, `"csrf_token":`, `"cover_media_id":`, `"background_media_id":`, "community-private-"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("private field/content exposed (%s): %s", forbidden, raw)
		}
	}
}

func TestCommunityPageReadsPublicSnapshotsAndRealCreators(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	ctx := context.Background()
	postAuthor, postCookie, postCSRF := testIdentity(t, a, "author")
	momentAuthor, momentCookie, momentCSRF := testIdentity(t, a, "author")
	seriesAuthor, seriesCookie, seriesCSRF := testIdentity(t, a, "author")
	draftAuthor, draftCookie, draftCSRF := testIdentity(t, a, "author")
	for _, id := range []string{postAuthor.ID, momentAuthor.ID, seriesAuthor.ID, draftAuthor.ID} {
		if _, err := a.Pool.Exec(ctx, `UPDATE users SET email='community-private-email@example.invalid',phone='community-private-phone' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	var oldest, newest string
	for n := 0; n < 13; n++ {
		id := seriesTestPost(t, a, postCookie, postCSRF, fmt.Sprintf("公开文章 %d", n), true)
		if n == 0 {
			oldest = id
		}
		newest = id
	}
	contentWant(t, contentCall(t, a, postCookie, postCSRF, "PUT", "/api/v1/studio/posts/"+newest+"/draft", map[string]any{"title": "community-private-working-title", "markdown": "community-private-working-body", "expected_draft_version": 1}, 2, ""), 200)
	seriesTestPost(t, a, draftCookie, draftCSRF, "community-private-draft-post", false)
	withdrawn := seriesTestPost(t, a, draftCookie, draftCSRF, "community-private-withdrawn-post", true)
	contentWant(t, contentCall(t, a, draftCookie, draftCSRF, "DELETE", "/api/v1/studio/posts/"+withdrawn+"/publication", nil, 2, ""), 200)
	moment := momentFixture(t, a, momentCookie, momentCSRF, "公开动态", true)
	momentResponse(t, momentCall(t, a, momentCookie, momentCSRF, "PUT", "/api/v1/studio/moments/"+webString(moment["id"])+"/draft", map[string]any{"text": "community-private-moment-edit"}, 2, RandID()), 200)
	momentFixture(t, a, draftCookie, draftCSRF, "community-private-draft-moment", false)
	withdrawnMoment := momentFixture(t, a, draftCookie, draftCSRF, "community-private-withdrawn-moment", true)
	momentResponse(t, momentCall(t, a, draftCookie, draftCSRF, "DELETE", "/api/v1/studio/moments/"+webString(withdrawnMoment["id"])+"/publication", nil, 2, RandID()), 200)
	for n := 0; n < 4; n++ {
		communitySeriesFixture(t, a, seriesCookie, seriesCSRF, fmt.Sprintf("公开专栏 %d", n), "published")
	}
	communitySeriesFixture(t, a, draftCookie, draftCSRF, "community-private-draft-series", "draft")
	removed := communitySeriesFixture(t, a, draftCookie, draftCSRF, "community-private-deleted-series", "published")
	contentWant(t, contentCall(t, a, draftCookie, draftCSRF, "DELETE", "/api/v1/studio/series/"+webString(removed["id"]), nil, 1, ""), 200)

	query := url.Values{"author_id": {draftAuthor.ID}, "status": {"draft"}, "scope": {"site"}, "limit": {"50"}, "lang": {"en"}}
	before := query.Encode()
	page, err := a.communityPage(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	if page.Kind != "discover" || page.Title != "站内" || len(page.Items) != 12 || len(page.Series) != 3 || len(page.Authors) != 3 {
		t.Fatal("incorrect discovery page shape/bounds", page.Kind, page.Title, len(page.Items), len(page.Series), page.Authors)
	}
	if webString(page.Items[0]["id"]) != newest || webString(page.Items[0]["title"]) == "community-private-working-title" {
		t.Fatal("discovery did not use current public snapshot", page.Items[0])
	}
	for _, item := range page.Items {
		if item["id"] == oldest || item["author"].(map[string]any)["id"] != postAuthor.ID {
			t.Fatal("discovery cap/order/public scope incorrect", item)
		}
	}
	wanted := map[string]bool{postAuthor.ID: true, momentAuthor.ID: true, seriesAuthor.ID: true}
	for _, author := range page.Authors {
		id := webString(author["id"])
		if !wanted[id] || author["url"] != "/users/"+id || author["last_public_activity_at"] == "" {
			t.Fatal("directory included an unpublished account or incorrect public destination", author)
		}
		delete(wanted, id)
	}
	if len(wanted) != 0 {
		t.Fatal("one public content type failed to discover its author", wanted)
	}
	if page.Next == "" || query.Encode() != before {
		t.Fatal("discovery lost continuation or changed caller filters", page.Next, query)
	}
	nextURL, err := url.Parse(page.Next)
	if err != nil || nextURL.Path != "/discover" || nextURL.Query().Get("lang") != "en" || len(nextURL.Query()) != 2 {
		t.Fatal("invalid discovery continuation", page.Next, err)
	}
	next, err := a.communityPage(ctx, nextURL.Query())
	if err != nil || len(next.Items) != 1 || next.Items[0]["id"] != oldest || next.Next != "" || len(next.Authors) != 3 || len(next.Series) != 3 {
		t.Fatal("discovery continuation lost public post order or sidebar bounds", next, err)
	}
	communityAssertPublic(t, []any{page.Items, page.Series, page.Authors})
}

func TestCommunityAuthorsBoundOrderAndAccountVisibility(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	type activity struct {
		id    string
		stamp time.Time
	}
	var expected []activity
	for n := 0; n < 10; n++ {
		p, cookie, csrf := testIdentity(t, a, "author")
		moment := momentFixture(t, a, cookie, csrf, fmt.Sprintf("创作者动态 %d", n), true)
		stamp, err := time.Parse(time.RFC3339Nano, webString(moment["published_at"]))
		if err != nil {
			t.Fatal(err)
		}
		expected = append(expected, activity{id: p.ID, stamp: stamp})
	}
	for _, kind := range []string{"disabled", "reader"} {
		p, cookie, csrf := testIdentity(t, a, "author")
		momentFixture(t, a, cookie, csrf, "已有公开内容的"+kind, true)
		var err error
		if kind == "disabled" {
			_, err = a.Pool.Exec(ctx, `UPDATE users SET disabled_at_ms=$2 WHERE id=$1`, p.ID, NowMillis())
		} else {
			_, err = a.Pool.Exec(ctx, `UPDATE users SET role='reader' WHERE id=$1`, p.ID)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Slice(expected, func(i, j int) bool {
		if !expected[i].stamp.Equal(expected[j].stamp) {
			return expected[i].stamp.After(expected[j].stamp)
		}
		left, _ := strconv.ParseInt(expected[i].id, 10, 64)
		right, _ := strconv.ParseInt(expected[j].id, 10, 64)
		return left > right
	})
	for pass := 0; pass < 2; pass++ {
		authors, err := a.communityAuthors(ctx)
		if err != nil || len(authors) != 8 {
			t.Fatal("directory was not bounded", authors, err)
		}
		for i, author := range authors {
			if author["id"] != expected[i].id {
				t.Fatal("directory order/visibility was unstable", i, author, expected[i])
			}
		}
		communityAssertPublic(t, authors)
	}
}

func TestAuthorContentTabsBindRouteScopeAndRejectForeignCursors(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	ctx := context.Background()
	first, firstCookie, firstCSRF := testIdentity(t, a, "author")
	second, secondCookie, secondCSRF := testIdentity(t, a, "author")
	for _, actor := range []struct {
		cookie *http.Cookie
		csrf   string
	}{{firstCookie, firstCSRF}, {secondCookie, secondCSRF}} {
		for n := 0; n < 2; n++ {
			seriesTestPost(t, a, actor.cookie, actor.csrf, "匹配词 公开文章", true)
			momentFixture(t, a, actor.cookie, actor.csrf, "公开动态", true)
			communitySeriesFixture(t, a, actor.cookie, actor.csrf, "匹配词 公开专栏", "published")
		}
		seriesTestPost(t, a, actor.cookie, actor.csrf, "community-private-post", false)
		momentFixture(t, a, actor.cookie, actor.csrf, "community-private-moment", false)
		communitySeriesFixture(t, a, actor.cookie, actor.csrf, "community-private-series", "draft")
	}
	for _, tab := range []string{"posts", "moments", "series"} {
		q := url.Values{"author_id": {second.ID}, "limit": {"1"}, "scope": {"site"}, "status": {"draft"}, "type": {"phrase"}, "tag_id": {"not-a-tag"}}
		before := q.Encode()
		result, err := a.AuthorContent(ctx, first.ID, tab, q)
		if err != nil {
			t.Fatal(tab, err)
		}
		if q.Encode() != before {
			t.Fatal("caller query mutated", q)
		}
		items := webMaps(result["items"])
		if len(items) != 1 || result["has_more"] != true || items[0]["author"].(map[string]any)["id"] != first.ID {
			t.Fatal("route identity did not override client author/status/scope", tab, result)
		}
		communityAssertPublic(t, result)
		cursor := webString(result["next_cursor"])
		q.Set("cursor", cursor)
		next, err := a.AuthorContent(ctx, first.ID, tab, q)
		if err != nil || len(webMaps(next["items"])) != 1 || next["has_more"] != false || webMaps(next["items"])[0]["id"] == items[0]["id"] {
			t.Fatal("scoped continuation did not advance exactly once", tab, next, err)
		}
		if _, err = a.AuthorContent(ctx, second.ID, tab, q); err == nil {
			t.Fatal("cursor crossed author boundary", tab)
		}
		for _, otherTab := range []string{"posts", "moments", "series"} {
			if otherTab == tab {
				continue
			}
			if _, err = a.AuthorContent(ctx, first.ID, otherTab, q); err == nil {
				t.Fatal("cursor crossed tab boundary", tab, otherTab)
			}
		}
	}
	query := url.Values{"q": {"匹配词"}, "limit": {"1"}, "author_id": {second.ID}}
	original := url.Values{"q": {"匹配词"}, "limit": {"1"}, "author_id": {second.ID}}
	for _, tab := range []string{"posts", "series"} {
		result, err := a.AuthorContent(ctx, first.ID, tab, query)
		if err != nil || len(webMaps(result["items"])) != 1 || webMaps(result["items"])[0]["author"].(map[string]any)["id"] != first.ID {
			t.Fatal("supported tab search failed", tab, result, err)
		}
	}
	result, err := a.AuthorContent(ctx, first.ID, "moments", query)
	if err != nil || len(webMaps(result["items"])) != 1 || !reflect.DeepEqual(query, original) {
		t.Fatal("unsupported moment search was applied or mutated caller filters", result, err, query)
	}
	if _, err := a.AuthorContent(ctx, first.ID, "unknown", nil); err == nil {
		t.Fatal("unsupported content tab accepted")
	} else {
		var api *APIError
		if !errors.As(err, &api) || api.Status != 400 {
			t.Fatal("incorrect unknown tab failure", err)
		}
	}
	if _, err := a.AuthorContent(ctx, "1 OR 1=1", "posts", nil); err == nil {
		t.Fatal("invalid route identity accepted")
	}
}

func TestCommunityWebAuthorTabsPublicScopeAndCanonical(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterWeb()
	first, firstCookie, firstCSRF := testIdentity(t, a, "author")
	_, secondCookie, secondCSRF := testIdentity(t, a, "author")
	reader, _, _ := testIdentity(t, a, "reader")
	for _, actor := range []struct {
		cookie *http.Cookie
		csrf   string
		prefix string
	}{{firstCookie, firstCSRF, "SSR-first-"}, {secondCookie, secondCSRF, "SSR-second-"}} {
		seriesTestPost(t, a, actor.cookie, actor.csrf, actor.prefix+"post", true)
		momentFixture(t, a, actor.cookie, actor.csrf, actor.prefix+"moment", true)
		communitySeriesFixture(t, a, actor.cookie, actor.csrf, actor.prefix+"series", "published")
		seriesTestPost(t, a, actor.cookie, actor.csrf, "community-private-SSR-post", false)
		momentFixture(t, a, actor.cookie, actor.csrf, "community-private-SSR-moment", false)
		communitySeriesFixture(t, a, actor.cookie, actor.csrf, "community-private-SSR-series", "draft")
	}
	for _, cookie := range []*http.Cookie{nil, firstCookie} {
		response := testRequest(a, "GET", "/discover?author_id="+first.ID+"&scope=site&status=draft&lang=en", "", cookie, "")
		doc := friendWebDocument(t, response.Body.String())
		if response.Code != 200 || len(friendWebClass(doc, "discovery-post")) != 2 || !strings.Contains(friendWebText(doc), "SSR-first-post") || !strings.Contains(friendWebText(doc), "SSR-second-post") || strings.Contains(response.Body.String(), "community-private-") {
			t.Fatal("site discovery filtered to one author, lost SSR content or leaked a draft", response.Code, response.Body.String())
		}
		canonical := friendWebNodes(doc, func(node *nethtml.Node) bool {
			return node.Type == nethtml.ElementNode && node.Data == "link" && friendWebAttr(node, "rel") == "canonical"
		})
		if len(canonical) != 1 || friendWebAttr(canonical[0], "href") != a.Config.PublicBaseURL+"/discover" {
			t.Fatal("discovery canonical includes content filters or locale", canonical)
		}
	}
	for _, tab := range []string{"posts", "moments", "series"} {
		for _, cookie := range []*http.Cookie{nil, firstCookie} {
			path := "/users/" + first.ID + "?tab=" + tab + "&author_id=999999&scope=site&status=draft"
			if tab == "moments" {
				path += "&q=ignored-by-moments&lang=en"
			}
			response := testRequest(a, "GET", path, "", cookie, "")
			if response.Code != 200 || response.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("author tab unavailable or cacheable", tab, response.Code, response.Header())
			}
			doc := friendWebDocument(t, response.Body.String())
			content := friendWebClass(doc, "author-writing")
			want := "SSR-first-" + map[string]string{"posts": "post", "moments": "moment", "series": "series"}[tab]
			if len(content) != 1 || friendWebAttr(content[0], "data-author-tab") != tab || !strings.Contains(friendWebText(content[0]), want) || strings.Contains(friendWebText(content[0]), "SSR-second-") || strings.Contains(response.Body.String(), "community-private-") {
				t.Fatal("SSR tab showed another author/private content or lost tab", tab, response.Body.String())
			}
			canonical := friendWebNodes(doc, func(node *nethtml.Node) bool {
				return node.Type == nethtml.ElementNode && node.Data == "link" && friendWebAttr(node, "rel") == "canonical"
			})
			wantCanonical := a.Config.PublicBaseURL + "/users/" + first.ID
			if tab != "posts" {
				wantCanonical += "?tab=" + tab
			}
			if len(canonical) != 1 || friendWebAttr(canonical[0], "href") != wantCanonical {
				t.Fatal("canonical lost stable author/tab identity or accepted injected scope", tab, canonical)
			}
			if tab == "moments" && len(friendWebClass(doc, "author-search")) != 0 {
				t.Fatal("moment tab exposes unsupported search")
			}
		}
	}
	if response := testRequest(a, "GET", "/users/"+reader.ID+"?tab=moments", "", nil, ""); response.Code != 200 || len(friendWebClass(friendWebDocument(t, response.Body.String()), "author-writing")) != 0 {
		t.Fatal("reader profile was promoted to a creator page", response.Code)
	}
	if response := testRequest(a, "GET", "/users/"+first.ID+"?tab=invalid", "", nil, ""); response.Code != 400 {
		t.Fatal("unknown author tab accepted", response.Code)
	}
	if _, err := a.Pool.Exec(context.Background(), `UPDATE users SET disabled_at_ms=$2 WHERE id=$1`, first.ID, NowMillis()); err != nil {
		t.Fatal(err)
	}
	if response := testRequest(a, "GET", "/users/"+first.ID+"?tab=posts", "", nil, ""); response.Code != 404 || strings.Contains(response.Body.String(), "SSR-first-post") {
		t.Fatal("disabled public profile leaked", response.Code)
	}
}
