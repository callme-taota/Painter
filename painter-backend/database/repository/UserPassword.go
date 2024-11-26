package repository

import (
	"errors"

	"gorm.io/gorm"
)

const UserPasswordTableName = "userpass"

type UserPassword struct {
	BaseTable

	ID       int
	Password string `gorm:"type:varchar(255)"`
}

func NewEmptyUserPassword() *UserPassword {
	return &UserPassword{}
}

func (u *UserPassword) Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(&UserPassword{
		BaseTable: &BaseTableImplement{},
	})
	return err
}

func (u *UserPassword) TableName() string {
	return UserPasswordTableName
}

func (u *UserPassword) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	userPwd, ok := row.(*UserPassword)
	if !ok {
		return nil, errors.New("invalid row type")
	}

	return db.Create(userPwd), nil
}

func (u *UserPassword) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	var userPwd UserPassword
	if err := query(db).First(&userPwd).Error; err != nil {
		return err
	}

	if err := updater(&userPwd); err != nil {
		return err
	}
	if err := db.Save(&userPwd).Error; err != nil {
		return err
	}
	return nil
}

func (u *UserPassword) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var userPwd UserPassword
	if err := query(db).First(&userPwd).Error; err != nil {
		return err
	}

	return db.Delete(&userPwd).Error
}

func (u *UserPassword) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var userPwd UserPassword
	tx := query(db).First(&userPwd)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	result := []Table{&userPwd}
	return result, tx, nil
}
