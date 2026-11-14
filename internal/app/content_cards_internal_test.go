package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func internalCardFixture(kind, id string) contentCard {
	return contentCard{V: 1, Type: kind, RefID: id, URL: contentCardInternalURL(kind, id)}
}
func TestInternalContentCardsPersistIdentityOnly(t *testing.T) {
	for _, kind := range []string{"moment", "phrase"} {
		c := internalCardFixture(kind, "42")
		raw, _ := json.Marshal(c)
		parsed, known, e := parseContentCard(raw)
		if e != nil || !known || parsed.RefID != "42" {
			t.Fatal(parsed, known, e)
		}
		rendered, e := RenderMarkdown(contentCardMarkdown(c))
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(rendered.HTML, `content-card--`+kind) || !strings.Contains(rendered.HTML, c.URL) || len(rendered.MediaIDs) != 0 || strings.Contains(rendered.HTML, "<img") {
			t.Fatal("unsafe or missing placeholder", rendered)
		}
		if _, e = RenderMarkdown(contentCardInlineMarkdown(c)); e == nil {
			t.Fatal("internal block became inline snapshot")
		}
	}
	for _, raw := range []string{
		`{"v":1,"type":"moment","ref_id":"01","url":"/moments/01"}`, `{"v":1,"type":"moment","ref_id":"9223372036854775808","url":"/moments/9223372036854775808"}`,
		`{"v":1,"type":"moment","ref_id":"1","url":"https://example.org/moments/1"}`, `{"v":1,"type":"phrase","ref_id":"1","url":"/moments/1"}`,
		`{"v":1,"type":"moment","ref_id":"1","url":"/moments/1","title":"hidden source text"}`, `{"v":1,"type":"phrase","ref_id":"1","url":"/phrases/1","title":null}`,
		`{"v":1,"type":"moment","ref_id":"1","url":"/moments/1","image_url":"/media/0123456789abcdef"}`, `{"v":1,"type":"phrase","ref_id":"1","url":"/phrases/1","description":"private description"}`,
		`{"v":1,"type":"moment","ref_id":"1","url":"/moments/1?scope=private"}`, `{"v":1,"type":"moment","ref_id":"1","url":"//evil.test/moments/1"}`,
	} {
		if _, e := RenderMarkdown("```painter-card\n" + raw + "\n```"); e == nil {
			t.Fatal("unsafe reference accepted", raw)
		}
	}
	unknown, e := RenderMarkdown("```painter-card\n{\"v\":2,\"type\":\"moment\",\"ref_id\":\"1\"}\n```")
	if e != nil || strings.Contains(unknown.HTML, "content-card--moment") || !strings.Contains(unknown.HTML, "<pre") {
		t.Fatal("future protocol became trusted", unknown, e)
	}
}

func TestInternalContentCardsResolveCurrentPublicOnly(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	_, owner, csrf := testIdentity(t, a, "owner")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	public := momentFixture(t, a, owner, csrf, "public moment visible", true)
	private := momentFixture(t, a, owner, csrf, "private moment secret", false)
	phrase := phraseCreateFixture(t, a, owner, csrf, "public phrase visible", "public attribution", true)
	privatePhrase := phraseCreateFixture(t, a, owner, csrf, "private phrase secret", "private source", false)
	var requests atomic.Int32
	homeFake(a, func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		t.Error("internal card contacted a provider")
		return homeHTTP(500, ""), nil
	})
	for _, v := range []struct {
		kind, id string
		status   int
	}{{"moment", public["id"].(string), 200}, {"phrase", phrase["id"].(string), 200}, {"moment", private["id"].(string), 404}, {"phrase", privatePhrase["id"].(string), 404}, {"moment", "9999999", 404}} {
		input := contentCardResolveInput{Type: v.kind, RefID: v.id}
		result := contentCall(t, a, owner, csrf, "POST", "/api/v1/editor/cards/resolve", input, 0, "")
		if result.Code != v.status {
			t.Fatal(v, result.Code, result.Body.String())
		}
		if v.status == 200 {
			response := jsonMap(t, result)
			c := response["card"].(map[string]any)
			if len(c) != 4 || c["title"] != nil || c["image_url"] != nil || !strings.Contains(response["markdown"].(string), "ref_id") {
				t.Fatal("reference persisted content snapshot", response)
			}
			if response["reference"] == nil {
				t.Fatal("current public preview absent")
			}
		} else if strings.Contains(result.Body.String(), "secret") || strings.Contains(result.Body.String(), "private source") {
			t.Fatal("private response leaked", result.Body.String())
		}
	}
	for _, v := range []struct {
		kind, id string
		status   int
	}{{"moment", public["id"].(string), 200}, {"phrase", phrase["id"].(string), 200}, {"moment", private["id"].(string), 404}, {"phrase", privatePhrase["id"].(string), 404}} {
		response := contentCall(t, a, nil, "", "GET", "/api/v1/content-cards/"+v.kind+"/"+v.id, nil, 0, "")
		if response.Code != v.status {
			t.Fatal("public card endpoint", v, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Header().Get("Cache-Control"), "no-store") {
			t.Fatal("public card could be cached", response.Header())
		}
		if strings.Contains(response.Body.String(), "secret") {
			t.Fatal("public card leaked draft", response.Body.String())
		}
	}
	contentWant(t, contentCall(t, a, reader, readerCSRF, "POST", "/api/v1/editor/cards/resolve", contentCardResolveInput{Type: "moment", RefID: public["id"].(string)}, 0, ""), 403)
	if requests.Load() != 0 {
		t.Fatal("unexpected network", requests.Load())
	}
}

func TestInternalContentCardsWithdrawHidePublishedTextAndPhotos(t *testing.T) {
	a := momentImageApp(t)
	a.RegisterContent()
	a.RegisterSearch()
	_, owner, csrf := testIdentity(t, a, "owner")
	media := momentImageUpload(t, a, owner, csrf, 76)
	item := momentImageFixture(t, a, owner, csrf, "<script>public moment literal</script>", []momentImage{{MediaID: media, Alt: "original image"}})
	momentID := item["id"].(string)
	base := "/api/v1/studio/moments/" + momentID
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", base+"/publish", nil, 1, RandID()), 200)
	phrase := phraseCreateFixture(t, a, owner, csrf, "<img src=x> public phrase", "source <script>literal</script>", true)
	phraseID := phrase["id"].(string)
	markdown := contentCardMarkdown(internalCardFixture("moment", momentID)) + "\n\n" + contentCardMarkdown(internalCardFixture("phrase", phraseID))
	rendered, e := RenderMarkdown(markdown)
	if e != nil {
		t.Fatal(e)
	}
	if len(rendered.MediaIDs) != 0 {
		t.Fatal("dynamic image became a persistent article media reference")
	}
	expanded, e := a.ExpandInternalContentCards(context.Background(), rendered.HTML)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(expanded, "&lt;script&gt;public moment") || !strings.Contains(expanded, "/media/"+media) || !strings.Contains(expanded, "public phrase") || strings.Contains(expanded, "<script>") {
		t.Fatal("unsafe public projection", expanded)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", base+"/draft", map[string]any{"text": "unpublished private replacement", "images": []momentImage{}}, 2, RandID()), 200)
	unchanged, e := a.ExpandInternalContentCards(context.Background(), rendered.HTML)
	if e != nil || strings.Contains(unchanged, "private replacement") || !strings.Contains(unchanged, media) {
		t.Fatal("draft leaked into card", unchanged, e)
	}
	d := map[string]any{"client_draft_id": RandID(), "title": "站内引用文章", "markdown": markdown}
	created := contentWant(t, contentCall(t, a, owner, csrf, "POST", "/api/v1/posts", d, 0, ""), 201)
	post := created["id"].(string)
	published := contentWant(t, contentCall(t, a, owner, csrf, "POST", "/api/v1/studio/posts/"+post+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", base+"/publication", nil, 3, RandID()), 200)
	contentWant(t, phraseCall(t, a, owner, csrf, "PUT", "/api/v1/admin/phrases/"+phraseID, map[string]any{"text": "new private phrase", "kind": "sentence", "source": "private source", "published": false}, 1, RandID()), 200)
	after, e := a.ExpandInternalContentCards(context.Background(), expanded)
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{"public moment literal", "public phrase", media, "private replacement", "new private phrase", "private source"} {
		if strings.Contains(after, secret) {
			t.Fatal("withdrawn source remained in expanded old HTML", secret, after)
		}
	}
	if strings.Count(after, "暂不可用") != 2 {
		t.Fatal(after)
	}
	publicPost := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+post, nil, 0, ""), 200)
	html := publicPost["sanitized_html"].(string)
	if !strings.Contains(html, "暂不可用") || strings.Contains(html, media) || strings.Contains(html, "public phrase") {
		t.Fatal("public article boundary failed current visibility", html)
	}
	history := contentWant(t, contentCall(t, a, owner, csrf, "GET", "/api/v1/studio/posts/"+post+"/revisions/"+published["revision_id"].(string), nil, 0, ""), 200)
	preview := contentWant(t, contentCall(t, a, owner, csrf, "POST", "/api/v1/editor/preview", map[string]any{"markdown": markdown, "seq": 1}, 0, ""), 200)
	for _, value := range []map[string]any{history, preview} {
		html, _ := value["sanitized_html"].(string)
		if strings.Count(html, "暂不可用") != 2 || strings.Contains(html, media) || strings.Contains(html, "public phrase") || strings.Contains(html, "private replacement") {
			t.Fatal("historical/preview boundary retained withdrawn source", value)
		}
	}
	momentImageRead(t, a, nil, media, 404)
	search := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/search?q=public%20phrase", nil, 0, ""), 200)
	if len(search["items"].([]any)) != 0 {
		t.Fatal("volatile/private source text entered article search projection", search)
	}
}

func TestInternalPhraseDirectSSRUsesCurrentVisibility(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterWeb()
	_, owner, csrf := testIdentity(t, a, "owner")
	phrase := phraseCreateFixture(t, a, owner, csrf, "public direct phrase", "actual attribution", true)
	id := phrase["id"].(string)
	response := contentCall(t, a, nil, "", "GET", "/phrases/"+id, nil, 0, "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), "public direct phrase") || !strings.Contains(response.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("direct phrase SSR unavailable or cacheable", response.Code, response.Body.String())
	}
	contentWant(t, phraseCall(t, a, owner, csrf, "PUT", "/api/v1/admin/phrases/"+id, map[string]any{"text": "private direct phrase", "kind": "sentence", "published": false}, 1, RandID()), 200)
	response = contentCall(t, a, nil, "", "GET", "/phrases/"+id, nil, 0, "")
	if response.Code != 404 || strings.Contains(response.Body.String(), "direct phrase") {
		t.Fatal("withdrawn phrase detail leaked", response.Code, response.Body.String())
	}
}

func TestInternalContentCardsPreviewSingleConnection(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	_, owner, csrf := testIdentity(t, a, "owner")
	moment := momentFixture(t, a, owner, csrf, "single connection public moment", true)
	phrase := phraseCreateFixture(t, a, owner, csrf, "single connection public phrase", "public source", true)
	markdown := contentCardMarkdown(internalCardFixture("moment", moment["id"].(string))) + "\n\n" + contentCardMarkdown(internalCardFixture("phrase", phrase["id"].(string)))

	// Reopen this test's isolated database with exactly one connection. Preview's
	// Write transaction occupies it, so a lookup through a.Pool would wait until
	// the HTTP deadline instead of reusing that transaction's connection.
	config := a.Pool.Config()
	config.MaxConns, config.MinConns = 1, 0
	pool, e := pgxpool.NewWithConfig(context.Background(), config)
	if e != nil {
		t.Fatal(e)
	}
	previous := a.Pool
	a.Pool = pool
	previous.Close()
	if a.Pool.Config().MaxConns != 1 {
		t.Fatal("single connection pool not configured")
	}

	server := httptest.NewServer(a.Handler())
	defer server.Close()
	body, _ := json.Marshal(map[string]any{"markdown": markdown, "seq": 7})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r, e := http.NewRequestWithContext(ctx, "POST", server.URL+"/api/v1/editor/preview", bytes.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", a.Config.PublicBaseURL)
	r.Header.Set("X-CSRF-Token", csrf)
	r.AddCookie(owner)
	response, e := server.Client().Do(r)
	if e != nil {
		t.Fatalf("preview must finish while its transaction owns the only connection: %v", e)
	}
	defer response.Body.Close()
	raw, e := io.ReadAll(response.Body)
	if e != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("preview response %d: %s (%v)", response.StatusCode, raw, e)
	}
	var result struct {
		HTML string `json:"sanitized_html"`
		Seq  int64  `json:"seq"`
	}
	if e = json.Unmarshal(raw, &result); e != nil {
		t.Fatal(e)
	}
	if result.Seq != 7 || !strings.Contains(result.HTML, "single connection public moment") || !strings.Contains(result.HTML, "single connection public phrase") || strings.Count(result.HTML, `data-internal-card-state="ready"`) != 2 {
		t.Fatal("preview did not expand both public references", string(raw))
	}
}
