package conf

import (
	"painter-server-new/utils"
	"reflect"

	"github.com/callme-taota/tolog"
)

var confFilePath = utils.ProjectDirRoot() + "/conf/conf.json"

// The server struct defines the configuration options for the server.
type server struct {
	Port      string `json:"port"`  // Server port
	Model     string `json:"model"` // Server model
	Name      string `json:"name"`
	Author    string `json:"author"`
	Version   string `json:"version"`
	FirstInit string `json:"firstInit"`
	Timezone  string `json:"timezone"`
}

// The cacheConf struct defines the configuration options for the cache.
type cacheConf struct {
	Host     string `json:"host"`     // Cache host
	Port     string `json:"port"`     // Cache port
	Password string `json:"password"` // Cache password
	DB       string `json:"DB"`       // Cache database
}

type mysqlConf struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Port     string `json:"port"`
	Database string `json:"database"`
	Host     string `json:"host"`
}

type Config struct {
	Server server    `json:"Server"`
	Redis  cacheConf `json:"Redis"`
	Mysql  mysqlConf `json:"Mysql"`
}

func (c *Config) Config() Config {
	return *c
}

var Conf Config

var RandomKey string

var RunningStatus runningStatus

type runningStatus struct {
	Conf   bool
	DB     bool
	Cache  bool
	Server bool
	Daily  bool
}

func CheckHealth() bool {
	return RunningStatus.DB && RunningStatus.Cache && RunningStatus.Conf && RunningStatus.Server
}

func DefaultConf() map[string]interface{} {
	conf := Config{
		Server: server{
			Author:    "",
			FirstInit: "",
			Model:     "debug",
			Name:      "",
			Port:      "3003",
			Version:   "1.0.0",
			Timezone:  "Asia/Shanghai",
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
	conf := DefaultConf()
	_, err := utils.JSONWriter(confFilePath, conf)
	if err != nil {
		tolog.Errorf("Error while CreateConf %e", err).PrintAndWriteSafe()
		return
	}
}
