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

func writeFirstInit() {
	if Conf.Server.FirstInit == "0" || Conf.Server.FirstInit == "" {
		timestamp := time.Now().Unix()
		Conf.Server.FirstInit = strconv.Itoa(int(timestamp))
		tolog.Infof("Server first init time %s", Conf.Server.FirstInit).PrintAndWriteSafe()
		updateConfigFile()
		tolog.Infof("First initialization completed at timestamp: %s", Conf.Server.FirstInit).PrintAndWriteSafe()
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
	server["firstInit"] = fmt.Sprintf("%s", Conf.Server.FirstInit)
	confJSON["Server"] = server

	if _, err := utils.JSONWriter(confFilePath, confJSON); err != nil {
		tolog.Error(fmt.Sprintf("jsonWriter%e", err)).PrintAndWriteSafe()
		return
	}
}
