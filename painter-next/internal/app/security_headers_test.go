package app

import (
	"context"
	"encoding/base64"
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

func securityPolicyDirectives(policy string) map[string]string {
	directives := map[string]string{}
	for _, part := range strings.Split(policy, ";") {
		name, value, _ := strings.Cut(strings.TrimSpace(part), " ")
		directives[name] = value
	}
	return directives
}

func TestSecurityHeadersUseFreshServerNonce(t *testing.T) {
	a := &App{}
	original := httptest.NewRequest("GET", "https://blog.example/posts", nil)
	original.Header.Set("X-Script-Nonce", "client-controlled")
	seen := map[string]bool{}
	for range 16 {
		w := httptest.NewRecorder()
		r := a.securityHeaders(w, original)
		nonce := requestScriptNonce(r.Context())
		decoded, err := base64.RawStdEncoding.DecodeString(nonce)
		if err != nil || len(decoded) != 24 {
			t.Fatalf("want a 192-bit random nonce, got %q (%v)", nonce, err)
		}
		if seen[nonce] {
			t.Fatal("nonce reused across responses")
		}
		seen[nonce] = true
		if requestScriptNonce(original.Context()) != "" {
			t.Fatal("middleware mutated the original request context")
		}
		scriptPolicy := securityPolicyDirectives(w.Header().Get("Content-Security-Policy"))["script-src"]
		if scriptPolicy != "'self' 'nonce-"+nonce+"'" {
			t.Fatalf("script policy and template nonce differ: %q", scriptPolicy)
		}
	}
	if nonce := requestScriptNonce(context.Background()); nonce != "" {
		t.Fatalf("missing middleware should not invent a nonce: %q", nonce)
	}
}

func TestSecurityHeadersRestrictActiveContentAndKeepRequiredResources(t *testing.T) {
	w := httptest.NewRecorder()
	(&App{}).securityHeaders(w, httptest.NewRequest("GET", "https://blog.example/", nil))
	directives := securityPolicyDirectives(w.Header().Get("Content-Security-Policy"))
	for directive, want := range map[string]string{
		"default-src":     "'self'",
		"script-src-attr": "'none'",
		"style-src":       "'self' 'unsafe-inline'",
		"img-src":         "'self' https: data: blob:",
		"media-src":       "'self' https: blob:",
		"font-src":        "'self' data:",
		"connect-src":     "'self'",
		"worker-src":      "'self' blob:",
		"frame-src":       "'none'",
		"base-uri":        "'none'",
		"object-src":      "'none'",
		"frame-ancestors": "'none'",
		"form-action":     "'self'",
	} {
		if got := directives[directive]; got != want {
			t.Errorf("%s: want %q, got %q", directive, want, got)
		}
	}
	for _, unsafe := range []string{"'unsafe-inline'", "'unsafe-eval'", "data:", "https:"} {
		if strings.Contains(directives["script-src"], unsafe) {
			t.Errorf("script execution unexpectedly permits %s", unsafe)
		}
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("pages can be framed")
	}
	if got := w.Header().Get("Permissions-Policy"); got != "camera=(), microphone=(), geolocation=(), payment=(), usb=()" {
		t.Errorf("unexpected permissions policy: %q", got)
	}
}

func TestSecurityHeadersEmbeddedTemplatesAuthorizeEveryInlineScript(t *testing.T) {
	templates := v31NavigationTemplates(t)
	for _, kind := range []string{"home", "article", "login", "studio", "setup", "error"} {
		t.Run(kind, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := (&App{}).securityHeaders(w, httptest.NewRequest("GET", "https://blog.example/", nil))
			page := v31NavigationPage(kind, defaultFloatingControls(), false, true)
			page.ScriptNonce = requestScriptNonce(r.Context())
			page.JSONLD = template.JS(`{"@context":"https://schema.org","@type":"BlogPosting","headline":"Nonce contract"}`)
			page.JS = "/assets/public-test.js"
			doc := v31NavigationDocument(t, templates, page)
			inline := friendWebNodes(doc, func(node *nethtml.Node) bool {
				return node.Type == nethtml.ElementNode && node.Data == "script" && friendWebAttr(node, "src") == ""
			})
			if len(inline) != 2 {
				t.Fatalf("want theme bootstrap and JSON-LD scripts, got %d", len(inline))
			}
			for _, script := range inline {
				if got := friendWebAttr(script, "nonce"); got != page.ScriptNonce {
					t.Errorf("inline %q script has nonce %q, want response nonce", friendWebAttr(script, "type"), got)
				}
			}
			if !strings.Contains(friendWebText(inline[0]), "painter-theme") {
				t.Error("theme bootstrap missing")
			}
			if friendWebAttr(inline[1], "type") != "application/ld+json" || !strings.Contains(friendWebText(inline[1]), "Nonce contract") {
				t.Error("article structured data missing")
			}
		})
	}
}
