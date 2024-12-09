package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const HistoryTableName = "history"

type History struct {
	BaseTable `gorm:"-"`

	HistoryID   int `gorm:"primaryKey;autoIncrement"`
	UserID      int `gorm:"index"`
	ArticleID   int
	HistoryTime time.Time
}

func NewEmptyHistory() *History {
	return &History{
		BaseTable: &BaseTableImplement{},
	}
}

func (h *History) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&History{})
}

func (h *History) TableName() string {
	return HistoryTableName
}

func (h *History) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	history, ok := row.(*History)
	if !ok {
		return nil, errors.New("invalid row type")
	}

	return db.Create(history), nil
}

func (h *History) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	var history = NewEmptyHistory()
	if err := query(db).First(&history).Error; err != nil {
		return err
	}

	if err := updater(history); err != nil {
		return err
	}
	if err := db.Save(&history).Error; err != nil {
		return err
	}
	return nil
}

func (h *History) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var histories []History
	if err := query(db).Find(&histories).Error; err != nil {
		return err
	}

	for _, user := range histories {
		if err := db.Delete(&user).Error; err != nil {
			return err
		}
	}
	return nil
}

func (h *History) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var history []History
	tx := query(db).Find(&history)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range history {
		history[i].BaseTable = &BaseTableImplement{}
		result = append(result, &history[i])
	}
	return result, tx, nil
}

func (h *History) Constructor() Table {
	return NewEmptyHistory()
}
