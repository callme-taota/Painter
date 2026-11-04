package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const CommentTableName = "comment"

type Comment struct {
	BaseTable `gorm:"-"`

	CommentID  int `gorm:"primaryKey;autoIncrement"`
	Content    string
	UserID     int
	ArticleID  int
	CreateTime time.Time `gorm:"autoUpdateTime"`
}

func NewEmptyComment() *Comment {
	return &Comment{
		BaseTable: &BaseTableImplement{},
	}
}

func (c *Comment) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Comment{})
}

func (c *Comment) TableName() string {
	return CommentTableName
}

func (c *Comment) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	comment, ok := row.(*Comment)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(comment), nil
}

func (c *Comment) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	return updateRows(c, db, query, updater)
}

func (c *Comment) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	return deleteRows[Comment](db, query)
}

func (c *Comment) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var comments []Comment
	tx := query(db).Find(&comments)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range comments {
		comments[i].BaseTable = &BaseTableImplement{}
		result = append(result, &comments[i])
	}
	return result, tx, nil
}

func (c *Comment) Constructor() Table {
	return NewEmptyComment()
}
