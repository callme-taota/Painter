package api

import (
	"net/http"
	"painter-server-new/database"
	"painter-server-new/models"
	"painter-server-new/models/APIs/Request"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetSettings(c *gin.Context) {
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
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{
		"Github":      github,
		"ICPCode":     icp,
		"CanRegister": canReg,
		"SiteName":    siteName,
		"Mail":        poster,
	}))
	return
}

func SetSetting(c *gin.Context) {
	var json Request.SetSettingJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"SiteName", "ICPCode", "Github", "CanRegister", "MailActive", "MailFrom", "MailPassword", "MailSMTPHost", "MailSMTPPort"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.SetSetting(json)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(models.SuccessR())
	return
}

func GetUserList(c *gin.Context) {
	var json models.OnlyPageOption
	json.Limit, _ = strconv.Atoi(c.DefaultQuery("Limit", "20"))
	json.Offset, _ = strconv.Atoi(c.DefaultQuery("Offset", "0"))
	Limit, Offset := json.Limit, json.Offset
	list, err := database.GetUserList(Limit, Offset)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	count, _ := database.GetUserCount()
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{
		"Users":      list,
		"UserNumber": count,
	}))
	return
}

func SetUserGroup(c *gin.Context) {
	var json Request.SetUserGroupJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"UserID", "GroupID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.AssignGroupToUser(json.UserID, json.GroupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(models.SuccessR())
	return
}
