package database

import (
	"errors"
	"fmt"
	"painter-server-new/models"
	"painter-server-new/utils"
	"time"
)

func CreateUser(username, email, nickname string, phonenum int, header_field, password string) (int, error) {
	user := models.UserTable{
		UserName:    username,
		Email:       email,
		NickName:    nickname,
		PhoneNum:    phonenum,
		HeaderField: header_field,
		LastLogin:   time.Now(),
	}
	err := Dbengine.Create(&user).Error
	if err != nil {
		return -1, err
	}
	id := user.ID
	password, _ = utils.HashPassword(password)
	userpass := models.UserPassTable{
		ID:       id,
		Password: password,
	}
	err = Dbengine.Create(&userpass).Error
	if err != nil {
		return -2, err
	}
	return user.ID, nil
}

func UpdateUserName(id int, name string) error {
	user := &models.UserTable{}
	res := Dbengine.First(&user, id)
	if res.Error != nil {
		return res.Error
	}
	user.UserName = name
	res = Dbengine.Save(&user)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func UpdateUserEmail(id int, email string) error {
	user := &models.UserTable{}
	res := Dbengine.First(&user, id)
	if res.Error != nil {
		return res.Error
	}
	user.Email = email
	res = Dbengine.Save(&user)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func UpdateUserNickName(id int, nickname string) error {
	user := &models.UserTable{}
	res := Dbengine.First(&user, id)
	if res.Error != nil {
		return res.Error
	}
	user.NickName = nickname
	res = Dbengine.Save(&user)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func UpdateUserPhoneNum(id int, number int) error {
	user := &models.UserTable{}
	res := Dbengine.First(&user, id)
	if res.Error != nil {
		return res.Error
	}
	user.PhoneNum = number
	res = Dbengine.Save(&user)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func UpdateUserHeaderField(id int, headerfield string) error {
	user := &models.UserTable{}
	res := Dbengine.First(&user, id)
	if res.Error != nil {
		return res.Error
	}
	user.HeaderField = headerfield
	res = Dbengine.Save(&user)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func UpdateUserProfile(id int, username, email, nickname string, number int) error {
	user := &models.UserTable{}
	res := Dbengine.First(&user, id)
	if res.Error != nil {
		return res.Error
	}
	user.UserName = username
	user.Email = email
	user.NickName = nickname
	user.PhoneNum = number
	Dbengine.Save(&user)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

func GetUserIdUsingPhoneNum(phone string) (int, error) {
	user := &models.UserTable{}
	err := Dbengine.Where("phone_num = ?", phone).First(&user).Error
	if err != nil {
		return -1, err
	}
	return user.ID, nil
}

func GetUserIdUsingEmail(email string) (int, error) {
	user := &models.UserTable{}
	err := Dbengine.Where("email = ?", email).First(&user).Error
	if err != nil {
		return -1, err
	}
	return user.ID, nil
}

func GetUserIDUsingUserName(username string) (int, error) {
	user := &models.UserTable{}
	err := Dbengine.Where("user_name = ?", username).First(&user).Error
	if err != nil {
		return -1, err
	}
	return user.ID, nil
}

func CheckUserPassword(id int, password string) (bool, error) {
	userpass := &models.UserPassTable{}
	res := Dbengine.Where("ID = ?", id).First(&userpass)
	hashPassword := userpass.Password
	ok := utils.CheckPasswordHash(password, hashPassword)
	if res.RowsAffected == 1 && ok == true {
		return true, nil
	}
	return false, res.Error
}

func ResetPassWord(id int, oldpassword, newpassword string) error {
	userpass := &models.UserPassTable{}
	res := Dbengine.Where(&models.UserPassTable{ID: id, Password: oldpassword}).First(&userpass)
	if res.RowsAffected == 0 {
		return errors.New(fmt.Sprintln(res.RowsAffected, res.Error))
	}
	userpass.Password = newpassword
	Dbengine.Save(userpass)
	return nil
}

func GetUserInfo(id string) (map[string]any, error) {
	user := &models.UserTable{}
	res := Dbengine.First(&user, id)
	if res.Error != nil {
		return nil, res.Error
	}
	var userJson map[string]interface{}
	userJson["Email"] = user.Email
	userJson["PhoneNum"] = user.PhoneNum
	userJson["LastLogin"] = user.LastLogin
	userJson["UserName"] = user.UserName
	userJson["UserNickName"] = user.NickName
	userJson["HeaderField"] = user.HeaderField
	return userJson, nil
}

func GetUserInfoDetail(id string) (map[string]any, error) {
	user := &models.UserTable{}
	res := Dbengine.First(&user, id)
	if res.Error != nil {
		return nil, res.Error
	}

	var art []models.ArticleTable
	res = Dbengine.Where("author = ?", id).Find(&art)
	articleNum := res.RowsAffected
	var artList []map[string]string
	for _, artItem := range art {
		var item map[string]string
		title := artItem.Title
		readCount := artItem.ReadCount
		summary := artItem.Summary
		item["Title"] = title
		item["ReadCount"] = fmt.Sprintln(readCount)
		item["Summary"] = summary
		artList = append(artList, item)
	}

	coll := &models.CollectionTable{}
	res = Dbengine.Where("user_id", id).Find(&coll)
	collectionNum := res.RowsAffected

	//followingNum, _ := GetFollowingNum(id)
	//followerNum, _ := GetFollowerNum(id)

	var userJson map[string]interface{}
	userJson["Email"] = user.Email
	userJson["PhoneNum"] = user.PhoneNum
	userJson["LastLogin"] = user.LastLogin
	userJson["UserName"] = user.UserName
	userJson["UserNickName"] = user.NickName
	userJson["HeaderField"] = user.HeaderField
	userJson["ArticleNum"] = articleNum
	userJson["ArticleList"] = artList
	userJson["CollectionNum"] = collectionNum
	//userJson["FollowingNum"] = followingNum
	//userJson["FollowerNum"] = followerNum

	return userJson, nil
}
