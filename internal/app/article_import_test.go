package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	xhtml "golang.org/x/net/html"
	importdocs "painter-next/docs"
)

func articleImportDocument(t *testing.T, blocks ...any) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"format": "painter.article", "version": 1, "title": "导入测试", "summary": "纯文本摘要", "blocks": blocks})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func articleImportCardDocument(raw string) []byte {
	return []byte(`{"format":"painter.article","version":1,"title":"导入测试","blocks":[{"type":"card","card":` + raw + `}]}`)
}

func articleImportWantError(t *testing.T, raw []byte, status int) {
	t.Helper()
	_, err := compileArticleImport(raw)
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != status {
		t.Fatalf("expected import HTTP %d, got %v", status, err)
	}
}

func TestArticleImportEmbeddedFixtureAndRendererParity(t *testing.T) {
	compiled, err := compileArticleImport([]byte(importdocs.ArticleImportExample))
	if err != nil {
		t.Fatal(err)
	}
	if compiled.BlockCount != 7 || compiled.CardCount != 4 || len(compiled.MediaIDs) != 0 || !strings.Contains(compiled.Title, "虚构") || !strings.Contains(compiled.Summary, "虚构") {
		t.Fatal("example contract lost", compiled)
	}
	for _, want := range []string{"<table>", "content-panel--callout", "content-panel--metrics", "content-panel--summary", "content-panel--layout", "下一步如何整理（组合示例）", "example.com/research-notes", "2026-10-01", "2026-10-14", "12 条", "待核验"} {
		if !strings.Contains(compiled.HTML, want) {
			t.Errorf("embedded example missing %q: %s", want, compiled.HTML)
		}
	}
	rendered, err := RenderMarkdown(compiled.Markdown)
	if err != nil || rendered.HTML != compiled.HTML || rendered.Text != compiled.Text || rendered.Hash != compiled.Hash || compiled.Version != RendererVersion {
		t.Fatal("import and normal rendering diverged", rendered, compiled, err)
	}
	second, err := compileArticleImport([]byte(importdocs.ArticleImportExample))
	if err != nil || second.Markdown != compiled.Markdown || second.HTML != compiled.HTML || second.Hash != compiled.Hash {
		t.Fatal("import is not deterministic", second, err)
	}
	var schema map[string]any
	if json.Unmarshal([]byte(importdocs.ArticleImportSchema), &schema) != nil || schema["type"] != "object" || importdocs.ArticleImportPrompt == "" {
		t.Fatal("embedded authoring aids are incomplete")
	}
}

func TestArticleImportHistoricalPartialFixture(t *testing.T) {
	raw, err := os.ReadFile("../../docs/examples/web3-radar-2026-11-13.partial.json")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compileArticleImport(raw)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.CardCount != 4 || compiled.BlockCount != 7 || compiled.Title != "Web3 每周交易机会雷达 | 2026-11-13（历史片段）" || !strings.Contains(compiled.Summary, "未更新行情或重新核验") {
		t.Fatal("historical fixture identity or scope changed", compiled)
	}
	for _, class := range []string{"content-panel--warning", "content-panel--insight", "content-panel--metrics", "content-panel--summary"} {
		if strings.Count(compiled.HTML, class) != 1 {
			t.Errorf("expected exactly one %s panel", class)
		}
	}
	for _, original := range []string{
		"本次未更新行情，也未重新核验原报告的事实与预测", "缺失部分未补写", "非截稿实时价", "商业收入未明",
		"北京时间 2026-11-13 18:03", "过去7天：11-06 18:03—11-13 18:03", "本月已观察：11-01 00:00—11-13 18:03（未满13天）", "未来30天：11-13—12-13",
		"11/06–12", "−$1.055B", "5个已结束交易日", "$285.58B", "7日−1.66%", "$74.76B", "7日约−7.4%", "$69k", "$68,408",
		"DEX7日$43.271B、周变−21.17%", "链上永续7日$150.911B、−9.14%", "DOJ查控失窃资产不等于交易所全部风险解除",
	} {
		if !strings.Contains(compiled.Text, original) {
			t.Errorf("historical value, window or limitation changed: %s", original)
		}
	}
	root, err := xhtml.Parse(strings.NewReader(compiled.HTML))
	if err != nil {
		t.Fatal(err)
	}
	var nodeText func(*xhtml.Node) string
	nodeText = func(n *xhtml.Node) string {
		if n.Type == xhtml.TextNode {
			return n.Data
		}
		var value strings.Builder
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			value.WriteString(nodeText(child))
		}
		return value.String()
	}
	tables, sources := 0, map[string]bool{}
	var dimensions []string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			switch n.Data {
			case "table":
				tables++
			case "a":
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						sources[attr.Val] = true
					}
				}
			case "tr":
				if n.Parent != nil && n.Parent.Data == "tbody" {
					for child := n.FirstChild; child != nil; child = child.NextSibling {
						if child.Type == xhtml.ElementNode && child.Data == "td" {
							dimensions = append(dimensions, strings.TrimSpace(nodeText(child)))
							break
						}
					}
				}
			case "h1", "h2", "h3", "h4", "h5", "h6":
				if strings.Contains(nodeText(n), "双榜机会矩阵") {
					t.Error("truncated matrix became a fabricated section")
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	if tables != 1 || strings.Join(dimensions, ",") != "资产价格,资金供给,DeFi活跃,结构性采用,对手方风险" || strings.Count(compiled.Text, "双榜机会矩阵") != 1 {
		t.Fatal("original five-row table or truncated-matrix boundary changed", tables, dimensions)
	}
	originalSources := []string{
		"https://farside.co.uk/btc/", "https://defillama.com/",
		"https://www.theblock.co/news/markets/2026-11-13-bitcoin-tumbles-below-69000-439643",
		"https://www.justice.gov/opa/pr/justice-department-announces-actions-disrupt-north-koreas-massive-theft-cryptocurrency",
		"https://www.coinbase.com/en-gb/blog/coinbase-and-pnc-expand-to-stablecoin-payment-acceptance",
		"https://solana.com/news/solana-privacy-rings-bring-programmable-privacy-onchain",
	}
	if len(sources) != len(originalSources) {
		t.Fatal("historical sources were omitted or invented", len(sources))
	}
	for _, source := range originalSources {
		if !sources[source] {
			t.Errorf("original source URL changed: %s", source)
		}
	}
}

func TestArticleImportRejectsInvalidEnvelopeBlocksAndCanonicalJSON(t *testing.T) {
	invalid := []string{
		`null`, `[]`, `true`, `"article"`, `{broken`, `{} {}`, "```json\n{}\n```",
		`{"format":"painter.article","version":2,"title":"x","blocks":[{"type":"markdown","markdown":"text"}]}`,
		`{"format":"other","version":1,"title":"x","blocks":[{"type":"markdown","markdown":"text"}]}`,
		`{"format":"painter.article","version":1.0,"title":"x","blocks":[{"type":"markdown","markdown":"text"}]}`,
		`{"format":"painter.article","version":1,"title":" ","blocks":[{"type":"markdown","markdown":"text"}]}`,
		`{"format":"painter.article","version":1,"title":"x\ny","blocks":[{"type":"markdown","markdown":"text"}]}`,
		`{"format":"painter.article","version":1,"title":"x","summary":null,"blocks":[{"type":"markdown","markdown":"text"}]}`,
		`{"format":"painter.article","version":1,"title":"x","blocks":[]}`,
		`{"format":"painter.article","version":1,"title":"x","blocks":[null]}`,
		`{"format":"painter.article","version":1,"title":"x","blocks":["markdown"]}`,
		`{"format":"painter.article","version":1,"title":"x","blocks":[{"type":"html","html":"text"}]}`,
		`{"format":"painter.article","version":1,"title":"x","blocks":[{"type":"markdown","markdown":" "}]}`,
		`{"format":"painter.article","version":1,"title":"x","blocks":[{"type":"markdown","markdown":12}]}`,
		`{"format":"painter.article","version":1,"title":"x","blocks":[{"type":"markdown","markdown":"text","style":"color:red"}]}`,
		`{"format":"painter.article","version":1,"title":"x","blocks":[{"type":"card"}]}`,
		`{"format":"painter.article","version":1,"title":"x","blocks":[{"type":"card","card":{"v":2,"type":"callout","tone":"note","body":"text"},"markdown":"text"}]}`,
		`{"format":"painter.article","version":1,"title":"x","published":true,"blocks":[{"type":"markdown","markdown":"text"}]}`,
		`{"format":"painter.article","version":1,"title":"x","title":"y","blocks":[{"type":"markdown","markdown":"text"}]}`,
		`{"format":"painter.article","version":1,"title":"x","\u0074itle":"y","blocks":[{"type":"markdown","markdown":"text"}]}`,
		`{"format":"painter.article","version":1,"title":"x","Title":"y","blocks":[{"type":"markdown","markdown":"text"}]}`,
		`{"Format":"painter.article","version":1,"title":"x","blocks":[{"type":"markdown","markdown":"text"}]}`,
		`{"format":"painter.article","version":1,"title":"x","blocks":[{"type":"markdown","Markdown":"text"}]}`,
	}
	for _, raw := range invalid {
		t.Run(raw, func(t *testing.T) { articleImportWantError(t, []byte(raw), 400) })
	}
	for _, card := range []string{
		`{"v":2,"type":"metrics","metrics":[{"label":"a","value":"1","source":{"label":"s","url":"https://example.org/","url":"https://other.example/"}}]}`,
		`{"v":1,"type":"github","url":"https://github.com/openai/codex","title":"x","stats":{"stars":1,"stars":2}}`,
		`{"v":1,"type":"link","url":"https://example.org/","title":"x","Title":"y"}`,
		`{"v":1,"type":"github","url":"https://github.com/openai/codex","title":"x","stats":{"Stars":1}}`,
		`{"v":1,"type":"link","url":"https://example.org/","title":"x","description":null}`,
		`{"v":2,"type":"summary","entries":[{"title":"x","body":"text","html":"<b>text</b>"}]}`,
	} {
		articleImportWantError(t, articleImportCardDocument(card), 400)
	}
	valid := articleImportDocument(t, map[string]any{"type": "markdown", "markdown": "text"})
	articleImportWantError(t, append(append([]byte{}, valid[:len(valid)-1]...), 0xff, '}'), 400)
	articleImportWantError(t, bytes.Replace(valid, []byte(`"导入测试"`), []byte(`"`+strings.Repeat("界", 201)+`"`), 1), 400)
	articleImportWantError(t, bytes.Replace(valid, []byte(`"纯文本摘要"`), []byte(`"`+strings.Repeat("x", 501)+`"`), 1), 400)
	if err := validateArticleImportJSON([]byte(strings.Repeat("[", 24) + "0" + strings.Repeat("]", 24))); err != nil {
		t.Fatal("depth-24 JSON rejected", err)
	}
	if err := validateArticleImportJSON([]byte(strings.Repeat("[", 25) + "0" + strings.Repeat("]", 25))); err == nil {
		t.Fatal("excessive JSON nesting accepted")
	}
}

func TestArticleImportMarkdownSafetyAndAllCardForms(t *testing.T) {
	for _, markdown := range []string{`<script>alert(1)</script>`, `<div style="position:fixed">layout</div>`, `text <img src=x onerror=alert(1)>`, `![remote](https://example.org/image.png)`, `![data](data:image/png;base64,YQ==)`, "```painter-card\n{\"v\":9,\"type\":\"callout\"}\n```", "```painter-card\n{\"v\":2,\"type\":\"unknown\"}\n```"} {
		articleImportWantError(t, articleImportDocument(t, map[string]any{"type": "markdown", "markdown": markdown}), 400)
	}
	for _, raw := range []string{`{"v":9,"type":"callout"}`, `{"v":2,"type":"unknown"}`, `{"v":1,"type":"link","title":"x","url":"javascript:alert(1)"}`, `{"v":2,"type":"metrics","metrics":[{"label":"x","value":"1","source":{"label":"s","url":"https://127.0.0.1/"}}]}`} {
		articleImportWantError(t, articleImportCardDocument(raw), 400)
	}
	for _, raw := range []string{`{"v":1,"type":"link","url":"https://example.org/","title":"x","title":"y"}`, `{"v":1,"type":"link","url":"https://example.org/","title":"x","Title":"y"}`} {
		articleImportWantError(t, articleImportDocument(t, map[string]any{"type": "markdown", "markdown": "```painter-card\n" + raw + "\n```"}), 400)
	}
	remote := cardFixture("github")
	if _, err := RenderMarkdown(contentCardMarkdown(remote)); err != nil {
		t.Fatal("normal v1 provider image rendering regressed", err)
	}
	for _, block := range []any{map[string]any{"type": "card", "card": remote}, map[string]any{"type": "markdown", "markdown": contentCardMarkdown(remote)}} {
		articleImportWantError(t, articleImportDocument(t, block), 400)
	}
	link := cardFixture("link")
	for _, markdown := range []string{contentCardInlineMarkdown(link), contentCardMarkdown(link)} {
		compiled, err := compileArticleImport(articleImportDocument(t, map[string]any{"type": "markdown", "markdown": markdown}))
		if err != nil || compiled.CardCount != 1 || !strings.Contains(compiled.HTML, link.Title) {
			t.Fatal("recognized Markdown card not counted/rendered", compiled, err)
		}
	}
	for _, markdown := range []string{
		strings.Replace(contentCardInlineMarkdown(link), contentCardInlinePrefix, "painter:v2:", 1),
		strings.Replace(contentCardInlineMarkdown(link), "<"+link.URL+">", "<https://different.example/>", 1),
		`[reference](https://example.org/ "painter:v1:!bad")`,
	} {
		articleImportWantError(t, articleImportDocument(t, map[string]any{"type": "markdown", "markdown": markdown}), 400)
	}
	for _, raw := range []string{`{"v":2,"type":"link","url":"https://example.org/"}`, `{"v":1,"type":"future","url":"https://example.org/"}`, `{"v":1,"type":"link","url":"https://example.org/","title":"x","title":"y"}`} {
		inline := `[reference](https://example.org/ "` + contentCardInlinePrefix + base64.RawURLEncoding.EncodeToString([]byte(raw)) + `")`
		articleImportWantError(t, articleImportDocument(t, map[string]any{"type": "markdown", "markdown": inline}), 400)
	}
	mediaID := "0123456789abcdef0123456789abcdef"
	compiled, err := compileArticleImport(articleImportDocument(t, map[string]any{"type": "markdown", "markdown": "```html\n<script>literal example</script>\n```\n\n![uploaded](/media/" + mediaID + ")"}, map[string]any{"type": "card", "card": contentCard{V: 2, Type: "callout", Tone: "note", Body: "<script>literal panel text</script>"}}))
	if err != nil || len(compiled.MediaIDs) != 1 || compiled.MediaIDs[0] != mediaID || strings.Contains(compiled.HTML, "<script>") || !strings.Contains(compiled.Text, "literal example") || !strings.Contains(compiled.Text, "literal panel text") {
		t.Fatal("literal code/panel text or uploaded media contract", compiled, err)
	}
}

func TestArticleImportBoundsAndCrossBlockFenceIntegrity(t *testing.T) {
	valid := articleImportDocument(t, map[string]any{"type": "markdown", "markdown": "text"})
	boundary := append(append([]byte{}, valid...), bytes.Repeat([]byte(" "), articleImportLimit-len(valid))...)
	if _, err := compileArticleImport(boundary); err != nil {
		t.Fatal("exact 1 MiB file rejected", err)
	}
	articleImportWantError(t, append(boundary, ' '), 413)
	var blocks []any
	for i := 0; i < 256; i++ {
		blocks = append(blocks, map[string]any{"type": "markdown", "markdown": "text"})
	}
	if compiled, err := compileArticleImport(articleImportDocument(t, blocks...)); err != nil || compiled.BlockCount != 256 {
		t.Fatal("256-block boundary rejected", compiled, err)
	}
	articleImportWantError(t, articleImportDocument(t, append(blocks, blocks[0])...), 400)
	card := panelFixture("callout")
	blocks = nil
	for i := 0; i < 64; i++ {
		blocks = append(blocks, map[string]any{"type": "card", "card": card})
	}
	blocks = append(blocks, map[string]any{"type": "markdown", "markdown": strings.Repeat(contentCardMarkdown(card)+"\n\n", 64)})
	if compiled, err := compileArticleImport(articleImportDocument(t, blocks...)); err != nil || compiled.CardCount != 128 {
		t.Fatal("mixed 128-card boundary rejected", compiled.CardCount, err)
	}
	articleImportWantError(t, articleImportDocument(t, append(blocks, map[string]any{"type": "card", "card": card})...), 400)
	inline := contentCardInlineMarkdown(cardFixture("link"))
	articleImportWantError(t, articleImportDocument(t, map[string]any{"type": "markdown", "markdown": strings.Repeat(inline+"\n\n", 129)}), 400)
	articleImportWantError(t, articleImportDocument(t, map[string]any{"type": "markdown", "markdown": "```go\nunclosed example"}, map[string]any{"type": "card", "card": card}), 400)
	articleImportWantError(t, articleImportDocument(t, map[string]any{"type": "markdown", "markdown": "```go\nunclosed example"}, map[string]any{"type": "markdown", "markdown": contentCardMarkdown(card)}), 400)
}

func articleImportRequest(a *App, cookie *http.Cookie, csrf string, raw []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/editor/import-preview", bytes.NewReader(raw))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", a.Config.PublicBaseURL)
	r.Header.Set("X-CSRF-Token", csrf)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}

func TestArticleImportEndpointAuthorizationBeforeParsingAndFormat(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	_, author, authorCSRF := testIdentity(t, a, "author")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	for _, v := range []struct {
		cookie *http.Cookie
		csrf   string
		status int
	}{{nil, "", 401}, {reader, readerCSRF, 403}, {author, "", 403}, {author, authorCSRF, 400}} {
		if response := articleImportRequest(a, v.cookie, v.csrf, []byte("{broken")); response.Code != v.status {
			t.Fatalf("auth must precede JSON parsing: want=%d got=%d body=%s", v.status, response.Code, response.Body.String())
		}
	}
	for _, v := range []struct {
		cookie *http.Cookie
		status int
	}{{nil, 401}, {reader, 403}, {author, 200}} {
		response := contentCall(t, a, v.cookie, "", http.MethodGet, "/api/v1/editor/import-format", nil, 0, "")
		if response.Code != v.status {
			t.Fatal("format authorization", v.status, response.Code, response.Body.String())
		}
		if v.status == 200 {
			format := jsonMap(t, response)
			if len(format) != 3 || format["schema"] != importdocs.ArticleImportSchema || format["example"] != importdocs.ArticleImportExample || format["prompt"] != importdocs.ArticleImportPrompt {
				t.Fatal("format endpoint did not return exact embedded texts", format)
			}
		}
	}
}

func TestArticleImportPreviewDoesNotMutateOrPublishAndKeepsSnapshots(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	_, author, csrf := testIdentity(t, a, "author")
	created := contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "原文章", "markdown": "原正文"}, 0, ""), 201)
	id := created["id"].(string)
	contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	before := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	studioBefore := contentWant(t, contentCall(t, a, author, csrf, "GET", "/api/v1/studio/posts/"+id, nil, 0, ""), 200)
	counts := func() [7]int64 {
		t.Helper()
		var n [7]int64
		err := a.Pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM posts),(SELECT count(*) FROM post_drafts),(SELECT count(*) FROM post_revisions),(SELECT count(*) FROM post_publications),(SELECT count(*) FROM audit_logs),(SELECT count(*) FROM outbox),(SELECT count(*) FROM search_documents)`).Scan(&n[0], &n[1], &n[2], &n[3], &n[4], &n[5], &n[6])
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	initial := counts()
	var requests atomic.Int32
	homeFake(a, func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		t.Error("import preview contacted an external provider")
		return homeHTTP(500, ""), nil
	})
	c := cardFixture("github")
	c.ImageURL, c.Title, c.Description, c.MetadataSource = "", "作者保留的历史快照", "历史口径；未经再次核验", "author"
	raw := articleImportDocument(t, map[string]any{"type": "card", "card": c}, map[string]any{"type": "card", "card": panelFixture("metrics")})
	first := contentWant(t, articleImportRequest(a, author, csrf, raw), 200)
	second := contentWant(t, articleImportRequest(a, author, csrf, raw), 200)
	if first["markdown"] != second["markdown"] || first["sanitized_html"] != second["sanitized_html"] || first["content_hash"] != second["content_hash"] || !strings.Contains(first["markdown"].(string), c.Title) || !strings.Contains(first["plain_text"].(string), c.Description) || requests.Load() != 0 {
		t.Fatal("import re-resolved or changed author snapshots", first, second, requests.Load())
	}
	if after := counts(); after != initial {
		t.Fatal("preview created/updated content state", initial, after)
	}
	after := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	studioAfter := contentWant(t, contentCall(t, a, author, csrf, "GET", "/api/v1/studio/posts/"+id, nil, 0, ""), 200)
	if before["revision_id"] != after["revision_id"] || before["sanitized_html"] != after["sanitized_html"] || studioBefore["markdown"] != studioAfter["markdown"] || studioBefore["version"] != studioAfter["version"] || studioBefore["draft_version"] != studioAfter["draft_version"] {
		t.Fatal("preview mutated existing draft/publication", before, after, studioBefore, studioAfter)
	}
}

func TestArticleImportPreviewChecksUploadedMediaAuthorization(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterMedia()
	_, author, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	own := contentWant(t, mediaTestUpload(t, a, author, csrf, RandID(), 81), 201)["media_id"].(string)
	foreign := contentWant(t, mediaTestUpload(t, a, other, otherCSRF, RandID(), 82), 201)["media_id"].(string)
	for _, v := range []struct {
		id     string
		status int
	}{{own, 200}, {foreign, 409}, {"0123456789abcdef0123456789abcdef", 409}} {
		raw := articleImportDocument(t, map[string]any{"type": "markdown", "markdown": "![已上传图片](/media/" + v.id + ")"})
		response := articleImportRequest(a, author, csrf, raw)
		if response.Code != v.status {
			t.Fatal("import media reference authorization", v.id, v.status, response.Code, response.Body.String())
		}
	}
}
