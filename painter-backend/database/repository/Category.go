package repository

import (
	"errors"

	"gorm.io/gorm"
)

const CategoryTableName = "category"

type Category struct {
	BaseTable `gorm:"-"`

	CategoryID   int    `gorm:"primaryKey;autoIncrement"`
	CategoryName string `gorm:"type:varchar(255)"`
	Description  string
}

func NewEmptyCategory() *Category {
	return &Category{
		BaseTable: &BaseTableImplement{},
	}
}

func (c *Category) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Category{})
}

func (c *Category) TableName() string {
	return CategoryTableName
}

func (c *Category) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	category, ok := row.(*Category)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(category), nil
}

func (c *Category) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	var categories []Category
	if err := query(db).Find(&categories).Error; err != nil {
		return err
	}

	for i := range categories {
		categories[i].BaseTable = &BaseTableImplement{}
		if err := updater(&categories[i]); err != nil {
			return err
		}
		if err := db.Save(&categories[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (c *Category) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var categories []Category
	if err := query(db).Find(&categories).Error; err != nil {
		return err
	}

	for _, category := range categories {
		if err := db.Delete(&category).Error; err != nil {
			return err
		}
	}
	return nil
}

func (c *Category) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var categories []Category
	tx := query(db).Find(&categories)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range categories {
		categories[i].BaseTable = &BaseTableImplement{}
		result = append(result, &categories[i])
	}
	return result, tx, nil
}

func (c *Category) Constructor() Table {
	return NewEmptyCategory()
}
