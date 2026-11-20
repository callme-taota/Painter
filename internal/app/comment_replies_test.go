package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type commentReplyActor struct {
	user   Principal
	cookie *http.Cookie
	csrf   string
}

func commentReplyIdentity(t *testing.T, a *App, role string) commentReplyActor {
	t.Helper()
	u, c, csrf := testIdentity(t, a, role)
	return commentReplyActor{u, c, csrf}
}

func commentReplyCreate(t *testing.T, a *App, actor commentReplyActor, post, target, text string) map[string]any {
	t.Helper()
	body := map[string]any{"body": text}
	if target != "" {
		body["reply_to_id"] = target
	}
	return contentWant(t, contentCall(t, a, actor.cookie, actor.csrf, "POST", "/api/v1/posts/"+post+"/comments", body, 0, RandID()), 201)
}

func commentReplyGet(t *testing.T, a *App, path string) map[string]any {
	t.Helper()
	w := contentCall(t, a, nil, "", "GET", path, nil, 0, "")
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("comment state may be cached across readers", w.Header())
	}
	return contentWant(t, w, 200)
}

func commentReplyItem(t *testing.T, page map[string]any, index int) map[string]any {
	t.Helper()
	items := page["items"].([]any)
	if index >= len(items) {
		t.Fatal("missing comment", page)
	}
	return items[index].(map[string]any)
}

func commentReplyStatus(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("expected HTTP %d got %d: %s", status, w.Code, w.Body.String())
	}
}

func TestCommentRepliesShapeTargetAndIdempotency(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterInteractions()
	owner, reader := commentReplyIdentity(t, a, "owner"), commentReplyIdentity(t, a, "reader")
	post, _ := interactionPublished(t, a, owner.cookie, owner.csrf)
	root := commentReplyCreate(t, a, reader, post, "", "  root comment  ")
	rootID := root["id"].(string)
	if root["body"] != "root comment" || root["parent_id"] != nil || root["reply_to"] != nil || root["reply_count"] != float64(0) || root["is_author"] != false || root["is_self"] != true || root["reply_to_deleted"] != false {
		t.Fatal("legacy root shape changed", root)
	}
	if _, ok := root["user"].(map[string]any)["avatar_url"]; !ok {
		t.Fatal("creation omitted the author avatar", root)
	}
	key := RandID()
	body := map[string]any{"body": "author reply", "reply_to_id": rootID}
	reply := contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "POST", "/api/v1/posts/"+post+"/comments", body, 0, key), 201)
	again := contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "POST", "/api/v1/posts/"+post+"/comments", body, 0, key), 201)
	replyID := reply["id"].(string)
	if again["id"] != replyID || reply["is_author"] != true || reply["parent_id"] != rootID || reply["reply_to"].(map[string]any)["id"] != rootID || reply["reply_count"] != float64(0) || reply["thread_reply_count"] != float64(1) {
		t.Fatal("reply creation/replay shape incorrect", reply, again)
	}
	body["body"] = "changed idempotent request"
	contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "POST", "/api/v1/posts/"+post+"/comments", body, 0, key), 409)
	nested := commentReplyCreate(t, a, reader, post, replyID, "answer to an answer")
	if nested["parent_id"] != rootID || nested["reply_to"].(map[string]any)["id"] != replyID || nested["reply_to"].(map[string]any)["user"].(map[string]any)["id"] != owner.user.ID {
		t.Fatal("reply-to-reply created another nesting level or lost target", nested)
	}
	body["body"] = "author reply"
	refreshed := contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "POST", "/api/v1/posts/"+post+"/comments", body, 0, key), 201)
	if refreshed["id"] != replyID || refreshed["reply_count"] != float64(0) || refreshed["thread_reply_count"] != float64(2) {
		t.Fatal("idempotent replay did not refresh authoritative thread total", refreshed)
	}
	roots := commentReplyGet(t, a, "/api/v1/posts/"+post+"/comments")
	if len(roots["items"].([]any)) != 1 || commentReplyItem(t, roots, 0)["reply_count"] != float64(2) || commentReplyItem(t, roots, 0)["is_self"] != false {
		t.Fatal("root collection included replies or personalized guest state", roots)
	}
	page := commentReplyGet(t, a, "/api/v1/comments/"+rootID+"/replies")
	if len(page["items"].([]any)) != 2 || commentReplyItem(t, page, 0)["id"] != replyID || commentReplyItem(t, page, 1)["id"] != nested["id"] {
		t.Fatal("replies lost chronological order", page)
	}
	commentReplyStatus(t, contentCall(t, a, reader.cookie, reader.csrf, "PUT", "/api/v1/comments/"+replyID+"/like", nil, 0, ""), 204)
	guest := commentReplyItem(t, commentReplyGet(t, a, "/api/v1/comments/"+rootID+"/replies"), 0)
	auth := contentWant(t, contentCall(t, a, reader.cookie, reader.csrf, "GET", "/api/v1/comments/"+rootID+"/replies", nil, 0, ""), 200)
	if guest["liked"] != false || guest["like_count"] != float64(1) || commentReplyItem(t, auth, 0)["liked"] != true {
		t.Fatal("personal like state crossed readers", guest, auth)
	}
	var comments, facts int
	if e := a.Pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM comments),(SELECT count(*) FROM outbox WHERE topic='comment.created.v1')`).Scan(&comments, &facts); e != nil || comments != 3 || facts != 3 {
		t.Fatal("reply replay duplicated comment or outbox fact", comments, facts, e)
	}
}

func TestCommentRepliesModerationDeletionAndCounts(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterInteractions()
	a.RegisterAdmin()
	owner, reader, target := commentReplyIdentity(t, a, "owner"), commentReplyIdentity(t, a, "reader"), commentReplyIdentity(t, a, "reader")
	if _, e := a.Pool.Exec(context.Background(), `UPDATE users SET display_name='Unavailable Target Identity' WHERE id=$1`, target.user.ID); e != nil {
		t.Fatal(e)
	}
	post, _ := interactionPublished(t, a, owner.cookie, owner.csrf)
	root := commentReplyCreate(t, a, reader, post, "", "root")
	rootID := root["id"].(string)
	reply := commentReplyCreate(t, a, target, post, rootID, "unavailable target body")
	replyID := reply["id"].(string)
	answerKey := RandID()
	answerBody := map[string]any{"body": "remaining visible answer", "reply_to_id": replyID}
	answer := contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "POST", "/api/v1/posts/"+post+"/comments", answerBody, 0, answerKey), 201)
	answerID := answer["id"].(string)
	visibility := func(id, state string, status int) {
		t.Helper()
		contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "PUT", "/api/v1/admin/comments/"+id+"/visibility", map[string]any{"state": state}, 0, ""), status)
	}
	count := func(want float64) {
		t.Helper()
		metrics := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+post+"/metrics", nil, 0, ""), 200)
		if metrics["comment_count"] != want {
			t.Fatal("public count includes suppressed descendants", metrics, want)
		}
	}
	unavailableTarget := func() {
		t.Helper()
		w := contentCall(t, a, nil, "", "GET", "/api/v1/comments/"+rootID+"/replies", nil, 0, "")
		page := contentWant(t, w, 200)
		item := commentReplyItem(t, page, 0)
		if len(page["items"].([]any)) != 1 || item["id"] != answerID || item["reply_to"] != nil || item["reply_to_deleted"] != true || strings.Contains(w.Body.String(), "Unavailable Target Identity") || strings.Contains(w.Body.String(), "unavailable target body") {
			t.Fatal("unavailable reply target leaked identity or body", w.Body.String())
		}
		replay := contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "POST", "/api/v1/posts/"+post+"/comments", answerBody, 0, answerKey), 201)
		if replay["id"] != answerID || replay["reply_to"] != nil || replay["reply_to_deleted"] != true {
			t.Fatal("idempotent replay leaked an unavailable target", replay)
		}
	}
	count(3)
	visibility(replyID, "hidden", 200)
	unavailableTarget()
	contentWant(t, contentCall(t, a, reader.cookie, reader.csrf, "POST", "/api/v1/posts/"+post+"/comments", map[string]any{"body": "blocked", "reply_to_id": replyID}, 0, ""), 404)
	count(2)
	visibility(rootID, "hidden", 200)
	count(0)
	roots := commentReplyGet(t, a, "/api/v1/posts/"+post+"/comments")
	if len(roots["items"].([]any)) != 0 {
		t.Fatal("hidden root remained public", roots)
	}
	for _, id := range []string{rootID, answerID} {
		contentWant(t, contentCall(t, a, reader.cookie, reader.csrf, "POST", "/api/v1/posts/"+post+"/comments", map[string]any{"body": "blocked", "reply_to_id": id}, 0, ""), 404)
		contentWant(t, contentCall(t, a, reader.cookie, reader.csrf, "PUT", "/api/v1/comments/"+id+"/like", nil, 0, ""), 404)
	}
	contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/comments/"+rootID+"/replies", nil, 0, ""), 404)
	contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "POST", "/api/v1/posts/"+post+"/comments", answerBody, 0, answerKey), 404)
	visibility(rootID, "visible", 200)
	unavailableTarget()
	count(2)
	visibility(replyID, "visible", 200)
	commentReplyStatus(t, contentCall(t, a, target.cookie, target.csrf, "DELETE", "/api/v1/comments/"+replyID, nil, 0, ""), 204)
	unavailableTarget()
	contentWant(t, contentCall(t, a, reader.cookie, reader.csrf, "POST", "/api/v1/posts/"+post+"/comments", map[string]any{"body": "blocked", "reply_to_id": replyID}, 0, ""), 404)
	visibility(replyID, "visible", 409)
	count(2)
	admin := contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "GET", "/api/v1/admin/comments?post_id="+post, nil, 0, ""), 200)
	if commentReplyItem(t, admin, 0)["is_reply"] != false || commentReplyItem(t, admin, 1)["parent_id"] != rootID || commentReplyItem(t, admin, 1)["reply_to_id"] != rootID || commentReplyItem(t, admin, 1)["is_reply"] != true {
		t.Fatal("moderation list lost thread context", admin)
	}
	commentReplyStatus(t, contentCall(t, a, reader.cookie, reader.csrf, "DELETE", "/api/v1/comments/"+rootID, nil, 0, ""), 204)
	count(0)
	contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/comments/"+rootID+"/replies", nil, 0, ""), 404)
	contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "POST", "/api/v1/posts/"+post+"/comments", answerBody, 0, answerKey), 404)
	contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "PUT", "/api/v1/comments/"+answerID+"/like", nil, 0, ""), 404)
	var retained int
	if e := a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM comments WHERE parent_id=$1`, rootID).Scan(&retained); e != nil || retained != 2 {
		t.Fatal("deleting root destroyed descendant records", retained, e)
	}
}

func TestCommentRepliesValidationGuardsAndPagination(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterInteractions()
	owner, reader := commentReplyIdentity(t, a, "owner"), commentReplyIdentity(t, a, "reader")
	post, _ := interactionPublished(t, a, owner.cookie, owner.csrf)
	otherPost, _ := interactionPublished(t, a, owner.cookie, owner.csrf)
	rootIDs := []string{}
	for range 3 {
		rootIDs = append(rootIDs, commentReplyCreate(t, a, reader, post, "", "root")["id"].(string))
	}
	replyIDs := []string{}
	for range 3 {
		replyIDs = append(replyIDs, commentReplyCreate(t, a, owner, post, rootIDs[0], "reply")["id"].(string))
	}
	rootPage := commentReplyGet(t, a, "/api/v1/posts/"+post+"/comments?limit=1")
	replyPage := commentReplyGet(t, a, "/api/v1/comments/"+rootIDs[0]+"/replies?limit=1")
	rootCursor, replyCursor := rootPage["next_cursor"].(string), replyPage["next_cursor"].(string)
	if rootCursor == "" || replyCursor == "" {
		t.Fatal("missing root/reply pagination", rootPage, replyPage)
	}
	for i := 1; i < 3; i++ {
		rootPage = commentReplyGet(t, a, "/api/v1/posts/"+post+"/comments?limit=1&cursor="+rootPage["next_cursor"].(string))
		replyPage = commentReplyGet(t, a, "/api/v1/comments/"+rootIDs[0]+"/replies?limit=1&cursor="+replyPage["next_cursor"].(string))
		if commentReplyItem(t, rootPage, 0)["id"] != rootIDs[i] || commentReplyItem(t, replyPage, 0)["id"] != replyIDs[i] || rootPage["has_more"] != (i != 2) || replyPage["has_more"] != (i != 2) {
			t.Fatal("pagination skipped, duplicated or mixed thread records", rootPage, replyPage)
		}
	}
	for _, path := range []string{
		"/api/v1/posts/" + post + "/comments?cursor=" + replyCursor,
		"/api/v1/posts/" + otherPost + "/comments?cursor=" + rootCursor,
		"/api/v1/comments/" + rootIDs[0] + "/replies?cursor=" + rootCursor,
		"/api/v1/comments/" + rootIDs[1] + "/replies?cursor=" + replyCursor,
		"/api/v1/comments/" + rootIDs[0] + "/replies?cursor=" + replyCursor + "tampered",
	} {
		contentWant(t, contentCall(t, a, nil, "", "GET", path, nil, 0, ""), 400)
	}
	for _, id := range []string{replyIDs[0], "invalid", "999999999"} {
		contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/comments/"+id+"/replies", nil, 0, ""), 404)
	}
	for _, input := range []struct {
		body   any
		status int
	}{
		{map[string]any{"body": "   ", "reply_to_id": rootIDs[0]}, 400},
		{map[string]any{"body": strings.Repeat("文", 5001), "reply_to_id": rootIDs[0]}, 400},
		{map[string]any{"body": "reply", "reply_to_id": 1}, 400},
		{map[string]any{"body": "reply", "reply_to_id": "invalid"}, 404},
		{map[string]any{"body": "reply", "reply_to_id": "999999999"}, 404},
	} {
		contentWant(t, contentCall(t, a, reader.cookie, reader.csrf, "POST", "/api/v1/posts/"+post+"/comments", input.body, 0, ""), input.status)
	}
	body := map[string]any{"body": "reply", "reply_to_id": rootIDs[0]}
	contentWant(t, contentCall(t, a, reader.cookie, reader.csrf, "POST", "/api/v1/posts/"+otherPost+"/comments", body, 0, ""), 404)
	contentWant(t, contentCall(t, a, nil, "", "POST", "/api/v1/posts/"+post+"/comments", body, 0, ""), 401)
	contentWant(t, contentCall(t, a, reader.cookie, "invalid", "POST", "/api/v1/posts/"+post+"/comments", body, 0, ""), 403)
	if _, e := a.Pool.Exec(context.Background(), `UPDATE site_settings SET comments_enabled=false,version=version+1 WHERE id=1`); e != nil {
		t.Fatal(e)
	}
	contentWant(t, contentCall(t, a, reader.cookie, reader.csrf, "POST", "/api/v1/posts/"+post+"/comments", body, 0, ""), 403)
	commentReplyGet(t, a, "/api/v1/comments/"+rootIDs[0]+"/replies")
	contentWant(t, contentCall(t, a, owner.cookie, owner.csrf, "DELETE", "/api/v1/studio/posts/"+post+"/publication", nil, 2, ""), 200)
	for _, path := range []string{"/api/v1/posts/" + post + "/comments", "/api/v1/comments/" + rootIDs[0] + "/replies", "/api/v1/posts/" + post + "/metrics"} {
		contentWant(t, contentCall(t, a, nil, "", "GET", path, nil, 0, ""), 404)
	}
	contentWant(t, contentCall(t, a, reader.cookie, reader.csrf, "POST", "/api/v1/posts/"+post+"/comments", body, 0, ""), 404)
}

func TestCommentReplySchemaRejectsInvalidOrMutableTrees(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterInteractions()
	owner := commentReplyIdentity(t, a, "owner")
	post, _ := interactionPublished(t, a, owner.cookie, owner.csrf)
	otherPost, _ := interactionPublished(t, a, owner.cookie, owner.csrf)
	root := commentReplyCreate(t, a, owner, post, "", "root")["id"].(string)
	otherRoot := commentReplyCreate(t, a, owner, post, "", "other root")["id"].(string)
	foreignRoot := commentReplyCreate(t, a, owner, otherPost, "", "foreign root")["id"].(string)
	reply := commentReplyCreate(t, a, owner, post, root, "reply")["id"].(string)
	ctx := context.Background()
	for _, links := range [][2]any{{reply, reply}, {root, otherRoot}, {foreignRoot, foreignRoot}, {root, nil}, {nil, root}} {
		if _, e := a.Pool.Exec(ctx, `INSERT INTO comments(post_id,user_id,body,created_at_ms,updated_at_ms,parent_id,reply_to_id) VALUES($1,$2,'invalid',1,1,$3,$4)`, post, owner.user.ID, links[0], links[1]); e == nil {
			t.Fatal("database accepted an invalid reply tree", links)
		}
	}
	for _, statement := range []string{
		`UPDATE comments SET parent_id=NULL,reply_to_id=NULL WHERE id=$1`,
		`UPDATE comments SET parent_id=` + otherRoot + `,reply_to_id=` + otherRoot + ` WHERE id=$1`,
		`UPDATE comments SET post_id=` + otherPost + ` WHERE id=$1`,
	} {
		if _, e := a.Pool.Exec(ctx, statement, reply); e == nil {
			t.Fatal("database allowed reply identity to change", statement)
		}
	}
}
