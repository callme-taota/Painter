package api

import (
	"net/http"
	"painter-server-new/database"
	"painter-server-new/models"
	"painter-server-new/models/APIs/Request"
	"strconv"

	"github.com/gin-gonic/gin"
)

const maxCommentLength = 100

func CreateComment(c *gin.Context) {
	var json Request.CreateCommentJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"ArticleID", "Content"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	if len(json.Content) > maxCommentLength {
		c.JSON(http.StatusOK, models.R(models.KErrorInvalid, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	check, err := database.CanUserComment(userID.(int))
	if err != nil || !check {
		c.JSON(http.StatusOK, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
		return
	}
	commentID, err := database.CreateComment(json.ArticleID, userID.(int), json.Content)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"UserID": userID, "CommentID": commentID, "ArticleID": json.ArticleID}))
	return
}

func DeleteComment(c *gin.Context) {
	var json Request.CommentJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"CommentID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	flag, err := database.DeleteComment(userID.(int), json.CommentID)
	if err != nil || flag == true {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"UserID": userID, "CommentID": json.CommentID}))
	return
}

func CreateCommentLike(c *gin.Context) {
	var json Request.CommentJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"CommentID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	flag, err := database.CreateCommentLike(json.CommentID, userID.(int))
	if err != nil || flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"UserID": userID, "CommentID": json.CommentID}))
	return
}

func DeleteCommentLike(c *gin.Context) {
	var json Request.CommentJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"CommentID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	flag, err := database.DeleteCommentLike(json.CommentID, userID.(int))
	if err != nil || flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"UserID": userID, "CommentID": json.CommentID}))
	return
}

func GetCommentsByArticleID(c *gin.Context) {
	var json Request.GetCommentJSON
	json.ArticleID, _ = strconv.Atoi(c.DefaultQuery("ArticleID", ""))
	json.Limit, _ = strconv.Atoi(c.DefaultQuery("Limit", ""))
	json.Offset, _ = strconv.Atoi(c.DefaultQuery("Offset", ""))
	ok := models.ShouldCheckJSON(json, []string{"ArticleID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	Limit, Offset := json.Limit, json.Offset
	if Limit == 0 {
		Limit = 20
	}
	count, err := database.GetCommentCountByArticleID(json.ArticleID)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	isLogin, _ := c.Get("isLogin")
	if isLogin.(bool) {
		userID, _ := c.Get("userID")
		list, err := database.GetCommentsWithLikeInfoByArticleID(json.ArticleID, Limit, Offset, userID.(int))
		if err != nil {
			c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
			return
		}
		c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Comments": list, "CommentCount": count}))
		return
	}
	list, err := database.GetCommentByArticleID(json.ArticleID, Limit, Offset)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Comments": list, "CommentCount": count}))
	return
}

func GetCommentsByArticleIDWithLiked(c *gin.Context) {
	var json Request.GetCommentJSON
	json.ArticleID, _ = strconv.Atoi(c.DefaultQuery("ArticleID", ""))
	json.Limit, _ = strconv.Atoi(c.DefaultQuery("Limit", ""))
	json.Offset, _ = strconv.Atoi(c.DefaultQuery("Offset", ""))
	ok := models.ShouldCheckJSON(json, []string{"ArticleID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	Limit, Offset := json.Limit, json.Offset
	if Limit == 0 {
		Limit = 20
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	list, err := database.GetCommentsWithLikeInfoByArticleID(json.ArticleID, Limit, Offset, userID.(int))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Comments": list}))
	return
}
