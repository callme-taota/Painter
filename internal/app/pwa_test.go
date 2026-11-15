package app

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"painter-next/web"
)

func TestPWAManifestIdentityAndSafeSiteName(t *testing.T) {
	for _, name := range []string{"Painter", "Callum & <书房> · 手记", "带有\n换行的名字"} {
		w := httptest.NewRecorder()
		writePWAManifest(w, httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil), name)
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/manifest+json; charset=utf-8" || w.Header().Get("Cache-Control") != "no-cache" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("manifest headers: status %d headers %v", w.Code, w.Header())
		}
		var m pwaManifest
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		if m.Name != name || m.ShortName != name {
			t.Fatalf("site name changed: want %q, got %q / %q", name, m.Name, m.ShortName)
		}
		if m.ID != "/" || m.StartURL != "/" || m.Scope != "/" || m.Display != "standalone" || m.PreferRelatedApplications {
			t.Fatalf("unstable app identity or launch mode: %+v", m)
		}
		if strings.Contains(w.Body.String(), "<书房>") {
			t.Fatal("manifest JSON does not escape markup")
		}
		if len(m.Icons) != 2 || m.Icons[0].Sizes != "192x192" || m.Icons[1].Sizes != "512x512" {
			t.Fatalf("missing required install icons: %+v", m.Icons)
		}
		for _, icon := range m.Icons {
			if icon.Type != "image/png" || icon.Purpose != "any" || !strings.HasPrefix(icon.Source, "/pwa/") {
				t.Fatalf("invalid icon declaration: %+v", icon)
			}
		}
		wantPaths := []string{"/posts", "/products", "/studio"}
		if len(m.Shortcuts) != len(wantPaths) {
			t.Fatal("unexpected shortcuts", m.Shortcuts)
		}
		for i, shortcut := range m.Shortcuts {
			u, err := url.Parse(shortcut.URL)
			if err != nil || u.IsAbs() || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || shortcut.URL != wantPaths[i] || shortcut.Name == "" {
				t.Fatalf("shortcut must use a plain same-origin entry: %+v", shortcut)
			}
		}
		// Keep this public document small and independent from deployment settings,
		// user/session data, and any future private configuration fields.
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(w.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields) != 12 {
			t.Fatalf("review newly exposed manifest fields: %v", fields)
		}
	}
	if got := manifestForSite(" \n\t").Name; got != "Painter" {
		t.Fatal("missing site name has no usable default", got)
	}
	w := httptest.NewRecorder()
	writePWAManifest(w, httptest.NewRequest(http.MethodHead, "/manifest.webmanifest", nil), "Painter")
	if w.Body.Len() != 0 || w.Header().Get("Cache-Control") != "no-cache" {
		t.Fatal("manifest HEAD must preserve headers without a body")
	}
}

func TestPWAIconsMatchManifestAndPreservePainterLogo(t *testing.T) {
	a := &App{Mux: http.NewServeMux()}
	a.registerPWA()
	for _, icon := range manifestForSite("Painter").Icons {
		w := httptest.NewRecorder()
		a.Mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, icon.Source, nil))
		if w.Code != 200 || w.Header().Get("Content-Type") != icon.Type || w.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("icon %s: status %d headers %v", icon.Source, w.Code, w.Header())
		}
		config, err := png.DecodeConfig(bytes.NewReader(w.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		want := 192
		if icon.Sizes == "512x512" {
			want = 512
			original, err := web.FS.ReadFile("assets/images/painter-logo.png")
			if err != nil || !bytes.Equal(original, w.Body.Bytes()) {
				t.Fatal("512px install icon must retain the existing Painter logo")
			}
		}
		if config.Width != want || config.Height != want {
			t.Fatalf("%s declares %s but is %dx%d", icon.Source, icon.Sizes, config.Width, config.Height)
		}
		w = httptest.NewRecorder()
		a.Mux.ServeHTTP(w, httptest.NewRequest(http.MethodHead, icon.Source, nil))
		if w.Code != http.StatusOK || w.Body.Len() != 0 || w.Header().Get("Cache-Control") != "no-cache" {
			t.Fatal("icon HEAD must preserve headers without a body")
		}
	}
}

func TestPWAManifestUsesCurrentSiteNameWithoutImmutableCaching(t *testing.T) {
	a := testApp(t)
	a.registerPWA()
	for _, name := range []string{"Initial name", "新的 & <站名>"} {
		if _, err := a.Pool.Exec(context.Background(), "UPDATE site_profile SET display_name=$1 WHERE id=1", name); err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil))
		var m pwaManifest
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil || w.Code != http.StatusOK || m.Name != name || w.Header().Get("Cache-Control") != "no-cache" {
			t.Fatalf("manifest must use the current name: status %d manifest %+v error %v", w.Code, m, err)
		}
	}
}
