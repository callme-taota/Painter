package repository

import (
	"errors"
	"time"

	"github.com/callme-taota/painter/painter-backend/models"
	"github.com/callme-taota/tolog"
	"gorm.io/gorm"
)

const RuleTableName = "rule"

type Rule struct {
	BaseTable `gorm:"-"`

	ID        int       `gorm:"primaryKey;autoIncrement"`
	Name      string    `gorm:"notNull"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

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

func NewEmptyRule() *Rule {
	return &Rule{
		BaseTable: &BaseTableImplement{},
	}
}

func (r *Rule) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Rule{})
}

func (r *Rule) TableName() string {
	return RuleTableName
}

func (r *Rule) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	rule, ok := row.(*Rule)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(rule), nil
}

func (r *Rule) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	var rules []Rule
	if err := query(db).Find(&rules).Error; err != nil {
		return err
	}

	for i := range rules {
		if err := updater(&rules[i]); err != nil {
			return err
		}
		if err := db.Save(&rules[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Rule) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var rules []Rule
	if err := query(db).Find(&rules).Error; err != nil {
		return err
	}

	for _, rule := range rules {
		if err := db.Delete(&rule).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Rule) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var rules []Rule
	tx := query(db).Find(&rules)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range rules {
		result = append(result, &rules[i])
	}
	return result, tx, nil
}

func InitRules() {
	DbEngine := dbImplement.GetDB()
	for id, name := range ruleMap {
		rule := models.RuleTable{
			ID:   id,
			Name: name,
		}
		if err := DbEngine.FirstOrCreate(&rule, models.RuleTable{ID: id}).Error; err != nil {
			tolog.Infof("Error while init rule %e", err).PrintAndWriteSafe()
			panic(err)
		}
	}

	for id, name := range groupMap {
		group := models.UserGroupTable{
			ID:   id,
			Name: name,
		}
		if err := DbEngine.FirstOrCreate(&group, models.UserGroupTable{ID: id}).Error; err != nil {
			tolog.Infof("Error while init rule %e", err).PrintAndWriteSafe()
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
				tolog.Infof("Error while init rule %e", err).PrintAndWriteSafe()
				panic(err)
			}
		}
	}
}
