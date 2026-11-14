package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestAdminSettingsSecretsCASRolesAndTaxonomy(t *testing.T) {
	a := testApp(t)
	a.RegisterAdmin()
	owner, c, csrf := testIdentity(t, a, "owner")
	reader, rc, rt := testIdentity(t, a, "reader")
	w := contentCall(t, a, rc, rt, "GET", "/api/v1/admin/settings", nil, 0, "")
	if w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	settings := contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/admin/settings", nil, 0, ""), 200)
	settings["smtp_password"] = "private-test-value"
	settings["smtp_host"] = "mail.example.test"
	saved := contentWant(t, contentCall(t, a, c, csrf, "PUT", "/api/v1/admin/settings", settings, 1, ""), 200)
	if saved["smtp_password"] != nil || saved["smtp_password_configured"] != true {
		t.Fatal("secret returned", saved)
	}
	var encrypted []byte
	if e := a.Pool.QueryRow(context.Background(), `SELECT smtp_password_ciphertext FROM site_settings`).Scan(&encrypted); e != nil || bytes.Contains(encrypted, []byte("private-test-value")) {
		t.Fatal("secret not encrypted", e)
	}
	w = contentCall(t, a, c, csrf, "PUT", "/api/v1/admin/settings", settings, 1, "")
	if w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	public := contentCall(t, a, nil, "", "GET", "/api/v1/site", nil, 0, "")
	if public.Code != 200 || strings.Contains(public.Body.String(), "smtp") {
		t.Fatal(public.Code, public.Body.String())
	}
	w = contentCall(t, a, c, csrf, "PATCH", "/api/v1/admin/users/"+owner.ID, map[string]any{"role": "reader"}, 0, "")
	if w.Code != 409 {
		t.Fatal("last owner demoted", w.Code, w.Body.String())
	}
	contentWant(t, contentCall(t, a, c, csrf, "PATCH", "/api/v1/admin/users/"+reader.ID, map[string]any{"role": "author", "reason": "允许创作"}, 0, ""), 200)
	w = contentCall(t, a, rc, rt, "GET", "/api/v1/me", nil, 0, "")
	if w.Code != 401 {
		t.Fatal("role change kept session", w.Code, w.Body.String())
	}
	created := contentWant(t, contentCall(t, a, c, csrf, "POST", "/api/v1/admin/tags", map[string]any{"name": "Go", "description": "语言"}, 0, ""), 201)
	id := created["id"].(string)
	contentWant(t, contentCall(t, a, c, csrf, "PATCH", "/api/v1/admin/tags/"+id, map[string]any{"archived": true}, 0, ""), 200)
	tags := contentWant(t, contentCall(t, a, nil, "", "GET", "/api/v1/tags", nil, 0, ""), 200)
	if len(tags["items"].([]any)) != 0 {
		t.Fatal("archived tag exposed")
	}
	contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/admin/storage", nil, 0, ""), 200)
	contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/admin/jobs", nil, 0, ""), 200)
	contentWant(t, contentCall(t, a, c, csrf, "GET", "/api/v1/admin/audit", nil, 0, ""), 200)
}
