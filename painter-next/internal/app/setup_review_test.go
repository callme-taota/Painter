package app

import (
	"context"
	"strings"
	"testing"
)

// Reader-only legacy identity imports can occur after the installation migration
// has run. Presence of any real credential must close setup independently of the
// owner role and independently of the pre-migration sentinel initialization.
func TestSetupReviewExistingCredentialsAndDisabledOwnerBlockInstallation(t *testing.T) {
	for _, kind := range []string{"reader-local", "reader-github", "disabled-owner"} {
		t.Run(kind, func(t *testing.T) {
			a := testApp(t)
			a.Config.SetupToken = strings.Repeat("review-fixture-only-", 3)
			before := testRequest(a, "GET", "/api/v1/setup/status", "", nil, "")
			if before.Code != 200 || jsonMap(t, before)["available"] != true {
				t.Fatal("fresh setup unavailable", before.Code, before.Body.String())
			}
			role := "reader"
			if kind == "disabled-owner" {
				role = "owner"
			}
			var id string
			now := NowMillis()
			var disabled *int64
			if kind == "disabled-owner" {
				disabled = &now
			}
			if err := a.Pool.QueryRow(context.Background(), "INSERT INTO users(username,display_name,role,disabled_at_ms,created_at_ms,updated_at_ms) VALUES($1,'Imported identity',$2,$3,$4,$4) RETURNING id::text", kind, role, disabled, now).Scan(&id); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "reader-local":
				_, err = a.Pool.Exec(context.Background(), "INSERT INTO local_accounts(user_id,password_hash,password_changed_at_ms,created_at_ms) VALUES($1,$2,$3,$3)", id, dummyPasswordHash, now)
			case "reader-github":
				_, err = a.Pool.Exec(context.Background(), "INSERT INTO oauth_accounts(provider,issuer,subject,user_id,provider_login,created_at_ms,updated_at_ms) VALUES('github',$1,'314159',$2,'imported-reader',$3,$3)", githubIssuer, id, now)
			}
			if err != nil {
				t.Fatal(err)
			}
			var sentinelStillEmpty bool
			if err := a.Pool.QueryRow(context.Background(), "SELECT completed_at_ms IS NULL FROM installation_state WHERE id=1").Scan(&sentinelStillEmpty); err != nil || !sentinelStillEmpty {
				t.Fatal("fixture unexpectedly used bootstrap sentinel", err)
			}
			after := testRequest(a, "GET", "/api/v1/setup/status", "", nil, "")
			if after.Code != 200 {
				t.Fatal(after.Code, after.Body.String())
			}
			status := jsonMap(t, after)
			if status["installed"] != true || status["available"] != false || status["csrf_token"] != "" || strings.Contains(after.Body.String(), a.Config.SetupToken) {
				t.Fatal("existing identity exposed installation", status)
			}
			if got := testRequest(a, "POST", "/api/v1/setup/complete", "{}", nil, ""); got.Code != 409 || !strings.Contains(got.Body.String(), "SETUP_COMPLETED") {
				t.Fatal("existing identity allowed installation command", got.Code, got.Body.String())
			}
			var count int
			if err := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&count); err != nil || count != 1 {
				t.Fatal("rejected installation changed users", count, err)
			}
		})
	}
}
