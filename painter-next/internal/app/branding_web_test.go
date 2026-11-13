package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	nethtml "golang.org/x/net/html"
)

func TestWebBrandTitlesFollowPageAndSite(t *testing.T) {
	a := testApp(t)
	testIdentity(t, a, "owner")
	if _, err := a.SeedDemo(context.Background()); err != nil {
		t.Fatal(err)
	}
	a.RegisterWeb()
	rows, err := a.Pool.Query(context.Background(), "SELECT post_id::text,title FROM public_posts ORDER BY post_id LIMIT 2")
	if err != nil {
		t.Fatal(err)
	}
	articles := map[string]string{}
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			t.Fatal(err)
		}
		articles["/posts/"+id] = title
	}
	rows.Close()
	if len(articles) != 2 {
		t.Fatal("two public article fixtures are required")
	}
	for _, site := range []string{"Painter", "Callum & <书房>"} {
		if _, err := a.Pool.Exec(context.Background(), "UPDATE site_profile SET display_name=$1 WHERE id=1", site); err != nil {
			t.Fatal(err)
		}
		pages := map[string]string{"/": site, "/posts": "所有文章 · " + site}
		for path, title := range articles {
			pages[path] = title + " · " + site
		}
		for path, want := range pages {
			w := httptest.NewRecorder()
			a.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			if w.Code != 200 {
				t.Fatalf("%s status %d", path, w.Code)
			}
			doc := friendWebDocument(t, w.Body.String())
			titles := friendWebNodes(doc, func(n *nethtml.Node) bool { return n.Type == nethtml.ElementNode && n.Data == "title" })
			if len(titles) != 1 || friendWebText(titles[0]) != want {
				t.Fatalf("%s title does not follow content/site: want %q", path, want)
			}
			icons := friendWebNodes(doc, func(n *nethtml.Node) bool {
				return n.Type == nethtml.ElementNode && n.Data == "link" && friendWebAttr(n, "rel") == "icon"
			})
			if len(icons) != 1 || friendWebAttr(icons[0], "href") != "/assets/"+siteFaviconPath || friendWebAttr(icons[0], "type") != "image/x-icon" {
				t.Fatal("missing legacy favicon declaration", path)
			}
			if len(friendWebNodes(doc, func(n *nethtml.Node) bool { return n.Type == nethtml.ElementNode && n.Data == "书房" })) != 0 {
				t.Fatal("site name injected markup")
			}
		}
	}
}

func TestWebLegacyFaviconRootAndAsset(t *testing.T) {
	// RegisterWeb only uses the embedded filesystem for these requests.
	a := &App{Mux: http.NewServeMux()}
	a.RegisterWeb()
	var icon []byte
	for _, target := range []string{"/assets/" + siteFaviconPath, "/favicon.ico"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			w := httptest.NewRecorder()
			a.Handler().ServeHTTP(w, httptest.NewRequest(method, target, nil))
			contentType := w.Header().Get("Content-Type")
			if w.Code != 200 || (contentType != "image/x-icon" && contentType != "image/vnd.microsoft.icon") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("%s %s did not return an icon: status %d headers %v", method, target, w.Code, w.Header())
			}
			if method == http.MethodHead {
				if w.Body.Len() != 0 {
					t.Fatal("favicon HEAD returned a body")
				}
				continue
			}
			if icon == nil {
				icon = bytes.Clone(w.Body.Bytes())
			} else if !bytes.Equal(icon, w.Body.Bytes()) {
				t.Fatal("root favicon differs from declared legacy asset")
			}
		}
	}
	if len(icon) < 6 || !bytes.Equal(icon[:4], []byte{0, 0, 1, 0}) {
		t.Fatal("favicon is not an ICO file")
	}
}
