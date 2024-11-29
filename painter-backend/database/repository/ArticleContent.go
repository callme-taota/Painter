package repository

import (
	"errors"

	"gorm.io/gorm"
)

const ArticleContentTableName = "article_content"

type ArticleContent struct {
	BaseTable `gorm:"-"`

	ArticleID int    `gorm:"uniqueIndex:art_cont"`
	Content   string `gorm:"type:text;uniqueIndex:art_cont,length:12"`
}

func NewEmptyArticleContent() *ArticleContent {
	return &ArticleContent{
		BaseTable: &BaseTableImplement{},
	}
}

func (a *ArticleContent) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&ArticleContent{})
}

func (a *ArticleContent) TableName() string {
	return ArticleContentTableName
}

func (a *ArticleContent) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	articleContent, ok := row.(*ArticleContent)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(articleContent), nil
}

func (a *ArticleContent) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	var articles []ArticleContent
	if err := query(db).Find(&articles).Error; err != nil {
		return err
	}

	for i := range articles {
		if err := updater(&articles[i]); err != nil {
			return err
		}
		if err := db.Save(&articles[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *ArticleContent) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var articles []ArticleContent
	if err := query(db).Find(&articles).Error; err != nil {
		return err
	}

	for _, article := range articles {
		if err := db.Delete(&article).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *ArticleContent) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var articles []ArticleContent
	tx := query(db).Find(&articles)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range articles {
		result = append(result, &articles[i])
	}
	return result, tx, nil
}
