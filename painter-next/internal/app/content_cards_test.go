package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
)

func cardFixture(kind string) contentCard {
	c := contentCard{V: 1, Type: kind, Title: "可搜索的快照标题", Description: "正文参考说明", MetadataSource: "author"}
	switch kind {
	case "github":
		c.URL = "https://github.com/openai/codex"
		c.ImageURL = "https://avatars.githubusercontent.com/u/14957082?v=4"
	case "steam":
		c.URL = "https://store.steampowered.com/app/730/"
		c.ImageURL = "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/730/header.jpg"
	case "apple_music":
		c.URL = "https://music.apple.com/cn/album/example/123456"
		c.ImageURL = "https://is1-ssl.mzstatic.com/image/thumb/a/100x100bb.jpg"
	case "link":
		c.URL = "https://example.org/reference"
	}
	return c
}

func TestContentCardRenderPortableSnapshotsAndText(t *testing.T) {
	var pieces []string
	for _, kind := range []string{"github", "steam", "apple_music", "link"} {
		pieces = append(pieces, contentCardMarkdown(cardFixture(kind)))
	}
	inline := cardFixture("link")
	inline.Title = "🔎 中文 *参考* [链接] ~~字面~~"
	markdown := "# " + contentCardInlineMarkdown(inline) + "\n\n" + strings.Join(pieces, "\n\n")
	r, e := RenderMarkdown(markdown)
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"github", "steam", "apple_music", "link"} {
		if !strings.Contains(r.HTML, "content-card--"+kind) {
			t.Fatal("card missing", kind, r.HTML)
		}
	}
	for _, word := range []string{"可搜索的快照标题", "正文参考说明", "中文", "参考"} {
		if !strings.Contains(r.Text, word) {
			t.Fatal("search text lost", word, r.Text)
		}
	}
	if len(r.TOC) != 1 || !strings.Contains(r.TOC[0].Text, "🔎") || !strings.Contains(r.HTML, `class="content-link__preview"`) {
		t.Fatal("inline/TOC lost", r)
	}
	if strings.Contains(r.HTML, "<del>") {
		t.Fatal("reference label was interpreted as Markdown instead of literal text", r.HTML)
	}
	if !strings.Contains(r.HTML, `target="_blank"`) || !strings.Contains(r.HTML, "noopener") {
		t.Fatal("external link contract lost", r.HTML)
	}
	if r.Version != RendererVersion || !strings.Contains(r.Version, "painter-cards-v1") {
		t.Fatal("version missing", r.Version)
	}
	copy, e := RenderMarkdown(markdown)
	if e != nil || copy.Hash != r.Hash || copy.HTML != r.HTML {
		t.Fatal("snapshot rendering nondeterministic", copy, e)
	}
}

func TestContentCardsEscapeMetadataAndNeverTrustHTML(t *testing.T) {
	c := cardFixture("github")
	c.Title = `<img src=x onerror=alert(1)>`
	c.Description = `</a><script>alert(1)</script>`
	r, e := RenderMarkdown(contentCardMarkdown(c))
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(r.HTML, "<script") || strings.Contains(r.HTML, "<img src=x") || !strings.Contains(r.HTML, "&lt;script&gt;") {
		t.Fatal("unescaped card metadata", r.HTML)
	}
	for _, raw := range []string{`<a class="content-card" href="https://example.org">raw html card</a>`, "`{\"v\":1,\"type\":\"github\"}`", "```json\n{\"v\":1,\"type\":\"github\"}\n```", "````markdown\n" + contentCardMarkdown(c) + "\n````"} {
		r, e := RenderMarkdown(raw)
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(r.HTML, `class="content-card `) {
			t.Fatal("unmarked source became card", raw, r.HTML)
		}
	}
}

func TestContentCardUnknownVersionsRemainCodeAndInvalidKnownFail(t *testing.T) {
	for _, raw := range []string{`{"v":2,"type":"github","url":"javascript:alert(1)"}`, `{"v":1,"type":"future-widget","html":"<script>bad()</script>"}`, `null`} {
		r, e := RenderMarkdown("```painter-card\n" + raw + "\n```")
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(r.HTML, `class="content-card `) || !strings.Contains(r.HTML, "<pre") || strings.Contains(r.HTML, "<script>") {
			t.Fatal("unknown card became trusted UI", r.HTML)
		}
	}
	for _, raw := range []string{`{"v":1,"type":"github"}`, `{"v":1,"type":"link","url":"https://example.org","title":"x","html":"<img>"}`, `{"v":1,"type":"link","url":"https://example.org","title":"x","stats":{"stars":1}}`, `{"v":1,"type":"link","url":"https://example.org","title":"x"} {}`, `{broken`} {
		if _, e := RenderMarkdown("```painter-card\n" + raw + "\n```"); e == nil {
			t.Fatal("invalid card accepted", raw)
		}
	}
	if _, e := RenderMarkdown("```painter-card\n" + strings.Repeat(" ", contentCardLimit+1) + "\n```"); e == nil {
		t.Fatal("oversized card accepted")
	}
	if _, e := RenderMarkdown(strings.Repeat(contentCardMarkdown(cardFixture("link"))+"\n\n", 129)); e == nil {
		t.Fatal("card count bound absent")
	}
}

func TestContentCardURLImageAndInlineValidation(t *testing.T) {
	for _, destination := range []string{"javascript:alert(1)", "data:text/html,bad", "//example.org/a", "http://example.org", "https://localhost/", "https://sub.localhost/", "https://127.0.0.1/", "https://169.254.169.254/", "https://100.64.0.1/", "https://[::1]/", "https://[::ffff:127.0.0.1]/", "https://192.0.2.1/", "https://example.org:443/", "https://user:pass@example.org/", "https://example.org/%0a", "https://example.org\\@evil.test/", "https://example.org./"} {
		c := cardFixture("link")
		c.URL = destination
		if _, e := RenderMarkdown(contentCardMarkdown(c)); e == nil {
			t.Fatal("unsafe URL accepted", destination)
		}
	}
	for _, kind := range []string{"github", "steam", "apple_music"} {
		c := cardFixture(kind)
		c.ImageURL = "https://evil.test/track.png"
		if _, e := RenderMarkdown(contentCardMarkdown(c)); e == nil {
			t.Fatal("untrusted remote image", kind)
		}
		c = cardFixture(kind)
		c.URL = "https://example.org/"
		if _, e := RenderMarkdown(contentCardMarkdown(c)); e == nil {
			t.Fatal("provider mismatch accepted", kind)
		}
	}
	c := cardFixture("link")
	raw, _ := json.Marshal(c)
	for _, markdown := range []string{`[x](https://different.example "` + contentCardInlinePrefix + base64.RawURLEncoding.EncodeToString(raw) + `")`, `[x](https://example.org "painter:v1:!bad")`} {
		if _, e := RenderMarkdown(markdown); e == nil {
			t.Fatal("bad inline snapshot accepted", markdown)
		}
	}
	c = cardFixture("steam")
	if _, e := RenderMarkdown(contentCardInlineMarkdown(c)); e == nil {
		t.Fatal("provider block accepted as inline")
	}
	c = cardFixture("link")
	c.ImageURL = "/media/0123456789abcdef0123456789abcdef"
	r, e := RenderMarkdown(contentCardMarkdown(c))
	if e != nil || len(r.MediaIDs) != 1 || r.MediaIDs[0] != "0123456789abcdef0123456789abcdef" {
		t.Fatal("local card image not linked", r, e)
	}
	for _, suffix := range []string{"?download=1", "#part", "/../x"} {
		bad := c
		bad.ImageURL += suffix
		if _, e := RenderMarkdown(contentCardMarkdown(bad)); e == nil {
			t.Fatal("invalid media URL", bad.ImageURL)
		}
	}
}

func TestContentCardPublicNetworkBoundaries(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "::1", "::ffff:127.0.0.1", "10.1.2.3", "169.254.1.1", "100.64.0.1", "192.0.2.1", "198.18.0.1", "198.51.100.1", "203.0.113.1", "2001:db8::1", "64:ff9b::a00:1", "2002:7f00:1::"} {
		if contentCardPublicIP(netip.MustParseAddr(raw)) {
			t.Fatal("nonpublic IP allowed", raw)
		}
	}
	for _, raw := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"} {
		if !contentCardPublicIP(netip.MustParseAddr(raw)) {
			t.Fatal("public IP refused", raw)
		}
	}
}

func TestContentCardGitHubProviderAndMissingStats(t *testing.T) {
	a := &App{Config: Config{Environment: "test"}}
	var calls atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.String() != "https://api.github.com/repos/OpenAI/codex" || r.Header.Get("Authorization") != "" || r.Header.Get("x-webapi-key") != "" || r.Header.Get("X-GitHub-Api-Version") == "" {
			t.Error("provider request changed", r.URL, r.Header)
		}
		return homeHTTP(200, `{"name":"codex","full_name":"openai/codex","html_url":"https://github.com/openai/codex","description":"Repository description","owner":{"login":"openai","avatar_url":"https://avatars.githubusercontent.com/u/14957082?v=4"},"private":false,"language":"Rust"}`), nil
	})
	c, e := a.contentCardResolve(context.Background(), contentCardResolveInput{Type: "github", URL: "https://github.com/OpenAI/codex"})
	if e != nil {
		t.Fatal(e)
	}
	if c.Stats != nil || c.Title != "openai/codex" || c.MetadataSource != "provider" || calls.Load() != 1 {
		t.Fatal("fake stats or bad provider snapshot", c, calls.Load())
	}
	r, e := RenderMarkdown(contentCardMarkdown(c))
	if e != nil || strings.Contains(r.HTML, "★ 0") || strings.Contains(r.HTML, "Forks 0") {
		t.Fatal("unknown metrics rendered as zero", r, e)
	}
	for _, body := range []string{`{"private":true}`, `{"name":"codex","full_name":"other/codex","html_url":"https://github.com/other/codex","owner":{"login":"other"}}`, `{"name":"codex","full_name":"openai/codex","html_url":"https://evil.test/","owner":{"login":"openai"}}`} {
		homeFake(a, func(*http.Request) (*http.Response, error) { return homeHTTP(200, body), nil })
		if _, e := a.fetchPublicGitHubRepository(context.Background(), "openai/codex"); e == nil {
			t.Fatal("provider identity mismatch accepted", body)
		}
	}
}

func TestContentCardWebReferenceMetadataManualAndRedirectBounds(t *testing.T) {
	a := &App{Config: Config{Environment: "test"}}
	var calls atomic.Int32
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return homeHTTP(200, `<html><title>fallback</title><meta property="og:title" content="真实网页 &amp; 标题"><meta name="description" content="网页说明"><meta property="og:image" content="http://127.0.0.1/private"><script>secret()</script></html>`), nil
	})
	c, e := a.contentCardResolve(context.Background(), contentCardResolveInput{Type: "link", URL: "https://example.org/reference"})
	if e != nil || c.Title != "真实网页 & 标题" || c.Description != "网页说明" || c.ImageURL != "" || c.MetadataSource != "provider" {
		t.Fatal(c, e)
	}
	manual, e := a.contentCardResolve(context.Background(), contentCardResolveInput{Type: "link", URL: "https://example.org/reference", Title: "我写的引用", Description: "作者说明"})
	if e != nil || manual.MetadataSource != "author" || calls.Load() != 1 {
		t.Fatal("manual metadata fetched or mislabeled", manual, e, calls.Load())
	}
	for _, target := range []string{"https://127.0.0.1/secret", "http://example.org/insecure", "https://example.org/loop"} {
		calls.Store(0)
		homeFake(a, func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			response := homeHTTP(302, "")
			response.Header.Set("Location", target)
			return response, nil
		})
		if _, e := a.contentCardResolve(context.Background(), contentCardResolveInput{Type: "link", URL: "https://example.org/reference"}); e == nil {
			t.Fatal("redirect accepted", target)
		}
		if calls.Load() > 3 {
			t.Fatal("redirect unbounded", calls.Load())
		}
		if strings.Contains(target, "127.0.0.1") && calls.Load() != 1 {
			t.Fatal("private redirect contacted", calls.Load())
		}
	}
	homeFake(a, func(*http.Request) (*http.Response, error) { return homeHTTP(200, strings.Repeat("x", (1<<20)+1)), nil })
	if _, e := a.contentCardResolve(context.Background(), contentCardResolveInput{Type: "link", URL: "https://example.org/reference"}); e == nil {
		t.Fatal("oversized HTML accepted")
	}
	homeFake(a, func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network contains provider-key-never-expose")
	})
	if _, e := a.contentCardResolve(context.Background(), contentCardResolveInput{Type: "link", URL: "https://example.org/reference"}); e == nil || strings.Contains(e.Error(), "never-expose") {
		t.Fatal("provider failure not redacted", e)
	}
}

func TestContentCardSteamAndAppleResolveActualMetadata(t *testing.T) {
	a := &App{Config: Config{Environment: "test"}}
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "store.steampowered.com":
			return homeHTTP(200, `{"730":{"success":true,"data":{"name":"真实游戏","steam_appid":730,"header_image":"https://shared.fastly.steamstatic.com/store_item_assets/steam/apps/730/header.jpg","short_description":"真实商店介绍"}}}`), nil
		case "itunes.apple.com":
			return homeHTTP(200, `{"results":[{"wrapperType":"collection","collectionType":"Album","collectionId":123456,"collectionName":"真实专辑","artistName":"音乐人","artworkUrl100":"https://is1-ssl.mzstatic.com/image/thumb/a/100x100bb.jpg"}]}`), nil
		case "music.apple.com":
			return homeHTTP(200, `<meta property="og:title" content="真实歌单"><meta property="og:image" content="https://is1-ssl.mzstatic.com/image/thumb/a/100x100bb.jpg">`), nil
		default:
			t.Fatal("unexpected host", r.URL)
			return nil, nil
		}
	})
	for _, in := range []contentCardResolveInput{{Type: "steam", URL: "https://store.steampowered.com/app/730/"}, {Type: "apple_music", URL: "https://music.apple.com/cn/album/123456"}, {Type: "apple_music", URL: "https://music.apple.com/cn/playlist/example/pl.u-test"}} {
		c, e := a.contentCardResolve(context.Background(), in)
		if e != nil || !strings.HasPrefix(c.Title, "真实") || c.ImageURL == "" {
			t.Fatal(in, c, e)
		}
		if _, e := RenderMarkdown(contentCardMarkdown(c)); e != nil {
			t.Fatal("resolved card did not render", c, e)
		}
	}
}

func TestContentCardResolveAuthorizationAndRoleRecheck(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	_, owner, ownerCSRF := testIdentity(t, a, "owner")
	author, authorCookie, authorCSRF := testIdentity(t, a, "author")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	var calls atomic.Int32
	homeFake(a, func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return homeHTTP(200, `<title>公开网页</title>`), nil
	})
	input := contentCardResolveInput{Type: "link", URL: "https://example.org/reference"}
	contentWant(t, contentCall(t, a, nil, "", "POST", "/api/v1/editor/cards/resolve", input, 0, ""), 401)
	contentWant(t, contentCall(t, a, reader, readerCSRF, "POST", "/api/v1/editor/cards/resolve", input, 0, ""), 403)
	contentWant(t, contentCall(t, a, owner, "wrong", "POST", "/api/v1/editor/cards/resolve", input, 0, ""), 403)
	if calls.Load() != 0 {
		t.Fatal("unauthorized resolution fetched network")
	}
	for _, identity := range []struct {
		cookie *http.Cookie
		csrf   string
	}{{owner, ownerCSRF}, {authorCookie, authorCSRF}} {
		response := contentWant(t, contentCall(t, a, identity.cookie, identity.csrf, "POST", "/api/v1/editor/cards/resolve", input, 0, ""), 200)
		if response["inline_markdown"] == "" || response["markdown"] == "" {
			t.Fatal("portable Markdown missing", response)
		}
	}
	homeFake(a, func(*http.Request) (*http.Response, error) {
		if _, e := a.Pool.Exec(context.Background(), "UPDATE users SET role='reader' WHERE id=$1", author.ID); e != nil {
			t.Fatal(e)
		}
		return homeHTTP(200, `<title>公开网页</title>`), nil
	})
	contentWant(t, contentCall(t, a, authorCookie, authorCSRF, "POST", "/api/v1/editor/cards/resolve", input, 0, ""), 403)
}

func TestContentCardMediaPublicationRestoreAndSearch(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterMedia()
	a.RegisterSearch()
	_, cookie, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	upload := contentWant(t, mediaTestUpload(t, a, cookie, csrf, RandID(), 91), 201)
	mediaID := upload["media_id"].(string)
	c := cardFixture("github")
	c.ImageURL = "/media/" + mediaID
	markdown := "# 内容卡片\n\n" + contentCardMarkdown(c)
	d := map[string]any{"client_draft_id": RandID(), "title": "卡片媒体生命周期", "markdown": markdown, "media_ids": []string{mediaID}}
	contentWant(t, contentCall(t, a, other, otherCSRF, "POST", "/api/v1/posts", d, 0, ""), 409)
	created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", d, 0, ""), 201)
	id := created["id"].(string)
	preview := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/editor/preview", map[string]any{"markdown": markdown, "seq": 7}, 0, ""), 200)
	contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/media/"+mediaID, nil, 0, ""), 409)
	contentWant(t, contentCall(t, a, nil, "", "GET", "/media/"+mediaID, nil, 0, ""), 404)
	published := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	before := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	if before["sanitized_html"] != preview["sanitized_html"] || preview["renderer_version"] != RendererVersion {
		t.Fatal("preview/publish card pipelines diverged", preview, before)
	}
	if !strings.Contains(before["sanitized_html"].(string), "/media/"+mediaID) {
		t.Fatal("published card image lost", before)
	}
	if response := contentCall(t, a, nil, "", "GET", "/media/"+mediaID, nil, 0, ""); response.Code != 200 {
		t.Fatal("published card image not public", response.Code)
	}
	search := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/search?q=可搜索的快照标题", nil, 0, ""), 200)
	if len(search["items"].([]any)) != 1 {
		t.Fatal("card metadata not searchable", search)
	}
	saved := contentWant(t, contentCall(t, a, cookie, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", map[string]any{"title": "工作副本", "markdown": "移除卡片", "media_ids": []string{}, "expected_draft_version": 1}, contentNumber(published, "version"), ""), 200)
	contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/media/"+mediaID, nil, 0, ""), 409)
	contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/restore", map[string]any{"revision_id": published["revision_id"]}, contentNumber(saved, "version"), ""), 200)
	studio := contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/studio/posts/"+id, nil, 0, ""), 200)
	if studio["markdown"] != markdown || len(studio["media_ids"].([]any)) != 1 {
		t.Fatal("restore lost snapshot/media refs", studio)
	}
	after := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	if after["sanitized_html"] != before["sanitized_html"] || after["revision_id"] != before["revision_id"] {
		t.Fatal("draft restore mutated published immutable HTML")
	}
}
