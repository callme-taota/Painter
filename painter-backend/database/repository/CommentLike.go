package repository

import (
	"errors"

	"gorm.io/gorm"
)

const CommentLikeTableName = "comment_like"

type CommentLike struct {
	BaseTable `gorm:"-"`

	CommentID int `gorm:"uniqueIndex:com_like"`
	UserID    int `gorm:"uniqueIndex:com_like"`
}

func NewEmptyCommentLike() *CommentLike {
	return &CommentLike{
		BaseTable: &BaseTableImplement{},
	}
}

func (c *CommentLike) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&CommentLike{})
}

func (c *CommentLike) TableName() string {
	return CommentLikeTableName
}

func (c *CommentLike) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	commentLike, ok := row.(*CommentLike)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(commentLike), nil
}

func (c *CommentLike) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	return nil
}

func (c *CommentLike) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var like CommentLike
	if err := query(db).First(&like).Error; err != nil {
		return err
	}

	if err := db.Delete(&like).Error; err != nil {
		return err
	}
	return nil
}

func (c *CommentLike) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var likes []CommentLike
	tx := query(db).Find(&likes)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range likes {
		result = append(result, &likes[i])
	}
	return result, tx, nil
}
