package app

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"strconv"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

type rssTestLink struct {
	XMLName xml.Name
	Text    string `xml:",chardata"`
	Href    string `xml:"href,attr"`
	Rel     string `xml:"rel,attr"`
	Type    string `xml:"type,attr"`
}

type rssTestItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	GUID        string `xml:"guid"`
	Date        string `xml:"pubDate"`
}

type rssTestFeed struct {
	XMLName xml.Name
	Version string `xml:"version,attr"`
	Channel struct {
		Title       string        `xml:"title"`
		Description string        `xml:"description"`
		Links       []rssTestLink `xml:"link"`
		Items       []rssTestItem `xml:"item"`
	} `xml:"channel"`
}

func rssTestRead(a *App, method, condition string) *httptest.ResponseRecorder {
	// The request Host must never become the feed's canonical origin.
	r := httptest.NewRequest(method, "http://untrusted-host.invalid/rss.xml", nil)
	if condition != "" {
		r.Header.Set("If-None-Match", condition)
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}

func rssTestDecode(t *testing.T, response *httptest.ResponseRecorder) rssTestFeed {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("RSS HTTP %d: %s", response.Code, response.Body.String())
	}
	var feed rssTestFeed
	if err := xml.Unmarshal(response.Body.Bytes(), &feed); err != nil {
		t.Fatalf("RSS is not well-formed XML: %v\n%s", err, response.Body.String())
	}
	if feed.XMLName.Local != "rss" || feed.Version != "2.0" {
		t.Fatal("RSS 2.0 envelope missing", feed.XMLName, feed.Version)
	}
	return feed
}

func rssTestSetEnabled(t *testing.T, a *App, enabled bool) {
	t.Helper()
	if _, err := a.Pool.Exec(context.Background(), "UPDATE site_settings SET rss_enabled=$1 WHERE id=1", enabled); err != nil {
		t.Fatal(err)
	}
}

func rssTestDiscovery(t *testing.T, a *App, enabled bool, name string) {
	t.Helper()
	response := testRequest(a, http.MethodGet, "/about", "", nil, "")
	if response.Code != http.StatusOK {
		t.Fatal("public page unavailable during RSS discovery check", response.Code, response.Body.String())
	}
	doc := friendWebDocument(t, response.Body.String())
	footers := friendWebClass(doc, "site-footer")
	if len(footers) != 1 {
		t.Fatal("public page must retain its footer", len(footers))
	}
	links := friendWebNodes(footers[0], func(n *nethtml.Node) bool {
		return n.Type == nethtml.ElementNode && n.Data == "a" && friendWebAttr(n, "href") == "/rss.xml"
	})
	alternates := friendWebNodes(doc, func(n *nethtml.Node) bool {
		return n.Type == nethtml.ElementNode && n.Data == "link" && friendWebAttr(n, "rel") == "alternate" && friendWebAttr(n, "type") == "application/rss+xml"
	})
	if !enabled {
		if len(links) != 0 || len(alternates) != 0 {
			t.Fatal("disabled RSS remains discoverable in public chrome", len(links), len(alternates))
		}
		return
	}
	if len(links) != 1 || strings.TrimSpace(friendWebText(links[0])) != "RSS" || friendWebAttr(links[0], "type") != "application/rss+xml" || friendWebAttr(links[0], "aria-label") == "" {
		t.Fatal("footer RSS is not a labelled native feed link", links)
	}
	if len(alternates) != 1 || friendWebAttr(alternates[0], "href") != "/rss.xml" || friendWebAttr(alternates[0], "title") != name+" RSS" {
		t.Fatal("enabled RSS autodiscovery missing or not tied to site identity", alternates)
	}
}

func TestRSSHTTPRevalidationSwitchAndDiscovery(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterWeb()
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := rssTestRead(a, method, "*")
		if response.Code != http.StatusNotFound || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("ETag") != "" || method == http.MethodHead && response.Body.Len() != 0 {
			t.Fatal("disabled RSS was cached, revalidated or returned a HEAD body", method, response.Code, response.Header(), response.Body.String())
		}
	}
	rssTestDiscovery(t, a, false, "")
	name, tagline, seo := `站点 <标题> & "引号"`, `介绍 </description><item>不能注入</item> & 中文`, "搜索摘要作为订阅说明"
	if _, err := a.Pool.Exec(context.Background(), "UPDATE site_profile SET display_name=$1,tagline=$2,seo_description=$3 WHERE id=1", name, tagline, seo); err != nil {
		t.Fatal(err)
	}
	rssTestSetEnabled(t, a, true)
	response := rssTestRead(a, http.MethodGet, "")
	feed := rssTestDecode(t, response)
	if feed.Channel.Title != name || feed.Channel.Description != tagline || len(feed.Channel.Items) != 0 {
		t.Fatal("empty feed lost escaped site identity or invented items", feed)
	}
	regular, self := 0, 0
	for _, link := range feed.Channel.Links {
		if link.XMLName.Space == "" && link.Text == a.Config.PublicBaseURL {
			regular++
		}
		if link.XMLName.Space == "http://www.w3.org/2005/Atom" && link.Href == a.Config.PublicBaseURL+"/rss.xml" && link.Rel == "self" && link.Type == "application/rss+xml" {
			self++
		}
	}
	if regular != 1 || self != 1 || strings.Contains(response.Body.String(), "untrusted-host.invalid") {
		t.Fatal("feed canonical or namespaced Atom self link invalid", feed.Channel.Links)
	}
	if response.Header().Get("Content-Type") != "application/rss+xml; charset=utf-8" || response.Header().Get("Cache-Control") != "public, max-age=0, must-revalidate" || response.Header().Get("Content-Length") != strconv.Itoa(response.Body.Len()) || response.Header().Get("Last-Modified") != "" {
		t.Fatal("RSS representation headers do not match the feed contract", response.Header())
	}
	etag := response.Header().Get("ETag")
	if len(etag) < 3 || !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) || strings.HasPrefix(etag, "W/") {
		t.Fatal("feed ETag must be a strong quoted validator", etag)
	}
	rssTestDiscovery(t, a, true, name)
	head := rssTestRead(a, http.MethodHead, "")
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatal("RSS HEAD must return GET metadata without the XML body", head.Code, head.Body.String())
	}
	for _, header := range []string{"Content-Type", "Content-Length", "Cache-Control", "ETag"} {
		if head.Header().Get(header) != response.Header().Get(header) {
			t.Errorf("RSS GET/HEAD %s differs: %q / %q", header, response.Header().Get(header), head.Header().Get(header))
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, condition := range []string{etag, "W/" + etag, `"different", W/` + etag, "*"} {
			cached := rssTestRead(a, method, condition)
			if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 || cached.Header().Get("ETag") != etag || cached.Header().Get("Cache-Control") != response.Header().Get("Cache-Control") {
				t.Errorf("RSS %s validator %q did not revalidate without a body: %d %v %s", method, condition, cached.Code, cached.Header(), cached.Body.String())
			}
		}
	}
	if fresh := rssTestRead(a, http.MethodGet, `"different"`); fresh.Code != http.StatusOK || fresh.Body.String() != response.Body.String() || fresh.Header().Get("ETag") != etag {
		t.Fatal("nonmatching validator changed or suppressed a stable feed", fresh.Code, fresh.Header())
	}
	for _, fallback := range []struct{ tagline, seo, want string }{{"", seo, seo}, {"", "", name}} {
		if _, err := a.Pool.Exec(context.Background(), "UPDATE site_profile SET tagline=$1,seo_description=$2 WHERE id=1", fallback.tagline, fallback.seo); err != nil {
			t.Fatal(err)
		}
		fresh := rssTestRead(a, http.MethodGet, etag)
		if got := rssTestDecode(t, fresh); got.Channel.Description != fallback.want || fresh.Header().Get("ETag") == etag {
			t.Fatal("changed site metadata was incorrectly cached or description fallback failed", got.Channel.Description, fresh.Header())
		}
		etag = fresh.Header().Get("ETag")
	}
	rssTestSetEnabled(t, a, false)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		closed := rssTestRead(a, method, etag)
		if closed.Code != http.StatusNotFound || closed.Header().Get("Cache-Control") != "no-store" || closed.Header().Get("ETag") != "" || method == http.MethodHead && closed.Body.Len() != 0 {
			t.Fatal("closing RSS reused a previously public representation", method, closed.Code, closed.Header())
		}
	}
	rssTestDiscovery(t, a, false, name)
	// Unavailability must not be mistaken for a cache hit or negatively cached.
	a.Pool.Close()
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		unavailable := rssTestRead(a, method, etag)
		if unavailable.Code != http.StatusServiceUnavailable || unavailable.Header().Get("Cache-Control") != "no-store" || unavailable.Header().Get("ETag") != "" || method == http.MethodHead && unavailable.Body.Len() != 0 {
			t.Fatal("unavailable RSS was cached or returned a HEAD body", method, unavailable.Code, unavailable.Header())
		}
	}
}

func TestRSSUsesCurrentImmutablePublicationAndInvalidatesRemovedItems(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterWeb()
	_, author, csrf := testIdentity(t, a, "author")
	rssTestSetEnabled(t, a, true)
	create := func(title, summary string, publish bool) (string, map[string]any) {
		created := contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": title, "summary": summary, "markdown": "只属于文章全文的正文，不应进入摘要订阅"}, 0, ""), http.StatusCreated)
		id := created["id"].(string)
		if !publish {
			return id, nil
		}
		published := contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), http.StatusOK)
		return id, published
	}
	title, summary := `公开 <title> & "Unicode 中文"`, `摘要 </description><item>不应成为条目</item> & "说明"`
	id, published := create(title, summary, true)
	deleted, _ := create("待删除的发布条目", "删除后应消失", true)
	withdrawn, _ := create("待撤回的发布条目", "撤回后应消失", true)
	create("机密草稿标题", "机密草稿摘要", false)
	initial := rssTestRead(a, http.MethodGet, "")
	if feed := rssTestDecode(t, initial); len(feed.Channel.Items) != 3 {
		t.Fatal("RSS included a draft or lost a published item", feed.Channel.Items)
	}
	contentWant(t, contentCall(t, a, author, csrf, "DELETE", "/api/v1/studio/posts/"+deleted, nil, 2, ""), http.StatusOK)
	afterDelete := rssTestRead(a, http.MethodGet, initial.Header().Get("ETag"))
	if feed := rssTestDecode(t, afterDelete); len(feed.Channel.Items) != 2 || afterDelete.Header().Get("ETag") == initial.Header().Get("ETag") {
		t.Fatal("deleted publication was reused by conditional RSS", feed.Channel.Items, afterDelete.Header())
	}
	contentWant(t, contentCall(t, a, author, csrf, "DELETE", "/api/v1/studio/posts/"+withdrawn+"/publication", nil, 2, ""), http.StatusOK)
	current := rssTestRead(a, http.MethodGet, afterDelete.Header().Get("ETag"))
	feed := rssTestDecode(t, current)
	if len(feed.Channel.Items) != 1 || current.Header().Get("ETag") == afterDelete.Header().Get("ETag") {
		t.Fatal("withdrawal reused a stale RSS representation", feed.Channel.Items, current.Header())
	}
	entry := feed.Channel.Items[0]
	canonical := a.Config.PublicBaseURL + "/posts/" + id
	if entry.Title != title || entry.Description != summary || entry.Link != canonical || entry.GUID != canonical {
		t.Fatal("RSS escaped text did not round trip or canonical identity changed", entry)
	}
	if _, err := mail.ParseDate(entry.Date); err != nil {
		t.Fatal("RSS pubDate is not a compatible Internet message date", entry.Date, err)
	}
	for _, forbidden := range []string{"机密草稿", "待删除的发布条目", "待撤回的发布条目", "只属于文章全文的正文"} {
		if strings.Contains(current.Body.String(), forbidden) {
			t.Errorf("RSS leaked unpublished/removed/full article data %q", forbidden)
		}
	}
	draft := map[string]any{"title": "尚未发布的第二版", "summary": "第二版摘要", "markdown": "第二版正文", "expected_draft_version": 1}
	saved := contentWant(t, contentCall(t, a, author, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", draft, contentNumber(published, "version"), ""), http.StatusOK)
	contentWant(t, contentCall(t, a, author, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", draft, contentNumber(published, "version"), ""), http.StatusConflict)
	if cached := rssTestRead(a, http.MethodGet, current.Header().Get("ETag")); cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
		t.Fatal("draft autosave or rejected CAS write changed the published feed", cached.Code, cached.Body.String())
	}
	if stable := rssTestRead(a, http.MethodGet, ""); stable.Body.String() != current.Body.String() {
		t.Fatal("draft content entered the RSS publication snapshot", stable.Body.String())
	}
	republished := contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 2}, contentNumber(saved, "version"), ""), http.StatusOK)
	updated := rssTestRead(a, http.MethodGet, current.Header().Get("ETag"))
	updatedFeed := rssTestDecode(t, updated)
	if len(updatedFeed.Channel.Items) != 1 || updatedFeed.Channel.Items[0].Title != draft["title"] || updatedFeed.Channel.Items[0].Description != draft["summary"] || updatedFeed.Channel.Items[0].GUID != canonical || updated.Header().Get("ETag") == current.Header().Get("ETag") {
		t.Fatal("explicit republish lost stable item identity or retained old content", updatedFeed.Channel.Items, updated.Header())
	}
	contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/studio/posts/"+id+"/restore", map[string]any{"revision_id": published["revision_id"]}, contentNumber(republished, "version"), ""), http.StatusOK)
	if cached := rssTestRead(a, http.MethodGet, updated.Header().Get("ETag")); cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
		t.Fatal("restoring an old revision implicitly changed the public feed", cached.Code, cached.Body.String())
	}
}

func TestRSSBoundsRecentArticlesAndOrdersTimestampTies(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterWeb()
	_, author, csrf := testIdentity(t, a, "author")
	rssTestSetEnabled(t, a, true)
	ids := make([]string, 0, 22)
	for i := range 22 {
		id := seriesTestPost(t, a, author, csrf, fmt.Sprintf("订阅排序条目 %02d", i), true)
		ids = append(ids, id)
		// Each pair shares a publication timestamp to exercise deterministic ID
		// ordering without depending on wall-clock timing or adding test sleeps.
		stamp := int64(1700000000000 + i/2)
		if _, err := a.Pool.Exec(context.Background(), `WITH changed AS (UPDATE posts SET first_published_at_ms=$2,version=version+1 WHERE id=$1) UPDATE post_publications SET published_at_ms=$2 WHERE post_id=$1`, id, stamp); err != nil {
			t.Fatal(err)
		}
	}
	response := rssTestRead(a, http.MethodGet, "")
	feed := rssTestDecode(t, response)
	if len(feed.Channel.Items) != 20 {
		t.Fatal("RSS must contain only the most recent twenty published articles", len(feed.Channel.Items))
	}
	for i, entry := range feed.Channel.Items {
		if want := a.Config.PublicBaseURL + "/posts/" + ids[len(ids)-1-i]; entry.Link != want {
			t.Errorf("RSS recent/timestamp-tie ordering at %d: got %q, want %q", i, entry.Link, want)
		}
	}
	if again := rssTestRead(a, http.MethodGet, ""); again.Body.String() != response.Body.String() || again.Header().Get("ETag") != response.Header().Get("ETag") {
		t.Fatal("unchanged publication set produced unstable RSS bytes or validator")
	}
}
