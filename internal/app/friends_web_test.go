package app

import (
	"encoding/xml"
	"html"
	"net/http"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

func friendWebDocument(t *testing.T, source string) *nethtml.Node {
	t.Helper()
	doc, err := nethtml.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func friendWebAttr(node *nethtml.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func friendWebHasAttr(node *nethtml.Node, key string) bool {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

func friendWebNodes(root *nethtml.Node, matches func(*nethtml.Node) bool) []*nethtml.Node {
	nodes := []*nethtml.Node{}
	var visit func(*nethtml.Node)
	visit = func(node *nethtml.Node) {
		if matches(node) {
			nodes = append(nodes, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	return nodes
}

func friendWebClass(root *nethtml.Node, class string) []*nethtml.Node {
	return friendWebNodes(root, func(node *nethtml.Node) bool {
		return node.Type == nethtml.ElementNode && strings.Contains(" "+friendWebAttr(node, "class")+" ", " "+class+" ")
	})
}

func friendWebText(node *nethtml.Node) string {
	var out strings.Builder
	for _, text := range friendWebNodes(node, func(node *nethtml.Node) bool { return node.Type == nethtml.TextNode }) {
		out.WriteString(text.Data)
	}
	return out.String()
}

func TestFriendsPublicSSRNativeLinksEscapingAndVisibility(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	a.RegisterWeb()
	empty := testRequest(a, "GET", "/friends", "", nil, "")
	if empty.Code != 200 || !strings.Contains(empty.Body.String(), "这里还没有友链") || !strings.Contains(empty.Body.String(), `href="/posts"`) {
		t.Fatal("empty public friend directory", empty.Code, empty.Body.String())
	}
	title := `<em>常去的网站</em> & Friends`
	description := "沿着链接，认识朋友。\n<script data-setting-xss>alert('no')</script>"
	settings := map[string]any{"enabled": true, "title": title, "description": description}
	if w := friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friend-settings", settings, 1, ""); w.Code != 200 {
		t.Fatal("settings fixture", w.Code, w.Body.String())
	}
	late := friendFixture(t, a, owner, csrf, "Late-public-site", 10, true)
	friendFixture(t, a, owner, csrf, "Private-friend-sentinel", -100, false)
	name := `<script data-friend-xss>alert("no")</script>`
	text := "<img src=x onerror=alert('no')>\nPlain & quoted \"text\""
	target := "https://example.com/read?one=1&two=%22%3E%3Cscript%3E#end"
	avatar := "https://images.example.com/avatar.png?x=1&y=2"
	input := friendInput{Name: name, URL: target, Description: text, AvatarURL: avatar, SortOrder: -2, Visible: true}
	if w := friendCall(t, a, owner, csrf, "POST", "/api/v1/studio/friends", input, 0, RandID()); w.Code != 201 {
		t.Fatal("public escaping fixture", w.Code, w.Body.String())
	}
	tied := friendFixture(t, a, owner, csrf, "Tied-public-site", 10, true)
	for _, cookie := range []*http.Cookie{nil, owner} {
		response := testRequest(a, "GET", "/friends?visible=false&scope=admin", "", cookie, "")
		body := response.Body.String()
		if response.Code != 200 || !strings.Contains(response.Header().Get("Content-Type"), "text/html") || response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("public friend SSR response", response.Code, response.Header())
		}
		if strings.Contains(body, "Private-friend-sentinel") || !strings.Contains(body, html.EscapeString(name)) || !strings.Contains(body, html.EscapeString(text)) || !strings.Contains(body, html.EscapeString(target)) {
			t.Fatal("SSR escaped text or public projection failed")
		}
		doc := friendWebDocument(t, body)
		entries := friendWebClass(doc, "friend-entry")
		if len(entries) != 3 {
			t.Fatal("native SSR friend entries absent", len(entries))
		}
		for i, expected := range []struct{ name, url string }{{name, target}, {late["name"].(string), late["url"].(string)}, {tied["name"].(string), tied["url"].(string)}} {
			entry := entries[i]
			rel := " " + friendWebAttr(entry, "rel") + " "
			if entry.Data != "a" || friendWebAttr(entry, "href") != expected.url || friendWebAttr(entry, "target") != "_blank" || !strings.Contains(rel, " noopener ") || !strings.Contains(rel, " noreferrer ") || !strings.Contains(friendWebText(entry), expected.name) {
				t.Fatal("native new-tab link, no-JS contents, or stable ordering failed", i)
			}
			if friendWebAttr(entry, "aria-label") != expected.name+"（在新标签页打开）" {
				t.Fatal("new-tab announcement missing", i)
			}
		}
		injected := friendWebNodes(doc, func(node *nethtml.Node) bool {
			return node.Type == nethtml.ElementNode && (friendWebHasAttr(node, "data-setting-xss") || friendWebHasAttr(node, "data-friend-xss") || friendWebHasAttr(node, "onerror"))
		})
		if len(injected) != 0 {
			t.Fatal("friend text injected HTML elements or handlers")
		}
		headings := friendWebNodes(doc, func(node *nethtml.Node) bool { return node.Type == nethtml.ElementNode && node.Data == "h1" })
		titles := friendWebNodes(doc, func(node *nethtml.Node) bool { return node.Type == nethtml.ElementNode && node.Data == "title" })
		meta := friendWebNodes(doc, func(node *nethtml.Node) bool {
			return node.Type == nethtml.ElementNode && node.Data == "meta" && friendWebAttr(node, "name") == "description"
		})
		if len(headings) != 1 || friendWebText(headings[0]) != title || len(titles) != 1 || friendWebText(titles[0]) != title+" · Painter" || len(meta) != 1 || friendWebAttr(meta[0], "content") != description {
			t.Fatal("SSR page title/description were lost or not escaped")
		}
		images := friendWebNodes(entries[0], func(node *nethtml.Node) bool { return node.Type == nethtml.ElementNode && node.Data == "img" })
		if len(images) != 1 || friendWebAttr(images[0], "src") != avatar || friendWebAttr(images[0], "referrerpolicy") != "no-referrer" || friendWebAttr(images[0], "loading") != "lazy" {
			t.Fatal("remote avatar isolation attributes or escaped source missing")
		}
	}
}

func TestFriendsPublicEntryPointsAndSitemapFollowPageVisibility(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	a.RegisterWeb()
	friendFixture(t, a, owner, csrf, "Public-link-not-a-disabled-leak", 0, true)
	for _, enabled := range []bool{true, false, true} {
		settings := jsonMap(t, friendCall(t, a, owner, csrf, "GET", "/api/v1/studio/friend-settings", nil, 0, ""))
		body := map[string]any{"enabled": enabled, "title": "友链", "description": "朋友们的网站"}
		if w := friendCall(t, a, owner, csrf, "PUT", "/api/v1/studio/friend-settings", body, int64(settings["version"].(float64)), ""); w.Code != 200 {
			t.Fatal("toggle friend directory", w.Code, w.Body.String())
		}
		for _, path := range []string{"/", "/about"} {
			page := testRequest(a, "GET", path, "", nil, "")
			if page.Code != 200 {
				t.Fatal("public entry point", path, page.Code)
			}
			doc := friendWebDocument(t, page.Body.String())
			links := friendWebNodes(doc, func(node *nethtml.Node) bool {
				return node.Type == nethtml.ElementNode && node.Data == "a" && friendWebAttr(node, "href") == "/friends"
			})
			if (len(links) == 1) != enabled || (!enabled && len(links) != 0) {
				t.Fatal("friend entry did not follow visibility", path, enabled, len(links))
			}
			for _, class := range []string{"site-header", "site-footer"} {
				areas := friendWebClass(doc, class)
				want := 0
				if enabled && class == "site-footer" {
					want = 1
				}
				if len(areas) != 1 || len(friendWebNodes(areas[0], func(node *nethtml.Node) bool {
					return node.Type == nethtml.ElementNode && node.Data == "a" && friendWebAttr(node, "href") == "/friends"
				})) != want {
					t.Fatal("friend entry must follow visibility in footer only", path, class, enabled)
				}
			}
		}
		sitemap := testRequest(a, "GET", "/sitemap.xml", "", nil, "")
		var parsed struct {
			URLs []struct {
				Loc string `xml:"loc"`
			} `xml:"url"`
		}
		if err := xml.Unmarshal(sitemap.Body.Bytes(), &parsed); sitemap.Code != 200 || err != nil {
			t.Fatal("sitemap unavailable", sitemap.Code, err)
		}
		count := 0
		for _, location := range parsed.URLs {
			if location.Loc == a.Config.PublicBaseURL+"/friends" {
				count++
			}
		}
		if (count == 1) != enabled || (!enabled && count != 0) {
			t.Fatal("sitemap friend route did not follow visibility", enabled, count)
		}
		public := testRequest(a, "GET", "/friends", "", nil, "")
		if enabled && (public.Code != 200 || !strings.Contains(public.Body.String(), "Public-link-not-a-disabled-leak")) {
			t.Fatal("enabled directory lost original links", public.Code)
		}
		if !enabled && (public.Code != 404 || strings.Contains(public.Body.String(), "Public-link-not-a-disabled-leak") || !strings.Contains(public.Body.String(), `name="robots" content="noindex,nofollow"`)) {
			t.Fatal("disabled directory leaked content or remained indexed", public.Code)
		}
	}
}

func TestFriendsWorkbenchServerPermissions(t *testing.T) {
	a := testApp(t)
	_, owner, _ := testIdentity(t, a, "owner")
	_, reader, _ := testIdentity(t, a, "reader")
	_, author, _ := testIdentity(t, a, "author")
	a.RegisterWeb()
	for _, path := range []string{"/studio/friends", "/studio/friends/"} {
		for _, actor := range []struct {
			name   string
			cookie *http.Cookie
			status int
		}{{"anonymous", nil, 303}, {"reader", reader, 403}, {"author", author, 403}, {"owner", owner, 200}} {
			got := testRequest(a, "GET", path, "", actor.cookie, "")
			if got.Code != actor.status {
				t.Fatal("friend workbench route permission", actor.name, path, got.Code)
			}
			if actor.cookie == nil && !strings.HasPrefix(got.Header().Get("Location"), "/login?return_to=") {
				t.Fatal("anonymous workbench did not redirect to local login")
			}
			if actor.status == 200 && !strings.Contains(got.Body.String(), `id="workbench"`) {
				t.Fatal("owner did not receive workbench shell")
			}
		}
	}
}
