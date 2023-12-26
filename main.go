package main

import (
	"backendQucikStart/api"
	"backendQucikStart/cache"
	conf "backendQucikStart/config"
	"backendQucikStart/database"
)

func main() {
	conf.InitConf()
	cache.InitCache()
	database.InitDB()
	api.InitServer()
}
