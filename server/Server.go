package server

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	conf "painter-server-new/conf"
	"painter-server-new/models"
	"painter-server-new/server/mid"
	"painter-server-new/tolog"
)

// Server represents the main Gin engine.
var BaseServer *gin.Engine
var Server *gin.RouterGroup
var LoginGroup *gin.RouterGroup
var AdminGroup *gin.RouterGroup

const StaticWebRoot = "./static/webroot/dist"
const StaticFileRoot = "./static/upload"

// InitServer initializes the main Gin server with CORS configuration.
func InitServer() error {
	// Create a new default Gin server instance.
	mid.SetGinMode()
	ginServer := gin.Default()

	// Configure CORS settings.
	ginServer.Use(mid.CorsMid)
	ginServer.Use(mid.LogMid)
	ginServer.Use(mid.VisitorRecorder())
	ginServer.Use(mid.BetterLogin())
	ginServer.Use(mid.DebugModeMid())

	loginGroup := ginServer.Group("")
	loginGroup.Use(mid.SessionCheckMid())
	LoginGroup = loginGroup

	adminGroup := ginServer.Group("")
	adminGroup.Use(mid.CheckAdminMid())
	AdminGroup = adminGroup

	// Set the global Server variable to the configured Gin server.
	apiServer := ginServer.Group("/api")
	BaseServer = ginServer
	Server = apiServer
	LinkAPI()
	if conf.Server.Model == mid.TestMode {
		ginServer.POST("/test", TestHandler)
	}

	// Log server initialization information.
	tolog.Log().Info("Gin Main Server Start").PrintAndWriteSafe()
	port := conf.Server.Port

	tolog.Log().Infoln("Gin listening on:"+port, "host: http://127.0.0.1:"+port).PrintAndWriteSafe()
	// Run the Gin server on the specified port.
	err := ginServer.Run(fmt.Sprintf(":%s", port))
	if err != nil {
		tolog.Log().Errorf("Gin start in error %e", err).PrintAndWriteSafe()
		return err
	}
	return nil
}

// LinkAPI connects various API routes to the main Gin server.
func LinkAPI() {
	// Link User and Message APIs to the main server.
	LinkUser()
	LinkTag()
	LinkHistory()
	LinkFollow()
	LinkComment()
	LinkCollection()
	LinkCategory()
	LinkArticle()
	LinkCommon()
	LinkFile()
	LinkDebug()
	StaticWeb()
	StaticFiles()
}

func StaticWeb() {
	if conf.Server.Model == "release" {
		BaseServer.Static("", StaticWebRoot)
	}
}

func StaticFiles() {
	if conf.Server.Model == "debug" {
		BaseServer.Static("", StaticWebRoot)
	}
}

func TestHandler(c *gin.Context) {
	//cache.SetContext(c)
	var json TestJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	stringmap := []string{"Name", "Email"}
	stringmap2 := []string{"Name", "axsa"}
	flag1 := models.ShouldCheckJSON(json, stringmap)
	flag2 := models.ShouldCheckJSON(json, stringmap2)
	tolog.Log().Infoln("JSON checking flag1", flag1).PrintLog()
	tolog.Log().Infoln("JSON checking flag2", flag2).PrintLog()
	return
}

type TestJSON struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
