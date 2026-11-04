package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func mediaTestUpload(t *testing.T, a *App, c *http.Cookie, csrf, key string, shade uint8) *httptest.ResponseRecorder {
	t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	part, e := mw.CreateFormFile("file", "test.png")
	if e != nil {
		t.Fatal(e)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: shade, A: 255})
	if e = png.Encode(part, img); e != nil {
		t.Fatal(e)
	}
	mw.Close()
	r := httptest.NewRequest("POST", "/api/v1/media", &b)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("Origin", a.Config.PublicBaseURL)
	r.Header.Set("X-CSRF-Token", csrf)
	r.Header.Set("Idempotency-Key", key)
	r.AddCookie(c)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}

func TestMediaCrashRecoveryAndConservativeGC(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	p, _, _ := testIdentity(t, a, "owner")
	if e := os.MkdirAll(a.mediaPath(""), 0700); e != nil {
		t.Fatal(e)
	}
	bytes := []byte("durable image bytes already validated before staging")
	hash := sha256.Sum256(bytes)
	sum := hex.EncodeToString(hash[:])
	readyID, missingID := RandID(), RandID()
	readyKey := RandID()
	now := NowMillis()
	for _, v := range []struct{ id, key string }{{readyID, readyKey}, {missingID, RandID()}} {
		if _, e := a.Pool.Exec(ctx, `INSERT INTO media_assets(id,owner_user_id,storage_key,original_name,mime_type,byte_size,sha256,state,created_at_ms,updated_at_ms) VALUES($1,$2,$3,'crash.png','image/png',$4,$5,'staging',$6,$6)`, v.id, p.ID, v.key, len(bytes), sum, now-2*60*60*1000); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(a.mediaPath(readyKey), bytes, 0600); e != nil {
		t.Fatal(e)
	}
	if e := a.ReconcileMedia(ctx); e != nil {
		t.Fatal(e)
	}
	var ready, missing string
	if e := a.Pool.QueryRow(ctx, `SELECT (SELECT state FROM media_assets WHERE id=$1),(SELECT state FROM media_assets WHERE id=$2)`, readyID, missingID).Scan(&ready, &missing); e != nil || ready != "ready" || missing != "failed" {
		t.Fatal(ready, missing, e)
	}
	if _, e := a.Pool.Exec(ctx, `UPDATE media_assets SET created_at_ms=$1 WHERE id=$2`, now-8*24*60*60*1000, readyID); e != nil {
		t.Fatal(e)
	}
	if e := a.ReconcileMedia(ctx); e != nil {
		t.Fatal(e)
	}
	if e := a.Pool.QueryRow(ctx, `SELECT state FROM media_assets WHERE id=$1`, readyID).Scan(&ready); e != nil || ready != "deleted" {
		t.Fatal(ready, e)
	}
	if _, e := os.Stat(a.mediaPath(readyKey)); !os.IsNotExist(e) {
		t.Fatal("orphan bytes survived GC", e)
	}
}
func TestMediaDurabilityAuthorizationAndHistoryProtection(t *testing.T) {
	a := testApp(t)
	a.RegisterMedia()
	a.RegisterContent()
	p, c, csrf := testIdentity(t, a, "owner")
	_, other, othercsrf := testIdentity(t, a, "author")
	key := RandID()
	upload := contentWant(t, mediaTestUpload(t, a, c, csrf, key, 30), 201)
	id := upload["media_id"].(string)
	again := contentWant(t, mediaTestUpload(t, a, c, csrf, key, 30), 201)
	if again["media_id"] != id {
		t.Fatal("upload duplicated")
	}
	w := mediaTestUpload(t, a, c, csrf, key, 31)
	if w.Code != 409 {
		t.Fatal("same key different bytes accepted", w.Code, w.Body.String())
	}
	w = contentCall(t, a, nil, "", "GET", "/media/"+id, nil, 0, "")
	if w.Code != 404 {
		t.Fatal("private upload exposed", w.Code)
	}
	w = contentCall(t, a, c, csrf, "GET", "/media/"+id, nil, 0, "")
	if w.Code != 200 || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal(w.Code)
	}
	w = contentCall(t, a, other, othercsrf, "GET", "/media/"+id, nil, 0, "")
	if w.Code != 404 {
		t.Fatal("other author read upload", w.Code)
	}
	d := map[string]any{"client_draft_id": RandID(), "title": "图片", "markdown": "![图](/media/" + id + ")", "media_ids": []string{id}}
	created := contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/posts", d, 0, ""), 201)
	post := created["id"].(string)
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+post+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	w = contentCall(t, a, nil, "", "GET", "/media/"+id, nil, 0, "")
	if w.Code != 200 {
		t.Fatal("published media unavailable", w.Code, w.Body.String())
	}
	w = contentCall(t, a, c, csrf, "DELETE", "/api/v1/media/"+id, nil, 0, "")
	if w.Code != 409 {
		t.Fatal("referenced media deletion accepted", w.Code, w.Body.String())
	}
	contentWant(t, contentCall(t, a, c, csrf, "DELETE", "/api/v1/studio/posts/"+post+"/publication", nil, 2, ""), 200)
	w = contentCall(t, a, nil, "", "GET", "/media/"+id, nil, 0, "")
	if w.Code != 404 {
		t.Fatal("withdrawn image still public", w.Code)
	}
	w = contentCall(t, a, c, csrf, "DELETE", "/api/v1/media/"+id, nil, 0, "")
	if w.Code != 409 {
		t.Fatal("historical reference ignored", w.Code, w.Body.String())
	}
	second := contentWant(t, mediaTestUpload(t, a, c, csrf, RandID(), 90), 201)
	unreferenced := second["media_id"].(string)
	var storage string
	if e := a.Pool.QueryRow(context.Background(), `SELECT storage_key FROM media_assets WHERE id=$1 AND owner_user_id=$2`, unreferenced, p.ID).Scan(&storage); e != nil {
		t.Fatal(e)
	}
	w = contentCall(t, a, c, csrf, "DELETE", "/api/v1/media/"+unreferenced, nil, 0, "")
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, e := os.Stat(a.mediaPath(storage)); !os.IsNotExist(e) {
		t.Fatal("deleted bytes remain", e)
	}
}
