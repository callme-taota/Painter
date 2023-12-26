package models

type User_sql struct {
	// the post struct
	Username string `gorm:"primaryKey;autoIncrement"`
	Password string
}
