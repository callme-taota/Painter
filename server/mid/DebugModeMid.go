package mid

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"painter-server-new/conf"
	"painter-server-new/models"
)

func DebugModeMid() gin.HandlerFunc {
	return func(c *gin.Context) {
		if conf.Server.Model != "debug" {
			c.Next()
		}
		key := conf.RandomKey
		if c.Request.URL.Path == "/"+key {
			c.Next()
		}
		debugKey, _ := c.Cookie("painter-debug-key")
		if debugKey != key {
			c.JSON(http.StatusForbidden, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
			c.Abort()
		}
		c.Next()
	}
}
