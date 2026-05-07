package domain

import "testing"

func TestQueryValidate(t *testing.T) {
	if err := (Query{}).Validate(); err == nil {
		t.Fatal("expected required namespace error")
	}
	if err := (Query{Namespace: "public"}).Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
