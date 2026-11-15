package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func layoutText(role, text string) *ContentLayoutNode {
	return &ContentLayoutNode{Kind: "text", Role: role, Text: text}
}

func layoutFixture() contentCard {
	return contentCard{V: 3, Type: "layout", Root: &ContentLayoutNode{Kind: "panel", Tone: "insight", Children: []*ContentLayoutNode{
		layoutText("title", "组合观察"),
		layoutText("body", "同一面板可以组合正文、指标和来源。\n不增加专用卡片类型。"),
		{Kind: "grid", Children: []*ContentLayoutNode{
			{Kind: "stack", Children: []*ContentLayoutNode{layoutText("label", "公开样本"), layoutText("value", "42"), layoutText("body", "指标旁也可以有正文。"), {Kind: "link", Text: "原始资料", URL: "https://example.org/report?a=1&b=2"}}},
			{Kind: "stack", Children: []*ContentLayoutNode{layoutText("label", "阅读提示"), layoutText("detail", "保持上下文\r\n再核对来源"), {Kind: "stack", Children: []*ContentLayoutNode{layoutText("body", "分组中的补充说明")}}}},
		}},
		{Kind: "stack", Children: []*ContentLayoutNode{layoutText("detail", "正文末尾的分组"), {Kind: "link", Text: "继续阅读", URL: "https://example.org/next"}}},
	}}}
}

func TestContentLayoutComposableRenderAndMetadata(t *testing.T) {
	c := layoutFixture()
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	parsed, known, err := parseContentCard(raw)
	if err != nil || !known || parsed.Root == nil || parsed.Type != "layout" {
		t.Fatal("layout not recognized", parsed, known, err)
	}
	source := "# 正文标题\n\n" + contentCardMarkdown(c)
	rendered, err := RenderMarkdown(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`class="content-panel content-panel--layout content-panel--insight"`, `class="content-panel__grid"`, `class="content-panel__item"`, `class="content-panel__stack"`, `class="content-panel__title"`, `class="content-panel__value"`, `class="content-panel__body"`, `class="content-panel__detail"`, `class="content-panel__source"`, `href="https://example.org/report?a=1&amp;b=2"`, `target="_blank"`, `rel="noopener noreferrer nofollow"`} {
		if !strings.Contains(rendered.HTML, want) {
			t.Errorf("layout markup missing %q: %s", want, rendered.HTML)
		}
	}
	for _, want := range []string{"组合观察", "公开样本", "42", "指标旁也可以有正文", "原始资料", "分组中的补充说明", "继续阅读"} {
		if !strings.Contains(rendered.Text, want) {
			t.Errorf("layout search text missing %q", want)
		}
	}
	if len(rendered.TOC) != 1 || rendered.TOC[0].Text != "正文标题" || len(rendered.MediaIDs) != 0 || !strings.Contains(rendered.Version, "layout-v3") {
		t.Fatal("layout changed heading/media metadata", rendered)
	}
	copy, err := RenderMarkdown(source)
	if err != nil || copy.HTML != rendered.HTML || copy.Hash != rendered.Hash {
		t.Fatal("layout rendering is not deterministic", err)
	}
}

func TestContentLayoutLegacyPanelHTMLParity(t *testing.T) {
	golden := map[string]string{
		"callout": `<div class="content-panel content-panel--callout content-panel--insight"><p class="content-panel__title">结构化内容</p><p class="content-panel__body">洞察正文<br />第二行保留</p></div>` + "\n",
		"metrics": `<div class="content-panel content-panel--metrics"><p class="content-panel__title">结构化内容</p><div class="content-panel__grid"><div class="content-panel__item"><p class="content-panel__label">响应延迟</p><p class="content-panel__value">120 ms</p><p class="content-panel__detail">较上期改善</p><a class="content-panel__source" href="https://example.org/report?a=1&amp;b=2" target="_blank" rel="noopener noreferrer">公开报告</a></div><div class="content-panel__item"><p class="content-panel__label">样本数</p><p class="content-panel__value">42</p></div></div></div>` + "\n",
		"summary": `<div class="content-panel content-panel--summary"><p class="content-panel__title">结构化内容</p><div class="content-panel__grid"><div class="content-panel__item"><p class="content-panel__label">主要发现</p><p class="content-panel__body">摘要正文<br />补充说明</p></div><div class="content-panel__item"><p class="content-panel__label">下一步</p><p class="content-panel__body">继续核实公开资料</p></div></div></div>` + "\n",
	}
	for kind, want := range golden {
		t.Run(kind, func(t *testing.T) {
			c := panelFixture(kind)
			if got := contentPanelHTML(c); got != want {
				t.Fatalf("v2 HTML changed during compilation\nwant: %s\ngot:  %s", want, got)
			}
			raw, _ := json.Marshal(c)
			if strings.Contains(string(raw), `"root"`) || strings.Contains(string(raw), `"kind"`) {
				t.Fatal("legacy source was rewritten as layout data", string(raw))
			}
		})
	}
}

func TestContentLayoutRejectsUnsafeOrAmbiguousJSON(t *testing.T) {
	wrap := func(node string) string { return `{"v":3,"type":"layout","root":` + node + `}` }
	leaf := `{"kind":"text","role":"body","text":"safe"}`
	panel := func(nodes string) string { return `{"kind":"panel","children":[` + nodes + `]}` }
	invalid := []string{
		`{"v":3,"v":3,"type":"layout","root":` + panel(leaf) + `}`,
		`{"v":3,"type":"layout","root":` + panel(leaf) + `,"root":` + panel(leaf) + `}`,
		`{"v":3,"type":"layout","root":` + panel(leaf) + `,"title":"legacy"}`,
		`{"v":3,"type":"layout","root":null}`,
		`{"v":3,"type":"layout"}`,
		wrap(`{"kind":"panel","children":[` + leaf + `],"tone":null}`),
		wrap(`{"kind":"panel","children":[` + leaf + `],"tone":""}`),
		wrap(`{"kind":"panel","children":[` + leaf + `],"tone":"danger"}`),
		wrap(`{"kind":"panel","children":[` + leaf + `],"style":"position:fixed"}`),
		wrap(`{"kind":"panel","children":[` + leaf + `],"children":[]}`),
		wrap(`{"kind":"panel","children":[]}`),
		wrap(`{"kind":"panel","children":null}`),
		wrap(`{"kind":"panel","children":[null]}`),
		wrap(`{"Kind":"panel","children":[` + leaf + `]}`),
		wrap(leaf),
		wrap(panel(panel(leaf))),
		wrap(panel(`{"kind":"grid","children":[` + leaf + `]}`)),
		wrap(panel(`{"kind":"grid","columns":2,"children":[{"kind":"stack","children":[` + leaf + `]}]}`)),
		wrap(panel(`{"kind":"text","role":"body","text":"safe","text":"replacement"}`)),
		wrap(panel(`{"kind":"text","role":"body","text":"safe","Text":"other casing"}`)),
		wrap(panel(`{"kind":"text","role":"script","text":"x"}`)),
		wrap(panel(`{"kind":"text","role":"body","text":" "}`)),
		wrap(panel(`{"kind":"text","role":"detail","text":"\n"}`)),
		wrap(panel(`{"kind":"text","role":"body","text":"bad\ttext"}`)),
		wrap(panel(`{"kind":"text","role":"title","text":"two\nlines"}`)),
		wrap(panel(`{"kind":"text","role":"body","text":"safe","html":"<b>bad</b>"}`)),
		wrap(panel(`{"kind":"text","role":"body","text":null}`)),
		wrap(panel(`{"kind":"text","role":"body"}`)),
		wrap(panel(`{"kind":"text","text":"missing role"}`)),
		wrap(panel(`{"kind":"link","url":"https://example.org","text":" "}`)),
		wrap(panel(`{"kind":"link","url":"https://example.org","text":"safe","onclick":"bad()"}`)),
		wrap(panel(`{"kind":"image","src":"/media/0123456789abcdef"}`)),
		wrap(panel(`{"kind":"image","src":"/media/0123456789abcdef","alt":null}`)),
		wrap(panel(`{"kind":"image","src":"/media/0123456789abcdef","alt":"bad\ntext"}`)),
		wrap(panel(`{"kind":"button","text":"run"}`)),
		wrap(panel(`{"kind":"stack","children":[]}`)),
	}
	for _, target := range []string{"javascript:alert(1)", "data:text/html,x", "/posts/1", "http://example.org/", "https://127.0.0.1/", "https://10.0.0.1/", "https://example.local/", "https://example.org:443/", "https://user@example.org/", "https://example.org/%0a"} {
		link, _ := json.Marshal(map[string]string{"kind": "link", "text": "bad target", "url": target})
		invalid = append(invalid, wrap(panel(string(link))))
	}
	for _, src := range []string{"https://example.org/x.png", "data:image/png;base64,YQ==", "/media/0123456789abcdef?x=1", "/media/0123456789abcdef#x", "/media/0123456789abcdef/", "/media/../x", "//example.org/x.png"} {
		image, _ := json.Marshal(map[string]string{"kind": "image", "src": src, "alt": ""})
		invalid = append(invalid, wrap(panel(string(image))))
	}
	for i, raw := range invalid {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if _, err := RenderMarkdown("```painter-card\n" + raw + "\n```"); err == nil {
				t.Fatal("invalid hand-written layout accepted", raw)
			}
			articleImportWantError(t, articleImportCardDocument(raw), 400)
		})
	}
	for _, c := range []contentCard{cardFixture("link"), panelFixture("metrics")} {
		raw, _ := json.Marshal(c)
		for _, root := range []string{"null", `{}`, panel(leaf)} {
			withRoot := strings.TrimSuffix(string(raw), "}") + `,"root":` + root + "}"
			if _, err := RenderMarkdown("```painter-card\n" + withRoot + "\n```"); err == nil {
				t.Fatal("legacy card accepted root", withRoot)
			}
		}
	}
}

func TestContentLayoutBounds(t *testing.T) {
	for role, limit := range map[string]int{"title": 200, "label": 120, "value": 100, "body": 2000, "detail": 500} {
		c := contentCard{V: 3, Type: "layout", Root: &ContentLayoutNode{Kind: "panel", Children: []*ContentLayoutNode{layoutText(role, strings.Repeat("界", limit))}}}
		if _, err := RenderMarkdown(contentCardMarkdown(c)); err != nil {
			t.Fatal("text boundary rejected", role, err)
		}
		c.Root.Children[0].Text += "界"
		if _, err := RenderMarkdown(contentCardMarkdown(c)); err == nil {
			t.Fatal("oversized text accepted", role)
		}
	}
	makeNodes := func(last int) contentCard {
		c := contentCard{V: 3, Type: "layout", Root: &ContentLayoutNode{Kind: "panel"}}
		for i := range 5 {
			stack := &ContentLayoutNode{Kind: "stack"}
			count := 12
			if i == 4 {
				count = last
			}
			for range count {
				stack.Children = append(stack.Children, layoutText("body", "x"))
			}
			c.Root.Children = append(c.Root.Children, stack)
		}
		return c
	}
	if _, err := RenderMarkdown(contentCardMarkdown(makeNodes(10))); err != nil {
		t.Fatal("64-node layout rejected", err)
	}
	if _, err := RenderMarkdown(contentCardMarkdown(makeNodes(11))); err == nil {
		t.Fatal("65-node layout accepted")
	}
	depth := layoutText("body", "deep")
	for range 4 {
		depth = &ContentLayoutNode{Kind: "stack", Children: []*ContentLayoutNode{depth}}
	}
	c := contentCard{V: 3, Type: "layout", Root: &ContentLayoutNode{Kind: "panel", Children: []*ContentLayoutNode{depth}}}
	if _, err := RenderMarkdown(contentCardMarkdown(c)); err != nil {
		t.Fatal("six levels rejected", err)
	}
	c.Root.Children = []*ContentLayoutNode{{Kind: "stack", Children: []*ContentLayoutNode{depth}}}
	if _, err := RenderMarkdown(contentCardMarkdown(c)); err == nil {
		t.Fatal("seven levels accepted")
	}
	for _, kind := range []string{"panel", "stack", "grid", "images"} {
		t.Run(kind, func(t *testing.T) {
			limit := 12
			if kind == "grid" || kind == "images" {
				limit = 6
			}
			for count := limit; count <= limit+1; count++ {
				root := &ContentLayoutNode{Kind: "panel"}
				parent := root
				if kind == "grid" || kind == "stack" {
					parent = &ContentLayoutNode{Kind: kind}
					root.Children = []*ContentLayoutNode{parent}
				}
				for range count {
					child := layoutText("body", "x")
					if kind == "grid" {
						child = &ContentLayoutNode{Kind: "stack", Children: []*ContentLayoutNode{child}}
					} else if kind == "images" {
						child = &ContentLayoutNode{Kind: "image", Src: "/media/0123456789abcdef"}
					}
					parent.Children = append(parent.Children, child)
				}
				_, err := RenderMarkdown(contentCardMarkdown(contentCard{V: 3, Type: "layout", Root: root}))
				if (err != nil) != (count > limit) {
					t.Fatal("child/image boundary mismatch", count, err)
				}
			}
		})
	}
	c = layoutFixture()
	c.Root.Children = []*ContentLayoutNode{layoutText("body", "x")}
	one := contentCardMarkdown(c)
	if _, err := RenderMarkdown(strings.Repeat(one+"\n\n", 128)); err != nil {
		t.Fatal("128 layouts rejected", err)
	}
	if _, err := RenderMarkdown(strings.Repeat(one+"\n\n", 129)); err == nil {
		t.Fatal("129 layouts accepted")
	}
	c.Root.Children[0].Text = strings.Repeat("界", 2000)
	c.Root.Children = append(c.Root.Children, layoutText("body", strings.Repeat("界", 1000)))
	if _, err := RenderMarkdown(contentCardMarkdown(c)); err == nil {
		t.Fatal("layout exceeded the existing 8 KiB snapshot limit")
	}
}

func TestContentLayoutMediaEscapingAndImportParity(t *testing.T) {
	const media = "/media/0123456789abcdef0123456789abcdef"
	c := layoutFixture()
	attack := `<script>alert("x")</script><img src=x onerror=alert(1)>`
	c.Root.Children = []*ContentLayoutNode{layoutText("body", attack), {Kind: "link", Text: attack, URL: "https://example.org/?q=%22%3E%3Cscript%3E"}, {Kind: "image", Src: media, Alt: `photo " onerror="bad`}, {Kind: "image", Src: media, Alt: ""}}
	raw, _ := json.Marshal(c)
	if !strings.Contains(string(raw), `"alt":""`) {
		t.Fatal("empty alt was lost in canonical source", string(raw))
	}
	rendered, err := RenderMarkdown(contentCardMarkdown(c))
	if err != nil {
		t.Fatal(err)
	}
	if len(rendered.MediaIDs) != 1 || rendered.MediaIDs[0] != strings.TrimPrefix(media, "/media/") {
		t.Fatal("nested media references not collected or deduplicated", rendered.MediaIDs)
	}
	if strings.Contains(rendered.HTML, "<script>") || strings.Contains(rendered.HTML, `<img src=x`) || strings.Contains(rendered.HTML, ` onerror="bad`) || !strings.Contains(rendered.HTML, `class="content-panel__image"`) || !strings.Contains(rendered.HTML, `loading="lazy"`) {
		t.Fatal("layout output did not safely escape author data", rendered.HTML)
	}
	for _, block := range []any{map[string]any{"type": "card", "card": c}, map[string]any{"type": "markdown", "markdown": contentCardMarkdown(c)}} {
		compiled, err := compileArticleImport(articleImportDocument(t, block))
		if err != nil || compiled.CardCount != 1 || compiled.HTML != rendered.HTML || compiled.Text != rendered.Text || compiled.Hash != rendered.Hash || len(compiled.MediaIDs) != 1 || compiled.MediaIDs[0] != rendered.MediaIDs[0] {
			t.Fatal("import/Markdown layout pipeline diverged", compiled, err)
		}
		if _, known, err := parseContentCard([]byte(strings.TrimSuffix(strings.TrimPrefix(compiled.Markdown, "```painter-card\n"), "\n```"))); err != nil || !known {
			t.Fatal("canonical import lost the required image alt", err)
		}
	}
	inline := `[layout](https://example.org "` + contentCardInlinePrefix + base64.RawURLEncoding.EncodeToString(raw) + `")`
	if _, err := RenderMarkdown(inline); err == nil {
		t.Fatal("layout accepted as a legacy inline reference")
	}
}

func TestContentLayoutUnknownVersionsStayReadableButCannotImport(t *testing.T) {
	for _, raw := range []string{`{"v":4,"type":"layout","root":{"kind":"panel","children":[]}}`, `{"v":3,"type":"future-layout","html":"<script>bad()</script>"}`, `{"v":2,"type":"layout","root":{"kind":"panel"}}`} {
		rendered, err := RenderMarkdown("```painter-card\n" + raw + "\n```")
		if err != nil || !strings.Contains(rendered.HTML, "<pre") || strings.Contains(rendered.HTML, `class="content-panel`) || strings.Contains(rendered.HTML, "<script>") {
			t.Fatal("unknown snapshot did not retain safe fenced source", rendered, err)
		}
		articleImportWantError(t, articleImportCardDocument(raw), 400)
	}
}
