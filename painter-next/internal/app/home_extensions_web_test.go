package app

import (
	nethtml "golang.org/x/net/html"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomeExtensionsSSRTruthVisibilityAndEscaping(t *testing.T) {
	a, calls := homeSSRApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	for _, visible := range []bool{true, false} {
		title := `<img src=x onerror=alert(1)> open source`
		if !visible {
			title = "Hidden project must remain private"
		}
		momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/projects", map[string]any{"source": "manual", "repo_url": "", "title": title, "description": "<script>metadata</script>", "url": "https://example.com/project", "language": "Go", "position": 0, "visible": visible}, 0, RandID()), 201)
	}
	homeSSRSave(t, a, owner, csrf, func(c map[string]any) {
		for _, key := range []string{"activity", "projects"} {
			s := c["sections"].(map[string]any)[key].(map[string]any)
			s["enabled"] = true
			s["title"] = "<em>" + key + "</em>"
		}
	})
	doc := homeSSRDocument(t, a, nil)
	if len(friendWebClass(doc, "activity-day")) < 365 || len(friendWebClass(doc, "project-playing-card")) != 1 {
		t.Fatal("new sections did not use actual saved visibility")
	}
	text := friendWebText(doc)
	if !strings.Contains(text, "0 次发布") || strings.Contains(text, "Hidden project must remain private") || !strings.Contains(text, "按首次发布时间计数") {
		t.Fatal("activity semantics or private project leaked", text)
	}
	for _, n := range friendWebNodes(doc, func(n *nethtml.Node) bool {
		return n.Type == nethtml.ElementNode && (friendWebHasAttr(n, "onerror") || n.Data == "script" && strings.Contains(friendWebText(n), "metadata"))
	}) {
		t.Fatal("stored plain text injected HTML", n)
	}
	if calls.Load() != 0 {
		t.Fatal("homepage renderer called a provider")
	}
	game := homeTestInput("steam")
	game["provider_id"] = "413150"
	homeTestItem(t, a, owner, csrf, game)
	homeSSRSave(t, a, owner, csrf, func(c map[string]any) { c["sections"].(map[string]any)["steam"].(map[string]any)["enabled"] = true })
	w := httptest.NewRecorder()
	a.Mux.ServeHTTP(w, httptest.NewRequest("GET", "/games", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "413150/library_600x900.jpg") {
		t.Fatal("library needs actual portrait artwork", w.Code)
	}
}
