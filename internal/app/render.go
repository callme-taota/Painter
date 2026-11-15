package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	xhtml "golang.org/x/net/html"
)

const RendererVersion = "goldmark-gfm-v1;painter-cards-v1;painter-panels-v2;layout-v3;unicode-nfkc-fold-nfkc-v1"

type TOCEntry struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Level int    `json:"level"`
}
type Rendered struct {
	HTML     string     `json:"sanitized_html"`
	Text     string     `json:"plain_text"`
	TOC      []TOCEntry `json:"toc"`
	Hash     string     `json:"content_hash"`
	MediaIDs []string   `json:"media_ids"`
	Version  string     `json:"renderer_version"`
}

var mediaReference = regexp.MustCompile(`^/media/([a-fA-F0-9-]{16,128})$`)

// RenderMarkdown is the sole preview/checkpoint/publish pipeline. Raw HTML is
// omitted by goldmark, and an independent allow-list sanitizes the result.
func RenderMarkdown(markdown string) (Rendered, error) {
	if len(markdown) > 1<<20 || !utf8.ValidString(markdown) {
		return Rendered{}, E(413, "PAYLOAD_TOO_LARGE", "正文最多1 MiB有效UTF-8文本")
	}
	engine := goldmark.New(goldmark.WithExtensions(extension.GFM, highlighting.NewHighlighting(highlighting.WithFormatOptions(chromahtml.WithClasses(true)))), goldmark.WithParserOptions(parser.WithAutoHeadingID()), goldmark.WithRendererOptions(html.WithXHTML()))
	engine.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(contentCardRenderer{}, 100)))
	source := []byte(markdown)
	doc := engine.Parser().Parse(text.NewReader(source))
	toc := []TOCEntry{}
	media := map[string]bool{}
	if err := prepareContentCards(doc, source, media); err != nil {
		return Rendered{}, err
	}
	counter := 0
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if h, ok := n.(*ast.Heading); ok {
			counter++
			id := "section-" + strconv.Itoa(counter)
			h.SetAttributeString("id", []byte(id))
			toc = append(toc, TOCEntry{ID: id, Text: string(h.Text(source)), Level: h.Level})
		}
		if image, ok := n.(*ast.Image); ok {
			dest := string(image.Destination)
			u, e := url.Parse(dest)
			if e != nil || u.Scheme != "" || u.Host != "" || !mediaReference.MatchString(u.Path) || u.RawQuery != "" {
				return ast.WalkStop, E(400, "VALIDATION_FAILED", "文章图片必须先上传并使用 /media/{id} 引用")
			}
			media[mediaReference.FindStringSubmatch(u.Path)[1]] = true
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return Rendered{}, err
	}
	var buffer bytes.Buffer
	if err = engine.Renderer().Render(&buffer, source, doc); err != nil {
		return Rendered{}, err
	}
	policy := bluemonday.UGCPolicy()
	policy.AllowElements("input")
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^[a-zA-Z0-9_ -]+$`)).OnElements("span", "pre", "code")
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^content-(?:card|link)(?:__[a-z]+|--[a-z_]+)?(?: content-card--[a-z_]+)?$`)).OnElements("a", "img")
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^(?:content-panel content-panel--callout content-panel--(?:note|insight|warning)|content-panel content-panel--(?:metrics|summary)|content-panel content-panel--layout(?: content-panel--(?:note|insight|warning))?|content-panel__(?:grid|item|stack))$`)).OnElements("div")
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^content-panel__(?:title|body|label|value|detail)$`)).OnElements("p")
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^content-panel__source$`)).OnElements("a")
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^content-panel__image$`)).OnElements("img")
	policy.AllowAttrs("loading").Matching(regexp.MustCompile(`^lazy$`)).OnElements("img")
	policy.AllowAttrs("referrerpolicy").Matching(regexp.MustCompile(`^no-referrer$`)).OnElements("img")
	policy.AllowAttrs("target").Matching(regexp.MustCompile(`^_blank$`)).OnElements("a")
	policy.AllowAttrs("rel").Matching(regexp.MustCompile(`^noopener noreferrer$`)).OnElements("a")
	policy.AllowAttrs("id").Matching(regexp.MustCompile(`^section-\d+$`)).OnElements("h1", "h2", "h3", "h4", "h5", "h6")
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^language-[\w+-]+$`)).OnElements("code")
	policy.AllowAttrs("type", "checked", "disabled").OnElements("input")
	safe := policy.SanitizeBytes(buffer.Bytes())
	root, err := xhtml.Parse(bytes.NewReader(safe))
	if err != nil {
		return Rendered{}, err
	}
	var parts []string
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			parts = append(parts, n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	hash := sha256.Sum256([]byte(RendererVersion + "\x00" + markdown))
	ids := make([]string, 0, len(media))
	for id := range media {
		ids = append(ids, id)
	}
	return Rendered{HTML: string(safe), Text: strings.Join(strings.Fields(strings.Join(parts, " ")), " "), TOC: toc, Hash: hex.EncodeToString(hash[:]), MediaIDs: ids, Version: RendererVersion}, nil
}

func contentHash(title, summary, markdown string, category *string, tags, media []string) string {
	raw, _ := json.Marshal([]any{RendererVersion, title, summary, markdown, category, tags, media})
	h := sha256.Sum256(raw)
	return fmt.Sprintf("%x", h)
}
