package conf

import (
	"painter-server-new/tolog"
	"painter-server-new/utils"
	"reflect"
)

const confFilePath = "./conf/conf.json"

// The server struct defines the configuration options for the server.
type server struct {
	Port      string `json:"port"`  // Server port
	Model     string `json:"model"` // Server model
	Name      string `json:"name"`
	Author    string `json:"author"`
	Version   string `json:"version"`
	FirstInit string `json:"firstInit"`
}

// Server is a global variable to store server configuration.
var Server server

// The cacheConf struct defines the configuration options for the cache.
type cacheConf struct {
	Host     string `json:"host"`     // Cache host
	Port     string `json:"port"`     // Cache port
	Password string `json:"password"` // Cache password
	DB       string `json:"DB"`       // Cache database
}

// CacheConf is a global variable to store cache configuration.
var CacheConf cacheConf

type mysqlConf struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Port     string `json:"port"`
	Database string `json:"database"`
	Host     string `json:"host"`
}

var MysqlConf mysqlConf

type Conf struct {
	Server server    `json:"server"`
	Redis  cacheConf `json:"redis"`
	Mysql  mysqlConf `json:"mysql"`
}

var RandomKey string

func NewConf() map[string]interface{} {
	conf := Conf{
		Server: server{
			Author:    "",
			FirstInit: "",
			Model:     "debug",
			Name:      "",
			Port:      "3003",
			Version:   "1.0.0",
		},
		Redis: cacheConf{
			Host:     "localhost",
			Port:     "6379",
			Password: "",
			DB:       "0",
		},
		Mysql: mysqlConf{
			User:     "root",
			Password: "root",
			Port:     "3006",
			Database: "db",
			Host:     "localhost",
		},
	}
	result := make(map[string]interface{})
	v := reflect.ValueOf(conf)
	t := reflect.TypeOf(conf)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i).Interface()
		result[field.Name] = value
	}
	return result
}

func CreateConf() {
	conf := NewConf()
	_, err := utils.JSONWriter(confFilePath, conf)
	if err != nil {
		tolog.Log().Errorf("Error while CreateConf %e", err).PrintAndWriteSafe()
		return
	}
}
