package conf

import (
	"fmt"
	"painter-server-new/tolog"
	"painter-server-new/utils"
	"strconv"
	"time"
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
		tolog.Log().Infof("Server first init time %s", Server.FirstInit).PrintAndWriteSafe()
		updateConfigFile()
		tolog.Log().Infof("First initialization completed at timestamp: %d", Server.FirstInit).PrintAndWriteSafe()
	}
}

func updateConfigFile() {
	confJSON, err := utils.JSONReader(confFilePath)
	if err != nil {
		tolog.Log().Error(fmt.Sprintf("jsonReader%e", err)).PrintAndWriteSafe()
		return
	}

	serverMap := confJSON["Server"]
	server := utils.JSONConvertToMapString(serverMap)
	server["firstInit"] = fmt.Sprintf("%s", Server.FirstInit)
	confJSON["Server"] = server

	if _, err := utils.JSONWriter(confFilePath, confJSON); err != nil {
		tolog.Log().Error(fmt.Sprintf("jsonWriter%e", err)).PrintAndWriteSafe()
		return
	}
}
