package app

import (
	"bytes"
	"encoding/json"
	stdhtml "html"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ContentCardSource is an optional, author-supplied citation for a metric.
// Rendering these links never fetches their destinations.
type ContentCardSource struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type ContentCardMetric struct {
	Label  string             `json:"label"`
	Value  string             `json:"value"`
	Detail string             `json:"detail,omitempty"`
	Source *ContentCardSource `json:"source,omitempty"`
}

type ContentCardEntry struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func contentPanelType(kind string) bool {
	return kind == "callout" || kind == "metrics" || kind == "summary"
}

// Each v2 shape has its own field allow-list. The shared Go representation is
// deliberately not permission to mix panel fields with v1 provider snapshots.
func contentPanelObject(raw []byte, allowed ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, contentCardInvalid("面板字段需要 JSON 对象")
	}
	for key, value := range fields {
		known := false
		for _, name := range allowed {
			known = known || key == name
		}
		if !known || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, contentCardInvalid("面板字段或格式不正确")
		}
	}
	return fields, nil
}

func contentPanelFields(raw []byte, kind string) error {
	allowed := []string{"v", "type", "title"}
	switch kind {
	case "callout":
		allowed = append(allowed, "tone", "body")
	case "metrics":
		allowed = append(allowed, "metrics")
	case "summary":
		allowed = append(allowed, "entries")
	}
	fields, err := contentPanelObject(raw, allowed...)
	if err != nil {
		return err
	}
	if kind == "callout" {
		return nil
	}
	key := "metrics"
	if kind == "summary" {
		key = "entries"
	}
	var items []json.RawMessage
	if json.Unmarshal(fields[key], &items) != nil || len(items) < 1 || len(items) > 6 {
		return contentCardInvalid("面板需要1至6项内容")
	}
	for _, raw := range items {
		if kind == "summary" {
			if _, err = contentPanelObject(raw, "title", "body"); err != nil {
				return err
			}
			continue
		}
		metric, err := contentPanelObject(raw, "label", "value", "detail", "source")
		if err != nil {
			return err
		}
		if source, ok := metric["source"]; ok {
			if _, err = contentPanelObject(source, "label", "url"); err != nil {
				return err
			}
		}
	}
	return nil
}

func contentPanelText(value string, limit int, required bool) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= limit && (!required || strings.TrimSpace(value) != "") && !strings.ContainsFunc(value, func(r rune) bool {
		return unicode.IsControl(r) && r != '\n' && r != '\r'
	})
}

func validateContentPanel(c contentCard) error {
	if c.URL != "" || c.RefID != "" || c.Description != "" || c.Subtitle != "" || c.ImageURL != "" || c.Stats != nil || c.MetadataSource != "" || c.ResolvedAt != "" {
		return contentCardInvalid("内容面板不能附带链接或供应商快照")
	}
	if !contentCardText(c.Title, 200, false) {
		return contentCardInvalid("面板标题超长或含控制字符")
	}
	switch c.Type {
	case "callout":
		if c.Tone != "note" && c.Tone != "insight" && c.Tone != "warning" || !contentPanelText(c.Body, 2000, true) || c.Metrics != nil || c.Entries != nil {
			return contentCardInvalid("提示面板需要有效语气和1至2000字正文")
		}
	case "metrics":
		if c.Tone != "" || c.Body != "" || c.Entries != nil || len(c.Metrics) < 1 || len(c.Metrics) > 6 {
			return contentCardInvalid("指标面板需要1至6项指标")
		}
		for _, metric := range c.Metrics {
			if !contentCardText(metric.Label, 120, true) || !contentCardText(metric.Value, 100, true) || !contentPanelText(metric.Detail, 500, false) {
				return contentCardInvalid("指标名称、数值或说明长度无效或含控制字符")
			}
			if metric.Source != nil {
				if !contentCardText(metric.Source.Label, 120, true) {
					return contentCardInvalid("指标来源需要1至120字名称")
				}
				if _, err := contentCardPublicURL(metric.Source.URL); err != nil {
					return err
				}
			}
		}
	case "summary":
		if c.Tone != "" || c.Body != "" || c.Metrics != nil || len(c.Entries) < 1 || len(c.Entries) > 6 {
			return contentCardInvalid("摘要面板需要1至6项内容")
		}
		for _, entry := range c.Entries {
			if !contentCardText(entry.Title, 200, true) || !contentPanelText(entry.Body, 2000, true) {
				return contentCardInvalid("摘要项目需要1至200字标题和1至2000字正文")
			}
		}
	}
	return nil
}

func contentPanelParagraph(class, value string) string {
	if value == "" {
		return ""
	}
	text := stdhtml.EscapeString(value)
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	return `<p class="content-panel__` + class + `">` + strings.ReplaceAll(text, "\n", "<br />") + `</p>`
}

func contentPanelHTML(c contentCard) string {
	classes := "content-panel content-panel--" + c.Type
	if c.Type == "callout" {
		classes += " content-panel--" + c.Tone
	}
	markup := `<div class="` + classes + `">` + contentPanelParagraph("title", c.Title)
	if c.Type == "callout" {
		return markup + contentPanelParagraph("body", c.Body) + "</div>\n"
	}
	markup += `<div class="content-panel__grid">`
	if c.Type == "metrics" {
		for _, metric := range c.Metrics {
			markup += `<div class="content-panel__item">` + contentPanelParagraph("label", metric.Label) + contentPanelParagraph("value", metric.Value) + contentPanelParagraph("detail", metric.Detail)
			if metric.Source != nil {
				markup += `<a class="content-panel__source" href="` + stdhtml.EscapeString(metric.Source.URL) + `" target="_blank" rel="noopener noreferrer">` + stdhtml.EscapeString(metric.Source.Label) + `</a>`
			}
			markup += `</div>`
		}
	} else {
		for _, entry := range c.Entries {
			markup += `<div class="content-panel__item">` + contentPanelParagraph("label", entry.Title) + contentPanelParagraph("body", entry.Body) + `</div>`
		}
	}
	return markup + "</div></div>\n"
}
