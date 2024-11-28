package repository

import (
	"errors"
	"time"

	"github.com/callme-taota/tolog"
	"gorm.io/gorm"
)

const PainterSettingTableName = "painter_setting"

type SettingKey string

const (
	MailFrom     SettingKey = "mail_from"
	MailPassword SettingKey = "mail_password"
	MailSmtpHost SettingKey = "mail_smtphost"
	MailSmtpPort SettingKey = "mail_smtpport"
	MailActive   SettingKey = "mail_active"
	SiteName     SettingKey = "site_name"
	GithubHref   SettingKey = "github_href"
	ICPCode      SettingKey = "icp_code"
	CanRegister  SettingKey = "can_register"
	EntryArticle SettingKey = "entry_article"
)

var settingKey = []SettingKey{MailFrom, MailPassword, MailSmtpHost, MailSmtpPort, MailActive, SiteName, GithubHref, ICPCode, CanRegister, EntryArticle}

type PainterSetting struct {
	BaseTable `gorm:"-"`

	ID          int    `gorm:"primaryKey;autoIncrement"`
	Name        string `gorm:"type:varchar(255);unique"`
	Value       string `gorm:"type:text"`
	Description string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func newEmptyPainterSetting() *PainterSetting {
	return &PainterSetting{
		BaseTable: &BaseTableImplement{},
	}
}

func InitSettings() {
	for _, setting := range settingKey {
		checkKeyExistOrCreate(string(setting))
	}
}

func (p PainterSetting) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&PainterSetting{})
}

func (p PainterSetting) TableName() string {
	return PainterSettingTableName
}

func (p PainterSetting) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	setting, ok := row.(*PainterSetting)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(setting), nil
}

func (p PainterSetting) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	var setting PainterSetting
	if err := query(db).First(&setting).Error; err != nil {
		return err
	}

	if err := updater(&setting); err != nil {
		return err
	}
	if err := db.Save(&setting).Error; err != nil {
		return err
	}
	return nil
}

func (p PainterSetting) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	return nil
}

func (p PainterSetting) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var setting PainterSetting
	tx := query(db).First(&setting)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	result := []Table{&setting}
	return result, tx, nil
}

func checkKeyExistOrCreate(key string) (bool, error) {
	db := dbImplement.GetDB()
	tb := dbImplement.UseTable(PainterSettingTableName)
	_, t, _ := tb.Select(db, func(g *gorm.DB) *gorm.DB {
		return g.Where("name = ?", key)
	})
	// exist
	if t.RowsAffected == 1 {
		return true, nil
	}
	if t.RowsAffected == 0 {
		setting := newEmptyPainterSetting()
		err := setting.SetValues(map[string]interface{}{
			"Name":  key,
			"Value": "0",
		})
		if err != nil {
			return false, err
		}
		res, err := tb.Create(db, setting)
		if err != nil {
			tolog.Infof("Error while create setting %e", res.Error).PrintAndWriteSafe()
			return false, err
		}
	}
	return true, nil
}
