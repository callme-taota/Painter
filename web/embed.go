package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	"sync"
)

// FS includes the production frontend. Building the Go application never needs Node.
// The all: prefix intentionally includes Vite's .vite/manifest.json.
//
//go:embed templates locales all:assets
var FS embed.FS

type Chunk struct {
	File    string   `json:"file"`
	CSS     []string `json:"css"`
	Imports []string `json:"imports"`
}

var manifest map[string]Chunk
var once sync.Once

func Assets(entry string) (string, []string) {
	once.Do(func() {
		manifest = map[string]Chunk{}
		b, e := FS.ReadFile("assets/.vite/manifest.json")
		if e == nil {
			_ = json.Unmarshal(b, &manifest)
		}
	})
	chunk := manifest[entry]
	styles := []string{}
	seen := map[string]bool{}
	var collect func(string)
	collect = func(key string) {
		if seen[key] {
			return
		}
		seen[key] = true
		c := manifest[key]
		for _, dep := range c.Imports {
			collect(dep)
		}
		for _, style := range c.CSS {
			styles = append(styles, "/assets/"+style)
		}
	}
	collect(entry)
	return "/assets/" + chunk.File, styles
}
func AssetFS() fs.FS { f, _ := fs.Sub(FS, "assets"); return f }
