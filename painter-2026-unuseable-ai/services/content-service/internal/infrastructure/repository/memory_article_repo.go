package repository

import (
	"fmt"
	"sync"
	"time"

	"painter-2026/content-service/internal/domain"
)

type ArticleRepository interface {
	List(start, limit int) ([]domain.Article, int)
	Create(cmd domain.CreateArticleCommand) string
	Count() int
}

type MemoryArticleRepo struct {
	mu       sync.RWMutex
	articles []domain.Article
}

func NewMemoryArticleRepo() *MemoryArticleRepo {
	return &MemoryArticleRepo{
		articles: []domain.Article{
			{
				ArticleID:  "a-1",
				Title:      "Painter-2026 Preview",
				Summary:    "first migrated article",
				AuthorID:   "u-admin",
				CategoryID: "c-general",
				UpdatedAt:  time.Now(),
			},
		},
	}
}

func (r *MemoryArticleRepo) List(start, limit int) ([]domain.Article, int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	end := start + limit
	if end > len(r.articles) {
		end = len(r.articles)
	}
	return r.articles[start:end], len(r.articles)
}

func (r *MemoryArticleRepo) Create(cmd domain.CreateArticleCommand) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := fmt.Sprintf("a-%d", len(r.articles)+1)
	r.articles = append(r.articles, domain.Article{
		ArticleID:  id,
		Title:      cmd.Title,
		Summary:    cmd.Summary,
		Content:    cmd.Content,
		AuthorID:   "u-admin",
		CategoryID: cmd.CategoryID,
		UpdatedAt:  time.Now(),
	})
	return id
}

func (r *MemoryArticleRepo) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.articles)
}
