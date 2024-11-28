package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const FollowTableName = "follow"

type Follow struct {
	BaseTable `gorm:"-"`

	FollowID    int       `gorm:"primaryKey;autoIncrement"`
	FollowerID  int       `gorm:"uniqueIndex:fl;not null"`
	FollowingID int       `gorm:"uniqueIndex:fl;not null"`
	FollowTime  time.Time `gorm:"autoCreateTime"`
}

func NewEmptyFollow() *Follow {
	return &Follow{
		BaseTable: &BaseTableImplement{},
	}
}

func (f *Follow) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Follow{})
}

func (f *Follow) TableName() string {
	return FollowTableName
}

func (f *Follow) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	follow, ok := row.(*Follow)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(follow), nil
}

func (f *Follow) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	return nil
}

func (f *Follow) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var follows []Follow
	if err := query(db).Find(&follows).Error; err != nil {
		return err
	}

	for _, follow := range follows {
		if err := db.Delete(&follow).Error; err != nil {
			return err
		}
	}
	return nil
}

func (f *Follow) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var follows []Follow
	tx := query(db).Find(&follows)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range follows {
		result = append(result, &follows[i])
	}
	return result, tx, nil
}
