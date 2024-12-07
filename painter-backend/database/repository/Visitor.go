package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const VisitorRecordTableName = "visitor_record"

type VisitorRecord struct {
	BaseTable `gorm:"-"`

	ID    int `gorm:"primaryKey;autoIncrement"`
	Date  time.Time
	Total int
}

func NewEmptyVisitorRecord() *VisitorRecord {
	return &VisitorRecord{
		BaseTable: &BaseTableImplement{},
	}
}

func (v *VisitorRecord) Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(&VisitorRecord{})
	return err
}

func (v *VisitorRecord) TableName() string {
	return VisitorRecordTableName
}

func (v *VisitorRecord) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	recd, ok := row.(*VisitorRecord)
	if !ok {
		return nil, errors.New("invalid row type")
	}

	return db.Create(recd), nil
}

func (v *VisitorRecord) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	var recd = NewEmptyVisitorRecord()
	if err := query(db).First(&recd).Error; err != nil {
		return err
	}

	if err := updater(recd); err != nil {
		return err
	}
	if err := db.Save(&recd).Error; err != nil {
		return err
	}
	return nil
}

func (v *VisitorRecord) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	return nil
}

func (v *VisitorRecord) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var recd = NewEmptyVisitorRecord()
	tx := query(db).First(&recd)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	result := []Table{recd}
	return result, tx, nil
}

func (v *VisitorRecord) Constructor() Table {
	return NewEmptyVisitorRecord()
}
