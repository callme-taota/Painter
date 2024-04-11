package mid

import (
	"github.com/gin-gonic/gin"
	"painter-server-new/cache"
	"painter-server-new/models"
	"painter-server-new/tolog"
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
			tolog.Log().Warningf("Visitor recorder can't write recode into redis %e", err).PrintAndWriteSafe()
		}
		c.Next()
	}
}
