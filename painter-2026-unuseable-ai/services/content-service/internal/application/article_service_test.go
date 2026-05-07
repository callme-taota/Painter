package application

import (
	"testing"

	"painter-2026/content-service/internal/domain"
	"painter-2026/content-service/internal/infrastructure/repository"
)

func TestArticleServiceCreateAndList(t *testing.T) {
	svc := NewArticleService(repository.NewMemoryArticleRepo())
	if _, err := svc.Create(domain.CreateArticleCommand{Title: "t", Content: "c", CategoryID: "cat"}); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	items, _ := svc.List("", 20)
	if len(items) == 0 {
		t.Fatal("expected at least one article")
	}
}
