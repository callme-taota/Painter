package app

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func legacyFixture(t *testing.T) (string, LegacyPackage) {
	t.Helper()
	dir := t.TempDir()
	f, e := os.Create(filepath.Join(dir, "photo.png"))
	if e != nil {
		t.Fatal(e)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	if e = png.Encode(f, img); e != nil {
		t.Fatal(e)
	}
	f.Close()
	photo := "photo-old"
	category := "3"
	published := int64(1700000000000)
	deleted := int64(1700100000000)
	pub := LegacyContent{Title: "公开旧文中文测试", Summary: "published only", Markdown: "# Public\n\n公开正文\n\n![photo](/uploads/photo.png)", CategoryID: &category, TagIDs: []string{"4"}, MediaIDs: []string{photo}}
	draft := pub
	draft.Title = "绝密草稿搜索不应找到"
	draft.Markdown = "# Private\n\n草稿独有秘密\n\n![photo](/uploads/photo.png)"
	packet := LegacyPackage{Version: 1, BatchID: "fixture-migration-1", SourceSystem: "painter-legacy", Users: []LegacyUser{{ID: "10", Username: "legacy-owner", DisplayName: "Old Owner", Role: "owner", GitHubSubject: "42", Email: strptr("private@example.com"), AvatarMediaID: &photo}, {ID: "20", Username: "legacy-author", DisplayName: "Old Author", Role: "author"}}, Media: []LegacyMedia{{ID: photo, OwnerID: "20", Path: "photo.png", OriginalName: "photo.png", LegacyPaths: []string{"/uploads/photo.png"}}}, Categories: []LegacyTaxonomy{{ID: "3", Name: "历史分类", Archived: true}}, Tags: []LegacyTaxonomy{{ID: "4", Name: "历史标签", Archived: true}}, Posts: []LegacyPost{{ID: "40", AuthorID: "20", Draft: draft, Publication: &pub, PublishedAtMS: &published, LegacyReadCount: 77}, {ID: "41", AuthorID: "20", Draft: draft, Publication: &pub, PublishedAtMS: &published, DeletedAtMS: &deleted}, {ID: "42", AuthorID: "20", Draft: LegacyContent{Title: "未发布草稿", Markdown: "完全私有"}}}, Comments: []LegacyComment{{ID: "60", PostID: "40", UserID: "10", Body: "历史评论", State: "visible"}}, PostLikes: []LegacyPostRelation{{PostID: "40", UserID: "10"}}, CommentLikes: []LegacyCommentRelation{{CommentID: "60", UserID: "20"}}, Bookmarks: []LegacyPostRelation{{PostID: "40", UserID: "10"}}, Follows: []LegacyFollow{{FollowerID: "10", FollowedID: "20"}}, History: []LegacyPostRelation{{PostID: "40", UserID: "20", AtMS: published}}, LegacyCounters: []LegacyCounter{{SourceKey: "visitor-day-2023-11-14", Day: "2023-11-14", Count: 12}}, SiteProfile: &LegacySiteProfile{DisplayName: "Legacy Site", AvatarMediaID: &photo}, Settings: &LegacySettings{CommentsEnabled: true, CollectionsEnabled: true, FollowsEnabled: true, AnalyticsEnabled: true}}
	path := filepath.Join(dir, "manifest.json")
	raw, e := json.MarshalIndent(packet, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	return path, packet
}
func strptr(s string) *string { return &s }
func writeLegacy(t *testing.T, path string, p LegacyPackage) {
	t.Helper()
	raw, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestLegacyJSONImportDryRunRelationsAndPublishedIsolation(t *testing.T) {
	a := testApp(t)
	path, _ := legacyFixture(t)
	ctx := context.Background()
	dry, e := a.ImportLegacy(ctx, path, true)
	if e != nil {
		t.Fatal(e)
	}
	if !dry.DryRun || dry.Committed || dry.Counts["posts"] != 3 {
		t.Fatal(dry)
	}
	var count int
	if e = a.Pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&count); e != nil || count != 0 {
		t.Fatal("dryrun wrote users", count, e)
	}
	if entries, e := os.ReadDir(filepath.Join(a.Config.MediaDir, "objects")); e == nil && len(entries) > 0 {
		t.Fatal("dryrun copied media")
	}
	report, e := a.ImportLegacy(ctx, path, false)
	if e != nil {
		t.Fatal(e)
	}
	if !report.Committed || report.IDMappings["users"]["20"] != "20" || report.IDMappings["media"]["photo-old"] != dry.IDMappings["media"]["photo-old"] {
		t.Fatal(report)
	}
	var author, title, markdown string
	if e = a.Pool.QueryRow(ctx, "SELECT author_user_id::text,title,markdown FROM public_posts WHERE post_id=40").Scan(&author, &title, &markdown); e != nil || author != "20" || title != "公开旧文中文测试" || strings.Contains(markdown, "草稿独有秘密") {
		t.Fatal(author, title, markdown, e)
	}
	if !strings.Contains(markdown, "/media/"+report.IDMappings["media"]["photo-old"]) {
		t.Fatal("legacy image not rewritten", markdown)
	}
	if e = a.Pool.QueryRow(ctx, "SELECT title FROM post_drafts WHERE post_id=40").Scan(&title); e != nil || title != "绝密草稿搜索不应找到" {
		t.Fatal(title, e)
	}
	if e = a.Pool.QueryRow(ctx, "SELECT count(*) FROM public_posts").Scan(&count); e != nil || count != 1 {
		t.Fatal("draft/trash leaked", count, e)
	}
	if e = a.Pool.QueryRow(ctx, "SELECT count(*) FROM post_revisions").Scan(&count); e != nil || count != 5 {
		t.Fatal("import revision lineage missing", count, e)
	}
	for _, table := range []string{"comments", "comment_likes", "post_likes", "bookmarks", "follows", "reading_history", "media_aliases"} {
		if e = a.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); e != nil || count != 1 {
			t.Fatal(table, count, e)
		}
	}
	if e = a.Pool.QueryRow(ctx, "SELECT count(*) FROM legacy_counters").Scan(&count); e != nil || count != 4 {
		t.Fatal(count, e)
	}
	if e = a.Pool.QueryRow(ctx, "SELECT count(*) FROM analytics_events").Scan(&count); e != nil || count != 0 {
		t.Fatal("legacy numbers polluted analytics", count, e)
	}
	var next int64
	if e = a.Pool.QueryRow(ctx, "INSERT INTO posts(author_user_id,client_draft_id,created_at_ms,updated_at_ms) VALUES(20,'new-after-import',1,1) RETURNING id").Scan(&next); e != nil || next != 43 {
		t.Fatal("identity sequence not advanced", next, e)
	}
	if _, e = a.ImportLegacy(ctx, path, false); e == nil {
		t.Fatal("import appended into nonempty site")
	}
	if _, e = os.Stat(report.JournalPath + ".committed"); e != nil {
		t.Fatal("no commit journal", e)
	}
}
func TestLegacyInvalidReferencesAndRollbackRemoveMedia(t *testing.T) {
	a := testApp(t)
	path, packet := legacyFixture(t)
	packet.Posts[0].AuthorID = "9999"
	writeLegacy(t, path, packet)
	if _, e := a.ImportLegacy(context.Background(), path, true); e == nil {
		t.Fatal("dryrun accepted dangling author")
	}
	packet.Posts[0].AuthorID = "20"
	packet.Media[0].Path = "../escape.png"
	writeLegacy(t, path, packet)
	if _, e := a.ImportLegacy(context.Background(), path, true); e == nil {
		t.Fatal("dryrun accepted traversal")
	}
	packet.Media[0].Path = "photo.png"
	writeLegacy(t, path, packet)
	if _, e := a.Pool.Exec(context.Background(), `CREATE FUNCTION test_import_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected post import failure'; END $$; CREATE TRIGGER test_import_fail BEFORE INSERT ON posts FOR EACH ROW EXECUTE FUNCTION test_import_fail()`); e != nil {
		t.Fatal(e)
	}
	if _, e := a.ImportLegacy(context.Background(), path, false); e == nil {
		t.Fatal("injected DB failure ignored")
	}
	var count int
	if e := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&count); e != nil || count != 0 {
		t.Fatal("partial DB import", count, e)
	}
	entries, e := os.ReadDir(filepath.Join(a.Config.MediaDir, "objects"))
	if e != nil || len(entries) != 0 {
		t.Fatal("failed import left media", len(entries), e)
	}
}
func TestBackupRestoreChecksumsAndMaintenanceGate(t *testing.T) {
	if _, e := pgBinary("pg_dump"); e != nil {
		local := "/tmp/painter-next-pg18/root/usr/lib/postgresql/18/bin"
		if _, e = os.Stat(filepath.Join(local, "pg_dump")); e != nil {
			t.Skip("PostgreSQL 18 client binaries required")
		}
		t.Setenv("POSTGRES_BIN_DIR", local)
	}
	a := testApp(t)
	path, _ := legacyFixture(t)
	if _, e := a.ImportLegacy(context.Background(), path, false); e != nil {
		t.Fatal(e)
	}
	// Maintenance and migrations share the same advisory key, across connections.
	gate, e := a.maintenance(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	var canMigrate bool
	if e = a.Pool.QueryRow(context.Background(), "SELECT pg_try_advisory_lock(7284923151123)").Scan(&canMigrate); e != nil || canMigrate {
		t.Fatal("migration crossed maintenance advisory", canMigrate, e)
	}
	gate.Rollback(context.Background())
	// An in-flight writer SHARE gate must drain before backup starts.
	tx, e := a.Pool.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(context.Background(), "SELECT id FROM policy_barrier WHERE id=1 FOR SHARE"); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_, e = a.maintenance(ctx)
	cancel()
	tx.Rollback(context.Background())
	if e == nil {
		t.Fatal("maintenance crossed active mutation gate")
	}
	dir := filepath.Join(t.TempDir(), "backup")
	backup, e := a.Backup(context.Background(), dir)
	if e != nil {
		t.Fatal(e)
	}
	if backup.MediaObjects != 1 || len(backup.DatabaseSHA256) != 64 || backup.ID == "" {
		t.Fatal(backup)
	}
	var recordDir, origin string
	var observed *int64
	if e = a.Pool.QueryRow(context.Background(), "SELECT directory,origin,restore_verified_at_ms FROM backup_records WHERE id=$1", backup.ID).Scan(&recordDir, &origin, &observed); e != nil || recordDir != dir || origin != "local" || observed != nil {
		t.Fatal("incorrect backup observation", recordDir, origin, observed, e)
	}
	restored, e := a.RestoreCheck(context.Background(), dir)
	if e != nil {
		t.Fatal(e)
	}
	if !restored.Verified || !restored.DatabaseDropped || restored.PublicPosts != 1 || restored.MediaObjects != 1 || restored.BackupID != backup.ID {
		t.Fatal(restored)
	}
	var beforeVerification int64
	if e = a.Pool.QueryRow(context.Background(), "SELECT restore_verified_at_ms FROM backup_records WHERE id=$1", backup.ID).Scan(&beforeVerification); e != nil || beforeVerification <= 0 {
		t.Fatal("restore success was not persisted", beforeVerification, e)
	}
	manifest, e := readBackupManifest(dir)
	if e != nil {
		t.Fatal(e)
	}
	f, e := os.OpenFile(filepath.Join(dir, "media", "objects", manifest.Media[0].StorageKey), os.O_WRONLY|os.O_APPEND, 0600)
	if e != nil {
		t.Fatal(e)
	}
	f.Write([]byte("corrupt"))
	f.Close()
	if _, e = a.RestoreCheck(context.Background(), dir); e == nil {
		t.Fatal("corrupt backup media passed restore-check")
	}
	var afterVerification int64
	if e = a.Pool.QueryRow(context.Background(), "SELECT restore_verified_at_ms FROM backup_records WHERE id=$1", backup.ID).Scan(&afterVerification); e != nil || beforeVerification != afterVerification {
		t.Fatal("failed verification changed observation", beforeVerification, afterVerification, e)
	}
}

func TestLibpqTLSKeywordParsing(t *testing.T) {
	got := libpqKeywords(`host=db port=5432 user='app user' sslmode=verify-full sslrootcert='/path with space/root.pem' sslcert='client\'s.pem' sslkey='key\\file.pem'`)
	want := map[string]string{"host": "db", "port": "5432", "user": "app user", "sslmode": "verify-full", "sslrootcert": "/path with space/root.pem", "sslcert": "client's.pem", "sslkey": `key\file.pem`}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s=%q want %q", k, got[k], v)
		}
	}
}

func opsPGTools(t *testing.T) (string, string) {
	t.Helper()
	if _, e := pgBinary("pg_dump"); e != nil {
		local := "/tmp/painter-next-pg18/root/usr/lib/postgresql/18/bin"
		if _, e = os.Stat(filepath.Join(local, "pg_dump")); e != nil {
			t.Skip("PostgreSQL 18 client binaries required")
		}
		t.Setenv("POSTGRES_BIN_DIR", local)
	}
	dump, e := pgBinary("pg_dump")
	if e != nil {
		t.Fatal(e)
	}
	restore, e := pgBinary("pg_restore")
	if e != nil {
		t.Fatal(e)
	}
	return dump, restore
}
func opsShellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
func TestBackupManifestFailureDoesNotRecordSuccess(t *testing.T) {
	dump, _ := opsPGTools(t)
	a := testApp(t)
	tools := t.TempDir()
	// The real dump succeeds, then a directory occupies the manifest file path.
	// This exercises the artifact's last write, after pg_dump, before success ledger.
	script := "#!/bin/sh\nREAL=" + opsShellQuote(dump) + "\nif [ \"$1\" = --version ]; then exec \"$REAL\" --version; fi\noutput=\nfor arg in \"$@\"; do case \"$arg\" in --file=*) output=${arg#--file=} ;; esac; done\n\"$REAL\" \"$@\" || exit $?\nmkdir \"$(dirname \"$output\")/manifest.json\"\n"
	if e := os.WriteFile(filepath.Join(tools, "pg_dump"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("POSTGRES_BIN_DIR", tools)
	dir := filepath.Join(t.TempDir(), "backup")
	if _, e := a.Backup(context.Background(), dir); e == nil {
		t.Fatal("manifest write failure accepted")
	}
	if _, e := os.Stat(filepath.Join(dir, "database.dump")); e != nil {
		t.Fatal("fixture failed before actual pg_dump", e)
	}
	var count int
	if e := a.Pool.QueryRow(context.Background(), "SELECT (SELECT count(*) FROM backup_records)+(SELECT count(*) FROM audit_logs WHERE action='system.backup')").Scan(&count); e != nil || count != 0 {
		t.Fatal("artifact failure recorded success", count, e)
	}
}
func TestRestoreImportedRecordAndPreserveLocalHistory(t *testing.T) {
	opsPGTools(t)
	a := testApp(t)
	source := filepath.Join(t.TempDir(), "source-backup")
	backup, e := a.Backup(context.Background(), source)
	if e != nil {
		t.Fatal(e)
	}
	moved := filepath.Join(t.TempDir(), "relocated-backup")
	if e = os.Rename(source, moved); e != nil {
		t.Fatal(e)
	}
	checked, e := a.RestoreCheck(context.Background(), moved)
	if e != nil || !checked.Verified || !checked.DatabaseDropped || checked.BackupID != backup.ID {
		t.Fatal(checked, e)
	}
	var directory, origin string
	if e = a.Pool.QueryRow(context.Background(), "SELECT directory,origin FROM backup_records WHERE id=$1", backup.ID).Scan(&directory, &origin); e != nil || directory != source || origin != "local" {
		t.Fatal("local backup history was rewritten", directory, origin, e)
	}
	other := testApp(t)
	imported, e := other.RestoreCheck(context.Background(), moved)
	if e != nil || !imported.Verified || !imported.DatabaseDropped || imported.BackupID == "" {
		t.Fatal(imported, e)
	}
	var verified int64
	if e = other.Pool.QueryRow(context.Background(), "SELECT directory,origin,restore_verified_at_ms FROM backup_records WHERE id=$1", imported.BackupID).Scan(&directory, &origin, &verified); e != nil || directory != moved || origin != "imported" || verified <= 0 {
		t.Fatal("imported observation missing", directory, origin, verified, e)
	}
	if _, e = other.Pool.Exec(context.Background(), "UPDATE backup_records SET directory='rewritten' WHERE id=$1", imported.BackupID); e == nil {
		t.Fatal("observation history rewritten")
	}
	if _, e = other.Pool.Exec(context.Background(), "UPDATE backup_records SET restore_verified_at_ms=NULL WHERE id=$1", imported.BackupID); e == nil {
		t.Fatal("verification history cleared")
	}
	if _, e = other.Pool.Exec(context.Background(), "DELETE FROM backup_records WHERE id=$1", imported.BackupID); e == nil {
		t.Fatal("observation history deleted")
	}
}
