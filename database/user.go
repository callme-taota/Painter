package database

import (
	"backendQucikStart/models"
)

func MigrateUserTable() error {
	db := GetDB()
	err := db.AutoMigrate(&models.User_sql{})
	if err != nil {
		return err
	}
	return nil
}

func SetUser(username, password string) (int, error) {
	db := GetDB()
	user := models.User_sql{
		Username: username,
		Password: password,
	}
	err := db.Create(&user).Error
	if err != nil {
		return -1, err
	}
	return 1, nil
}
