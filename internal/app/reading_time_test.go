package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestArticleReadingMinutesLanguagesAndRounding(t *testing.T) {
	for _, test := range []struct {
		name, html string
		minutes    int
	}{
		{"empty", "", 1},
		{"only images", `<p><img src="/media/0123456789abcdef" alt="` + strings.Repeat("description ", 500) + `"></p>`, 1},
		{"only punctuation", `<p>✨，。！？ {} [] : ;</p>`, 1},
		{"Chinese one minute", "<p>" + strings.Repeat("文", 400) + "</p>", 1},
		{"Chinese rounds up", "<p>" + strings.Repeat("文", 401) + "</p>", 2},
		{"long Chinese", "<p>" + strings.Repeat("文", 2400) + "</p>", 6},
		{"English one minute", "<p>" + strings.Repeat("word ", 200) + "</p>", 1},
		{"English rounds up", "<p>" + strings.Repeat("word ", 201) + "</p>", 2},
		{"long English", "<p>" + strings.Repeat("word ", 1000) + "</p>", 5},
		{"mixed effort combined", "<p>" + strings.Repeat("文", 200) + strings.Repeat(" word", 100) + "</p>", 1},
		{"mixed rounds once", "<p>" + strings.Repeat("文", 201) + strings.Repeat(" word", 100) + "</p>", 2},
		{"Japanese syllables", "<p>" + strings.Repeat("あア字", 200) + "</p>", 2},
		{"Korean syllables", "<p>" + strings.Repeat("한", 401) + "</p>", 2},
		{"long identifier is one word", "<p>" + strings.Repeat("x", 5000) + "</p>", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := articleReadingMinutes(test.html); got != test.minutes {
				t.Fatalf("got %d minutes, want %d", got, test.minutes)
			}
		})
	}
}

func TestArticleReadingTextMarkupEscapesURLsAndCode(t *testing.T) {
	noise := strings.Repeat("irrelevant ", 500)
	markup := `<p>inter<strong>national</strong> reader &amp; 中&#x6587;</p>` +
		`<pre class="chroma"><code><span class="line"><span class="cl"><span class="k">return</span> <span class="n">user</span><span class="p">.</span><span class="n">name</span></span></span></code></pre>` +
		`<script>` + noise + `</script><style>` + noise + `</style><template>` + noise + `</template>` +
		`<span hidden>` + noise + `</span><span aria-hidden="true">` + noise + `</span><span class="ln">12345</span>` +
		`<p><a href="https://example.org/` + noise + `" data-json="` + noise + `">Read docs</a> https://example.org/very/long/path?source=ignored，继续。</p>`
	text := articleReadingText(markup)
	if strings.Contains(text, "irrelevant") || strings.Contains(text, "12345") || !strings.Contains(text, "international") || !strings.Contains(text, "中文") || !strings.Contains(text, "return user.name") {
		t.Fatal("presentation leaked or readable text was split/lost", text)
	}
	if cjk, words := articleReadingUnits(text); cjk != 4 || words != 7 {
		t.Fatalf("decoded visible text: got %d CJK characters and %d words, want 4 and 7: %q", cjk, words, text)
	}
	if cjk, words := articleReadingUnits("Go语言 don't state-of-the-art cafe\u0301 2026"); cjk != 2 || words != 5 {
		t.Fatal("mixed/Unicode word boundaries", cjk, words)
	}
	code := "```go\n" + strings.Repeat("identifier ", 201) + "\n```"
	rendered, err := RenderMarkdown(code)
	if err != nil || articleReadingMinutes(rendered.HTML) != 2 {
		t.Fatal("readable highlighted code words were omitted", rendered, err)
	}
	urlsAndSymbols := "```text\n" + strings.Repeat("https://example.org/long/path?key=value {} [] ();\n", 500) + "```"
	rendered, err = RenderMarkdown(urlsAndSymbols)
	if err != nil || articleReadingMinutes(rendered.HTML) != 1 {
		t.Fatal("source URLs or punctuation inflated reading time", err)
	}
}

func TestArticleReadingMinutesCountVisibleCardFields(t *testing.T) {
	cards := []contentCard{
		{V: 2, Type: "callout", Tone: "note", Title: "提示", Body: strings.Repeat("文", 200)},
		{V: 2, Type: "metrics", Metrics: []ContentCardMetric{{Label: "Count", Value: "42", Detail: strings.Repeat("文", 200), Source: &ContentCardSource{Label: "Report", URL: "https://example.org/" + strings.Repeat("opaque/", 100)}}}},
		{V: 2, Type: "summary", Title: "摘要", Entries: []ContentCardEntry{{Title: "结论", Body: strings.Repeat("文", 200)}}},
		{V: 3, Type: "layout", Root: &ContentLayoutNode{Kind: "panel", Children: []*ContentLayoutNode{
			{Kind: "text", Role: "title", Text: "组合"},
			{Kind: "grid", Children: []*ContentLayoutNode{
				{Kind: "stack", Children: []*ContentLayoutNode{{Kind: "text", Role: "body", Text: strings.Repeat("文", 200)}, {Kind: "link", Text: "Source", URL: "https://example.org/reference"}, {Kind: "image", Src: "/media/0123456789abcdef", Alt: "图片说明"}}},
				{Kind: "stack", Children: []*ContentLayoutNode{{Kind: "text", Role: "value", Text: "17"}, {Kind: "text", Role: "detail", Text: "补充"}}},
			}},
		}}},
	}
	var markdown []string
	for _, card := range cards {
		markdown = append(markdown, contentCardMarkdown(card))
	}
	rendered, err := RenderMarkdown(strings.Join(markdown, "\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	text := articleReadingText(rendered.HTML)
	if cjk, words := articleReadingUnits(text); cjk != 810 || words != 5 {
		t.Fatalf("v2/v3 card text units: got %d CJK characters and %d words, want 810 and 5", cjk, words)
	}
	if got := articleReadingMinutes(rendered.HTML); got != 3 {
		t.Fatal("multiple rendered cards were omitted", got)
	}
	external := contentCard{V: 1, Type: "link", URL: "https://example.org/" + strings.Repeat("path/", 100), Title: "Useful", Subtitle: "Linked", Description: "Story"}
	rendered, err = RenderMarkdown(contentCardMarkdown(external))
	if err != nil {
		t.Fatal(err)
	}
	if _, words := articleReadingUnits(articleReadingText(rendered.HTML)); words != 3 {
		t.Fatal("displayed card fields merged or URL metadata counted", rendered.HTML, words)
	}
}

func TestArticleReadingMinutesIgnoreHiddenInlinePreviews(t *testing.T) {
	card := contentCard{V: 1, Type: "link", URL: "https://example.org/reference", Title: "Reference", Description: strings.Repeat("hover ", 150)}
	var inline, blocks []string
	for i := 0; i < 20; i++ {
		inline = append(inline, contentCardInlineMarkdown(card))
		blocks = append(blocks, contentCardMarkdown(card))
	}
	rendered, err := RenderMarkdown(strings.Join(inline, "\n\n"))
	if err != nil || !strings.Contains(rendered.HTML, "content-link__preview") || !strings.Contains(rendered.HTML, "hover") {
		t.Fatal("fixture lost the hidden snapshot metadata", err)
	}
	text := articleReadingText(rendered.HTML)
	if _, words := articleReadingUnits(text); words != 20 || strings.Contains(text, "hover") || articleReadingMinutes(rendered.HTML) != 1 {
		t.Fatal("hidden long descriptions inflated short inline references", words, text)
	}
	rendered, err = RenderMarkdown(strings.Join(blocks, "\n\n"))
	if err != nil || articleReadingMinutes(rendered.HTML) != 16 {
		t.Fatal("visible block-card descriptions must still contribute", err)
	}
}

func TestArticleReadingTimeUsesPublishedHTMLAndCurrentInternalReferences(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterSearch()
	a.RegisterWeb()
	_, cookie, csrf := testIdentity(t, a, "owner")
	ctx := context.Background()
	createPublished := func(markdown string) string {
		t.Helper()
		draft := map[string]any{"client_draft_id": RandID(), "title": "阅读估算验收", "markdown": markdown}
		created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", draft, 0, ""), 201)
		id := created["id"].(string)
		contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]int{"expected_draft_version": 1}, 1, ""), 200)
		return id
	}
	publicHTML := func(id string, minutes int, absent ...string) string {
		t.Helper()
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/posts/"+id, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), fmt.Sprintf("约 %d 分钟阅读", minutes)) {
			t.Fatalf("public article reading time: HTTP %d, want %d minutes", w.Code, minutes)
		}
		for _, value := range absent {
			if strings.Contains(w.Body.String(), value) {
				t.Fatalf("nonpublic content leaked into article: %q", value)
			}
		}
		return w.Body.String()
	}
	id := createPublished(strings.Repeat("文", 1200))
	var before, after string
	query := `SELECT row_to_json(r)::text FROM post_revisions r JOIN public_posts p ON p.revision_id=r.id WHERE p.post_id=$1`
	if err := a.Pool.QueryRow(ctx, query, id).Scan(&before); err != nil {
		t.Fatal(err)
	}
	publicHTML(id, 3)
	if err := a.Pool.QueryRow(ctx, query, id).Scan(&after); err != nil || before != after {
		t.Fatal("reading estimate modified an immutable published snapshot", err)
	}
	draft := map[string]any{"title": "未发布草稿", "markdown": "短文", "expected_draft_version": 1}
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", draft, 2, ""), 200)
	publicHTML(id, 3, "未发布草稿")

	var refs []string
	var phraseIDs []string
	for i := 0; i < 2; i++ {
		phrase := phraseCreateFixture(t, a, cookie, csrf, strings.Repeat("引", 280), "公开来源", true)
		phraseIDs = append(phraseIDs, phrase["id"].(string))
		refs = append(refs, contentCardMarkdown(internalCardFixture("phrase", phrase["id"].(string))))
	}
	referenced := createPublished(strings.Join(refs, "\n\n"))
	publicHTML(referenced, 2)
	for _, phraseID := range phraseIDs {
		contentWant(t, phraseCall(t, a, cookie, csrf, "PUT", "/api/v1/admin/phrases/"+phraseID, map[string]any{"text": "private replacement", "kind": "sentence", "source": "private source", "published": false}, 1, RandID()), 200)
	}
	publicHTML(referenced, 1, "private replacement", "private source", strings.Repeat("引", 30), "公开来源")
}
