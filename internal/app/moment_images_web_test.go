package app

import (
	"net/http"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

func TestMomentImagesWebImageOnlyGalleryEscapingAndSnapshotMetadata(t *testing.T) {
	a := momentWebApp(t)
	a.RegisterMedia()
	_, author, csrf := testIdentity(t, a, "author")
	alt := `<script data-moment-image-xss>alert("no")</script> & "quoted alt"`
	old := momentImage{momentImageUpload(t, a, author, csrf, 1), alt}
	newImage := momentImage{momentImageUpload(t, a, author, csrf, 2), "private-image-alt-sentinel"}
	item := momentImageFixture(t, a, author, csrf, "", []momentImage{old})
	id := item["id"].(string)
	base := "/api/v1/studio/moments/" + id
	momentResponse(t, momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 1, RandID()), 200)
	momentResponse(t, momentCall(t, a, author, csrf, "PUT", base+"/draft", map[string]any{"text": "private-image-text-sentinel", "images": []momentImage{newImage}}, 2, RandID()), 200)
	for _, cookie := range []*http.Cookie{nil, author} {
		for _, path := range []string{"/moments", "/moments/" + id} {
			response := testRequest(a, "GET", path, "", cookie, "")
			if response.Code != 200 || strings.Contains(response.Body.String(), newImage.MediaID) || strings.Contains(response.Body.String(), "private-image-") {
				t.Fatal("draft replacement/alt/metadata leaked in SSR", path, response.Code)
			}
			doc := friendWebDocument(t, response.Body.String())
			galleries := friendWebClass(doc, "moment-gallery")
			if len(galleries) != 1 || friendWebAttr(galleries[0], "data-image-count") != "1" {
				t.Fatal("no-JavaScript image-only gallery missing", path)
			}
			anchors := friendWebNodes(galleries[0], func(n *nethtml.Node) bool {
				return n.Type == nethtml.ElementNode && n.Data == "a" && friendWebHasAttr(n, "data-moment-image")
			})
			images := friendWebNodes(galleries[0], func(n *nethtml.Node) bool { return n.Type == nethtml.ElementNode && n.Data == "img" })
			if len(anchors) != 1 || friendWebAttr(anchors[0], "href") != "/media/"+old.MediaID || len(images) != 1 || friendWebAttr(images[0], "src") != "/media/"+old.MediaID || friendWebAttr(images[0], "alt") != alt {
				t.Fatal("gallery image, native link or alt text wrong", path)
			}
			injected := friendWebNodes(doc, func(n *nethtml.Node) bool {
				return n.Type == nethtml.ElementNode && (friendWebHasAttr(n, "data-moment-image-xss") || friendWebHasAttr(n, "onerror"))
			})
			if len(injected) != 0 {
				t.Fatal("alt text injected HTML or event handler")
			}
			if path == "/moments/"+id {
				og := friendWebNodes(doc, func(n *nethtml.Node) bool {
					return n.Type == nethtml.ElementNode && n.Data == "meta" && friendWebAttr(n, "property") == "og:image"
				})
				description := friendWebNodes(doc, func(n *nethtml.Node) bool {
					return n.Type == nethtml.ElementNode && n.Data == "meta" && friendWebAttr(n, "name") == "description"
				})
				if len(og) != 1 || friendWebAttr(og[0], "content") != a.Config.PublicBaseURL+"/media/"+old.MediaID || len(description) != 1 || !strings.Contains(friendWebAttr(description[0], "content"), "分享了1张图片") {
					t.Fatal("image-only published metadata missing/private", path)
				}
			}
		}
	}
	momentResponse(t, momentCall(t, a, author, csrf, "DELETE", base+"/publication", nil, 3, RandID()), 200)
	for _, path := range []string{"/moments/" + id, "/media/" + old.MediaID, "/media/" + newImage.MediaID} {
		if w := testRequest(a, "GET", path, "", nil, ""); w.Code != 404 {
			t.Fatal("withdraw did not revoke SSR/media visibility", path, w.Code)
		}
	}
}
