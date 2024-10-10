package main

import (
	"painter-server-new/cache"
	conf "painter-server-new/conf"
	"painter-server-new/daily"
	"painter-server-new/database"
	"painter-server-new/server"
	"time"

	"github.com/callme-taota/tolog"
)

func main() {
	tolog.SetLogPrefix("painter-blog")
	err := conf.InitConf()
	if err != nil {
		conf.RunningStatus.Conf = false
		return
	}

	loc, _ := time.LoadLocation(conf.Server.Timezone)
	tolog.SetLogTimeZone(loc)
	tolog.SetLogTimeFormat(tolog.DateTime)
	tolog.SetLogFileDateFormat(tolog.DateOnly)

	err = cache.InitCache()
	if err != nil {
		conf.RunningStatus.Cache = false
		tolog.Infof("Cache init %s", err).PrintAndWriteSafe()
	}
	err = database.InitDB()
	if err != nil {
		conf.RunningStatus.DB = false
		tolog.Infof("DB init %s", err).PrintAndWriteSafe()
	}
	go func() {
		err := daily.InitDaily()
		if err != nil {
			conf.RunningStatus.Daily = false
			tolog.Infof("Daily init %s", err).PrintAndWriteSafe()
		}
	}()
	err = server.InitServer()
	if err != nil {
		conf.RunningStatus.Server = false
		tolog.Infof("Server init %s", err).PrintAndWriteSafe()
	}
}
