package api

import (
	"net/http"
	"painter-server-new/cache"
	"painter-server-new/conf"
	"painter-server-new/database"
	"painter-server-new/models"
	"strconv"

	"github.com/gin-gonic/gin"
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

func GetSettingsOnLoad(c *gin.Context) {
	github, err := database.GetGithubHref()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	icp, err := database.GetICPCode()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	canReg, err := database.CanRegister()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	siteName, err := database.GetSiteName()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	poster, err := database.GetMailSetting()
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{
		"Github":      github,
		"ICPCode":     icp,
		"CanRegister": canReg,
		"SiteName":    siteName,
		"CanSendMail": poster.Active,
	}))
	return
}

func CheckUserAdmin(c *gin.Context) {
	session, _ := c.Cookie("painter-session")
	if session == "" {
		c.JSON(http.StatusOK, models.R(models.KErrorInvalid, models.KReturnFalse, models.RDC{"isAdmin": false}))
		return
	}
	userid, err := cache.GetUserIDByUserSession(session)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorSessionInvalid, models.KReturnFalse, models.RDC{"isAdmin": false}))
		return
	}
	if userid == "" {
		c.JSON(http.StatusOK, models.R(models.KErrorSessionInvalid, models.KReturnFalse, models.RDC{"isAdmin": false}))
		return
	}
	userID, err := strconv.Atoi(userid)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{"isAdmin": false}))
		return
	}
	adminFlag, err := database.GetAdminFlag(userID)
	if err != nil {
		c.JSON(http.StatusForbidden, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{"isAdmin": false}))
		return
	}
	if !adminFlag {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"isAdmin": false}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"isAdmin": true}))
	return
}

func GetEntryInfo(c *gin.Context) {
	timeStamp := conf.Server.FirstInit
	jsTimeStamp, _ := strconv.Atoi(timeStamp)
	jsTimeStamp = jsTimeStamp * 1000
	preDayCount, err := database.GetPreDayVisitors()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	currMonthCount, err := database.GetMonthlyVisitors()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	articleCount, err := database.GetArticleCount()
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(
		models.KReturnMsgSuccess,
		models.KReturnTrue,
		models.RDC{
			"TimeStamp":         timeStamp,
			"JSTimeStamp":       jsTimeStamp,
			"YesterdayCount":    preDayCount,
			"CurrentMonthCount": currMonthCount,
			"ArticleCount":      articleCount,
		}))
	return
}
