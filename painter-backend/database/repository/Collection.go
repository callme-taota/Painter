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
	return updateRows(c, db, query, updater)
}

func (c *Collection) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	return deleteRows[Collection](db, query)
}

func (c *Collection) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var collections []Collection
	tx := query(db).Find(&collections)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range collections {
		collections[i].BaseTable = &BaseTableImplement{}
		result = append(result, &collections[i])
	}
	return result, tx, nil
}

func (c *Collection) Constructor() Table {
	return NewEmptyCollection()
}
