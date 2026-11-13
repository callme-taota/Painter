package app

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// assetHTTPHandler serves built files, never build metadata, source files, or
// directory listings. The embedded manifest remains available to the server.
func assetHTTPHandler(assets fs.FS) http.Handler {
	files := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if !publicAssetPath(name) {
			assetNotFound(w, r)
			return
		}
		info, err := fs.Stat(assets, name)
		if err != nil || !info.Mode().IsRegular() {
			assetNotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		files.ServeHTTP(w, r)
	})
}

func publicAssetPath(name string) bool {
	if !fs.ValidPath(name) || strings.Contains(name, "\\") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".map", ".ts", ".tsx", ".vue", ".jsx", ".svelte", ".coffee",
		".scss", ".sass", ".less", ".styl", ".env", ".go", ".py", ".rb",
		".php", ".rs", ".java", ".kt", ".swift", ".sh", ".bash", ".zsh",
		".sql", ".yaml", ".yml", ".toml", ".lock":
		return false
	}
	return true
}

func assetNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.NotFound(w, r)
}
