package search

import (
	"strings"
	"unicode/utf8"

	"painter-2026/content-service/internal/infrastructure/repository"
)

const maxIndexedContentRunes = 20000

// ArticleSearchRepository 倒排索引持久化（由 MySQLContentRepo 实现）。
type ArticleSearchRepository interface {
	ReplaceArticleSearchIndex(articleID string, weights map[string]int) error
	DeleteArticleSearchIndex(articleID string) error
	SearchArticlesByTokens(tokens []string, limit int) ([]repository.ArticleSearchHit, error)
}

// Indexer 将标题 / 摘要 / 正文离线分词后写入倒排索引。
type Indexer struct {
	tok *Tokenizer
	db  ArticleSearchRepository
}

func NewIndexer(tok *Tokenizer, db ArticleSearchRepository) *Indexer {
	return &Indexer{tok: tok, db: db}
}

func (ix *Indexer) truncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	rs := []rune(s)
	return string(rs[:maxRunes])
}

func (ix *Indexer) aggregateWeights(title, summary, content string) map[string]int {
	out := map[string]int{}
	add := func(field string, weight int) {
		for _, w := range ix.tok.Segment(field) {
			w = strings.TrimSpace(w)
			if w == "" {
				continue
			}
			out[w] += weight
		}
	}
	add(title, 8)
	add(summary, 5)
	add(ix.truncateRunes(content, maxIndexedContentRunes), 2)
	return out
}

// IndexArticle 全量替换某篇文章的索引项。
func (ix *Indexer) IndexArticle(articleID, title, summary, content string) error {
	w := ix.aggregateWeights(title, summary, content)
	return ix.db.ReplaceArticleSearchIndex(articleID, w)
}

// DeleteArticle 移除索引。
func (ix *Indexer) DeleteArticle(articleID string) error {
	return ix.db.DeleteArticleSearchIndex(articleID)
}

// Search 对用户查询分词后在倒排表中聚合打分。
func (ix *Indexer) Search(query string, limit int) ([]repository.ArticleSearchHit, []string, error) {
	tokens := ix.tok.Segment(query)
	if len(tokens) == 0 {
		return nil, tokens, nil
	}
	hits, err := ix.db.SearchArticlesByTokens(tokens, limit)
	return hits, tokens, err
}
