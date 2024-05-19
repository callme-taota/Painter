package database

import (
	"painter-server-new/models"
	"painter-server-new/tolog"
)

var ruleMap = map[int]string{
	1: "管理权限",
	2: "文章权限",
	3: "评论权限",
}

var groupMap = map[int]string{
	1: "管理员",
	2: "普通用户",
	3: "限制用户",
}

var groupRuleMap = map[int][]int{
	1: {1, 2, 3},
	2: {2, 3},
	3: {3},
}

func InitRules() {
	for id, name := range ruleMap {
		rule := models.RuleTable{
			ID:   id,
			Name: name,
		}
		if err := DbEngine.FirstOrCreate(&rule, models.RuleTable{ID: id}).Error; err != nil {
			tolog.Log().Infof("Error while init rule %e", err).PrintAndWriteSafe()
			panic(err)
		}
	}

	for id, name := range groupMap {
		group := models.UserGroupTable{
			ID:   id,
			Name: name,
		}
		if err := DbEngine.FirstOrCreate(&group, models.UserGroupTable{ID: id}).Error; err != nil {
			tolog.Log().Infof("Error while init rule %e", err).PrintAndWriteSafe()
			panic(err)
		}
	}

	for groupID, rules := range groupRuleMap {
		for _, ruleID := range rules {
			groupRule := models.GroupRuleTable{
				GroupID: groupID,
				RuleID:  ruleID,
			}
			if err := DbEngine.FirstOrCreate(&groupRule, models.GroupRuleTable{GroupID: groupID, RuleID: ruleID}).Error; err != nil {
				tolog.Log().Infof("Error while init rule %e", err).PrintAndWriteSafe()
				panic(err)
			}
		}
	}
}

func AssignGroupToUser(userID, groupID int) error {
	user := models.UserTable{}
	res := DbEngine.First(&user, userID)
	if res.Error != nil {
		tolog.Log().Infof("Error while AssignGroupToUser %e", res.Error).PrintAndWriteSafe()
		return res.Error
	}
	user.UserGroup = groupID
	res = DbEngine.Save(&user)
	if res.Error != nil {
		tolog.Log().Infof("Error while AssignGroupToUser %e", res.Error).PrintAndWriteSafe()
		return res.Error
	}
	return nil
}

func CheckUsersPermission(userID, ruleID int) (bool, error) {
	user := models.UserTable{}
	res := DbEngine.Select("user_group").First(&user, userID)
	if res.Error != nil {
		tolog.Log().Infof("Error while CheckUsersPermission %e", res.Error).PrintAndWriteSafe()
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
