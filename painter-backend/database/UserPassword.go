package database

import (
	"errors"
	"painter-server-new/utils"

	"gorm.io/gorm"
)

const UserPasswordTableName = "userpass"

type UserPassword struct {
	BaseTableImplement

	ID       int
	Password string `gorm:"type:varchar(255)"`
}

func (u *UserPassword) Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(&UserPassword{})
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
		return nil, tx, err
	}

	result := []Table{&userPwd}
	return result, tx, nil
}

func CheckUserPasswordV2(id int, password string) (bool, error) {
	tx := db.GetTransaction()
	tb := db.UseTable(UserPasswordTableName)
	res, t, err := tb.Select(tx, func(g *gorm.DB) *gorm.DB {
		return g.Where("id = ?", id)
	})
	if err != nil {
		tx.Rollback()
		return false, err
	}
	if len(res) != 1 {
		tx.Rollback()
		return false, err
	}
	if !res[0].CheckColumnsExist("Password") {
		tx.Rollback()
		return false, err
	}
	hashPassword, err := res[0].GetValue("Password")
	if err != nil {
		tx.Rollback()
		return false, err
	}
	ok := utils.CheckPasswordHash(password, hashPassword.(string))
	if t.RowsAffected == 1 && ok {
		return true, nil
	}
	return false, t.Error
}
