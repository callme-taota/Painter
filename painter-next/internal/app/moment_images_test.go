package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func momentImageApp(t *testing.T) *App {
	t.Helper()
	a := momentTestApp(t)
	a.RegisterMedia()
	return a
}

func momentImageUpload(t *testing.T, a *App, cookie *http.Cookie, csrf string, shade byte) string {
	t.Helper()
	return momentResponse(t, mediaTestUpload(t, a, cookie, csrf, RandID(), shade), 201)["media_id"].(string)
}

func momentImageFixture(t *testing.T, a *App, cookie *http.Cookie, csrf, text string, images []momentImage) map[string]any {
	t.Helper()
	return momentResponse(t, momentCall(t, a, cookie, csrf, "POST", "/api/v1/moments", map[string]any{"text": text, "images": images}, 0, RandID()), 201)
}

func momentImageValues(t *testing.T, item map[string]any, key string) []momentImage {
	t.Helper()
	values, ok := item[key].([]any)
	if !ok {
		t.Fatalf("%s missing/non-array: %#v", key, item[key])
	}
	result := []momentImage{}
	for _, value := range values {
		image := value.(map[string]any)
		id := image["media_id"].(string)
		if image["url"] != "/media/"+id || len(image) != 3 {
			t.Fatal("unexpected/private image DTO fields", image)
		}
		result = append(result, momentImage{MediaID: id, Alt: image["alt"].(string)})
	}
	return result
}

func momentImageRead(t *testing.T, a *App, cookie *http.Cookie, id string, status int) {
	t.Helper()
	w := testRequest(a, "GET", "/media/"+id, "", cookie, "")
	if w.Code != status {
		t.Fatalf("image read status=%d want=%d: %s", w.Code, status, w.Body.String())
	}
	if status == 200 && (w.Header().Get("Content-Type") != "image/png" || w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff") {
		t.Fatal("image response safety headers", w.Header())
	}
}

func TestMomentImagesImageOnlySnapshotPrivacyAndLifecycle(t *testing.T) {
	a := momentImageApp(t)
	_, author, csrf := testIdentity(t, a, "author")
	first := momentImage{momentImageUpload(t, a, author, csrf, 1), "公开图一"}
	second := momentImage{momentImageUpload(t, a, author, csrf, 2), "公开图二"}
	replacement := momentImage{momentImageUpload(t, a, author, csrf, 3), "private-new-alt"}
	item := momentImageFixture(t, a, author, csrf, "", []momentImage{second, first})
	id := item["id"].(string)
	base := "/api/v1/studio/moments/" + id
	if item["text"] != "" || item["published"] != false || !reflect.DeepEqual(momentImageValues(t, item, "images"), []momentImage{second, first}) || len(momentImageValues(t, item, "public_images")) != 0 {
		t.Fatal("image-only draft response", item)
	}
	for _, media := range []string{first.MediaID, second.MediaID, replacement.MediaID} {
		momentImageRead(t, a, nil, media, 404)
		momentImageRead(t, a, author, media, 200)
	}
	momentResponse(t, momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 1, RandID()), 200)
	public := momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/moments/"+id, nil, 0, ""), 200)
	if !reflect.DeepEqual(momentImageValues(t, public, "images"), []momentImage{second, first}) || public["text"] != "" || public["public_images"] != nil {
		t.Fatal("public image snapshot/order", public)
	}
	momentImageRead(t, a, nil, second.MediaID, 200)
	changed := momentResponse(t, momentCall(t, a, author, csrf, "PUT", base+"/draft", map[string]any{"text": "", "images": []momentImage{replacement}}, 2, RandID()), 200)
	if changed["has_unpublished_changes"] != true || !reflect.DeepEqual(momentImageValues(t, changed, "public_images"), []momentImage{second, first}) {
		t.Fatal("draft replacement altered public snapshot", changed)
	}
	for _, path := range []string{"/api/v1/moments", "/api/v1/moments/" + id} {
		w := momentCall(t, a, author, csrf, "GET", path, nil, 0, "")
		if w.Code != 200 || strings.Contains(w.Body.String(), replacement.MediaID) || strings.Contains(w.Body.String(), "private-new-alt") {
			t.Fatal("replacement leaked before explicit publication", path, w.Body.String())
		}
	}
	momentImageRead(t, a, nil, replacement.MediaID, 404)
	momentResponse(t, momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 3, RandID()), 200)
	momentImageRead(t, a, nil, replacement.MediaID, 200)
	momentImageRead(t, a, nil, second.MediaID, 404)
	withdraw := momentResponse(t, momentCall(t, a, author, csrf, "DELETE", base+"/publication", nil, 4, RandID()), 200)
	if len(momentImageValues(t, withdraw, "public_images")) != 0 || len(momentImageValues(t, withdraw, "images")) != 1 {
		t.Fatal("withdraw dropped draft or retained public refs")
	}
	momentImageRead(t, a, nil, replacement.MediaID, 404)
	momentImageRead(t, a, author, replacement.MediaID, 200)
	momentResponse(t, momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 5, RandID()), 200)
	deleted := momentResponse(t, momentCall(t, a, author, csrf, "DELETE", base, nil, 6, RandID()), 200)
	if len(momentImageValues(t, deleted, "public_images")) != 0 || len(momentImageValues(t, deleted, "images")) != 1 {
		t.Fatal("soft-delete snapshot references")
	}
	momentImageRead(t, a, nil, replacement.MediaID, 404)
	momentImageRead(t, a, author, replacement.MediaID, 200)
}

func TestMomentImagesOmittedSaveOrderingAltAndRemoval(t *testing.T) {
	a := momentImageApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	images := []momentImage{{momentImageUpload(t, a, owner, csrf, 1), "one"}, {momentImageUpload(t, a, owner, csrf, 2), "two"}}
	item := momentImageFixture(t, a, owner, csrf, "same", images)
	base := "/api/v1/studio/moments/" + item["id"].(string)
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", base+"/publish", nil, 1, RandID()), 200)
	omitted := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", base+"/draft", map[string]any{"text": "same"}, 2, RandID()), 200)
	if !reflect.DeepEqual(momentImageValues(t, omitted, "images"), images) || omitted["has_unpublished_changes"] != false {
		t.Fatal("old text-only client silently removed images", omitted)
	}
	ordered := []momentImage{images[1], images[0]}
	changed := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", base+"/draft", map[string]any{"text": "same", "images": ordered}, 3, RandID()), 200)
	if changed["has_unpublished_changes"] != true || !reflect.DeepEqual(momentImageValues(t, changed, "public_images"), images) {
		t.Fatal("order not included in unpublished comparison", changed)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", base+"/publish", nil, 4, RandID()), 200)
	ordered[0].Alt = "alt changed, text unchanged"
	changed = momentResponse(t, momentCall(t, a, owner, csrf, "PUT", base+"/draft", map[string]any{"text": "same", "images": ordered}, 5, RandID()), 200)
	if changed["has_unpublished_changes"] != true {
		t.Fatal("alt-only edit not marked unpublished")
	}
	removed := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", base+"/draft", map[string]any{"text": "same", "images": []momentImage{}}, 6, RandID()), 200)
	if len(momentImageValues(t, removed, "images")) != 0 || len(momentImageValues(t, removed, "public_images")) != 2 {
		t.Fatal("explicit image removal was omitted or premature")
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", base+"/publish", nil, 7, RandID()), 200)
	for _, media := range images {
		momentImageRead(t, a, nil, media.MediaID, 404)
	}
	list := momentResponse(t, momentCall(t, a, owner, csrf, "GET", "/api/v1/me/moments", nil, 0, ""), 200)
	if list["max_images"] != float64(9) || list["max_image_bytes"] != float64(a.Config.UploadLimit) || list["items"].([]any)[0].(map[string]any)["max_images"] != float64(9) {
		t.Fatal("page/private limits omitted", list)
	}
	a.Config.UploadLimit = 12345
	custom := momentResponse(t, momentCall(t, a, owner, csrf, "GET", base, nil, 0, ""), 200)
	if custom["max_image_bytes"] != float64(12345) {
		t.Fatal("configured image limit ignored")
	}
	a.Config.UploadLimit = 0
	fallback := momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/moments", nil, 0, ""), 200)
	if fallback["max_image_bytes"] != float64(10<<20) {
		t.Fatal("image limit fallback wrong")
	}
}

func TestMomentImagesOwnershipAndAttachedSharedRetention(t *testing.T) {
	a := momentImageApp(t)
	_, owner, ownerCSRF := testIdentity(t, a, "owner")
	_, author, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	authorImage := momentImage{momentImageUpload(t, a, author, csrf, 1), "author"}
	ownerImage := momentImage{momentImageUpload(t, a, owner, ownerCSRF, 2), "owner"}
	foreign := momentImage{momentImageUpload(t, a, other, otherCSRF, 3), "foreign-private"}
	item := momentImageFixture(t, a, author, csrf, "author text", []momentImage{authorImage})
	base := "/api/v1/studio/moments/" + item["id"].(string)
	shared := []momentImage{authorImage, ownerImage}
	momentResponse(t, momentCall(t, a, owner, ownerCSRF, "PUT", base+"/draft", map[string]any{"text": "owner adds own image", "images": shared}, 1, RandID()), 200)
	momentImageRead(t, a, author, ownerImage.MediaID, 200)
	momentImageRead(t, a, other, ownerImage.MediaID, 404)
	shared = []momentImage{{ownerImage.MediaID, "author may edit attached alt"}, authorImage}
	retained := momentResponse(t, momentCall(t, a, author, csrf, "PUT", base+"/draft", map[string]any{"text": "author retains shared", "images": shared}, 2, RandID()), 200)
	if !reflect.DeepEqual(momentImageValues(t, retained, "images"), shared) {
		t.Fatal("author could not retain/reorder owner's attached media")
	}
	for _, actor := range []struct {
		cookie *http.Cookie
		csrf   string
	}{{owner, ownerCSRF}, {author, csrf}} {
		if w := momentCall(t, a, actor.cookie, actor.csrf, "PUT", base+"/draft", map[string]any{"text": "must rollback", "images": []momentImage{foreign}}, 3, RandID()); w.Code != 409 {
			t.Fatal("new arbitrary foreign private image attached", w.Code, w.Body.String())
		}
	}
	if w := momentCall(t, a, owner, ownerCSRF, "POST", "/api/v1/moments", map[string]any{"text": "owner cannot blind reuse", "images": []momentImage{authorImage}}, 0, RandID()); w.Code != 409 {
		t.Fatal("owner blindly attached foreign media", w.Code)
	}
	if w := momentCall(t, a, author, csrf, "POST", "/api/v1/moments", map[string]any{"text": "different moment not authorized by preview", "images": []momentImage{ownerImage}}, 0, RandID()); w.Code != 409 {
		t.Fatal("shared preview granted arbitrary new attachment", w.Code)
	}
	current := momentResponse(t, momentCall(t, a, author, csrf, "GET", base, nil, 0, ""), 200)
	if current["version"] != float64(3) || !reflect.DeepEqual(momentImageValues(t, current, "images"), shared) {
		t.Fatal("rejected attachment partially committed", current)
	}
}

func TestMomentImagesValidationBoundsAndState(t *testing.T) {
	a := momentImageApp(t)
	p, author, csrf := testIdentity(t, a, "author")
	images := []momentImage{}
	for i := range 9 {
		images = append(images, momentImage{momentImageUpload(t, a, author, csrf, byte(i)), strings.Repeat("🌿", 200)})
	}
	item := momentImageFixture(t, a, author, csrf, "", images)
	if len(momentImageValues(t, item, "images")) != 9 {
		t.Fatal("nine-image boundary rejected")
	}
	badAlt := []momentImage{{images[0].MediaID, strings.Repeat("🌿", 201)}}
	for _, body := range []any{
		map[string]any{"text": nil, "images": images},
		map[string]any{"text": "", "images": []momentImage{}},
		map[string]any{"text": "　\n\t", "images": []momentImage{}},
		map[string]any{"text": "valid", "images": nil},
		map[string]any{"text": "valid", "images": "not an array"},
		map[string]any{"text": "valid", "images": append(append([]momentImage{}, images...), images[0])},
		map[string]any{"text": "valid", "images": []momentImage{images[0], images[0]}},
		map[string]any{"text": "valid", "images": badAlt},
		map[string]any{"text": "valid", "images": []momentImage{{"../invalid", ""}}},
		map[string]any{"text": "valid", "images": []any{map[string]any{"media_id": images[0].MediaID, "url": "https://attacker.invalid/x", "alt": ""}}},
		map[string]any{"text": strings.Repeat("字", 501), "images": images},
	} {
		if w := momentCall(t, a, author, csrf, "POST", "/api/v1/moments", body, 0, RandID()); w.Code != 400 {
			t.Fatal("invalid images/body accepted", w.Code, w.Body.String())
		}
	}
	for _, fixture := range []struct{ state, mime string }{{"staging", "image/png"}, {"failed", "image/png"}, {"ready", "image/svg+xml"}} {
		id := RandID()
		if _, err := a.Pool.Exec(context.Background(), "INSERT INTO media_assets(id,owner_user_id,storage_key,original_name,mime_type,byte_size,sha256,state,created_at_ms,updated_at_ms,ready_at_ms) VALUES($1,$2,$3,'bad-image', $4,10,$5,$6,1,1,1)", id, p.ID, RandID(), fixture.mime, strings.Repeat("a", 64), fixture.state); err != nil {
			t.Fatal(err)
		}
		if w := momentCall(t, a, author, csrf, "POST", "/api/v1/moments", map[string]any{"text": "bad reference", "images": []momentImage{{id, ""}}}, 0, RandID()); w.Code != 409 {
			t.Fatal("unsupported/not-ready media attached", fixture, w.Code)
		}
	}
	base := "/api/v1/studio/moments/" + item["id"].(string)
	if w := momentCall(t, a, author, csrf, "PUT", base+"/draft", map[string]any{"text": "", "images": []momentImage{}}, 1, RandID()); w.Code != 400 {
		t.Fatal("image-only draft emptied", w.Code)
	}
	current := momentResponse(t, momentCall(t, a, author, csrf, "GET", base, nil, 0, ""), 200)
	if current["version"] != float64(1) || len(momentImageValues(t, current, "images")) != 9 {
		t.Fatal("invalid removal changed saved state")
	}
}

func TestMomentImagesIdempotencyAndCASAreAtomic(t *testing.T) {
	a := momentImageApp(t)
	_, author, csrf := testIdentity(t, a, "author")
	one := momentImage{momentImageUpload(t, a, author, csrf, 1), "one"}
	two := momentImage{momentImageUpload(t, a, author, csrf, 2), "two"}
	key := RandID()
	body := map[string]any{"text": "initial", "images": []momentImage{one}}
	created := momentResponse(t, momentCall(t, a, author, csrf, "POST", "/api/v1/moments", body, 0, key), 201)
	replayed := momentResponse(t, momentCall(t, a, author, csrf, "POST", "/api/v1/moments", body, 0, key), 201)
	if !reflect.DeepEqual(created, replayed) {
		t.Fatal("image create retry changed response/identity")
	}
	if w := momentCall(t, a, author, csrf, "POST", "/api/v1/moments", map[string]any{"text": "initial", "images": []momentImage{two}}, 0, key); w.Code != 409 {
		t.Fatal("create key reused with different image set", w.Code)
	}
	id := created["id"].(string)
	base := "/api/v1/studio/moments/" + id
	momentResponse(t, momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 1, RandID()), 200)
	start := make(chan struct{})
	done := make(chan *httptest.ResponseRecorder, 2)
	for _, image := range []momentImage{one, two} {
		go func(image momentImage) {
			<-start
			done <- momentCall(t, a, author, csrf, "PUT", base+"/draft", map[string]any{"text": image.Alt, "images": []momentImage{image}}, 2, RandID())
		}(image)
	}
	close(start)
	statuses := map[int]int{}
	var winning map[string]any
	for range 2 {
		w := <-done
		statuses[w.Code]++
		if w.Code == 200 {
			winning = jsonMap(t, w)
		}
	}
	if statuses[200] != 1 || statuses[409] != 1 {
		t.Fatal("image CAS accepted conflicting saves", statuses)
	}
	current := momentResponse(t, momentCall(t, a, author, csrf, "GET", base, nil, 0, ""), 200)
	if current["text"] != winning["text"] || !reflect.DeepEqual(current["images"], winning["images"]) || current["version"] != float64(3) {
		t.Fatal("winning text and image set were not atomic", current, winning)
	}
	public := momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/moments/"+id, nil, 0, ""), 200)
	if public["text"] != "initial" || !reflect.DeepEqual(momentImageValues(t, public, "images"), []momentImage{one}) {
		t.Fatal("concurrent draft save altered public image snapshot")
	}
	pubKey := RandID()
	first := momentResponse(t, momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 3, pubKey), 200)
	again := momentResponse(t, momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 3, pubKey), 200)
	if !reflect.DeepEqual(first, again) || !reflect.DeepEqual(first["public_images"], winning["images"]) {
		t.Fatal("image publication replay changed snapshot")
	}
}

func TestMomentImagesProtectMediaDeleteAndReconciler(t *testing.T) {
	a := momentImageApp(t)
	_, author, csrf := testIdentity(t, a, "author")
	old := momentImage{momentImageUpload(t, a, author, csrf, 1), "old public"}
	current := momentImage{momentImageUpload(t, a, author, csrf, 2), "new private"}
	item := momentImageFixture(t, a, author, csrf, "", []momentImage{old})
	base := "/api/v1/studio/moments/" + item["id"].(string)
	momentResponse(t, momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 1, RandID()), 200)
	momentResponse(t, momentCall(t, a, author, csrf, "PUT", base+"/draft", map[string]any{"text": "", "images": []momentImage{current}}, 2, RandID()), 200)
	ctx := context.Background()
	for _, image := range []momentImage{old, current} {
		if _, err := a.Pool.Exec(ctx, "UPDATE media_assets SET created_at_ms=$2 WHERE id=$1", image.MediaID, NowMillis()-8*24*60*60*1000); err != nil {
			t.Fatal(err)
		}
		if w := momentCall(t, a, author, csrf, "DELETE", "/api/v1/media/"+image.MediaID, nil, 0, ""); w.Code != 409 {
			t.Fatal("referenced image manually deleted", w.Code)
		}
		if _, err := a.Pool.Exec(ctx, "UPDATE media_assets SET state='deleting' WHERE id=$1", image.MediaID); err == nil {
			t.Fatal("DB guard allowed referenced media to enter deleting")
		}
	}
	if err := a.ReconcileMedia(ctx); err != nil {
		t.Fatal(err)
	}
	for _, image := range []momentImage{old, current} {
		var state string
		if err := a.Pool.QueryRow(ctx, "SELECT state FROM media_assets WHERE id=$1", image.MediaID).Scan(&state); err != nil || state != "ready" {
			t.Fatal("GC ignored draft/public refs", state, err)
		}
	}
	momentResponse(t, momentCall(t, a, author, csrf, "DELETE", base+"/publication", nil, 3, RandID()), 200)
	if err := a.ReconcileMedia(ctx); err != nil {
		t.Fatal(err)
	}
	var oldState, currentState string
	if err := a.Pool.QueryRow(ctx, "SELECT (SELECT state FROM media_assets WHERE id=$1),(SELECT state FROM media_assets WHERE id=$2)", old.MediaID, current.MediaID).Scan(&oldState, &currentState); err != nil || oldState != "deleted" || currentState != "ready" {
		t.Fatal("released snapshot/media GC mismatch", oldState, currentState, err)
	}
	momentResponse(t, momentCall(t, a, author, csrf, "DELETE", base, nil, 4, RandID()), 200)
	if err := a.ReconcileMedia(ctx); err != nil {
		t.Fatal(err)
	}
	momentImageRead(t, a, author, current.MediaID, 200)
	if w := momentCall(t, a, author, csrf, "DELETE", "/api/v1/media/"+current.MediaID, nil, 0, ""); w.Code != 409 {
		t.Fatal("soft-deleted draft image was unprotected", w.Code)
	}
}

func TestMomentImagesDeferredNonemptyIntegrity(t *testing.T) {
	a := momentImageApp(t)
	_, author, csrf := testIdentity(t, a, "author")
	image := momentImage{momentImageUpload(t, a, author, csrf, 1), "alt"}
	item := momentImageFixture(t, a, author, csrf, "", []momentImage{image})
	id := item["id"].(string)
	ctx := context.Background()
	if _, err := a.Pool.Exec(ctx, "DELETE FROM moment_images WHERE moment_id=$1 AND scope='draft'", id); err == nil {
		t.Fatal("deferred guard allowed empty image-only draft")
	}
	if _, err := a.Pool.Exec(ctx, "INSERT INTO moment_images(moment_id,scope,position,media_id,alt) VALUES($1,'public',0,$2,'not yet published')", id, image.MediaID); err == nil {
		t.Fatal("public refs without a publication accepted")
	}
	for _, values := range []struct {
		position int
		alt      string
	}{{9, ""}, {1, strings.Repeat("字", 201)}} {
		if _, err := a.Pool.Exec(ctx, "INSERT INTO moment_images(moment_id,scope,position,media_id,alt) VALUES($1,'draft',$2,$3,$4)", id, values.position, RandID(), values.alt); err == nil {
			t.Fatal("invalid relational image bounds accepted")
		}
	}
	var count int
	if err := a.Pool.QueryRow(ctx, "SELECT count(*) FROM moment_images WHERE moment_id=$1 AND scope='draft'", id).Scan(&count); err != nil || count != 1 {
		t.Fatal("failed deferred transaction damaged image refs", count, err)
	}
}

func momentImageWaitForLock(t *testing.T, a *App, fragment string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		if err := a.Pool.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE $1)", "%"+fragment+"%").Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("request did not reach expected lock", fragment)
}

func TestMomentImagesAttachWinsDeleteRace(t *testing.T) {
	a := momentImageApp(t)
	_, author, csrf := testIdentity(t, a, "author")
	image := momentImage{momentImageUpload(t, a, author, csrf, 1), "race image"}
	item := momentFixture(t, a, author, csrf, "base", false)
	base := "/api/v1/studio/moments/" + item["id"].(string)
	ctx := context.Background()
	if _, err := a.Pool.Exec(ctx, `CREATE FUNCTION painter_test_pause_moment_image() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN PERFORM pg_advisory_xact_lock(943219010); RETURN NEW; END $$;
 CREATE TRIGGER painter_test_pause_moment_image BEFORE INSERT ON moment_images FOR EACH ROW EXECUTE FUNCTION painter_test_pause_moment_image()`); err != nil {
		t.Fatal(err)
	}
	gate, err := a.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Rollback(ctx)
	if _, err := gate.Exec(ctx, "SELECT pg_advisory_xact_lock(943219010)"); err != nil {
		t.Fatal(err)
	}
	saved, removed := make(chan *httptest.ResponseRecorder, 1), make(chan *httptest.ResponseRecorder, 1)
	go func() {
		saved <- momentCall(t, a, author, csrf, "PUT", base+"/draft", map[string]any{"text": "base", "images": []momentImage{image}}, 1, RandID())
	}()
	momentImageWaitForLock(t, a, "INSERT INTO moment_images")
	go func() {
		removed <- momentCall(t, a, author, csrf, "DELETE", "/api/v1/media/"+image.MediaID, nil, 0, "")
	}()
	momentImageWaitForLock(t, a, "owner_user_id::text,state FROM media_assets")
	if err := gate.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case w := <-saved:
		momentResponse(t, w, 200)
	case <-time.After(5 * time.Second):
		t.Fatal("image save stuck")
	}
	select {
	case w := <-removed:
		if w.Code != 409 {
			t.Fatal("delete did not recheck references after queued media lock", w.Code, w.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued delete stuck")
	}
	momentImageRead(t, a, author, image.MediaID, 200)
}

func TestMomentImagesDeleteWinsAttachRace(t *testing.T) {
	a := momentImageApp(t)
	_, author, csrf := testIdentity(t, a, "author")
	image := momentImage{momentImageUpload(t, a, author, csrf, 1), "race image"}
	item := momentFixture(t, a, author, csrf, "base", false)
	base := "/api/v1/studio/moments/" + item["id"].(string)
	ctx := context.Background()
	gate, err := a.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Rollback(ctx)
	if _, err := gate.Exec(ctx, "SELECT id FROM media_assets WHERE id=$1 FOR UPDATE", image.MediaID); err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- momentCall(t, a, author, csrf, "PUT", base+"/draft", map[string]any{"text": "must rollback", "images": []momentImage{image}}, 1, RandID())
	}()
	momentImageWaitForLock(t, a, "owner_user_id::text,state,mime_type FROM media_assets")
	if _, err := gate.Exec(ctx, "UPDATE media_assets SET state='deleting' WHERE id=$1", image.MediaID); err != nil {
		t.Fatal(err)
	}
	if err := gate.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case w := <-done:
		if w.Code != 409 {
			t.Fatal("attach retained stale ready state after deletion lock", w.Code, w.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued attachment stuck")
	}
	current := momentResponse(t, momentCall(t, a, author, csrf, "GET", base, nil, 0, ""), 200)
	if current["version"] != float64(1) || current["text"] != "base" || len(momentImageValues(t, current, "images")) != 0 {
		t.Fatal("failed attachment partially committed", current)
	}
}

func TestMomentImagesMissingFilePublishRollsBack(t *testing.T) {
	a := momentImageApp(t)
	_, author, csrf := testIdentity(t, a, "author")
	image := momentImage{momentImageUpload(t, a, author, csrf, 1), "missing file"}
	item := momentImageFixture(t, a, author, csrf, "", []momentImage{image})
	base := "/api/v1/studio/moments/" + item["id"].(string)
	var storage string
	if err := a.Pool.QueryRow(context.Background(), "SELECT storage_key FROM media_assets WHERE id=$1", image.MediaID).Scan(&storage); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(a.mediaPath(storage)); err != nil {
		t.Fatal(err)
	}
	w := momentCall(t, a, author, csrf, "POST", base+"/publish", nil, 1, RandID())
	if w.Code != 503 || !strings.Contains(w.Body.String(), "STORAGE_UNAVAILABLE") {
		t.Fatal("missing image was published", w.Code, w.Body.String())
	}
	current := momentResponse(t, momentCall(t, a, author, csrf, "GET", base, nil, 0, ""), 200)
	if current["version"] != float64(1) || current["published"] != false || len(momentImageValues(t, current, "public_images")) != 0 || len(momentImageValues(t, current, "images")) != 1 {
		t.Fatal("failed publication changed saved state", current)
	}
}

func TestMomentImagesConcurrentPublishUsesOnlyItsTransactionConnection(t *testing.T) {
	a := momentImageApp(t)
	_, author, csrf := testIdentity(t, a, "author")
	images, items := []momentImage{}, []map[string]any{}
	for i := range 2 {
		image := momentImage{momentImageUpload(t, a, author, csrf, byte(i)), "pool limit"}
		images = append(images, image)
		items = append(items, momentImageFixture(t, a, author, csrf, "", []momentImage{image}))
	}
	ctx := context.Background()
	config := a.Pool.Config()
	config.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	old := a.Pool
	a.Pool = pool
	old.Close()
	// The gate/observer must be independent of the bounded application pool.
	connection, err := pgx.ConnectConfig(ctx, config.ConnConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(ctx)
	gate, err := connection.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Rollback(ctx)
	if _, err := gate.Exec(ctx, "SELECT id FROM media_assets WHERE id=ANY($1::text[]) ORDER BY id FOR UPDATE", []string{images[0].MediaID, images[1].MediaID}); err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 2)
	for _, item := range items {
		go func(id string) {
			requestCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
			defer cancel()
			r := httptest.NewRequest("POST", "/api/v1/studio/moments/"+id+"/publish", nil).WithContext(requestCtx)
			r.AddCookie(author)
			r.Header.Set("Origin", a.Config.PublicBaseURL)
			r.Header.Set("X-CSRF-Token", csrf)
			r.Header.Set("Idempotency-Key", RandID())
			r.Header.Set("If-Match", strconv.Quote("1"))
			w := httptest.NewRecorder()
			a.Handler().ServeHTTP(w, r)
			done <- w
		}(item["id"].(string))
	}
	deadline, waiting := time.Now().Add(2*time.Second), 0
	for time.Now().Before(deadline) {
		// The gate is a long-running transaction; clear its statistics snapshot
		// so this observer sees publishers that started after the first poll.
		if _, err := gate.Exec(ctx, "SELECT pg_stat_clear_snapshot()"); err != nil {
			t.Fatal(err)
		}
		if err := gate.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT owner_user_id::text,state,mime_type FROM media_assets%'").Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if waiting != 2 {
		t.Fatal("two publishers did not fill the bounded pool", waiting)
	}
	if err := gate.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case w := <-done:
			momentResponse(t, w, 200)
		case <-time.After(5 * time.Second):
			t.Fatal("publish waited for an extra pool connection")
		}
	}
}
