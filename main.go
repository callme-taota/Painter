package main

import (
	"painter-server-new/cache"
	conf "painter-server-new/conf"
	"painter-server-new/database"
	"painter-server-new/server"
)

func main() {
	conf.InitConf()
	cache.InitCache()
	database.InitDB()
	server.InitServer()
}
