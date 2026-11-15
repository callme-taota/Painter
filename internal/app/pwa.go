package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"painter-next/web"
)

type pwaIcon struct {
	Source  string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose"`
}

type pwaShortcut struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type pwaManifest struct {
	ID                        string        `json:"id"`
	Name                      string        `json:"name"`
	ShortName                 string        `json:"short_name"`
	Language                  string        `json:"lang"`
	StartURL                  string        `json:"start_url"`
	Scope                     string        `json:"scope"`
	Display                   string        `json:"display"`
	BackgroundColor           string        `json:"background_color"`
	ThemeColor                string        `json:"theme_color"`
	PreferRelatedApplications bool          `json:"prefer_related_applications"`
	Icons                     []pwaIcon     `json:"icons"`
	Shortcuts                 []pwaShortcut `json:"shortcuts"`
}

func manifestForSite(name string) pwaManifest {
	if strings.TrimSpace(name) == "" {
		name = "Painter"
	}
	return pwaManifest{
		ID: "/", Name: name, ShortName: name, Language: "zh-CN",
		StartURL: "/", Scope: "/", Display: "standalone",
		BackgroundColor: "#ffffff", ThemeColor: "#ffffff",
		Icons: []pwaIcon{
			{Source: "/pwa/icon-192.png", Sizes: "192x192", Type: "image/png", Purpose: "any"},
			{Source: "/pwa/icon-512.png", Sizes: "512x512", Type: "image/png", Purpose: "any"},
		},
		// These are normal entry URLs: their usual visibility and login checks apply.
		Shortcuts: []pwaShortcut{{Name: "文章", URL: "/posts"}, {Name: "产品", URL: "/products"}, {Name: "工作台", URL: "/studio"}},
	}
}

func writePWAManifest(w http.ResponseWriter, r *http.Request, name string) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(manifestForSite(name)); err != nil {
		Error(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodHead {
		_, _ = w.Write(body.Bytes())
	}
}

// The manifest and its stable icon URLs must revalidate independently of Vite's
// immutable asset handler. No service worker caches pages, authentication or API
// responses: the first installed release has the same online behavior as the site.
func (a *App) registerPWA() {
	a.Mux.HandleFunc("GET /manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		var name string
		if err := a.Pool.QueryRow(r.Context(), "SELECT display_name FROM site_profile WHERE id=1").Scan(&name); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			w.Header().Set("Cache-Control", "no-store")
			Error(w, err)
			return
		}
		writePWAManifest(w, r, name)
	})
	for route, file := range map[string]string{
		"GET /pwa/icon-192.png": "images/painter-app-192.png",
		"GET /pwa/icon-512.png": "images/painter-logo.png",
	} {
		a.Mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			http.ServeFileFS(w, r, web.AssetFS(), file)
		})
	}
}
