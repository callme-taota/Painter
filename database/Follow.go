package database

import (
	"painter-server-new/models"
	"painter-server-new/models/APIs/Response"
	"painter-server-new/tolog"
	"time"
)

func CreateFollow(followerID, followingID int) error {
	follow := models.FollowTable{
		FollowerID:  followerID,
		FollowingID: followingID,
		FollowTime:  time.Now(),
	}

	err := DbEngine.Create(&follow).Error
	if err != nil {
		tolog.Log().Infof("Error while create follow %e", err)
		return err
	}
	return nil
}

func DeleteFollow(followerID, followingID int) error {
	err := DbEngine.Where("follower_id = ? AND following_id = ?", followerID, followingID).Delete(&models.FollowTable{}).Error
	if err != nil {
		tolog.Log().Infof("Error while delete follow %e", err)
		return err
	}
	return nil
}

// GetFollowers means if a follow b, A is b's follower(fans).
func GetFollowers(userID, limit, offset int) ([]models.FollowTable, error) {
	var followers []models.FollowTable
	result := DbEngine.Where("following_id = ?", userID).Limit(limit).Offset(offset).Find(&followers)
	if result.Error != nil {
		return nil, result.Error
	}
	return followers, nil
}

func GetFollowerUsers(followings []models.FollowTable) ([]Response.FollowUserInfo, error) {
	var users []Response.FollowUserInfo
	for _, following := range followings {
		var user Response.FollowUserInfo
		result := DbEngine.Model(&models.UserTable{}).
			Select("id, email, nick_name, header_field, created_at, last_login").
			Where("id = ?", following.FollowingID).
			First(&user)
		if result.Error != nil {
			return nil, result.Error
		}
		user.Following, _ = CheckFollow(following.FollowerID, following.FollowingID)
		users = append(users, user)
	}
	return users, nil
}

// GetFollowings means if a follow b, b is a's followed people
func GetFollowings(userID, limit, offset int) ([]models.FollowTable, error) {
	var followings []models.FollowTable
	result := DbEngine.Where("follower_id = ?", userID).Limit(limit).Offset(offset).Find(&followings)
	if result.Error != nil {
		return nil, result.Error
	}
	return followings, nil
}

func GetFollowingsUsers(followings []models.FollowTable) ([]Response.FollowUserInfo, error) {
	var users []Response.FollowUserInfo
	for _, following := range followings {
		var user Response.FollowUserInfo
		result := DbEngine.Model(&models.UserTable{}).
			Select("id, email, nick_name, header_field, created_at, last_login").
			Where("id = ?", following.FollowerID).
			First(&user)
		if result.Error != nil {
			return nil, result.Error
		}
		user.Following, _ = CheckFollow(following.FollowingID, following.FollowerID)
		users = append(users, user)
	}
	return users, nil
}

func GetFollowerNumber(userID int) (int, error) {
	var count int64
	result := DbEngine.Model(&models.FollowTable{}).Where("following_id = ?", userID).Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(count), nil
}

func GetFollowingNumber(userID int) (int, error) {
	var count int64
	result := DbEngine.Model(&models.FollowTable{}).Where("follower_id = ?", userID).Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(count), nil
}

// CheckFollow check if b follow a
func CheckFollow(a, b int) (bool, error) {
	var count int64
	result := DbEngine.Model(&models.FollowTable{}).Where("follower_id = ? AND following_id = ?", b, a).Count(&count)
	if result.Error != nil {
		return false, result.Error
	}
	return count > 0, nil
}
