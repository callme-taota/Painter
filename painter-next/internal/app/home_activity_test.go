package app

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestHomeV24LegacyReadAndWritePreserveAdditions(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	ctx := context.Background()
	var before []byte
	if e := a.Pool.QueryRow(ctx, `SELECT config FROM homepage_settings`).Scan(&before); e != nil {
		t.Fatal(e)
	}
	var stored map[string]any
	json.Unmarshal(before, &stored)
	if len(stored["order"].([]any)) != 6 {
		t.Fatal("migration rewrote old order")
	}
	current := homeTestGet(t, a, owner)
	c := current["config"].(map[string]any)
	if len(c["order"].([]any)) != 8 {
		t.Fatal("additions absent")
	}
	for _, key := range []string{"activity", "projects"} {
		if c["sections"].(map[string]any)[key].(map[string]any)["enabled"] != false {
			t.Fatal("new section enabled automatically")
		}
	}
	var after []byte
	a.Pool.QueryRow(ctx, `SELECT config FROM homepage_settings`).Scan(&after)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("reading rewrote owner config")
	}
	c["activity"].(map[string]any)["source"] = "moments"
	c["sections"].(map[string]any)["projects"].(map[string]any)["enabled"] = true
	steam := map[string]any{"steam_id": "", "mode": "owned", "auto_sync_enabled": false}
	homeTestPut(t, a, owner, csrf, c, steam, 1)
	legacy := stored
	result := homeTestPut(t, a, owner, csrf, legacy, steam, 2)["config"].(map[string]any)
	if result["activity"].(map[string]any)["source"] != "moments" || result["sections"].(map[string]any)["projects"].(map[string]any)["enabled"] != true {
		t.Fatal("old client omission cleared new fields", result)
	}
}
func TestHomeActivityRealPublicSourcesLocalBoundariesAndWithdrawal(t *testing.T) {
	a := homeTestApp(t)
	owner, cookie, csrf := testIdentity(t, a, "owner")
	ctx := context.Background()
	s, e := homeRead(ctx, a.Pool, false)
	if e != nil {
		t.Fatal(e)
	}
	c := s.Config
	v := c.Sections["activity"]
	v.Enabled = true
	c.Sections["activity"] = v
	c.Activity = HomeActivityOptions{Source: "posts_moments", Timezone: "Asia/Shanghai", Period: "year", Year: 2024}
	insert := func(stamp string, public bool) string {
		t.Helper()
		ms, e := time.Parse(time.RFC3339, stamp)
		if e != nil {
			t.Fatal(e)
		}
		var id string
		var text *string
		if public {
			x := "公开瞬间"
			text = &x
		}
		e = a.Pool.QueryRow(ctx, `INSERT INTO moments(author_user_id,draft_text,public_text,first_published_at_ms,published_at_ms,created_at_ms,updated_at_ms) VALUES($1,'草稿',$2,$3,$3,$3,$3) RETURNING id::text`, owner.ID, text, ms.UnixMilli()).Scan(&id)
		if e != nil {
			t.Fatal(e)
		}
		return id
	}
	first := insert("2024-01-01T15:59:59Z", true)
	insert("2024-01-01T16:00:00Z", true)
	insert("2024-02-29T04:00:00Z", true)
	insert("2024-02-28T04:00:00Z", false)
	insert("2023-12-31T15:59:59Z", true)
	// A true published article enters the same first-publication series.
	a.RegisterContent()
	draft := map[string]any{"client_draft_id": RandID(), "title": "Activity article", "summary": "", "markdown": "A real publication.", "tag_ids": []string{}, "media_ids": []string{}}
	post := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/posts", draft, 0, RandID()), 201)
	pid := post["id"].(string)
	pub := contentWant(t, contentCall(t, a, cookie, csrf, "POST", "/api/v1/studio/posts/"+pid+"/publish", map[string]any{"expected_draft_version": 1}, 1, RandID()), 200)
	ms := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli()
	if _, e = a.Pool.Exec(ctx, `UPDATE posts SET first_published_at_ms=$2,version=version+1 WHERE id=$1`, pid, ms); e != nil {
		t.Fatal(e)
	}
	now := time.Date(2024, 12, 31, 12, 0, 0, 0, time.UTC)
	activity, e := homeActivityAt(ctx, a.Pool, c, now)
	if e != nil {
		t.Fatal(e)
	}
	if len(activity.Days) != 366 || activity.Total != 4 || activity.ActiveDays != 3 || activity.Start != "2024-01-01" || activity.End != "2024-12-31" {
		t.Fatalf("wrong real counts: %+v", activity)
	}
	find := func(out HomeActivity, date string) HomeActivityDay {
		t.Helper()
		for _, d := range out.Days {
			if d.Date == date {
				return d
			}
		}
		t.Fatal("missing date", date)
		return HomeActivityDay{}
	}
	if find(activity, "2024-01-01").Count != 1 || find(activity, "2024-01-02").Count != 2 || find(activity, "2024-02-29").Count != 1 || find(activity, "2024-02-28").Level != 0 {
		t.Fatal("incorrect local date boundary or fake zero")
	}
	for _, week := range activity.Weeks {
		if len(week) != 7 {
			t.Fatal("broken grid")
		}
		for _, d := range week {
			if !d.InRange && d.Count != 0 {
				t.Fatal("padding counted")
			}
		}
	}
	c.Activity.Source = "posts"
	only, e := homeActivityAt(ctx, a.Pool, c, now)
	if e != nil || only.Total != 1 {
		t.Fatal("post filter", e, only.Total)
	}
	contentWant(t, contentCall(t, a, cookie, csrf, "DELETE", "/api/v1/studio/posts/"+pid+"/publication", nil, contentNumber(pub, "version")+1, RandID()), 200)
	c.Activity.Source = "posts_moments"
	if _, e = a.Pool.Exec(ctx, `UPDATE moments SET public_text=NULL,version=version+1 WHERE id=$1`, first); e != nil {
		t.Fatal(e)
	}
	after, e := homeActivityAt(ctx, a.Pool, c, now)
	if e != nil || after.Total != 2 || find(after, "2024-01-01").Count != 0 {
		t.Fatal("withdrawal still counted", e, after.Total)
	}
	if _, e = a.Pool.Exec(ctx, `UPDATE moments SET public_text=NULL,deleted_at_ms=updated_at_ms,version=version+1 WHERE first_published_at_ms=$1`, time.Date(2024, 2, 29, 4, 0, 0, 0, time.UTC).UnixMilli()); e != nil {
		t.Fatal(e)
	}
	after, e = homeActivityAt(ctx, a.Pool, c, now)
	if e != nil || after.Total != 1 {
		t.Fatal("deletion still counted", e, after.Total)
	}
	c.Activity.Period = "rolling"
	c.Activity.Timezone = "America/New_York"
	rolling, e := homeActivityAt(ctx, a.Pool, c, time.Date(2024, 11, 4, 12, 0, 0, 0, time.UTC))
	if e != nil || len(rolling.Days) != 365 {
		t.Fatal("DST rolling count", e, len(rolling.Days))
	}
	c.Activity.Period = "year"
	c.Activity.Year = 2025
	future, e := homeActivityAt(ctx, a.Pool, c, now)
	if e != nil || future.Total != 0 || !future.Days[0].Future {
		t.Fatal("future activity invented", e)
	}
}
func TestHomeActivityAuthorizationAndValidation(t *testing.T) {
	a := homeTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	_, reader, rCSRF := testIdentity(t, a, "reader")
	momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/admin/home/activity", nil, 0, ""), 401)
	momentResponse(t, momentCall(t, a, reader, rCSRF, "GET", "/api/v1/admin/home/activity", nil, 0, ""), 403)
	for _, query := range []string{"timezone=Local", "timezone=Not/A_Zone", "source=github", "year=1900", "period=forever"} {
		momentResponse(t, momentCall(t, a, owner, csrf, "GET", "/api/v1/admin/home/activity?"+query, nil, 0, ""), 400)
	}
	result := momentResponse(t, momentCall(t, a, owner, csrf, "GET", "/api/v1/admin/home/activity?period=year&year=2024", nil, 0, ""), 200)
	if result["total"] != float64(0) || len(result["days"].([]any)) != 366 {
		t.Fatal("honest empty calendar", result)
	}
}
