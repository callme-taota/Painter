package database

import (
	"errors"
	"painter-server-new/models"
	"painter-server-new/models/APIs/Response"
	"painter-server-new/tolog"
)

func CreateCategory(name, description string) (int, error) {
	flag := CheckCategoryExist(name)
	if flag == true {
		return -1, errors.New("Category is already exist ")
	}
	category := models.CategoryTable{
		CategoryName: name,
		Description:  description,
	}
	err := Dbengine.Create(&category).Error
	if err != nil {
		tolog.Log().Infof("Error while create category %e", err).PrintAndWriteSafe()
		return -1, err
	}
	id := category.CategoryID
	return id, nil
}

func UpdateCategoryName(id int, name string) error {
	category := &models.CategoryTable{}
	res := Dbengine.First(&category, id)
	if res.Error != nil {
		tolog.Log().Infof("Error while update category name %e", res.Error).PrintAndWriteSafe()
		return res.Error
	}
	category.CategoryName = name
	res = Dbengine.Save(&category)
	if res.Error != nil {
		tolog.Log().Infof("Error while update category name %e", res.Error).PrintAndWriteSafe()
		return res.Error
	}
	return nil
}

func UpdateCategoryNameByName(oldName, newName string) error {
	categoryID, err := GetCategoryID(oldName)
	if err != nil {
		tolog.Log().Infof("Error while UpdateCategoryNameByName %e", err).PrintAndWriteSafe()
		return err
	}
	err = UpdateCategoryName(categoryID, newName)
	if err != nil {
		tolog.Log().Infof("Error while UpdateCategoryNameByName %e", err).PrintAndWriteSafe()
		return err
	}
	return nil
}

func UpdateCategoryDesc(id int, description string) error {
	category := &models.CategoryTable{}
	res := Dbengine.First(&category, id)
	if res.Error != nil {
		tolog.Log().Infof("Error while update category description %e", res.Error).PrintAndWriteSafe()
		return res.Error
	}
	category.Description = description
	res = Dbengine.Save(&category)
	if res.Error != nil {
		tolog.Log().Infof("Error while update category description %e", res.Error).PrintAndWriteSafe()
		return res.Error
	}
	return nil
}

func UpdateCategoryDescByName(name, desc string) error {
	categoryID, err := GetCategoryID(name)
	if err != nil {
		tolog.Log().Infof("Error while UpdateCategoryDescByName %e", err).PrintAndWriteSafe()
		return err
	}
	err = UpdateCategoryDesc(categoryID, desc)
	if err != nil {
		tolog.Log().Infof("Error while UpdateCategoryDescByName %e", err).PrintAndWriteSafe()
		return err
	}
	return nil
}

func CheckCategoryExist(name string) bool {
	var category models.CategoryTable
	result := Dbengine.Where("category_name = ?", name).First(&category)
	return result.RowsAffected > 0
}

func GetCategoryID(name string) (int, error) {
	var category models.CategoryTable
	result := Dbengine.Where("category_name = ?", name).First(&category)
	if result.Error != nil {
		tolog.Log().Infof("Error while get category id %e", result.Error)
		return -1, result.Error
	}
	return category.CategoryID, nil
}

func GetCategory(id int) (models.CategoryTable, error) {
	var category models.CategoryTable
	result := Dbengine.First(&category, id)
	if result.Error != nil {
		return category, result.Error
	}
	return category, nil
}

func GetCategories(limit, offset int) ([]models.CategoryTable, error) {
	var category []models.CategoryTable
	result := Dbengine.Limit(limit).Offset(offset).Find(&category)
	if result.Error != nil {
		return nil, result.Error
	}
	return category, nil
}

func GetCategoriesWithCount(limit, offset int) ([]Response.CategoryWithCount, error) {
	var categories []models.CategoryTable
	var categoriesWithCount []Response.CategoryWithCount

	result := Dbengine.Limit(limit).Offset(offset).Find(&categories)
	if result.Error != nil {
		return nil, result.Error
	}

	for _, category := range categories {
		var count int64
		Dbengine.Model(&models.ArticleTable{}).Where("category_id = ?", category.CategoryID).Count(&count)

		categoryWithCount := Response.CategoryWithCount{
			CategoryTable: category,
			ArticleCount:  int(count),
		}
		categoriesWithCount = append(categoriesWithCount, categoryWithCount)
	}
	return categoriesWithCount, nil
}

func GetCategoriesNumber() int {
	var count int64
	Dbengine.Model(&models.CategoryTable{}).Count(&count)
	return int(count)
}

func UpdateArticleCategory(articleID, categoryID int) error {
	var article models.ArticleTable
	result := Dbengine.First(&article, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while update article category %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.CategoryID = categoryID
	result = Dbengine.Save(&article)
	if result.Error != nil {
		tolog.Log().Infof("Error while update article category %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func DeleteArticleCategory(articleID int) error {
	err := UpdateArticleCategory(articleID, -1)
	if err != nil {
		tolog.Log().Infof("Error while delete article category %e", err).PrintAndWriteSafe()
		return err
	}
	return nil
}

func GetArticleCategoryByArticleID(articleID int) (int, error) {
	var article models.ArticleTable
	result := Dbengine.First(&article, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetArticleCategoryByArticleID %e", result.Error).PrintAndWriteSafe()
		return -1, result.Error
	}
	categoryID := article.CategoryID
	return categoryID, nil
}

func GetArticlesByCategoryID(categoryID, limit, offset int) ([]models.ArticleTable, error) {
	var articles []models.ArticleTable
	result := Dbengine.Limit(limit).Offset(offset).Find(&articles)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetArticlesByCategoryID %e", result.Error).PrintAndWriteSafe()
		return nil, result.Error
	}
	return articles, nil
}
