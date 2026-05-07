package repository

import (
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ConfigRecord struct {
	ID        uint `gorm:"primaryKey"`
	Namespace string
	Key       string
	Value     string
	ValueType string
	UpdatedAt time.Time
}

type SettingRecord struct {
	ID          uint `gorm:"primaryKey"`
	SiteName    string
	ICPCode     string
	Github      string
	CanRegister bool
	UpdatedAt   time.Time
}

type FileRecord struct {
	ID        uint `gorm:"primaryKey"`
	FileName  string
	URL       string
	CreatedAt time.Time
}

type MySQLRepo struct {
	db *gorm.DB
}

func NewMySQLRepo(dsn string) (*MySQLRepo, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&ConfigRecord{}, &SettingRecord{}, &FileRecord{}); err != nil {
		return nil, err
	}
	repo := &MySQLRepo{db: db}
	if err := repo.seed(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *MySQLRepo) seed() error {
	var settingCount int64
	if err := r.db.Model(&SettingRecord{}).Count(&settingCount).Error; err != nil {
		return err
	}
	if settingCount == 0 {
		if err := r.db.Create(&SettingRecord{
			SiteName:    "Painter 2026",
			ICPCode:     "",
			Github:      "",
			CanRegister: true,
		}).Error; err != nil {
			return err
		}
	}
	var configCount int64
	if err := r.db.Model(&ConfigRecord{}).Count(&configCount).Error; err != nil {
		return err
	}
	if configCount == 0 {
		return r.db.Create([]ConfigRecord{
			{Namespace: "public", Key: "feature.comment.enabled", Value: "true", ValueType: "boolean"},
			{Namespace: "public", Key: "theme.web.primaryColor", Value: "#4f46e5", ValueType: "string"},
		}).Error
	}
	return nil
}

func (r *MySQLRepo) ListConfigs(namespace string) ([]ConfigRecord, error) {
	var items []ConfigRecord
	err := r.db.Where("namespace = ?", namespace).Find(&items).Error
	return items, err
}

func (r *MySQLRepo) GetSetting() (SettingRecord, error) {
	var setting SettingRecord
	err := r.db.Order("id asc").First(&setting).Error
	return setting, err
}

func (r *MySQLRepo) UpdateSetting(updates map[string]interface{}) (SettingRecord, error) {
	setting, err := r.GetSetting()
	if err != nil {
		return SettingRecord{}, err
	}
	if err := r.db.Model(&setting).Updates(updates).Error; err != nil {
		return SettingRecord{}, err
	}
	return setting, nil
}

func (r *MySQLRepo) SaveFile(fileName, url string) error {
	return r.db.Create(&FileRecord{FileName: fileName, URL: url}).Error
}

func (r *MySQLRepo) CountFiles() (int64, error) {
	var count int64
	err := r.db.Model(&FileRecord{}).Count(&count).Error
	return count, err
}

func (r *MySQLRepo) CountConfigs(namespace string) (int64, error) {
	var count int64
	err := r.db.Model(&ConfigRecord{}).Where("namespace = ?", namespace).Count(&count).Error
	return count, err
}
