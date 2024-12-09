package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const ArticleTableName = "article"

type Article struct {
	BaseTable `gorm:"-"`

	ArticleID  int `gorm:"primaryKey;autoIncrement"`
	Title      string
	Author     int
	Summary    string
	ReadCount  int
	Status     int `gorm:"comment:0 草稿，1 发布，2 隐藏，3 限制，4 封禁'"`
	CategoryID int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

func NewEmptyArticle() *Article {
	return &Article{
		BaseTable: &BaseTableImplement{},
	}
}

func (a *Article) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Article{})
}

func (a *Article) TableName() string {
	return ArticleTableName
}

func (a *Article) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	article, ok := row.(*Article)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(article), nil
}

func (a *Article) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	var articles []Article
	if err := query(db).Find(&articles).Error; err != nil {
		return err
	}

	for i := range articles {
		articles[i].BaseTable = &BaseTableImplement{}
		if err := updater(&articles[i]); err != nil {
			return err
		}
		if err := db.Save(&articles[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *Article) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var articles []Article
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

func (a *Article) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var articles []Article
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

func (a *Article) Constructor() Table {
	return NewEmptyArticle()
}
