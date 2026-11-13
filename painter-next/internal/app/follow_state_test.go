package app

import (
	"context"
	"strings"
	"testing"
)

func TestFollowStatePrivateRelationshipAndTargetVisibility(t *testing.T) {
	a := testApp(t)
	a.RegisterInteractions()
	reader, cookie, csrf := testIdentity(t, a, "reader")
	_, otherCookie, otherCSRF := testIdentity(t, a, "reader")
	author, _, _ := testIdentity(t, a, "author")
	path := "/api/v1/users/" + author.ID + "/follow-state"
	contentWant(t, contentCall(t, a, nil, "", "GET", path, nil, 0, ""), 401)
	for _, id := range []string{"0", "invalid", "9223372036854775807"} {
		contentWant(t, contentCall(t, a, cookie, "", "GET", "/api/v1/users/"+id+"/follow-state", nil, 0, ""), 404)
	}
	response := contentCall(t, a, cookie, "", "GET", path, nil, 0, "")
	initial := contentWant(t, response, 200)
	if initial["followed"] != false || initial["follows_enabled"] != true || len(initial) != 2 {
		t.Fatal("follow button response exposed an unexpected state or private collection", initial)
	}
	if !strings.Contains(response.Header().Get("Cache-Control"), "private") || !strings.Contains(response.Header().Get("Cache-Control"), "no-store") {
		t.Fatal("reader relationship could be shared by a cache", response.Header())
	}
	if response := contentCall(t, a, cookie, csrf, "PUT", "/api/v1/users/"+author.ID+"/follow", nil, 0, ""); response.Code != 204 {
		t.Fatal("could not set up follow relationship", response.Code, response.Body.String())
	}
	// A cookie and route target determine this result. Caller-supplied user IDs
	// cannot ask for someone else's relationship or reveal their follow list.
	followed := contentWant(t, contentCall(t, a, cookie, "", "GET", path+"?follower_id="+author.ID, nil, 0, ""), 200)
	other := contentWant(t, contentCall(t, a, otherCookie, otherCSRF, "GET", path+"?follower_id="+reader.ID, nil, 0, ""), 200)
	if followed["followed"] != true || other["followed"] != false {
		t.Fatal("relationship state crossed reader identities", followed, other)
	}
	if _, err := a.Pool.Exec(context.Background(), `UPDATE users SET disabled_at_ms=$2 WHERE id=$1`, author.ID, NowMillis()); err != nil {
		t.Fatal(err)
	}
	contentWant(t, contentCall(t, a, cookie, "", "GET", path, nil, 0, ""), 404)
	var retained int
	if err := a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM follows WHERE follower_user_id=$1 AND followed_user_id=$2`, reader.ID, author.ID).Scan(&retained); err != nil || retained != 1 {
		t.Fatal("a read or suspension destroyed the retained follow relationship", retained, err)
	}
	if _, err := a.Pool.Exec(context.Background(), `UPDATE users SET disabled_at_ms=$2 WHERE id=$1`, reader.ID, NowMillis()); err != nil {
		t.Fatal(err)
	}
	contentWant(t, contentCall(t, a, cookie, "", "GET", path, nil, 0, ""), 401)
}

func TestFollowStateReportsFeatureSwitchAndAllowsUnfollow(t *testing.T) {
	a := testApp(t)
	a.RegisterInteractions()
	_, cookie, csrf := testIdentity(t, a, "reader")
	target, _, _ := testIdentity(t, a, "author")
	followPath := "/api/v1/users/" + target.ID + "/follow"
	statePath := "/api/v1/users/" + target.ID + "/follow-state"
	if response := contentCall(t, a, cookie, csrf, "PUT", followPath, nil, 0, ""); response.Code != 204 {
		t.Fatal(response.Code, response.Body.String())
	}
	if _, err := a.Pool.Exec(context.Background(), `UPDATE site_settings SET follows_enabled=false,version=version+1,updated_at_ms=$1 WHERE id=1`, NowMillis()); err != nil {
		t.Fatal(err)
	}
	state := contentWant(t, contentCall(t, a, cookie, "", "GET", statePath, nil, 0, ""), 200)
	if state["followed"] != true || state["follows_enabled"] != false {
		t.Fatal("feature switch obscured the existing relationship", state)
	}
	blocked := contentWant(t, contentCall(t, a, cookie, csrf, "PUT", followPath, nil, 0, ""), 403)
	if blocked["error"].(map[string]any)["code"] != "FEATURE_DISABLED" {
		t.Fatal("new follow ignored the feature policy", blocked)
	}
	if response := contentCall(t, a, cookie, csrf, "DELETE", followPath, nil, 0, ""); response.Code != 204 {
		t.Fatal("disabled feature prevented removing a relationship", response.Code, response.Body.String())
	}
	state = contentWant(t, contentCall(t, a, cookie, "", "GET", statePath, nil, 0, ""), 200)
	if state["followed"] != false || state["follows_enabled"] != false {
		t.Fatal("unfollow not reflected by compact state endpoint", state)
	}
}
