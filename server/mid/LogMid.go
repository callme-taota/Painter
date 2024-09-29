package mid

import (
	"github.com/callme-taota/tolog"
	"github.com/gin-gonic/gin"
)

func LogMid(c *gin.Context) {
	tolog.Infof("Gin RequestMethod: %s, URL Path: %s", c.Request.Method, c.Request.URL.Path).PrintAndWriteSafe()
	c.Next()
}
