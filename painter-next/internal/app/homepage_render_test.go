package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	nethtml "golang.org/x/net/html"
)

func homeSSRApp(t *testing.T) (*App, *atomic.Int32) {
	t.Helper()
	a := homeTestApp(t)
	a.RegisterContent()
	a.RegisterMedia()
	a.RegisterWeb()
	calls := &atomic.Int32{}
	homeFake(a, func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, fmt.Errorf("homepage must not request provider metadata")
	})
	return a, calls
}
func homeSSRSave(t *testing.T, a *App, cookie *http.Cookie, csrf string, change func(map[string]any)) map[string]any {
	t.Helper()
	current := homeTestGet(t, a, cookie)
	config := current["config"].(map[string]any)
	change(config)
	steam := current["steam"].(map[string]any)
	input := map[string]any{"steam_id": steam["steam_id"], "mode": steam["mode"], "auto_sync_enabled": steam["auto_sync_enabled"]}
	return homeTestPut(t, a, cookie, csrf, config, input, int64(current["version"].(float64)))
}
func homeSSRDocument(t *testing.T, a *App, cookie *http.Cookie) *nethtml.Node {
	t.Helper()
	response := testRequest(a, "GET", "/?scope=site&visible=false", "", cookie, "")
	if response.Code != 200 || !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("homepage SSR status=%d: %s", response.Code, response.Body.String())
	}
	return friendWebDocument(t, response.Body.String())
}
func homeSSRSections(doc *nethtml.Node) []string {
	result := []string{}
	for _, node := range friendWebNodes(doc, func(n *nethtml.Node) bool {
		return n.Type == nethtml.ElementNode && friendWebHasAttr(n, "data-home-section")
	}) {
		result = append(result, friendWebAttr(node, "data-home-section"))
	}
	return result
}
func homeSSRSection(doc *nethtml.Node, key string) *nethtml.Node {
	for _, node := range friendWebNodes(doc, func(n *nethtml.Node) bool {
		return n.Type == nethtml.ElementNode && friendWebAttr(n, "data-home-section") == key
	}) {
		return node
	}
	return nil
}
func homeSSRProfile(t *testing.T, a *App) {
	t.Helper()
	if _, e := a.Pool.Exec(context.Background(), `UPDATE site_profile SET display_name='SiteBrandFixture',intro_name='SiteInheritedNameFixture',tagline='SiteInheritedDescriptionFixture',intro_roles_json='["SiteInheritedRoleFixture"]'::jsonb WHERE id=1`); e != nil {
		t.Fatal(e)
	}
}

func TestHomeCollectionGroupingPreservesRenderedOrder(t *testing.T) {
	for _, sample := range []struct {
		name string
		keys []string
		head int
	}{
		{name: "empty", head: -1},
		{name: "games alone", keys: []string{"steam"}, head: -1},
		{name: "music alone", keys: []string{"music"}, head: -1},
		{name: "games then music", keys: []string{"intro", "steam", "music", "closing"}, head: 1},
		{name: "music then games", keys: []string{"photos", "music", "steam", "articles"}, head: 1},
		{name: "games separated", keys: []string{"steam", "photos", "music"}, head: -1},
		{name: "music separated", keys: []string{"music", "activity", "steam"}, head: -1},
	} {
		t.Run(sample.name, func(t *testing.T) {
			sections := make([]homeWebSection, len(sample.keys))
			for i, key := range sample.keys {
				sections[i] = homeWebSection{Key: key, Title: key + " title"}
			}
			groupHomeSections(sections)
			for i := range sections {
				s := &sections[i]
				if s.Key != sample.keys[i] || s.Title != s.Key+" title" {
					t.Fatal("grouping reordered or changed a section", sections)
				}
				if i == sample.head {
					next := &sections[i+1]
					if s.Key == "music" {
						if s.FollowingGames != next || s.FollowingMusic != nil {
							t.Fatal("music must point to the original following game section")
						}
					} else if s.FollowingMusic != next || s.FollowingGames != nil {
						t.Fatal("games must point to the original following music section")
					}
				} else if s.FollowingMusic != nil || s.FollowingGames != nil {
					t.Fatal("non-leading section unexpectedly contains another section", s.Key)
				}
				trailing := sample.head >= 0 && i == sample.head+1
				if s.GroupedWithGames != (trailing && s.Key == "music") || s.GroupedWithMusic != (trailing && s.Key == "steam") {
					t.Fatal("only the trailing collection should skip standalone rendering", s.Key)
				}
			}
		})
	}
}

func TestHomeWorkJourneyRequiresRenderedSequence(t *testing.T) {
	for _, sample := range []struct {
		name       string
		keys       []string
		start, end int
	}{
		{name: "contiguous", keys: []string{"intro", "projects", "activity"}, start: 0, end: 2},
		{name: "surrounded", keys: []string{"photos", "intro", "projects", "activity", "music", "steam"}, start: 1, end: 3},
		{name: "separated", keys: []string{"intro", "projects", "photos", "activity"}, start: -1, end: -1},
		{name: "reordered", keys: []string{"projects", "intro", "activity"}, start: -1, end: -1},
		{name: "intro filtered", keys: []string{"projects", "activity"}, start: -1, end: -1},
		{name: "projects filtered", keys: []string{"intro", "activity"}, start: -1, end: -1},
		{name: "activity filtered", keys: []string{"intro", "projects"}, start: -1, end: -1},
	} {
		t.Run(sample.name, func(t *testing.T) {
			sections := make([]homeWebSection, len(sample.keys))
			for i, key := range sample.keys {
				sections[i].Key = key
			}
			groupHomeSections(sections)
			for i, s := range sections {
				if s.Key != sample.keys[i] || s.WorkJourneyStart != (i == sample.start) || s.WorkJourneyEnd != (i == sample.end) {
					t.Fatal("work journey changed order or marked a noncontiguous sequence", sections)
				}
			}
		})
	}
}

func TestHomePageWorkJourneyFollowsVisibleOrder(t *testing.T) {
	a, _ := homeSSRApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	for _, sample := range []struct {
		name                string
		order, want         []string
		disabled, zeroLimit string
		visibleSeparator    bool
		grouped             bool
	}{
		{name: "contiguous", order: []string{"intro", "projects", "activity"}, want: []string{"intro", "projects", "activity"}, grouped: true},
		{name: "reordered", order: []string{"projects", "intro", "activity"}, want: []string{"projects", "intro", "activity"}},
		{name: "visible separator", order: []string{"intro", "closing", "projects", "activity"}, want: []string{"intro", "closing", "projects", "activity"}, visibleSeparator: true},
		{name: "disabled separator", order: []string{"intro", "closing", "projects", "activity"}, want: []string{"intro", "projects", "activity"}, grouped: true},
		{name: "disabled intro", order: []string{"intro", "projects", "activity"}, want: []string{"projects", "activity"}, disabled: "intro"},
		{name: "disabled projects", order: []string{"intro", "projects", "activity"}, want: []string{"intro", "activity"}, disabled: "projects"},
		{name: "zero projects", order: []string{"intro", "projects", "activity"}, want: []string{"intro", "activity"}, zeroLimit: "projects"},
		{name: "disabled activity", order: []string{"intro", "projects", "activity"}, want: []string{"intro", "projects"}, disabled: "activity"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			homeSSRSave(t, a, owner, csrf, func(c map[string]any) {
				order := append([]string{}, sample.order...)
				for _, key := range homeSectionKeys {
					if !slices.Contains(order, key) {
						order = append(order, key)
					}
					s := c["sections"].(map[string]any)[key].(map[string]any)
					s["enabled"] = ((key == "intro" || key == "projects" || key == "activity") && key != sample.disabled) || (key == "closing" && sample.visibleSeparator)
					if key == "projects" {
						s["limit"] = 6
						if sample.zeroLimit == key {
							s["limit"] = 0
						}
					}
				}
				c["order"] = order
			})
			page, err := a.homepagePage(context.Background(), map[string]any{})
			if err != nil {
				t.Fatal(err)
			}
			keys := make([]string, len(page.HomeSections))
			for i, s := range page.HomeSections {
				keys[i] = s.Key
				if s.WorkJourneyStart != (sample.grouped && s.Key == "intro") || s.WorkJourneyEnd != (sample.grouped && s.Key == "activity") {
					t.Fatal("work journey ignored rendered sequence", s.Key)
				}
			}
			if !reflect.DeepEqual(keys, sample.want) {
				t.Fatal("work journey changed rendered order", keys)
			}
			doc := homeSSRDocument(t, a, nil)
			if got := homeSSRSections(doc); !reflect.DeepEqual(got, sample.want) {
				t.Fatal("work journey template reordered or duplicated sections", got)
			}
			journeys := friendWebClass(doc, "home-work-journey")
			if (len(journeys) == 1) != sample.grouped || len(journeys) > 1 {
				t.Fatal("work journey template ignored rendered boundaries", len(journeys))
			}
			if sample.grouped && !reflect.DeepEqual(homeSSRSections(journeys[0]), []string{"intro", "projects", "activity"}) {
				t.Fatal("work journey wrapper contains the wrong sections", homeSSRSections(journeys[0]))
			}
		})
	}
}

func TestHomePageCollectionsGroupAfterVisibilityFiltering(t *testing.T) {
	a, calls := homeSSRApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	for i := 0; i < 6; i++ {
		game := homeTestInput("steam")
		game["title"] = fmt.Sprintf("Ordered game %d", i)
		game["position"] = i
		homeTestItem(t, a, owner, csrf, game)
	}
	album := homeTestInput("music")
	album["url"] = "https://music.apple.com/us/album/12345"
	album["image_url"] = "https://is1-ssl.mzstatic.com/image/thumb/a/632x632bb.jpg"
	homeTestItem(t, a, owner, csrf, album)
	for _, sample := range []struct {
		name                string
		order               []string
		separator           string
		separatorEnabled    bool
		disabled, zeroLimit string
		want                []string
		grouped             bool
	}{
		{name: "music before games", order: []string{"music", "steam"}, want: []string{"music", "steam"}, grouped: true},
		{name: "games before music", order: []string{"steam", "music"}, want: []string{"steam", "music"}, grouped: true},
		{name: "visible separator", order: []string{"music", "activity", "steam"}, separator: "activity", separatorEnabled: true, want: []string{"music", "activity", "steam"}},
		{name: "disabled separator", order: []string{"steam", "activity", "music"}, separator: "activity", want: []string{"steam", "music"}, grouped: true},
		{name: "empty separator", order: []string{"music", "photos", "steam"}, separator: "photos", separatorEnabled: true, want: []string{"music", "steam"}, grouped: true},
		{name: "disabled music", order: []string{"music", "steam"}, disabled: "music", want: []string{"steam"}},
		{name: "zero music", order: []string{"steam", "music"}, zeroLimit: "music", want: []string{"steam"}},
		{name: "disabled games", order: []string{"steam", "music"}, disabled: "steam", want: []string{"music"}},
		{name: "zero games", order: []string{"music", "steam"}, zeroLimit: "steam", want: []string{"music"}},
	} {
		t.Run(sample.name, func(t *testing.T) {
			homeSSRSave(t, a, owner, csrf, func(c map[string]any) {
				order := append([]string{}, sample.order...)
				for _, key := range homeSectionKeys {
					if !slices.Contains(order, key) {
						order = append(order, key)
					}
					s := c["sections"].(map[string]any)[key].(map[string]any)
					s["enabled"] = ((key == "steam" || key == "music") && key != sample.disabled) || (key == sample.separator && sample.separatorEnabled)
					if key == "steam" || key == "music" || key == "photos" {
						s["limit"] = 24
						if key == sample.zeroLimit {
							s["limit"] = 0
						}
					}
				}
				c["order"] = order
			})
			page, err := a.homepagePage(context.Background(), map[string]any{})
			if err != nil {
				t.Fatal(err)
			}
			keys := make([]string, len(page.HomeSections))
			grouped := 0
			for i := range page.HomeSections {
				s := &page.HomeSections[i]
				keys[i] = s.Key
				if s.Key == "steam" {
					if len(s.Items) != 5 || s.Items[0]["title"] != "Ordered game 0" || s.Items[4]["title"] != "Ordered game 4" {
						t.Fatal("grouping changed the ordered five-game snapshot", s.Items)
					}
				}
				if s.FollowingMusic != nil || s.FollowingGames != nil {
					grouped++
					if i+1 >= len(page.HomeSections) {
						t.Fatal("collection points beyond rendered sections")
					}
					next := &page.HomeSections[i+1]
					if s.Key == "music" {
						if s.FollowingGames != next || !next.GroupedWithMusic || next.GroupedWithGames {
							t.Fatal("music-to-games grouping lost its original neighbor")
						}
					} else if s.FollowingMusic != next || !next.GroupedWithGames || next.GroupedWithMusic {
						t.Fatal("games-to-music grouping lost its original neighbor")
					}
				} else if !sample.grouped && (s.GroupedWithGames || s.GroupedWithMusic) {
					t.Fatal("standalone collection was marked as grouped", s.Key)
				}
			}
			if !reflect.DeepEqual(keys, sample.want) || (grouped == 1) != sample.grouped || grouped > 1 {
				t.Fatal("rendered order or collection adjacency changed", keys, grouped)
			}
			doc := homeSSRDocument(t, a, nil)
			if got := homeSSRSections(doc); !reflect.DeepEqual(got, sample.want) {
				t.Fatal("collection template reordered or duplicated sections", got)
			}
			journeys := friendWebClass(doc, "home-collection-journey")
			if (len(journeys) == 1) != sample.grouped || len(journeys) > 1 {
				t.Fatal("collection template ignored adjacency", len(journeys))
			}
			if sample.grouped && !reflect.DeepEqual(homeSSRSections(journeys[0]), sample.want) {
				t.Fatal("collection wrapper changed configured order", homeSSRSections(journeys[0]))
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatal("collection rendering contacted a provider", calls.Load())
	}
}

func TestHomeSSRDefaultsPreserveIntroductionAndEmptyWalls(t *testing.T) {
	a, calls := homeSSRApp(t)
	_, owner, _ := testIdentity(t, a, "owner")
	homeSSRProfile(t, a)
	// Legacy writing copy remains configurable, but the introduction now renders
	// the inherited profile rather than the old writing scene.
	config, _ := homeTestConfig(t, a, owner)
	for key, want := range map[string]string{
		"writing_title":       "把想法，\n变成文字。",
		"writing_description": "技术笔记，也有日常。\n把想法写下来，和你分享。",
	} {
		if config[key] != want {
			t.Fatalf("default %s changed: got %q, want %q", key, config[key], want)
		}
	}
	compactText := func(value string) string { return strings.Join(strings.Fields(value), "") }
	for _, cookie := range []*http.Cookie{nil, owner} {
		doc := homeSSRDocument(t, a, cookie)
		if !reflect.DeepEqual(homeSSRSections(doc), []string{"intro", "articles", "closing"}) {
			t.Fatal("migration changed existing sections", homeSSRSections(doc))
		}
		intro := homeSSRSection(doc, "intro")
		// Presentation may split the same copy across spans or whitespace.
		text := compactText(friendWebText(intro))
		for _, want := range []string{"你好，我是", "SiteInheritedNameFixture", "SiteInheritedDescriptionFixture", "SiteInheritedRoleFixture", "向下探索"} {
			if !strings.Contains(text, compactText(want)) {
				t.Fatal("default introduction copy/real profile missing", want)
			}
		}
		body := friendWebNodes(doc, func(n *nethtml.Node) bool { return n.Type == nethtml.ElementNode && n.Data == "body" })
		if len(body) != 1 || friendWebAttr(body[0], "data-home-motion") != "true" || friendWebAttr(body[0], "data-home-warmth") != "true" {
			t.Fatal("default V21 motion/warmth flags changed")
		}
		if len(friendWebClass(doc, "home-game"))+len(friendWebClass(doc, "home-record"))+len(friendWebClass(doc, "home-photo")) != 0 {
			t.Fatal("empty walls fabricated items")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("homepage contacted provider", calls.Load())
	}
}
func TestHomeSSRArticleCountAndPublishedSnapshot(t *testing.T) {
	a, calls := homeSSRApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	f := relatedFixture{t, a, owner, csrf}
	publicIDs := map[string]bool{}
	for i := 0; i < 5; i++ {
		publicIDs[f.post(fmt.Sprintf("Home public article %d", i), true)] = true
	}
	f.post("Home private unpublished title sentinel", false)
	for _, limit := range []int{2, 4, 0} {
		homeSSRSave(t, a, owner, csrf, func(config map[string]any) {
			config["sections"].(map[string]any)["articles"].(map[string]any)["limit"] = limit
		})
		doc := homeSSRDocument(t, a, nil)
		entries := friendWebClass(doc, "home-writing-entry")
		if len(entries) != limit {
			t.Fatal("configured article count ignored", limit, len(entries))
		}
		seen := map[string]bool{}
		for _, entry := range entries {
			target := strings.TrimPrefix(friendWebAttr(entry, "href"), "/posts/")
			if !publicIDs[target] || seen[target] {
				t.Fatal("homepage article is private/duplicated", target)
			}
			seen[target] = true
		}
		if strings.Contains(friendWebText(doc), "Home private unpublished title sentinel") {
			t.Fatal("draft title entered homepage")
		}
		if limit == 0 {
			if homeSSRSection(doc, "articles") != nil {
				t.Fatal("zero count left article section visible")
			}
			links := friendWebNodes(homeSSRSection(doc, "intro"), func(n *nethtml.Node) bool {
				return n.Type == nethtml.ElementNode && n.Data == "a" && friendWebAttr(n, "href") == "/posts"
			})
			if len(links) != 1 {
				t.Fatal("hidden writing section left broken default jump")
			}
		}
	}
	if calls.Load() != 0 {
		t.Fatal("article homepage fetched provider")
	}
}
func TestHomeSSRSixSectionOrderEscapingAndNativeWalls(t *testing.T) {
	a, calls := homeSSRApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	homeSSRProfile(t, a)
	game := homeTestInput("steam")
	game["title"] = `<em data-home-item-xss>Game & title</em>`
	game["subtitle"] = `<img src=x onerror=alert(1)>`
	homeTestItem(t, a, owner, csrf, game)
	albumURL := "https://music.apple.com/us/album/example/123456"
	album := homeTestInput("music")
	album["title"] = "Album & original"
	album["url"] = albumURL
	album["image_url"] = "https://is1-ssl.mzstatic.com/image/thumb/a/100x100bb.jpg"
	album["provider_id"] = "123456"
	homeTestItem(t, a, owner, csrf, album)
	media := momentImageUpload(t, a, owner, csrf, 8)
	photo := homeTestInput("photos")
	photo["title"] = `Photo <b data-home-photo-xss>title</b>`
	photo["alt"] = `<img src=x onerror=alert(2)> & "alt"`
	photo["subtitle"] = "Photo subtitle"
	photo["url"] = "https://example.com/photo-story"
	photo["media_id"] = media
	photo["image_url"] = ""
	photo["provider_id"] = ""
	homeTestItem(t, a, owner, csrf, photo)
	order := []string{"photos", "closing", "music", "steam", "articles", "intro"}
	homeSSRSave(t, a, owner, csrf, func(config map[string]any) {
		config["order"] = order
		config["use_site_intro"] = false
		config["intro_name"] = "CustomName"
		config["intro_roles"] = []string{"CustomRole"}
		// This regression covers the original six-section client contract.
		// New opt-in sections remain disabled unless explicitly configured.
		for _, key := range order {
			s := config["sections"].(map[string]any)[key].(map[string]any)
			s["enabled"] = true
			s["title"] = "<em data-home-setting-xss>" + key + " & title</em>"
			s["description"] = `<script data-home-script-xss>alert(1)</script>`
		}
	})
	for _, cookie := range []*http.Cookie{nil, owner} {
		doc := homeSSRDocument(t, a, cookie)
		if !reflect.DeepEqual(homeSSRSections(doc), order) {
			t.Fatal("section order ignored", homeSSRSections(doc))
		}
		if len(friendWebNodes(doc, func(n *nethtml.Node) bool {
			return n.Type == nethtml.ElementNode && (friendWebHasAttr(n, "data-home-item-xss") || friendWebHasAttr(n, "data-home-photo-xss") || friendWebHasAttr(n, "data-home-setting-xss") || friendWebHasAttr(n, "data-home-script-xss") || friendWebHasAttr(n, "onerror"))
		})) != 0 {
			t.Fatal("home plain-text configuration injected elements")
		}
		text := friendWebText(doc)
		for _, want := range []string{game["title"].(string), game["subtitle"].(string), photo["title"].(string), `<script data-home-script-xss>alert(1)</script>`, "CustomName", "CustomRole"} {
			if !strings.Contains(text, want) {
				t.Fatal("SSR lost plain-text content", want)
			}
		}
		pictures := friendWebClass(doc, "home-photo")
		if len(pictures) != 1 {
			t.Fatal("photo wall missing")
		}
		links := friendWebNodes(pictures[0], func(n *nethtml.Node) bool {
			return n.Type == nethtml.ElementNode && n.Data == "a" && friendWebHasAttr(n, "data-moment-image")
		})
		if len(links) != 1 || friendWebAttr(links[0], "href") != "/media/"+media || friendWebAttr(links[0], "target") != "_blank" || !strings.Contains(" "+friendWebAttr(links[0], "rel")+" ", " noopener ") {
			t.Fatal("no-JS photo anchor missing/safe target lost")
		}
		captionLinks := friendWebNodes(pictures[0], func(n *nethtml.Node) bool {
			return n.Type == nethtml.ElementNode && n.Data == "a" && !friendWebHasAttr(n, "data-moment-image")
		})
		if len(captionLinks) != 1 || friendWebAttr(captionLinks[0], "href") != photo["url"] {
			t.Fatal("configured photo caption destination was ignored")
		}
		images := friendWebNodes(links[0], func(n *nethtml.Node) bool { return n.Type == nethtml.ElementNode && n.Data == "img" })
		if len(images) != 1 || friendWebAttr(images[0], "src") != "/media/"+media || friendWebAttr(images[0], "alt") != photo["alt"] || friendWebAttr(links[0], "data-image-alt") != photo["alt"] {
			t.Fatal("photo/alt was not escaped and retained")
		}
		galleries := friendWebNodes(doc, func(n *nethtml.Node) bool {
			return n.Type == nethtml.ElementNode && friendWebHasAttr(n, "data-moment-gallery") && friendWebAttr(n, "data-gallery-title") == "图片墙"
		})
		if len(galleries) != 1 {
			t.Fatal("native photo wall gallery context missing")
		}
		badges := friendWebClass(doc, "home-music-badge")
		if len(badges) != 1 || friendWebAttr(badges[0], "href") != albumURL {
			t.Fatal("Apple Music badge not linked to real album")
		}
		badgeImages := friendWebNodes(badges[0], func(n *nethtml.Node) bool { return n.Type == nethtml.ElementNode && n.Data == "img" })
		if len(badgeImages) != 1 || friendWebAttr(badgeImages[0], "src") != "/assets/brands/listen-on-apple-music-zh-cn.svg" || friendWebAttr(badgeImages[0], "alt") != "在 Apple Music 上聆听" {
			t.Fatal("official album badge missing")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("wall render requested external metadata")
	}
}
func TestHomeSSRHiddenDisabledAndZeroPhotoWallStayPrivate(t *testing.T) {
	a, calls := homeSSRApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	media := momentImageUpload(t, a, owner, csrf, 10)
	item := homeTestPhoto(t, a, owner, csrf, media, false)
	homeSSRSave(t, a, owner, csrf, func(c map[string]any) { c["sections"].(map[string]any)["photos"].(map[string]any)["enabled"] = true })
	for _, cookie := range []*http.Cookie{nil, owner} {
		doc := homeSSRDocument(t, a, cookie)
		if homeSSRSection(doc, "photos") != nil {
			t.Fatal("hidden photo rendered for visitor/owner cookie")
		}
	}
	momentImageRead(t, a, nil, media, 404)
	in := homeTestInput("photos")
	in["media_id"] = media
	in["image_url"] = ""
	in["provider_id"] = ""
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/items/"+item["id"].(string), in, 1, RandID()), 200)
	for _, state := range []struct {
		enabled bool
		limit   int
		visible bool
	}{{false, 9, false}, {true, 0, false}, {true, 1, true}} {
		homeSSRSave(t, a, owner, csrf, func(c map[string]any) {
			section := c["sections"].(map[string]any)["photos"].(map[string]any)
			section["enabled"] = state.enabled
			section["limit"] = state.limit
		})
		doc := homeSSRDocument(t, a, nil)
		if (homeSSRSection(doc, "photos") != nil) != state.visible {
			t.Fatal("wall ignored enabled/count", state)
		}
		status := 404
		if state.visible {
			status = 200
		}
		momentImageRead(t, a, nil, media, status)
	}
	if calls.Load() != 0 {
		t.Fatal("photo visibility check contacted provider")
	}
}
func TestHomeSSRCustomIntroDoesNotInheritAndAllowsEmpty(t *testing.T) {
	a, _ := homeSSRApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	homeSSRProfile(t, a)
	homeSSRSave(t, a, owner, csrf, func(c map[string]any) {
		c["use_site_intro"] = false
		c["intro_name"] = "CustomIndependentName"
		c["intro_roles"] = []string{"CustomIndependentRole"}
		c["sections"].(map[string]any)["intro"].(map[string]any)["description"] = "CustomIndependentDescription"
	})
	text := friendWebText(homeSSRSection(homeSSRDocument(t, a, nil), "intro"))
	for _, want := range []string{"CustomIndependentName", "CustomIndependentRole", "CustomIndependentDescription"} {
		if !strings.Contains(text, want) {
			t.Fatal("custom intro omitted", want)
		}
	}
	if strings.Contains(text, "SiteInherited") {
		t.Fatal("custom intro fell back to site profile")
	}
	homeSSRSave(t, a, owner, csrf, func(c map[string]any) {
		c["intro_name"] = ""
		c["intro_roles"] = []string{}
		c["writing_title"] = ""
		c["writing_description"] = ""
		c["sections"].(map[string]any)["intro"].(map[string]any)["description"] = ""
	})
	text = friendWebText(homeSSRSection(homeSSRDocument(t, a, nil), "intro"))
	if strings.Contains(text, "SiteInherited") || strings.Contains(text, "CustomIndependent") {
		t.Fatal("explicit empty custom intro silently inherited")
	}
}
func TestHomeSSRAllOffAndStudioPermissions(t *testing.T) {
	a, calls := homeSSRApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	_, author, _ := testIdentity(t, a, "author")
	_, reader, _ := testIdentity(t, a, "reader")
	homeSSRSave(t, a, owner, csrf, func(c map[string]any) {
		for _, key := range homeSectionKeys {
			c["sections"].(map[string]any)[key].(map[string]any)["enabled"] = false
		}
		c["motion_enabled"] = false
		c["warmth_enabled"] = false
	})
	doc := homeSSRDocument(t, a, nil)
	if len(homeSSRSections(doc)) != 0 {
		t.Fatal("disabled home sections remain")
	}
	h1 := friendWebNodes(doc, func(n *nethtml.Node) bool { return n.Type == nethtml.ElementNode && n.Data == "h1" })
	if len(h1) != 1 || friendWebText(h1[0]) != "Painter" {
		t.Fatal("minimal homepage lost semantic heading")
	}
	body := friendWebNodes(doc, func(n *nethtml.Node) bool { return n.Type == nethtml.ElementNode && n.Data == "body" })
	if friendWebAttr(body[0], "data-home-motion") != "false" || friendWebAttr(body[0], "data-home-warmth") != "false" {
		t.Fatal("motion settings not wired")
	}
	for _, path := range []string{"/studio/home", "/studio/home/"} {
		for _, actor := range []struct {
			cookie *http.Cookie
			status int
		}{{nil, 303}, {reader, 403}, {author, 403}, {owner, 200}} {
			response := testRequest(a, "GET", path, "", actor.cookie, "")
			if response.Code != actor.status {
				t.Fatal("homepage studio permission", path, response.Code, actor.status)
			}
			if actor.cookie == nil {
				target, e := url.Parse(response.Header().Get("Location"))
				if e != nil || target.Path != "/login" || target.Query().Get("return_to") != path {
					t.Fatal("studio login redirect lost return path")
				}
			}
			if actor.status == 200 && (!strings.Contains(response.Body.String(), `id="workbench"`) || !strings.Contains(response.Body.String(), `content="noindex,nofollow"`)) {
				t.Fatal("owner did not receive private studio shell")
			}
		}
	}
	if calls.Load() != 0 {
		t.Fatal("minimal homepage contacted provider")
	}
}
