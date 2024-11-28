package repository

import (
	"errors"

	"gorm.io/gorm"
)

const CollectionTableName = "collection"

type Collection struct {
	BaseTable `gorm:"-"`

	CollectionID   int `gorm:"primaryKey;autoIncrement"`
	UserID         int `gorm:"uniqueIndex:collection_ua"`
	ArticleID      int `gorm:"uniqueIndex:collection_ua"`
	CollectionTime int `gorm:"autoUpdateTime"`
}

func NewEmptyCollection() *Collection {
	return &Collection{
		BaseTable: &BaseTableImplement{},
	}
}

func (c *Collection) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Collection{})
}

func (c *Collection) TableName() string {
	return CollectionTableName
}

func (c *Collection) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	collection, ok := row.(*Collection)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(collection), nil
}

func (c *Collection) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	var collections []Collection
	if err := query(db).Find(&collections).Error; err != nil {
		return err
	}

	for i := range collections {
		if err := updater(&collections[i]); err != nil {
			return err
		}
		if err := db.Save(&collections[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (c *Collection) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var collections []Collection
	if err := query(db).Find(&collections).Error; err != nil {
		return err
	}

	for _, collection := range collections {
		if err := db.Delete(&collection).Error; err != nil {
			return err
		}
	}
	return nil
}

func (c *Collection) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var collections []Collection
	tx := query(db).Find(&collections)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range collections {
		result = append(result, &collections[i])
	}
	return result, tx, nil
}
