package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

func panelFixture(kind string) contentCard {
	c := contentCard{V: 2, Type: kind, Title: "结构化内容"}
	switch kind {
	case "callout":
		c.Tone, c.Body = "insight", "洞察正文\n第二行保留"
	case "metrics":
		c.Metrics = []ContentCardMetric{{Label: "响应延迟", Value: "120 ms", Detail: "较上期改善", Source: &ContentCardSource{Label: "公开报告", URL: "https://example.org/report?a=1&b=2"}}, {Label: "样本数", Value: "42"}}
	case "summary":
		c.Entries = []ContentCardEntry{{Title: "主要发现", Body: "摘要正文\n补充说明"}, {Title: "下一步", Body: "继续核实公开资料"}}
	}
	return c
}

func TestContentPanelsRenderSemanticHTMLAndSearchText(t *testing.T) {
	var markdown []string
	for _, kind := range []string{"callout", "metrics", "summary"} {
		c := panelFixture(kind)
		raw, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), `"url":""`) {
			t.Fatal("panel gained a v1 root URL", string(raw))
		}
		parsed, known, err := parseContentCard(raw)
		if err != nil || !known || parsed.V != 2 || parsed.Type != kind {
			t.Fatal("v2 card did not round-trip", parsed, known, err)
		}
		markdown = append(markdown, contentCardMarkdown(c))
	}
	source := "# 结构化文章\n\n" + strings.Join(markdown, "\n\n")
	rendered, err := RenderMarkdown(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`class="content-panel content-panel--callout content-panel--insight"`, `class="content-panel content-panel--metrics"`, `class="content-panel content-panel--summary"`, `class="content-panel__title"`, `class="content-panel__body"`, `class="content-panel__grid"`, `class="content-panel__item"`, `class="content-panel__label"`, `class="content-panel__value"`, `class="content-panel__detail"`, `class="content-panel__source"`, "<br", `href="https://example.org/report?a=1&amp;b=2"`, `target="_blank"`} {
		if !strings.Contains(rendered.HTML, want) {
			t.Errorf("panel markup missing %q: %s", want, rendered.HTML)
		}
	}
	for _, want := range []string{"洞察正文", "第二行保留", "响应延迟", "120 ms", "较上期改善", "公开报告", "样本数", "42", "主要发现", "摘要正文", "继续核实公开资料"} {
		if !strings.Contains(rendered.Text, want) {
			t.Errorf("panel text missing %q: %s", want, rendered.Text)
		}
	}
	if len(rendered.MediaIDs) != 0 || len(rendered.TOC) != 1 || !strings.Contains(rendered.Version, "painter-panels-v2") {
		t.Fatal("unexpected panel render metadata", rendered)
	}
	copy, err := RenderMarkdown(source)
	if err != nil || copy.HTML != rendered.HTML || copy.Hash != rendered.Hash {
		t.Fatal("panels render nondeterministically", copy, err)
	}
	for _, tone := range []string{"note", "insight", "warning"} {
		c := panelFixture("callout")
		c.Title, c.Tone = "", tone
		r, err := RenderMarkdown(contentCardMarkdown(c))
		if err != nil || !strings.Contains(r.HTML, "content-panel--"+tone) || strings.Contains(r.HTML, "content-panel__title") {
			t.Fatal("callout tone/optional title", tone, r, err)
		}
	}
}

func TestContentPanelsRejectUnknownCrossTypeAndNullFields(t *testing.T) {
	invalid := []string{
		`{"v":2,"type":"callout","tone":"note","body":"text","url":""}`,
		`{"v":2,"type":"callout","tone":"note","body":"text","image_url":null}`,
		`{"v":2,"type":"callout","tone":"note","body":"text","html":"<b>text</b>"}`,
		`{"v":2,"type":"callout","tone":"note","body":"text","metrics":[]}`,
		`{"v":2,"type":"callout","tone":"note","body":"text","entries":[]}`,
		`{"v":2,"type":"metrics","metrics":[{"label":"a","value":"1"}],"body":""}`,
		`{"v":2,"type":"summary","entries":[{"title":"a","body":"b"}],"tone":"note"}`,
		`{"v":2,"type":"callout","tone":"note","body":"text","title":null}`,
		`{"v":2,"type":"callout","tone":"note","body":null}`,
		`{"v":2,"type":"callout","tone":"note","body":"text","Title":"wrong casing"}`,
		`{"v":2,"type":"metrics","metrics":[{"label":"a","value":"1","extra":true}]}`,
		`{"v":2,"type":"metrics","metrics":[{"label":"a","value":"1","detail":null}]}`,
		`{"v":2,"type":"metrics","metrics":[{"label":"a","value":"1","source":null}]}`,
		`{"v":2,"type":"metrics","metrics":[{"label":"a","value":"1","source":{"label":"a","url":"https://example.org","html":"x"}}]}`,
		`{"v":2,"type":"metrics","metrics":[{"Label":"a","value":"1"}]}`,
		`{"v":2,"type":"metrics","metrics":[null]}`,
		`{"v":2,"type":"metrics","metrics":[{"label":"a","value":"1","source":{"label":"a"}}]}`,
		`{"v":2,"type":"metrics","metrics":[{"label":"a","value":1}]}`,
		`{"v":2,"type":"summary","entries":[{"title":"a","body":"b","url":"https://example.org"}]}`,
		`{"v":2,"type":"summary","entries":[{"title":"a","body":"b","Body":"other"}]}`,
		`{"v":2,"type":"summary","entries":[{"title":"a","body":null}]}`,
		`{"v":2,"type":"summary","entries":[null]}`,
	}
	for _, kind := range []string{"github", "steam", "apple_music", "link"} {
		raw, _ := json.Marshal(cardFixture(kind))
		for _, extra := range []string{`"tone":""`, `"body":null`, `"metrics":[]`, `"entries":null`} {
			invalid = append(invalid, strings.TrimSuffix(string(raw), "}")+","+extra+"}")
		}
	}
	for _, raw := range invalid {
		if _, err := RenderMarkdown("```painter-card\n" + raw + "\n```"); err == nil {
			t.Errorf("invalid panel accepted: %s", raw)
		}
	}
	for _, raw := range []string{`{"v":1,"type":"callout","html":"<script>bad()</script>"}`, `{"v":2,"type":"github","url":"javascript:alert(1)"}`, `{"v":3,"type":"callout","html":"<script>bad()</script>"}`, `{"v":2,"type":"future-panel","html":"<script>bad()</script>"}`} {
		r, err := RenderMarkdown("```painter-card\n" + raw + "\n```")
		if err != nil || !strings.Contains(r.HTML, "<pre") || strings.Contains(r.HTML, `class="content-panel`) || strings.Contains(r.HTML, "<script>") {
			t.Fatal("unknown version/type did not retain safe source", raw, r, err)
		}
	}
}

func TestContentPanelsTextBoundsCollectionsAndInline(t *testing.T) {
	for _, kind := range []string{"callout", "metrics", "summary"} {
		c := panelFixture(kind)
		c.Title = strings.Repeat("界", 200)
		if kind == "callout" {
			c.Body = strings.Repeat("界", 2000)
		} else if kind == "metrics" {
			c.Metrics = []ContentCardMetric{{Label: strings.Repeat("界", 120), Value: strings.Repeat("界", 100), Detail: strings.Repeat("界", 500), Source: &ContentCardSource{Label: strings.Repeat("界", 120), URL: "https://example.org/"}}}
		} else {
			c.Entries = []ContentCardEntry{{Title: strings.Repeat("界", 200), Body: strings.Repeat("界", 2000)}}
			// The per-field limit remains subject to the shared 8 KiB JSON cap.
			c.Title = ""
		}
		if _, err := RenderMarkdown(contentCardMarkdown(c)); err != nil {
			t.Fatal("valid field boundary rejected", kind, err)
		}
		c.Title = strings.Repeat("界", 201)
		if _, err := RenderMarkdown(contentCardMarkdown(c)); err == nil {
			t.Fatal("long panel title accepted", kind)
		}
		c = panelFixture(kind)
		raw, _ := json.Marshal(c)
		inline := `[panel](https://example.org "` + contentCardInlinePrefix + base64.RawURLEncoding.EncodeToString(raw) + `")`
		if _, err := RenderMarkdown(inline); err == nil {
			t.Fatal("v2 block accepted as inline card", kind)
		}
	}
	mutations := []struct {
		name string
		kind string
		edit func(*contentCard)
	}{
		{"callout missing body", "callout", func(c *contentCard) { c.Body = " \r\n " }},
		{"callout body", "callout", func(c *contentCard) { c.Body = strings.Repeat("x", 2001) }},
		{"callout tone", "callout", func(c *contentCard) { c.Tone = "danger" }},
		{"callout tab", "callout", func(c *contentCard) { c.Body = "bad\tvalue" }},
		{"metrics empty", "metrics", func(c *contentCard) { c.Metrics = nil }},
		{"metrics count", "metrics", func(c *contentCard) { c.Metrics = make([]ContentCardMetric, 7) }},
		{"metrics label", "metrics", func(c *contentCard) { c.Metrics[0].Label = strings.Repeat("x", 121) }},
		{"metrics value", "metrics", func(c *contentCard) { c.Metrics[0].Value = strings.Repeat("x", 101) }},
		{"metrics detail", "metrics", func(c *contentCard) { c.Metrics[0].Detail = strings.Repeat("x", 501) }},
		{"metrics missing value", "metrics", func(c *contentCard) { c.Metrics[0].Value = " " }},
		{"metrics source label", "metrics", func(c *contentCard) { c.Metrics[0].Source.Label = strings.Repeat("x", 121) }},
		{"summary empty", "summary", func(c *contentCard) { c.Entries = nil }},
		{"summary count", "summary", func(c *contentCard) { c.Entries = make([]ContentCardEntry, 7) }},
		{"summary title", "summary", func(c *contentCard) { c.Entries[0].Title = strings.Repeat("x", 201) }},
		{"summary body", "summary", func(c *contentCard) { c.Entries[0].Body = strings.Repeat("x", 2001) }},
		{"summary missing body", "summary", func(c *contentCard) { c.Entries[0].Body = "\n" }},
		{"summary control", "summary", func(c *contentCard) { c.Entries[0].Body = "bad\x00value" }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			c := panelFixture(mutation.kind)
			mutation.edit(&c)
			if _, err := RenderMarkdown(contentCardMarkdown(c)); err == nil {
				t.Fatal("invalid panel accepted")
			}
		})
	}
	for _, kind := range []string{"metrics", "summary"} {
		c := panelFixture(kind)
		if kind == "metrics" {
			c.Metrics = nil
			for i := 0; i < 6; i++ {
				c.Metrics = append(c.Metrics, ContentCardMetric{Label: "指标", Value: fmt.Sprint(i)})
			}
		} else {
			c.Entries = nil
			for i := 0; i < 6; i++ {
				c.Entries = append(c.Entries, ContentCardEntry{Title: "项目", Body: fmt.Sprint(i)})
			}
		}
		if _, err := RenderMarkdown(contentCardMarkdown(c)); err != nil {
			t.Fatal("six-item panel rejected", kind, err)
		}
	}
	c := panelFixture("summary")
	c.Entries = []ContentCardEntry{{Title: "a", Body: strings.Repeat("界", 2000)}, {Title: "b", Body: strings.Repeat("界", 2000)}}
	if _, err := RenderMarkdown(contentCardMarkdown(c)); err == nil {
		t.Fatal("shared 8 KiB card limit bypassed")
	}
}

func TestContentPanelsEscapeHTMLAndRestrictSourceURLs(t *testing.T) {
	attack := `</p></div><script>alert(1)</script><img src=x onerror=alert(1)> **literal** [link](javascript:alert(1))`
	for _, kind := range []string{"callout", "metrics", "summary"} {
		c := panelFixture(kind)
		c.Title = attack
		switch kind {
		case "callout":
			c.Body = attack + "\r\nnext\rthird"
		case "metrics":
			c.Metrics = []ContentCardMetric{{Label: "<b>label</b>", Value: "<i>value</i>", Detail: attack, Source: &ContentCardSource{Label: "<img src=x onerror=alert(1)>", URL: "https://example.org/?q=%22%3E%3Cscript%3E"}}}
		case "summary":
			c.Entries = []ContentCardEntry{{Title: "<b>entry</b>", Body: attack}}
		}
		r, err := RenderMarkdown(contentCardMarkdown(c))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(r.HTML, "<script") || strings.Contains(r.HTML, "<img") || strings.Contains(r.HTML, "<b>") || strings.Contains(r.HTML, "<i>") || strings.Contains(r.HTML, "<strong>") || !strings.Contains(r.HTML, "&lt;script&gt;") || !strings.Contains(r.Text, "**literal**") {
			t.Fatal("panel text was interpreted as markup", kind, r)
		}
		root, err := xhtml.Parse(strings.NewReader(r.HTML))
		if err != nil {
			t.Fatal(err)
		}
		var inspect func(*xhtml.Node)
		inspect = func(n *xhtml.Node) {
			for _, attr := range n.Attr {
				if strings.HasPrefix(attr.Key, "on") || attr.Key == "style" {
					t.Errorf("unsafe panel attribute: %s", attr.Key)
				}
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				inspect(child)
			}
		}
		inspect(root)
	}
	for _, destination := range []string{"javascript:alert(1)", "data:text/html,bad", "http://example.org", "//example.org", "/media/0123456789abcdef", "https://localhost/", "https://127.0.0.1/", "https://[::1]/", "https://169.254.169.254/", "https://example.org:443/", "https://user:pass@example.org", "https://example.org/%0a", "https://example.org\\@evil.test/", "https://example.org/" + strings.Repeat("x", 2048)} {
		c := panelFixture("metrics")
		c.Metrics[0].Source.URL = destination
		if _, err := RenderMarkdown(contentCardMarkdown(c)); err == nil {
			t.Errorf("unsafe source URL accepted: %s", destination)
		}
	}
	if r, err := RenderMarkdown(`<div class="content-panel content-panel--callout content-panel--warning" style="position:fixed">raw HTML</div>`); err != nil || strings.Contains(r.HTML, `class="content-panel`) {
		t.Fatal("raw HTML became a panel", r, err)
	}
}

func TestContentPanelsPreviewPublicationSearchAndSSR(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterSearch()
	a.RegisterWeb()
	_, cookie, csrf := testIdentity(t, a, "author")
	var panels []string
	for _, kind := range []string{"callout", "metrics", "summary"} {
		panels = append(panels, contentCardMarkdown(panelFixture(kind)))
	}
	markdown := "# 面板文章\n\n" + strings.Join(panels, "\n\n")
	created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "结构化文章面板", "markdown": markdown}, 0, ""), 201)
	id := created["id"].(string)
	preview := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/editor/preview", map[string]any{"markdown": markdown, "seq": 2}, 0, ""), 200)
	contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	public := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	if public["sanitized_html"] != preview["sanitized_html"] || preview["renderer_version"] != RendererVersion {
		t.Fatal("panel preview/publish pipelines differ", preview, public)
	}
	search := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/search?q=响应延迟", nil, 0, ""), 200)
	if len(search["items"].([]any)) != 1 {
		t.Fatal("metric text absent from search", search)
	}
	response := contentCall(t, a, nil, "", http.MethodGet, "/posts/"+id, nil, 0, "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), public["sanitized_html"].(string)) {
		t.Fatal("SSR lost panel markup", response.Code, response.Body.String())
	}
}
