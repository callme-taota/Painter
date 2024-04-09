package main

import (
	"painter-server-new/cache"
	conf "painter-server-new/conf"
	"painter-server-new/database"
	"painter-server-new/server"
	"painter-server-new/tolog"
)

func main() {
	err := conf.InitConf()
	if err != nil {
		tolog.Log().Errorf("Config init %e", err).PrintAndWriteSafe()
		return
	}
	cache.InitCache()
	database.InitDB()
	server.InitServer()
}
