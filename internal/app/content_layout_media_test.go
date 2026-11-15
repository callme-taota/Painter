package app

import (
	"context"
	"strings"
	"testing"
)

func TestContentLayoutMediaAuthorizationAcrossPreviewImportSaveAndPublish(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterMedia()
	ctx := context.Background()
	p, author, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	own := contentWant(t, mediaTestUpload(t, a, author, csrf, RandID(), 91), 201)["media_id"].(string)
	foreign := contentWant(t, mediaTestUpload(t, a, other, otherCSRF, RandID(), 92), 201)["media_id"].(string)
	staging := RandID()
	if _, err := a.Pool.Exec(ctx, `INSERT INTO media_assets(id,owner_user_id,storage_key,original_name,mime_type,byte_size,sha256,state,created_at_ms,updated_at_ms) VALUES($1,$2,$3,'pending.png','image/png',10,$4,'staging',$5,$5)`, staging, p.ID, RandID(), strings.Repeat("a", 64), NowMillis()); err != nil {
		t.Fatal(err)
	}
	var post string
	for _, sample := range []struct {
		name, id string
		allowed  bool
	}{{"own ready", own, true}, {"foreign", foreign, false}, {"missing", RandID(), false}, {"not ready", staging, false}} {
		t.Run(sample.name, func(t *testing.T) {
			card := contentCard{V: 3, Type: "layout", Root: &ContentLayoutNode{Kind: "panel", Children: []*ContentLayoutNode{{Kind: "stack", Children: []*ContentLayoutNode{layoutText("body", "组合图片权限"), {Kind: "image", Src: "/media/" + sample.id}}}}}}
			markdown := contentCardMarkdown(card)
			status := 409
			if sample.allowed {
				status = 200
			}
			preview := contentCall(t, a, author, csrf, "POST", "/api/v1/editor/preview", map[string]any{"markdown": markdown, "seq": 1}, 0, "")
			contentWant(t, preview, status)
			imported := articleImportRequest(a, author, csrf, articleImportDocument(t, map[string]any{"type": "card", "card": card}))
			contentWant(t, imported, status)
			createStatus := 409
			if sample.allowed {
				createStatus = 201
			}
			created := contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "组合图片权限", "markdown": markdown, "media_ids": []string{sample.id}}, 0, ""), createStatus)
			if sample.allowed {
				post = created["id"].(string)
			}
		})
	}
	publicMediaStatus := func(want int) {
		t.Helper()
		if response := contentCall(t, a, nil, "", "GET", "/media/"+own, nil, 0, ""); response.Code != want {
			t.Fatal("unexpected public media availability", response.Code, want)
		}
	}
	publicMediaStatus(404)
	contentWant(t, contentCall(t, a, author, csrf, "POST", "/api/v1/studio/posts/"+post+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	publicMediaStatus(200)
	var refs int
	if err := a.Pool.QueryRow(ctx, `SELECT count(*) FROM post_revision_media_refs mr JOIN post_publications pub ON pub.revision_id=mr.revision_id WHERE pub.post_id=$1 AND mr.media_id=$2`, post, own).Scan(&refs); err != nil || refs != 1 {
		t.Fatal("nested image was not sealed into published references", refs, err)
	}
	contentWant(t, contentCall(t, a, author, csrf, "DELETE", "/api/v1/studio/posts/"+post+"/publication", nil, 2, ""), 200)
	publicMediaStatus(404)
}
