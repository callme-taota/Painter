package database

import (
	conf "backendQucikStart/config"
	"backendQucikStart/tolog"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"strings"
)

var (
	user     = conf.MysqlConf.User
	password = conf.MysqlConf.Password
	port     = conf.MysqlConf.Port
	database = conf.MysqlConf.Database
	host     = conf.MysqlConf.Host
)

var Dbengine *gorm.DB

var dataSourceName = strings.Join([]string{user + ":" + password + "@tcp(" + host + ":" + port + ")/" + database}, "")

func InitDB() error {
	db, err := gorm.Open(mysql.Open(dataSourceName), &gorm.Config{})
	if err != nil {
		tolog.Log().Errorf("Mysql init error %e", err)
		return err
	}
	Dbengine = db
	err = Migrate()
	if err != nil {
		tolog.Log().Errorf("migrate user table error %e:", err).PrintAndWriteSafe()
		return err
	}
	return nil
}

func GetDB() *gorm.DB {
	return Dbengine
}

func Migrate() error {
	err := MigrateUserTable()
	if err != nil {
		tolog.Log().Errorf("migrate user table error %e:", err).PrintAndWriteSafe()
		return err
	}
	return nil
}
