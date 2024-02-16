package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"painter-server-new/database"
	"painter-server-new/models"
	"painter-server-new/models/APIs"
)

func CreateHistory(c *gin.Context) {
	var json APIs.CreateHistoryJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"ArticleID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	id, err := database.AutoHistory(userID.(int), json.ArticleID)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"ArticleID": json.ArticleID, "UserID": userID, "ID": id}))
	return
}

func GetHistories(c *gin.Context) {
	var json models.OnlyPageOption
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Limit", "Offset"})
	Limit, Offset := json.Limit, json.Offset
	if ok != true {
		Limit, Offset = 20, 0
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	histories, err := database.GetUserHistories(userID.(int), Limit, Offset)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, histories))
	return
}
