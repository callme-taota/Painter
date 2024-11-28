package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const FileStorageTableName = "file_storage"

type FileStorage struct {
	BaseTable `gorm:"-"`

	ID        int    `gorm:"primaryKey;autoIncrement"`
	FileName  string `gorm:"type:varchar(255);unique"`
	FilePath  string `gorm:"unique"`
	FileSize  int64
	FileType  string `gorm:"type:varchar(255)"`
	CreatedAt time.Time
}

func NewEmptyFileStorage() *FileStorage {
	return &FileStorage{
		BaseTable: &BaseTableImplement{},
	}
}

func (f *FileStorage) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&FileStorage{})
}

func (f *FileStorage) TableName() string {
	return FileStorageTableName
}

func (f *FileStorage) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	storage, ok := row.(*FileStorage)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(storage), nil
}

func (f *FileStorage) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	return nil
}

func (f *FileStorage) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var storages []FileStorage
	if err := query(db).Find(&storages).Error; err != nil {
		return err
	}

	for _, storage := range storages {
		if err := db.Delete(&storage).Error; err != nil {
			return err
		}
	}
	return nil
}

func (f *FileStorage) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var storages []FileStorage
	tx := query(db).Find(&storages)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range storages {
		result = append(result, &storages[i])
	}
	return result, tx, nil
}
