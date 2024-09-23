package main

import (
	"painter-server-new/cache"
	conf "painter-server-new/conf"
	"painter-server-new/daily"
	"painter-server-new/database"
	"painter-server-new/server"
	"painter-server-new/tolog"
)

func main() {
	err := conf.InitConf()
	if err != nil {
		conf.RunningStatus.Conf = false
		tolog.Log().Infof("Config init %s", err).PrintAndWriteSafe()
		return
	}
	err = cache.InitCache()
	if err != nil {
		conf.RunningStatus.Cache = false
		tolog.Log().Infof("Cache init %s", err).PrintAndWriteSafe()
	}
	err = database.InitDB()
	if err != nil {
		conf.RunningStatus.DB = false
		tolog.Log().Infof("DB init %s", err).PrintAndWriteSafe()
	}
	go func() {
		err := daily.InitDaily()
		if err != nil {
			conf.RunningStatus.Daily = false
			tolog.Log().Infof("Daily init %s", err).PrintAndWriteSafe()
		}
	}()
	err = server.InitServer()
	if err != nil {
		conf.RunningStatus.Server = false
		tolog.Log().Infof("Server init %s", err).PrintAndWriteSafe()
	}
}
