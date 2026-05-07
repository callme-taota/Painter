package application

import (
	"strconv"

	"painter-2026/content-service/internal/domain"
	"painter-2026/content-service/internal/infrastructure/repository"
)

type ArticleService struct {
	repo repository.ArticleRepository
}

func NewArticleService(repo repository.ArticleRepository) *ArticleService {
	return &ArticleService{repo: repo}
}

func (s *ArticleService) List(cursor string, limit int) ([]domain.Article, string) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	start := 0
	if cursor != "" {
		if parsed, err := strconv.Atoi(cursor); err == nil && parsed >= 0 && parsed < s.repo.Count() {
			start = parsed
		}
	}
	items, total := s.repo.List(start, limit)
	next := ""
	if start+limit < total {
		next = strconv.Itoa(start + limit)
	}
	return items, next
}

func (s *ArticleService) Create(cmd domain.CreateArticleCommand) (string, error) {
	if err := cmd.Validate(); err != nil {
		return "", err
	}
	return s.repo.Create(cmd), nil
}
