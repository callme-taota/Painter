package app

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestArticleAutoCoversAreSafeRevisionSnapshots(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	_, c, csrf := testIdentity(t, a, "author")
	title := `<script>alert("cover")</script> & 中文文章标题`
	images := map[string]string{}
	for _, template := range []string{"editorial", "geometric", "minimal"} {
		w := contentCall(t, a, c, csrf, "POST", "/api/v1/editor/cover-preview", map[string]any{"title": title, "cover": CoverConfig{Mode: "auto", Template: template, Color: "moss"}}, 0, "")
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "image/svg+xml") || w.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") {
			t.Fatal(w.Code, w.Header(), w.Body.String())
		}
		if strings.Contains(w.Body.String(), "<script>") || !strings.Contains(w.Body.String(), "&lt;script&gt;") {
			t.Fatal("title was not XML escaped", w.Body.String())
		}
		decoder := xml.NewDecoder(bytes.NewReader(w.Body.Bytes()))
		for {
			if _, e := decoder.Token(); e == io.EOF {
				break
			} else if e != nil {
				t.Fatal("invalid SVG", e)
			}
		}
		images[template] = w.Body.String()
	}
	if images["editorial"] == images["geometric"] || images["minimal"] == images["editorial"] {
		t.Fatal("cover layouts are identical")
	}
	if w := contentCall(t, a, c, "wrong", "POST", "/api/v1/editor/cover-preview", map[string]any{"title": "draft"}, 0, ""); w.Code != 403 {
		t.Fatal("preview bypassed CSRF", w.Code)
	}
	if w := contentCall(t, a, nil, "", "POST", "/api/v1/editor/cover-preview", map[string]any{"title": "draft"}, 0, ""); w.Code != 401 {
		t.Fatal("preview allowed anonymous", w.Code)
	}
	d := map[string]any{"client_draft_id": RandID(), "title": title, "markdown": "文章正文"}
	created := contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/posts", d, 0, ""), 201)
	id := created["id"].(string)
	private := contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/studio/posts/"+id, nil, 0, ""), 200)
	if private["cover"].(map[string]any)["mode"] != "auto" {
		t.Fatal("old client did not default to auto", private)
	}
	pub := contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	rid := pub["revision_id"].(string)
	url := "/api/v1/posts/" + id + "/cover?revision=" + rid
	old := contentCall(t, a, nil, "", "GET", url, nil, 0, "")
	if old.Code != 200 {
		t.Fatal(old.Code, old.Body.String())
	}
	d["title"] = "尚未发布的新标题"
	d["cover"] = CoverConfig{Mode: "auto", Template: "minimal", Color: "plum"}
	d["expected_draft_version"] = 1
	contentWant(t, contentCall(t, a, c, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", d, 2, ""), 200)
	stillOld := contentCall(t, a, nil, "", "GET", url, nil, 0, "")
	if stillOld.Body.String() != old.Body.String() {
		t.Fatal("working cover changed published SVG")
	}
	working := contentCall(t, a, c, csrf, "GET", "/api/v1/studio/posts/"+id+"/cover", nil, 0, "")
	if working.Code != 200 || !strings.Contains(working.Body.String(), "尚未发布的新标题") {
		t.Fatal(working.Code, working.Body.String())
	}
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+id+"/restore", map[string]any{"revision_id": rid}, 3, ""), 200)
	private = contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/studio/posts/"+id, nil, 0, ""), 200)
	if private["title"] != title || private["cover"].(map[string]any)["template"] != "editorial" {
		t.Fatal("restore lost cover", private)
	}
	revision := contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/studio/posts/"+id+"/revisions/"+rid, nil, 0, ""), 200)
	if revision["cover"].(map[string]any)["color"] != "ink" {
		t.Fatal(revision)
	}
	contentWant(t, contentCall(t, a, c, csrf, "DELETE", "/api/v1/studio/posts/"+id+"/publication", nil, 4, ""), 200)
	if w := contentCall(t, a, nil, "", "GET", url, nil, 0, ""); w.Code != 404 {
		t.Fatal("withdrawn SVG remained public", w.Code)
	}
}

func TestUploadedCoverUsesMediaProtectionAndPublicationVisibility(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterMedia()
	_, c, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	upload := contentWant(t, mediaTestUpload(t, a, c, csrf, RandID(), 88), 201)
	media := upload["media_id"].(string)
	d := map[string]any{"client_draft_id": RandID(), "title": "上传封面", "markdown": "没有正文图片", "media_ids": []string{}, "cover": CoverConfig{Mode: "upload", MediaID: media}}
	if w := contentCall(t, a, other, otherCSRF, "POST", "/api/v1/posts", d, 0, ""); w.Code != 409 {
		t.Fatal("another author attached a private cover", w.Code, w.Body.String())
	}
	created := contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/posts", d, 0, ""), 201)
	id := created["id"].(string)
	private := contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/studio/posts/"+id, nil, 0, ""), 200)
	if len(private["media_ids"].([]any)) != 0 {
		t.Fatal("cover leaked into inline media contract", private)
	}
	if w := contentCall(t, a, c, csrf, "DELETE", "/api/v1/media/"+media, nil, 0, ""); w.Code != 409 {
		t.Fatal("draft cover did not protect upload", w.Code)
	}
	if w := contentCall(t, a, nil, "", "GET", "/media/"+media, nil, 0, ""); w.Code != 404 {
		t.Fatal("draft cover leaked", w.Code)
	}
	pub := contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	rid := pub["revision_id"].(string)
	url := "/api/v1/posts/" + id + "/cover?revision=" + rid
	if w := contentCall(t, a, nil, "", "GET", url, nil, 0, ""); w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal("published cover unreadable", w.Code, w.Body.String())
	}
	d["expected_draft_version"] = 1
	d["cover"] = CoverConfig{Mode: "none"}
	contentWant(t, contentCall(t, a, c, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", d, 2, ""), 200)
	if w := contentCall(t, a, nil, "", "GET", url, nil, 0, ""); w.Code != 200 {
		t.Fatal("draft change hid publication cover", w.Code)
	}
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 2}, 3, ""), 200)
	public := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	if public["cover_url"] != "" {
		t.Fatal("none cover URL present", public)
	}
	for _, path := range []string{url, "/media/" + media} {
		if w := contentCall(t, a, nil, "", "GET", path, nil, 0, ""); w.Code != 404 {
			t.Fatal("historical cover remained public", path, w.Code)
		}
	}
	if w := contentCall(t, a, c, csrf, "DELETE", "/api/v1/media/"+media, nil, 0, ""); w.Code != 409 {
		t.Fatal("historical cover lost GC protection", w.Code)
	}
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+id+"/restore", map[string]any{"revision_id": rid}, 4, ""), 200)
	private = contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/studio/posts/"+id, nil, 0, ""), 200)
	if private["cover"].(map[string]any)["media_id"] != media {
		t.Fatal("uploaded cover restore failed", private)
	}
	var refs int
	if e := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM post_revision_media_refs WHERE revision_id=$1 AND media_id=$2", rid, media).Scan(&refs); e != nil || refs != 1 {
		t.Fatal(refs, e)
	}
}

func TestArticleCoverBackupRestoresPublishedAndWorkingConfigurations(t *testing.T) {
	opsPGTools(t)
	a := testApp(t)
	a.RegisterContent()
	a.RegisterMedia()
	p, c, csrf := testIdentity(t, a, "author")
	upload := contentWant(t, mediaTestUpload(t, a, c, csrf, RandID(), 91), 201)
	media := upload["media_id"].(string)
	d := map[string]any{"client_draft_id": RandID(), "title": "备份的文章", "markdown": "稳定正文", "cover": CoverConfig{Mode: "auto", Template: "geometric", Color: "moss"}}
	created := contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/posts", d, 0, ""), 201)
	id := created["id"].(string)
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	d["title"] = "备份时的私有稿"
	d["cover"] = CoverConfig{Mode: "upload", Template: "minimal", Color: "plum", MediaID: media}
	d["expected_draft_version"] = 1
	contentWant(t, contentCall(t, a, c, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", d, 2, ""), 200)
	ctx := context.Background()
	before, e := a.privatePost(ctx, a.Pool, p, id)
	if e != nil {
		t.Fatal(e)
	}
	backupDir := filepath.Join(t.TempDir(), "cover-backup")
	if _, e = a.Backup(ctx, backupDir); e != nil {
		t.Fatal(e)
	}
	// Restore into another testApp database; neither this test nor its tools can
	// address the preview database. RestoreCheck independently validates the
	// same artifact in its own disposable database as the operator command does.
	checked, e := a.RestoreCheck(ctx, backupDir)
	if e != nil || !checked.Verified || !checked.DatabaseDropped {
		t.Fatal(checked, e)
	}
	restored := testApp(t)
	if e = pgRun(ctx, "pg_restore", restored.Config.DatabaseURL, "--clean", "--if-exists", "--no-owner", "--no-acl", "--dbname="+restored.Config.DatabaseURL, filepath.Join(backupDir, "database.dump")); e != nil {
		t.Fatal(e)
	}
	restored.Pool.Reset()
	after, e := restored.privatePost(ctx, restored.Pool, p, id)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("backup changed working content/cover metadata", before, after)
	}
	public, e := restored.PublicPost(ctx, id)
	if e != nil {
		t.Fatal(e)
	}
	if public["title"] != "备份的文章" || public["cover"].(map[string]any)["template"] != "geometric" {
		t.Fatal("backup lost immutable publication cover", public)
	}
	manifest, e := readBackupManifest(backupDir)
	if e != nil {
		t.Fatal(e)
	}
	if len(manifest.Media) != 1 || manifest.Media[0].ID != media {
		t.Fatal("backup omitted cover-only image", manifest.Media)
	}
	for _, object := range manifest.Media {
		if e = copyVerified(filepath.Join(backupDir, "media", "objects", object.StorageKey), restored.mediaPath(object.StorageKey), object.SHA256, object.Bytes); e != nil {
			t.Fatal(e)
		}
	}
	var refs int
	if e = restored.Pool.QueryRow(ctx, "SELECT count(*) FROM post_draft_media_refs WHERE post_id=$1 AND media_id=$2", id, media).Scan(&refs); e != nil || refs != 1 {
		t.Fatal("backup lost cover reference protection", refs, e)
	}
}
