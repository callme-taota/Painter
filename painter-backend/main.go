package main

import (
	"time"

	"github.com/callme-taota/painter/painter-backend/cache"
	"github.com/callme-taota/painter/painter-backend/common"
	conf "github.com/callme-taota/painter/painter-backend/conf"
	"github.com/callme-taota/painter/painter-backend/daily"
	"github.com/callme-taota/painter/painter-backend/server"

	"github.com/callme-taota/tolog"
)

func main() {
	tolog.SetLogPrefix("painter-blog")
	err := conf.InitConf()
	if err != nil {
		conf.RunningStatus.Conf = false
		return
	}

	loc, _ := time.LoadLocation(conf.Conf.Server.Timezone)
	tolog.SetLogTimeZone(loc)
	tolog.SetLogTimeFormat(tolog.DateTime)
	tolog.SetLogFileDateFormat(tolog.DateOnly)

	err = common.StartModule()
	if err != nil {
		tolog.Infof("Starting module error: %v", err).PrintAndWriteSafe()
	}

	err = cache.InitCache()
	if err != nil {
		conf.RunningStatus.Cache = false
		tolog.Infof("Cache init %s", err).PrintAndWriteSafe()
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
