package app

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHomeProjectsOwnerCASVisibilityRefreshAndStaleSnapshots(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	ctx := context.Background()
	momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/admin/home/projects", nil, 0, ""), 401)
	momentResponse(t, momentCall(t, a, reader, readerCSRF, "GET", "/api/v1/admin/home/projects", nil, 0, ""), 403)
	momentResponse(t, momentCall(t, a, reader, readerCSRF, "POST", "/api/v1/admin/home/projects", map[string]any{}, 0, RandID()), 403)
	input := map[string]any{"source": "github", "repo_url": "openai/openai-go", "title": nil, "description": nil, "url": nil, "language": nil, "position": 10, "visible": true}
	momentResponse(t, momentCall(t, a, owner, "bad", "POST", "/api/v1/admin/home/projects", input, 0, RandID()), 403)
	key := RandID()
	created := momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/projects", input, 0, key), 201)
	id := created["id"].(string)
	replay := momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/projects", input, 0, key), 201)
	if !reflect.DeepEqual(created, replay) {
		t.Fatal("create replay changed")
	}
	if created["stars"] != nil || created["title"] != "openai/openai-go" || created["fetched_at"] != nil {
		t.Fatal("unknown metadata fabricated", created)
	}
	calls := atomic.Int32{}
	failed := atomic.Bool{}
	homeFake(a, func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.URL.Scheme != "https" || r.URL.Host != "api.github.com" || r.URL.Path != "/repos/openai/openai-go" || r.Header.Get("Authorization") != "" {
			t.Error("unsafe upstream request", r.URL)
		}
		if failed.Load() {
			return homeHTTP(503, "unavailable"), nil
		}
		return homeHTTP(200, `{"name":"openai-go","full_name":"openai/openai-go","html_url":"https://github.com/openai/openai-go","private":false,"description":"Official Go API library","stargazers_count":123,"forks_count":12,"language":"Go","updated_at":"2026-10-31T10:00:00Z","owner":{"login":"openai","avatar_url":"https://avatars.githubusercontent.com/u/14957082?v=4"}}`), nil
	})
	path := "/api/v1/admin/home/projects/" + id
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", path+"/refresh", nil, 0, RandID()), 428)
	refreshKey := RandID()
	fresh := momentResponse(t, momentCall(t, a, owner, csrf, "POST", path+"/refresh", nil, 1, refreshKey), 200)
	if fresh["stars"] != float64(123) || fresh["description"] != "Official Go API library" || fresh["last_error"] != "" || fresh["fetched_at"] == nil {
		t.Fatal("refresh failed", fresh)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", path+"/refresh", nil, 1, refreshKey), 200)
	if calls.Load() != 1 {
		t.Fatal("replay repeated upstream")
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", path+"/refresh", nil, 2, RandID()), 429)
	input["title"] = "My own title"
	input["description"] = "My own description"
	input["position"] = 3
	edited := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, input, 2, RandID()), 200)
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, input, 2, RandID()), 409)
	if edited["title"] != "My own title" {
		t.Fatal("override missing")
	}
	if _, e := a.Pool.Exec(ctx, `UPDATE homepage_projects SET attempted_at_ms=0 WHERE id=$1`, id); e != nil {
		t.Fatal(e)
	}
	fresh = momentResponse(t, momentCall(t, a, owner, csrf, "POST", path+"/refresh", nil, 3, RandID()), 200)
	if fresh["title"] != "My own title" || fresh["description"] != "My own description" {
		t.Fatal("refresh replaced local override", fresh)
	}
	failed.Store(true)
	a.Pool.Exec(ctx, `UPDATE homepage_projects SET attempted_at_ms=0 WHERE id=$1`, id)
	stale := momentResponse(t, momentCall(t, a, owner, csrf, "POST", path+"/refresh", nil, 4, RandID()), 200)
	if stale["stars"] != float64(123) || stale["fetched_at"] != fresh["fetched_at"] || !strings.Contains(stale["last_error"].(string), "保留") {
		t.Fatal("failure erased saved data", stale)
	}
	c, steam := homeTestConfig(t, a, owner)
	section := c["sections"].(map[string]any)["projects"].(map[string]any)
	section["enabled"] = true
	section["limit"] = 1
	homeTestPut(t, a, owner, csrf, c, map[string]any{"steam_id": steam["steam_id"], "mode": steam["mode"], "auto_sync_enabled": false}, 1)
	before := calls.Load()
	data, e := a.HomepageData(ctx)
	if e != nil {
		t.Fatal(e)
	}
	projects := data["walls"].(map[string][]map[string]any)["projects"]
	if len(projects) != 1 || projects[0]["title"] != "My own title" || projects[0]["last_error"] != nil || projects[0]["version"] != nil {
		t.Fatal("bad public snapshot", projects)
	}
	if before != calls.Load() {
		t.Fatal("reader contacted GitHub")
	}
	input["visible"] = false
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, input, 5, RandID()), 200)
	data, e = a.HomepageData(ctx)
	if e != nil || len(data["walls"].(map[string][]map[string]any)["projects"]) != 0 {
		t.Fatal("hidden project leaked", e)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", path, nil, 5, RandID()), 409)
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", path, nil, 6, RandID()), 200)
}
func TestHomeProjectsReferenceValidationManualOrderAndLimits(t *testing.T) {
	for _, raw := range []string{"https://github.com.evil.test/openai/go", "http://github.com/openai/go", "https://user:pass@github.com/openai/go", "https://github.com:443/openai/go", "https://github.com/openai/go/issues", "https://github.com/openai/%2e%2e", "https://127.0.0.1/openai/go", "file:///etc/passwd", "../go", "openai/go?x=1", "openai/go#x", "openai/go.git", "openai/go\\x"} {
		if _, e := homeProjectReference(raw); e == nil {
			t.Fatal("unsafe reference", raw)
		}
	}
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	ctx := context.Background()
	in := map[string]any{"source": "manual", "repo_url": "", "title": "First", "description": "Hand-maintained", "url": "https://example.org/project", "language": "", "position": 20, "visible": true}
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/projects", map[string]any{"source": "manual", "title": "", "url": "javascript:alert(1)"}, 0, RandID()), 400)
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/projects", in, 0, RandID()), 201)
	in["title"] = "Second"
	in["position"] = 10
	second := momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/projects", in, 0, RandID()), 201)
	s, e := homeRead(ctx, a.Pool, false)
	if e != nil {
		t.Fatal(e)
	}
	v := s.Config.Sections["projects"]
	v.Enabled = true
	v.Limit = 1
	s.Config.Sections["projects"] = v
	public, e := a.PublicHomeProjects(ctx, s.Config)
	if e != nil || len(public) != 1 || public[0]["title"] != "Second" {
		t.Fatal("project order or limit", e, public)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/home/projects/"+second["id"].(string)+"/refresh", nil, 1, RandID()), 400)
	v.Limit = 0
	s.Config.Sections["projects"] = v
	public, e = a.PublicHomeProjects(ctx, s.Config)
	if e != nil || len(public) != 0 {
		t.Fatal("zero should hide", e, public)
	}
}
