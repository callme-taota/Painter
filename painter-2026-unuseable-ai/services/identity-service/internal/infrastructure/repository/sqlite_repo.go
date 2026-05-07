package repository

import (
	"errors"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type User struct {
	UserID      string `gorm:"primaryKey"`
	UserName    string
	Email       string
	NickName    string
	Phone       string
	HeaderField string
	Password    string
	Group       int
}

type Follow struct {
	ID         uint `gorm:"primaryKey"`
	UserID     string
	TargetUser string
}

type MySQLRepo struct {
	db *gorm.DB
}

func NewMySQLRepo(dsn string) (*MySQLRepo, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&User{}, &Follow{}); err != nil {
		return nil, err
	}
	repo := &MySQLRepo{db: db}
	if err := repo.seed(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *MySQLRepo) seed() error {
	var count int64
	if err := r.db.Model(&User{}).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return r.db.Create(&User{
			UserID:      "u-admin",
			UserName:    "admin",
			Email:       "admin@example.com",
			NickName:    "管理员",
			HeaderField: "",
			Password:    "admin",
			Group:       1,
		}).Error
	}
	return nil
}

func (r *MySQLRepo) CreateUser(u User) error {
	return r.db.Create(&u).Error
}

func (r *MySQLRepo) UserExists(userID, userName, email string) (bool, error) {
	var count int64
	q := r.db.Model(&User{})
	if userID != "" {
		q = q.Or("user_id = ?", userID)
	}
	if userName != "" {
		q = q.Or("user_name = ?", userName)
	}
	if email != "" {
		q = q.Or("email = ?", email)
	}
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *MySQLRepo) GetUser(userID string) (User, error) {
	var u User
	err := r.db.Where("user_id = ?", userID).First(&u).Error
	return u, err
}

func (r *MySQLRepo) GetUserByCredential(identity string) (User, error) {
	var u User
	err := r.db.Where("user_name = ? OR email = ? OR user_id = ?", identity, identity, identity).First(&u).Error
	return u, err
}

func (r *MySQLRepo) ListUsers() ([]User, error) {
	var users []User
	err := r.db.Find(&users).Error
	return users, err
}

func (r *MySQLRepo) UpdateUser(userID string, updates map[string]interface{}) error {
	return r.db.Model(&User{}).Where("user_id = ?", userID).Updates(updates).Error
}

func (r *MySQLRepo) SetUserGroup(userID string, group int) error {
	return r.db.Model(&User{}).Where("user_id = ?", userID).Update("group", group).Error
}

func (r *MySQLRepo) CreateFollow(userID, target string) error {
	var f Follow
	err := r.db.Where("user_id = ? AND target_user = ?", userID, target).First(&f).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(&Follow{UserID: userID, TargetUser: target}).Error
}

func (r *MySQLRepo) DeleteFollow(userID, target string) error {
	return r.db.Where("user_id = ? AND target_user = ?", userID, target).Delete(&Follow{}).Error
}

func (r *MySQLRepo) ListFollowings(userID string) ([]Follow, error) {
	var items []Follow
	err := r.db.Where("user_id = ?", userID).Find(&items).Error
	return items, err
}

func (r *MySQLRepo) ListFollowers(userID string) ([]Follow, error) {
	var items []Follow
	err := r.db.Where("target_user = ?", userID).Find(&items).Error
	return items, err
}
