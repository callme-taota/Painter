package app

import (
	"context"
	"encoding/xml"
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

func momentWebApp(t *testing.T) *App {
	t.Helper()
	a := momentTestApp(t)
	a.RegisterAdmin()
	a.RegisterContent()
	a.RegisterWeb()
	return a
}

func TestMomentsWebPlainTextSnapshotAndVisibility(t *testing.T) {
	a := momentWebApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	text := `<script data-moment-xss>alert("no")</script>` + "\n<img src=x onerror=alert('no')> & 纯文本动态"
	item := momentFixture(t, a, owner, csrf, text, true)
	id := item["id"].(string)
	base := "/api/v1/studio/moments/" + id
	momentFixture(t, a, owner, csrf, "SSR-moment-private-only", false)
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", base+"/draft", map[string]any{"text": "SSR-moment-unpublished-edit"}, 2, RandID()), 200)
	for _, cookie := range []*http.Cookie{nil, owner} {
		for _, path := range []string{"/moments?scope=site&status=draft", "/moments/" + id} {
			response := testRequest(a, "GET", path, "", cookie, "")
			body := response.Body.String()
			if response.Code != 200 || !strings.Contains(response.Header().Get("Content-Type"), "text/html") || response.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("public moments SSR response", path, response.Code, response.Header())
			}
			if strings.Contains(body, "SSR-moment-private-only") || strings.Contains(body, "SSR-moment-unpublished-edit") || !strings.Contains(body, html.EscapeString(text)) {
				t.Fatal("SSR public snapshot missing/unescaped or private edit leaked", path)
			}
			doc := friendWebDocument(t, body)
			if injected := friendWebNodes(doc, func(node *nethtml.Node) bool {
				return node.Type == nethtml.ElementNode && (friendWebHasAttr(node, "data-moment-xss") || friendWebHasAttr(node, "onerror"))
			}); len(injected) != 0 {
				t.Fatal("plain moment injected executable markup", path)
			}
			if !strings.Contains(friendWebText(doc), text) {
				t.Fatal("no-JavaScript document lacks the original text", path)
			}
			canonical := friendWebNodes(doc, func(node *nethtml.Node) bool {
				return node.Type == nethtml.ElementNode && node.Data == "link" && friendWebAttr(node, "rel") == "canonical"
			})
			wantCanonical := a.Config.PublicBaseURL + "/moments"
			if strings.HasPrefix(path, "/moments/") {
				wantCanonical += "/" + id
			}
			if len(canonical) != 1 || friendWebAttr(canonical[0], "href") != wantCanonical {
				t.Fatal("moment canonical escaped/missing", path)
			}
		}
	}
	privateID := momentItems(t, a, owner, "/api/v1/me/moments?status=draft")[0].(map[string]any)["id"].(string)
	if w := testRequest(a, "GET", "/moments/"+privateID, "", owner, ""); w.Code != 404 || strings.Contains(w.Body.String(), "SSR-moment-private-only") {
		t.Fatal("owner cookie made a draft detail public", w.Code)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", base+"/publication", nil, 3, RandID()), 200)
	if w := testRequest(a, "GET", "/moments/"+id, "", owner, ""); w.Code != 404 {
		t.Fatal("withdrawn SSR detail remained public", w.Code)
	}
	if w := testRequest(a, "GET", "/moments", "", nil, ""); w.Code != 200 || strings.Contains(w.Body.String(), "data-moment-xss") || strings.Contains(w.Body.String(), "SSR-moment-") {
		t.Fatal("withdrawn/private moment remained in SSR feed", w.Code)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", base+"/publish", nil, 4, RandID()), 200)
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", base, nil, 5, RandID()), 200)
	if w := testRequest(a, "GET", "/moments/"+id, "", nil, ""); w.Code != 404 || strings.Contains(w.Body.String(), "SSR-moment-unpublished-edit") {
		t.Fatal("soft-deleted SSR detail remained public", w.Code)
	}
}

func TestMomentsWebStudioRoleBoundary(t *testing.T) {
	a := momentWebApp(t)
	_, owner, _ := testIdentity(t, a, "owner")
	_, author, _ := testIdentity(t, a, "author")
	_, reader, _ := testIdentity(t, a, "reader")
	for _, path := range []string{"/studio/moments", "/studio/moments/"} {
		for _, actor := range []struct {
			name   string
			cookie *http.Cookie
			status int
		}{{"anonymous", nil, 303}, {"reader", reader, 403}, {"author", author, 200}, {"owner", owner, 200}} {
			response := testRequest(a, "GET", path, "", actor.cookie, "")
			if response.Code != actor.status {
				t.Fatal("studio moments role boundary", path, actor.name, response.Code)
			}
			if actor.cookie == nil {
				target, err := url.Parse(response.Header().Get("Location"))
				if err != nil || target.Path != "/login" || target.Query().Get("return_to") != path {
					t.Fatal("studio anonymous redirect lost return path", response.Header().Get("Location"), err)
				}
			}
			if response.Code == 200 && (!strings.Contains(response.Body.String(), `id="workbench"`) || !strings.Contains(response.Body.String(), `content="noindex,nofollow"`)) {
				t.Fatal("writer did not receive private workbench shell", actor.name)
			}
		}
	}
}

func TestMomentsWebFloatingCreateRequiresRoleAndSetting(t *testing.T) {
	a := momentWebApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	_, author, _ := testIdentity(t, a, "author")
	_, reader, _ := testIdentity(t, a, "reader")
	item := momentFixture(t, a, owner, csrf, "public-floating-control-context", true)
	post := contentWant(t, contentCall(t, a, owner, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "创作控件文章场景", "markdown": "文章场景正文"}, 0, ""), 201)
	postID := post["id"].(string)
	contentWant(t, contentCall(t, a, owner, csrf, "POST", "/api/v1/studio/posts/"+postID+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	for _, enabled := range []bool{true, false} {
		settings := jsonMap(t, testRequest(a, "GET", "/api/v1/admin/settings", "", owner, ""))
		controls := []string{"theme", "top", "account"}
		if enabled {
			controls = append(controls, "create")
		}
		settings["floating_controls"] = map[string]any{"home": controls, "article": controls, "other": controls}
		momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/settings", settings, int64(settings["version"].(float64)), RandID()), 200)
		for _, actor := range []struct {
			name   string
			cookie *http.Cookie
			writer bool
		}{{"anonymous", nil, false}, {"reader", reader, false}, {"author", author, true}, {"owner", owner, true}} {
			for _, path := range []string{"/", "/about", "/moments", "/moments/" + item["id"].(string), "/posts/" + postID} {
				response := testRequest(a, "GET", path, "", actor.cookie, "")
				if response.Code != 200 {
					t.Fatal("floating create fixture page failed", path, response.Code)
				}
				doc := friendWebDocument(t, response.Body.String())
				create := friendWebClass(doc, "floating-create")
				want := 0
				if enabled && actor.writer {
					want = 1
				}
				if len(create) != want {
					t.Fatal("creation entry role/config leaked", path, actor.name, enabled, len(create), want)
				}
				if want == 1 {
					if create[0].Data != "details" {
						t.Fatal("creation entry lacks native disclosure")
					}
					links := friendWebNodes(create[0], func(node *nethtml.Node) bool { return node.Type == nethtml.ElementNode && node.Data == "a" })
					targets := map[string]bool{}
					momentCompose := false
					for _, link := range links {
						target, err := url.Parse(friendWebAttr(link, "href"))
						if err != nil {
							t.Fatal("invalid creation link", err)
						}
						targets[target.Path] = true
						if target.Path == "/studio/moments" && target.Query().Get("compose") == "1" && target.Fragment == "moment-compose" {
							momentCompose = true
						}
					}
					if !targets["/studio/posts/new"] || !momentCompose {
						t.Fatal("creation choices lack native article/moment routes", targets)
					}
				}
			}
		}
	}
}

func TestMomentsWebSitemapOnlyCurrentPublicRows(t *testing.T) {
	a := momentWebApp(t)
	p, owner, csrf := testIdentity(t, a, "owner")
	ctx := context.Background()
	if _, err := a.Pool.Exec(ctx, `INSERT INTO moments(author_user_id,draft_text,public_text,first_published_at_ms,published_at_ms,created_at_ms,updated_at_ms)
 SELECT $1,'sitemap-private-draft-'||n,'sitemap-public-'||n,100,100,1,100 FROM generate_series(1,57) n`, p.ID); err != nil {
		t.Fatal(err)
	}
	private := momentFixture(t, a, owner, csrf, "sitemap-private-only", false)
	withdrawn := momentFixture(t, a, owner, csrf, "sitemap-withdrawn", true)
	deleted := momentFixture(t, a, owner, csrf, "sitemap-deleted", true)
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", "/api/v1/studio/moments/"+withdrawn["id"].(string)+"/publication", nil, 2, RandID()), 200)
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", "/api/v1/studio/moments/"+deleted["id"].(string), nil, 2, RandID()), 200)
	response := testRequest(a, "GET", "/sitemap.xml", "", owner, "")
	var sitemap struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(response.Body.Bytes(), &sitemap); err != nil || response.Code != 200 || !strings.Contains(response.Header().Get("Content-Type"), "application/xml") {
		t.Fatal("moments sitemap failed", response.Code, err)
	}
	seen, details := map[string]int{}, 0
	for _, entry := range sitemap.URLs {
		seen[entry.Loc]++
		if strings.HasPrefix(entry.Loc, a.Config.PublicBaseURL+"/moments/") {
			details++
		}
	}
	if seen[a.Config.PublicBaseURL+"/moments"] != 1 || details != 57 {
		t.Fatal("sitemap lost a keyset page/collection entry", seen[a.Config.PublicBaseURL+"/moments"], details)
	}
	for path, count := range seen {
		if count != 1 {
			t.Fatal("sitemap duplicated a public URL", path, count)
		}
	}
	for _, item := range []map[string]any{private, withdrawn, deleted} {
		if seen[a.Config.PublicBaseURL+"/moments/"+item["id"].(string)] != 0 {
			t.Fatal("private/withdrawn/deleted moment in sitemap", item["id"])
		}
	}
}
