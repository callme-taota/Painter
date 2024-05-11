package database

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	conf "painter-server-new/conf"
	"painter-server-new/tolog"
)

var DbEngine *gorm.DB

func InitDB() error {
	dataSourceName := conf.MysqlConf.User + ":" + conf.MysqlConf.Password + "@tcp(" + conf.MysqlConf.Host + ":" + conf.MysqlConf.Port + ")/" + conf.MysqlConf.Database + "?charset=utf8&parseTime=true"
	db, err := gorm.Open(mysql.Open(dataSourceName), &gorm.Config{})
	if err != nil {
		tolog.Log().Errorf("Mysql init error %e", err).PrintAndWriteSafe()
		return err
	}
	DbEngine = db
	err = Migrate()
	if err != nil {
		tolog.Log().Errorf("Migrate user table error %e:", err).PrintAndWriteSafe()
		return err
	}
	tolog.Log().Infof("Connect to mysql: Success").PrintAndWriteSafe()
	InitSettings()
	return nil
}

func GetDB() *gorm.DB {
	return DbEngine
}
