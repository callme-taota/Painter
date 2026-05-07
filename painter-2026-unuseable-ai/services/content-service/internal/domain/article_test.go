package domain

import "testing"

func TestCreateArticleValidate(t *testing.T) {
	if err := (CreateArticleCommand{}).Validate(); err == nil {
		t.Fatal("expected validation error")
	}
	if err := (CreateArticleCommand{Title: "a", Content: "b", CategoryID: "c"}).Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
