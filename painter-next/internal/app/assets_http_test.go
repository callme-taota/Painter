package app

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"painter-next/web"
)

func assetTestHandler() http.Handler {
	assets := fstest.MapFS{
		"public-test.js":        {Data: []byte("export const value=42;"), ModTime: time.Unix(1700000000, 0)},
		"chunks/lazy-test.js":   {Data: []byte("export default 1;")},
		"theme-test.css":        {Data: []byte("body{color:#111}")},
		"images/logo.svg":       {Data: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>")},
		"fonts/text.woff2":      {Data: []byte("font bytes")},
		"module-test.wasm":      {Data: []byte("\x00asm")},
		"public-test.js.map":    {Data: []byte("private source map")},
		"PUBLIC-TEST.JS.MAP":    {Data: []byte("private source map")},
		".vite/manifest.json":   {Data: []byte("private manifest")},
		".env":                  {Data: []byte("private config")},
		"chunks/.hidden.js":     {Data: []byte("private hidden asset")},
		"chunks/.secret/app.js": {Data: []byte("private hidden path")},
		"source/public.ts":      {Data: []byte("private TypeScript")},
		"source/Editor.vue":     {Data: []byte("private Vue source")},
		"source/file.scss":      {Data: []byte("private style source")},
		"server.go":             {Data: []byte("private server source")},
		"config.env":            {Data: []byte("private config")},
	}
	return http.StripPrefix("/assets/", assetHTTPHandler(assets))
}

func TestAssetHTTPBuiltFiles(t *testing.T) {
	handler := assetTestHandler()
	for _, target := range []string{
		"public-test.js", "chunks/lazy-test.js", "theme-test.css",
		"images/logo.svg", "fonts/text.woff2", "module-test.wasm",
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method+"/"+target, func(t *testing.T) {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(method, "/assets/"+target, nil))
				if response.Code != http.StatusOK {
					t.Fatalf("status = %d", response.Code)
				}
				if response.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
					t.Fatal("built asset lost immutable cache policy")
				}
				if response.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Fatal("built asset missing nosniff")
				}
				if response.Header().Get("Content-Type") == "" || response.Header().Get("Content-Length") == "" {
					t.Fatal("built asset missing file response metadata")
				}
				if method == http.MethodHead && response.Body.Len() != 0 {
					t.Fatal("HEAD returned a body")
				}
				if method == http.MethodGet && response.Body.Len() == 0 {
					t.Fatal("GET lost asset body")
				}
			})
		}
	}
}

func TestAssetHTTPRejectsMetadataSourcesAndDirectories(t *testing.T) {
	handler := assetTestHandler()
	for _, target := range []string{
		"", "chunks", "chunks/", "source", "images/", "missing.js",
		"public-test.js.map", "PUBLIC-TEST.JS.MAP", ".vite/manifest.json",
		"%2evite/manifest.json", ".env", "chunks/.hidden.js", "chunks/.secret/app.js",
		"source/public.ts", "source/Editor.vue", "source/file.scss", "server.go", "config.env",
		"chunks/../public-test.js", "chunks//lazy-test.js", "chunks/%5cpublic-test.js",
	} {
		t.Run(target, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/"+target, nil))
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d; expected non-disclosing 404", response.Code)
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("asset rejection must not be cached as immutable")
			}
			if strings.Contains(response.Body.String(), "private") || strings.Contains(response.Body.String(), "href=") {
				t.Fatal("asset rejection leaked metadata or a directory listing")
			}
		})
	}
}

func TestAssetHTTPPreservesRangeAndConditionalRequests(t *testing.T) {
	handler := assetTestHandler()
	request := httptest.NewRequest(http.MethodGet, "/assets/public-test.js", nil)
	request.Header.Set("Range", "bytes=0-5")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent || response.Body.String() != "export" {
		t.Fatalf("range response = %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Range") != "bytes 0-5/22" {
		t.Fatalf("Content-Range = %q", response.Header().Get("Content-Range"))
	}
	request = httptest.NewRequest(http.MethodGet, "/assets/public-test.js", nil)
	request.Header.Set("If-Modified-Since", time.Unix(1700000000, 0).UTC().Format(http.TimeFormat))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotModified || response.Body.Len() != 0 {
		t.Fatalf("conditional response = %d, body bytes %d", response.Code, response.Body.Len())
	}
}

func TestAssetHTTPEmbeddedProductionFiles(t *testing.T) {
	assets := web.AssetFS()
	handler := http.StripPrefix("/assets/", assetHTTPHandler(assets))
	entry, _ := web.Assets("src/public.ts")
	if entry == "/assets/" {
		t.Fatal("production manifest has no public entry")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, entry, nil))
	if response.Code != http.StatusOK || response.Body.Len() == 0 {
		t.Fatalf("embedded public entry = %d, body bytes %d", response.Code, response.Body.Len())
	}
	if _, err := fs.Stat(assets, ".vite/manifest.json"); err != nil {
		t.Fatal("server-side embedded manifest should remain available", err)
	}
	for _, target := range []string{"/assets/", "/assets/chunks/", "/assets/.vite/manifest.json"} {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusNotFound || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("embedded metadata %s returned %d with cache %q", target, response.Code, response.Header().Get("Cache-Control"))
		}
	}
}
