package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func seriesTestApp(t *testing.T) *App {
	t.Helper()
	a := testApp(t)
	if _, pattern := a.Mux.Handler(httptest.NewRequest("GET", "/api/v1/series", nil)); pattern == "" {
		a.RegisterSeries()
	}
	return a
}

func seriesTestPost(t *testing.T, a *App, cookie *http.Cookie, csrf, title string, published bool) string {
	t.Helper()
	value := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": title, "markdown": "章节内容"}, 0, ""), 201)
	id := value["id"].(string)
	if published {
		contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	}
	return id
}

func seriesTestInput(ids ...string) map[string]any {
	return map[string]any{"title": "从零开始的连载", "slug": "serial-" + RandID()[:12], "description": "由已有文章组成的连载", "state": "ongoing", "status": "published", "chapter_post_ids": ids}
}

func seriesChapters(t *testing.T, value map[string]any) []map[string]any {
	t.Helper()
	return webMaps(value["chapters"])
}

func TestSeriesPublicProjectionTracksPublicationAndNeverLeaksDrafts(t *testing.T) {
	a := seriesTestApp(t)
	a.RegisterContent()
	_, cookie, csrf := testIdentity(t, a, "author")
	first := seriesTestPost(t, a, cookie, csrf, "第一章", true)
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", "/api/v1/studio/posts/"+first+"/draft", map[string]any{"title": "秘密未发布的新标题", "markdown": "新的私人正文", "expected_draft_version": 1}, 2, ""), 200)
	draft := seriesTestPost(t, a, cookie, csrf, "不应泄露的秘密草稿", false)
	last := seriesTestPost(t, a, cookie, csrf, "第三章", true)
	in := seriesTestInput(first, draft, last)
	created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/series", in, 0, ""), 201)
	id := created["id"].(string)
	if len(seriesChapters(t, created)) != 3 || created["total_chapter_count"] != float64(3) {
		t.Fatal("private draft references missing", created)
	}
	ctx := context.Background()
	public, err := a.PublicSeriesDetail(ctx, id)
	if err != nil || len(seriesChapters(t, public)) != 2 || public["chapter_count"] != float64(2) {
		t.Fatal("incorrect public chapter projection", public, err)
	}
	encoded, _ := json.Marshal(public)
	if strings.Contains(string(encoded), "秘密") || public["version"] != nil || public["status"] != nil || public["cover_media_id"] != nil {
		t.Fatal("private metadata leaked", string(encoded))
	}
	compact, err := a.PublicSeriesSummary(ctx, id)
	if err != nil || compact["title"] != public["title"] || compact["chapters"] != nil {
		t.Fatal("compact public reference loaded chapter contents", compact, err)
	}
	navigation, err := a.PostSeries(ctx, first)
	if err != nil || len(navigation) != 1 || navigation[0]["previous"] != nil || navigation[0]["next"].(map[string]any)["id"] != last {
		t.Fatal("navigation did not skip draft", navigation, err)
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/studio/posts/"+last+"/publication", nil, 2, ""), 200)
	public, err = a.PublicSeriesDetail(ctx, id)
	if err != nil || len(seriesChapters(t, public)) != 1 || public["chapter_count"] != float64(1) {
		t.Fatal("withdrawn chapter remained visible", public, err)
	}
	navigation, err = a.PostSeries(ctx, first)
	if err != nil || len(navigation) != 1 || navigation[0]["next"] != nil {
		t.Fatal("withdrawn neighbour remained visible", navigation, err)
	}
	if navigation, err = a.PostSeries(ctx, last); err != nil || len(navigation) != 0 {
		t.Fatal("withdrawn article retained membership", navigation, err)
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/studio/posts/"+first, nil, 3, ""), 200)
	public, err = a.PublicSeriesDetail(ctx, id)
	if err != nil || len(seriesChapters(t, public)) != 0 {
		t.Fatal("deleted chapter remained public", public, err)
	}
	// A private series is never made public by sending a creator cookie or by
	// guessing visibility query parameters on the public API.
	in["status"] = "draft"
	in["chapter_post_ids"] = []string{draft}
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", "/api/v1/studio/series/"+id, in, 1, ""), 200)
	contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/series/"+id+"?status=draft", nil, 0, ""), 404)
	if _, err = a.PublicSeriesSummary(ctx, id); err == nil {
		t.Fatal("compact reference exposed a private series")
	}
	list := contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/series?status=draft", nil, 0, ""), 200)
	if len(webMaps(list["items"])) != 0 {
		t.Fatal("public list accepted private visibility", list)
	}
}

func TestSeriesAuthorizationCASAndOwnerScope(t *testing.T) {
	a := seriesTestApp(t)
	a.RegisterContent()
	author, cookie, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	_, owner, ownerCSRF := testIdentity(t, a, "owner")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	first := seriesTestPost(t, a, cookie, csrf, "作者自己的章节", true)
	foreign := seriesTestPost(t, a, other, otherCSRF, "另一作者章节", true)
	in := seriesTestInput(first)
	contentWant(t, contentCall(t, a, nil, "", "POST", "/api/v1/series", in, 0, ""), 401)
	contentWant(t, contentCall(t, a, reader, readerCSRF, "POST", "/api/v1/series", in, 0, ""), 403)
	contentWant(t, contentCall(t, a, cookie, "", "POST", "/api/v1/series", in, 0, ""), 403)
	created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/series", in, 0, ""), 201)
	id, path := created["id"].(string), "/api/v1/studio/series/"+created["id"].(string)
	contentWant(t, contentCall(t, a, other, otherCSRF, "GET", path, nil, 0, ""), 404)
	contentWant(t, contentCall(t, a, other, otherCSRF, "PUT", path, in, 1, ""), 404)
	contentWant(t, contentCall(t, a, other, otherCSRF, "DELETE", path, nil, 1, ""), 404)
	contentWant(t, contentCall(t, a, reader, readerCSRF, "GET", path, nil, 0, ""), 403)
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", path, in, 0, ""), 428)
	in["chapter_post_ids"] = []string{foreign}
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", path, in, 1, ""), 400)
	// Owner can manage another creator's series but cannot turn it into a
	// collection of their own/third-party articles or transfer its author.
	contentWant(t, contentCall(t, a, owner, ownerCSRF, "PUT", path, in, 1, ""), 400)
	in["chapter_post_ids"] = []string{first}
	in["state"] = "completed"
	saved := contentWant(t, contentCall(t, a, owner, ownerCSRF, "PUT", path, in, 1, ""), 200)
	if saved["version"] != float64(2) || saved["state"] != "completed" || saved["author"].(map[string]any)["id"] != author.ID {
		t.Fatal("owner edit lost author or CAS state", saved)
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", path, in, 1, ""), 409)
	contentWant(t, contentCall(t, a, other, otherCSRF, "GET", "/api/v1/me/series?scope=site", nil, 0, ""), 403)
	list := contentWant(t, contentCall(t, a, owner, ownerCSRF, "GET", "/api/v1/me/series?scope=site", nil, 0, ""), 200)
	if len(webMaps(list["items"])) != 1 {
		t.Fatal("owner site scope missing series", list)
	}
	contentWant(t, contentCall(t, a, owner, ownerCSRF, "DELETE", path, nil, 1, ""), 409)
	key := RandID()
	deleted := contentWant(t, contentCall(t, a, owner, ownerCSRF, "DELETE", path, nil, 2, key), 200)
	replay := contentWant(t, contentCall(t, a, owner, ownerCSRF, "DELETE", path, nil, 2, key), 200)
	if deleted["version"] != float64(3) || replay["version"] != float64(3) {
		t.Fatal("delete replay modified aggregate", deleted, replay)
	}
	contentWant(t, contentCall(t, a, owner, ownerCSRF, "GET", path, nil, 0, ""), 404)
	contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/series/"+id, nil, 0, ""), 404)
	var chapters, articles int
	if err := a.Pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM series_chapters WHERE series_id=$1),(SELECT count(*) FROM posts WHERE id=$2)`, id, first).Scan(&chapters, &articles); err != nil || chapters != 0 || articles != 1 {
		t.Fatal("delete did not remove references or deleted the article", chapters, articles, err)
	}
}

func TestSeriesConcurrentReorderAndReplay(t *testing.T) {
	a := seriesTestApp(t)
	a.RegisterContent()
	_, cookie, csrf := testIdentity(t, a, "author")
	first := seriesTestPost(t, a, cookie, csrf, "一", true)
	second := seriesTestPost(t, a, cookie, csrf, "二", true)
	third := seriesTestPost(t, a, cookie, csrf, "三", true)
	in := seriesTestInput(first, second, third)
	key := RandID()
	created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/series", in, 0, key), 201)
	again := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/series", in, 0, key), 201)
	if again["id"] != created["id"] {
		t.Fatal("create replay duplicated aggregate")
	}
	path := "/api/v1/studio/series/" + created["id"].(string)
	responses := make(chan *httptest.ResponseRecorder, 2)
	var group sync.WaitGroup
	for _, ids := range [][]string{{third, first, second}, {second, third, first}} {
		group.Add(1)
		go func(ids []string) {
			defer group.Done()
			next := seriesTestInput(ids...)
			next["slug"] = in["slug"]
			responses <- contentCall(t, a, cookie, csrf, "PUT", path, next, 1, RandID())
		}(ids)
	}
	group.Wait()
	close(responses)
	wins, conflicts := 0, 0
	for response := range responses {
		switch response.Code {
		case 200:
			wins++
		case 409:
			conflicts++
		default:
			t.Fatal(response.Code, response.Body.String())
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("CAS reorder not serialized", wins, conflicts)
	}
	current := contentWant(t, contentCall(t, a, cookie, csrf, "GET", path, nil, 0, ""), 200)
	chapters := seriesChapters(t, current)
	if len(chapters) != 3 || current["version"] != float64(2) {
		t.Fatal("concurrent reorder lost chapters", current)
	}
	for index, chapter := range chapters {
		if chapter["position"] != float64(index+1) {
			t.Fatal("reorder produced holes", chapters)
		}
	}
	key = RandID()
	in["chapter_post_ids"] = []string{third, second, first}
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", path, in, 2, key), 200)
	replay := contentWant(t, contentCall(t, a, cookie, csrf, "PUT", path, in, 2, key), 200)
	if replay["version"] != float64(3) {
		t.Fatal("replay bumped version", replay)
	}
	in["title"] = "不同请求"
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", path, in, 2, key), 409)
	var audits int
	if err := a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE target_type='series' AND target_id=$1`, created["id"]).Scan(&audits); err != nil || audits != 3 {
		t.Fatal("replay duplicated audit", audits, err)
	}
}

func TestSeriesValidationAndCursorIsolation(t *testing.T) {
	a := seriesTestApp(t)
	a.RegisterContent()
	p, cookie, csrf := testIdentity(t, a, "author")
	otherPrincipal, other, otherCSRF := testIdentity(t, a, "author")
	first := seriesTestPost(t, a, cookie, csrf, "文章", false)
	for _, in := range []map[string]any{
		{"title": "", "chapter_post_ids": []string{}},
		{"title": "标题", "slug": "a/b"},
		{"title": "标题", "state": "finished"},
		{"title": "标题", "status": "private"},
		{"title": "标题", "chapter_post_ids": []string{first, first}},
		{"title": "标题", "chapter_post_ids": []string{first, "0" + first}},
		{"title": "标题", "chapter_post_ids": []string{"9223372036854775807"}},
		{"title": "标题", "chapter_post_ids": []string{"-2"}},
	} {
		contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/series", in, 0, ""), 400)
	}
	tooMany := make([]string, 301)
	for index := range tooMany {
		tooMany[index] = first
	}
	in := seriesTestInput(tooMany...)
	contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/series", in, 0, ""), 400)
	for range 3 {
		contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/series", seriesTestInput(), 0, ""), 201)
	}
	contentWant(t, contentCall(t, a, other, otherCSRF, "POST", "/api/v1/series", seriesTestInput(), 0, ""), 201)
	q := url.Values{"author_id": {p.ID}, "limit": {"1"}}
	firstPage, err := a.PublicSeries(context.Background(), q)
	if err != nil || firstPage["has_more"] != true || len(webMaps(firstPage["items"])) != 1 {
		t.Fatal("first cursor page", firstPage, err)
	}
	q.Set("cursor", firstPage["next_cursor"].(string))
	secondPage, err := a.PublicSeries(context.Background(), q)
	if err != nil || webMaps(secondPage["items"])[0]["id"] == webMaps(firstPage["items"])[0]["id"] {
		t.Fatal("cursor repeated item", secondPage, err)
	}
	q.Set("author_id", otherPrincipal.ID)
	if _, err = a.PublicSeries(context.Background(), q); err == nil {
		t.Fatal("cursor reused for another author")
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/me/series?limit=1&cursor="+url.QueryEscape(firstPage["next_cursor"].(string)), nil, 0, ""), 400)
	contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/series?cursor=bad", nil, 0, ""), 400)
}

func TestSeriesCoverOwnershipUsageAndPublicRead(t *testing.T) {
	a := seriesTestApp(t)
	a.RegisterMedia()
	author, cookie, csrf := testIdentity(t, a, "author")
	_, other, otherCSRF := testIdentity(t, a, "author")
	media := contentWant(t, mediaTestUpload(t, a, cookie, csrf, RandID(), 43), 201)["media_id"].(string)
	foreign := contentWant(t, mediaTestUpload(t, a, other, otherCSRF, RandID(), 51), 201)["media_id"].(string)
	in := seriesTestInput()
	in["status"] = "draft"
	in["cover_media_id"] = foreign
	contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/series", in, 0, ""), 400)
	in["cover_media_id"] = media
	created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/series", in, 0, ""), 201)
	path := "/api/v1/studio/series/" + created["id"].(string)
	contentWant(t, contentCall(t, a, nil, "", "GET", "/media/"+media, nil, 0, ""), 404)
	contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/media/"+media, nil, 0, ""), 409)
	status := contentWant(t, contentCall(t, a, cookie, csrf, "GET", "/api/v1/media/"+media+"/status", nil, 0, ""), 200)
	if status["in_use"] != true {
		t.Fatal("series cover omitted from usage", status)
	}
	in["status"] = "published"
	contentWant(t, contentCall(t, a, cookie, csrf, "PUT", path, in, 1, ""), 200)
	response := contentCall(t, a, nil, "", "GET", "/media/"+media, nil, 0, "")
	if response.Code != 200 {
		t.Fatal("published cover inaccessible", response.Code, response.Body.String())
	}
	// Disabling an account revokes its sessions; it does not withdraw already
	// published articles/serials. Cover authorization must follow that same
	// public projection instead of producing a broken image on a visible page.
	ctx := context.Background()
	if _, err := a.Pool.Exec(ctx, `UPDATE users SET disabled_at_ms=$2 WHERE id=$1`, author.ID, NowMillis()); err != nil {
		t.Fatal(err)
	}
	public, err := a.PublicSeriesDetail(ctx, created["id"].(string))
	if err != nil || public["cover_url"] != "/media/"+media {
		t.Fatal("account suspension silently withdrew a published serial", public, err)
	}
	if response := contentCall(t, a, nil, "", "GET", "/media/"+media, nil, 0, ""); response.Code != 200 {
		t.Fatal("published serial retained a broken cover after author suspension", response.Code, response.Body.String())
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "GET", path, nil, 0, ""), 401)
	if _, err := a.Pool.Exec(ctx, `UPDATE users SET disabled_at_ms=NULL WHERE id=$1`, author.ID); err != nil {
		t.Fatal(err)
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", path, nil, 2, ""), 200)
	contentWant(t, contentCall(t, a, nil, "", "GET", "/media/"+media, nil, 0, ""), 404)
	if response := contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/media/"+media, nil, 0, ""); response.Code != 204 {
		t.Fatal("released cover cannot be deleted", response.Code, response.Body.String())
	}
}

func TestSeriesDatabaseIdentityOwnershipAndPositionGuards(t *testing.T) {
	a := seriesTestApp(t)
	a.RegisterContent()
	p, cookie, csrf := testIdentity(t, a, "author")
	otherPrincipal, other, otherCSRF := testIdentity(t, a, "author")
	first := seriesTestPost(t, a, cookie, csrf, "第一篇", false)
	second := seriesTestPost(t, a, cookie, csrf, "第二篇", false)
	foreign := seriesTestPost(t, a, other, otherCSRF, "外部文章", false)
	value := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/series", seriesTestInput(first), 0, ""), 201)
	id := value["id"].(string)
	for _, mutation := range []struct {
		sql  string
		args []any
		code string
	}{
		{`UPDATE series SET author_user_id=$2,version=version+1 WHERE id=$1`, []any{id, otherPrincipal.ID}, "23514"},
		{`UPDATE series SET version=version+2 WHERE id=$1`, []any{id}, "23514"},
		{`UPDATE series SET created_at_ms=created_at_ms-1,version=version+1 WHERE id=$1`, []any{id}, "23514"},
		{`INSERT INTO series_chapters(series_id,post_id,position) VALUES($1,$2,2)`, []any{id, foreign}, "23514"},
		{`INSERT INTO series_chapters(series_id,post_id,position) VALUES($1,$2,301)`, []any{id, second}, "23514"},
		{`INSERT INTO series_chapters(series_id,post_id,position) VALUES($1,$2,1)`, []any{id, second}, "23505"},
		{`INSERT INTO series_chapters(series_id,post_id,position) VALUES($1,$2,2)`, []any{id, first}, "23505"},
	} {
		_, err := a.Pool.Exec(context.Background(), mutation.sql, mutation.args...)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != mutation.code {
			t.Fatal("database failed to enforce serial aggregate constraints", mutation.sql, err)
		}
	}
	current, err := a.privateSeries(context.Background(), a.Pool, p, id)
	if err != nil || current["version"] != float64(1) || len(seriesChapters(t, current)) != 1 {
		t.Fatal("rejected writes changed serial", current, err)
	}
}
