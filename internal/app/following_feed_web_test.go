package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

func followingFeedBootstrap(t *testing.T, page string) map[string]any {
	t.Helper()
	doc := friendWebDocument(t, page)
	nodes := friendWebNodes(doc, func(n *nethtml.Node) bool {
		return n.Type == nethtml.ElementNode && n.Data == "script" && friendWebAttr(n, "id") == "following-feed-data"
	})
	if len(nodes) != 1 || friendWebAttr(nodes[0], "type") != "application/json" {
		t.Fatal("missing non-executable feed bootstrap")
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(friendWebText(nodes[0])), &data); err != nil {
		t.Fatal("invalid bootstrap JSON", err)
	}
	return data
}

func TestFollowingFeedWebAuthenticationAndPrivatePolicy(t *testing.T) {
	a := followingFeedApp(t)
	a.RegisterWeb()
	followingFeedIdentity(t, a, "owner")
	reader := followingFeedIdentity(t, a, "reader")
	path := "/following?type=moment&lang=en"
	response := testRequest(a, "GET", path, "", nil, "")
	if response.Code != http.StatusSeeOther || response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatal("unauthenticated private feed must not render or cache", response.Code, response.Header())
	}
	redirect, err := url.Parse(response.Header().Get("Location"))
	if err != nil || redirect.Path != "/login" || redirect.Query().Get("return_to") != path {
		t.Fatal("sign-in lost safe return path", redirect, err)
	}
	empty := testRequest(a, "GET", "/following", "", reader.cookie, "")
	if empty.Code != 200 || empty.Header().Get("Cache-Control") != "private, no-store" || !strings.Contains(empty.Body.String(), `name="robots" content="noindex,nofollow"`) {
		t.Fatal("empty feed must be readable and non-indexed", empty.Code, empty.Body.String())
	}
	data := followingFeedBootstrap(t, empty.Body.String())
	if data["viewer_id"] != reader.user.ID || data["following_count"] != float64(0) || len(data["items"].([]any)) != 0 {
		t.Fatal("empty SSR feed used another reader's scope", data)
	}
	if _, err = a.Pool.Exec(context.Background(), `UPDATE users SET disabled_at_ms=$2 WHERE id=$1`, reader.user.ID, NowMillis()); err != nil {
		t.Fatal(err)
	}
	denied := testRequest(a, "GET", "/following", "", reader.cookie, "")
	if denied.Code != http.StatusSeeOther || strings.Contains(denied.Body.String(), `id="following-feed-data"`) {
		t.Fatal("disabled reader retained SSR access", denied.Code)
	}
}

func TestFollowingFeedWebReadableSnapshotsEscapingAndCursors(t *testing.T) {
	a := followingFeedApp(t)
	a.RegisterWeb()
	followingFeedIdentity(t, a, "owner")
	reader, other, author := followingFeedIdentity(t, a, "reader"), followingFeedIdentity(t, a, "reader"), followingFeedIdentity(t, a, "author")
	followingFeedFollow(t, a, reader, author, true)
	attack := `</script><script data-following-xss>alert(1)</script>`
	post := followingFeedPost(t, a, author, "SSR-public-title "+attack, "SSR-public-summary", 3000)
	moment := followingFeedMoment(t, a, author, "SSR-public-moment "+attack, 2000)
	followingFeedMoment(t, a, author, "SSR-second-public-moment", 1000)
	response := testRequest(a, "GET", "/following?limit=1&lang=en", "", reader.cookie, "")
	page := response.Body.String()
	if response.Code != 200 || !strings.Contains(page, "SSR-public-title") || !strings.Contains(page, "/posts/"+post) || !strings.Contains(page, "/users/"+author.user.ID) || !strings.Contains(page, "Following") {
		t.Fatal("SSR first screen lost real reading links or locale", response.Code, page)
	}
	doc := friendWebDocument(t, page)
	if injected := friendWebNodes(doc, func(n *nethtml.Node) bool {
		return n.Type == nethtml.ElementNode && friendWebHasAttr(n, "data-following-xss")
	}); len(injected) != 0 {
		t.Fatal("feed title escaped into executable HTML")
	}
	data := followingFeedBootstrap(t, page)
	items := data["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["title"] != "SSR-public-title "+attack || data["viewer_id"] != reader.user.ID || data["has_more"] != true {
		t.Fatal("bootstrap lost escaped content or paging state", data)
	}
	next := data["next_cursor"].(string)
	second := testRequest(a, "GET", "/following?limit=1&cursor="+url.QueryEscape(next), "", reader.cookie, "")
	secondData := followingFeedBootstrap(t, second.Body.String())
	if second.Code != 200 || secondData["items"].([]any)[0].(map[string]any)["id"] != "moment:"+moment || !strings.Contains(second.Body.String(), "SSR-public-moment") {
		t.Fatal("native continuation disagreed with API paging", second.Code, secondData)
	}
	filtered := testRequest(a, "GET", "/following?type=moment", "", reader.cookie, "")
	if filtered.Code != 200 || strings.Contains(filtered.Body.String(), "SSR-public-title") || !strings.Contains(filtered.Body.String(), "SSR-public-moment") {
		t.Fatal("native filter leaked another type", filtered.Code)
	}
	foreign := testRequest(a, "GET", "/following?limit=1&cursor="+url.QueryEscape(next), "", other.cookie, "")
	if foreign.Code != 400 || strings.Contains(foreign.Body.String(), "SSR-public-title") || strings.Contains(foreign.Body.String(), `id="following-feed-data"`) {
		t.Fatal("SSR accepted another reader's cursor", foreign.Code)
	}
	followingFeedFollow(t, a, reader, author, false)
	removed := testRequest(a, "GET", "/following", "", reader.cookie, "")
	if strings.Contains(removed.Body.String(), "SSR-public-title") || strings.Contains(removed.Body.String(), "SSR-public-moment") {
		t.Fatal("SSR retained content after unfollow")
	}
}

func TestFollowingFeedWebDiscoveryExcludesPrivateFeed(t *testing.T) {
	a := followingFeedApp(t)
	a.RegisterWeb()
	followingFeedIdentity(t, a, "owner")
	robots := testRequest(a, "GET", "/robots.txt", "", nil, "")
	sitemap := testRequest(a, "GET", "/sitemap.xml", "", nil, "")
	if robots.Code != 200 || !strings.Contains(robots.Body.String(), "Disallow: /following\n") || sitemap.Code != 200 || strings.Contains(sitemap.Body.String(), "/following") {
		t.Fatal("personal follow set entered crawl discovery", robots.Code, sitemap.Code)
	}
	page := testRequest(a, "GET", "/discover", "", nil, "")
	if page.Code != 200 || !strings.Contains(page.Body.String(), `href="/following"`) {
		t.Fatal("follow feed is not findable through native navigation", page.Code)
	}
}
