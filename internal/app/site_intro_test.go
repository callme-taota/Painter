package app

import (
	"strings"
	"testing"
)

func TestSiteIntroductionProfilePermissionsValidationAndCompatibility(t *testing.T) {
	a := testApp(t)
	a.RegisterAdmin()
	a.RegisterWeb()
	_, owner, csrf := testIdentity(t, a, "owner")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	profile := contentWant(t, contentCall(t, a, owner, csrf, "GET", "/api/v1/admin/site-profile", nil, 0, ""), 200)
	profile["intro_name"] = "逃塔"
	profile["intro_roles"] = []string{"Golang工程师", "摄影佬"}
	contentWant(t, contentCall(t, a, reader, readerCSRF, "PUT", "/api/v1/admin/site-profile", profile, 1, ""), 403)
	contentWant(t, contentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/site-profile", profile, 1, ""), 200)
	public := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/site", nil, 0, ""), 200)
	if public["intro_name"] != "逃塔" || len(public["intro_roles"].([]any)) != 2 {
		t.Fatal("intro not publicly configured", public)
	}
	for _, invalid := range []any{[]any{17}, []string{strings.Repeat("过", 61)}, make([]string, 13)} {
		profile["intro_roles"] = invalid
		contentWant(t, contentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/site-profile", profile, 2, ""), 400)
	}
	profile["intro_roles"] = []string{"Golang工程师"}
	profile["intro_name"] = strings.Repeat("过", 81)
	contentWant(t, contentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/site-profile", profile, 2, ""), 400)
	delete(profile, "intro_name")
	delete(profile, "intro_roles")
	contentWant(t, contentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/site-profile", profile, 2, ""), 200)
	current := contentWant(t, contentCall(t, a, owner, csrf, "GET", "/api/v1/admin/site-profile", nil, 0, ""), 200)
	if current["intro_name"] != "逃塔" || len(current["intro_roles"].([]any)) != 2 {
		t.Fatal("old profile client erased intro", current)
	}
	current["intro_name"] = "逃塔<script>alert('x')</script>"
	contentWant(t, contentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/site-profile", current, 3, ""), 200)
	page := contentCall(t, a, nil, "", "GET", "/", nil, 0, "")
	if page.Code != 200 || strings.Contains(page.Body.String(), "<script>alert('x')</script>") || !strings.Contains(page.Body.String(), "&lt;script&gt;") {
		t.Fatal("personal introduction not safely rendered", page.Code)
	}
}
