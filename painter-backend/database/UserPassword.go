package database

import (
	"errors"

	"github.com/callme-taota/painter/painter-backend/database/repository"
	"github.com/callme-taota/painter/painter-backend/utils"

	"gorm.io/gorm"
)

func CheckUserPasswordV2(id int, password string) (bool, error) {
	db := repository.GetDBImplement()
	tx := db.GetTransaction()
	tb := db.UseTable(repository.UserPasswordTableName)
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
	if !res[0].CheckColumnsExist("Password", res[0]) {
		tx.Rollback()
		return false, err
	}
	hashPassword, err := res[0].GetValue("Password", res[0])
	if err != nil {
		tx.Rollback()
		return false, err
	}
	ok := utils.CheckPasswordHash(password, hashPassword.(string))
	if t.RowsAffected == 1 && ok {
		tx.Rollback()
		return true, nil
	}
	tx.Commit()
	return false, t.Error
}

func ResetPasswordV2(id int, oldPsw, newPsw string) error {
	db := repository.GetDBImplement()
	tx := db.GetTransaction()
	tb := db.UseTable(repository.UserPasswordTableName)
	res, t, err := tb.Select(tx, func(g *gorm.DB) *gorm.DB {
		return g.Where("id = ?", id)
	})
	if t.Error != nil || err != nil || len(res) != 1 {
		tx.Rollback()
		return t.Error
	}
	old, err := utils.HashPassword(oldPsw)
	if err != nil {
		tx.Rollback()
		return err
	}
	userPassword, err := res[0].GetValue("Password", res[0])
	if err != nil {
		tx.Rollback()
		return err
	}
	if userPassword != old {
		return errors.New("password not correct! ")
	}
	err = tb.Update(tx, func(g *gorm.DB) *gorm.DB {
		return g.Where("id = ?", id)
	}, func(table repository.Table) error {
		return table.SetValue("Password", newPsw, table)
	})
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func CreateUserPassword(db *gorm.DB, id int, psw string) error {
	userPassword := repository.NewEmptyUserPassword()
	err := userPassword.SetValue("ID", id, userPassword)
	if err != nil {
		return err
	}
	err = userPassword.SetValue("Password", psw, userPassword)
	if err != nil {
		return err
	}
	_, err = userPassword.Create(db, userPassword)
	if err != nil {
		return err
	}
	return nil
}
