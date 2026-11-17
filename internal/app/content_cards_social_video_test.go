package app

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	xhtml "golang.org/x/net/html"
)

func socialVideoFixtures() []contentCard {
	return []contentCard{
		{V: 1, Type: "link", Display: "social", URL: "https://x.com/example/status/123", Title: "分享卡片测试标题", Subtitle: "作者 @example", Description: "第一行分享文字\n第二行说明", MetadataSource: "author"},
		{V: 1, Type: "bilibili", URL: "https://www.bilibili.com/video/BV1xx411c7mD/?p=2", Title: "视频卡片测试标题", Subtitle: "作者填写的创作者", Description: "作者填写的视频说明", MetadataSource: "author"},
	}
}

func TestContentSocialSnapshotsPreserveTextAndLocalMedia(t *testing.T) {
	c := socialVideoFixtures()[0]
	c.Title = `<script>标题 & "文字"</script>`
	c.Subtitle = `<img src=x onerror="bad()">`
	c.Description += "\n<script>只是文字</script>"
	c.ImageURL = "/media/0123456789abcdef0123456789abcdef"
	r, err := RenderMarkdown("# 正文\n\n" + contentCardMarkdown(c))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`class="content-social"`, `class="content-social__meta"`, `class="content-social__platform">X`, `class="content-social__author"`, `class="content-social__title"`, `class="content-social__text"`, `class="content-social__image"`, `class="content-social__url">x.com`, `target="_blank"`, `rel="noopener noreferrer`, "第一行分享文字\n第二行说明", "&lt;script&gt;"} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("share snapshot lost %q: %s", want, r.HTML)
		}
	}
	for _, unsafe := range []string{"<script", "<iframe", "platform.twitter", "widgets.js", "player.bilibili.com"} {
		if strings.Contains(r.HTML, unsafe) {
			t.Errorf("share snapshot contains active third-party markup %q: %s", unsafe, r.HTML)
		}
	}
	if len(r.MediaIDs) != 1 || r.MediaIDs[0] != "0123456789abcdef0123456789abcdef" || len(r.TOC) != 1 || !strings.Contains(r.Text, c.Title) || !strings.Contains(r.Text, c.Subtitle) || !strings.Contains(r.Version, "social-video-v1") {
		t.Fatal("social text, media, TOC or renderer identity changed", r)
	}
	for _, candidate := range []struct{ host, platform string }{{"x.com", "X"}, {"twitter.com", "X"}, {"m.weibo.cn", "微博"}, {"www.xiaohongshu.com", "小红书"}, {"xhslink.com", "小红书"}, {"v.douyin.com", "抖音"}, {"www.bilibili.com", "B站"}, {"b23.tv", "B站"}, {"evilx.com", "分享"}, {"x.com.example.org", "分享"}} {
		if got := contentSocialPlatform(candidate.host); got != candidate.platform {
			t.Errorf("platform label for %s: got %q, want %q", candidate.host, got, candidate.platform)
		}
	}
}

func TestContentSocialVideoStrictFieldsAndInlineBoundary(t *testing.T) {
	for _, c := range socialVideoFixtures() {
		raw, _ := json.Marshal(c)
		decoded, known, err := parseContentCard(raw)
		if err != nil || !known || !reflect.DeepEqual(decoded, c) {
			t.Fatal("snapshot round trip lost metadata", decoded, known, err)
		}
		for _, changed := range []string{
			string(raw[:len(raw)-1]) + `,"title":"duplicate"}`,
			string(raw[:len(raw)-1]) + `,"\u0074itle":"duplicate"}`,
			string(raw[:len(raw)-1]) + `,"html":"<iframe>"}`,
			string(raw[:len(raw)-1]) + `,"image_url":null}`,
			strings.Replace(string(raw), `"title":`, `"Title":`, 1),
		} {
			if _, known, err := parseContentCard([]byte(changed)); !known || err == nil {
				t.Errorf("strict %s snapshot accepted %s (%v)", c.Type, changed, err)
			}
			articleImportWantError(t, articleImportCardDocument(changed), 400)
		}
		for _, unknown := range []string{strings.Replace(string(raw), `"v":1`, `"v":null`, 1), strings.Replace(string(raw), `"v":1`, `"v":99`, 1)} {
			rendered, err := RenderMarkdown("```painter-card\n" + unknown + "\n```")
			if err != nil || !strings.Contains(rendered.HTML, "<pre") || strings.Contains(rendered.HTML, `class="content-social"`) || strings.Contains(rendered.HTML, `class="content-video"`) {
				t.Fatal("unknown version lost its inert source fallback", rendered, err)
			}
			articleImportWantError(t, articleImportCardDocument(unknown), 400)
		}
		if _, err := RenderMarkdown(contentCardInlineMarkdown(c)); err == nil {
			t.Errorf("block-only %s snapshot was accepted inline", c.Type)
		}
		articleImportWantError(t, articleImportDocument(t, map[string]any{"type": "markdown", "markdown": contentCardInlineMarkdown(c)}), 400)
		c.ImageURL = "https://www.bilibili.com/poster.jpg"
		if _, err := RenderMarkdown(contentCardMarkdown(c)); err == nil {
			t.Errorf("%s snapshot accepted a remote poster", c.Type)
		}
	}
	social := socialVideoFixtures()[0]
	for _, description := range []string{strings.Repeat("中", 1001), "control\tcharacter", "control\x00character"} {
		social.Description = description
		if _, err := RenderMarkdown(contentCardMarkdown(social)); err == nil {
			t.Errorf("social text control/limit accepted %q", description)
		}
	}
	social.Description = strings.Repeat("中", 998) + "\r\n"
	if _, err := RenderMarkdown(contentCardMarkdown(social)); err != nil {
		t.Fatal("1000-character multiline social text rejected", err)
	}
	for _, display := range []string{"", "card", "title"} {
		social.Display, social.Description = display, "first\nsecond"
		if _, err := RenderMarkdown(contentCardMarkdown(social)); err == nil {
			t.Errorf("social multiline relaxation leaked to display %q", display)
		}
	}
	video := socialVideoFixtures()[1]
	video.Description = "first\nsecond"
	if _, err := RenderMarkdown(contentCardMarkdown(video)); err == nil {
		t.Fatal("social multiline relaxation leaked to Bilibili")
	}
}

func TestContentBilibiliReferencesCanonicalAndClosed(t *testing.T) {
	for _, candidate := range []struct{ input, want string }{
		{"https://www.bilibili.com/video/BV1xx411c7mD", "https://www.bilibili.com/video/BV1xx411c7mD/"},
		{"https://www.bilibili.com/video/BV1xx411c7mD/?p=1", "https://www.bilibili.com/video/BV1xx411c7mD/"},
		{"https://www.bilibili.com/video/BV1xx411c7mD?p=1000", "https://www.bilibili.com/video/BV1xx411c7mD/?p=1000"},
		{"https://www.bilibili.com/video/av1/", "https://www.bilibili.com/video/av1/"},
		{"https://www.bilibili.com/video/av999999999999999999/?p=2", "https://www.bilibili.com/video/av999999999999999999/?p=2"},
	} {
		canonical, err := contentBilibiliURL(candidate.input)
		if err != nil || canonical != candidate.want {
			t.Errorf("canonical %s: got %q, want %q (%v)", candidate.input, canonical, candidate.want, err)
		}
		video := socialVideoFixtures()[1]
		video.URL = canonical
		if _, err := RenderMarkdown(contentCardMarkdown(video)); err != nil {
			t.Errorf("canonical reference did not render: %s (%v)", canonical, err)
		}
		if candidate.input != canonical {
			video.URL = candidate.input
			if _, err := RenderMarkdown(contentCardMarkdown(video)); err == nil {
				t.Errorf("snapshot silently accepted noncanonical source: %s", video.URL)
			}
		}
	}
	for _, invalid := range []string{
		"http://www.bilibili.com/video/BV1xx411c7mD/", "https://bilibili.com/video/BV1xx411c7mD/", "https://m.bilibili.com/video/BV1xx411c7mD/", "https://www.bilibili.com.evil.test/video/BV1xx411c7mD/", "https://b23.tv/example", "https://player.bilibili.com/player.html?bvid=BV1xx411c7mD", "<iframe src=\"https://player.bilibili.com/\"></iframe>",
		"https://user@www.bilibili.com/video/BV1xx411c7mD/", "https://www.bilibili.com:443/video/BV1xx411c7mD/", "https://www.bilibili.com/video/bv1xx411c7mD/", "https://www.bilibili.com/video/BV1xx411c7m/", "https://www.bilibili.com/video/BV1xx411c7mDD/", "https://www.bilibili.com/video/BV1xx411c7m_/", "https://www.bilibili.com/video/av0/", "https://www.bilibili.com/video/av01/", "https://www.bilibili.com/video/av1000000000000000000/", "https://www.bilibili.com/video/AV1/",
		"https://www.bilibili.com/video/BV1xx411c7mD/#", "https://www.bilibili.com/video/BV1xx411c7mD/#part", "https://www.bilibili.com/video/BV1xx411c7mD/?", "https://www.bilibili.com/video/BV1xx411c7mD/?p=0", "https://www.bilibili.com/video/BV1xx411c7mD/?p=1001", "https://www.bilibili.com/video/BV1xx411c7mD/?p=01", "https://www.bilibili.com/video/BV1xx411c7mD/?p=2&p=3", "https://www.bilibili.com/video/BV1xx411c7mD/?p=2&", "https://www.bilibili.com/video/BV1xx411c7mD/?autoplay=1", "https://www.bilibili.com/video/BV1xx411c7mD/?%70=2", "https://www.bilibili.com/video/BV1xx411c7mD/?p=%32", "https://www.bilibili.com/video/BV1xx411c7mD/?p=2#fragment", "https://www.bilibili.com/video/%42V1xx411c7mD/", "https://www.bilibili.com/video/../video/BV1xx411c7mD/", "https://www.bilibili.com/video/BV1xx411c7mD//",
	} {
		if canonical, err := contentBilibiliURL(invalid); err == nil {
			t.Errorf("unsafe Bilibili reference accepted: %s => %s", invalid, canonical)
		}
	}
}

func TestContentBilibiliResolveIsLocalAndRendererInert(t *testing.T) {
	a := &App{Config: Config{Environment: "test"}}
	var requests atomic.Int32
	homeFake(a, func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		t.Error("Bilibili reference unexpectedly contacted an external provider")
		return nil, nil
	})
	for _, title := range []string{"", "我填写的视频标题"} {
		c, err := a.contentCardResolve(context.Background(), contentCardResolveInput{Type: "bilibili", URL: "https://www.bilibili.com/video/BV1xx411c7mD?spm_id_from=333.337.search-card.all.click&vd_source=discarded-attribution&p=1", Title: title, Description: "作者说明"})
		wantTitle := title
		if title == "" {
			wantTitle = "哔哩哔哩视频"
		}
		if err != nil || c.URL != "https://www.bilibili.com/video/BV1xx411c7mD/" || c.Title != wantTitle || c.Description != "作者说明" || c.MetadataSource != "author" || c.ResolvedAt != "" || requests.Load() != 0 {
			t.Fatal("local resolver invented or fetched platform metadata", c, err, requests.Load())
		}
		if markdown := contentCardMarkdown(c); strings.Contains(markdown, "spm_id_from") || strings.Contains(markdown, "vd_source") || strings.Contains(markdown, "discarded-attribution") {
			t.Fatal("resolver retained discarded tracking parameters in the snapshot", markdown)
		}
	}
	c := socialVideoFixtures()[1]
	c.ImageURL = "/media/0123456789abcdef0123456789abcdef"
	c.Title = `<script>字面标题</script>`
	c.Subtitle = `<img src=x onerror="bad()">`
	c.Description = `<iframe src="https://evil.test/">文字</iframe>`
	r, err := RenderMarkdown(contentCardMarkdown(c))
	if err != nil {
		t.Fatal(err)
	}
	root, err := xhtml.Parse(strings.NewReader(r.HTML))
	if err != nil {
		t.Fatal(err)
	}
	buttons := friendWebNodes(root, func(n *xhtml.Node) bool { return n.Type == xhtml.ElementNode && n.Data == "button" })
	if len(buttons) != 1 || friendWebAttr(buttons[0], "class") != "content-video__load" || friendWebAttr(buttons[0], "type") != "button" || !strings.Contains(r.HTML, `data-video-load=""`) || !strings.Contains(r.HTML, `hidden=""`) {
		t.Fatal("inert player button lost strict enhancement attributes", r.HTML)
	}
	links := friendWebNodes(root, func(n *xhtml.Node) bool { return n.Type == xhtml.ElementNode && n.Data == "a" })
	if len(links) != 1 || friendWebAttr(links[0], "class") != "content-video__source" || friendWebAttr(links[0], "href") != c.URL || friendWebAttr(links[0], "target") != "_blank" || !strings.Contains(friendWebAttr(links[0], "rel"), "noopener noreferrer") {
		t.Fatal("video source lost its static, safe fallback", r.HTML)
	}
	for _, want := range []string{`class="content-video" data-content-video="bilibili"`, `class="content-video__stage"`, `class="content-video__poster"`, `class="content-video__details"`, `class="content-video__title"`, `class="content-video__subtitle"`, `class="content-video__description"`, `class="content-video__destination">在B站打开`, "&lt;script&gt;", "&lt;iframe"} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("video placeholder lost %q: %s", want, r.HTML)
		}
	}
	if strings.Contains(r.HTML, "<iframe") || strings.Contains(r.HTML, "<script") || strings.Contains(r.HTML, "player.bilibili.com") || len(r.MediaIDs) != 1 || !strings.Contains(r.Text, c.Title) || !strings.Contains(r.Text, c.Description) {
		t.Fatal("video SSR contains active embed or lost local-media/search data", r)
	}
	for _, raw := range []string{`<iframe src="https://player.bilibili.com/player.html?bvid=BV1xx411c7mD"></iframe>`, `<div class="content-video" data-content-video="bilibili"><button data-video-load onclick="bad()">spoofed</button></div>`, `<a class="content-social" href="https://x.com/">spoofed</a>`} {
		result, err := RenderMarkdown(raw)
		if err != nil || strings.Contains(result.HTML, "<iframe") || strings.Contains(result.HTML, "<button") || strings.Contains(result.HTML, `class="content-video"`) || strings.Contains(result.HTML, `class="content-social"`) {
			t.Fatal("raw HTML spoofed a controlled card or embed", result, err)
		}
	}
}

func TestContentBilibiliPastedTrackingParametersAreBoundedAndDiscarded(t *testing.T) {
	base := "https://www.bilibili.com/video/BV1B7411m7LV/"
	for _, candidate := range []struct{ suffix, canonical string }{
		{"?spm_id_from=333.337.search-card.all.click&vd_source=abc&p=2", "?p=2"},
		{"?p=1000&vd_source=abc&spm_id_from=web", "?p=1000"},
		{"?vd_source=abc&p=1", ""},
		{"?vd_source=&spm_id_from=", ""},
		{"?vd_source=%E4%B8%AD%E6%96%87", ""},
		{"?spm_id_from=" + strings.Repeat("x", 512), ""},
	} {
		canonical, err := contentBilibiliInputURL(base + candidate.suffix)
		if err != nil || canonical != base+candidate.canonical {
			t.Errorf("pasted query %s: got %q, want %q (%v)", candidate.suffix, canonical, base+candidate.canonical, err)
		}
		if _, err := contentBilibiliURL(base + candidate.suffix); err == nil {
			t.Errorf("strict snapshot parser accepted pasted tracking query %q", candidate.suffix)
		}
	}
	for _, suffix := range []string{
		"?", "?spm_id_from=a&spm_id_from=b", "?vd_source=a&vd_source=b", "?vd_source=a&p=2&p=3", "?vd_source=a&p=01", "?vd_source=a&p=0", "?vd_source=a&p=1001", "?vd_source=a&p=%32", "?vd_source=a&%70=2", "?%76d_source=a", "?vd_source=a&unknown=b", "?vd_source=a&", "?vd_source=a&&p=2", "?vd_source", "?vd_source=%", "?vd_source=%ff", "?vd_source=a#fragment", "?vd_source=a#", "?vd_source=" + strings.Repeat("x", 513),
	} {
		if canonical, err := contentBilibiliInputURL(base + suffix); err == nil {
			t.Errorf("unsafe pasted query accepted %s => %s", suffix, canonical)
		}
	}
	card := socialVideoFixtures()[1]
	card.URL = base + "?spm_id_from=web&vd_source=abc&p=2"
	if _, err := RenderMarkdown(contentCardMarkdown(card)); err == nil {
		t.Fatal("renderer accepted tracking parameters instead of requiring canonical snapshot URL")
	}
	raw, _ := json.Marshal(card)
	articleImportWantError(t, articleImportCardDocument(string(raw)), 400)
}

func TestContentSocialVideoImportRoundTrip(t *testing.T) {
	cards := socialVideoFixtures()
	for i := range cards {
		cards[i].ImageURL = "/media/0123456789abcdef0123456789abcdef"
		for _, block := range []any{map[string]any{"type": "card", "card": cards[i]}, map[string]any{"type": "markdown", "markdown": contentCardMarkdown(cards[i])}} {
			compiled, err := compileArticleImport(articleImportDocument(t, block))
			if err != nil || compiled.CardCount != 1 || len(compiled.MediaIDs) != 1 || !strings.Contains(compiled.Markdown, `"image_url"`) {
				t.Fatal("import lost canonical snapshot or local media", compiled, err)
			}
			rendered, err := RenderMarkdown(compiled.Markdown)
			if err != nil || rendered.HTML != compiled.HTML || rendered.Text != compiled.Text || rendered.Hash != compiled.Hash {
				t.Fatal("import used a separate card rendering path", compiled, rendered, err)
			}
		}
	}
}

func TestContentSocialVideoMediaPublicationRestoreAndResolverAuthorization(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterMedia()
	a.RegisterSearch()
	_, author, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	var requests atomic.Int32
	homeFake(a, func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		t.Error("Bilibili resolution contacted provider")
		return nil, nil
	})
	input := contentCardResolveInput{Type: "bilibili", URL: "https://www.bilibili.com/video/BV1xx411c7mD?p=1", Title: "作者视频标题"}
	contentWant(t, contentCall(t, a, nil, "", "POST", "/api/v1/editor/cards/resolve", input, 0, ""), 401)
	contentWant(t, contentCall(t, a, reader, readerCSRF, "POST", "/api/v1/editor/cards/resolve", input, 0, ""), 403)
	contentWant(t, contentCall(t, a, author, "wrong", "POST", "/api/v1/editor/cards/resolve", input, 0, ""), 403)
	resolved := contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/editor/cards/resolve", input, 0, ""), 200)
	if requests.Load() != 0 || resolved["inline_markdown"] != nil || !strings.Contains(resolved["markdown"].(string), `https://www.bilibili.com/video/BV1xx411c7mD/`) {
		t.Fatal("Bilibili endpoint fetched network or lost canonical block-only contract", resolved, requests.Load())
	}
	mediaID := contentWant(t, mediaTestUpload(t, a, author, csrf, RandID(), 91), 201)["media_id"].(string)
	cards := socialVideoFixtures()
	for i := range cards {
		cards[i].ImageURL = "/media/" + mediaID
	}
	markdown := contentCardMarkdown(cards[0]) + "\n\n" + contentCardMarkdown(cards[1])
	imported := articleImportDocument(t, map[string]any{"type": "card", "card": cards[0]}, map[string]any{"type": "card", "card": cards[1]})
	preview := contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/editor/preview", map[string]any{"markdown": markdown}, 0, ""), 200)
	contentWant(t, contentCall(t, a, other, otherCSRF, "POST", "/api/v1/editor/preview", map[string]any{"markdown": markdown}, 0, ""), 409)
	contentWant(t, articleImportRequest(a, other, otherCSRF, imported), 409)
	importPreview := contentWant(t, articleImportRequest(a, author, csrf, imported), 200)
	if importPreview["sanitized_html"] != preview["sanitized_html"] {
		t.Fatal("import and editor preview diverged", importPreview, preview)
	}
	draft := map[string]any{"client_draft_id": RandID(), "title": "社交与视频媒体生命周期", "markdown": markdown}
	contentWant(t, contentCall(t, a, other, otherCSRF, "POST", "/api/v1/posts", draft, 0, ""), 409)
	created := contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/posts", draft, 0, ""), 201)
	id := created["id"].(string)
	published := contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	public := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	if public["sanitized_html"] != preview["sanitized_html"] || strings.Contains(public["sanitized_html"].(string), "<iframe") {
		t.Fatal("published video became active or preview rendering changed", public)
	}
	var draftRefs, revisionRefs int
	if err := a.Pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM post_draft_media_refs WHERE post_id=$1 AND media_id=$2),(SELECT count(*) FROM post_revision_media_refs WHERE revision_id=$3 AND media_id=$2)`, id, mediaID, published["revision_id"]).Scan(&draftRefs, &revisionRefs); err != nil || draftRefs != 1 || revisionRefs != 1 {
		t.Fatal("shared local covers lost their deduplicated media references", draftRefs, revisionRefs, err)
	}
	search := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/search?q=视频卡片测试标题", nil, 0, ""), 200)
	if len(search["items"].([]any)) != 1 {
		t.Fatal("video snapshot title is not searchable", search)
	}
	saved := contentWant(t, contentCall(t, a, author, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", map[string]any{"title": "工作草稿", "markdown": "移除两张卡片", "expected_draft_version": 1}, contentNumber(published, "version"), ""), 200)
	contentWant(t, contentCall(t, a, author, csrf, "DELETE", "/api/v1/media/"+mediaID, nil, 0, ""), 409)
	contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/studio/posts/"+id+"/restore", map[string]any{"revision_id": published["revision_id"]}, contentNumber(saved, "version"), ""), 200)
	studio := contentWant(t, contentCall(t, a, author, csrf, "GET", "/api/v1/studio/posts/"+id, nil, 0, ""), 200)
	after := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	if studio["markdown"] != markdown || len(studio["media_ids"].([]any)) != 1 || after["revision_id"] != public["revision_id"] || after["sanitized_html"] != public["sanitized_html"] {
		t.Fatal("restore lost snapshot/cover identity or changed immutable publication", studio, after)
	}
}
