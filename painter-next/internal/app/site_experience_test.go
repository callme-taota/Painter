package app

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestSuggestionPublicationAndLiteralMatching(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterAdmin()
	a.RegisterSearch()
	_, cookie, csrf := testIdentity(t, a, "owner")
	publicTag := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/admin/tags", map[string]any{"name": "ＤＥＳＩＧＮ"}, 0, ""), 201)["id"].(string)
	privateTag := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/admin/tags", map[string]any{"name": "OnlyDraft"}, 0, ""), 201)["id"].(string)
	create := func(title string, tag string, publish bool) string {
		id := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": title, "markdown": "正文", "tag_ids": []string{tag}}, 0, ""), 201)["id"].(string)
		if publish {
			contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
		}
		return id
	}
	id := create("中文浮层搜索 %_!", publicTag, true)
	create("另一篇公开文章", publicTag, true)
	create("机密草稿独有词", privateTag, false)
	suggest := func(q string) []any {
		result := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/search/suggestions?q="+url.QueryEscape(q), nil, 0, ""), 200)
		return result["items"].([]any)
	}
	if got := suggest("%_"); len(got) != 1 || got[0].(map[string]any)["url"] != "/posts/"+id {
		t.Fatal("suggestions treated wildcard characters as patterns", got)
	}
	if got := suggest("中文"); len(got) != 1 {
		t.Fatal("short Chinese title suggestion missing", got)
	}
	if got := suggest("design"); len(got) != 1 || got[0].(map[string]any)["kind"] != "tag" {
		t.Fatal("NFKC/case-fold tag suggestion missing", got)
	}
	for _, q := range []string{"机密", "OnlyDraft"} {
		if got := suggest(q); len(got) != 0 {
			t.Fatal("private data leaked through suggestions", q, got)
		}
	}
	if got := suggest(""); len(got) != 3 {
		t.Fatal("empty query did not return two public articles and their real tag", got)
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/studio/posts/"+id+"/publication", nil, 2, ""), 200)
	if got := suggest("%_"); len(got) != 0 {
		t.Fatal("withdrawn title remained suggested", got)
	}
	response := contentCall(t, a, nil, "", "GET", "/api/v1/search/suggestions?q="+strings.Repeat("x", 65), nil, 0, "")
	if response.Code != 400 {
		t.Fatal("overlong query was accepted", response.Code)
	}
}

func TestSiteExperienceSettingsRSSAndCompatibility(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterAdmin()
	a.RegisterWeb()
	_, cookie, csrf := testIdentity(t, a, "owner")
	_, readerCookie, readerCSRF := testIdentity(t, a, "reader")
	if got := testRequest(a, "GET", "/rss.xml", "", nil, ""); got.Code != 404 {
		t.Fatal("RSS should initially be disabled", got.Code)
	}
	settings := contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/admin/settings", nil, 0, ""), 200)
	settings["rss_enabled"] = true
	settings["floating_controls"] = map[string]any{"home": []string{}, "article": []string{"theme", "bottom", "like", "account"}, "other": []string{"account"}}
	saved := contentWant(t, contentCall(t, a, cookie, csrf, "PUT", "/api/v1/admin/settings", settings, 1, ""), 200)
	if saved["rss_enabled"] != true || len(saved["floating_controls"].(map[string]any)["home"].([]any)) != 0 {
		t.Fatal("experience settings not saved", saved)
	}
	if got := testRequest(a, "GET", "/rss.xml", "", nil, ""); got.Code != 200 || !strings.Contains(got.Body.String(), "<rss") {
		t.Fatal("enabled RSS missing", got.Code, got.Body.String())
	}
	public := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/site", nil, 0, ""), 200)
	if public["rss_enabled"] != true || public["floating_controls"] == nil {
		t.Fatal("public control configuration missing", public)
	}
	for _, invalid := range []any{
		map[string]any{"home": []string{"like"}},
		map[string]any{"article": []string{"theme", "theme"}},
		map[string]any{"article": []string{"script"}},
		map[string]any{"unknown": []string{}},
		map[string]any{"home": nil},
	} {
		bad := map[string]any{}
		for key, value := range saved {
			bad[key] = value
		}
		bad["floating_controls"] = invalid
		if got := contentCall(t, a, cookie, csrf, "PUT", "/api/v1/admin/settings", bad, 2, ""); got.Code != 400 {
			t.Fatal("invalid controls accepted", invalid, got.Code, got.Body.String())
		}
	}
	// An old settings client must not accidentally turn off a newly configured
	// feature when it saves unrelated preferences without the new fields.
	delete(saved, "rss_enabled")
	delete(saved, "floating_controls")
	compatible := contentWant(t, contentCall(t, a, cookie, csrf, "PUT", "/api/v1/admin/settings", saved, 2, ""), 200)
	if compatible["rss_enabled"] != true || len(compatible["floating_controls"].(map[string]any)["home"].([]any)) != 0 {
		t.Fatal("older settings request reset experience configuration", compatible)
	}
	if got := contentCall(t, a, readerCookie, readerCSRF, "PUT", "/api/v1/admin/settings", compatible, 3, ""); got.Code != 403 {
		t.Fatal("reader changed settings", got.Code)
	}
	compatible["rss_enabled"] = false
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", "/api/v1/admin/settings", compatible, 3, ""), 200)
	if got := testRequest(a, "GET", "/rss.xml", "", nil, ""); got.Code != 404 {
		t.Fatal("disabled RSS is still available", got.Code)
	}
}

func TestFloatingControlValidation(t *testing.T) {
	if _, e := normalizeFloatingControls(json.RawMessage(`{"home":[]}`)); e != nil {
		t.Fatal(e)
	}
	for _, value := range []string{`null`, `[]`, `{"home":null}`, `{"home":["like"]}`, `{"article":["theme","theme"]}`, `{"other":["<script>"]}`} {
		if _, e := normalizeFloatingControls(json.RawMessage(value)); e == nil {
			t.Fatal("invalid control configuration accepted", value)
		}
	}
}
