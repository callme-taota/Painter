package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const UserTableName = "user"

type User struct {
	BaseTable

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

func NewEmptyUser() *User {
	return &User{
		BaseTable: &BaseTableImplement{},
	}
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
