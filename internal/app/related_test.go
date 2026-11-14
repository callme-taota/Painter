package app

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

type relatedFixture struct {
	t      *testing.T
	a      *App
	cookie *http.Cookie
	csrf   string
}

func newRelatedFixture(t *testing.T) relatedFixture {
	a := testApp(t)
	a.RegisterContent()
	_, c, csrf := testIdentity(t, a, "owner")
	return relatedFixture{t, a, c, csrf}
}

func (f relatedFixture) tag(name string) string {
	f.t.Helper()
	var id string
	if e := f.a.Pool.QueryRow(context.Background(), `INSERT INTO tags(name,created_at_ms,updated_at_ms) VALUES($1,1,1) RETURNING id::text`, name).Scan(&id); e != nil {
		f.t.Fatal(e)
	}
	return id
}

func (f relatedFixture) post(title string, published bool, tags ...string) string {
	f.t.Helper()
	d := map[string]any{"client_draft_id": RandID(), "title": title, "summary": "公开摘要", "markdown": "文章正文", "tag_ids": tags}
	created := contentWant(f.t, contentCall(f.t, f.a, f.cookie, f.csrf, "POST", "/api/v1/posts", d, 0, ""), 201)
	id := created["id"].(string)
	if published {
		contentWant(f.t, contentCall(f.t, f.a, f.cookie, f.csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	}
	return id
}

func (f relatedFixture) want(source string, ids ...string) []map[string]any {
	f.t.Helper()
	rows, e := f.a.RelatedPosts(context.Background(), source)
	if e != nil {
		f.t.Fatal(e)
	}
	got := []string{}
	for _, row := range rows {
		got = append(got, row["id"].(string))
	}
	if ids == nil {
		ids = []string{}
	}
	if !reflect.DeepEqual(got, ids) {
		f.t.Fatalf("recommendations for %s: got %v, want %v", source, got, ids)
	}
	return rows
}

func TestRelatedPostsRankingFallbackAndVisibility(t *testing.T) {
	f := newRelatedFixture(t)
	a, b := f.tag("Go"), f.tag("系统设计")
	source := f.post("当前文章", true, a, b)
	best := f.post("两个相同标签", true, a, b)
	weak := f.post("一个相同标签", true, a)
	tie := f.post("同时间用ID确定顺序", true, a)
	recent := f.post("最近的其他文章", true)
	f.post("不能泄漏的私有草稿", false, a, b)
	if _, e := f.a.Pool.Exec(context.Background(), `UPDATE post_publications SET published_at_ms=CASE WHEN post_id=$1 THEN 1000 WHEN post_id=$2 THEN 9000 ELSE 5000 END`, best, recent); e != nil {
		t.Fatal(e)
	}
	f.want(source, best, tie, weak)
	contentWant(t, contentCall(t, f.a, f.cookie, f.csrf, "DELETE", "/api/v1/studio/posts/"+tie+"/publication", nil, 2, ""), 200)
	f.want(source, best, weak, recent)
	contentWant(t, contentCall(t, f.a, f.cookie, f.csrf, "DELETE", "/api/v1/studio/posts/"+weak, nil, 2, ""), 200)
	f.want(source, best, recent)
	contentWant(t, contentCall(t, f.a, f.cookie, f.csrf, "DELETE", "/api/v1/studio/posts/"+source+"/publication", nil, 2, ""), 200)
	f.want(source)
	f.want("invalid")
	f.want("999999")
}

func TestRelatedPostsUsePublishedCanonicalTagsAndRenderWithoutJS(t *testing.T) {
	f := newRelatedFixture(t)
	a, b, alias := f.tag("Go"), f.tag("摄影"), f.tag("旧Go拼写")
	source := f.post("当前公开文章", true, a)
	match := f.post("旧标签仍然相同", true, alias)
	recent := f.post("其他兴趣的文章", true, b)
	if _, e := f.a.Pool.Exec(context.Background(), `INSERT INTO tag_aliases(tag_id,canonical_tag_id) VALUES($1,$2)`, alias, a); e != nil {
		t.Fatal(e)
	}
	if _, e := f.a.Pool.Exec(context.Background(), `UPDATE post_publications SET published_at_ms=CASE WHEN post_id=$1 THEN 1000 ELSE 9000 END`, match); e != nil {
		t.Fatal(e)
	}
	f.want(source, match, recent)
	for _, id := range []string{source, match} {
		contentWant(t, contentCall(t, f.a, f.cookie, f.csrf, "PUT", "/api/v1/studio/posts/"+id+"/draft", map[string]any{"title": "私有的新标题", "markdown": "私有正文", "tag_ids": []string{b}, "expected_draft_version": 1}, 2, ""), 200)
	}
	rows := f.want(source, match, recent)
	if rows[0]["title"] != "旧标签仍然相同" || strings.Contains(rows[0]["title"].(string), "私有") {
		t.Fatal("private title changed recommendation", rows)
	}
	f.a.RegisterWeb()
	w := contentCall(t, f.a, nil, "", "GET", "/posts/"+source, nil, 0, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `id="related-heading">相关推荐`) || !strings.Contains(w.Body.String(), "旧标签仍然相同") || strings.Contains(w.Body.String(), "私有的新标题") {
		t.Fatal("SSR recommendation missing or private draft exposed", w.Code, w.Body.String())
	}
	contentWant(t, contentCall(t, f.a, f.cookie, f.csrf, "POST", "/api/v1/studio/posts/"+source+"/publish", map[string]any{"expected_draft_version": 2}, 3, ""), 200)
	f.want(source, recent, match)
}
