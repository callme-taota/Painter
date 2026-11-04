package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const ArticleLikeTableName = "article_like"

type ArticleLike struct {
	BaseTable `gorm:"-"`

	ArticleID int `gorm:"uniqueIndex:art_like"`
	UserID    int `gorm:"uniqueIndex:art_like"`
	CreatedAt time.Time
}

func NewEmptyArticleLikeTable() *ArticleLike {
	return &ArticleLike{
		BaseTable: &BaseTableImplement{},
	}
}

func (a *ArticleLike) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&ArticleLike{})
}

func (a *ArticleLike) TableName() string {
	return ArticleLikeTableName
}

func (a *ArticleLike) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	article, ok := row.(*ArticleLike)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(article), nil
}

func (a *ArticleLike) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	return updateRows(a, db, query, updater)
}

func (a *ArticleLike) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	return deleteRows[ArticleLike](db, query)
}

func (a *ArticleLike) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var articles []ArticleLike
	tx := query(db).Find(&articles)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range articles {
		articles[i].BaseTable = &BaseTableImplement{}
		result = append(result, &articles[i])
	}
	return result, tx, nil
}

func (a *ArticleLike) Constructor() Table {
	return NewEmptyArticleLikeTable()
}
