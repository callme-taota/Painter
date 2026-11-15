package app

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/url"
	"slices"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
	"painter-next/web"
)

// Render the embedded production templates directly. These contracts need no
// App, sessions, providers, or database, and exercise the no-JavaScript HTML.
func v31NavigationTemplates(t *testing.T) *template.Template {
	t.Helper()
	templates, err := template.New("page").Funcs(template.FuncMap{
		"str": webString, "date": webDate, "dateTime": webDateTime,
		"initial": webInitial, "gameHours": gameHours,
		"rank":    func(i int) int { return i + 1 },
		"phrases": func(v any) []string { return strings.SplitAfter(webString(v), "，") },
		"hasTag": func(q url.Values, id any) bool {
			return slices.Contains(q["tag_id"], webString(id))
		},
		"linkHost": func(v any) string {
			u, _ := url.Parse(webString(v))
			if u == nil {
				return ""
			}
			return strings.TrimPrefix(u.Host, "www.")
		},
	}).Funcs(localeTemplateFuncs(defaultLocale)).ParseFS(web.FS, "templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	return templates
}

func v31NavigationPage(kind string, controls floatingControls, creator, friends bool) webPage {
	return webPage{
		Kind: kind, Title: "Navigation contract", Creator: creator, Query: url.Values{},
		Site: map[string]any{
			"display_name": "Painter", "floating_controls": controls,
			"friend_links_enabled": friends,
		},
		Post: map[string]any{
			"id": "public-post", "title": "Public article",
			"author":       map[string]any{"id": "author", "display_name": "Author"},
			"date_display": map[string]bool{},
		},
	}
}

func v31NavigationDocument(t *testing.T, templates *template.Template, page webPage) *nethtml.Node {
	t.Helper()
	var output bytes.Buffer
	if err := templates.ExecuteTemplate(&output, "page", page); err != nil {
		t.Fatal(err)
	}
	return friendWebDocument(t, output.String())
}

func v31NavigationClass(t *testing.T, root *nethtml.Node, class string) *nethtml.Node {
	t.Helper()
	nodes := friendWebClass(root, class)
	if len(nodes) != 1 {
		t.Fatalf("want one %s, got %d", class, len(nodes))
	}
	return nodes[0]
}

func v31NavigationLinks(root *nethtml.Node) []*nethtml.Node {
	return friendWebNodes(root, func(node *nethtml.Node) bool {
		return node.Type == nethtml.ElementNode && node.Data == "a"
	})
}

func v31AssertBrowseLinks(t *testing.T, root *nethtml.Node, kind string) {
	t.Helper()
	want := map[string]string{"/": "首页", "/products": "产品", "/posts": "文章", "/series": "专栏", "/moments": "动态", "/phrases": "片语", "/games": "游戏墙"}
	current := map[string]string{
		"home": "/", "products": "/products", "product": "/products",
		"posts": "/posts", "article": "/posts", "series-list": "/series", "series": "/series",
		"moments": "/moments", "moment": "/moments", "phrases": "/phrases", "games": "/games",
	}[kind]
	links := v31NavigationLinks(root)
	if len(links) != len(want) {
		t.Fatalf("public browsing should expose seven native links, got %d", len(links))
	}
	for _, link := range links {
		href := friendWebAttr(link, "href")
		label, ok := want[href]
		if !ok {
			t.Fatalf("unexpected or duplicate browsing destination %q", href)
		}
		if friendWebAttr(link, "aria-label") != label && strings.TrimSpace(friendWebText(link)) != label {
			t.Errorf("%s lacks its accessible content label %q", href, label)
		}
		if friendWebAttr(link, "target") != "" || friendWebHasAttr(link, "download") || friendWebAttr(link, "role") == "button" {
			t.Errorf("%s lost native same-tab navigation semantics", href)
		}
		if friendWebHasAttr(link, "hidden") || friendWebAttr(link, "aria-hidden") == "true" || friendWebAttr(link, "tabindex") == "-1" {
			t.Errorf("%s cannot be reached through the native disclosure", href)
		}
		if (friendWebAttr(link, "aria-current") == "page") != (href == current) {
			t.Errorf("%s: browsing destination %s has incorrect current-page state", kind, href)
		}
		delete(want, href)
	}
}

// The same native disclosures serve every viewport. Configured page tools and
// install actions stay inside More rather than adding permanent Dock controls.
func v31AssertCompactDock(t *testing.T, dock *nethtml.Node) (browse, more *nethtml.Node) {
	t.Helper()
	browse = v31NavigationClass(t, dock, "dock-browse")
	more = v31NavigationClass(t, dock, "dock-more")
	for _, menu := range []struct {
		node  *nethtml.Node
		label string
	}{{browse, "浏览内容"}, {more, "更多操作"}} {
		if menu.node.Data != "details" || menu.node.Parent != dock || friendWebHasAttr(menu.node, "open") {
			t.Fatal("Browse and More must be closed native disclosures at the Dock root")
		}
		var summary *nethtml.Node
		for child := menu.node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == nethtml.ElementNode {
				summary = child
				break
			}
		}
		if summary == nil || summary.Data != "summary" || friendWebAttr(summary, "aria-label") != menu.label {
			t.Fatalf("Dock disclosure must retain its native accessible summary %q", menu.label)
		}
	}
	back := friendWebNodes(dock, func(node *nethtml.Node) bool {
		return node.Type == nethtml.ElementNode && friendWebHasAttr(node, "data-app-back")
	})
	if len(back) != 1 || back[0].Parent != dock || back[0].Data != "button" || friendWebAttr(back[0], "type") != "button" || !friendWebHasAttr(back[0], "hidden") || friendWebAttr(back[0], "aria-label") == "" {
		t.Fatal("app Back must be a single accessible, initially hidden contextual button")
	}
	for child := dock.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != nethtml.ElementNode || child == browse || child == more || child == back[0] {
			continue
		}
		if len(friendWebNodes(child, func(node *nethtml.Node) bool {
			return node.Type == nethtml.ElementNode && slices.Contains([]string{"a", "button", "details", "summary", "input", "select"}, node.Data)
		})) != 0 {
			t.Error("Dock root contains permanent controls beyond Browse, More and contextual app Back")
		}
	}
	if len(friendWebClass(dock, "dock-navigation")) != 0 {
		t.Error("a second permanent browsing rail duplicates the compact Browse control")
	}
	navs := friendWebNodes(browse, func(node *nethtml.Node) bool {
		return node.Type == nethtml.ElementNode && node.Data == "nav" && friendWebAttr(node, "aria-label") == "内容导航"
	})
	if len(navs) != 1 {
		t.Error("Browse must contain a named native content navigation region")
	}
	for _, attribute := range []string{"data-pwa-install", "data-pwa-help"} {
		buttons := friendWebNodes(dock, func(node *nethtml.Node) bool {
			return node.Type == nethtml.ElementNode && friendWebHasAttr(node, attribute)
		})
		if len(buttons) != 1 || buttons[0].Data != "button" || len(friendWebNodes(more, func(node *nethtml.Node) bool { return node == buttons[0] })) != 1 {
			t.Fatalf("%s must be a single action reachable through More", attribute)
		}
		if attribute == "data-pwa-install" && !friendWebHasAttr(buttons[0], "hidden") {
			t.Error("install action must remain hidden until browser support is known")
		}
	}
	return browse, more
}

func TestV31HeaderAndDockKeepNativePublicNavigation(t *testing.T) {
	templates := v31NavigationTemplates(t)
	for _, kind := range []string{"home", "products", "product", "article", "posts", "series-list", "series", "moments", "moment", "phrases", "games", "about", "search", "error"} {
		t.Run(kind, func(t *testing.T) {
			doc := v31NavigationDocument(t, templates, v31NavigationPage(kind, defaultFloatingControls(), false, true))
			header := v31NavigationClass(t, doc, "site-header")
			links := v31NavigationLinks(header)
			if len(links) != 2 {
				t.Fatalf("top bar must contain only home/logo and search links, got %d", len(links))
			}
			for index, href := range []string{"/", "/search"} {
				if friendWebAttr(links[index], "href") != href {
					t.Errorf("top link %d: want %q, got %q", index, href, friendWebAttr(links[index], "href"))
				}
			}
			if len(friendWebNodes(links[0], func(node *nethtml.Node) bool {
				return node.Type == nethtml.ElementNode && node.Data == "img"
			})) != 1 || friendWebAttr(links[0], "aria-label") == "" {
				t.Error("home link must retain the logo and an accessible name")
			}
			if !friendWebHasAttr(links[1], "data-search-open") || friendWebAttr(links[1], "aria-label") == "" {
				t.Error("search must retain both its native href and enhancement trigger")
			}
			if extra := friendWebNodes(header, func(node *nethtml.Node) bool {
				return node.Type == nethtml.ElementNode && slices.Contains([]string{"button", "details", "form", "select"}, node.Data)
			}); len(extra) != 0 {
				t.Error("top bar contains controls beyond logo and search")
			}

			dock := v31NavigationClass(t, doc, "site-dock")
			browse, _ := v31AssertCompactDock(t, dock)
			// Responsive placement and 44px targets belong to browser checks;
			// these links remain reachable with native details and no JavaScript.
			v31AssertBrowseLinks(t, browse, kind)
		})
	}
}

func TestV31EmptyToolsPreservePublicBrowsing(t *testing.T) {
	templates := v31NavigationTemplates(t)
	raw, err := normalizeFloatingControls(json.RawMessage(`{"home":[],"article":[],"other":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	var controls floatingControls
	if err := json.Unmarshal(raw, &controls); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"home", "article", "about"} {
		for _, creator := range []bool{false, true} {
			name := kind + "/visitor"
			if creator {
				name = kind + "/creator"
			}
			t.Run(name, func(t *testing.T) {
				doc := v31NavigationDocument(t, templates, v31NavigationPage(kind, controls, creator, true))
				dock := v31NavigationClass(t, doc, "site-dock")
				browse, _ := v31AssertCompactDock(t, dock)
				v31AssertBrowseLinks(t, browse, kind)
				tools := v31NavigationClass(t, dock, "dock-tools")
				if len(friendWebNodes(tools, func(node *nethtml.Node) bool {
					return node.Type == nethtml.ElementNode && slices.Contains([]string{"a", "button", "summary", "input"}, node.Data) && !friendWebHasAttr(node, "data-pwa-install") && !friendWebHasAttr(node, "data-pwa-help") && !friendWebHasAttr(node, "data-locale-switch")
				})) != 0 {
					t.Error("an explicit empty array must hide configured tools")
				}
			})
		}
	}
}

func TestV31CreateRequiresConfiguredTrustedCreator(t *testing.T) {
	templates := v31NavigationTemplates(t)
	for _, configured := range []bool{false, true} {
		controls := floatingControls{"home": {}, "article": {}, "other": {}}
		if configured {
			for scene := range controls {
				controls[scene] = []string{"create"}
			}
		}
		for _, actor := range []struct {
			name    string
			creator bool
		}{
			// Creator is the trusted render input, resolved from Principal by
			// RegisterWeb. Session-to-role authorization needs integration tests;
			// these fixtures test the template's configuration AND creator gate.
			{"anonymous", false}, {"reader", false}, {"author", true}, {"owner", true},
		} {
			for _, kind := range []string{"home", "article", "about"} {
				name := "unconfigured/" + actor.name + "/" + kind
				if configured {
					name = "configured/" + actor.name + "/" + kind
				}
				t.Run(name, func(t *testing.T) {
					doc := v31NavigationDocument(t, templates, v31NavigationPage(kind, controls, actor.creator, true))
					dock := v31NavigationClass(t, doc, "site-dock")
					create := friendWebClass(dock, "floating-create")
					if !configured || !actor.creator {
						if len(create) != 0 {
							t.Error("creation menu leaked without configuration and creator authorization")
						}
						for _, link := range v31NavigationLinks(dock) {
							if strings.HasPrefix(friendWebAttr(link, "href"), "/studio/") {
								t.Error("hidden creation links remain in the public Dock HTML")
							}
						}
						return
					}
					if len(create) != 1 || create[0].Data != "details" {
						t.Fatal("authorized configured creator lacks the native creation menu")
					}
					targets := map[string]bool{}
					for _, link := range v31NavigationLinks(create[0]) {
						targets[friendWebAttr(link, "href")] = true
					}
					if len(targets) != 2 || !targets["/studio/posts/new"] || !targets["/studio/moments?compose=1#moment-compose"] {
						t.Errorf("creation menu lost real article/moment destinations: %v", targets)
					}
				})
			}
		}
	}
}

func TestV31ToolsUseTheCurrentSceneAndArticleLikeGuard(t *testing.T) {
	templates := v31NavigationTemplates(t)
	controls := floatingControls{
		"home": {"theme"}, "article": {"bottom", "like"}, "other": {"account"},
	}
	for _, sample := range []struct {
		kind, wantClass, wantAction, wantHref string
	}{
		{kind: "home", wantClass: "floating-theme"},
		{kind: "article", wantClass: "floating-like", wantAction: "bottom"},
		{kind: "posts", wantHref: "/me"},
		{kind: "moment", wantHref: "/me"},
		{kind: "about", wantHref: "/me"},
	} {
		t.Run(sample.kind, func(t *testing.T) {
			page := v31NavigationPage(sample.kind, controls, false, false)
			page.Authenticated = true // V33: article likes are authenticated tools.
			doc := v31NavigationDocument(t, templates, page)
			tools := v31NavigationClass(t, doc, "dock-tools")
			if (len(friendWebClass(tools, "floating-theme")) == 1) != (sample.wantClass == "floating-theme") {
				t.Error("theme control did not follow the current scene")
			}
			if (len(friendWebClass(tools, "floating-like")) == 1) != (sample.kind == "article") {
				t.Error("like control did not follow the article scene")
			}
			if actions := friendWebNodes(tools, func(node *nethtml.Node) bool {
				return friendWebAttr(node, "data-floating-action") == "bottom"
			}); (len(actions) == 1) != (sample.wantAction == "bottom") {
				t.Error("article-bottom control leaked from another scene")
			}
			links := friendWebNodes(tools, func(node *nethtml.Node) bool {
				return node.Type == nethtml.ElementNode && node.Data == "a" && !friendWebHasAttr(node, "data-locale-switch")
			})
			if sample.wantHref == "" {
				if len(links) != 0 {
					t.Error("account link leaked from the other scene")
				}
			} else if len(links) != 1 || friendWebAttr(links[0], "href") != sample.wantHref {
				t.Error("other-scene account destination missing")
			}
		})
	}
	// Even malformed/stale stored configuration must not render an article-only
	// action on another scene. The normal settings validator also rejects it.
	for _, kind := range []string{"home", "about"} {
		invalid := floatingControls{"home": {"like"}, "other": {"like"}}
		doc := v31NavigationDocument(t, templates, v31NavigationPage(kind, invalid, false, false))
		if len(friendWebClass(doc, "floating-like")) != 0 {
			t.Errorf("article-like guard failed on %s", kind)
		}
	}
}

func TestV31FriendsStayInTheFooterAndRespectVisibility(t *testing.T) {
	templates := v31NavigationTemplates(t)
	for _, enabled := range []bool{false, true} {
		for _, kind := range []string{"home", "article", "friends"} {
			doc := v31NavigationDocument(t, templates, v31NavigationPage(kind, defaultFloatingControls(), false, enabled))
			footer := v31NavigationClass(t, doc, "site-footer")
			friends := []*nethtml.Node{}
			for _, link := range v31NavigationLinks(footer) {
				if friendWebAttr(link, "href") == "/friends" {
					friends = append(friends, link)
				}
			}
			if (len(friends) == 1) != enabled {
				t.Errorf("%s: footer friend entry visibility=%t, got %d links", kind, enabled, len(friends))
			}
			if enabled && len(friends) == 1 {
				if strings.TrimSpace(friendWebText(friends[0])) != "友链" {
					t.Error("footer friend destination lacks its readable label")
				}
				if (friendWebAttr(friends[0], "aria-current") == "page") != (kind == "friends") {
					t.Errorf("%s: footer friend current-page state is incorrect", kind)
				}
			}
			for _, class := range []string{"site-header", "site-dock"} {
				for _, link := range v31NavigationLinks(v31NavigationClass(t, doc, class)) {
					if friendWebAttr(link, "href") == "/friends" {
						t.Errorf("friend entry moved out of the footer into %s", class)
					}
				}
			}
		}
	}
}
