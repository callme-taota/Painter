package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"painter-server-new/database"
	"painter-server-new/models"
	"painter-server-new/models/APIs/Request"
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
	c.JSON(http.StatusOK, models.SuccessR())
	return
}
