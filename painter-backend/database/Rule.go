package database

import (
	"github.com/callme-taota/painter/painter-backend/database/repository"
	"github.com/callme-taota/painter/painter-backend/models"

	"github.com/callme-taota/tolog"
)

func AssignGroupToUser(userID, groupID int) error {
	DbEngine := repository.GetDBImplement().GetDB()
	user := models.UserTable{}
	res := DbEngine.First(&user, userID)
	if res.Error != nil {
		tolog.Infof("Error while AssignGroupToUser %e", res.Error).PrintAndWriteSafe()
		return res.Error
	}
	user.UserGroup = groupID
	res = DbEngine.Save(&user)
	if res.Error != nil {
		tolog.Infof("Error while AssignGroupToUser %e", res.Error).PrintAndWriteSafe()
		return res.Error
	}
	return nil
}

func CheckUsersPermission(userID, ruleID int) (bool, error) {
	DbEngine := repository.GetDBImplement().GetDB()
	user := models.UserTable{}
	res := DbEngine.Select("user_group").First(&user, userID)
	if res.Error != nil {
		tolog.Infof("Error while CheckUsersPermission %e", res.Error).PrintAndWriteSafe()
		return false, res.Error
	}
	groupID := user.UserGroup
	groupRule := models.GroupRuleTable{}
	res = DbEngine.Where("group_id = ? and rule_id = ?", groupID, ruleID).First(&groupRule)
	return res.RowsAffected > 0, nil
}

func CheckUserAdmin(userID int) (bool, error) {
	return CheckUsersPermission(userID, 1)
}

func CanUserArticle(userID int) (bool, error) {
	return CheckUsersPermission(userID, 2)
}

func CanUserComment(userID int) (bool, error) {
	return CheckUsersPermission(userID, 3)
}
