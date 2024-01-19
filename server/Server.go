package server

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"painter-server-new/cache"
	conf "painter-server-new/conf"
	"painter-server-new/tolog"
)

// Server represents the main Gin engine.
var Server *gin.Engine

// InitServer initializes the main Gin server with CORS configuration.
func InitServer() error {
	// Create a new default Gin server instance.
	ginServer := gin.Default()
	SetGinMode()

	// Configure CORS settings.
	ginServer.Use(CorsMid)

	// Set the global Server variable to the configured Gin server.
	Server = ginServer
	LinkAPI()
	ginServer.GET("/test", TestHandler)

	// Log server initialization information.
	tolog.Log().Info("Gin Main Server Start").PrintAndWriteSafe()
	port := conf.Server.Port

	tolog.Log().Infoln("Gin listening on:"+port, "host: http://127.0.0.1:"+port).PrintAndWriteSafe()
	// Run the Gin server on the specified port.
	err := ginServer.Run(fmt.Sprintf(":%s", port))
	if err != nil {
		tolog.Log().Errorf("Gin start in error %e", err)
		return err
	}
	return nil
}

// LinkAPI connects various API routes to the main Gin server.
func LinkAPI() {
	// Link User and Message APIs to the main server.
	LinkUser()
}

func TestHandler(c *gin.Context) {
	cache.SetContext(c)
	return
}
