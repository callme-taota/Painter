package app

import (
	"context"
	"net/http"
	"testing"
)

func interactionPublished(t *testing.T, a *App, cookie *http.Cookie, csrf string) (string, string) {
	t.Helper()
	created := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "公开互动文章", "markdown": "正文"}, 0, ""), 201)
	id := created["id"].(string)
	published := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+id+"/publish", map[string]any{"expected_draft_version": 1}, 1, ""), 200)
	return id, published["revision_id"].(string)
}
func TestInteractionsIdempotencyVisibilityAndModeration(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterInteractions()
	a.RegisterAdmin()
	_, author, ac := testIdentity(t, a, "owner")
	reader, rcookie, rc := testIdentity(t, a, "reader")
	_, other, oc := testIdentity(t, a, "reader")
	post, _ := interactionPublished(t, a, author, ac)
	for i := 0; i < 2; i++ {
		w := contentCall(t, a, rcookie, rc, "PUT", "/api/v1/posts/"+post+"/like", nil, 0, "")
		if w.Code != 204 {
			t.Fatal(w.Code, w.Body.String())
		}
		w = contentCall(t, a, rcookie, rc, "PUT", "/api/v1/posts/"+post+"/bookmark", nil, 0, "")
		if w.Code != 204 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	m := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+post+"/metrics", nil, 0, ""), 200)
	if m["like_count"] != float64(1) {
		t.Fatal(m)
	}
	key := RandID()
	body := map[string]any{"body": "纯文本 <script>不会运行</script>"}
	c := contentWant(t, contentCall(t, a, rcookie, rc, "POST", "/api/v1/posts/"+post+"/comments", body, 0, key), 201)
	again := contentWant(t, contentCall(t, a, rcookie, rc, "POST", "/api/v1/posts/"+post+"/comments", body, 0, key), 201)
	if c["id"] != again["id"] {
		t.Fatal("comment duplicated")
	}
	id := c["id"].(string)
	w := contentCall(t, a, other, oc, "DELETE", "/api/v1/comments/"+id, nil, 0, "")
	if w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	contentWant(t, contentCall(t, a, author, ac, "PUT", "/api/v1/admin/comments/"+id+"/visibility", map[string]any{"state": "hidden", "reason": "管理测试"}, 0, ""), 200)
	visible := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+post+"/comments", nil, 0, ""), 200)
	if len(visible["items"].([]any)) != 0 {
		t.Fatal("hidden comment leaked")
	}
	w = contentCall(t, a, rcookie, rc, "PUT", "/api/v1/comments/"+id+"/like", nil, 0, "")
	if w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	contentWant(t, contentCall(t, a, author, ac, "DELETE", "/api/v1/studio/posts/"+post+"/publication", nil, 2, ""), 200)
	w = contentCall(t, a, rcookie, rc, "PUT", "/api/v1/posts/"+post+"/like", nil, 0, "")
	if w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	b := contentWant(t, contentCall(t, a, rcookie, rc, "GET", "/api/v1/me/bookmarks", nil, 0, ""), 200)
	if len(b["items"].([]any)) != 0 {
		t.Fatal("withdrawn bookmark leaked")
	}
	var n int
	if e := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM bookmarks WHERE user_id=$1", reader.ID).Scan(&n); e != nil || n != 1 {
		t.Fatal("withdrawal destroyed bookmark", n, e)
	}
}
func TestFollowAndSignedPrivateCursor(t *testing.T) {
	a := testApp(t)
	a.RegisterInteractions()
	_, c, csrf := testIdentity(t, a, "reader")
	u2, _, _ := testIdentity(t, a, "reader")
	u3, _, _ := testIdentity(t, a, "reader")
	for _, id := range []string{u2.ID, u3.ID} {
		w := contentCall(t, a, c, csrf, "PUT", "/api/v1/users/"+id+"/follow", nil, 0, "")
		if w.Code != 204 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	page := contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/me/following?limit=1", nil, 0, ""), 200)
	cursor := page["next_cursor"].(string)
	if cursor == "" {
		t.Fatal("missing cursor")
	}
	contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/me/following?limit=1&cursor="+cursor, nil, 0, ""), 200)
	w := contentCall(t, a, c, csrf, "GET", "/api/v1/me/followers?cursor="+cursor, nil, 0, "")
	if w.Code != 400 {
		t.Fatal("cursor route substitution accepted", w.Code, w.Body.String())
	}
}
