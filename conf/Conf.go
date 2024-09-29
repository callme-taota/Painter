package conf

import (
	"errors"
	"math/rand"
	"os"

	"painter-server-new/utils"

	"github.com/callme-taota/tolog"
)

// InitConf initializes the configuration by reading from a JSON file.
func InitConf() error {
	if !CheckExist() {
		CreateConf()
		return errors.New("Create conf... ")
	}
	// Read configuration from the JSON file.
	confJSON, err := ConfReader()
	if err != nil {
		tolog.Warningf("Conf read %e", err).PrintAndWriteSafe()
		return err
	}
	Conf2Memory(confJSON)

	if CheckFirstInit() {
		writeFirstInit()
	}
	PrintConfWhileStart()
	if Server.Model == "debug" {
		SetRandomKey()
	}
	RunningStatus.Conf = true

	return nil
}

func ConfReader() (map[string]interface{}, error) {
	confJSON, err := utils.JSONReader(confFilePath)
	if err != nil {
		tolog.Errorf("jsonReader%e", err).PrintAndWriteSafe()
		return nil, err
	}
	return confJSON, nil
}

func Conf2Memory(data map[string]interface{}) {
	// Process server configuration.
	serverMap := data["Server"]
	server := utils.JSONConvertToMapString(serverMap)
	port := getEnv("SERVER_PORT", server["port"])
	model := getEnv("SERVER_MODEL", server["model"])
	name, version, firstInit, author, timezone := server["name"], server["version"], server["firstInit"], server["author"], server["timezone"]
	Server.Port, Server.Model, Server.Name, Server.Version, Server.Author, Server.Timezone = port, model, name, version, author, timezone
	Server.FirstInit = firstInit

	// Process cache configuration.
	cacheMap := data["Redis"]
	cache := utils.JSONConvertToMapString(cacheMap)
	cacheHost := getEnv("REDIS_HOST", cache["host"])
	cachePort := getEnv("REDIS_PORT", cache["port"])
	cachePassword := getEnv("REDIS_PASSWORD", cache["password"])
	cacheDB := getEnv("REDIS_DB", cache["db"])
	CacheConf.Host, CacheConf.Port, CacheConf.Password, CacheConf.DB = cacheHost, cachePort, cachePassword, cacheDB

	//Mysql
	mysqlMap := data["Mysql"]
	mysql := utils.JSONConvertToMapString(mysqlMap)
	mysqlUser := getEnv("MYSQL_USER", mysql["user"])
	mysqlPassword := getEnv("MYSQL_PASS", mysql["password"])
	mysqlHost := getEnv("MYSQL_HOST", mysql["host"])
	mysqlPort := getEnv("MYSQL_PORT", mysql["port"])
	mysqlDatabase := getEnv("MYSQL_DB", mysql["database"])
	MysqlConf.User, MysqlConf.Password, MysqlConf.Host, MysqlConf.Port, MysqlConf.Database = mysqlUser, mysqlPassword, mysqlHost, mysqlPort, mysqlDatabase

}

func SetRandomKey() {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123!@#$%^&*()_+-=[]{};:,./<>?'\"")
	b := make([]rune, 10)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	tolog.Warningf("You are running at debug model! This is your key to access server. Key: %s. Use /%s to make your frontend storage this key. ", string(b), string(b)).PrintAndWriteSafe()
	RandomKey = string(b)
}

func PrintConfWhileStart() {
	// Print server configuration information.
	tolog.Infof("%s Conf Start", Server.Name).PrintAndWriteSafe()
	tolog.Infof("Server version: %s", Server.Version).PrintAndWriteSafe()
	tolog.Infof("Server port: %s", Server.Port).PrintAndWriteSafe()
	tolog.Infof("Running on model: %s", Server.Model).PrintAndWriteSafe()
}

// getEnv retrieves the value of an environment variable, using a default value if it doesn't exist.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
