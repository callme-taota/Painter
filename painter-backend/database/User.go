package database

import (
	"errors"
	"fmt"
	"painter-server-new/models"
	"painter-server-new/models/APIs/Response"

	"github.com/callme-taota/tolog"
	"gorm.io/gorm"

	"painter-server-new/utils"
	"time"
)

const UserTableName = "user"

type User struct {
	BaseTableImplement

	ID          int    `gorm:"primaryKey;autoIncrement"`
	UserName    string `gorm:"type:varchar(255);unique"`
	Email       string `gorm:"type:varchar(255);unique"`
	AdminFlag   int    `gorm:"type:tinyint"`
	UserGroup   int
	LastLogin   time.Time
	NickName    string `gorm:"type:varchar(255)"`
	PhoneNum    int    `gorm:"type:int"`
	HeaderField string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt
}

func newEmptyUser() *User {
	return &User{}
}

func (u *User) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	user, ok := row.(*User)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(user), nil
}

func (u *User) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	var users []User
	if err := query(db).Find(&users).Error; err != nil {
		return err
	}

	for i := range users {
		if err := updater(&users[i]); err != nil {
			return err
		}
		if err := db.Save(&users[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (u *User) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var users []User
	if err := query(db).Find(&users).Error; err != nil {
		return err
	}

	for _, user := range users {
		if err := db.Delete(&user).Error; err != nil {
			return err
		}
	}
	return nil
}

func (u *User) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var users []User
	tx := query(db).Find(&users)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range users {
		result = append(result, &users[i])
	}
	return result, tx, nil
}

func (u *User) TableName() string {
	return UserTableName
}

func (u *User) Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(&User{})
	return err
}

func CreateUser(username, email, nickname string, phoneNum int, headerField, password string) (int, error) {
	user := models.UserTable{
		UserName:    username,
		Email:       email,
		NickName:    nickname,
		PhoneNum:    phoneNum,
		HeaderField: headerField,
		UserGroup:   3,
		LastLogin:   time.Now(),
	}
	err := DbEngine.Create(&user).Error
	if err != nil {
		return -1, err
	}
	id := user.ID
	password, _ = utils.HashPassword(password)
	userpass := models.UserPassTable{
		ID:       id,
		Password: password,
	}
	err = DbEngine.Create(&userpass).Error
	if err != nil {
		return -2, err
	}
	return user.ID, nil
}

func CreateUserV2(kv map[string]interface{}) (int, error) {
	user := newEmptyUser()
	for k, v := range kv {
		if user.CheckColumnsExist(k) {
			if err := user.SetValue(k, v); err != nil {
				tolog.Errorf("Set %s table cloumn %s faild: %v", UserTableName, k, err).PrintAndWriteSafe()
			}
		}
	}
	tx := db.GetTransaction()
	_, err := db.UseTable(UserTableName).Create(tx, user)
	if err != nil {
		tx.Rollback()
		return -1, err
	}
	tx.Commit()

	return user.ID, err
}

func HasUser(key string) (bool, error) {
	var user []models.UserTable
	res := DbEngine.Where("user_name = ? or email = ?", key, key).Find(&user)
	if res.Error != nil {
		tolog.Infof("Error while hasuser %e", res.Error).PrintAndWriteSafe()
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func IsUserExist(key string) (bool, error) {
	tx := db.GetTransaction()
	res, _, err := db.UseTable(UserTableName).Select(tx, func(g *gorm.DB) *gorm.DB {
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
	user := &models.UserTable{}
	res := DbEngine.First(&user, id)
	if res.Error != nil {
		return res.Error
	}
	user.UserName = name
	res = DbEngine.Save(&user)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func UpdateUserEmail(id int, email string) error {
	user := &models.UserTable{}
	res := DbEngine.First(&user, id)
	if res.Error != nil {
		return res.Error
	}
	user.Email = email
	res = DbEngine.Save(&user)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func UpdateUserNickName(id int, nickname string) error {
	user := &models.UserTable{}
	res := DbEngine.First(&user, id)
	if res.Error != nil {
		return res.Error
	}
	user.NickName = nickname
	res = DbEngine.Save(&user)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func UpdateUserPhoneNum(id int, number int) error {
	user := &models.UserTable{}
	res := DbEngine.First(&user, id)
	if res.Error != nil {
		return res.Error
	}
	user.PhoneNum = number
	res = DbEngine.Save(&user)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func UpdateUserHeaderField(id int, headerField string) error {
	res := DbEngine.Model(&models.UserTable{}).Where("id = ?", id).Update("header_field", headerField)
	if res.Error != nil {
		tolog.Errorf("Error while UpdateUserHeaderField %v", res.Error).PrintAndWriteSafe()
		return res.Error
	}
	return nil
}

func UpdateUserProfile(id int, username, email, nickname string, number int) error {
	user := &models.UserTable{}
	res := DbEngine.First(&user, id)
	if res.Error != nil {
		return res.Error
	}
	user.UserName = username
	user.Email = email
	user.NickName = nickname
	user.PhoneNum = number
	DbEngine.Save(&user)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func UpdateUser(id int, key string, value interface{}) error {
	tx := db.GetTransaction()
	tb := db.UseTable(UserTableName)
	if !tb.CheckColumnsExist(key) {
		tx.Rollback()
		return fmt.Errorf("column %s does not exist", key)
	}
	err := tb.Update(tx, func(g *gorm.DB) *gorm.DB {
		return g.Where("id = ?", id)
	}, func(table Table) error {
		return table.SetValue(key, value)
	})
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func GetUserIdUsingPhoneNum(phone string) (int, error) {
	user := &models.UserTable{}
	err := DbEngine.Where("phone_num = ?", phone).First(&user).Error
	if err != nil {
		return -1, err
	}
	return user.ID, nil
}

func GetUserIdUsingEmail(email string) (int, error) {
	user := &models.UserTable{}
	err := DbEngine.Where("email = ?", email).First(&user).Error
	if err != nil {
		return -1, err
	}
	return user.ID, nil
}

func GetUserIDUsingUserName(username string) (int, error) {
	user := &models.UserTable{}
	err := DbEngine.Where("user_name = ?", username).First(&user).Error
	if err != nil {
		return -1, err
	}
	return user.ID, nil
}

func GetUserIDUsingIdentityKey(value string, keyType string) (int, error) {
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
	tb := db.UseTable(UserTableName)
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
	user := &models.UserTable{}
	err := DbEngine.Where("user_name = ?", userName).First(&user).Error
	if err != nil {
		return "", err
	}
	return user.Email, nil
}

func GetUserEmailUsingUserNameV2(userName string) (string, error) {
	tx := db.GetTransaction()
	tb := db.UseTable(UserTableName)
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
	userpass := &models.UserPassTable{}
	res := DbEngine.Where("ID = ?", id).First(&userpass)
	hashPassword := userpass.Password
	ok := utils.CheckPasswordHash(password, hashPassword)
	if res.RowsAffected == 1 && ok {
		return true, nil
	}
	return false, res.Error
}

func ResetPassWord(id int, oldpassword, newpassword string) error {
	userpass := &models.UserPassTable{}
	res := DbEngine.First(&userpass, id)
	if res.Error != nil {
		return res.Error
	}
	old, err := utils.HashPassword(oldpassword)
	if err != nil {
		return err
	}
	if userpass.Password != old {
		return errors.New("password not correct! ")
	}
	userpass.Password = newpassword
	DbEngine.Save(userpass)
	return nil
}

func GetUserInfo(id int) (models.UserTable, error) {
	user := models.UserTable{}
	res := DbEngine.First(&user, id)
	if res.Error != nil {
		return user, res.Error
	}
	return user, nil
}

func GetUserSelfInfo(id int) (Response.SelfFullUser, error) {
	full := Response.SelfFullUser{}
	user := models.UserTable{}
	res := DbEngine.First(&user, id)
	if res.Error != nil {
		return full, res.Error
	}

	var art []models.ArticleTable
	res = DbEngine.Where("author = ?", id).Find(&art)
	articleNum := res.RowsAffected

	var totalCount int
	for _, a := range art {
		totalCount += a.ReadCount
	}

	coll := &models.CollectionTable{}
	res = DbEngine.Where("user_id", id).Find(&coll)
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
	full := Response.FullUser{}

	var user Response.MiniUserFullInfo
	result := DbEngine.Table("user").Select("id, email, nick_name, header_field, created_at, last_login").Where("id = ?", id).First(&user)
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
	user := models.UserTable{}
	result := DbEngine.First(&user, id)
	if result.Error != nil {
		tolog.Infof("Error while UpdateUserLoginTime %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	user.LastLogin = time.Now().Add(-time.Hour)
	result = DbEngine.Save(&user)
	if result.Error != nil {
		tolog.Infof("Error while UpdateUserLoginTime %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	return true, nil
}
