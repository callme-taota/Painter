package application

import (
	"time"

	"painter-2026/analytics-service/internal/domain"
)

type OverviewService struct{}

func NewOverviewService() *OverviewService {
	return &OverviewService{}
}

func (s *OverviewService) GetOverview() domain.Overview {
	return domain.Overview{
		QPS:              12,
		P95MS:            128,
		CacheHitRatio:    0.93,
		GeneratedAt:      time.Now().Format(time.RFC3339),
		SlowSQLThreshold: "200ms",
	}
}
