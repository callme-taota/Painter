package conf

import (
	"fmt"
	"os"
	"painter-server-new/tolog"
	"painter-server-new/utils"
)

// The server struct defines the configuration options for the server.
type server struct {
	Port  string // Server port
	Model string // Server model
}

// Server is a global variable to store server configuration.
var Server server

// The cacheConf struct defines the configuration options for the cache.
type cacheConf struct {
	Host     string // Cache host
	Port     string // Cache port
	Password string // Cache password
	DB       string // Cache database
}

// CacheConf is a global variable to store cache configuration.
var CacheConf cacheConf

type mysqlConf struct {
	User     string
	Password string
	Port     string
	Database string
	Host     string
}

var MysqlConf mysqlConf

// InitConf initializes the configuration by reading from a JSON file.
func InitConf() error {
	// Read configuration from the JSON file.
	confJSON, err := utils.JSONReader("./conf/conf.json")
	if err != nil {
		tolog.Log().Error(fmt.Sprintf("jsonReader%e", err)).PrintAndWriteSafe()
		return err
	}

	// Process server configuration.
	serverMap := confJSON["server"]
	server := utils.JSONConvertToMapString(serverMap)
	port := getEnv("SERVER_PORT", server["port"])
	name, version, model := server["name"], server["version"], server["model"]
	Server.Port, Server.Model = port, model

	// Print server configuration information.
	tolog.Log().Info("SendUCode-Server Conf Start").PrintAndWriteSafe()
	tolog.Log().Infof("ServerName:%s", name).PrintAndWriteSafe()
	tolog.Log().Infof("ServerVersion:%s", version).PrintAndWriteSafe()
	tolog.Log().Infof("ServerPort:%s", port).PrintAndWriteSafe()
	tolog.Log().Infof("Running on model:%s", model).PrintAndWriteSafe()

	// Process cache configuration.
	cacheMap := confJSON["redis"]
	cache := utils.JSONConvertToMapString(cacheMap)
	cacheHost := getEnv("REDIS_HOST", cache["host"])
	cachePort := getEnv("REDIS_PORT", cache["port"])
	cachePassword := getEnv("REDIS_PASSWORD", cache["password"])
	cacheDB := getEnv("REDIS_DB", cache["db"])
	CacheConf.Host, CacheConf.Port, CacheConf.Password, CacheConf.DB = cacheHost, cachePort, cachePassword, cacheDB

	//Mysql
	mysqlMap := confJSON["mysql"]
	mysql := utils.JSONConvertToMapString(mysqlMap)
	mysqlUser := getEnv("MYSQL_USER", mysql["user"])
	mysqlPassword := getEnv("MYSQL_PASS", mysql["password"])
	mysqlHost := getEnv("MYSQL_HOST", mysql["host"])
	mysqlPort := getEnv("MYSQL_PORT", mysql["port"])
	mysqlDatabase := getEnv("MYSQL_DB", mysql["database"])
	MysqlConf.User, MysqlConf.Password, MysqlConf.Host, MysqlConf.Port, MysqlConf.Database = mysqlUser, mysqlPassword, mysqlHost, mysqlPort, mysqlDatabase

	return nil
}

// getEnv retrieves the value of an environment variable, using a default value if it doesn't exist.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
