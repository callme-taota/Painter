package database

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	conf "painter-server-new/conf"
	"painter-server-new/tolog"
)

var Dbengine *gorm.DB

func InitDB() error {
	dataSourceName := conf.MysqlConf.User + ":" + conf.MysqlConf.Password + "@tcp(" + conf.MysqlConf.Host + ":" + conf.MysqlConf.Port + ")/" + conf.MysqlConf.Database
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
