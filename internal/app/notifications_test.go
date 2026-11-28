package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func notificationInbox(t *testing.T, a *App, actor commentReplyActor, query string) map[string]any {
	t.Helper()
	w := contentCall(t, a, actor.cookie, actor.csrf, "GET", "/api/v1/me/notifications"+query, nil, 0, "")
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("private inbox cache", w.Header())
	}
	return contentWant(t, w, 200)
}
func TestNotificationsRecipientsReplayAndReadIsolation(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterInteractions()
	owner, reader, other := commentReplyIdentity(t, a, "owner"), commentReplyIdentity(t, a, "reader"), commentReplyIdentity(t, a, "reader")
	post, _ := interactionPublished(t, a, owner.cookie, owner.csrf)
	root := commentReplyCreate(t, a, reader, post, "", "root-secret")
	notificationInbox(t, a, reader, "")
	own := notificationInbox(t, a, owner, "")
	if own["unread_count"] != float64(1) {
		t.Fatal(own)
	}
	item := commentReplyItem(t, own, 0)
	if item["kind"] != "comment" || item["body"] != "root-secret" || item["available"] != true || !strings.Contains(item["url"].(string), "?comment="+root["id"].(string)) {
		t.Fatal(item)
	}
	if notificationInbox(t, a, reader, "")["unread_count"] != float64(0) {
		t.Fatal("self notification")
	}
	key := RandID()
	payload := map[string]any{"body": "reply-secret", "reply_to_id": root["id"]}
	first := contentWant(t, contentCall(t, a, other.cookie, other.csrf, "POST", "/api/v1/posts/"+post+"/comments", payload, 0, key), 201)
	contentWant(t, contentCall(t, a, other.cookie, other.csrf, "POST", "/api/v1/posts/"+post+"/comments", payload, 0, key), 201)
	if notificationInbox(t, a, owner, "")["unread_count"] != float64(2) || notificationInbox(t, a, reader, "")["unread_count"] != float64(1) || notificationInbox(t, a, other, "")["unread_count"] != float64(0) {
		t.Fatal("wrong recipients or duplicate delivery")
	}
	reply := commentReplyItem(t, notificationInbox(t, a, reader, ""), 0)
	if reply["kind"] != "reply" || reply["comment_id"] != first["id"] {
		t.Fatal(reply)
	}
	ownerRoot := commentReplyCreate(t, a, owner, post, "", "author root")
	commentReplyCreate(t, a, reader, post, ownerRoot["id"].(string), "reply to author")
	own = notificationInbox(t, a, owner, "")
	if own["unread_count"] != float64(3) || commentReplyItem(t, own, 0)["kind"] != "reply" {
		t.Fatal("author/target duplicated", own)
	}
	readURL := "/api/v1/me/notifications/" + item["id"].(string) + "/read"
	commentReplyStatus(t, contentCall(t, a, reader.cookie, reader.csrf, "PUT", readURL, nil, 0, ""), 404)
	commentReplyStatus(t, contentCall(t, a, nil, "", "GET", "/api/v1/me/notifications", nil, 0, ""), 401)
	commentReplyStatus(t, contentCall(t, a, owner.cookie, "", "PUT", readURL, nil, 0, ""), 403)
	for range 2 {
		commentReplyStatus(t, contentCall(t, a, owner.cookie, owner.csrf, "PUT", readURL, nil, 0, ""), 204)
	}
	if notificationInbox(t, a, owner, "")["unread_count"] != float64(2) {
		t.Fatal("mark read is not idempotent")
	}
	var n int
	if err := a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM notifications WHERE comment_id=$1`, first["id"]).Scan(&n); err != nil || n != 2 {
		t.Fatal(n, err)
	}
}

func TestNotificationsCurrentVisibilityAndContext(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterInteractions()
	a.RegisterAdmin()
	a.RegisterWeb()
	owner, reader, other := commentReplyIdentity(t, a, "owner"), commentReplyIdentity(t, a, "reader"), commentReplyIdentity(t, a, "reader")
	post, _ := interactionPublished(t, a, owner.cookie, owner.csrf)
	root := commentReplyCreate(t, a, reader, post, "", "root-original-secret")
	target := commentReplyCreate(t, a, other, post, root["id"].(string), "target-secret")
	reply := commentReplyCreate(t, a, owner, post, target["id"].(string), "reply-original-secret")
	contextURL := "/api/v1/posts/" + post + "/comments/" + reply["id"].(string) + "/context"
	data := contentWant(t, contentCall(t, a, nil, "", "GET", contextURL, nil, 0, ""), 200)
	if data["root"].(map[string]any)["id"] != root["id"] || data["target"].(map[string]any)["id"] != reply["id"] {
		t.Fatal(data)
	}
	otherPost, _ := interactionPublished(t, a, owner.cookie, owner.csrf)
	commentReplyStatus(t, contentCall(t, a, nil, "", "GET", strings.Replace(contextURL, "/posts/"+post+"/", "/posts/"+otherPost+"/", 1), nil, 0, ""), 404)
	// Hiding the replied-to comment removes all identity and cached text from its notification.
	contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "PUT", "/api/v1/admin/comments/"+target["id"].(string)+"/visibility", map[string]any{"state": "hidden"}, 0, ""), 200)
	inbox := notificationInbox(t, a, other, "")
	item := commentReplyItem(t, inbox, 0)
	if item["available"] != false {
		t.Fatal(item)
	}
	encoded, _ := json.Marshal(item)
	for _, field := range []string{"actor", "post_title", "post_id", "comment_id", "url", "body", "original-secret"} {
		if strings.Contains(string(encoded), field) {
			t.Fatal("unavailable source leaked", string(encoded))
		}
	}
	contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "PUT", "/api/v1/admin/comments/"+root["id"].(string)+"/visibility", map[string]any{"state": "hidden"}, 0, ""), 200)
	commentReplyStatus(t, contentCall(t, a, nil, "", "GET", contextURL, nil, 0, ""), 404)
	for _, r := range notificationInbox(t, a, reader, "")["items"].([]any) {
		if r.(map[string]any)["available"] != false {
			t.Fatal("hidden thread leaked", r)
		}
	}
	contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "PUT", "/api/v1/admin/comments/"+root["id"].(string)+"/visibility", map[string]any{"state": "visible"}, 0, ""), 200)
	commentReplyStatus(t, contentCall(t, a, reader.cookie, reader.csrf, "DELETE", "/api/v1/comments/"+root["id"].(string), nil, 0, ""), 204)
	commentReplyStatus(t, contentCall(t, a, nil, "", "GET", contextURL, nil, 0, ""), 404)
	contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "DELETE", "/api/v1/studio/posts/"+post+"/publication", nil, 2, ""), 200)
	for _, r := range notificationInbox(t, a, owner, "")["items"].([]any) {
		if r.(map[string]any)["available"] != false {
			t.Fatal("withdrawn post leaked", r)
		}
	}
	request := httptest.NewRequest("GET", "/me/notifications", nil)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, request)
	if w.Code != http.StatusSeeOther || w.Header().Get("Cache-Control") != "private, no-store" || !strings.Contains(w.Header().Get("Location"), "return_to=") {
		t.Fatal(w.Code, w.Header())
	}
	request = httptest.NewRequest("GET", "/me/notifications", nil)
	request.AddCookie(reader.cookie)
	w = httptest.NewRecorder()
	a.Handler().ServeHTTP(w, request)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "private, no-store" || w.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatal(w.Code, w.Header())
	}
}

func TestNotificationsPaginationScopeAndAtomicRollback(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterInteractions()
	owner, reader := commentReplyIdentity(t, a, "owner"), commentReplyIdentity(t, a, "reader")
	post, _ := interactionPublished(t, a, owner.cookie, owner.csrf)
	for range 3 {
		commentReplyCreate(t, a, reader, post, "", "notification")
	}
	page := notificationInbox(t, a, owner, "?limit=1")
	cursor := page["next_cursor"].(string)
	if page["has_more"] != true || cursor == "" {
		t.Fatal(page)
	}
	second := notificationInbox(t, a, owner, "?limit=1&cursor="+cursor)
	if commentReplyItem(t, page, 0)["id"] == commentReplyItem(t, second, 0)["id"] {
		t.Fatal("duplicate page")
	}
	for _, path := range []string{"?limit=1&filter=unread&cursor=" + cursor, "?limit=2&cursor=" + cursor, "?limit=1&cursor=bad", "?filter=invalid"} {
		commentReplyStatus(t, contentCall(t, a, owner.cookie, owner.csrf, "GET", "/api/v1/me/notifications"+path, nil, 0, ""), 400)
	}
	commentReplyStatus(t, contentCall(t, a, reader.cookie, reader.csrf, "GET", "/api/v1/me/notifications?limit=1&cursor="+cursor, nil, 0, ""), 400)
	read := commentReplyItem(t, page, 0)["id"].(string)
	commentReplyStatus(t, contentCall(t, a, owner.cookie, owner.csrf, "PUT", "/api/v1/me/notifications/"+read+"/read", nil, 0, ""), 204)
	if len(notificationInbox(t, a, owner, "?filter=unread")["items"].([]any)) != 2 {
		t.Fatal("unread filter")
	}
	// Delivery failure must roll back comment, fact and ledger together.
	_, err := a.Pool.Exec(context.Background(), `CREATE FUNCTION fail_notification() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'notification failure'; END $$; CREATE TRIGGER fail_delivery BEFORE INSERT ON notifications FOR EACH ROW EXECUTE FUNCTION fail_notification()`)
	if err != nil {
		t.Fatal(err)
	}
	key := RandID()
	payload := map[string]any{"body": "must roll back"}
	commentReplyStatus(t, contentCall(t, a, reader.cookie, reader.csrf, "POST", "/api/v1/posts/"+post+"/comments", payload, 0, key), 500)
	var n int
	if err = a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM comments WHERE body='must roll back'`).Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	_, err = a.Pool.Exec(context.Background(), `DROP TRIGGER fail_delivery ON notifications`)
	if err != nil {
		t.Fatal(err)
	}
	contentWant(t, contentCall(t, a, reader.cookie, reader.csrf, "POST", "/api/v1/posts/"+post+"/comments", payload, 0, key), 201)
	if notificationInbox(t, a, owner, "")["unread_count"] != float64(3) {
		t.Fatal("recovered delivery duplicated or lost")
	}
}
