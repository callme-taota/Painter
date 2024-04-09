package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"painter-server-new/database"
	"painter-server-new/models"
	"painter-server-new/models/APIs/Request"
)

// CreateArticle handles the creation of a new article.
func CreateArticle(c *gin.Context) {
	var json Request.CreateArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		// Bad request
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	// Check for required fields
	ok := models.ShouldCheckJSON(json, []string{"Title", "Content"})
	if ok != true {
		// Missing required fields
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	// Get userID from context
	userID, flag := c.Get("userID")
	if flag == false {
		// Error getting userID
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	// Create article in database
	id, err := database.CreateArticle(json.Title, userID.(int), json.Summary, json.CategoryID, json.Content, json.Tags)
	if err != nil {
		// Error creating article
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	// Return success response
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"ID": id}))
	return
}

// UpdateArticleContent updates the content of an existing article.
func UpdateArticleContent(c *gin.Context) {
	var json Request.UpdateArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		// Bad request
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	// Check for required fields
	ok := models.ShouldCheckJSON(json, []string{"ArticleID", "Content"})
	if ok != true {
		// Missing required fields
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	// Get userID from context
	userID, flag := c.Get("userID")
	if flag == false {
		// Error getting userID
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	// Check if user is the author of the article
	flag, err := database.CheckArticleAuthor(json.ArticleID, userID.(int))
	if flag == false || err != nil {
		// Permission denied
		c.JSON(http.StatusOK, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
		return
	}
	// Update article content in database
	err = database.UpdateArticleContent(json.ArticleID, json.Content)
	if err != nil {
		// Error updating article content
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	// Return success response
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

// UpdateArticleSummary updates the summary of an existing article.
func UpdateArticleSummary(c *gin.Context) {
	var json Request.UpdateArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		// Bad request
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	// Check for required fields
	ok := models.ShouldCheckJSON(json, []string{"ArticleID", "Summary"})
	if ok != true {
		// Missing required fields
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	// Get userID from context
	userID, flag := c.Get("userID")
	if flag == false {
		// Error getting userID
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	// Check if user is the author of the article
	flag, err := database.CheckArticleAuthor(json.ArticleID, userID.(int))
	if flag == false || err != nil {
		// Permission denied
		c.JSON(http.StatusOK, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
		return
	}
	// Update article summary in database
	err = database.UpdateArticleSummary(json.ArticleID, json.Summary)
	if err != nil {
		// Error updating article summary
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	// Return success response
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

// UpdateArticleReadCount updates the read count of an existing article.
func UpdateArticleReadCount(c *gin.Context) {
	var json Request.UpdateArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		// Bad request
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	// Check for required fields
	ok := models.ShouldCheckJSON(json, []string{"ArticleID", "ReadCount"})
	if ok != true {
		// Missing required fields
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	// Get userID from context
	userID, flag := c.Get("userID")
	if flag == false {
		// Error getting userID
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	// Check if user is the author of the article
	flag, err := database.CheckArticleAuthor(json.ArticleID, userID.(int))
	if flag == false || err != nil {
		// Permission denied
		c.JSON(http.StatusOK, models.R(models.KErrorPermissionDenied, models.KReturnFalse, models.RDC{}))
		return
	}
	// Update article read count in database
	err = database.UpdateArticleReadCount(json.ArticleID, json.ReadCount)
	if err != nil {
		// Error updating article read count
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	// Return success response
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func UpdateArticleTitle(c *gin.Context) {
	var json Request.UpdateArticleJSON
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
	var json Request.UpdateArticleJSON
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
	var json Request.UpdateArticleJSON
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
	var json Request.ArticleJSON
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
	var json Request.GetArticleJSON
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
	count, err := database.GetArticleCountByAuthor(json.Author)
	ArticleList, err := database.GetArticleByIntList(list)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"ArticleCount": count, "ArticleList": ArticleList}))
	return
}

func GetArticleSelf(c *gin.Context) {
	var json models.OnlyPageOption
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	Limit, Offset := json.Limit, json.Offset
	if Limit == 0 {
		Limit = 20
	}
	userid, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusBadRequest, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{}))
		return
	}
	list, err := database.GetArticlesByAuthor(userid.(int), Limit, Offset)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	count, err := database.GetArticleCountByAuthor(userid.(int))
	ArticleList, err := database.GetArticleByIntList(list)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"ArticleCount": count, "ArticleList": ArticleList}))
	return
}

func GetArticlesByTitle(c *gin.Context) {
	var json Request.GetArticleJSON
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
	var json Request.GetArticleJSON
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
	var json Request.GetArticleJSON
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
	total, err := database.GetArticleCountByCategory(json.CategoryID)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ArticleList, err := database.GetArticleByIntList(list)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"ArticleList": ArticleList, "ArticleCount": total}))
	return
}

func GetArticlesByCollection(c *gin.Context) {
	var json Request.GetArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"UserID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	Limit, Offset := json.Limit, json.Offset
	if Limit == 0 {
		Limit = 20
	}
	list, err := database.GetArticlesByCollection(json.UserID, Limit, Offset)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	total, err := database.GetCollectionCountByUser(json.CategoryID)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ArticleList, err := database.GetArticleByIntList(list)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"ArticleList": ArticleList, "ArticleCount": total}))
	return
}

func GetArticlesByTag(c *gin.Context) {
	var json Request.GetArticleJSON
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
	count, err := database.GetArticlesCountByTag(json.TagID)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ArticleList, err := database.GetArticleByIntList(list)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"ArticleList": ArticleList, "ArticleCount": count}))
	return
}

func GetArticlesByTime(c *gin.Context) {
	var json Request.GetArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	Limit, Offset := json.Limit, json.Offset
	if Limit == 0 {
		Limit = 20
	}
	list, err := database.GetArticleIDsByTime(Limit, Offset)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	count, err := database.GetArticleCount()
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ArticleList, err := database.GetArticleByIntList(list)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"ArticleList": ArticleList, "ArticleCount": count}))
	return
}

func GetArticlesCount(c *gin.Context) {
	var json Request.GetArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	count, err := database.GetArticleCount()
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"ArticleCount": count}))
	return
}

func GetFullArticle(c *gin.Context) {
	var json Request.ArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"ArticleID"})
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
	var json Request.ArticleJSON
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
	var json Request.ArticleJSON
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

func ArticleLike(c *gin.Context) {
	var json Request.ArticleJSON
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
	err := database.ToggleArticleLike(json.ArticleID, userID.(int))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func CheckArticleLike(c *gin.Context) {
	var json Request.ArticleJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"err": err}))
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
	isliked, err := database.HasLikedArticle(json.ArticleID, userID.(int))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"IsLiked": isliked}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"IsLiked": isliked}))
	return
}

func CreateArticleTag(c *gin.Context) {
	var json Request.ArticleTagJSON
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
	var json Request.ArticleTagJSON
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
