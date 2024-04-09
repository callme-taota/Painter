package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"painter-server-new/conf"
	"painter-server-new/models"
)

func GetServerRunningTime(c *gin.Context) {
	timeStamp := conf.Server.FirstInit
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"TimeStamp": timeStamp}))
	return
}
