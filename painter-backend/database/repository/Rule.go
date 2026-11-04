package repository

import (
	"errors"
	"time"

	"github.com/callme-taota/painter/painter-backend/models"
	"github.com/callme-taota/tolog"
	"gorm.io/gorm"
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

const RuleTableName = "rule"

type Rule struct {
	BaseTable `gorm:"-"`

	ID        int       `gorm:"primaryKey;autoIncrement"`
	Name      string    `gorm:"notNull"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
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
	return updateRows(r, db, query, updater)
}

func (r *Rule) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	return deleteRows[Rule](db, query)
}

func (r *Rule) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var rules []Rule
	tx := query(db).Find(&rules)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range rules {
		rules[i].BaseTable = &BaseTableImplement{}
		result = append(result, &rules[i])
	}
	return result, tx, nil
}

func (r *Rule) Constructor() Table {
	return NewEmptyRule()
}

const GroupTableName = "user_group"

type Group struct {
	BaseTable `gorm:"-"`

	ID        int       `gorm:"primaryKey;autoIncrement"`
	Name      string    `gorm:"notNull"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func NewEmptyGroup() *Group {
	return &Group{
		BaseTable: &BaseTableImplement{},
	}
}

func (g *Group) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Group{})
}

func (g *Group) TableName() string {
	return GroupTableName
}

func (g *Group) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	group, ok := row.(*Group)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(group), nil
}

func (g *Group) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	return updateRows(g, db, query, updater)
}

func (g *Group) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	return deleteRows[Group](db, query)
}

func (g *Group) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var groups []Group
	tx := query(db).Find(&groups)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range groups {
		groups[i].BaseTable = &BaseTableImplement{}
		result = append(result, &groups[i])
	}
	return result, tx, nil
}

func (g *Group) Constructor() Table {
	return NewEmptyGroup()
}

const GroupRuleTableName = "group_rule"

type GroupRule struct {
	BaseTable `gorm:"-"`

	GroupID int
	RuleID  int
}

func NewEmptyGroupRule() *GroupRule {
	return &GroupRule{
		BaseTable: &BaseTableImplement{},
	}
}

func (g *GroupRule) Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&GroupRule{})
}

func (g *GroupRule) TableName() string {
	return GroupRuleTableName
}

func (g *GroupRule) Create(db *gorm.DB, row Table) (*gorm.DB, error) {
	groupRule, ok := row.(*GroupRule)
	if !ok {
		return nil, errors.New("invalid row type")
	}
	return db.Create(groupRule), nil
}

func (g *GroupRule) Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	return updateRows(g, db, query, updater)
}

func (g *GroupRule) Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	return deleteRows[GroupRule](db, query)
}

func (g *GroupRule) Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error) {
	var groupRules []GroupRule
	tx := query(db).Find(&groupRules)
	if tx.Error != nil {
		return nil, tx, tx.Error
	}

	var result []Table
	for i := range groupRules {
		groupRules[i].BaseTable = &BaseTableImplement{}
		result = append(result, &groupRules[i])
	}
	return result, tx, nil
}

func (g *GroupRule) Constructor() Table {
	return NewEmptyGroupRule()
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
