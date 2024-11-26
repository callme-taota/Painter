package repository

import (
	"errors"
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

var db *DBImplement

const connTemplate = `%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true`

func init() {
	db = newDBImplement()
	db.AddTable(NewEmptyUser())
	db.AddTable(NewEmptyUserPassword())
	db.AddTable(NewEmptyVisitorRecord())
	common.Register(db)
}

type DBImplement struct {
	db       *gorm.DB
	dbConfig gorm.Config

	tables map[string]Table

	conn   string
	Config conf.Config
}

func newDBImplement() *DBImplement {
	return &DBImplement{}
}

func (db *DBImplement) Register(c conf.Config) (common.Module, error) {
	db.conn = fmt.Sprintf(connTemplate, c.Mysql.User, c.Mysql.Password, c.Mysql.Host, c.Mysql.Port, c.Mysql.Database)

	config := gorm.Config{}
	if c.Server.Model == "debug" {
		config.Logger = logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
			logger.Config{
				SlowThreshold:             time.Second, // 慢 SQL 阈值
				LogLevel:                  logger.Info, // 日志级别
				IgnoreRecordNotFoundError: true,        // 忽略ErrRecordNotFound（记录未找到）错误
			},
		)
	}

	db.dbConfig = config
	db.config = c

	return db, nil
}

func (db *DBImplement) Start() error {
	gormDB, err := gorm.Open(mysql.Open(db.conn), &db.dbConfig)
	if err != nil {
		tolog.Errorf("Mysql init error %e", err).PrintAndWriteSafe()
		return err
	}

	db.db = gormDB

	err = db.Migrate()
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

func (db *DBImplement) Get() DBImplement {
	return *db
}

func (db *DBImplement) GetTransaction() *gorm.DB {
	return db.db.Begin()
}

func (db *DBImplement) GetDB() *gorm.DB {
	return db.db
}

func (db *DBImplement) AddTable(table Table) {
	if _, ok := db.tables[table.TableName()]; ok {
		return
	}
	db.tables[table.TableName()] = table
}

func (db *DBImplement) Migrate() error {
	errs := errors.Join()
	for _, table := range db.tables {
		err := table.Migrate(db.db)
		errs = errors.Join(errs, err)
	}
	return errs
}

func (db *DBImplement) UseTable(tableName string) Table {
	if table, ok := db.tables[tableName]; ok {
		return table
	}
	return nil
}

func (db *DBImplement) Name() string {
	return common.DB_MODULE
}

type DB interface {
	Start() error
	Migrate() error
	Get() DBImplement
	GetTransaction() *gorm.DB
	GetDB() *gorm.DB
	AddTable(table Table)
	UseTable(tableName string) Table
}

func GetDB() *DBImplement {
	return db
}
