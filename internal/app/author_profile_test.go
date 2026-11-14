package app

import (
	"context"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestAuthorPageSettingsCASIsolationAndAuthorization(t *testing.T) {
	a := testApp(t)
	p, author, csrf := testIdentity(t, a, "author")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	path := "/api/v1/me/author-page"
	initial := contentWant(t, contentCall(t, a, author, csrf, "GET", path, nil, 0, ""), 200)
	if initial["background_preset"] != "paper" || initial["background_media_id"] != nil || initial["version"] != float64(1) {
		t.Fatal("incorrect initial appearance", initial)
	}
	contentWant(t, contentCall(t, a, nil, "", "GET", path, nil, 0, ""), 401)
	input := map[string]any{"background_preset": "mist", "background_media_id": nil}
	contentWant(t, contentCall(t, a, author, csrf, "PUT", path, input, 0, ""), 428)
	contentWant(t, contentCall(t, a, author, "", "PUT", path, input, 1, ""), 403)
	key := RandID()
	saved := contentWant(t, contentCall(t, a, author, csrf, "PUT", path, input, 1, key), 200)
	if saved["version"] != float64(2) {
		t.Fatal("version not advanced", saved)
	}
	replay := contentWant(t, contentCall(t, a, author, csrf, "PUT", path, input, 1, key), 200)
	if replay["version"] != float64(2) {
		t.Fatal("replay changed result", replay)
	}
	contentWant(t, contentCall(t, a, author, csrf, "PUT", path, input, 1, ""), 409)
	input["background_preset"] = "rose"
	contentWant(t, contentCall(t, a, reader, readerCSRF, "PUT", path, input, 1, ""), 200)
	page, e := a.AuthorPage(context.Background(), p.ID)
	if e != nil || page["background_preset"] != "mist" {
		t.Fatal("another profile changed author's page", page, e)
	}
	if _, exists := page["version"]; exists {
		t.Fatal("public appearance exposed edit version", page)
	}
	var userVersion, audits int
	if e := a.Pool.QueryRow(context.Background(), `SELECT version,(SELECT count(*) FROM audit_logs WHERE actor_user_id=$1 AND action='user.update') FROM users WHERE id=$1`, p.ID).Scan(&userVersion, &audits); e != nil || userVersion != 1 || audits != 1 {
		t.Fatal("presentation save overwrote personal profile or replay audited twice", userVersion, audits, e)
	}
	for _, invalid := range []map[string]any{
		{"background_preset": "url(https://untrusted.invalid)"},
		{"background_preset": "paper", "background_media_id": "https://untrusted.invalid/a.png"},
		{"background_preset": "paper", "background_media_id": ""},
		{"background_preset": "paper", "background_url": "https://untrusted.invalid/a.png"},
	} {
		contentWant(t, contentCall(t, a, author, csrf, "PUT", path, invalid, 2, ""), 400)
	}
	contentWant(t, contentCall(t, a, author, csrf, "PUT", path, map[string]any{"background_preset": "ink", "background_media_id": RandID()}, 2, ""), 400)
	if _, e := a.Pool.Exec(context.Background(), `UPDATE users SET disabled_at_ms=$2 WHERE id=$1`, p.ID, NowMillis()); e != nil {
		t.Fatal(e)
	}
	contentWant(t, contentCall(t, a, author, csrf, "GET", path, nil, 0, ""), 401)
	if _, e := a.AuthorPage(context.Background(), p.ID); e == nil {
		t.Fatal("disabled author's appearance exposed")
	}
}

func TestAuthorPageBackgroundPublicReadDeletionAndGC(t *testing.T) {
	a := testApp(t)
	a.RegisterMedia()
	p, cookie, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	media := contentWant(t, mediaTestUpload(t, a, cookie, csrf, RandID(), 43), 201)
	id := media["media_id"].(string)
	otherMedia := contentWant(t, mediaTestUpload(t, a, other, otherCSRF, RandID(), 51), 201)["media_id"].(string)
	path := "/api/v1/me/author-page"
	for _, foreign := range []string{otherMedia, RandID()} {
		contentWant(t, contentCall(t, a, cookie, csrf, "PUT", path, map[string]any{"background_preset": "ink", "background_media_id": foreign}, 1, ""), 400)
	}
	contentWant(t, contentCall(t, a, nil, "", "GET", "/media/"+id, nil, 0, ""), 404)
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", path, map[string]any{"background_preset": "ink", "background_media_id": id}, 1, ""), 200)
	page, e := a.AuthorPage(context.Background(), p.ID)
	if e != nil || page["background_url"] != "/media/"+id {
		t.Fatal("saved image not publicly projected", page, e)
	}
	if w := contentCall(t, a, nil, "", "GET", "/media/"+id, nil, 0, ""); w.Code != 200 {
		t.Fatal("author background not readable", w.Code, w.Body.String())
	}
	if _, e := a.Pool.Exec(context.Background(), `UPDATE users SET disabled_at_ms=$2 WHERE id=$1`, p.ID, NowMillis()); e != nil {
		t.Fatal(e)
	}
	contentWant(t, contentCall(t, a, nil, "", "GET", "/media/"+id, nil, 0, ""), 404)
	if _, e := a.Pool.Exec(context.Background(), `UPDATE users SET disabled_at_ms=NULL WHERE id=$1`, p.ID); e != nil {
		t.Fatal(e)
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/media/"+id, nil, 0, ""), 409)
	status := contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/media/"+id+"/status", nil, 0, ""), 200)
	if status["in_use"] != true {
		t.Fatal("author background missing from media usage", status)
	}
	if _, e := a.Pool.Exec(context.Background(), `UPDATE media_assets SET created_at_ms=$2 WHERE id=$1`, id, NowMillis()-8*24*60*60*1000); e != nil {
		t.Fatal(e)
	}
	if e := a.ReconcileMedia(context.Background()); e != nil {
		t.Fatal(e)
	}
	var state string
	if e := a.Pool.QueryRow(context.Background(), `SELECT state FROM media_assets WHERE id=$1`, id).Scan(&state); e != nil || state != "ready" {
		t.Fatal("background collected while referenced", state, e)
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", path, map[string]any{"background_preset": "mist", "background_media_id": nil}, 2, ""), 200)
	contentWant(t, contentCall(t, a, nil, "", "GET", "/media/"+id, nil, 0, ""), 404)
	if e := a.ReconcileMedia(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := a.Pool.QueryRow(context.Background(), `SELECT state FROM media_assets WHERE id=$1`, id).Scan(&state); e != nil || state != "deleted" {
		t.Fatal("removed orphan background not collected", state, e)
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", path, map[string]any{"background_preset": "mist", "background_media_id": id}, 3, ""), 400)
}

func TestAuthorPageConcurrentInitialSaveOnlyOneWins(t *testing.T) {
	a := testApp(t)
	_, cookie, csrf := testIdentity(t, a, "author")
	responses := make(chan *httptest.ResponseRecorder, 2)
	var group sync.WaitGroup
	for _, preset := range []string{"ink", "rose"} {
		group.Add(1)
		go func(preset string) {
			defer group.Done()
			responses <- contentCall(t, a, cookie, csrf, "PUT", "/api/v1/me/author-page", map[string]any{"background_preset": preset}, 1, RandID())
		}(preset)
	}
	group.Wait()
	close(responses)
	wins, conflicts := 0, 0
	for response := range responses {
		if response.Code == 200 {
			wins++
		} else if response.Code == 409 {
			conflicts++
		} else {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("concurrent writes not serialized", wins, conflicts)
	}
}

func TestAuthorPostsSearchScopePaginationAndPublicProjection(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterSearch()
	ctx := context.Background()
	first, cookie, csrf := testIdentity(t, a, "author")
	second, other, otherCSRF := testIdentity(t, a, "author")
	create := func(otherAuthor, publish bool, title string) string {
		t.Helper()
		c, token := cookie, csrf
		if otherAuthor {
			c, token = other, otherCSRF
		}
		post := contentWant(t, contentCall(t, a, c, token, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": title, "markdown": "只搜索这个作者的可靠写作"}, 0, ""), 201)
		id := post["id"].(string)
		if publish {
			contentWant(t, contentCall(t, a, c, token, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
		}
		return id
	}
	create(false, true, "可靠写作 第一篇")
	create(false, true, "可靠写作 第二篇")
	create(false, false, "可靠写作 私人草稿")
	create(true, true, "可靠写作 别的作者")
	withdrawn := create(false, true, "可靠写作 已撤回")
	contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/studio/posts/"+withdrawn+"/publication", nil, 2, ""), 200)
	query := url.Values{"q": {"可靠写作"}, "author_id": {second.ID}, "limit": {"1"}}
	result, e := a.AuthorPosts(ctx, first.ID, query)
	if e != nil {
		t.Fatal(e)
	}
	if query.Get("author_id") != second.ID {
		t.Fatal("query arguments mutated")
	}
	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["author"].(map[string]any)["id"] != first.ID || result["has_more"] != true {
		t.Fatal("author search scope not applied", result)
	}
	query.Set("cursor", result["next_cursor"].(string))
	result, e = a.AuthorPosts(ctx, first.ID, query)
	if e != nil || len(result["items"].([]map[string]any)) != 1 || result["has_more"] != false {
		t.Fatal("scoped search pagination failed", result, e)
	}
	if _, e = a.AuthorPosts(ctx, second.ID, query); e == nil {
		t.Fatal("search cursor reused across authors")
	}
	all, e := a.AuthorPosts(ctx, first.ID, url.Values{})
	if e != nil || len(all["items"].([]map[string]any)) != 2 {
		t.Fatal("draft or withdrawn post exposed", all, e)
	}
	for _, post := range all["items"].([]map[string]any) {
		if strings.Contains(post["title"].(string), "草稿") || strings.Contains(post["title"].(string), "撤回") {
			t.Fatal("nonpublic content in author page", post)
		}
	}
}

func TestAuthorPageBackgroundBackupRestoresPresentationAndMedia(t *testing.T) {
	opsPGTools(t)
	a := testApp(t)
	a.RegisterMedia()
	p, cookie, csrf := testIdentity(t, a, "author")
	media := contentWant(t, mediaTestUpload(t, a, cookie, csrf, RandID(), 72), 201)["media_id"].(string)
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", "/api/v1/me/author-page", map[string]any{"background_preset": "rose", "background_media_id": media}, 1, ""), 200)
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "author-backup")
	report, e := a.Backup(ctx, dir)
	if e != nil || report.MediaObjects != 1 {
		t.Fatal("background omitted from backup", report, e)
	}
	manifest, e := readBackupManifest(dir)
	if e != nil || len(manifest.Media) != 1 || manifest.Media[0].ID != media || manifest.MigrationChecksums["14"] == "" {
		t.Fatal("background backup manifest incomplete", manifest, e)
	}
	admin, e := pgx.Connect(ctx, a.Config.DatabaseURL)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close(ctx)
	name := "painter_author_restore_" + RandID()[:16]
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE template0 ENCODING 'UTF8' LC_COLLATE 'C.UTF-8' LC_CTYPE 'C.UTF-8'"); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if _, e := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); e != nil {
			t.Error(e)
		}
	}()
	target, e := databaseDSN(a.Config.DatabaseURL, name)
	if e != nil {
		t.Fatal(e)
	}
	if e = pgRun(ctx, "pg_restore", target, "--no-owner", "--no-acl", "--exit-on-error", filepath.Join(dir, "database.dump")); e != nil {
		t.Fatal(e)
	}
	restored, e := pgx.Connect(ctx, target)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close(ctx)
	var preset, image, state string
	var version int
	if e := restored.QueryRow(ctx, `SELECT ap.background_preset,ap.background_media_id,ap.version,m.state FROM author_page_profiles ap JOIN media_assets m ON m.id=ap.background_media_id WHERE ap.user_id=$1`, p.ID).Scan(&preset, &image, &version, &state); e != nil || preset != "rose" || image != media || version != 2 || state != "ready" {
		t.Fatal("restored author background differs", preset, image, version, state, e)
	}
	if _, e := a.RestoreCheck(ctx, dir); e != nil {
		t.Fatal("background database and bytes failed restore verification", e)
	}
}
