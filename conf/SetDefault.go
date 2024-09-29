package conf

import (
	"fmt"

	"painter-server-new/utils"
	"strconv"
	"time"

	"github.com/callme-taota/tolog"
)

func CheckExist() bool {
	return utils.CheckJSONFileExist(confFilePath)
}

func CheckFirstInit() bool {
	return Server.FirstInit == "0" || Server.FirstInit == ""
}

func writeFirstInit() {
	if Server.FirstInit == "0" || Server.FirstInit == "" {
		timestamp := time.Now().Unix()
		Server.FirstInit = strconv.Itoa(int(timestamp))
		tolog.Infof("Server first init time %s", Server.FirstInit).PrintAndWriteSafe()
		updateConfigFile()
		tolog.Infof("First initialization completed at timestamp: %d", Server.FirstInit).PrintAndWriteSafe()
	}
}

func updateConfigFile() {
	confJSON, err := utils.JSONReader(confFilePath)
	if err != nil {
		tolog.Error(fmt.Sprintf("jsonReader%e", err)).PrintAndWriteSafe()
		return
	}

	serverMap := confJSON["Server"]
	server := utils.JSONConvertToMapString(serverMap)
	server["firstInit"] = fmt.Sprintf("%s", Server.FirstInit)
	confJSON["Server"] = server

	if _, err := utils.JSONWriter(confFilePath, confJSON); err != nil {
		tolog.Error(fmt.Sprintf("jsonWriter%e", err)).PrintAndWriteSafe()
		return
	}
}
