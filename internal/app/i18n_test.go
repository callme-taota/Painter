package app

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRequestLocalePrecedenceAndNegotiation(t *testing.T) {
	for _, tc := range []struct{ name, query, cookie, accept, want string }{
		{"default", "", "", "", defaultLocale},
		{"explicit wins", "en", "zh-CN", "zh-CN", "en"},
		{"cookie wins", "", "en", "zh-CN;q=1,en;q=0.1", "en"},
		{"unsupported explicit ignored", "de", "en", "zh-CN", "en"},
		{"browser weighted", "", "", "zh-CN;q=0.3,en-GB;q=0.9", "en"},
		{"English regional browser", "", "", "fr-FR,en-AU;q=0.7", "en"},
		{"Chinese regional browser", "", "", "zh-TW,en;q=0.8", defaultLocale},
		{"unacceptable English", "", "", "en;q=0,zh-CN;q=0.5", defaultLocale},
		{"unsupported browser", "", "", "de-DE,fr;q=0.8", defaultLocale},
		{"preferred wildcard uses site default", "", "", "*;q=1,en;q=0.5", defaultLocale},
		{"invalid cookie ignored", "", "<script>", "en", "en"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/posts?lang="+url.QueryEscape(tc.query), nil)
			if tc.cookie != "" {
				r.AddCookie(&http.Cookie{Name: localeCookie, Value: tc.cookie})
			}
			r.Header.Set("Accept-Language", tc.accept)
			if got := requestLocale(r); got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestLocaleSwitchNeverRedirectsOutsideSite(t *testing.T) {
	a := &App{Mux: http.NewServeMux()}
	a.registerLocale()
	for _, next := range []string{"https://evil.invalid/a", "//evil.invalid/a", "/%2f/evil.invalid", "/\\evil.invalid", "/%5cevil.invalid", "/%0aLocation:evil.invalid", "javascript:alert(1)", "/locale?lang=en", "/\x00bad"} {
		r := httptest.NewRequest(http.MethodGet, "/locale?lang=en&next="+url.QueryEscape(next), nil)
		w := httptest.NewRecorder()
		a.Mux.ServeHTTP(w, r)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
			t.Errorf("unsafe next %q: status %d location %q", next, w.Code, w.Header().Get("Location"))
		}
	}
	next := "/search?q=%E6%B5%8B%E8%AF%95&tag_id=2&lang=zh-CN#results"
	r := httptest.NewRequest(http.MethodGet, "/locale?lang=en&next="+url.QueryEscape(next), nil)
	w := httptest.NewRecorder()
	a.Mux.ServeHTTP(w, r)
	location, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != 303 || location.Path != "/search" || location.Query().Get("q") != "测试" || location.Query().Get("lang") != "" || location.Fragment != "results" {
		t.Fatal("lost safe return context", w.Header())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != localeCookie || cookies[0].Value != "en" || !cookies[0].HttpOnly || cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Path != "/" {
		t.Fatal("invalid LAN preference cookie", cookies)
	}
	if w.Header().Get("Content-Language") != "en" || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("language response headers", w.Header())
	}
	r = httptest.NewRequest(http.MethodGet, "https://example.test/locale?lang=zh-CN", nil)
	w = httptest.NewRecorder()
	a.Mux.ServeHTTP(w, r)
	if !w.Result().Cookies()[0].Secure {
		t.Fatal("HTTPS language cookie must be secure")
	}
	w = httptest.NewRecorder()
	a.Mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/locale?lang=unsupported", nil))
	if w.Code != 400 || len(w.Result().Cookies()) != 0 {
		t.Fatal("unsupported locale must not be persisted")
	}
}

func TestLocalizedUIFallbackPlaceholdersAndEscaping(t *testing.T) {
	if got := translateUI("en", "约 {count} 分钟阅读", "count", 12); got != "About 12 min read" {
		t.Fatal(got)
	}
	if got := translateUI("en", "还没有翻译的消息 {name}", "name", "Painter"); got != "还没有翻译的消息 Painter" {
		t.Fatal("fallback", got)
	}
	if got := translateUI(defaultLocale, "约 {count} 分钟阅读", "count", 12); got != "约 12 分钟阅读" {
		t.Fatal(got)
	}
	source := `<p>{{t "关于 {name}" "name" .}}</p>`
	view := template.Must(template.New("test").Funcs(localeTemplateFuncs("en")).Parse(source))
	var b bytes.Buffer
	if err := view.Execute(&b, `<script>alert(1)</script>`); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "<script>") || !strings.Contains(b.String(), "&lt;script&gt;") {
		t.Fatal("message values must remain escaped", b.String())
	}
}

func TestLocalizedDateAndGameHoursPreserveTimezone(t *testing.T) {
	if got := localizedDate("en", "2026-11-14T20:05:00Z", true); got != "Nov 15, 2026 04:05" {
		t.Fatal(got)
	}
	if got := localizedDate(defaultLocale, "2026-11-14T20:05:00Z", false); got != "2026年11月15日" {
		t.Fatal(got)
	}
	if got := localizedGameHours("en", 90); got != "1.5 hours" {
		t.Fatal(got)
	}
	if got := localizedGameHours("en", nil); got != "Playtime unavailable" {
		t.Fatal(got)
	}
	if got := localizedGameHours("en", 3); got != "Less than 0.1 hours" {
		t.Fatal(got)
	}
}

func TestPWAUILanguageKeepsInstallIdentityAndSiteName(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/manifest.webmanifest?lang=en", nil)
	w := httptest.NewRecorder()
	writePWAManifest(w, r, "产品 · Painter")
	var manifest pwaManifest
	if err := json.Unmarshal(w.Body.Bytes(), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Language != "en" || manifest.Shortcuts[0].Name != "Articles" || manifest.Shortcuts[1].Name != "Products" || manifest.Shortcuts[2].Name != "Workspace" {
		t.Fatal("localized app shortcuts", manifest)
	}
	if manifest.ID != "/" || manifest.StartURL != "/" || manifest.Name != "产品 · Painter" {
		t.Fatal("localization changed app identity or site name", manifest)
	}
	if w.Header().Get("Content-Language") != "en" || !strings.Contains(strings.Join(w.Header().Values("Vary"), ","), "Accept-Language") || !strings.Contains(strings.Join(w.Header().Values("Vary"), ","), "Cookie") {
		t.Fatal("manifest cache must vary with language", w.Header())
	}
}

func TestSSRLocalizesInterfaceWithoutRewritingContent(t *testing.T) {
	templates := v31NavigationTemplates(t)
	// Reuse parsed templates but bind the English functions before first execution.
	templates = templates.Funcs(localeTemplateFuncs("en"))
	page := v31NavigationPage("product", defaultFloatingControls(), false, false)
	page.Locale = "en"
	page.LocaleNext = "/products/painter"
	page.Post = map[string]any{"id": "1", "slug": "painter", "name": "我的产品", "summary": "保留作者的中文介绍", "state": "available", "platforms": []string{"Web"}, "links": map[string]any{"main": map[string]any{"url": "/", "type": "web"}}, "author": map[string]any{}}
	page.HTML = template.HTML("<p>正文内容保持原样</p>")
	var b bytes.Buffer
	if err := templates.ExecuteTemplate(&b, "page", page); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<html lang="en">`, "All products", "Available", "我的产品", "保留作者的中文介绍", "正文内容保持原样", "English", "简体中文"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(b.String(), ">所有产品<") {
		t.Fatal("untranslated navigation")
	}
}

func TestSSRExplicitLocalePersistsAcrossNativeNavigation(t *testing.T) {
	a := testApp(t)
	a.RegisterWeb()
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/products?lang=en", nil))
	if w.Code != 200 || w.Header().Get("Content-Language") != "en" || !strings.Contains(w.Body.String(), `<html lang="en">`) {
		t.Fatal("explicit page locale was not applied", w.Code, w.Header())
	}
	var preference *http.Cookie
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == localeCookie {
			preference = cookie
		}
	}
	if preference == nil || preference.Value != "en" || !preference.HttpOnly {
		t.Fatal("explicit query choice must persist", w.Result().Cookies())
	}
	if strings.Contains(w.Body.String(), `rel="canonical" href="`+a.Config.PublicBaseURL+`/products?lang=`) {
		t.Fatal("UI language must not fork canonical content URLs")
	}
	request := httptest.NewRequest(http.MethodGet, "/posts", nil)
	request.AddCookie(preference)
	w = httptest.NewRecorder()
	a.Handler().ServeHTTP(w, request)
	if w.Code != 200 || w.Header().Get("Content-Language") != "en" || !strings.Contains(w.Body.String(), ">Articles</h1>") {
		t.Fatal("native navigation lost locale", w.Code, w.Header())
	}
}
