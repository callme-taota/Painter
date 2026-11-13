package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestShareTokensAreStableCanonicalAndDomainSeparated(t *testing.T) {
	a := &App{Config: Config{EncryptionKey: bytes.Repeat([]byte{1}, 32)}}
	other := &App{Config: Config{EncryptionKey: bytes.Repeat([]byte{2}, 32)}}
	for _, kind := range []string{"post", "moment", "phrase", "series"} {
		for _, id := range []string{"1", "35", "9223372036854775807"} {
			ref := shareReference{Kind: kind, ID: id}
			token := a.shareToken(ref)
			if token == "" || token != a.shareToken(ref) {
				t.Fatalf("share identity is unstable: %s %s", kind, id)
			}
			parsed, err := a.parseShareToken(token)
			if err != nil || parsed != ref {
				t.Fatal("round trip", parsed, err)
			}
			if _, err := other.parseShareToken(token); err == nil {
				t.Fatal("token survived signing-key rotation")
			}
			for i := range token {
				changed := []byte(token)
				if changed[i] == 'a' {
					changed[i] = 'b'
				} else {
					changed[i] = 'a'
				}
				if _, err := a.parseShareToken(string(changed)); err == nil {
					t.Fatalf("modified token accepted at byte %d", i)
				}
			}
		}
	}
	for _, ref := range []shareReference{{"post", "01"}, {"post", "0"}, {"post", "-1"}, {"post", "https://evil.test"}, {"unknown", "1"}, {"post", "9223372036854775808"}} {
		if a.shareToken(ref) != "" {
			t.Fatal("noncanonical identity signed", ref)
		}
	}
	for _, payload := range []string{"1p01", "1p0", "1p-1", "1pZ", "1x1", "2p1"} {
		if _, err := a.parseShareToken(payload + "." + a.shareSignature(payload)); err == nil {
			t.Fatal("noncanonical signed payload accepted", payload)
		}
	}
}

func sharingTestApp(t *testing.T) *App {
	t.Helper()
	a := testApp(t)
	a.RegisterContent()
	if _, pattern := a.Mux.Handler(httptest.NewRequest("GET", "/api/v1/share", nil)); pattern == "" {
		a.RegisterSharing()
	}
	return a
}

func sharingPath(t *testing.T, v any) string {
	t.Helper()
	u, err := url.Parse(v.(string))
	if err != nil {
		t.Fatal(err)
	}
	return u.RequestURI()
}

func TestSharingPostPublicationMetadataIsolationAndWithdrawal(t *testing.T) {
	a := sharingTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	created := contentWant(t, contentCall(t, a, owner, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": `Published <img src=x onerror=alert(1)>`, "summary": "Public summary", "markdown": "BODY_NOT_IN_SHARE_DOCUMENT"}, 0, ""), 201)
	id := created["id"].(string)
	manifestPath := "/api/v1/share?type=post&id=" + id
	contentWant(t, contentCall(t, a, owner, csrf, "GET", manifestPath, nil, 0, ""), 404)
	contentWant(t, contentCall(t, a, owner, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	manifestResponse := contentCall(t, a, nil, "", "GET", manifestPath, nil, 0, "")
	manifest := contentWant(t, manifestResponse, 200)
	if !strings.Contains(manifestResponse.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("share manifest was cacheable")
	}
	if manifest["canonical_url"] != a.Config.PublicBaseURL+"/posts/"+id {
		t.Fatal("noncanonical share destination", manifest)
	}
	for _, snippet := range []string{`sandbox="allow-popups allow-popups-to-escape-sandbox"`, `referrerpolicy="no-referrer"`, `loading="lazy"`, `rel="noopener noreferrer"`, "&lt;img"} {
		if !strings.Contains(manifest["embed_html"].(string), snippet) {
			t.Fatal("unsafe or inaccessible embed HTML", snippet, manifest["embed_html"])
		}
	}
	contentWant(t, contentCall(t, a, owner, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", map[string]any{"title": "PRIVATE_DRAFT_TITLE", "summary": "PRIVATE_DRAFT_SUMMARY", "markdown": "PRIVATE_DRAFT_BODY", "expected_draft_version": 1}, 2, ""), 200)
	again := contentWant(t, contentCall(t, a, nil, "", "GET", manifestPath, nil, 0, ""), 200)
	if again["short_url"] != manifest["short_url"] || strings.Contains(again["title"].(string), "PRIVATE") {
		t.Fatal("private edit changed stable public share", again)
	}
	for _, key := range []string{"short_url", "share_url", "embed_url"} {
		w := contentCall(t, a, nil, "", "GET", sharingPath(t, manifest[key])+"?return_to=https://evil.test", nil, 0, "")
		if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
			t.Fatal("share response was cacheable", key)
		}
		if key == "short_url" {
			if w.Code != http.StatusFound || w.Header().Get("Location") != manifest["canonical_url"] {
				t.Fatal("share redirected to an untrusted destination", w.Code, w.Header())
			}
			continue
		}
		if w.Code != 200 || strings.Contains(w.Body.String(), "PRIVATE_DRAFT") || strings.Contains(w.Body.String(), "BODY_NOT_IN_SHARE_DOCUMENT") || strings.Contains(w.Body.String(), `<img src=x`) {
			t.Fatal("share document included private/body/unsafe markup", key, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "<script") || !strings.Contains(w.Body.String(), "Public summary") || !strings.Contains(w.Body.String(), "阅读完整文章") {
			t.Fatal("share document is not a minimal, script-free public preview", w.Body.String())
		}
		if key == "embed_url" {
			csp := w.Header().Get("Content-Security-Policy")
			if w.Header().Get("X-Frame-Options") != "" || !strings.Contains(csp, "frame-ancestors *") || !strings.Contains(csp, "script-src 'none'") || !strings.Contains(csp, "sandbox allow-popups allow-popups-to-escape-sandbox") {
				t.Fatal("embed has incorrect narrowly-scoped frame policy", w.Header())
			}
		} else if w.Header().Get("X-Frame-Options") != "DENY" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Fatal("share landing loosened the site's frame policy", w.Header())
		}
	}
	contentWant(t, contentCall(t, a, owner, csrf, "DELETE", "/api/v1/studio/posts/"+id+"/publication", nil, 3, ""), 200)
	contentWant(t, contentCall(t, a, owner, csrf, "GET", manifestPath, nil, 0, ""), 404)
	for _, key := range []string{"short_url", "share_url", "embed_url"} {
		w := contentCall(t, a, owner, csrf, "GET", sharingPath(t, manifest[key]), nil, 0, "")
		if w.Code != 404 || strings.Contains(w.Body.String(), "Public summary") || w.Header().Get("X-Frame-Options") != "DENY" {
			t.Fatal("withdrawn content or framing policy leaked", key, w.Code, w.Header())
		}
	}
}

func TestSharingMomentsPhrasesPrivateAndDeletedVisibility(t *testing.T) {
	a := sharingTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	moment := momentFixture(t, a, owner, csrf, "A public moment", true)
	privateMoment := momentFixture(t, a, owner, csrf, "PRIVATE_MOMENT", false)
	phrase := phraseCreateFixture(t, a, owner, csrf, "A public phrase", "A source", true)
	privatePhrase := phraseCreateFixture(t, a, owner, csrf, "PRIVATE_PHRASE", "Private source", false)
	for _, item := range []struct {
		kind string
		v    map[string]any
	}{{"moment", privateMoment}, {"phrase", privatePhrase}} {
		id := item.v["id"].(string)
		contentWant(t, contentCall(t, a, owner, csrf, "GET", "/api/v1/share?type="+item.kind+"&id="+id, nil, 0, ""), 404)
		// A valid signature is identity only, even if produced by trusted code.
		token := a.shareToken(shareReference{Kind: item.kind, ID: id})
		for _, prefix := range []string{"/s/", "/share/", "/embed/"} {
			contentWant(t, contentCall(t, a, owner, csrf, "GET", prefix+token, nil, 0, ""), 404)
		}
	}
	for _, item := range []struct {
		kind string
		v    map[string]any
		text string
	}{{"moment", moment, "A public moment"}, {"phrase", phrase, "A public phrase"}} {
		id := item.v["id"].(string)
		manifest := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/share?type="+item.kind+"&id="+id, nil, 0, ""), 200)
		for _, key := range []string{"share_url", "embed_url"} {
			w := contentCall(t, a, nil, "", "GET", sharingPath(t, manifest[key]), nil, 0, "")
			if w.Code != 200 || !strings.Contains(w.Body.String(), item.text) {
				t.Fatal("public share missing", item.kind, w.Code, w.Body.String())
			}
		}
		if item.kind == "moment" {
			contentWant(t, contentCall(t, a, owner, csrf, "DELETE", "/api/v1/studio/moments/"+id+"/publication", nil, 2, ""), 200)
		} else {
			if w := contentCall(t, a, owner, csrf, "DELETE", "/api/v1/admin/phrases/"+id, nil, 1, ""); w.Code != 204 {
				t.Fatal("phrase delete", w.Code, w.Body.String())
			}
		}
		for _, key := range []string{"short_url", "share_url", "embed_url"} {
			contentWant(t, contentCall(t, a, nil, "", "GET", sharingPath(t, manifest[key]), nil, 0, ""), 404)
		}
	}
	for _, path := range []string{"/api/v1/share?type=post&id=01", "/api/v1/share?type=post&id=1%20OR%201=1", "/api/v1/share?type=post&id=9223372036854775808", "/share/https:evil.test", "/embed/not-a-token", "/s/not-a-token"} {
		contentWant(t, contentCall(t, a, nil, "", "GET", path, nil, 0, ""), 404)
	}
	contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/share?type=https://evil.test&id=1", nil, 0, ""), 400)
}

func TestSharingSeriesResolverNeverGrantsVisibility(t *testing.T) {
	a := &App{Config: Config{PublicBaseURL: "https://painter.test"}}
	ref := shareReference{Kind: "series", ID: "1"}
	if _, err := a.publicShareDocument(context.Background(), ref, nil); err == nil {
		t.Fatal("series without public resolver was visible")
	}
	if _, err := a.publicShareDocument(context.Background(), ref, func(context.Context, string) (map[string]any, error) { return nil, NotFound() }); err == nil {
		t.Fatal("private series became visible")
	}
	doc, err := a.publicShareDocument(context.Background(), ref, func(context.Context, string) (map[string]any, error) {
		return map[string]any{"title": "A public series", "description": "An ordered collection", "site_name": "Painter", "author": map[string]any{"display_name": "Author"}, "cover_url": "https://tracker.test/pixel"}, nil
	})
	if err != nil || doc.Title != "A public series" || doc.Author != "Author" || doc.Canonical != "https://painter.test/series/1" || doc.CoverURL != "" {
		t.Fatal("series metadata reader contract", doc, err)
	}
}
