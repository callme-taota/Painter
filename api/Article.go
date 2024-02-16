package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"painter-server-new/database"
	"painter-server-new/models"
	"painter-server-new/models/APIs"
)

func CreateArticle(c *gin.Context) {
	var json APIs.CreateArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Title", "Content"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	id, err := database.CreateArticle(json.Title, userID.(int), json.Summary, json.CategoryID, json.Content, json.Tags)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"ID": id}))
	return
}

func UpdateArticleContent(c *gin.Context) {
	var json APIs.UpdateArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"ArticleID", "Content"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	flag, err := database.CheckArticleAuthor(json.ArticleID, userID.(int))
	if flag == false || err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
		return
	}
	err = database.UpdateArticleContent(json.ArticleID, json.Content)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func UpdateArticleSummary(c *gin.Context) {
	var json APIs.UpdateArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"ArticleID", "Summary"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	flag, err := database.CheckArticleAuthor(json.ArticleID, userID.(int))
	if flag == false || err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
		return
	}
	err = database.UpdateArticleSummary(json.ArticleID, json.Summary)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func UpdateArticleReadCount(c *gin.Context) {
	var json APIs.UpdateArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"ArticleID", "ReadCount"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	flag, err := database.CheckArticleAuthor(json.ArticleID, userID.(int))
	if flag == false || err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
		return
	}
	err = database.UpdateArticleReadCount(json.ArticleID, json.ReadCount)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func UpdateArticleTitle(c *gin.Context) {
	var json APIs.UpdateArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"ArticleID", "Title"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	flag, err := database.CheckArticleAuthor(json.ArticleID, userID.(int))
	if flag == false || err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
		return
	}
	err = database.UpdateArticleTitle(json.ArticleID, json.Title)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func UpdateArticleStatus(c *gin.Context) {
	var json APIs.UpdateArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"ArticleID", "Status"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	flag, err := database.CheckArticleAuthor(json.ArticleID, userID.(int))
	if flag == false || err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
		return
	}
	err = database.UpdateArticleStatus(json.ArticleID, json.Status)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func UpdateArticleIsTop(c *gin.Context) {
	var json APIs.UpdateArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"ArticleID", "IsTop"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	flag, err := database.CheckArticleAuthor(json.ArticleID, userID.(int))
	if flag == false || err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
		return
	}
	err = database.UpdateArticleIsTop(json.ArticleID, json.IsTop)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func DeleteArticle(c *gin.Context) {
	var json APIs.ArticleJSON
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
	err := database.DeleteArticle(json.ArticleID, userID.(int))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func GetArticleByAuthor(c *gin.Context) {
	var json APIs.GetArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Author"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	Limit, Offset := json.Limit, json.Offset
	if Limit == 0 {
		Limit = 20
	}
	list, err := database.GetArticlesByAuthor(json.Author, Limit, Offset)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ArticleList, err := database.GetArticleByIntList(list)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, ArticleList))
	return
}

func GetArticlesByTitle(c *gin.Context) {
	var json APIs.GetArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Title"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	Limit, Offset := json.Limit, json.Offset
	if Limit == 0 {
		Limit = 20
	}
	list, err := database.GetArticlesByTitle(json.Title, Limit, Offset)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ArticleList, err := database.GetArticleByIntList(list)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, ArticleList))
	return
}

func GetArticlesByContent(c *gin.Context) {
	var json APIs.GetArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Content"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	Limit, Offset := json.Limit, json.Offset
	if Limit == 0 {
		Limit = 20
	}
	list, err := database.GetArticlesByContent(json.Content, Limit, Offset)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ArticleList, err := database.GetArticleByIntList(list)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, ArticleList))
	return
}

func GetArticlesByCategory(c *gin.Context) {
	var json APIs.GetArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"CategoryID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	Limit, Offset := json.Limit, json.Offset
	if Limit == 0 {
		Limit = 20
	}
	list, err := database.GetArticlesByCategory(json.CategoryID, Limit, Offset)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ArticleList, err := database.GetArticleByIntList(list)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, ArticleList))
	return
}

func GetArticlesByTag(c *gin.Context) {
	var json APIs.GetArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"TagID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	Limit, Offset := json.Limit, json.Offset
	if Limit == 0 {
		Limit = 20
	}
	list, err := database.GetArticlesByTag(json.TagID, Limit, Offset)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ArticleList, err := database.GetArticleByIntList(list)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, ArticleList))
	return
}

func GetFullArticle(c *gin.Context) {
	var json APIs.ArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"TagID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	fullArticle, err := database.GetFullArticle(json.ArticleID)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, fullArticle))
	return
}

func CreateArticleLike(c *gin.Context) {
	var json APIs.ArticleJSON
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
	err := database.CreateArticleLike(json.ArticleID, userID.(int))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func DeleteArticleLike(c *gin.Context) {
	var json APIs.ArticleJSON
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
	err := database.DeleteArticleLike(json.ArticleID, userID.(int))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func CreateArticleTag(c *gin.Context) {
	var json APIs.ArticleTagJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"ArticleID", "TagID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	flag, err := database.CheckArticleAuthor(json.ArticleID, userID.(int))
	if flag == false || err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
		return
	}
	ok, err = database.CreateArticleTag(json.ArticleID, json.TagID)
	if err != nil || ok != true {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func DeleteArticleTag(c *gin.Context) {
	var json APIs.ArticleTagJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"ArticleID", "TagID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	userID, flag := c.Get("userID")
	if flag == false {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	flag, err := database.CheckArticleAuthor(json.ArticleID, userID.(int))
	if flag == false || err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
		return
	}
	ok, err = database.DeleteArticleTag(json.ArticleID, json.TagID)
	if err != nil || ok != true {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}
