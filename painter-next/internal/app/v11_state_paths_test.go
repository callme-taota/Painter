package app

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestPhraseAdminGetReloadsExactPrivateRecordBeyondFirstPage(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	_, reader, _ := testIdentity(t, a, "reader")
	_, author, _ := testIdentity(t, a, "author")
	old := phraseCreateFixture(t, a, owner, csrf, "private older phrase", "private attribution", false)
	id := old["id"].(string)
	if _, err := a.Pool.Exec(context.Background(), "INSERT INTO phrases(text,kind,published,created_at_ms,updated_at_ms) SELECT 'newer phrase '||n,'word',true,$1,$1 FROM generate_series(1,205) n", NowMillis()); err != nil {
		t.Fatal(err)
	}
	first := testRequest(a, "GET", "/api/v1/admin/phrases", "", owner, "")
	if first.Code != 200 || strings.Contains(first.Body.String(), "private older phrase") || jsonMap(t, first)["has_more"] != true {
		t.Fatal("fixture did not place edited record beyond first page", first.Code)
	}
	path := "/api/v1/admin/phrases/" + id
	for _, actor := range []struct {
		cookie *http.Cookie
		status int
	}{{nil, 401}, {reader, 403}, {author, 403}} {
		w := testRequest(a, "GET", path, "", actor.cookie, "")
		if w.Code != actor.status || strings.Contains(w.Body.String(), "private older phrase") || strings.Contains(w.Body.String(), "private attribution") {
			t.Fatal("private phrase detail leaked", w.Code, w.Body.String())
		}
	}
	update := map[string]any{"text": "latest private phrase", "kind": "sentence", "source": "latest private attribution", "published": false}
	if w := phraseCall(t, a, owner, csrf, "PUT", path, update, 1, ""); w.Code != 200 {
		t.Fatal("update fixture", w.Code, w.Body.String())
	}
	if w := phraseCall(t, a, owner, csrf, "PUT", path, update, 1, ""); w.Code != 409 {
		t.Fatal("stale edit should conflict before exact reload", w.Code)
	}
	w := testRequest(a, "GET", path, "", owner, "") // Reads require no CSRF token or If-Match.
	item := jsonMap(t, w)
	if w.Code != 200 || item["id"] != id || item["text"] != update["text"] || item["source"] != update["source"] || item["published"] != false || item["version"] != float64(2) || item["created_at"] == "" || item["updated_at"] == "" || len(item) != 8 || w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("exact private reload did not preserve phrase DTO/cache contract", w.Code, item)
	}
	for _, invalid := range []string{"0", "-1", "bad", "9223372036854775808", "1%20OR%20true", "9223372036854775807"} {
		if got := testRequest(a, "GET", "/api/v1/admin/phrases/"+invalid, "", owner, ""); got.Code != 404 {
			t.Fatal("invalid or missing detail ID must be a safe 404", invalid, got.Code)
		}
	}
	if w := phraseCall(t, a, owner, csrf, "DELETE", path, nil, 2, ""); w.Code != 204 {
		t.Fatal("delete fixture", w.Code)
	}
	if w := testRequest(a, "GET", path, "", owner, ""); w.Code != 404 || strings.Contains(w.Body.String(), "private") {
		t.Fatal("deleted phrase detail remained readable", w.Code, w.Body.String())
	}
	if w := testRequest(a, "GET", "/api/v1/phrases", "", owner, ""); strings.Contains(w.Body.String(), "private") {
		t.Fatal("detail endpoint expanded public visibility")
	}
}

func TestMediaListExcludesDeletedBeforePaginationAndPreservesState(t *testing.T) {
	a := testApp(t)
	a.RegisterMedia()
	p, owner, _ := testIdentity(t, a, "owner")
	other, otherCookie, _ := testIdentity(t, a, "author")
	ctx := context.Background()
	now := NowMillis()
	expected := map[string]string{}
	usedID, foreignID := "", ""
	for i, state := range []string{"ready", "deleted", "ready", "deleted", "staging", "failed", "deleting", "deleted", "ready"} {
		id, uid := RandID(), p.ID
		if i == 8 {
			uid, foreignID = other.ID, id
		} else if state != "deleted" {
			expected[id] = state
		}
		if i == 0 {
			usedID = id
		}
		if _, err := a.Pool.Exec(ctx, `INSERT INTO media_assets(id,owner_user_id,storage_key,original_name,mime_type,byte_size,sha256,state,created_at_ms,updated_at_ms,ready_at_ms) VALUES($1,$2,$3,'state.png','image/png',10,$4,$5,$6,$6,$6)`, id, uid, RandID(), strings.Repeat("a", 64), state, now+int64(i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.Pool.Exec(ctx, "UPDATE users SET avatar_media_id=$1 WHERE id=$2", usedID, p.ID); err != nil {
		t.Fatal(err)
	}
	if w := testRequest(a, "GET", "/api/v1/me/media", "", nil, ""); w.Code != 401 {
		t.Fatal("media list became anonymous", w.Code)
	}
	seen, cursor := map[string]bool{}, ""
	for page := 0; ; page++ {
		if page > len(expected) {
			t.Fatal("media pagination failed to terminate")
		}
		w := testRequest(a, "GET", "/api/v1/me/media?limit=1&cursor="+url.QueryEscape(cursor), "", owner, "")
		data := jsonMap(t, w)
		items := data["items"].([]any)
		if w.Code != 200 || len(items) != 1 || w.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("deleted rows consumed a page or changed privacy", w.Code, w.Body.String())
		}
		item := items[0].(map[string]any)
		id := item["media_id"].(string)
		if seen[id] || expected[id] != item["state"] || item["state"] == "deleted" || id == foreignID || item["in_use"] != (id == usedID) {
			t.Fatal("wrong media owner/state/reference or duplicate cursor result", item)
		}
		seen[id] = true
		if data["has_more"] != true {
			break
		}
		cursor = data["next_cursor"].(string)
		if cursor == "" {
			t.Fatal("missing media continuation cursor")
		}
	}
	if len(seen) != len(expected) {
		t.Fatal("non-deleted media disappeared", len(seen), len(expected))
	}
	w := testRequest(a, "GET", "/api/v1/me/media", "", otherCookie, "")
	items := jsonMap(t, w)["items"].([]any)
	if w.Code != 200 || len(items) != 1 || items[0].(map[string]any)["media_id"] != foreignID {
		t.Fatal("media list lost personal scope", w.Code, w.Body.String())
	}
}
