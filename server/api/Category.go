package api

import (
	"net/http"
	"painter-server-new/database"
	"painter-server-new/models"
	"painter-server-new/models/APIs/Request"
	"strconv"

	"github.com/gin-gonic/gin"
)

func CreateCategory(c *gin.Context) {
	var json Request.CategoryJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Name", "Description"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	categoryID, err := database.CreateCategory(json.Name, json.Description)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"CategoryID": categoryID}))
	return
}

func UpdateCategoryName(c *gin.Context) {
	var json Request.UpdateCategoryNameJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"OldName", "NewName"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.UpdateCategoryNameByName(json.OldName, json.NewName)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"OldName": json.OldName, "NewName": json.NewName}))
	return
}

func UpdateCategoryDesc(c *gin.Context) {
	var json Request.CategoryJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Name", "Description"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.UpdateCategoryDescByName(json.Name, json.Description)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Name": json.Name, "Description": json.Description}))
	return
}

func UpdateCategory(c *gin.Context) {
	var json Request.CategoryJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"CategoryID", "Name", "Description"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.UpdateCategory(json.CategoryID, json.Name, json.Description)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func GetCategories(c *gin.Context) {
	var json models.OnlyPageOption
	json.Limit, _ = strconv.Atoi(c.DefaultQuery("Limit", "20"))
	json.Offset, _ = strconv.Atoi(c.DefaultQuery("Offset", "0"))
	Limit, Offset := json.Limit, json.Offset
	categories, err := database.GetCategories(Limit, Offset)
	categoriesNumber := database.GetCategoriesNumber()
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgOK, models.KReturnTrue, models.RDC{"categoriesNumber": categoriesNumber, "categories": categories}))
	return
}

func GetCategory(c *gin.Context) {
	var json Request.CategoryIDJSON
	json.CategoryID, _ = strconv.Atoi(c.Query("ID"))
	category, err := database.GetCategory(json.CategoryID)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgOK, models.KReturnTrue, category))
	return
}

func GetCategoryList(c *gin.Context) {
	categoriesNumber := database.GetCategoriesNumber()
	categories, err := database.GetCategoriesWithCount(categoriesNumber, 0)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"categoriesNumber": categoriesNumber, "categories": categories}))
	return
}
