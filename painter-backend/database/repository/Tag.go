package repository

import (
	"errors"

	"gorm.io/gorm"
)

const TagTableName = "tag"

type Tag struct {
	BaseTable `gorm:"-"`

	TagID       int    `gorm:"primaryKey;not null,"`
	TagName     string `gorm:"type:varchar(255);unique"`
	Description string
}

func NewEmptyTag() *Tag {
	return &Tag{
		BaseTable: &BaseTableImplement{},
	}
}

func (t *Tag) Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(&Tag{})
	return err
}

func (t *Tag) TableName() string {
	return TagTableName
}

func (t *Tag) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	tag, ok := row.(*Tag)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(tag), nil
}

func (t *Tag) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	var tag = NewEmptyTag()
	if err := query(db).First(&tag).Error; err != nil {
		return err
	}

	if err := updater(tag); err != nil {
		return err
	}
	if err := db.Save(&tag).Error; err != nil {
		return err
	}
	return nil
}

func (t *Tag) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var tag Tag
	if err := query(db).First(&tag).Error; err != nil {
		return err
	}

	return db.Delete(&tag).Error
}

func (t *Tag) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var tags []Tag
	tx := query(db).Find(&tags)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range tags {
		result = append(result, &tags[i])
	}
	return result, tx, nil
}

func (t *Tag) Constructor() Table {
	return NewEmptyTag()
}
