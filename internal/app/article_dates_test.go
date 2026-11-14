package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func articleDateMillis(t *testing.T, value map[string]any, key string) int64 {
	t.Helper()
	raw, ok := value[key].(string)
	if !ok {
		t.Fatalf("%s is not a real timestamp: %#v", key, value[key])
	}
	parsed, e := time.Parse(time.RFC3339Nano, raw)
	if e != nil {
		t.Fatalf("invalid %s timestamp %q: %v", key, raw, e)
	}
	return parsed.UnixMilli()
}

func articleDateAfter(t *testing.T, previous int64) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for NowMillis() <= previous {
		if time.Now().After(deadline) {
			t.Fatal("test clock did not advance beyond the previous saved timestamp")
		}
		time.Sleep(time.Millisecond)
	}
}

func articleDateWantDisplay(t *testing.T, value map[string]any, want ArticleDateDisplay) {
	t.Helper()
	got, ok := value["date_display"].(map[string]any)
	expected := map[string]any{"created": want.Created, "updated": want.Updated, "published": want.Published}
	if !ok || !reflect.DeepEqual(got, expected) {
		t.Fatalf("date display changed: got %#v, want %#v", value["date_display"], expected)
	}
}

func articleDateWantPublicSnapshot(t *testing.T, value, previous map[string]any) {
	t.Helper()
	for _, key := range []string{"title", "revision_id", "created_at", "updated_at", "published_at", "date_display"} {
		if !reflect.DeepEqual(value[key], previous[key]) {
			t.Fatalf("unpublished work changed public %s: got %#v, want %#v", key, value[key], previous[key])
		}
	}
}

func TestArticleDatesPublicationSnapshotsAndRestore(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	_, cookie, csrf := testIdentity(t, a, "author")
	ctx := context.Background()
	draft := map[string]any{"client_draft_id": RandID(), "title": "日期快照第一稿", "markdown": "最初保存的正文"}
	createStarted := NowMillis()
	created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", draft, 0, ""), 201)
	createFinished := NowMillis()
	id := created["id"].(string)
	studioPath, publicPath := "/api/v1/studio/posts/"+id, "/api/v1/posts/"+id
	private := func() map[string]any {
		return contentWant(t, contentCall(t, a, cookie, csrf, "GET", studioPath, nil, 0, ""), 200)
	}
	public := func() map[string]any {
		return contentWant(t, contentCall(t, a, nil, "", "GET", publicPath, nil, 0, ""), 200)
	}
	initial := private()
	articleDateWantDisplay(t, initial, ArticleDateDisplay{Created: true, Updated: true, Published: true})
	createdAt := articleDateMillis(t, initial, "created_at")
	initialEdit := articleDateMillis(t, initial, "last_edited_at")
	if createdAt < createStarted || createdAt > createFinished || initialEdit < createdAt || initialEdit > createFinished {
		t.Fatal("create did not record its real creation and draft-save times", createdAt, initialEdit, createStarted, createFinished)
	}

	firstDisplay := ArticleDateDisplay{Created: false, Updated: true, Published: false}
	articleDateAfter(t, initialEdit)
	draft["date_display"], draft["expected_draft_version"] = firstDisplay, 1
	draft["markdown"] = "第一次发布前保存的正文"
	saveStarted := NowMillis()
	saved := contentWant(t, contentCall(t, a, cookie, csrf, "PUT", studioPath+"/draft", draft, 1, ""), 200)
	saveFinished := NowMillis()
	version, draftVersion := contentNumber(saved, "version"), contentNumber(saved, "draft_version")
	firstSaved := private()
	firstEdit := articleDateMillis(t, firstSaved, "last_edited_at")
	if firstEdit <= initialEdit || firstEdit < saveStarted || firstEdit > saveFinished || articleDateMillis(t, firstSaved, "created_at") != createdAt {
		t.Fatal("save confused the creation and edit times", firstSaved)
	}
	articleDateWantDisplay(t, firstSaved, firstDisplay)

	articleDateAfter(t, firstEdit)
	publishStarted := NowMillis()
	published := contentWant(t, contentCall(t, a, cookie, csrf, "POST", studioPath+"/publish", map[string]any{"expected_draft_version": draftVersion}, version, ""), 200)
	publishFinished := NowMillis()
	version = contentNumber(published, "version")
	firstRevision := published["revision_id"].(string)
	firstPublic := public()
	firstPublishedAt := articleDateMillis(t, firstPublic, "published_at")
	if articleDateMillis(t, firstPublic, "created_at") != createdAt || articleDateMillis(t, firstPublic, "updated_at") != firstEdit || firstPublishedAt <= firstEdit || firstPublishedAt < publishStarted || firstPublishedAt > publishFinished {
		t.Fatal("publication replaced creation or edit time with publication time", firstPublic)
	}
	articleDateWantDisplay(t, firstPublic, firstDisplay)
	var revisionCreated, revisionEdited, pointerPublished int64
	if e := a.Pool.QueryRow(ctx, `SELECT r.created_at_ms,r.content_updated_at_ms,p.published_at_ms FROM post_revisions r JOIN post_publications p ON p.revision_id=r.id WHERE r.id=$1`, firstRevision).Scan(&revisionCreated, &revisionEdited, &pointerPublished); e != nil {
		t.Fatal(e)
	}
	if revisionEdited != firstEdit || revisionCreated <= firstEdit || pointerPublished != firstPublishedAt {
		t.Fatal("published timestamps are not backed by the saved draft and publication pointer", revisionCreated, revisionEdited, pointerPublished)
	}

	secondDisplay := ArticleDateDisplay{Created: true, Updated: false, Published: true}
	articleDateAfter(t, firstPublishedAt)
	draft["title"], draft["markdown"] = "尚未发布的第二稿", "仍然只属于草稿的正文"
	draft["date_display"], draft["expected_draft_version"] = secondDisplay, draftVersion
	saved = contentWant(t, contentCall(t, a, cookie, csrf, "PUT", studioPath+"/draft", draft, version, ""), 200)
	version, draftVersion = contentNumber(saved, "version"), contentNumber(saved, "draft_version")
	secondSaved := private()
	secondEdit := articleDateMillis(t, secondSaved, "last_edited_at")
	if secondEdit <= firstPublishedAt {
		t.Fatal("second save did not advance the actual draft edit time", secondSaved)
	}
	articleDateWantDisplay(t, secondSaved, secondDisplay)
	articleDateWantPublicSnapshot(t, public(), firstPublic)
	listed := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts", nil, 0, ""), 200)
	if items := listed["items"].([]any); len(items) != 1 {
		t.Fatal("published article missing from list", listed)
	} else {
		articleDateWantPublicSnapshot(t, items[0].(map[string]any), firstPublic)
	}

	articleDateAfter(t, secondEdit)
	checkpoint := contentWant(t, contentCall(t, a, cookie, csrf, "POST", studioPath+"/checkpoints", map[string]any{"expected_draft_version": draftVersion}, version, ""), 200)
	version = contentNumber(checkpoint, "version")
	checkpointRevision := contentWant(t, contentCall(t, a, cookie, csrf, "GET", studioPath+"/revisions/"+checkpoint["revision_id"].(string), nil, 0, ""), 200)
	articleDateWantDisplay(t, checkpointRevision, secondDisplay)
	if articleDateMillis(t, checkpointRevision, "updated_at") != secondEdit || articleDateMillis(t, checkpointRevision, "created_at") <= secondEdit || articleDateMillis(t, private(), "last_edited_at") != secondEdit {
		t.Fatal("checkpoint invented a draft edit or lost its source edit time", checkpointRevision)
	}
	articleDateWantPublicSnapshot(t, public(), firstPublic)

	// An old client submits its usual complete draft without the new option.
	articleDateAfter(t, secondEdit)
	delete(draft, "date_display")
	draft["markdown"], draft["expected_draft_version"] = "旧客户端再次保存正文", draftVersion
	saved = contentWant(t, contentCall(t, a, cookie, csrf, "PUT", studioPath+"/draft", draft, version, ""), 200)
	version, draftVersion = contentNumber(saved, "version"), contentNumber(saved, "draft_version")
	latestSaved := private()
	latestEdit := articleDateMillis(t, latestSaved, "last_edited_at")
	articleDateWantDisplay(t, latestSaved, secondDisplay)
	if latestEdit <= secondEdit {
		t.Fatal("old-client save did not preserve a real new edit time", latestSaved)
	}
	articleDateWantPublicSnapshot(t, public(), firstPublic)

	articleDateAfter(t, latestEdit)
	republished := contentWant(t, contentCall(t, a, cookie, csrf, "POST", studioPath+"/publish", map[string]any{"expected_draft_version": draftVersion}, version, ""), 200)
	version = contentNumber(republished, "version")
	secondPublic := public()
	if secondPublic["revision_id"] == firstRevision || articleDateMillis(t, secondPublic, "created_at") != createdAt || articleDateMillis(t, secondPublic, "updated_at") != latestEdit || articleDateMillis(t, secondPublic, "published_at") <= firstPublishedAt || articleDateMillis(t, secondPublic, "published_at") <= latestEdit {
		t.Fatal("republish did not advance the publication with the latest saved edit and original creation", secondPublic)
	}
	articleDateWantDisplay(t, secondPublic, secondDisplay)

	articleDateAfter(t, articleDateMillis(t, secondPublic, "published_at"))
	restored := contentWant(t, contentCall(t, a, cookie, csrf, "POST", studioPath+"/restore", map[string]any{"revision_id": firstRevision}, version, ""), 200)
	version, draftVersion = contentNumber(restored, "version"), contentNumber(restored, "draft_version")
	restoredDraft := private()
	articleDateWantDisplay(t, restoredDraft, firstDisplay)
	if restoredDraft["title"] != firstPublic["title"] || articleDateMillis(t, restoredDraft, "created_at") != createdAt || articleDateMillis(t, restoredDraft, "last_edited_at") <= latestEdit {
		t.Fatal("restore did not restore a private working copy with its date choices", restoredDraft)
	}
	articleDateWantPublicSnapshot(t, public(), secondPublic)
	oldRevision := contentWant(t, contentCall(t, a, cookie, csrf, "GET", studioPath+"/revisions/"+firstRevision, nil, 0, ""), 200)
	articleDateWantDisplay(t, oldRevision, firstDisplay)
	if articleDateMillis(t, oldRevision, "updated_at") != firstEdit || articleDateMillis(t, oldRevision, "created_at") != revisionCreated {
		t.Fatal("restore rewrote historical revision timestamps", oldRevision)
	}

	// Omission also preserves the choices brought back by restore.
	draft["title"], draft["markdown"], draft["expected_draft_version"] = restoredDraft["title"], restoredDraft["markdown"], draftVersion
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", studioPath+"/draft", draft, version, ""), 200)
	articleDateWantDisplay(t, private(), firstDisplay)
	articleDateWantPublicSnapshot(t, public(), secondPublic)
}

func TestArticleDatesRejectInvalidJSONAndClientTimestamps(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	_, cookie, csrf := testIdentity(t, a, "author")
	created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "验证日期输入", "markdown": "正文"}, 0, ""), 201)
	id := created["id"].(string)
	path := "/api/v1/studio/posts/" + id
	before := contentWant(t, contentCall(t, a, cookie, csrf, "GET", path, nil, 0, ""), 200)
	cases := []struct {
		name, key string
		value     any
	}{
		{"string boolean", "date_display", map[string]any{"created": true, "updated": "false", "published": true}},
		{"unknown display key", "date_display", map[string]any{"created": true, "updated": true, "published": true, "edited": false}},
		{"array display", "date_display", []bool{true, false, true}},
		{"created override", "created_at", "2001-01-01T00:00:00Z"},
		{"updated override", "updated_at", "2001-01-01T00:00:00Z"},
		{"published override", "published_at", "2001-01-01T00:00:00Z"},
		{"revision edit override", "content_updated_at_ms", int64(1)},
		{"internal edit flag", "unknown_edited_at", true},
		{"internal Go field", "UnknownEditedAt", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"client_draft_id": RandID(), "title": "无效日期不得落库", "markdown": "正文", tc.key: tc.value}
			contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", body, 0, ""), 400)
			delete(body, "client_draft_id")
			body["expected_draft_version"] = 1
			contentWant(t, contentCall(t, a, cookie, csrf, "PUT", path+"/draft", body, 1, ""), 400)
		})
	}
	after := contentWant(t, contentCall(t, a, cookie, csrf, "GET", path, nil, 0, ""), 200)
	for _, key := range []string{"version", "draft_version", "title", "created_at", "updated_at", "last_edited_at", "date_display"} {
		if !reflect.DeepEqual(after[key], before[key]) {
			t.Fatalf("rejected date input mutated %s: got %#v, want %#v", key, after[key], before[key])
		}
	}
	var count int
	if e := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM posts").Scan(&count); e != nil || count != 1 {
		t.Fatal("rejected date input created an article", count, e)
	}
}

func TestArticleDateLegacyImportDoesNotUsePrivateEdit(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	path, packet := legacyFixture(t)
	post := &packet.Posts[0]
	publishedAt := *post.PublishedAtMS
	createdAt, privateEdit := publishedAt-86_400_000, publishedAt+86_400_000
	post.CreatedAtMS, post.UpdatedAtMS = createdAt, privateEdit
	post.Draft.Title, post.Draft.Markdown = "后来保存的私有草稿", "公开发布以后才写入的私有内容\n\n"+post.Draft.Markdown
	publicDisplay := ArticleDateDisplay{Created: true, Updated: true, Published: false}
	privateDisplay := ArticleDateDisplay{Created: false, Updated: false, Published: true}
	post.Publication.DateDisplay, post.Draft.DateDisplay = &publicDisplay, &privateDisplay
	writeLegacy(t, path, packet)
	ctx := context.Background()
	report, e := a.ImportLegacy(ctx, path, false)
	if e != nil || !report.Committed {
		t.Fatal("legacy date fixture did not import", report, e)
	}
	public := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+post.ID, nil, 0, ""), 200)
	if updated, exists := public["updated_at"]; !exists || updated != nil {
		t.Fatal("legacy publication used a private or guessed edit time", public)
	}
	if public["title"] != post.Publication.Title || articleDateMillis(t, public, "created_at") != createdAt || articleDateMillis(t, public, "published_at") != publishedAt {
		t.Fatal("unknown public edit time replaced the actual legacy creation or publication date", public)
	}
	articleDateWantDisplay(t, public, publicDisplay)
	private, e := a.privatePost(ctx, a.Pool, Principal{ID: post.AuthorID, Role: "author"}, post.ID)
	if e != nil {
		t.Fatal(e)
	}
	if private["title"] != post.Draft.Title || articleDateMillis(t, private, "last_edited_at") != privateEdit || articleDateMillis(t, private, "created_at") != createdAt {
		t.Fatal("legacy import lost the real private draft date or changed creation", private)
	}
	articleDateWantDisplay(t, private, privateDisplay)
	var snapshotEdited *int64
	if e = a.Pool.QueryRow(ctx, "SELECT content_updated_at_ms FROM post_revisions WHERE id=$1", public["revision_id"]).Scan(&snapshotEdited); e != nil || snapshotEdited != nil {
		t.Fatal("legacy public revision did not preserve an unknown edit timestamp", snapshotEdited, e)
	}
}

func TestArticleDatesDatabaseSnapshotAndImmutabilityGuards(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	_, cookie, csrf := testIdentity(t, a, "author")
	created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "数据库日期守卫", "markdown": "正文", "date_display": ArticleDateDisplay{Created: false, Updated: true, Published: false}}, 0, ""), 201)
	id := created["id"].(string)
	published := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	rid := published["revision_id"].(string)
	ctx := context.Background()
	publicBefore := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	wantCheckViolation := func(e error, message string) {
		t.Helper()
		var pgError *pgconn.PgError
		if !errors.As(e, &pgError) || pgError.Code != "23514" || !strings.Contains(pgError.Message, message) {
			t.Fatalf("date guard did not reject the write for %q: %v", message, e)
		}
	}
	for _, mutation := range []string{
		`jsonb_build_object('content_updated_at_ms',r.content_updated_at_ms+1)`,
		`jsonb_build_object('content_updated_at_ms',NULL)`,
		`jsonb_build_object('date_display','{"created":true,"updated":true,"published":true}'::jsonb)`,
	} {
		// Clone an otherwise valid saved snapshot; only the date data differs.
		_, e := a.Pool.Exec(ctx, `INSERT INTO post_revisions SELECT (jsonb_populate_record(NULL::post_revisions,to_jsonb(r)||jsonb_build_object('id',$2::text,'revision_no',r.revision_no+1,'reason','checkpoint')||`+mutation+`)).* FROM post_revisions r WHERE r.id=$1`, rid, RandID())
		wantCheckViolation(e, "revision dates must snapshot saved draft")
	}
	for _, assignment := range []string{
		"content_updated_at_ms=content_updated_at_ms+1",
		"created_at_ms=created_at_ms+1",
		`date_display='{"created":true,"updated":true,"published":true}'::jsonb`,
	} {
		_, e := a.Pool.Exec(ctx, "UPDATE post_revisions SET "+assignment+" WHERE id=$1", rid)
		wantCheckViolation(e, "immutable")
	}
	// Advancing the aggregate version must not make the original creation time
	// editable through SQL; public created_at is sourced from this row.
	if _, e := a.Pool.Exec(ctx, "UPDATE posts SET created_at_ms=created_at_ms+1,version=version+1 WHERE id=$1", id); e != nil {
		wantCheckViolation(e, "post creation time is immutable")
	} else {
		t.Fatal("the original article creation time was mutable")
	}
	// The same insert succeeds when every timestamp and visibility choice is
	// copied from the current saved draft, proving the date check caused rejection.
	validRevision := RandID()
	if _, e := a.Pool.Exec(ctx, `INSERT INTO post_revisions SELECT (jsonb_populate_record(NULL::post_revisions,to_jsonb(r)||jsonb_build_object('id',$2::text,'revision_no',r.revision_no+1,'reason','checkpoint'))).* FROM post_revisions r WHERE r.id=$1`, rid, validRevision); e != nil {
		t.Fatal("date guard rejected a valid saved snapshot", e)
	}
	var count int
	if e := a.Pool.QueryRow(ctx, "SELECT count(*) FROM post_revisions WHERE post_id=$1", id).Scan(&count); e != nil || count != 2 {
		t.Fatal("invalid date snapshots survived or valid control was lost", count, e)
	}
	publicAfter := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	articleDateWantPublicSnapshot(t, publicAfter, publicBefore)
}
