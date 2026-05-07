package application

import (
	"painter-2026/system-service/internal/domain"
	"painter-2026/system-service/internal/infrastructure/repository"
)

type ConfigService struct {
	repo repository.ConfigRepository
}

func NewConfigService(repo repository.ConfigRepository) *ConfigService {
	return &ConfigService{repo: repo}
}

func (s *ConfigService) List(query domain.Query) ([]domain.ConfigItem, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	return s.repo.List(query.Namespace), nil
}
