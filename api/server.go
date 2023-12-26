package api

import (
	conf "backendQucikStart/config"
	"backendQucikStart/tolog"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"time"
)

// Server represents the main Gin engine.
var Server *gin.Engine

// InitServer initializes the main Gin server with CORS configuration.
func InitServer() error {
	// Create a new default Gin server instance.
	ginServer := gin.Default()
	//gin.SetMode(gin.ReleaseMode)

	// Configure CORS settings.
	ginServer.Use(cors.New(cors.Config{ //conf
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "session"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour, //time
		ExposeHeaders:    []string{"Content-Length"},
		AllowOriginFunc: func(origin string) bool { //allow
			return true //all
		},
	}))

	// Set the global Server variable to the configured Gin server.
	Server = ginServer
	LinkAPI()

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
