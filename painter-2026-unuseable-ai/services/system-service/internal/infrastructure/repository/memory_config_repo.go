package repository

import (
	"time"

	"painter-2026/system-service/internal/domain"
)

type ConfigRepository interface {
	List(namespace string) []domain.ConfigItem
}

type MemoryConfigRepo struct{}

func NewMemoryConfigRepo() *MemoryConfigRepo {
	return &MemoryConfigRepo{}
}

func (r *MemoryConfigRepo) List(_ string) []domain.ConfigItem {
	now := time.Now()
	return []domain.ConfigItem{
		{Key: "feature.comment.enabled", Value: "true", ValueType: "boolean", UpdatedAt: now},
		{Key: "theme.web.primaryColor", Value: "#4f46e5", ValueType: "string", UpdatedAt: now},
	}
}
