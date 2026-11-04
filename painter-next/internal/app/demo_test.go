package app

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestSeedDemoUsesUnifiedTagsAndAutoCoversWithoutLoginIdentities(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	first, e := a.SeedDemo(ctx)
	if e != nil || first.CreatedPosts != 5 || first.PublicPosts != 5 || first.AuthorID == "" {
		t.Fatal(first, e)
	}
	second, e := a.SeedDemo(ctx)
	if e != nil || second.CreatedPosts != 0 || second.PublicPosts != 5 || second.AuthorID != first.AuthorID {
		t.Fatal("demo seed is not idempotent", first, second, e)
	}
	checks := map[string]int{
		"SELECT count(*) FROM posts":                                        5,
		"SELECT count(*) FROM post_drafts":                                  5,
		"SELECT count(*) FROM post_revisions":                               5,
		"SELECT count(*) FROM post_publications":                            5,
		"SELECT count(*) FROM categories":                                   0,
		"SELECT count(*) FROM users":                                        1,
		"SELECT count(*) FROM users WHERE role='owner'":                     0,
		"SELECT count(*) FROM local_accounts":                               0,
		"SELECT count(*) FROM oauth_accounts":                               0,
		"SELECT count(*) FROM sessions":                                     0,
		"SELECT count(*) FROM post_drafts WHERE category_id IS NOT NULL":    0,
		"SELECT count(*) FROM post_revisions WHERE category_id IS NOT NULL": 0,
		"SELECT count(*) FROM (SELECT post_id FROM effective_draft_tags GROUP BY post_id HAVING count(*)=2) p":            5,
		"SELECT count(*) FROM (SELECT revision_id FROM effective_revision_tags GROUP BY revision_id HAVING count(*)=2) r": 5,
		`SELECT count(*) FROM post_drafts WHERE cover='{"mode":"auto","template":"editorial","color":"ink"}'::jsonb`:      5,
		`SELECT count(*) FROM post_revisions WHERE cover='{"mode":"auto","template":"editorial","color":"ink"}'::jsonb`:   5,
		"SELECT count(*) FROM posts WHERE version<>2":                                                                     0,
	}
	for query, want := range checks {
		var got int
		if e = a.Pool.QueryRow(ctx, query).Scan(&got); e != nil || got != want {
			t.Fatalf("%s: got %d, want %d: %v", query, got, want, e)
		}
	}
	public, e := a.PublicPosts(ctx, url.Values{})
	if e != nil {
		t.Fatal(e)
	}
	for _, value := range public["items"].([]map[string]any) {
		if _, exists := value["category"]; exists {
			t.Fatal("demo exposes two vocabularies", value)
		}
		if len(value["tags"].([]any)) != 2 || value["cover"].(map[string]any)["mode"] != "auto" || value["cover_url"] == "" {
			t.Fatal("demo lost tags/cover", value)
		}
	}
	var bio, about, body string
	if e = a.Pool.QueryRow(ctx, `SELECT u.bio,s.about_markdown,(SELECT string_agg(markdown,E'\n') FROM post_drafts) FROM users u CROSS JOIN site_profile s WHERE u.id=$1 AND s.id=1`, first.AuthorID).Scan(&bio, &about, &body); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(strings.ToLower(bio+about+body), "github") || !strings.Contains(bio+about+body, "本地账号") {
		t.Fatal("outdated demo login copy", bio, about, body)
	}
}

func TestSeedDemoReusesNormalizedExistingTag(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	var canonical string
	if e := a.Pool.QueryRow(ctx, `INSERT INTO tags(name,created_at_ms,updated_at_ms) VALUES(' ＧＯ ',1,1) RETURNING id::text`).Scan(&canonical); e != nil {
		t.Fatal(e)
	}
	if _, e := a.SeedDemo(ctx); e != nil {
		t.Fatal(e)
	}
	var count int
	if e := a.Pool.QueryRow(ctx, `SELECT count(*) FROM tags WHERE painter_taxonomy_key(name)=painter_taxonomy_key('Go')`).Scan(&count); e != nil || count != 1 {
		t.Fatal("seed duplicated normalized tag", count, e)
	}
	if e := a.Pool.QueryRow(ctx, `SELECT count(*) FROM effective_revision_tags WHERE tag_id=$1`, canonical).Scan(&count); e != nil || count != 1 {
		t.Fatal("seed did not use canonical existing tag", count, e)
	}
}
