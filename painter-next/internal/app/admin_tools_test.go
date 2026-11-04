package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminToolsOwnerGuardAndReplay(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	actor, cookie, csrf := testIdentity(t, a, "owner")
	_, _, _ = testIdentity(t, a, "owner")
	call := func(method, path string, body any, version int64, key string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.AddCookie(cookie)
		r.Header.Set("Origin", a.Config.PublicBaseURL)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("X-Painter-Admin-Tool", "1")
		r.Header.Set("Idempotency-Key", key)
		if version > 0 {
			r.Header.Set("If-Match", fmt.Sprintf(`"%d"`, version))
		}
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		return w
	}
	session := contentWant(t, call("GET", "/api/v1/session", nil, 0, ""), 200)
	if session["admin_tools_version"] != float64(1) {
		t.Fatal("CLI support cannot be detected")
	}
	created := contentWant(t, call("POST", "/api/v1/posts", map[string]any{"client_draft_id": RandID(), "title": "管理工具", "markdown": "仅站长工具"}, 0, RandID()), 201)
	id := created["id"].(string)
	key := RandID()
	publish := map[string]any{"expected_draft_version": 1}
	contentWant(t, call("POST", "/api/v1/studio/posts/"+id+"/publish", publish, 1, key), 200)
	contentWant(t, call("POST", "/api/v1/studio/posts/"+id+"/publish", publish, 1, key), 200)
	if _, e := a.Pool.Exec(context.Background(), "UPDATE users SET role='author' WHERE id=$1", actor.ID); e != nil {
		t.Fatal(e)
	}
	contentWant(t, call("GET", "/api/v1/studio/posts/"+id, nil, 0, ""), 403)
	contentWant(t, call("POST", "/api/v1/studio/posts/"+id+"/publish", publish, 1, key), 403)
	contentWant(t, call("DELETE", "/api/v1/studio/posts/"+id+"/publication", nil, 2, RandID()), 403)
	// A normal author remains allowed to read their own article: the tool header
	// tightens permission and never grants capabilities to arbitrary callers.
	contentWant(t, contentCall(t, a, cookie, csrf, http.MethodGet, "/api/v1/studio/posts/"+id, nil, 0, ""), 200)
}
