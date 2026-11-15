package app

// Article cards are portable, versioned Markdown snapshots. The registry only
// interprets explicitly marked AST nodes; rendering never contacts a provider.
import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"io"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

const contentCardLimit = 8 << 10
const contentCardInlinePrefix = "painter:v1:"

type contentCardStats struct {
	Stars *int64 `json:"stars,omitempty"`
	Forks *int64 `json:"forks,omitempty"`
}

type contentCard struct {
	V              int                 `json:"v"`
	Type           string              `json:"type"`
	Display        string              `json:"display,omitempty"`
	URL            string              `json:"url,omitempty"`
	Title          string              `json:"title,omitempty"`
	RefID          string              `json:"ref_id,omitempty"`
	Description    string              `json:"description,omitempty"`
	Subtitle       string              `json:"subtitle,omitempty"`
	ImageURL       string              `json:"image_url,omitempty"`
	Stats          *contentCardStats   `json:"stats,omitempty"`
	MetadataSource string              `json:"metadata_source,omitempty"`
	ResolvedAt     string              `json:"resolved_at,omitempty"`
	Tone           string              `json:"tone,omitempty"`
	Body           string              `json:"body,omitempty"`
	Metrics        []ContentCardMetric `json:"metrics,omitempty"`
	Entries        []ContentCardEntry  `json:"entries,omitempty"`
	Root           *ContentLayoutNode  `json:"root,omitempty"`
}

type contentCardDefinition struct{ Label string }

// New snapshot versions/types are opt-in. Unknown ones retain readable fenced
// source, so future card pools can be added without a database dependency.
var contentCardRegistry = map[string]contentCardDefinition{
	"github": {"GitHub"}, "steam": {"Steam"}, "apple_music": {"Apple Music"}, "link": {"网页参考"},
	"moment": {"动态"}, "phrase": {"片语"},
}

// contentCardInvalid accepts validator-owned UI copy. Dynamic request values
// belong in contentCardInvalidT placeholders so they remain opaque.
func contentCardInvalid(message string) error {
	return contentCardInvalidT(message)
}

func contentCardInvalidT(source string, args ...any) error {
	return ET(400, "CARD_INVALID", "内容卡片无效：{reason}", "reason", translatableMessage(source, args...))
}

func contentCardText(value string, limit int, required bool) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= limit && (!required || strings.TrimSpace(value) != "") && !strings.ContainsFunc(value, unicode.IsControl)
}

// URL validation is also used before every server-side request/redirect. The
// transport separately resolves and pins public addresses to prevent rebinding.
func contentCardPublicURL(raw string) (*url.URL, error) {
	clean, e := friendURL(raw, true)
	if e != nil || raw != clean {
		return nil, contentCardInvalid("请输入完整的 HTTPS 网址")
	}
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Port() != "" || strings.HasSuffix(u.Hostname(), ".") {
		return nil, contentCardInvalid("请输入完整的 HTTPS 网址")
	}
	host := strings.ToLower(u.Hostname())
	ip, ipError := netip.ParseAddr(host)
	if ipError == nil {
		if !contentCardPublicIP(ip) {
			return nil, contentCardInvalid("不支持本地或私有网址")
		}
	} else if !strings.Contains(host, ".") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return nil, contentCardInvalid("不支持本地或私有网址")
	}
	return u, nil
}

func contentCardImage(c contentCard) (string, error) {
	if c.ImageURL == "" {
		return "", nil
	}
	if mediaReference.MatchString(c.ImageURL) {
		return mediaReference.FindStringSubmatch(c.ImageURL)[1], nil
	}
	u, e := contentCardPublicURL(c.ImageURL)
	if e != nil || u.Fragment != "" {
		return "", contentCardInvalid("图片须为已上传媒体或供应商的 HTTPS 图片")
	}
	valid := false
	switch c.Type {
	case "github":
		valid = u.Host == "avatars.githubusercontent.com"
	case "steam":
		valid = homeRemoteImage("steam", c.ImageURL)
	case "apple_music":
		valid = homeRemoteImage("music", c.ImageURL)
	}
	if !valid {
		return "", contentCardInvalid("图片须为已上传媒体或供应商的 HTTPS 图片")
	}
	return "", nil
}

func validateContentCard(c contentCard) (string, error) {
	if c.Display != "" && (c.V != 1 || c.Type != "link" || c.Display != "card" && c.Display != "title") {
		return "", contentCardInvalid("只有 v1 网页 link 支持 card 或 title 呈现方式")
	}
	if c.V == 3 && c.Type == "layout" {
		return "", validateContentLayout(c)
	}
	if c.V == 2 && contentPanelType(c.Type) {
		return "", validateContentPanel(c)
	}
	if c.Root != nil || c.Tone != "" || c.Body != "" || c.Metrics != nil || c.Entries != nil {
		return "", contentCardInvalid("此版本不支持内容面板字段")
	}
	if contentCardIsInternal(c.Type) {
		if !contentCardReferenceID(c.RefID) || c.URL != contentCardInternalURL(c.Type, c.RefID) {
			return "", contentCardInvalid("站内引用需要有效的内容编号和对应站内链接")
		}
		// Internal cards deliberately persist identity only. Storing text or image
		// snapshots here would let a withdrawn source remain public in articles.
		if c.Title != "" || c.Description != "" || c.Subtitle != "" || c.ImageURL != "" || c.Stats != nil || c.MetadataSource != "" || c.ResolvedAt != "" {
			return "", contentCardInvalid("站内卡片只能保存引用，不能附带正文、图片或元数据快照")
		}
		return "", nil
	}
	if c.RefID != "" {
		return "", contentCardInvalid("外部卡片不能包含站内引用编号")
	}
	if !contentCardText(c.Title, 200, true) || !contentCardText(c.Description, 1000, false) || !contentCardText(c.Subtitle, 200, false) {
		return "", contentCardInvalid("标题、简介或副标题超长或含控制字符")
	}
	if c.MetadataSource != "" && c.MetadataSource != "provider" && c.MetadataSource != "author" {
		return "", contentCardInvalid("元数据来源无效")
	}
	if c.ResolvedAt != "" {
		if _, e := time.Parse(time.RFC3339, c.ResolvedAt); e != nil {
			return "", contentCardInvalid("快照时间无效")
		}
	}
	u, e := contentCardPublicURL(c.URL)
	if e != nil {
		return "", e
	}
	switch c.Type {
	case "github":
		if _, _, e = parseGitHubRepository(c.URL); e != nil {
			return "", e
		}
	case "steam":
		if _, _, e = homeParseImport("steam", c.URL); e != nil {
			return "", contentCardInvalid("Steam 卡片需要商店游戏链接")
		}
	case "apple_music":
		if _, _, _, e = homeMusicParseURL(c.URL, "album"); e != nil {
			if _, _, _, e = homeMusicParseURL(c.URL, "playlist"); e != nil {
				return "", contentCardInvalid("Apple Music 卡片需要专辑或歌单链接")
			}
		}
	case "link":
		_ = u
	}
	if c.Stats != nil {
		if c.Type != "github" {
			return "", contentCardInvalid("此卡片不支持统计数据")
		}
		for _, n := range []*int64{c.Stats.Stars, c.Stats.Forks} {
			if n != nil && (*n < 0 || *n > 1<<53) {
				return "", contentCardInvalid("统计数据无效")
			}
		}
	}
	return contentCardImage(c)
}

func parseContentCard(raw []byte) (contentCard, bool, error) {
	var c contentCard
	if len(raw) > contentCardLimit || !utf8.Valid(raw) {
		return c, false, contentCardInvalid("快照最多8 KiB有效 UTF-8 JSON")
	}
	var identity struct {
		V    int    `json:"v"`
		Type string `json:"type"`
	}
	if e := json.Unmarshal(raw, &identity); e != nil {
		return c, false, contentCardInvalid("需要一个 JSON 对象")
	}
	_, legacyType := contentCardRegistry[identity.Type]
	if !(identity.V == 1 && legacyType || identity.V == 2 && contentPanelType(identity.Type) || identity.V == 3 && identity.Type == "layout") {
		return c, false, nil
	}
	if e := contentCardDisplayFields(raw, identity.V, identity.Type); e != nil {
		return c, true, e
	}
	if identity.V == 3 {
		if e := contentLayoutFields(raw); e != nil {
			return c, true, e
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&c); e != nil || d.Decode(new(any)) != io.EOF {
		return c, true, contentCardInvalid("JSON 字段或格式不正确")
	}
	if c.V == 2 {
		if e := contentPanelFields(raw, c.Type); e != nil {
			return c, true, e
		}
	} else if c.V == 1 {
		// Adding a new version must not make its fields valid on v1 cards,
		// including explicitly empty or null values.
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		for key := range fields {
			switch strings.ToLower(key) {
			case "tone", "body", "metrics", "entries", "root":
				return c, true, contentCardInvalid("此版本不支持内容面板字段")
			}
		}
	}
	if contentCardIsInternal(c.Type) {
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(raw, &fields)
		for key := range fields {
			if key != "v" && key != "type" && key != "url" && key != "ref_id" {
				return c, true, contentCardInvalid("站内卡片只能包含版本、类型、链接和引用编号")
			}
		}
	}
	_, e := validateContentCard(c)
	return c, true, e
}

// Old snapshots without display retain their existing decoding behavior. A new
// presentation field is exact and unambiguous, including when explicitly empty.
func contentCardDisplayFields(raw []byte, version int, kind string) error {
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	for key, value := range fields {
		if !strings.EqualFold(key, "display") {
			continue
		}
		var display string
		if key != "display" || version != 1 || kind != "link" || json.Unmarshal(value, &display) != nil || display != "card" && display != "title" {
			return contentCardInvalid("只有 v1 网页 link 支持 display: card 或 title")
		}
		if err := strictContentLayoutJSON(raw); err != nil {
			return contentCardInvalid("呈现方式需要严格 JSON，不能包含重复字段或 null")
		}
		if _, err := contentPanelObject(raw, "v", "type", "display", "url", "title", "description", "subtitle", "image_url", "stats", "metadata_source", "resolved_at"); err != nil {
			return err
		}
	}
	return nil
}

var contentCardBlockKind = ast.NewNodeKind("PainterCardBlock")
var contentCardInlineKind = ast.NewNodeKind("PainterCardInline")

type contentCardBlockNode struct {
	ast.BaseBlock
	Card contentCard
}

func (*contentCardBlockNode) Kind() ast.NodeKind { return contentCardBlockKind }
func (n *contentCardBlockNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type contentCardInlineNode struct {
	ast.BaseInline
	Card contentCard
}

func (*contentCardInlineNode) Kind() ast.NodeKind { return contentCardInlineKind }
func (n *contentCardInlineNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

func prepareContentCards(doc ast.Node, source []byte, media map[string]bool) error {
	var changes []struct{ old, next ast.Node }
	e := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var raw []byte
		inline := false
		switch node := n.(type) {
		case *ast.FencedCodeBlock:
			if string(node.Language(source)) != "painter-card" {
				return ast.WalkSkipChildren, nil
			}
			raw = node.Lines().Value(source)
		case *ast.Link:
			if !strings.HasPrefix(string(node.Title), contentCardInlinePrefix) {
				return ast.WalkContinue, nil
			}
			if len(node.Title) > contentCardLimit*2 {
				return ast.WalkStop, contentCardInvalid("行内快照过大")
			}
			var e error
			raw, e = base64.RawURLEncoding.DecodeString(strings.TrimPrefix(string(node.Title), contentCardInlinePrefix))
			if e != nil {
				return ast.WalkStop, contentCardInvalid("行内快照编码无效")
			}
			inline = true
		default:
			return ast.WalkContinue, nil
		}
		c, known, e := parseContentCard(raw)
		if e != nil {
			return ast.WalkStop, e
		}
		if !known {
			return ast.WalkContinue, nil
		}
		if len(changes) >= 128 {
			return ast.WalkStop, contentCardInvalid("每篇文章最多128张卡片")
		}
		if inline && (c.Type != "link" || c.Display == "title" || string(n.(*ast.Link).Destination) != c.URL) {
			return ast.WalkStop, contentCardInvalid("行内引用须为同一网址的 link 卡片")
		}
		id, _ := contentCardImage(c)
		if id != "" {
			media[id] = true
		}
		if c.Root != nil {
			collectContentLayoutMedia(c.Root, media)
		}
		var next ast.Node = &contentCardBlockNode{Card: c}
		if inline {
			next = &contentCardInlineNode{Card: c}
		}
		changes = append(changes, struct{ old, next ast.Node }{n, next})
		return ast.WalkContinue, nil
	})
	if e != nil {
		return e
	}
	for _, change := range changes {
		for child := change.old.FirstChild(); child != nil; {
			next := child.NextSibling()
			change.next.AppendChild(change.next, child)
			child = next
		}
		change.old.Parent().ReplaceChild(change.old.Parent(), change.old, change.next)
	}
	return nil
}

type contentCardRenderer struct{}

func (contentCardRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(contentCardBlockKind, renderContentCardBlock)
	r.Register(contentCardInlineKind, renderContentCardInline)
}

func contentCardSpan(class, value string) string {
	if value == "" {
		return ""
	}
	return `<span class="` + class + `">` + stdhtml.EscapeString(value) + `</span>`
}
func contentCardDetails(c contentCard) string {
	host, _ := url.Parse(c.URL)
	markup := contentCardSpan("content-card__title", c.Title) + contentCardSpan("content-card__subtitle", c.Subtitle) + contentCardSpan("content-card__description", c.Description)
	if c.Stats != nil {
		var stats []string
		if c.Stats.Stars != nil {
			stats = append(stats, "★ "+strconv.FormatInt(*c.Stats.Stars, 10))
		}
		if c.Stats.Forks != nil {
			stats = append(stats, "Forks "+strconv.FormatInt(*c.Stats.Forks, 10))
		}
		markup += contentCardSpan("content-card__stats", strings.Join(stats, " · "))
	}
	return markup + contentCardSpan("content-card__url", host.Hostname())
}
func renderContentCardBlock(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	c := node.(*contentCardBlockNode).Card
	if c.V == 3 {
		_, e := w.WriteString(contentLayoutHTML(c.Root, ""))
		return ast.WalkSkipChildren, e
	}
	if c.V == 2 {
		_, e := w.WriteString(contentPanelHTML(c))
		return ast.WalkSkipChildren, e
	}
	if contentCardIsInternal(c.Type) {
		_, e := w.WriteString(contentCardInternalPlaceholder(c))
		return ast.WalkSkipChildren, e
	}
	if c.Type == "link" && c.Display == "title" {
		_, e := w.WriteString(contentLinkRowHTML(c))
		return ast.WalkSkipChildren, e
	}
	markup := `<a class="content-card content-card--` + c.Type + `" href="` + stdhtml.EscapeString(c.URL) + `" target="_blank" rel="noopener noreferrer">`
	if c.ImageURL != "" {
		markup += `<img class="content-card__image" src="` + stdhtml.EscapeString(c.ImageURL) + `" alt="" loading="lazy" referrerpolicy="no-referrer" />`
	}
	markup += `<span class="content-card__body">` + contentCardSpan("content-card__eyebrow", contentCardRegistry[c.Type].Label) + contentCardDetails(c) + `</span></a>` + "\n"
	_, e := w.WriteString(markup)
	return ast.WalkSkipChildren, e
}

const contentLinkRowIconPath = "M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"
const contentLinkRowArrowPath = "M7 17 17 7M7 7h10v10"

func contentLinkRowSVG(class, path string) string {
	return `<svg class="` + class + `" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false"><path d="` + path + `"></path></svg>`
}

func contentLinkRowHTML(c contentCard) string {
	destination, _ := url.Parse(c.URL)
	return `<a class="content-link-row" href="` + stdhtml.EscapeString(c.URL) + `" target="_blank" rel="noopener noreferrer">` + contentLinkRowSVG("content-link-row__icon", contentLinkRowIconPath) + contentCardSpan("content-link-row__title", c.Title) + ` ` + contentCardSpan("content-link-row__host", destination.Hostname()) + contentLinkRowSVG("content-link-row__arrow", contentLinkRowArrowPath) + `</a>` + "\n"
}
func renderContentCardInline(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	c := node.(*contentCardInlineNode).Card
	if entering {
		_, e := fmt.Fprintf(w, `<span class="content-link"><a class="content-link__anchor" href="%s" target="_blank" rel="noopener noreferrer">`, stdhtml.EscapeString(c.URL))
		return ast.WalkContinue, e
	}
	_, e := w.WriteString(`</a><span class="content-link__preview">` + contentCardDetails(c) + `</span></span>`)
	return ast.WalkContinue, e
}

func contentCardMarkdown(c contentCard) string {
	raw, _ := json.Marshal(c)
	return "```painter-card\n" + string(raw) + "\n```"
}
func contentCardInlineMarkdown(c contentCard) string {
	raw, _ := json.Marshal(c)
	// Escape link syntax; JSON stays entirely in the quoted title attribute.
	var label strings.Builder
	for _, r := range c.Title {
		if strings.ContainsRune("\\`*_{}[]<>()#+-.!|&~", r) {
			label.WriteByte('\\')
		}
		label.WriteRune(r)
	}
	destination := strings.ReplaceAll(strings.ReplaceAll(c.URL, "<", "%3C"), ">", "%3E")
	return "[" + label.String() + "](<" + destination + "> \"" + contentCardInlinePrefix + base64.RawURLEncoding.EncodeToString(raw) + "\")"
}

var githubName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$`)

func parseGitHubRepository(reference string) (string, string, error) {
	path := reference
	if strings.Contains(reference, "://") {
		u, e := contentCardPublicURL(reference)
		if e != nil || u.Host != "github.com" || u.RawQuery != "" || u.Fragment != "" {
			return "", "", contentCardInvalid("请输入 GitHub 公开仓库链接")
		}
		path = strings.Trim(u.Path, "/")
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 || !githubName.MatchString(parts[0]) || !githubName.MatchString(parts[1]) || strings.Contains(parts[0], "..") || parts[1] == "." || parts[1] == ".." {
		return "", "", contentCardInvalid("请输入 GitHub owner/repo 或公开仓库链接")
	}
	return parts[0], parts[1], nil
}
