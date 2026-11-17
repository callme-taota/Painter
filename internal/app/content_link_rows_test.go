package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
	importdocs "painter-next/docs"
)

func TestContentLinkTitleRowVisibleTextMediaAndSafeNativeHTML(t *testing.T) {
	c := cardFixture("link")
	c.Display = "title"
	c.URL = "https://example.org/reference?one=1&two=2#details"
	c.Title = `中文与 English <img src=x onerror="bad()"> & "标题"`
	c.Description = "只保留在快照中的描述，不应出现在单行呈现"
	c.Subtitle = "仅保存的副标题"
	c.ImageURL = "/media/0123456789abcdef0123456789abcdef"
	r, err := RenderMarkdown(contentCardMarkdown(c))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`class="content-link-row"`, `class="content-link-row__title"`, `class="content-link-row__host"`, `class="content-link-row__icon"`, `class="content-link-row__arrow"`, `target="_blank"`, `rel="noopener noreferrer`, "&lt;img", "example.org"} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("title row lost %q: %s", want, r.HTML)
		}
	}
	for _, unwanted := range []string{"<img", "<script", "content-card", c.Description, c.Subtitle, c.ImageURL, "<style", "onclick="} {
		if strings.Contains(r.HTML, unwanted) {
			t.Errorf("title row exposed nonvisible or unsafe markup %q: %s", unwanted, r.HTML)
		}
	}
	if !strings.Contains(r.Text, c.Title) || strings.Contains(r.Text, c.Description) || strings.Contains(r.Text, c.Subtitle) || len(r.MediaIDs) != 1 || r.MediaIDs[0] != "0123456789abcdef0123456789abcdef" {
		t.Fatal("visible search text or preserved media reference changed", r)
	}
	root, err := xhtml.Parse(strings.NewReader(r.HTML))
	if err != nil {
		t.Fatal(err)
	}
	links, icons, paths, spans := 0, 0, 0, 0
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		attr := func(key string) string {
			for _, a := range n.Attr {
				if a.Key == key {
					return a.Val
				}
			}
			return ""
		}
		if n.Type == xhtml.ElementNode {
			switch n.Data {
			case "a":
				links++
				if attr("href") != c.URL || attr("class") != "content-link-row" {
					t.Errorf("native destination/class changed: %v", n.Attr)
				}
			case "svg":
				icons++
				if attr("viewBox") != "0 0 24 24" || attr("aria-hidden") != "true" || attr("focusable") != "false" || attr("stroke") != "currentColor" || attr("stroke-width") != "1.65" {
					t.Errorf("controlled icon attributes lost: %v", n.Attr)
				}
			case "path":
				paths++
				if attr("d") != contentLinkRowIconPath && attr("d") != contentLinkRowArrowPath {
					t.Errorf("unexpected icon path: %v", n.Attr)
				}
			case "span":
				spans++
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	if links != 1 || icons != 2 || paths != 2 || spans != 2 {
		t.Fatal("title row must be one native link with two inert icons and title/host spans", links, icons, paths, spans, r.HTML)
	}
	if text := articleReadingText(r.HTML); !strings.Contains(text, c.Title) || strings.Contains(text, "example.org") || strings.Contains(text, c.Description) {
		t.Fatal("reading time included URL metadata or hidden snapshot text", text)
	}
	copy, err := RenderMarkdown(contentCardMarkdown(c))
	if err != nil || copy.HTML != r.HTML || copy.Hash != r.Hash || !strings.Contains(r.Version, "painter-link-title-v1") {
		t.Fatal("row rendering/version is nondeterministic", copy, err)
	}
}

func TestContentLinkDisplayKeepsLegacyCardAndInlineCompatible(t *testing.T) {
	legacy := cardFixture("link")
	before, err := RenderMarkdown(contentCardMarkdown(legacy))
	if err != nil {
		t.Fatal(err)
	}
	explicit := legacy
	explicit.Display = "card"
	after, err := RenderMarkdown(contentCardMarkdown(explicit))
	if err != nil || before.HTML != after.HTML || before.Text != after.Text || !strings.Contains(after.HTML, legacy.Description) {
		t.Fatal("explicit card presentation changed the legacy card", after, err)
	}
	for _, c := range []contentCard{legacy, explicit} {
		r, err := RenderMarkdown(contentCardInlineMarkdown(c))
		if err != nil || !strings.Contains(r.HTML, `class="content-link__anchor"`) || strings.Contains(r.HTML, "content-link-row") {
			t.Fatal("legacy inline/card inline was not preserved", r, err)
		}
	}
	title := legacy
	title.Display = "title"
	if _, err := RenderMarkdown(contentCardInlineMarkdown(title)); err == nil {
		t.Fatal("title row was accepted as a contradictory inline reference")
	}
	for _, raw := range []string{`{"v":99,"type":"link","display":"title"}`, `{"v":1,"type":"future-link","display":"title"}`} {
		r, err := RenderMarkdown("```painter-card\n" + raw + "\n```")
		if err != nil || !strings.Contains(r.HTML, "<pre") || strings.Contains(r.HTML, `class="content-link-row"`) {
			t.Fatal("unknown card lost its safe source fallback", r, err)
		}
		articleImportWantError(t, articleImportCardDocument(raw), 400)
	}
}

func TestContentLinkDisplayStrictFieldsURLsAndBounds(t *testing.T) {
	valid := `{"v":1,"type":"link","url":"https://example.org/","title":"网页标题","display":"title"}`
	for _, changed := range []string{
		strings.Replace(valid, `"display":"title"`, `"display":"inline"`, 1),
		strings.Replace(valid, `"display":"title"`, `"display":""`, 1),
		strings.Replace(valid, `"display":"title"`, `"display":null`, 1),
		strings.Replace(valid, `"display":"title"`, `"display":false`, 1),
		strings.Replace(valid, `"display":"title"`, `"display":{}`, 1),
		strings.Replace(valid, `"display":"title"`, `"display":["title"]`, 1),
		strings.Replace(valid, `"display":"title"`, `"display":"Title"`, 1),
		strings.Replace(valid, `"display":"title"`, `"Display":"title"`, 1),
		strings.Replace(valid, `"display":"title"`, `"display":"card","display":"title"`, 1),
		strings.Replace(valid, `"display":"title"`, `"display":"title","style":"color:red"`, 1),
		strings.Replace(valid, `"title":"网页标题"`, `"title":""`, 1),
	} {
		if _, known, err := parseContentCard([]byte(changed)); !known || err == nil {
			t.Errorf("invalid presentation accepted: %s (%v)", changed, err)
		}
		articleImportWantError(t, articleImportCardDocument(changed), 400)
	}
	for _, c := range []contentCard{cardFixture("github"), cardFixture("steam"), cardFixture("apple_music"), internalCardFixture("moment", "41"), internalCardFixture("phrase", "42"), panelFixture("callout"), {V: 3, Type: "layout", Root: &ContentLayoutNode{Kind: "panel", Children: []*ContentLayoutNode{{Kind: "text", Role: "body", Text: "正文"}}}}} {
		for _, display := range []string{"card", "title"} {
			c.Display = display
			if _, err := validateContentCard(c); err == nil {
				t.Errorf("presentation on nonlink accepted: v%d %s", c.V, c.Type)
			}
			if _, err := RenderMarkdown(contentCardMarkdown(c)); err == nil {
				t.Errorf("nonlink display passed parser: v%d %s", c.V, c.Type)
			}
		}
	}
	for _, target := range []string{"javascript:alert(1)", "http://example.org/", "https://127.0.0.1/", "https://user:password@example.org/", "https://example.org/%0a", "https://example.org/" + strings.Repeat("x", 2049)} {
		c := cardFixture("link")
		c.Display, c.URL = "title", target
		if _, err := RenderMarkdown(contentCardMarkdown(c)); err == nil {
			t.Errorf("unsafe title-row destination accepted: %s", target)
		}
	}
	c := cardFixture("link")
	c.Display, c.Title = "title", strings.Repeat("中", 200)
	if _, err := RenderMarkdown(contentCardMarkdown(c)); err != nil {
		t.Fatal("200-rune title was rejected", err)
	}
	c.Title += "文"
	if _, err := RenderMarkdown(contentCardMarkdown(c)); err == nil {
		t.Fatal("oversize title accepted")
	}
	c.Title, c.Description = "标题", strings.Repeat("x", 1001)
	if _, err := RenderMarkdown(contentCardMarkdown(c)); err == nil {
		t.Fatal("nonvisible snapshot fields escaped their original bounds")
	}
	for _, raw := range []string{`<svg class="content-link-row__icon" onload="alert(1)"><script>alert(1)</script><path d="M0 0" /></svg>`, `<a class="content-link-row" href="javascript:alert(1)">not a real row</a>`} {
		r, err := RenderMarkdown(raw)
		if err != nil || strings.Contains(r.HTML, "<svg") || strings.Contains(r.HTML, `class="content-link-row"`) || strings.Contains(r.HTML, "<script") {
			t.Fatal("raw HTML spoofed controlled row markup", r, err)
		}
	}
}

func TestContentLinkTitleImportRoundTripAndInlineBoundary(t *testing.T) {
	link := cardFixture("link")
	link.Display = "title"
	link.ImageURL = "/media/0123456789abcdef0123456789abcdef"
	for _, block := range []any{map[string]any{"type": "card", "card": link}, map[string]any{"type": "markdown", "markdown": contentCardMarkdown(link)}} {
		compiled, err := compileArticleImport(articleImportDocument(t, block))
		if err != nil || compiled.CardCount != 1 || !strings.Contains(compiled.HTML, `class="content-link-row"`) || !strings.Contains(compiled.Markdown, `"display":"title"`) || len(compiled.MediaIDs) != 1 {
			t.Fatal("title import or portable presentation/media data lost", compiled, err)
		}
		cardRaw := strings.TrimSuffix(strings.TrimPrefix(compiled.Markdown, "```painter-card\n"), "\n```")
		var roundTrip contentCard
		if err := json.Unmarshal([]byte(cardRaw), &roundTrip); err != nil || !reflect.DeepEqual(roundTrip, link) {
			t.Fatal("presentation toggling must preserve description, image and metadata", roundTrip, err)
		}
		rendered, err := RenderMarkdown(compiled.Markdown)
		if err != nil || rendered.HTML != compiled.HTML || rendered.Hash != compiled.Hash {
			t.Fatal("import and ordinary preview pipeline diverged", rendered, err)
		}
	}
	for _, display := range []string{"", "card"} {
		link.Display = display
		compiled, err := compileArticleImport(articleImportDocument(t, map[string]any{"type": "markdown", "markdown": contentCardInlineMarkdown(link)}))
		if err != nil || compiled.CardCount != 1 || !strings.Contains(compiled.HTML, `class="content-link__anchor"`) {
			t.Fatal("existing inline import contract regressed", compiled, err)
		}
	}
	link.Display = "title"
	articleImportWantError(t, articleImportDocument(t, map[string]any{"type": "markdown", "markdown": contentCardInlineMarkdown(link)}), 400)
	// A manually encoded title mark must fail too; generation is not the boundary.
	raw, _ := json.Marshal(link)
	inline := `[标题](https://example.org/reference "` + contentCardInlinePrefix + base64.RawURLEncoding.EncodeToString(raw) + `")`
	articleImportWantError(t, articleImportDocument(t, map[string]any{"type": "markdown", "markdown": inline}), 400)
	var schema struct {
		Defs map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal([]byte(importdocs.ArticleImportSchema), &schema); err != nil {
		t.Fatal(err)
	}
	var display struct {
		Enum    []string `json:"enum"`
		Default string   `json:"default"`
	}
	if json.Unmarshal(schema.Defs["linkCard"].Properties["display"], &display) != nil || strings.Join(display.Enum, ",") != "card,title,social" || display.Default != "card" {
		t.Fatal("embedded import schema does not describe the exact display contract", display)
	}
	for _, kind := range []string{"githubCard", "steamCard", "apple_musicCard", "momentCard", "phraseCard", "calloutCard", "metricsCard", "summaryCard", "layoutCard"} {
		if _, ok := schema.Defs[kind].Properties["display"]; ok {
			t.Errorf("schema admitted presentation on nonlink %s", kind)
		}
	}
}

func TestContentLinkTitlePublicationRetainsMediaAuthorizationAndSnapshot(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterMedia()
	_, author, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	own := contentWant(t, mediaTestUpload(t, a, author, csrf, RandID(), 91), 201)["media_id"].(string)
	foreign := contentWant(t, mediaTestUpload(t, a, other, otherCSRF, RandID(), 92), 201)["media_id"].(string)
	var post, previewHTML string
	c := cardFixture("link")
	c.Display = "title"
	for _, media := range []struct {
		id      string
		allowed bool
	}{{own, true}, {foreign, false}, {RandID(), false}} {
		c.ImageURL = "/media/" + media.id
		status := 409
		if media.allowed {
			status = 200
		}
		preview := contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/editor/preview", map[string]any{"markdown": contentCardMarkdown(c)}, 0, ""), status)
		contentWant(t, articleImportRequest(a, author, csrf, articleImportDocument(t, map[string]any{"type": "card", "card": c})), status)
		if media.allowed {
			status = 201
		}
		created := contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "单行网页呈现", "markdown": contentCardMarkdown(c)}, 0, ""), status)
		if media.allowed {
			post, previewHTML = created["id"].(string), preview["sanitized_html"].(string)
		}
	}
	c.ImageURL = "/media/" + own
	published := contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/studio/posts/"+post+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	public := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+post, nil, 0, ""), 200)
	if public["sanitized_html"] != previewHTML || !strings.Contains(previewHTML, `class="content-link-row"`) || strings.Contains(previewHTML, "<img") || strings.Contains(previewHTML, c.Description) {
		t.Fatal("preview and immutable publication do not share title-only rendering", public, previewHTML)
	}
	var draftRefs, revisionRefs int
	if err := a.Pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM post_draft_media_refs WHERE post_id=$1 AND media_id=$2),(SELECT count(*) FROM post_revision_media_refs WHERE revision_id=$3 AND media_id=$2)`, post, own, published["revision_id"]).Scan(&draftRefs, &revisionRefs); err != nil || draftRefs != 1 || revisionRefs != 1 {
		t.Fatal("nonvisible preserved snapshot image lost its permissions/cleanup references", draftRefs, revisionRefs, err)
	}
	c.Display = "card"
	contentWant(t, contentCall(t, a, author, csrf, "PUT", "/api/v1/studio/posts/"+post+"/draft", map[string]any{"title": "单行网页呈现", "markdown": contentCardMarkdown(c), "expected_draft_version": 1}, contentNumber(published, "version"), ""), 200)
	studio := contentWant(t, contentCall(t, a, author, csrf, "GET", "/api/v1/studio/posts/"+post, nil, 0, ""), 200)
	after := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+post, nil, 0, ""), 200)
	if studio["markdown"] != contentCardMarkdown(c) || !strings.Contains(studio["markdown"].(string), c.Description) || after["sanitized_html"] != public["sanitized_html"] || after["revision_id"] != public["revision_id"] {
		t.Fatal("presentation switching lost source data or implicitly republished the draft", studio, after)
	}
}
