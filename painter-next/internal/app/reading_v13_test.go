package app

import (
	"strings"
	"testing"
)

// Date visibility is a publication contract, including native no-JS disclosure.
func TestReadingInfoRespectsEveryPublishedDateVisibilityCombination(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterWeb()
	_, cookie, csrf := testIdentity(t, a, "author")
	for mask := 0; mask < 8; mask++ {
		dates := map[string]bool{"created": mask&1 != 0, "updated": mask&2 != 0, "published": mask&4 != 0}
		created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "阅读时间合同", "summary": "首屏摘要", "markdown": "## 正文标题\n\n第一段正文。\n\n---\n\n第二段正文。", "cover": map[string]any{"mode": "none"}, "date_display": dates}, 0, ""), 201)
		id := created["id"].(string)
		contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
		response := contentCall(t, a, nil, "", "GET", "/posts/"+id, nil, 0, "")
		html := response.Body.String()
		if response.Code != 200 || response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("mask%d failed SSR %d", mask, response.Code)
		}
		if strings.Contains(html, `class="article-info"`) != (mask != 0) {
			t.Fatalf("mask%d unexpected info disclosure", mask)
		}
		for key, label := range map[string]string{"created": "创建时间", "updated": "最后编辑", "published": "发布时间"} {
			if strings.Contains(html, "<dt>"+label+"</dt>") != dates[key] {
				t.Fatalf("mask%d leaked or lost %s", mask, key)
			}
		}
		if mask != 0 {
			start := strings.Index(html, `<details class="article-info">`)
			if start < 0 {
				t.Fatalf("mask%d missing native disclosure", mask)
			}
			end := strings.Index(html[start:], "</details>") + start
			if start < 0 || end < start || !strings.Contains(html[start:end], `aria-label="文章时间信息"`) || !strings.Contains(html[start:end], `class="article-dates"`) {
				t.Fatalf("mask%d missing native labelled dates", mask)
			}
		}
		if strings.Count(html, `class="article-author"`) != 1 || strings.Contains(html, `class="article-cover"`) || !strings.Contains(html, `class="reading-paper"`) || !strings.Contains(html, "<hr") {
			t.Fatalf("mask%d broke reading structure: author=%d cover=%t paper=%t hr=%t", mask, strings.Count(html, `class="article-author"`), strings.Contains(html, `class="article-cover"`), strings.Contains(html, `class="reading-paper"`), strings.Contains(html, "<hr"))
		}
		h := strings.Index(html, `<h1>阅读时间合同</h1>`)
		summary := strings.Index(html, `class="article-summary"`)
		tags := strings.Index(html, `aria-label="文章标签"`)
		prose := strings.Index(html, `class="prose"`)
		if !(h >= 0 && h < summary && summary < tags && tags < prose) {
			t.Fatalf("mask%d incorrect introduction order", mask)
		}
	}
}
