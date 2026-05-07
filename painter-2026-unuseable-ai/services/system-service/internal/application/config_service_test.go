package application

import (
	"testing"

	"painter-2026/system-service/internal/domain"
	"painter-2026/system-service/internal/infrastructure/repository"
)

func TestConfigServiceList(t *testing.T) {
	svc := NewConfigService(repository.NewMemoryConfigRepo())
	items, err := svc.List(domain.Query{Namespace: "public"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected config items")
	}
}
