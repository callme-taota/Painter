package domain

import "testing"

func TestLoginCommandValidate(t *testing.T) {
	if err := (LoginCommand{Username: "", Password: "x"}).Validate(); err == nil {
		t.Fatal("expected missing credential error")
	}
	if err := (LoginCommand{Username: "u", Password: "p"}).Validate(); err != nil {
		t.Fatalf("expected valid command, got %v", err)
	}
}
