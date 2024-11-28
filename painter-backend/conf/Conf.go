package conf

import (
	"encoding/json"
	"errors"
	"math/rand"
	"os"

	"github.com/callme-taota/tolog"
)

// InitConf initializes the configuration by reading from a JSON file.
func InitConf() error {
	// Check config exist or create
	if !CheckExist() {
		CreateConf()
		return errors.New("Create conf... ")
	}

	// Read configuration from the JSON file.
	confJSON, err := readConfig()
	if err != nil {
		tolog.Warningf("Conf read %e", err).PrintAndWriteSafe()
		return err
	}
	CacheConfig(confJSON)

	writeFirstInit()

	PrintConfWhileStart()
	if Conf.Server.Model == "debug" {
		SetRandomKey()
	}
	RunningStatus.Conf = true

	return nil
}

func readConfig() (*Config, error) {
	file, err := os.Open(confFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config *Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		tolog.Errorf("Error decoding JSON: %v", err).PrintAndWriteSafe()
		return nil, err
	}
	return config, err
}

func CacheConfig(data *Config) {
	Conf = *data
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
	tolog.Infof("%s Conf Start", Conf.Server.Name).PrintAndWriteSafe()
	tolog.Infof("Server version: %s", Conf.Server.Version).PrintAndWriteSafe()
	tolog.Infof("Server port: %s", Conf.Server.Port).PrintAndWriteSafe()
	tolog.Infof("Running on model: %s", Conf.Server.Model).PrintAndWriteSafe()
}

// getEnv retrieves the value of an environment variable, using a default value if it doesn't exist.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
