package database

import (
	"errors"
	"fmt"

	"github.com/callme-taota/painter/painter-backend/database/repository"
	"github.com/callme-taota/painter/painter-backend/models"
	"github.com/callme-taota/painter/painter-backend/models/APIs/Response"

	"github.com/callme-taota/tolog"
	"gorm.io/gorm"

	"time"

	"github.com/callme-taota/painter/painter-backend/utils"
)

func CreateUser(username, email, nickname string, phoneNum int, headerField, password string) (int, error) {
	user := map[string]interface{}{
		"UserName":    username,
		"Email":       email,
		"NickName":    nickname,
		"PhoneNum":    phoneNum,
		"HeaderField": headerField,
	}
	return CreateUserV2(user, password)
}

func CreateUserV2(kv map[string]interface{}, password string) (int, error) {
	db := repository.GetDBImplement()
	user := repository.NewEmptyUser()
	for k, v := range kv {
		if user.CheckColumnsExist(k) {
			if err := user.SetValue(k, v); err != nil {
				tolog.Errorf("Set %s table cloumn %s faild: %v", repository.UserTableName, k, err).PrintAndWriteSafe()
			}
		}
	}
	tx := db.GetTransaction()
	_, err := db.UseTable(repository.UserTableName).Create(tx, user)
	if err != nil {
		tx.Rollback()
		return -1, err
	}
	password, _ = utils.HashPassword(password)
	err = CreateUserPassword(tx, user.ID, password)
	if err != nil {
		tx.Rollback()
		return -1, err
	}
	tx.Commit()

	return user.ID, err
}

func HasUser(key string) (bool, error) {
	return IsUserExist(key)
}

func IsUserExist(key string) (bool, error) {
	db := repository.GetDBImplement()
	tx := db.GetTransaction()
	res, _, err := db.UseTable(repository.UserTableName).Select(tx, func(g *gorm.DB) *gorm.DB {
		return g.Where("user_name = ? or email = ?", key, key)
	})
	if err != nil {
		tx.Rollback()
		return false, err
	}
	tx.Commit()
	return len(res) > 0, err
}

func UpdateUserName(id int, name string) error {
	return UpdateUser(id, "UserName", name)
}

func UpdateUserEmail(id int, email string) error {
	return UpdateUser(id, "Email", email)
}

func UpdateUserNickName(id int, nickname string) error {
	return UpdateUser(id, "NickName", nickname)
}

func UpdateUserPhoneNum(id int, number int) error {
	return UpdateUser(id, "PhoneNum", number)
}

func UpdateUserHeaderField(id int, headerField string) error {
	return UpdateUser(id, "HeaderField", headerField)
}

func UpdateUserProfile(id int, username, email, nickname string, number int) error {
	db := repository.GetDBImplement()
	tx := db.GetTransaction()
	err := UpdateUserUnix(tx, id, "UserName", username)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = UpdateUserUnix(tx, id, "Email", email)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = UpdateUserUnix(tx, id, "NickName", nickname)
	if err != nil {
		tx.Rollback()
		return err
	}
	err = UpdateUserUnix(tx, id, "PhoneNum", number)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func UpdateUser(id int, key string, value interface{}) error {
	db := repository.GetDBImplement()
	tx := db.GetTransaction()
	err := UpdateUserUnix(tx, id, key, value)
	return err
}

func UpdateUserUnix(db *gorm.DB, id int, key string, value interface{}) error {
	tb := repository.GetDBImplement().UseTable(repository.UserTableName)
	if !tb.CheckColumnsExist(key) {
		db.Rollback()
		return fmt.Errorf("column %s does not exist", key)
	}
	err := tb.Update(db, func(g *gorm.DB) *gorm.DB {
		return g.Where("id = ?", id)
	}, func(table repository.Table) error {
		return table.SetValue(key, value)
	})
	if err != nil {
		db.Rollback()
		return err
	}
	return db.Error
}

func GetUserIdUsingPhoneNum(phone string) (int, error) {
	return GetUserIDUsingIdentityKey(phone, "phone")
}

func GetUserIdUsingEmail(email string) (int, error) {
	return GetUserIDUsingIdentityKey(email, "email")
}

func GetUserIDUsingUserName(username string) (int, error) {
	return GetUserIDUsingIdentityKey(username, "userName")
}

func GetUserIDUsingIdentityKey(value string, keyType string) (int, error) {
	db := repository.GetDBImplement()
	whereClauses := "%s = ?"
	switch keyType {
	case "phone":
		whereClauses = fmt.Sprintf(whereClauses, "phone")
	case "email":
		whereClauses = fmt.Sprintf(whereClauses, "email")
	case "userName":
		whereClauses = fmt.Sprintf(whereClauses, "userName")
	}
	tx := db.GetTransaction()
	tb := db.UseTable(repository.UserTableName)
	res, _, err := tb.Select(tx, func(g *gorm.DB) *gorm.DB {
		return g.Where(whereClauses, value)
	})
	if err != nil {
		tx.Rollback()
		return -1, err
	}
	if len(res) == 0 {
		tx.Rollback()
		return -2, errors.Join(err, errors.New("no result"))
	}
	id, err := res[0].GetValue("ID")
	tx.Commit()
	return id.(int), err
}

func GetUserEmailUsingUserName(userName string) (string, error) {
	return GetUserEmailUsingUserNameV2(userName)
}

func GetUserEmailUsingUserNameV2(userName string) (string, error) {
	db := repository.GetDBImplement()
	tx := db.GetTransaction()
	tb := db.UseTable(repository.UserTableName)
	res, _, err := tb.Select(tx, func(g *gorm.DB) *gorm.DB {
		return g.Where("user_name = ?", userName)
	})
	if err != nil {
		tx.Rollback()
		return "", err
	}
	if len(res) == 0 {
		tx.Rollback()
		return "", errors.Join(err, errors.New("no result"))
	}
	email, err := res[0].GetValue("Email")
	tx.Commit()
	if err != nil {
		tx.Rollback()
		return "", err
	}
	return email.(string), nil
}

func CheckUserPassword(id int, password string) (bool, error) {
	return CheckUserPasswordV2(id, password)
}

func ResetPassWord(id int, oldpassword, newpassword string) error {
	return ResetPasswordV2(id, oldpassword, newpassword)
}

func GetUserInfo(id int) (models.UserTable, error) {
	db := repository.GetDBImplement().GetDB()
	user := models.UserTable{}
	res := db.First(&user, id)
	if res.Error != nil {
		return user, res.Error
	}
	return user, nil
}

func GetUserInfoV2(id int) (repository.User, error) {
	db := repository.GetDBImplement()
	tx := db.GetTransaction()
	tb := db.UseTable(repository.UserTableName)
	row, t, err := tb.Select(tx, func(g *gorm.DB) *gorm.DB {
		return g.Where("id = ?", id)
	})
	if t.Error != nil || err != nil || len(row) != 1 {
		tx.Rollback()
		return *repository.NewEmptyUser(), errors.Join(err, t.Error)
	}
	return *row[0].(*repository.User), nil
}

func GetUserSelfInfo(id int) (Response.SelfFullUser, error) {
	db := repository.GetDBImplement().GetDB()
	full := Response.SelfFullUser{}
	user := models.UserTable{}
	res := db.First(&user, id)
	if res.Error != nil {
		return full, res.Error
	}

	var art []models.ArticleTable
	res = db.Where("author = ?", id).Find(&art)
	articleNum := res.RowsAffected

	var totalCount int
	for _, a := range art {
		totalCount += a.ReadCount
	}

	coll := &models.CollectionTable{}
	res = db.Where("user_id", id).Find(&coll)
	collectionNum := res.RowsAffected

	followingNum, _ := GetFollowingNumber(id)
	followerNum, _ := GetFollowerNumber(id)

	full.UserInfo = user
	full.ArticleList = art
	full.ArticleNumber = int(articleNum)
	full.CollectionNumber = int(collectionNum)
	full.FollowingNumber = followingNum
	full.FollowerNumber = followerNum
	full.TotalCount = totalCount

	return full, nil
}

func GetUserFullInfo(id int) (Response.FullUser, error) {
	db := repository.GetDBImplement().GetDB()
	full := Response.FullUser{}

	var user Response.MiniUserFullInfo
	result := db.Table("user").Select("id, email, nick_name, header_field, created_at, last_login").Where("id = ?", id).First(&user)
	if result.Error != nil {
		tolog.Infof("Error while GetUserFullInfo %e", result.Error).PrintAndWriteSafe()
		return full, result.Error
	}

	articleNum, err := GetArticleCountByAuthor(id)
	if err != nil {
		tolog.Infof("Error while GetUserFullInfo %e", result.Error).PrintAndWriteSafe()
		return full, result.Error
	}
	list, err := GetArticlesByAuthor(id, articleNum, 0)
	if err != nil {
		tolog.Infof("Error while GetUserFullInfo %e", result.Error).PrintAndWriteSafe()
		return full, result.Error
	}
	art, err := GetArticleByIntList(list)
	if err != nil {
		tolog.Infof("Error while GetUserFullInfo %e", result.Error).PrintAndWriteSafe()
		return full, result.Error
	}

	collectionNum, _ := GetCollectionCountByUser(id)

	followingNum, _ := GetFollowingNumber(id)
	followerNum, _ := GetFollowerNumber(id)

	full.UserInfo = user
	full.ArticleList = art
	full.ArticleNumber = articleNum
	full.CollectionNumber = collectionNum
	full.FollowingNumber = followingNum
	full.FollowerNumber = followerNum
	full.Following = false
	full.Self = false

	return full, nil
}

func GetAdminFlag(id int) (bool, error) {
	return CheckUserAdmin(id)
}

func UpdateUserLoginTime(id int) (bool, error) {
	db := repository.GetDBImplement().GetDB()
	user := models.UserTable{}
	result := db.First(&user, id)
	if result.Error != nil {
		tolog.Infof("Error while UpdateUserLoginTime %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	user.LastLogin = time.Now().Add(-time.Hour)
	result = db.Save(&user)
	if result.Error != nil {
		tolog.Infof("Error while UpdateUserLoginTime %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	return true, nil
}
