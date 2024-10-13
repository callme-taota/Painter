package mid

import (
	"painter-server-new/cache"
	"painter-server-new/models"

	"github.com/callme-taota/tolog"
	"github.com/gin-gonic/gin"
)

func VisitorRecorder() gin.HandlerFunc {
	return func(c *gin.Context) {
		ua := c.GetHeader("User-Agent")
		ip := c.ClientIP()
		vr := models.VisitorRecord{
			UA: ua,
			IP: ip,
		}
		err := cache.AddVisRecord2Set(vr)
		if err != nil {
			tolog.Warningf("Visitor recorder can't write recode into redis %e", err).PrintAndWriteSafe()
		}
		c.Next()
	}
}
