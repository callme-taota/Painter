package app

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"painter-next/migrations"
)

func TestUnifiedTagMigrationKeepsLegacyDraftAndImmutablePublicationLinks(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	p, c, csrf := testIdentity(t, a, "owner")
	// Recreate only the pre-unification taxonomy schema inside this disposable
	// database, then execute the actual new migration against historical data.
	initial, e := migrations.Files.ReadFile("001_initial.sql")
	if e != nil {
		t.Fatal(e)
	}
	start := strings.Index(string(initial), "CREATE FUNCTION painter_revision_snapshot()")
	end := strings.Index(string(initial)[start:], "END $$;") + len("END $$;")
	oldSnapshot := strings.Replace(string(initial)[start:start+end], "CREATE FUNCTION", "CREATE OR REPLACE FUNCTION", 1)
	_, e = a.Pool.Exec(ctx, `DROP VIEW effective_revision_tags; DROP VIEW effective_draft_tags; DROP FUNCTION painter_sync_unified_tags(); DROP TABLE category_tag_map; DROP TABLE tag_aliases; DROP FUNCTION painter_taxonomy_key(text);`+oldSnapshot)
	if e != nil {
		t.Fatal(e)
	}
	var post string
	e = a.Pool.QueryRow(ctx, `INSERT INTO posts(author_user_id,client_draft_id,created_at_ms,updated_at_ms,first_published_at_ms) VALUES($1,'legacy-before-tags',1,1,1) RETURNING id::text`, p.ID).Scan(&post)
	if e != nil {
		t.Fatal(e)
	}
	_, e = a.Pool.Exec(ctx, `INSERT INTO tags(id,name,created_at_ms,updated_at_ms) VALUES(11,'Go',1,1),(12,' ＧＯ ',1,1),(13,'写作',1,1);
 INSERT INTO categories(id,name,created_at_ms,updated_at_ms) VALUES(21,'go',1,1),(22,'只有分类',1,1);
 INSERT INTO post_drafts(post_id,title,markdown,category_id,updated_by,updated_at_ms) VALUES($1,'不可变旧文','旧文正文',21,$2,1);
 INSERT INTO post_draft_tags(post_id,tag_id) VALUES($1,12),($1,13);
 INSERT INTO post_revision_tags(revision_id,tag_id,tag_name) VALUES('legacy-rid',12,' ＧＯ '),('legacy-rid',13,'写作');
 INSERT INTO post_revisions(id,post_id,revision_no,source_draft_version,reason,title,summary,markdown,sanitized_html,plain_text,renderer_version,content_hash,search_title,search_summary,search_body,category_id,category_name,created_by,created_at_ms,content_updated_at_ms)
 VALUES('legacy-rid',$1,1,1,'publish','不可变旧文','','旧文正文','<p>旧文正文</p>','旧文正文','old-renderer',repeat('a',64),'不可变旧文','','旧文正文',21,'go',$2,1,1);
 INSERT INTO post_publications(post_id,revision_id,published_by,published_at_ms) VALUES($1,'legacy-rid',$2,1);`, pgx.QueryExecModeSimpleProtocol, post, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	// An unpublished category-only draft must acquire a usable tag too.
	var onlyPost string
	e = a.Pool.QueryRow(ctx, `INSERT INTO posts(author_user_id,client_draft_id,created_at_ms,updated_at_ms) VALUES($1,'category-only-draft',1,1) RETURNING id::text`, p.ID).Scan(&onlyPost)
	if e != nil {
		t.Fatal(e)
	}
	_, e = a.Pool.Exec(ctx, `INSERT INTO post_drafts(post_id,title,category_id,updated_by,updated_at_ms) VALUES($1,'分类草稿',22,$2,1); SELECT setval(pg_get_serial_sequence('tags','id'),13,true)`, pgx.QueryExecModeSimpleProtocol, onlyPost, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	upgrade, e := migrations.Files.ReadFile("0006_unified_tags.sql")
	if e != nil {
		t.Fatal(e)
	}
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, string(upgrade)); e != nil {
		tx.Rollback(ctx)
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = a.Pool.QueryRow(ctx, `SELECT count(*) FROM effective_revision_tags WHERE revision_id='legacy-rid'`).Scan(&count); e != nil || count != 2 {
		t.Fatal("merged publication links", count, e)
	}
	if e = a.Pool.QueryRow(ctx, `SELECT count(*) FROM effective_draft_tags WHERE post_id=$1`, post).Scan(&count); e != nil || count != 2 {
		t.Fatal("merged draft links", count, e)
	}
	if e = a.Pool.QueryRow(ctx, `SELECT count(*) FROM effective_draft_tags WHERE post_id=$1`, onlyPost).Scan(&count); e != nil || count != 1 {
		t.Fatal("category-only draft link", count, e)
	}
	var body, hash, categoryName string
	if e = a.Pool.QueryRow(ctx, `SELECT markdown,content_hash,category_name FROM post_revisions WHERE id='legacy-rid'`).Scan(&body, &hash, &categoryName); e != nil || body != "旧文正文" || hash != strings.Repeat("a", 64) || categoryName != "go" {
		t.Fatal("migration changed immutable source", body, hash, categoryName, e)
	}
	if _, e = a.Pool.Exec(ctx, `UPDATE post_revision_tags SET tag_name='changed' WHERE revision_id='legacy-rid'`); e == nil {
		t.Fatal("migration removed revision immutability")
	}
	a.RegisterContent()
	a.RegisterAdmin()
	public := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+post, nil, 0, ""), 200)
	if _, ok := public["category"]; ok {
		t.Fatal("second taxonomy still public", public)
	}
	if len(public["tags"].([]any)) != 2 {
		t.Fatal(public)
	}
	list := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts?tag_id=12", nil, 0, ""), 200)
	if len(list["items"].([]any)) != 1 {
		t.Fatal("old tag alias lost filter link", list)
	}
	list = contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts?category_id=21", nil, 0, ""), 200)
	if len(list["items"].([]any)) != 1 {
		t.Fatal("old category filter lost link", list)
	}
	all := contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/admin/tags", nil, 0, ""), 200)
	if len(all["items"].([]any)) != 3 {
		t.Fatal("duplicate spelling visible", all)
	}
	if w := contentCall(t, a, c, csrf, "POST", "/api/v1/admin/tags", map[string]any{"name": "Ｇｏ"}, 0, ""); w.Code != 409 {
		t.Fatal("normalized duplicate was created", w.Code, w.Body.String())
	}
	if w := contentCall(t, a, c, csrf, "POST", "/api/v1/admin/categories", map[string]any{"name": "旧模型"}, 0, ""); w.Code != 410 {
		t.Fatal("new categories still writable", w.Code)
	}
	// Publish the untouched pre-migration working copy, then restore its original
	// immutable revision: both paths retain all canonical associations.
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+post+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/studio/posts/"+post+"/restore", map[string]any{"revision_id": "legacy-rid"}, 2, ""), 200)
	draft := contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/studio/posts/"+post, nil, 0, ""), 200)
	if len(draft["tag_ids"].([]any)) != 2 || draft["cover"].(map[string]any)["mode"] != "auto" {
		t.Fatal("legacy restore lost tags/default cover", draft)
	}
	if _, e = a.Pool.Exec(ctx, `SELECT painter_sync_unified_tags(); SELECT painter_sync_unified_tags()`); e != nil {
		t.Fatal("mapping reconciliation was not repeatable", e)
	}
}

func TestLegacyImportUnifiesNewCategoriesAndPreservesCoverMetadata(t *testing.T) {
	a := testApp(t)
	path, packet := legacyFixture(t)
	packet.Categories[0].Name = " ＧＯ "
	packet.Tags[0].Name = "go"
	// Keep the text identical: cover metadata alone must produce a separate
	// imported working revision instead of silently reusing the publication.
	packet.Posts[0].Draft = *packet.Posts[0].Publication
	packet.Posts[0].Draft.Cover = CoverConfig{Mode: "upload", Template: "minimal", Color: "plum", MediaID: "photo-old"}
	packet.Posts[0].Publication.Cover = CoverConfig{Mode: "auto", Template: "geometric", Color: "moss"}
	writeLegacy(t, path, packet)
	report, e := a.ImportLegacy(context.Background(), path, false)
	if e != nil {
		t.Fatal(e)
	}
	var canonical, count int
	if e = a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM post_revisions WHERE post_id=40`).Scan(&count); e != nil || count != 2 {
		t.Fatal("cover-only imported working revision was lost", count, e)
	}
	if e = a.Pool.QueryRow(context.Background(), `SELECT count(DISTINCT tag_id) FROM effective_draft_tags WHERE post_id=40`).Scan(&count); e != nil || count != 1 {
		t.Fatal("import produced two vocabularies", count, e)
	}
	if e = a.Pool.QueryRow(context.Background(), `SELECT tag_id FROM category_tag_map WHERE category_id=3`).Scan(&canonical); e != nil || canonical != 4 {
		t.Fatal(canonical, e)
	}
	var raw []byte
	if e = a.Pool.QueryRow(context.Background(), `SELECT cover FROM post_drafts WHERE post_id=40`).Scan(&raw); e != nil || !strings.Contains(string(raw), report.IDMappings["media"]["photo-old"]) {
		t.Fatal("import dropped uploaded cover", string(raw), e)
	}
	public, e := a.PublicPost(context.Background(), "40")
	if e != nil {
		t.Fatal(e)
	}
	if public["cover"].(map[string]any)["template"] != "geometric" {
		t.Fatal("import draft cover changed publication", public)
	}
}
