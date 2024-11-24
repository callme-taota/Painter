package database

import (
	"fmt"
	"log"
	"os"
	"painter-server-new/common"
	conf "painter-server-new/conf"

	"time"

	"github.com/callme-taota/tolog"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DbEngine *gorm.DB

const connTemplate = `%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true`

func init() {
	common.Register(&DBInplement{})
}

type DBInplement struct {
	db *gorm.DB

	tables map[string]Table

	conn string
}

func (db *DBInplement) Register(c conf.Config) (common.Module, error) {
	db.conn = fmt.Sprintf(connTemplate, conf.Conf.Mysql.User, conf.Conf.Mysql.Password, conf.Conf.Mysql.Host, conf.Conf.Mysql.Port, conf.Conf.Mysql.Database)

	configLog := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, // 慢 SQL 阈值
			LogLevel:                  logger.Info, // 日志级别
			IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		},
	)

	config := gorm.Config{}

	if conf.Conf.Server.Model == "debug" {
		config.Logger = configLog
	}

	gormDB, err := gorm.Open(mysql.Open(db.conn), &config)
	if err != nil {
		tolog.Errorf("Mysql init error %e", err).PrintAndWriteSafe()
		return nil, err
	}

	db.db = gormDB

	err = Migrate()
	if err != nil {
		tolog.Errorf("Migrate user table error %e:", err).PrintAndWriteSafe()
		return nil, err
	}
	tolog.Infof("Connect to mysql: Success").PrintAndWriteSafe()
	InitSettings()
	InitRules()
	conf.RunningStatus.DB = true

	return db, nil
}

func (db *DBInplement) Start() error {
	return nil
}

func (db *DBInplement) Get() DBInplement {
	return *db
}

func (db *DBInplement) AddTable(table Table) {
	if _, ok := db.tables[table.TableName()]; ok {
		return
	}
	db.tables[table.TableName()] = table
}

func (db *DBInplement) Migrate() {
	for _, table := range db.tables {
		table.Migrate(db.db)
	}
}

func (db *DBInplement) UseTable(tableName string) Table {
	if table, ok := db.tables[tableName]; ok {
		return table
	}
	return nil
}

func (db *DBInplement) Name() string {
	return common.DB_MODULE
}

type DB interface {
	Start()
	Migrate()
	Get() DBInplement
	UseTable()
}

func InitDB() error {
	dataSourceName := conf.Conf.Mysql.User + ":" + conf.Conf.Mysql.Password + "@tcp(" + conf.Conf.Mysql.Host + ":" + conf.Conf.Mysql.Port + ")/" + conf.Conf.Mysql.Database + "?charset=utf8mb4&parseTime=true"

	configLog := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, // 慢 SQL 阈值
			LogLevel:                  logger.Info, // 日志级别
			IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
		},
	)

	config := gorm.Config{}

	if conf.Conf.Server.Model == "debug" {
		config.Logger = configLog
	}

	db, err := gorm.Open(mysql.Open(dataSourceName), &config)
	if err != nil {
		tolog.Errorf("Mysql init error %e", err).PrintAndWriteSafe()
		return err
	}
	DbEngine = db
	err = Migrate()
	if err != nil {
		tolog.Errorf("Migrate user table error %e:", err).PrintAndWriteSafe()
		return err
	}
	tolog.Infof("Connect to mysql: Success").PrintAndWriteSafe()
	InitSettings()
	InitRules()
	conf.RunningStatus.DB = true
	return nil
}
