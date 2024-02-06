package database

import (
	"painter-server-new/models"
	"painter-server-new/tolog"
	"time"
)

func CreateArticle(title string, author int, summary string, categoryID int, content string, tags []int) (int, error) {
	tx := Dbengine.Begin()

	article := &models.ArticleTable{
		Title:      title,
		Author:     author,
		Summary:    summary,
		CategoryID: categoryID,
		CreatedAt:  time.Now(),
	}
	if err := tx.Create(&article).Error; err != nil {
		tx.Rollback()
		tolog.Log().Infof("Error while CreateArticle %e", err).PrintAndWriteSafe()
		return -1, err
	}

	articleID := article.ArticleID
	articleContent := &models.ArticleContentTable{
		ArticleID: articleID,
		Content:   content,
	}
	if err := tx.Create(&articleContent).Error; err != nil {
		tx.Rollback()
		tolog.Log().Infof("Error while CreateArticle %e", err).PrintAndWriteSafe()
		return -2, err
	}

	for _, tag := range tags {
		articleTag := models.ArticleTagTable{
			ArticleID: articleID,
			TagId:     tag,
		}
		if err := tx.Create(&articleTag).Error; err != nil {
			tx.Rollback()
			tolog.Log().Infof("Error while CreateArticle %e", err).PrintAndWriteSafe()
			return -3, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		tolog.Log().Infof("Error while committing transaction %e", err).PrintAndWriteSafe()
		return -4, err
	}

	return articleID, nil
}

func UpdateArticleContent(articleID int, content string) error {
	var articleContent models.ArticleContentTable
	result := Dbengine.First(&articleContent, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleContent %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	articleContent.Content = content
	result = Dbengine.Save(&articleContent)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleContent %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	err := UpdateArticleUpdateTime(articleID)
	if err != nil {
		tolog.Log().Infof("Error while UpdateArticleContent %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func UpdateArticleSummary(articleID int, summary string) error {
	var article models.ArticleTable
	result := Dbengine.First(&article, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleContent %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.Summary = summary
	result = Dbengine.Save(&article)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleContent %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	err := UpdateArticleUpdateTime(articleID)
	if err != nil {
		tolog.Log().Infof("Error while UpdateArticleContent %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func UpdateArticleReadCount(articleID, count int) error {
	var article models.ArticleTable
	result := Dbengine.First(&article, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleReadCount %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.ReadCount = count

	return nil
}

func UpdateArticleIsTop() {

}

func UpdateArticleTitle() {

}

func UpdateArticleStatus() {

}

func UpdateArticleUpdateTime(articleID int) error {
	var article models.ArticleTable
	result := Dbengine.First(&article, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleUpdateTime %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.UpdatedAt = time.Now()
	result = Dbengine.Save(&article)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleUpdateTime %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func DeleteArticle() {

}

func GetArticlesByAuthor() {

}

func GetArticlesByTitle() {

}

func GetArticlesByContent() {

}
