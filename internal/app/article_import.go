package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	importdocs "painter-next/docs"
)

const articleImportLimit = 1 << 20

type articleImportResult struct {
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	Markdown   string `json:"markdown"`
	BlockCount int    `json:"block_count"`
	CardCount  int    `json:"card_count"`
	Rendered
}

func articleImportInvalid(message string) error { return E(400, "INVALID_ARTICLE_IMPORT", message) }

// A canonical exchange file cannot contain duplicate keys, null values or
// excessive nesting. This also prevents different JSON clients seeing different
// versions of a field. It is inert data: no URL resolution or script execution.
func validateArticleImportJSON(raw []byte) error {
	if len(raw) > articleImportLimit {
		return E(413, "PAYLOAD_TOO_LARGE", "导入文件最多 1 MiB")
	}
	if !utf8.Valid(raw) {
		return articleImportInvalid("导入文件必须是 UTF-8 JSON")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 24 {
			return articleImportInvalid("JSON 嵌套过深")
		}
		t, err := d.Token()
		if err != nil {
			return articleImportInvalid("JSON 格式无效，请导出完整 JSON 对象")
		}
		if t == nil {
			return articleImportInvalid("字段不能为 null；没有内容时请省略可选字段")
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				keys := map[string]bool{}
				for d.More() {
					key, e := d.Token()
					if e != nil {
						return articleImportInvalid("JSON 对象无效")
					}
					s, ok := key.(string)
					if !ok || keys[s] {
						return articleImportInvalid("JSON 不能包含重复字段")
					}
					keys[s] = true
					if e = value(depth + 1); e != nil {
						return e
					}
				}
			case '[':
				for d.More() {
					if e := value(depth + 1); e != nil {
						return e
					}
				}
			default:
				return articleImportInvalid("JSON 格式无效")
			}
			if _, err = d.Token(); err != nil {
				return articleImportInvalid("JSON 对象不完整")
			}
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return articleImportInvalid("只允许一个 JSON 对象，不要包含代码围栏或说明文字")
	}
	return nil
}

func decodeArticleImport(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return articleImportInvalid("JSON 字段或类型无效，请按照 Painter 格式重新导出")
	}
	return nil
}

func validateImportedCard(raw []byte) (contentCard, error) {
	card, known, err := parseContentCard(raw)
	if err != nil {
		return card, err
	}
	if !known {
		return card, articleImportInvalid("卡片缺失或类型、版本不受支持")
	}
	if card.V == 1 {
		if _, err = contentPanelObject(raw, "v", "type", "display", "url", "title", "ref_id", "description", "subtitle", "image_url", "stats", "metadata_source", "resolved_at"); err != nil {
			return card, err
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		if stats, ok := fields["stats"]; ok {
			if _, err = contentPanelObject(stats, "stars", "forks"); err != nil {
				return card, err
			}
		}
	}
	if card.ImageURL != "" && !mediaReference.MatchString(card.ImageURL) {
		return card, articleImportInvalid("导入图片需先上传，再使用 /media/{id}，不能引用远程图片")
	}
	return card, nil
}

func articleImportText(s string, max int, multiline bool) bool {
	if utf8.RuneCountInString(s) > max {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\r')) {
			return false
		}
	}
	return true
}

// Count and validate known cards, including fences supplied inside Markdown.
// Unknown future cards remain readable in ordinary articles, but an exchange
// file must be unambiguous and completely supported before it can be imported.
func inspectArticleImportMarkdown(markdown string) (int, error) {
	source := []byte(markdown)
	doc := goldmark.New().Parser().Parse(text.NewReader(source))
	cards := 0
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.RawHTML, *ast.HTMLBlock:
			return ast.WalkStop, articleImportInvalid("正文含原始 HTML，请让 AI 转换为 Markdown 或 Painter 卡片")
		case *ast.FencedCodeBlock:
			if string(node.Language(source)) != "painter-card" {
				return ast.WalkContinue, nil
			}
			raw := node.Lines().Value(source)
			if err := validateArticleImportJSON(raw); err != nil {
				return ast.WalkStop, err
			}
			if _, err := validateImportedCard(raw); err != nil {
				return ast.WalkStop, err
			}
			cards++
		case *ast.Link:
			if !strings.HasPrefix(string(node.Title), "painter:") {
				return ast.WalkContinue, nil
			}
			if !strings.HasPrefix(string(node.Title), contentCardInlinePrefix) || len(node.Title) > contentCardLimit*2 {
				return ast.WalkStop, articleImportInvalid("行内卡片版本或大小无效")
			}
			raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(string(node.Title), contentCardInlinePrefix))
			if err != nil {
				return ast.WalkStop, articleImportInvalid("行内卡片编码无效")
			}
			if err = validateArticleImportJSON(raw); err != nil {
				return ast.WalkStop, err
			}
			card, err := validateImportedCard(raw)
			if err != nil {
				return ast.WalkStop, err
			}
			if card.Type != "link" || card.V != 1 || card.Display == "title" || string(node.Destination) != card.URL {
				return ast.WalkStop, articleImportInvalid("行内引用须为同一网址的 link 卡片")
			}
			cards++
		}
		return ast.WalkContinue, nil
	})
	return cards, err
}

func compileArticleImport(raw []byte) (articleImportResult, error) {
	var result articleImportResult
	if err := validateArticleImportJSON(raw); err != nil {
		return result, err
	}
	if _, err := contentPanelObject(raw, "format", "version", "title", "summary", "blocks"); err != nil {
		return result, articleImportInvalid("正文文件字段名称必须与 Painter 格式完全一致")
	}
	var in struct {
		Format  string            `json:"format"`
		Version int               `json:"version"`
		Title   string            `json:"title"`
		Summary string            `json:"summary"`
		Blocks  []json.RawMessage `json:"blocks"`
	}
	if err := decodeArticleImport(raw, &in); err != nil {
		return result, err
	}
	if in.Format != "painter.article" || in.Version != 1 {
		return result, articleImportInvalid("需要 format=painter.article、version=1 的正文文件")
	}
	if strings.TrimSpace(in.Title) == "" || !articleImportText(in.Title, 200, false) || !articleImportText(in.Summary, 500, true) {
		return result, articleImportInvalid("标题需为 1–200 字单行文本，摘要最多 500 字")
	}
	if len(in.Blocks) < 1 || len(in.Blocks) > 256 {
		return result, articleImportInvalid("正文需要 1–256 个区块")
	}
	parts := make([]string, 0, len(in.Blocks))
	cards := 0
	for i, rawBlock := range in.Blocks {
		fail := func(err error) (articleImportResult, error) {
			return result, articleImportInvalid(fmt.Sprintf("第 %d 个区块：%s", i+1, err.Error()))
		}
		var identity struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(rawBlock, &identity)
		switch identity.Type {
		case "markdown":
			if _, err := contentPanelObject(rawBlock, "type", "markdown"); err != nil {
				return fail(err)
			}
			var block struct {
				Type     string `json:"type"`
				Markdown string `json:"markdown"`
			}
			if err := decodeArticleImport(rawBlock, &block); err != nil {
				return fail(err)
			}
			if strings.TrimSpace(block.Markdown) == "" {
				return fail(articleImportInvalid("Markdown 不能为空"))
			}
			n, err := inspectArticleImportMarkdown(block.Markdown)
			if err != nil {
				return fail(err)
			}
			cards += n
			parts = append(parts, block.Markdown)
		case "card":
			if _, err := contentPanelObject(rawBlock, "type", "card"); err != nil {
				return fail(err)
			}
			var block struct {
				Type string          `json:"type"`
				Card json.RawMessage `json:"card"`
			}
			if err := decodeArticleImport(rawBlock, &block); err != nil {
				return fail(err)
			}
			card, err := validateImportedCard(block.Card)
			if err != nil {
				return fail(err)
			}
			cards++
			parts = append(parts, contentCardMarkdown(card))
		default:
			return fail(articleImportInvalid("区块类型只支持 markdown 或 card"))
		}
	}
	markdown := strings.Join(parts, "\n\n")
	if cards > 128 {
		return result, articleImportInvalid("一篇文章最多 128 张卡片")
	}
	count, err := inspectArticleImportMarkdown(markdown)
	if err != nil {
		return result, err
	}
	if count != cards {
		return result, articleImportInvalid("Markdown 围栏或缩进吞并了相邻卡片，请补全代码围栏后重新导入")
	}
	rendered, err := RenderMarkdown(markdown)
	if err != nil {
		return result, err
	}
	return articleImportResult{Title: in.Title, Summary: in.Summary, Markdown: markdown, BlockCount: len(in.Blocks), CardCount: cards, Rendered: rendered}, nil
}

func (a *App) articleImportPreview(w http.ResponseWriter, r *http.Request) error {
	// Write authenticates, validates CSRF and rechecks current role under the
	// policy barrier before decoding/rendering this bounded, potentially costly input.
	result, err := a.Write(r, []string{"author", "owner"}, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		raw, err := bodyBytes(r)
		if err != nil {
			return nil, err
		}
		compiled, err := compileArticleImport(raw)
		if err != nil {
			return nil, err
		}
		if err = contentLockRefs(r.Context(), tx, p, draftInput{MediaIDs: compiled.MediaIDs}); err != nil {
			return nil, err
		}
		compiled.HTML, err = a.expandInternalContentCards(r.Context(), tx, compiled.HTML)
		return compiled, err
	})
	if err == nil {
		JSON(w, 200, result)
	}
	return err
}

func (a *App) articleImportFormat(w http.ResponseWriter, r *http.Request) error {
	p, err := a.Principal(r)
	if err != nil {
		return err
	}
	if !hasRole(p.Role, []string{"author", "owner"}) {
		return E(403, "PERMISSION_DENIED", "只有创作者可以使用正文导入")
	}
	JSON(w, 200, map[string]string{"schema": importdocs.ArticleImportSchema, "example": importdocs.ArticleImportExample, "prompt": importdocs.ArticleImportPrompt})
	return nil
}
