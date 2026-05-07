package application

import (
	"testing"
	"time"

	"painter-2026/identity-service/internal/domain"
)

type fakeIssuer struct{}

func (f *fakeIssuer) Issue(subject string, _ time.Time) string {
	return "token-" + subject
}

func TestAuthServiceLogin(t *testing.T) {
	svc := NewAuthService(&fakeIssuer{})
	result, err := svc.Login(domain.LoginCommand{Username: "admin", Password: "123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AccessToken != "token-admin" {
		t.Fatalf("unexpected token: %s", result.AccessToken)
	}
}
