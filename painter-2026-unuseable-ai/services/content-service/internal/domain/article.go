package domain

import (
	"errors"
	"time"
)

var ErrInvalidArticleInput = errors.New("invalid article input")

type Article struct {
	ArticleID    string    `json:"articleId"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	Content      string    `json:"content,omitempty"`
	AuthorID     string    `json:"authorId"`
	CategoryID   string    `json:"categoryId"`
	ReadCount    int       `json:"readCount"`
	LikeCount    int       `json:"likeCount"`
	CommentCount int       `json:"commentCount"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CreateArticleCommand struct {
	Title      string
	Summary    string
	Content    string
	CategoryID string
	TagIDs     []string
}

func (c CreateArticleCommand) Validate() error {
	if c.Title == "" || c.Content == "" || c.CategoryID == "" {
		return ErrInvalidArticleInput
	}
	return nil
}
