package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"painter-server-new/conf"
	"painter-server-new/database"
	"painter-server-new/models"
	"strconv"
)

func GetServerRunningTime(c *gin.Context) {
	timeStamp := conf.Server.FirstInit
	jsTimeStamp, _ := strconv.Atoi(timeStamp)
	jsTimeStamp = jsTimeStamp * 1000
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"TimeStamp": timeStamp, "JSTimeStamp": jsTimeStamp}))
	return
}

func GetServerPreDayVis(c *gin.Context) {
	count, err := database.GetPreDayVisitors()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Count": count}))
	return
}

func GetServerCurrentMonthVis(c *gin.Context) {
	count, err := database.GetMonthlyVisitors()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Count": count}))
	return
}

func SetDebugKeyInCookie(c *gin.Context) {
	key := conf.RandomKey
	c.SetCookie("painter-debug-key", key, 3600*24*30, "/", "", false, true)
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}
