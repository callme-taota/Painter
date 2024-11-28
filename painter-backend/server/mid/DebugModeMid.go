package mid

import (
	"net/http"

	"github.com/callme-taota/painter/painter-backend/conf"
	"github.com/callme-taota/painter/painter-backend/models"

	"github.com/gin-gonic/gin"
)

func DebugModeMid() gin.HandlerFunc {
	return func(c *gin.Context) {
		if conf.Conf.Server.Model != "debug" {
			c.Next()
			return
		}
		key := conf.RandomKey
		if c.Request.URL.Path == "/"+key {
			c.Next()
			return
		}
		debugKey, _ := c.Cookie("painter-debug-key")
		if debugKey != key {
			c.JSON(http.StatusForbidden, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
			c.Abort()
			return

		}
		c.Next()
		return
	}
}
