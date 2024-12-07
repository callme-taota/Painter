package repository

import (
	"errors"

	"gorm.io/gorm"
)

const ArticleTagTableName = "article_tag"

type ArticleTag struct {
	BaseTable `gorm:"-"`

	ArticleID int `gorm:"uniqueIndex:art_tag"`
	TagID     int `gorm:"uniqueIndex:art_tag"`
}

func NewEmptyArticleTag() *ArticleTag {
	return &ArticleTag{
		BaseTable: &BaseTableImplement{},
	}
}

func (a *ArticleTag) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&ArticleTag{})
}

func (a *ArticleTag) TableName() string {
	return ArticleTagTableName
}

func (a *ArticleTag) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	article, ok := row.(*ArticleTag)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(article), nil
}

func (a *ArticleTag) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	var articleTags []ArticleTag
	if err := query(db).Find(&articleTags).Error; err != nil {
		return err
	}

	for i := range articleTags {
		articleTags[i].BaseTable = &BaseTableImplement{}
		if err := updater(&articleTags[i]); err != nil {
			return err
		}
		if err := db.Save(&articleTags[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *ArticleTag) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var articleTags []ArticleTag
	if err := query(db).Find(&articleTags).Error; err != nil {
		return err
	}

	for _, articleTag := range articleTags {
		if err := db.Delete(&articleTag).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *ArticleTag) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var articleTags []ArticleTag
	tx := query(db).Find(&articleTags)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range articleTags {
		articleTags[i].BaseTable = &BaseTableImplement{}
		result = append(result, &articleTags[i])
	}
	return result, tx, nil
}

func (a *ArticleTag) Constructor() Table {
	return NewEmptyArticleTag()
}
