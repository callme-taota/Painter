package app

import (
	"context"
	"net/http"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

func TestV33GuestInteractionTemplates(t *testing.T) {
	templates := v31NavigationTemplates(t)
	controls := floatingControls{"home": {"theme"}, "article": {"bottom", "like"}, "other": {"account"}}
	for _, authenticated := range []bool{false, true} {
		page := v31NavigationPage("article", controls, false, false)
		page.Authenticated = authenticated
		doc := v31NavigationDocument(t, templates, page)
		if (len(friendWebClass(doc, "floating-like")) == 1) != authenticated {
			t.Fatal("SSR article like did not follow session", authenticated)
		}
		if len(friendWebClass(doc, "article-discussion")) != 1 || !strings.Contains(friendWebText(doc), "启用 JavaScript 即可阅读评论。") {
			t.Fatal("public article discussion stopped being readable")
		}
		if strings.Contains(friendWebText(doc), "即可点赞、收藏") || strings.Contains(friendWebText(doc), "登录后参与讨论") {
			t.Fatal("guest interaction CTA remained in SSR")
		}
		page.Kind = "author"
		page.ViewerID = "reader"
		page.User = map[string]any{"id": "author", "display_name": "Author", "page": map[string]any{}}
		doc = v31NavigationDocument(t, templates, page)
		follow := friendWebNodes(doc, func(n *nethtml.Node) bool { return friendWebHasAttr(n, "data-follow-island") })
		if (len(follow) == 1) != authenticated {
			t.Fatal("SSR author follow did not follow session", authenticated)
		}
		if strings.Contains(friendWebText(doc), "关注作者") {
			t.Fatal("SSR follow placeholder became a login prompt")
		}
		page.Authenticated, page.ViewerID = true, "author"
		doc = v31NavigationDocument(t, templates, page)
		if len(friendWebNodes(doc, func(n *nethtml.Node) bool { return friendWebHasAttr(n, "data-follow-island") })) != 0 {
			t.Fatal("own author page offered self-follow")
		}
	}
}

func TestV33PublicReadAndAuthenticatedWritesRemainSeparated(t *testing.T) {
	a := momentWebApp(t)
	a.RegisterInteractions()
	writer, owner, csrf := testIdentity(t, a, "owner")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	post, _ := interactionPublished(t, a, owner, csrf)
	comment := contentWant(t, contentCall(t, a, reader, readerCSRF, "POST", "/api/v1/posts/"+post+"/comments", map[string]any{"body": "公开评论仍然可读"}, 0, RandID()), 201)
	commentID := comment["id"].(string)
	public := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/posts/"+post+"/comments", nil, 0, ""), 200)
	if len(public["items"].([]any)) != 1 {
		t.Fatal("guest lost public comments")
	}
	for _, request := range []struct {
		method, path string
		body         any
	}{
		{"PUT", "/api/v1/posts/" + post + "/like", nil}, {"DELETE", "/api/v1/posts/" + post + "/like", nil},
		{"PUT", "/api/v1/posts/" + post + "/bookmark", nil}, {"DELETE", "/api/v1/posts/" + post + "/bookmark", nil},
		{"POST", "/api/v1/posts/" + post + "/comments", map[string]any{"body": "unauthorized comment"}},
		{"PUT", "/api/v1/comments/" + commentID + "/like", nil}, {"DELETE", "/api/v1/comments/" + commentID + "/like", nil},
		{"DELETE", "/api/v1/comments/" + commentID, nil},
		{"PUT", "/api/v1/users/" + writer.ID + "/follow", nil}, {"DELETE", "/api/v1/users/" + writer.ID + "/follow", nil},
		{"POST", "/api/v1/moments", map[string]any{"text": "unauthorized moment"}},
	} {
		if response := contentCall(t, a, nil, "", request.method, request.path, request.body, 0, RandID()); response.Code != 401 {
			t.Fatal("guest write crossed existing authentication guard", request.method, request.path, response.Code, response.Body.String())
		}
	}
	for _, table := range []string{"post_likes", "comment_likes", "bookmarks", "follows", "moments"} {
		var count int
		if err := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatal("guest mutation persisted", table, count, err)
		}
	}
	var count int
	if err := a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM comments WHERE id=$1 AND state='visible'`, commentID).Scan(&count); err != nil || count != 1 {
		t.Fatal("guest modified/deleted visible comment", err)
	}
	for _, actor := range []struct {
		cookie        *http.Cookie
		authenticated bool
	}{{nil, false}, {reader, true}, {owner, true}, {&http.Cookie{Name: sessionCookie, Value: strings.Repeat("x", 48)}, false}} {
		response := testRequest(a, "GET", "/posts/"+post, "", actor.cookie, "")
		if response.Code != 200 {
			t.Fatal("public read failed", response.Code)
		}
		doc := friendWebDocument(t, response.Body.String())
		if (len(friendWebClass(doc, "floating-like")) == 1) != actor.authenticated {
			t.Fatal("real SSR like visibility ignored validated session", actor.authenticated)
		}
		response = testRequest(a, "GET", "/users/"+writer.ID, "", actor.cookie, "")
		if response.Code != 200 {
			t.Fatal("author page read failed", response.Code)
		}
		doc = friendWebDocument(t, response.Body.String())
		wantFollow := actor.authenticated && actor.cookie != owner
		if (len(friendWebClass(doc, "author-follow")) == 1) != wantFollow {
			t.Fatal("real SSR follow visibility ignored session/self", actor.authenticated, wantFollow)
		}
	}
}
