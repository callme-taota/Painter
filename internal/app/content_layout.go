package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"io"
	"strings"
)

// ContentLayoutNode is inert article data, not a DOM or application protocol.
// Public node fields are checked by kind before this representation is used.
// A panel is allowed only at the root; grids contain stacks, and all leaves
// contain plain text, safe destinations, or already-uploaded media references.
type ContentLayoutNode struct {
	Kind     string               `json:"kind"`
	Tone     string               `json:"tone,omitempty"`
	Children []*ContentLayoutNode `json:"children,omitempty"`
	Role     string               `json:"role,omitempty"`
	Text     string               `json:"text,omitempty"`
	URL      string               `json:"url,omitempty"`
	Src      string               `json:"src,omitempty"`
	Alt      string               `json:"alt,omitempty"`
}

// Empty alt text is explicit, meaningful image data. Preserve it when an
// imported layout is compiled back to the canonical painter-card fence.
func (n ContentLayoutNode) MarshalJSON() ([]byte, error) {
	type plain ContentLayoutNode
	if n.Kind == "image" {
		return json.Marshal(struct {
			plain
			Alt string `json:"alt"`
		}{plain: plain(n), Alt: n.Alt})
	}
	return json.Marshal(plain(n))
}

// Unlike encoding/json alone, the v3 boundary rejects duplicate keys and null
// everywhere. It runs for hand-written Markdown as well as structured imports.
func strictContentLayoutJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 24 {
			return contentCardInvalid("组合布局 JSON 嵌套过深")
		}
		token, err := d.Token()
		if err != nil || token == nil {
			return contentCardInvalid("组合布局需要完整 JSON，字段不能为 null")
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					key, err := d.Token()
					name, ok := key.(string)
					if err != nil || !ok || seen[name] {
						return contentCardInvalid("组合布局不能包含重复字段")
					}
					seen[name] = true
					if err := value(depth + 1); err != nil {
						return err
					}
				}
			case '[':
				for d.More() {
					if err := value(depth + 1); err != nil {
						return err
					}
				}
			default:
				return contentCardInvalid("组合布局 JSON 格式无效")
			}
			if _, err := d.Token(); err != nil {
				return contentCardInvalid("组合布局 JSON 不完整")
			}
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return contentCardInvalid("组合布局只允许一个 JSON 对象")
	}
	return nil
}

func contentLayoutRequired(fields map[string]json.RawMessage, keys ...string) error {
	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			return contentCardInvalid("组合布局缺少字段 " + key)
		}
	}
	return nil
}

// Enforce exact, case-sensitive fields before decoding the shared Go node
// representation. An omitted field differs from an explicitly empty field.
func contentLayoutFields(raw []byte) error {
	if err := strictContentLayoutJSON(raw); err != nil {
		return err
	}
	fields, err := contentPanelObject(raw, "v", "type", "root")
	if err != nil {
		return err
	}
	if err := contentLayoutRequired(fields, "v", "type", "root"); err != nil {
		return err
	}
	var walk func([]byte, int, bool) error
	walk = func(raw []byte, depth int, gridChild bool) error {
		if depth > 6 {
			return contentCardInvalid("组合布局最多6层节点")
		}
		var identity struct {
			Kind string `json:"kind"`
		}
		if json.Unmarshal(raw, &identity) != nil || depth == 1 && identity.Kind != "panel" || gridChild && identity.Kind != "stack" {
			return contentCardInvalid("组合布局根节点须为 panel，grid 只能包含 stack")
		}
		allowed := []string{"kind"}
		required := []string{"kind"}
		switch identity.Kind {
		case "panel":
			if depth != 1 {
				return contentCardInvalid("panel 只能作为组合布局根节点")
			}
			allowed = append(allowed, "tone", "children")
			required = append(required, "children")
		case "grid", "stack":
			allowed = append(allowed, "children")
			required = append(required, "children")
		case "text":
			allowed = append(allowed, "role", "text")
			required = append(required, "role", "text")
		case "link":
			allowed = append(allowed, "text", "url")
			required = append(required, "text", "url")
		case "image":
			allowed = append(allowed, "src", "alt")
			required = append(required, "src", "alt")
		default:
			return contentCardInvalid("不支持的组合布局节点")
		}
		fields, err := contentPanelObject(raw, allowed...)
		if err != nil {
			return err
		}
		if err := contentLayoutRequired(fields, required...); err != nil {
			return err
		}
		if tone, ok := fields["tone"]; ok {
			var value string
			if json.Unmarshal(tone, &value) != nil || value != "note" && value != "insight" && value != "warning" {
				return contentCardInvalid("panel 语气须为 note、insight 或 warning；没有语气时请省略字段")
			}
		}
		if children, ok := fields["children"]; ok {
			var items []json.RawMessage
			max := 12
			if identity.Kind == "grid" {
				max = 6
			}
			if json.Unmarshal(children, &items) != nil || len(items) < 1 || len(items) > max {
				return contentCardInvalid(fmt.Sprintf("%s 需要1至%d个子节点", identity.Kind, max))
			}
			for _, item := range items {
				if err := walk(item, depth+1, identity.Kind == "grid"); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(fields["root"], 1, false)
}

func validateContentLayout(c contentCard) error {
	if c.Root == nil || c.Root.Kind != "panel" || c.URL != "" || c.Title != "" || c.RefID != "" || c.Description != "" || c.Subtitle != "" || c.ImageURL != "" || c.Stats != nil || c.MetadataSource != "" || c.ResolvedAt != "" || c.Tone != "" || c.Body != "" || c.Metrics != nil || c.Entries != nil {
		return contentCardInvalid("组合布局只允许版本、类型和 panel 根节点")
	}
	nodes, images := 0, 0
	var walk func(*ContentLayoutNode, int, bool) error
	walk = func(n *ContentLayoutNode, depth int, gridChild bool) error {
		nodes++
		if n == nil || nodes > 64 || depth > 6 {
			return contentCardInvalid("组合布局最多64个节点、6层深度")
		}
		if gridChild && n.Kind != "stack" {
			return contentCardInvalid("grid 只能包含 stack")
		}
		switch n.Kind {
		case "panel":
			if depth != 1 || n.Tone != "" && n.Tone != "note" && n.Tone != "insight" && n.Tone != "warning" {
				return contentCardInvalid("panel 根节点或语气无效")
			}
		case "grid", "stack":
		case "text":
			limits := map[string]int{"title": 200, "label": 120, "value": 100, "body": 2000, "detail": 500}
			limit, ok := limits[n.Role]
			valid := ok && contentCardText(n.Text, limit, true)
			if n.Role == "body" || n.Role == "detail" {
				valid = contentPanelText(n.Text, limit, true)
			}
			if !valid {
				return contentCardInvalid("组合文本角色或长度无效，正文和说明之外不能含换行或控制字符")
			}
		case "link":
			if !contentCardText(n.Text, 120, true) {
				return contentCardInvalid("组合链接需要1至120字单行名称")
			}
			if _, err := contentCardPublicURL(n.URL); err != nil {
				return err
			}
		case "image":
			images++
			if images > 6 || !mediaReference.MatchString(n.Src) || !contentCardText(n.Alt, 200, false) {
				return contentCardInvalid("组合图片最多6张，须使用 /media/{id} 和最多200字单行替代文本")
			}
		default:
			return contentCardInvalid("不支持的组合布局节点")
		}
		if n.Kind == "panel" || n.Kind == "grid" || n.Kind == "stack" {
			max := 12
			if n.Kind == "grid" {
				max = 6
			}
			if len(n.Children) < 1 || len(n.Children) > max {
				return contentCardInvalid(fmt.Sprintf("%s 需要1至%d个子节点", n.Kind, max))
			}
			for _, child := range n.Children {
				if err := walk(child, depth+1, n.Kind == "grid"); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(c.Root, 1, false)
}

func collectContentLayoutMedia(n *ContentLayoutNode, media map[string]bool) {
	if n.Kind == "image" {
		media[mediaReference.FindStringSubmatch(n.Src)[1]] = true
	}
	for _, child := range n.Children {
		collectContentLayoutMedia(child, media)
	}
}

// Legacy input remains legacy input. Presets compile into the same node tree,
// while a trusted renderer context preserves their existing HTML and styling.
func compileContentPanel(c contentCard) *ContentLayoutNode {
	root := &ContentLayoutNode{Kind: "panel", Tone: c.Tone}
	addText := func(parent *ContentLayoutNode, role, text string) {
		if text != "" {
			parent.Children = append(parent.Children, &ContentLayoutNode{Kind: "text", Role: role, Text: text})
		}
	}
	addText(root, "title", c.Title)
	if c.Type == "callout" {
		addText(root, "body", c.Body)
		return root
	}
	grid := &ContentLayoutNode{Kind: "grid"}
	if c.Type == "metrics" {
		for _, metric := range c.Metrics {
			item := &ContentLayoutNode{Kind: "stack"}
			addText(item, "label", metric.Label)
			addText(item, "value", metric.Value)
			addText(item, "detail", metric.Detail)
			if metric.Source != nil {
				item.Children = append(item.Children, &ContentLayoutNode{Kind: "link", Text: metric.Source.Label, URL: metric.Source.URL})
			}
			grid.Children = append(grid.Children, item)
		}
	} else {
		for _, entry := range c.Entries {
			item := &ContentLayoutNode{Kind: "stack"}
			addText(item, "label", entry.Title)
			addText(item, "body", entry.Body)
			grid.Children = append(grid.Children, item)
		}
	}
	root.Children = append(root.Children, grid)
	return root
}

func contentPanelParagraph(class, value string) string {
	if value == "" {
		return ""
	}
	text := stdhtml.EscapeString(value)
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	return `<p class="content-panel__` + class + `">` + strings.ReplaceAll(text, "\n", "<br />") + `</p>`
}

// The only HTML renderer for v2 presets and v3 compositions. Input is validated
// before reaching this function; no author-provided HTML, CSS or tag is used.
func contentLayoutHTML(root *ContentLayoutNode, preset string) string {
	var out strings.Builder
	var render func(*ContentLayoutNode, bool)
	render = func(n *ContentLayoutNode, gridChild bool) {
		switch n.Kind {
		case "panel":
			kind := preset
			if kind == "" {
				kind = "layout"
			}
			out.WriteString(`<div class="content-panel content-panel--` + kind)
			if n.Tone != "" {
				out.WriteString(" content-panel--" + n.Tone)
			}
			out.WriteString(`">`)
		case "grid":
			out.WriteString(`<div class="content-panel__grid">`)
		case "stack":
			class := "stack"
			if gridChild {
				class = "item"
			}
			out.WriteString(`<div class="content-panel__` + class + `">`)
		case "text":
			out.WriteString(contentPanelParagraph(n.Role, n.Text))
		case "link":
			out.WriteString(`<a class="content-panel__source" href="` + stdhtml.EscapeString(n.URL) + `" target="_blank" rel="noopener noreferrer">` + stdhtml.EscapeString(n.Text) + `</a>`)
		case "image":
			out.WriteString(`<img class="content-panel__image" src="` + stdhtml.EscapeString(n.Src) + `" alt="` + stdhtml.EscapeString(n.Alt) + `" loading="lazy" />`)
		}
		for _, child := range n.Children {
			render(child, n.Kind == "grid")
		}
		if n.Kind == "panel" || n.Kind == "grid" || n.Kind == "stack" {
			out.WriteString("</div>")
		}
	}
	render(root, false)
	out.WriteByte('\n')
	return out.String()
}
