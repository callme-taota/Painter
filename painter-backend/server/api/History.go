package api

import (
	"net/http"
	"strconv"

	"github.com/callme-taota/painter/painter-backend/database"
	"github.com/callme-taota/painter/painter-backend/models"
	"github.com/callme-taota/painter/painter-backend/models/APIs/Request"

	"github.com/gin-gonic/gin"
)

func CreateHistory(c *gin.Context) {
	var json Request.CreateHistoryJSON
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
	success, err := database.AutoHistory(userID.(int), json.ArticleID)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"ArticleID": json.ArticleID, "UserID": userID, "Success": success}))
	return
}

func GetHistories(c *gin.Context) {
	var json models.OnlyPageOption
	json.Limit, _ = strconv.Atoi(c.DefaultQuery("Limit", "20"))
	json.Offset, _ = strconv.Atoi(c.DefaultQuery("Offset", "0"))
	Limit, Offset := json.Limit, json.Offset
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
