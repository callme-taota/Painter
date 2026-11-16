package app

import (
	"context"
	"net/http"
	"strconv"
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
			if path == "/moments" {
				covers := friendWebClass(doc, "moment-cover")
				previews := friendWebClass(doc, "moment-preview")
				if len(galleries) != 0 || len(covers) != 1 || friendWebAttr(covers[0], "src") != "/media/"+old.MediaID || friendWebAttr(covers[0], "alt") != alt || len(previews) != 1 || friendWebAttr(previews[0], "href") != "/moments/"+id || !friendWebHasAttr(previews[0], "data-moment-open") {
					t.Fatal("image-only card lacks escaped cover or native detail link", path)
				}
			} else {
				if len(galleries) != 1 || friendWebAttr(galleries[0], "data-image-count") != "1" {
					t.Fatal("no-JavaScript image-only detail gallery missing", path)
				}
				anchors := friendWebNodes(galleries[0], func(n *nethtml.Node) bool {
					return n.Type == nethtml.ElementNode && n.Data == "a" && friendWebHasAttr(n, "data-moment-image")
				})
				images := friendWebNodes(galleries[0], func(n *nethtml.Node) bool { return n.Type == nethtml.ElementNode && n.Data == "img" })
				if len(anchors) != 1 || friendWebAttr(anchors[0], "href") != "/media/"+old.MediaID || len(images) != 1 || friendWebAttr(images[0], "src") != "/media/"+old.MediaID || friendWebAttr(images[0], "alt") != alt {
					t.Fatal("detail gallery image, native link or alt text wrong", path)
				}
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

func TestMomentImagesWebCardsPreviewFirstImageAndDetailKeepsAllImages(t *testing.T) {
	a := momentWebApp(t)
	a.RegisterMedia()
	_, author, csrf := testIdentity(t, a, "author")
	images := make([]momentImage, 9)
	for i := range images {
		images[i] = momentImage{momentImageUpload(t, a, author, csrf, byte(i+1)), "图片 " + strconv.Itoa(i+1)}
	}
	images[0].Alt = `<img data-moment-alt-injection src=x onerror="alert('no')"> & 首图`
	text := strings.Repeat("这是一段需要完整保留的动态内容。", 20) + "\n<script data-moment-text-injection>alert('no')</script> & 末尾不可截断"
	for _, count := range []int{1, 3, 9} {
		t.Run(strconv.Itoa(count)+"_images", func(t *testing.T) {
			// Reverse the upload order to detect accidental sorting of published images.
			ordered := make([]momentImage, count)
			for i := range ordered {
				ordered[i] = images[count-i-1]
			}
			item := momentImageFixture(t, a, author, csrf, text, ordered)
			id := item["id"].(string)
			momentResponse(t, momentCall(t, a, author, csrf, "POST", "/api/v1/studio/moments/"+id+"/publish", nil, 1, RandID()), 200)
			list := testRequest(a, "GET", "/moments", "", nil, "")
			if list.Code != 200 {
				t.Fatal("public feed failed", list.Code)
			}
			doc := friendWebDocument(t, list.Body.String())
			cards := friendWebNodes(doc, func(n *nethtml.Node) bool {
				return n.Type == nethtml.ElementNode && n.Data == "article" && friendWebAttr(n, "id") == "moment-"+id
			})
			if len(cards) != 1 {
				t.Fatal("published moment missing or duplicated", id)
			}
			card := cards[0]
			previews := friendWebClass(card, "moment-preview")
			if len(previews) != 1 || previews[0].Data != "a" || friendWebAttr(previews[0], "href") != "/moments/"+id || !friendWebHasAttr(previews[0], "data-moment-open") {
				t.Fatal("card cannot open detail without JavaScript")
			}
			covers := friendWebNodes(previews[0], func(n *nethtml.Node) bool { return n.Type == nethtml.ElementNode && n.Data == "img" })
			if len(covers) != 1 || friendWebAttr(covers[0], "src") != "/media/"+ordered[0].MediaID || friendWebAttr(covers[0], "alt") != ordered[0].Alt {
				t.Fatal("feed must render only the first published image in order")
			}
			if len(friendWebClass(card, "moment-gallery")) != 0 || len(friendWebClass(card, "moment-slide-controls")) != 0 {
				t.Fatal("list card includes detail-gallery controls")
			}
			counts := friendWebClass(card, "moment-image-count")
			if count == 1 && len(counts) != 0 || count > 1 && (len(counts) != 1 || strings.TrimSpace(friendWebText(counts[0])) != strconv.Itoa(count)) {
				t.Fatal("card image count is missing or shown for a single image")
			}
			texts := friendWebClass(card, "moment-text")
			if len(texts) != 1 || friendWebText(texts[0]) != text {
				t.Fatal("list preview truncated or parsed the original plain text")
			}
			visibleH1 := friendWebNodes(doc, func(n *nethtml.Node) bool {
				return n.Type == nethtml.ElementNode && n.Data == "h1" && !strings.Contains(" "+friendWebAttr(n, "class")+" ", " sr-only ")
			})
			layoutControls := friendWebNodes(doc, func(n *nethtml.Node) bool {
				return n.Type == nethtml.ElementNode && (friendWebHasAttr(n, "data-moment-layout") || friendWebHasAttr(n, "data-moments-layout"))
			})
			if len(visibleH1) != 0 || len(friendWebClass(doc, "browse-heading")) != 0 || len(layoutControls) != 0 {
				t.Fatal("public feed adds a visible page heading or layout selector")
			}

			detail := testRequest(a, "GET", "/moments/"+id, "", nil, "")
			if detail.Code != 200 {
				t.Fatal("public detail failed", detail.Code)
			}
			detailDoc := friendWebDocument(t, detail.Body.String())
			readers := friendWebClass(detailDoc, "moment-reader")
			if len(readers) != 1 || friendWebAttr(readers[0], "data-moment-id") != id {
				t.Fatal("detail reader missing published identity")
			}
			anchors := friendWebNodes(readers[0], func(n *nethtml.Node) bool {
				return n.Type == nethtml.ElementNode && n.Data == "a" && friendWebHasAttr(n, "data-moment-image")
			})
			if len(anchors) != count {
				t.Fatal("detail omitted published images", len(anchors), count)
			}
			for i, link := range anchors {
				wantURL := "/media/" + ordered[i].MediaID
				photos := friendWebNodes(link, func(n *nethtml.Node) bool { return n.Type == nethtml.ElementNode && n.Data == "img" })
				if friendWebAttr(link, "href") != wantURL || len(photos) != 1 || friendWebAttr(photos[0], "src") != wantURL || friendWebAttr(photos[0], "alt") != ordered[i].Alt {
					t.Fatal("detail image/native link order or escaped alt changed", i)
				}
			}
			detailTexts := friendWebClass(readers[0], "moment-text")
			if len(detailTexts) != 1 || friendWebText(detailTexts[0]) != text {
				t.Fatal("detail omitted the full original plain text")
			}
			for _, page := range []*nethtml.Node{doc, detailDoc} {
				injected := friendWebNodes(page, func(n *nethtml.Node) bool {
					return n.Type == nethtml.ElementNode && (friendWebHasAttr(n, "data-moment-alt-injection") || friendWebHasAttr(n, "data-moment-text-injection") || friendWebHasAttr(n, "onerror"))
				})
				if len(injected) != 0 {
					t.Fatal("published text or alt injected executable markup")
				}
			}
			og := friendWebNodes(detailDoc, func(n *nethtml.Node) bool {
				return n.Type == nethtml.ElementNode && n.Data == "meta" && friendWebAttr(n, "property") == "og:image"
			})
			if len(og) != 1 || friendWebAttr(og[0], "content") != a.Config.PublicBaseURL+"/media/"+ordered[0].MediaID {
				t.Fatal("detail preview metadata no longer uses the first public image")
			}
		})
	}
}

func TestMomentsWebCardAndReaderPreserveLargeIntegerIDs(t *testing.T) {
	a := momentWebApp(t)
	principal, _, _ := testIdentity(t, a, "author")
	const id = "9007199254740993"
	const privateID = "9007199254740994"
	const text = "大整数动态 ID 的公开正文"
	_, err := a.Pool.Exec(context.Background(), `INSERT INTO moments(id,author_user_id,draft_text,public_text,first_published_at_ms,published_at_ms,created_at_ms,updated_at_ms) VALUES
 (9007199254740993,$1,'unpublished-large-id-edit',$2,200,200,1,200),
 (9007199254740994,$1,'private-large-id-moment',NULL,NULL,NULL,1,1)`, principal.ID, text)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/moments", "/moments/" + id} {
		response := testRequest(a, "GET", path, "", nil, "")
		if response.Code != 200 || strings.Contains(response.Body.String(), privateID) || strings.Contains(response.Body.String(), "private-large-id-moment") || strings.Contains(response.Body.String(), "unpublished-large-id-edit") {
			t.Fatal("large ID SSR failed or exposed an unpublished value", path, response.Code)
		}
		doc := friendWebDocument(t, response.Body.String())
		if path == "/moments" {
			cards := friendWebClass(doc, "moment-card")
			if len(cards) != 1 || friendWebAttr(cards[0], "id") != "moment-"+id || friendWebAttr(cards[0], "data-list-key") != "moment-"+id {
				t.Fatal("card identity rounded a large integer ID")
			}
			links := friendWebClass(cards[0], "moment-preview")
			if len(links) != 1 || friendWebAttr(links[0], "href") != "/moments/"+id || len(friendWebClass(cards[0], "moment-cover")) != 0 {
				t.Fatal("text-only card has a rounded native link or invented image")
			}
		} else {
			readers := friendWebClass(doc, "moment-reader")
			if len(readers) != 1 || friendWebAttr(readers[0], "data-moment-id") != id {
				t.Fatal("detail identity changed for a large integer ID")
			}
			texts := friendWebClass(readers[0], "moment-text")
			if len(texts) != 1 || friendWebText(texts[0]) != text {
				t.Fatal("detail text changed for a large integer ID")
			}
			shares := friendWebNodes(readers[0], func(n *nethtml.Node) bool {
				return n.Type == nethtml.ElementNode && friendWebAttr(n, "data-share-type") == "moment"
			})
			canonical := friendWebNodes(doc, func(n *nethtml.Node) bool {
				return n.Type == nethtml.ElementNode && n.Data == "link" && friendWebAttr(n, "rel") == "canonical"
			})
			if len(shares) != 1 || friendWebAttr(shares[0], "data-share-id") != id || len(canonical) != 1 || friendWebAttr(canonical[0], "href") != a.Config.PublicBaseURL+"/moments/"+id {
				t.Fatal("share/canonical rounded a large integer ID")
			}
		}
	}
}
