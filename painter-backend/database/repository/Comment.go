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
	var comments []Comment
	if err := query(db).Find(&comments).Error; err != nil {
		return err
	}

	for i := range comments {
		comments[i].BaseTable = &BaseTableImplement{}
		if err := updater(&comments[i]); err != nil {
			return err
		}
		if err := db.Save(&comments[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (c *Comment) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var comments []Comment
	if err := query(db).Find(&comments).Error; err != nil {
		return err
	}

	for _, comment := range comments {
		if err := db.Delete(&comment).Error; err != nil {
			return err
		}
	}
	return nil
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
