package app

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

type followingFeedActor struct {
	user   Principal
	cookie *http.Cookie
	csrf   string
}

func followingFeedIdentity(t *testing.T, a *App, role string) followingFeedActor {
	t.Helper()
	u, c, csrf := testIdentity(t, a, role)
	return followingFeedActor{u, c, csrf}
}

func followingFeedApp(t *testing.T) *App {
	t.Helper()
	a := momentImageApp(t)
	a.RegisterContent()
	a.RegisterInteractions()
	return a
}

func followingFeedFollow(t *testing.T, a *App, reader, author followingFeedActor, follow bool) {
	t.Helper()
	method := "PUT"
	if !follow {
		method = "DELETE"
	}
	w := contentCall(t, a, reader.cookie, reader.csrf, method, "/api/v1/users/"+author.user.ID+"/follow", nil, 0, "")
	if w.Code != 204 {
		t.Fatal("follow mutation failed", w.Code, w.Body.String())
	}
}

func followingFeedPost(t *testing.T, a *App, author followingFeedActor, title, summary string, stamp int64) string {
	t.Helper()
	created := contentWant(t, contentCall(t, a, author.cookie, author.csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": title, "summary": summary, "markdown": "公开正文 fallback body"}, 0, ""), 201)
	id := created["id"].(string)
	contentWant(t, contentCall(t, a, author.cookie, author.csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	if _, e := a.Pool.Exec(context.Background(), `UPDATE posts SET first_published_at_ms=$2,version=version+1 WHERE id=$1`, id, stamp); e != nil {
		t.Fatal(e)
	}
	return id
}

func followingFeedMoment(t *testing.T, a *App, author followingFeedActor, text string, stamp int64) string {
	t.Helper()
	item := momentFixture(t, a, author.cookie, author.csrf, text, true)
	id := item["id"].(string)
	if _, e := a.Pool.Exec(context.Background(), `UPDATE moments SET first_published_at_ms=$2 WHERE id=$1`, id, stamp); e != nil {
		t.Fatal(e)
	}
	return id
}

func followingFeedRead(t *testing.T, a *App, actor followingFeedActor, query string) map[string]any {
	t.Helper()
	w := contentCall(t, a, actor.cookie, "", "GET", "/api/v1/me/feed"+query, nil, 0, "")
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("personal feed may be cached", w.Header())
	}
	return contentWant(t, w, 200)
}

func followingFeedItems(page map[string]any) []map[string]any {
	items := []map[string]any{}
	for _, item := range page["items"].([]any) {
		items = append(items, item.(map[string]any))
	}
	return items
}

func TestFollowingFeedPublicSnapshotsCoversAndDTO(t *testing.T) {
	a := followingFeedApp(t)
	followingFeedIdentity(t, a, "owner")
	author, reader, other := followingFeedIdentity(t, a, "author"), followingFeedIdentity(t, a, "reader"), followingFeedIdentity(t, a, "author")
	followingFeedFollow(t, a, reader, author, true)
	post := followingFeedPost(t, a, author, "公开文章", strings.Repeat("🌿", 300), 3000)
	first := momentImageUpload(t, a, author.cookie, author.csrf, 1)
	second := momentImageUpload(t, a, author.cookie, author.csrf, 2)
	private := momentImageUpload(t, a, author.cookie, author.csrf, 3)
	moment := momentImageFixture(t, a, author.cookie, author.csrf, strings.Repeat("🌳", 300), []momentImage{{second, "第二张先展示"}, {first, "第一张后展示"}})
	id := moment["id"].(string)
	momentResponse(t, momentCall(t, a, author.cookie, author.csrf, "POST", "/api/v1/studio/moments/"+id+"/publish", nil, 1, RandID()), 200)
	if _, e := a.Pool.Exec(context.Background(), `UPDATE moments SET first_published_at_ms=2000 WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	imageOnly := momentImageFixture(t, a, author.cookie, author.csrf, "", []momentImage{{first, "纯图片动态"}})
	imageOnlyID := imageOnly["id"].(string)
	momentResponse(t, momentCall(t, a, author.cookie, author.csrf, "POST", "/api/v1/studio/moments/"+imageOnlyID+"/publish", nil, 1, RandID()), 200)
	if _, e := a.Pool.Exec(context.Background(), `UPDATE moments SET first_published_at_ms=1000 WHERE id=$1`, imageOnlyID); e != nil {
		t.Fatal(e)
	}
	contentWant(t, contentCall(t, a, author.cookie, author.csrf, "PUT", "/api/v1/studio/posts/"+post+"/draft", map[string]any{"title": "private-draft-title", "summary": "private-draft-summary", "markdown": "private-draft-body", "expected_draft_version": 1, "cover": map[string]any{"mode": "none"}}, 3, ""), 200)
	momentResponse(t, momentCall(t, a, author.cookie, author.csrf, "PUT", "/api/v1/studio/moments/"+id+"/draft", map[string]any{"text": "private-moment-edit", "images": []momentImage{{private, "private-alt"}}}, 2, RandID()), 200)
	momentFixture(t, a, author.cookie, author.csrf, "private-unpublished-moment", false)
	contentWant(t, contentCall(t, a, author.cookie, author.csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "private-unpublished-post", "markdown": "private-unpublished-body"}, 0, ""), 201)
	followingFeedPost(t, a, other, "unfollowed-post", "unfollowed-summary", 5000)
	followingFeedMoment(t, a, other, "unfollowed-moment", 5000)
	if _, e := a.Pool.Exec(context.Background(), `UPDATE users SET email='private-contact@example.invalid',phone='private-phone',display_name='当前作者昵称' WHERE id=$1`, author.user.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := a.Pool.Exec(context.Background(), `INSERT INTO phrases(text,kind,source,published,created_at_ms,updated_at_ms) VALUES('unowned-phrase','sentence','',true,1,1)`); e != nil {
		t.Fatal(e)
	}
	// A published serial containing this article must not create another event.
	var serial string
	if e := a.Pool.QueryRow(context.Background(), `INSERT INTO series(author_user_id,title,slug,status,created_at_ms,updated_at_ms) VALUES($1,'series-not-an-extra-event','feed-test-series','published',1,1) RETURNING id::text`, author.user.ID).Scan(&serial); e != nil {
		t.Fatal(e)
	}
	if _, e := a.Pool.Exec(context.Background(), `INSERT INTO series_chapters(series_id,post_id,position) VALUES($1,$2,1)`, serial, post); e != nil {
		t.Fatal(e)
	}
	page := followingFeedRead(t, a, reader, "")
	items := followingFeedItems(page)
	if len(page) != 7 || len(items) != 3 || page["following_count"] != float64(1) || page["viewer_id"] != reader.user.ID || page["type"] != "all" || !reflect.DeepEqual(page["available_types"], []any{"post", "moment"}) || page["next_cursor"] != "" || page["has_more"] != false {
		t.Fatal("unexpected feed envelope, ownership or duplicate series event", page)
	}
	publicPost := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+post, nil, 0, ""), 200)
	if items[0]["id"] != "post:"+post || items[0]["content_id"] != post || items[0]["title"] != "公开文章" || items[0]["cover_url"] != publicPost["cover_url"] || items[0]["image_count"] != float64(0) || items[0]["published_at"] != TimeString(3000) {
		t.Fatal("article projection or first-publication date differs", items[0])
	}
	if items[1]["id"] != "moment:"+id || items[1]["title"] != "" || items[1]["cover_url"] != "/media/"+second || items[1]["image_count"] != float64(2) || items[1]["url"] != "/moments/"+id {
		t.Fatal("moment uses draft images or loses public image order/count", items[1])
	}
	if items[2]["id"] != "moment:"+imageOnlyID || items[2]["summary"] != "" || items[2]["cover_url"] != "/media/"+first || items[2]["image_count"] != float64(1) {
		t.Fatal("image-only public moment lost its cover or acquired draft text", items[2])
	}
	for index, item := range items {
		if len(item) != 10 || len(item["author"].(map[string]any)) != 3 || item["author"].(map[string]any)["display_name"] != "当前作者昵称" || utf8.RuneCountInString(item["summary"].(string)) > followingFeedSummaryLimit || !utf8.ValidString(item["summary"].(string)) || (index < 2 && utf8.RuneCountInString(item["summary"].(string)) != followingFeedSummaryLimit) {
			t.Fatal("unbounded/private DTO fields or stale author profile", item)
		}
	}
	for _, query := range []string{"", "?type=post", "?type=moment"} {
		w := contentCall(t, a, reader.cookie, "", "GET", "/api/v1/me/feed"+query, nil, 0, "")
		contentWant(t, w, 200)
		for _, secret := range []string{"private-", private, "unfollowed-", "unowned-phrase", "series-not-an-extra-event"} {
			if strings.Contains(w.Body.String(), secret) {
				t.Fatal("private, unfollowed or unowned content leaked", secret, w.Body.String())
			}
		}
	}
}

func TestFollowingFeedStableMixedPaginationAndSignedScope(t *testing.T) {
	a := followingFeedApp(t)
	followingFeedIdentity(t, a, "owner")
	author, reader, other := followingFeedIdentity(t, a, "author"), followingFeedIdentity(t, a, "reader"), followingFeedIdentity(t, a, "reader")
	followingFeedFollow(t, a, reader, author, true)
	followingFeedFollow(t, a, other, author, true)
	posts, moments := []string{}, []string{}
	for range 3 {
		posts = append(posts, followingFeedPost(t, a, author, "同时间文章", "", 1000))
		moments = append(moments, followingFeedMoment(t, a, author, "同时间动态", 1000))
	}
	if posts[0] != moments[0] {
		t.Fatal("fixture did not create overlapping content IDs")
	}
	page := followingFeedRead(t, a, reader, "?limit=1")
	firstCursor := page["next_cursor"].(string)
	seen := []string{followingFeedItems(page)[0]["id"].(string)}
	followingFeedPost(t, a, author, "分页过程中发布的新文章", "", 2000)
	for page["has_more"] == true {
		page = followingFeedRead(t, a, reader, "?limit=1&cursor="+url.QueryEscape(page["next_cursor"].(string)))
		for _, item := range followingFeedItems(page) {
			seen = append(seen, item["id"].(string))
		}
		if len(seen) > 6 {
			t.Fatal("pagination duplicated or admitted newer publications", seen)
		}
	}
	want := []string{"post:" + posts[2], "post:" + posts[1], "post:" + posts[0], "moment:" + moments[2], "moment:" + moments[1], "moment:" + moments[0]}
	if !reflect.DeepEqual(seen, want) || page["next_cursor"] != "" {
		t.Fatal("same-time cross-type order skipped or duplicated items", seen, want)
	}
	for _, query := range []string{"?type=post&limit=1&cursor=" + firstCursor, "?type=moment&limit=1&cursor=" + firstCursor, "?limit=2&cursor=" + firstCursor, "?limit=1&cursor=" + firstCursor + "tampered"} {
		contentWant(t, contentCall(t, a, reader.cookie, "", "GET", "/api/v1/me/feed"+query, nil, 0, ""), 400)
	}
	contentWant(t, contentCall(t, a, other.cookie, "", "GET", "/api/v1/me/feed?limit=1&cursor="+firstCursor, nil, 0, ""), 400)
	for _, kind := range []string{"post", "moment"} {
		filtered := followingFeedRead(t, a, reader, "?type="+kind)
		for _, item := range followingFeedItems(filtered) {
			if item["type"] != kind {
				t.Fatal("type filter included another content type", filtered)
			}
		}
	}
}

func TestFollowingFeedRechecksVisibilityAndFollowingBetweenPages(t *testing.T) {
	a := followingFeedApp(t)
	followingFeedIdentity(t, a, "owner")
	author, reader := followingFeedIdentity(t, a, "author"), followingFeedIdentity(t, a, "reader")
	followingFeedFollow(t, a, reader, author, true)
	followingFeedPost(t, a, author, "first-page", "", 6000)
	unpublish := followingFeedPost(t, a, author, "withdraw-before-next-page", "", 5000)
	trash := followingFeedPost(t, a, author, "trash-before-next-page", "", 4000)
	remaining := followingFeedPost(t, a, author, "still-public", "", 3000)
	unpublishMoment := followingFeedMoment(t, a, author, "withdraw-moment-before-next-page", 2000)
	deleteMoment := followingFeedMoment(t, a, author, "delete-moment-before-next-page", 1000)
	first := followingFeedRead(t, a, reader, "?limit=1")
	query := "?limit=1&cursor=" + first["next_cursor"].(string)
	contentWant(t, contentCall(t, a, author.cookie, author.csrf, "DELETE", "/api/v1/studio/posts/"+unpublish+"/publication", nil, 3, ""), 200)
	contentWant(t, contentCall(t, a, author.cookie, author.csrf, "DELETE", "/api/v1/studio/posts/"+trash, nil, 3, ""), 200)
	momentResponse(t, momentCall(t, a, author.cookie, author.csrf, "DELETE", "/api/v1/studio/moments/"+unpublishMoment+"/publication", nil, 2, RandID()), 200)
	momentResponse(t, momentCall(t, a, author.cookie, author.csrf, "DELETE", "/api/v1/studio/moments/"+deleteMoment, nil, 2, RandID()), 200)
	page := followingFeedRead(t, a, reader, query)
	if len(followingFeedItems(page)) != 1 || followingFeedItems(page)[0]["id"] != "post:"+remaining || page["has_more"] != false {
		t.Fatal("cursor retained withdrawn or deleted content", page)
	}
	followingFeedFollow(t, a, reader, author, false)
	page = followingFeedRead(t, a, reader, query)
	if len(followingFeedItems(page)) != 0 || page["following_count"] != float64(0) {
		t.Fatal("old cursor retained a cancelled follow", page)
	}
	followingFeedFollow(t, a, reader, author, true)
	if _, e := a.Pool.Exec(context.Background(), `UPDATE users SET disabled_at_ms=$2 WHERE id=$1`, author.user.ID, NowMillis()); e != nil {
		t.Fatal(e)
	}
	page = followingFeedRead(t, a, reader, query)
	if len(followingFeedItems(page)) != 0 || page["following_count"] != float64(0) {
		t.Fatal("old cursor retained disabled author publications", page)
	}
}

func TestFollowingFeedFirstPublicationSurvivesEditsAndRepublication(t *testing.T) {
	a := followingFeedApp(t)
	followingFeedIdentity(t, a, "owner")
	author, reader := followingFeedIdentity(t, a, "author"), followingFeedIdentity(t, a, "reader")
	followingFeedFollow(t, a, reader, author, true)
	post := followingFeedPost(t, a, author, "older article", "", 1000)
	moment := followingFeedMoment(t, a, author, "newer moment", 2000)
	first := followingFeedRead(t, a, reader, "?limit=1")
	contentWant(t, contentCall(t, a, author.cookie, author.csrf, "PUT", "/api/v1/studio/posts/"+post+"/draft", map[string]any{"title": "updated public article", "summary": "new public summary", "markdown": "new public body", "expected_draft_version": 1, "cover": map[string]any{"mode": "none"}}, 3, ""), 200)
	contentWant(t, contentCall(t, a, author.cookie, author.csrf, "POST", "/api/v1/studio/posts/"+post+"/publish", map[string]any{"expected_draft_version": 2}, 4, ""), 200)
	momentResponse(t, momentCall(t, a, author.cookie, author.csrf, "PUT", "/api/v1/studio/moments/"+moment+"/draft", map[string]any{"text": "updated public moment"}, 2, RandID()), 200)
	momentResponse(t, momentCall(t, a, author.cookie, author.csrf, "POST", "/api/v1/studio/moments/"+moment+"/publish", nil, 3, RandID()), 200)
	page := followingFeedRead(t, a, reader, "")
	items := followingFeedItems(page)
	if len(items) != 2 || items[0]["id"] != "moment:"+moment || items[0]["published_at"] != TimeString(2000) || items[0]["summary"] != "updated public moment" || items[1]["id"] != "post:"+post || items[1]["published_at"] != TimeString(1000) || items[1]["title"] != "updated public article" || items[1]["cover_url"] != "" {
		t.Fatal("publication edits bumped order, duplicated content or reused old snapshot", page)
	}
	next := followingFeedRead(t, a, reader, "?limit=1&cursor="+first["next_cursor"].(string))
	if len(followingFeedItems(next)) != 1 || followingFeedItems(next)[0]["id"] != "post:"+post {
		t.Fatal("editing an older item made the existing cursor skip it", next)
	}
	contentWant(t, contentCall(t, a, author.cookie, author.csrf, "DELETE", "/api/v1/studio/posts/"+post+"/publication", nil, 5, ""), 200)
	contentWant(t, contentCall(t, a, author.cookie, author.csrf, "POST", "/api/v1/studio/posts/"+post+"/publish", map[string]any{"expected_draft_version": 2}, 6, ""), 200)
	momentResponse(t, momentCall(t, a, author.cookie, author.csrf, "DELETE", "/api/v1/studio/moments/"+moment+"/publication", nil, 4, RandID()), 200)
	momentResponse(t, momentCall(t, a, author.cookie, author.csrf, "POST", "/api/v1/studio/moments/"+moment+"/publish", nil, 5, RandID()), 200)
	items = followingFeedItems(followingFeedRead(t, a, reader, ""))
	if len(items) != 2 || items[0]["published_at"] != TimeString(2000) || items[1]["published_at"] != TimeString(1000) {
		t.Fatal("withdrawal and republication bumped feed entries", items)
	}
}

func TestFollowingFeedAuthenticationEmptyStateAndValidation(t *testing.T) {
	a := followingFeedApp(t)
	followingFeedIdentity(t, a, "owner")
	author, reader := followingFeedIdentity(t, a, "author"), followingFeedIdentity(t, a, "reader")
	contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/me/feed", nil, 0, ""), 401)
	page := followingFeedRead(t, a, reader, "")
	if len(followingFeedItems(page)) != 0 || page["following_count"] != float64(0) || page["next_cursor"] != "" || page["has_more"] != false {
		t.Fatal("empty personal feed shape changed", page)
	}
	followingFeedFollow(t, a, reader, author, true)
	page = followingFeedRead(t, a, reader, "")
	if len(followingFeedItems(page)) != 0 || page["following_count"] != float64(1) {
		t.Fatal("cannot distinguish no follow from no publication", page)
	}
	for _, query := range []string{"?type=phrase", "?type=series", "?type=post&type=moment", "?limit=0", "?limit=-1", "?limit=bad", "?limit=1&limit=2", "?cursor=bad", "?cursor=" + strings.Repeat("x", 5000), "?cursor=a&cursor=b"} {
		contentWant(t, contentCall(t, a, reader.cookie, "", "GET", "/api/v1/me/feed"+query, nil, 0, ""), 400)
	}
	if _, e := a.Pool.Exec(context.Background(), `UPDATE site_settings SET follows_enabled=false,version=version+1 WHERE id=1`); e != nil {
		t.Fatal(e)
	}
	page = followingFeedRead(t, a, reader, "?limit=999")
	if page["following_count"] != float64(1) {
		t.Fatal("closing new follows destroyed existing personal feed", page)
	}
	if _, e := a.Pool.Exec(context.Background(), `UPDATE users SET disabled_at_ms=$2 WHERE id=$1`, reader.user.ID, NowMillis()); e != nil {
		t.Fatal(e)
	}
	contentWant(t, contentCall(t, a, reader.cookie, "", "GET", "/api/v1/me/feed", nil, 0, ""), 401)
}
