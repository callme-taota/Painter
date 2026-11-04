package app

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func reviewPublish(t *testing.T, a *App, c *http.Cookie, csrf, title, summary, markdown string) (string, string) {
	t.Helper()
	d := map[string]any{"client_draft_id": RandID(), "title": title, "summary": summary, "markdown": markdown}
	v := contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/posts", d, 0, ""), 201)
	id := v["id"].(string)
	v = contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	return id, v["revision_id"].(string)
}
func TestReviewSearchLiteralANDUnicodeAndLexicographicRank(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterSearch()
	_, c, csrf := testIdentity(t, a, "owner")
	both, _ := reviewPublish(t, a, c, csrf, "alpha beta", "", "中文检索 %_! 中")
	title, _ := reviewPublish(t, a, c, csrf, "alpha", "", "beta")
	summary, _ := reviewPublish(t, a, c, csrf, "无关标题", "alpha beta", "普通正文")
	reviewPublish(t, a, c, csrf, "alpha only", "", "缺少第二个词")
	normalized, _ := reviewPublish(t, a, c, csrf, "Straße ＧＯ", "", "Unicode normalization")
	search := func(q string) []map[string]any {
		t.Helper()
		v, e := a.SearchPosts(context.Background(), url.Values{"q": {q}})
		if e != nil {
			t.Fatal(e)
		}
		return v["items"].([]map[string]any)
	}
	items := search("alpha beta")
	if len(items) != 3 || items[0]["id"] != both || items[1]["id"] != title || items[2]["id"] != summary {
		t.Fatalf("AND/ranking mismatch %#v", items)
	}
	for _, q := range []string{"中", "中文", "中文检索", "%", "%_!"} {
		items = search(q)
		if len(items) != 1 || items[0]["id"] != both {
			t.Fatalf("literal/short query %q => %#v", q, items)
		}
	}
	items = search("  STRASSE\u3000go  ")
	if len(items) != 1 || items[0]["id"] != normalized {
		t.Fatal("Unicode folding mismatch", items)
	}
	items = search("alpha alpha beta")
	if len(items) != 3 || items[0]["id"] != both {
		t.Fatal("duplicate query words inflated rank", items)
	}
}

func TestReviewSearchCursorGenerationAndDraftIsolation(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterSearch()
	_, c, csrf := testIdentity(t, a, "owner")
	first, _ := reviewPublish(t, a, c, csrf, "cursorword one", "", "公开正文")
	reviewPublish(t, a, c, csrf, "cursorword two", "", "公开正文")
	q := url.Values{"q": {"cursorword"}, "limit": {"1"}}
	page, e := a.SearchPosts(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	cursor := page["next_cursor"].(string)
	if cursor == "" {
		t.Fatal("no cursor")
	}
	q.Set("cursor", cursor)
	contentWant(t, contentCall(t, a, c, csrf, "PUT", "/api/v1/studio/posts/"+first+"/draft", map[string]any{"title": "草稿独有秘密", "markdown": "not-public-new-content", "expected_draft_version": 1}, 2, ""), 200)
	if _, e = a.SearchPosts(context.Background(), q); e != nil {
		t.Fatal("unpublished draft invalidated public search", e)
	}
	q.Set("q", "otherword")
	if _, e = a.SearchPosts(context.Background(), q); e == nil {
		t.Fatal("cursor filter substitution accepted")
	}
	q.Set("q", "cursorword")
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+first+"/publish", map[string]any{"expected_draft_version": 2}, 3, ""), 200)
	_, e = a.SearchPosts(context.Background(), q)
	var ae *APIError
	if !errors.As(e, &ae) || ae.Code != "SEARCH_CHANGED" {
		t.Fatal("publication generation did not invalidate cursor", e)
	}
}

func TestReviewSearchTimeoutClassAndConnectionRecovery(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `LOCK TABLE search_state IN ACCESS EXCLUSIVE MODE`); e != nil {
		t.Fatal(e)
	}
	_, e = a.SearchPosts(ctx, url.Values{"q": {"missing"}})
	var ae *APIError
	if !errors.As(e, &ae) || ae.Code != "SEARCH_TIMEOUT" {
		t.Errorf("generation query timeout classification: %T %v", e, e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = a.SearchPosts(ctx, url.Values{"q": {"missing"}}); e != nil {
		t.Fatal("canceled search left unusable connection", e)
	}
	var n int
	if e = a.Pool.QueryRow(ctx, `SELECT 1`).Scan(&n); e != nil || n != 1 {
		t.Fatal(n, e)
	}
}

func TestReviewRestoreKeepsPublicationAndRevocationRejectsReplay(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterAdmin()
	_, owner, oc := testIdentity(t, a, "owner")
	author, c, csrf := testIdentity(t, a, "author")
	id, oldRevision := reviewPublish(t, a, c, csrf, "old public title", "", "old body")
	contentWant(t, contentCall(t, a, c, csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", map[string]any{"title": "new public title", "markdown": "new body", "expected_draft_version": 1}, 2, ""), 200)
	key := RandID()
	newPublic := contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 2}, 3, key), 200)
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+id+"/restore", map[string]any{"revision_id": oldRevision}, 4, ""), 200)
	public := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+id, nil, 0, ""), 200)
	private := contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/studio/posts/"+id, nil, 0, ""), 200)
	if public["title"] != "new public title" || public["revision_id"] != newPublic["revision_id"] || private["title"] != "old public title" {
		t.Fatal("restoring changed online publication", public, private)
	}
	contentWant(t, contentCall(t, a, owner, oc, "PATCH", "/api/v1/admin/users/"+author.ID, map[string]any{"disabled": true}, 0, ""), 200)
	w := contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 2}, 3, key)
	if w.Code != 401 {
		t.Fatal("revoked session replayed committed command", w.Code, w.Body.String())
	}
	var state bool
	if e := a.Pool.QueryRow(context.Background(), `SELECT disabled_at_ms IS NOT NULL FROM users WHERE id=$1`, author.ID).Scan(&state); e != nil || !state {
		t.Fatal(state, e)
	}
}

// Keep a direct compile-time witness that all search queries use a PG transaction.
var _ contentQuerier = (pgx.Tx)(nil)

func TestReviewOwnerMediaEditingKeepsAuthorAbleToSave(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterMedia()
	_, owner, oc := testIdentity(t, a, "owner")
	_, author, ac := testIdentity(t, a, "author")
	uploaded := contentWant(t, mediaTestUpload(t, a, owner, oc, RandID(), 35), 201)
	media := uploaded["media_id"].(string)
	created := contentWant(t, contentCall(t, a, author, ac, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "作者文章", "markdown": "原文"}, 0, ""), 201)
	id := created["id"].(string)
	d := map[string]any{"title": "站长加图", "markdown": "![图](/media/" + media + ")", "media_ids": []string{media}, "expected_draft_version": 1}
	contentWant(t, contentCall(t, a, owner, oc, "PUT", "/api/v1/studio/posts/"+id+"/draft", d, 1, ""), 200)
	d["expected_draft_version"] = 2
	d["title"] = "作者继续编辑"
	contentWant(t, contentCall(t, a, author, ac, "PUT", "/api/v1/studio/posts/"+id+"/draft", d, 2, ""), 200)
}

func TestReviewDurableClientMappingPrecedesMutableReferences(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	_, c, csrf := testIdentity(t, a, "author")
	client := RandID()
	d := map[string]any{"client_draft_id": client, "title": "永久映射", "markdown": "创建成功但响应丢失"}
	created := contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/posts", d, 0, ""), 201)
	d["category_id"] = "99999999"
	replay := contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/posts", d, 0, ""), 201)
	if replay["id"] != created["id"] {
		t.Fatal("durable mapping changed")
	}
	d["client_draft_id"] = RandID()
	w := contentCall(t, a, c, csrf, "POST", "/api/v1/posts", d, 0, "")
	if w.Code != 400 {
		t.Fatal("new draft bypassed references validation", w.Code, w.Body.String())
	}
}

func TestReviewConcurrentRestoreLocksUnionOfMediaInOrder(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterMedia()
	_, c, csrf := testIdentity(t, a, "owner")
	m1 := contentWant(t, mediaTestUpload(t, a, c, csrf, RandID(), 17), 201)["media_id"].(string)
	m2 := contentWant(t, mediaTestUpload(t, a, c, csrf, RandID(), 18), 201)["media_id"].(string)
	create := func(media string) (string, string) {
		d := map[string]any{"client_draft_id": RandID(), "title": "交叉媒体恢复", "markdown": "![图](/media/" + media + ")", "media_ids": []string{media}}
		v := contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/posts", d, 0, ""), 201)
		id := v["id"].(string)
		v = contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
		return id, v["revision_id"].(string)
	}
	p1, r1 := create(m1)
	p2, r2 := create(m2)
	for _, v := range []struct{ post, media string }{{p1, m2}, {p2, m1}} {
		contentWant(t, contentCall(t, a, c, csrf, "PUT", "/api/v1/studio/posts/"+v.post+"/draft", map[string]any{"title": "交叉媒体当前草稿", "markdown": "![图](/media/" + v.media + ")", "media_ids": []string{v.media}, "expected_draft_version": 1}, 2, ""), 200)
	}
	ctx := context.Background()
	if _, e := a.Pool.Exec(ctx, `CREATE FUNCTION test_pause_restore_ref() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(38817123); RETURN NEW; END $$; CREATE TRIGGER test_pause_restore_ref BEFORE INSERT ON post_revision_media_refs FOR EACH ROW EXECUTE FUNCTION test_pause_restore_ref()`); e != nil {
		t.Fatal(e)
	}
	gate, e := a.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer gate.Rollback(ctx)
	if _, e = gate.Exec(ctx, `SELECT pg_advisory_xact_lock(38817123)`); e != nil {
		t.Fatal(e)
	}
	responses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, v := range []struct{ post, revision string }{{p1, r1}, {p2, r2}} {
		wg.Add(1)
		go func(post, revision string) {
			defer wg.Done()
			w := contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+post+"/restore", map[string]any{"revision_id": revision}, 3, "")
			responses <- w.Code
		}(v.post, v.revision)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var waits int
		if e = a.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND state='active' AND wait_event_type='Lock'`).Scan(&waits); e != nil {
			t.Fatal(e)
		}
		if waits >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if e = gate.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	wg.Wait()
	close(responses)
	for code := range responses {
		if code != 200 {
			t.Fatal("cross-media restore deadlocked or failed", code)
		}
	}
}
