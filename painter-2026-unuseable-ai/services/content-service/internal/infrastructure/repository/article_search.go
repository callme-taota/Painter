package repository

import (
	"gorm.io/gorm"
)

// ArticleSearchToken 离线分词后的倒排索引项（持久化于 MySQL）。
type ArticleSearchToken struct {
	ID        uint   `gorm:"primaryKey"`
	ArticleID string `gorm:"column:article_id;size:64;not null;uniqueIndex:uk_article_search_pair"`
	Token     string `gorm:"column:token;size:191;not null;uniqueIndex:uk_article_search_pair"`
	Weight    int    `gorm:"column:weight;not null"`
}

func (ArticleSearchToken) TableName() string {
	return "article_search_tokens"
}

// ArticleSearchHit 聚合打分结果。
type ArticleSearchHit struct {
	ArticleID string `gorm:"column:article_id"`
	Score     int    `gorm:"column:score"`
}

func (r *MySQLContentRepo) ReplaceArticleSearchIndex(articleID string, weights map[string]int) error {
	if articleID == "" {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("article_id = ?", articleID).Delete(&ArticleSearchToken{}).Error; err != nil {
			return err
		}
		if len(weights) == 0 {
			return nil
		}
		batch := make([]ArticleSearchToken, 0, len(weights))
		for tok, w := range weights {
			if tok == "" || w <= 0 {
				continue
			}
			batch = append(batch, ArticleSearchToken{ArticleID: articleID, Token: tok, Weight: w})
		}
		if len(batch) == 0 {
			return nil
		}
		return tx.CreateInBatches(batch, 128).Error
	})
}

func (r *MySQLContentRepo) DeleteArticleSearchIndex(articleID string) error {
	if articleID == "" {
		return nil
	}
	return r.db.Where("article_id = ?", articleID).Delete(&ArticleSearchToken{}).Error
}

func (r *MySQLContentRepo) SearchArticlesByTokens(tokens []string, limit int) ([]ArticleSearchHit, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if len(tokens) == 0 {
		return nil, nil
	}
	var hits []ArticleSearchHit
	err := r.db.Model(&ArticleSearchToken{}).
		Select("article_id as article_id, SUM(weight) as score").
		Where("token IN ?", tokens).
		Group("article_id").
		Order("score DESC").
		Limit(limit).
		Scan(&hits).Error
	return hits, err
}

// ListArticlesBatch 用于后台回填索引的稳定分页。
func (r *MySQLContentRepo) ListArticlesBatch(offset, limit int) ([]ArticleRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []ArticleRecord
	err := r.db.Order("article_id asc").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, err
}

// GetArticlesByIDsPreserveOrder 按给定顺序返回文章。
func (r *MySQLContentRepo) GetArticlesByIDsPreserveOrder(ids []string) ([]ArticleRecord, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []ArticleRecord
	if err := r.db.Where("article_id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	by := make(map[string]ArticleRecord, len(rows))
	for _, row := range rows {
		by[row.ArticleID] = row
	}
	out := make([]ArticleRecord, 0, len(ids))
	for _, id := range ids {
		if row, ok := by[id]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}
