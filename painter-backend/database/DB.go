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

var db *DBImplement

const connTemplate = `%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true`

func init() {
	db = newDBImplement()
	common.Register(db)
}

type DBImplement struct {
	db *gorm.DB

	tables map[string]Table

	conn string
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

func (db *DBImplement) Start() error {
	return nil
}

func (db *DBImplement) Get() DBImplement {
	return *db
}

func (db *DBImplement) GetTransaction() *gorm.DB {
	return db.db.Begin()
}

func (db *DBImplement) AddTable(table Table) {
	if _, ok := db.tables[table.TableName()]; ok {
		return
	}
	db.tables[table.TableName()] = table
}

func (db *DBImplement) Migrate() {
	for _, table := range db.tables {
		table.Migrate(db.db)
	}
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
	Migrate()
	Get() DBImplement
	GetTransaction() *gorm.DB
	AddTable(table Table)
	UseTable(tableName string) Table
}
