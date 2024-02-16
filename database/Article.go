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
			TagID:     tag,
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
		tolog.Log().Infof("Error while UpdateArticleSummary %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.Summary = summary
	result = Dbengine.Save(&article)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleSummary %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	err := UpdateArticleUpdateTime(articleID)
	if err != nil {
		tolog.Log().Infof("Error while UpdateArticleSummary %e", result.Error).PrintAndWriteSafe()
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
	err := UpdateArticleUpdateTime(articleID)
	if err != nil {
		tolog.Log().Infof("Error while UpdateArticleReadCount %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func UpdateArticleIsTop(articleID int, isTop bool) error {
	var article models.ArticleTable
	result := Dbengine.First(&article, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleIsTop %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.IsTop = isTop
	err := UpdateArticleUpdateTime(articleID)
	if err != nil {
		tolog.Log().Infof("Error while UpdateArticleIsTop %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func UpdateArticleTitle(articleID int, title string) error {
	var article models.ArticleTable
	result := Dbengine.First(&article, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleTitle %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.Title = title
	err := UpdateArticleUpdateTime(articleID)
	if err != nil {
		tolog.Log().Infof("Error while UpdateArticleTitle %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func UpdateArticleStatus(articleID int, status int) error {
	var article models.ArticleTable
	result := Dbengine.First(&article, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleStatus %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.Status = status
	err := UpdateArticleUpdateTime(articleID)
	if err != nil {
		tolog.Log().Infof("Error while UpdateArticleStatus %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
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

func DeleteArticle(articleID, author int) error {
	err := Dbengine.Where("article_id = ? and author = ?", articleID, author).Delete(&models.ArticleTable{}).Error
	if err != nil {
		tolog.Log().Infof("Error while delete article %e", err).PrintAndWriteSafe()
		return err
	}
	return nil
}

func GetArticlesByAuthor(userID, limit, offset int) ([]int, error) {
	var articles []models.ArticleTable
	result := Dbengine.Where("author = ?", userID).Limit(limit).Offset(offset).Find(&articles)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetArticlesByAuthor %e", result.Error).PrintAndWriteSafe()
		return nil, result.Error
	}
	var articleIntList []int
	for _, article := range articles {
		articleID := article.ArticleID
		articleIntList = append(articleIntList, articleID)
	}
	return articleIntList, nil
}

func GetArticlesByTitle(title string, limit, offset int) ([]int, error) {
	var articles []models.ArticleTable
	result := Dbengine.Where("title like", "%"+title+"%").Limit(limit).Offset(offset).Find(&articles)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetArticlesByTitle %e", result.Error).PrintAndWriteSafe()
		return nil, result.Error
	}
	var articleIntList []int
	for _, article := range articles {
		articleID := article.ArticleID
		articleIntList = append(articleIntList, articleID)
	}
	return articleIntList, nil
}

func GetArticlesByContent(content string, limit, offset int) ([]int, error) {
	var articles []models.ArticleContentTable
	result := Dbengine.Where("content like", "%"+content+"%").Limit(limit).Offset(offset).Find(&articles)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetArticlesByTitle %e", result.Error).PrintAndWriteSafe()
		return nil, result.Error
	}
	var articleIntList []int
	for _, article := range articles {
		articleID := article.ArticleID
		articleIntList = append(articleIntList, articleID)
	}
	return articleIntList, nil
}

func GetArticlesByCategory(category, limit, offset int) ([]int, error) {
	var articles []models.ArticleTable
	result := Dbengine.Where("category_id = ?", category).Limit(limit).Offset(offset).Find(&articles)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetArticlesByCategory %e", result.Error).PrintAndWriteSafe()
		return nil, result.Error
	}
	var articleIntList []int
	for _, article := range articles {
		articleID := article.ArticleID
		articleIntList = append(articleIntList, articleID)
	}
	return articleIntList, nil
}

func GetArticlesByTag(tagID, limit, offset int) ([]int, error) {
	var articles []models.ArticleTagTable
	result := Dbengine.Where("tag_id = ?", tagID).Limit(limit).Offset(offset).Find(&articles)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetArticleByTag %e", result.Error).PrintAndWriteSafe()
		return nil, result.Error
	}
	var articleIntList []int
	for _, article := range articles {
		articleID := article.ArticleID
		articleIntList = append(articleIntList, articleID)
	}
	return articleIntList, nil
}

func GetFullArticle(articleID int) (models.FullArticle, error) {
	var fullArticle models.FullArticle
	var article models.ArticleTable
	var articleContent models.ArticleContentTable
	var articleTag models.ArticleTagTable
	var likesNumber, commentNumber, collectionNumber int64
	result := Dbengine.First(&article, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = Dbengine.First(&articleContent, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = Dbengine.First(&articleTag, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = Dbengine.Model(&models.ArticleLikeTable{}).Where("article_id = ?", articleID).Count(&likesNumber)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = Dbengine.Model(&models.CommentTable{}).Where("article_id = ?", articleID).Count(&commentNumber)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = Dbengine.Model(&models.CollectionTable{}).Where("article_id = ?", articleID).Count(&collectionNumber)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	fullArticle = models.FullArticle{
		ArticleTable:        article,
		ArticleContentTable: articleContent,
		ArticleTagTable:     articleTag,
		LikesNumber:         int(likesNumber),
		CommentNumber:       int(commentNumber),
		CollectionNumber:    int(collectionNumber),
	}
	return fullArticle, nil
}

func CreateArticleLike(articleID, userID int) error {
	articleLike := &models.ArticleLikeTable{
		ArticleID: articleID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	result := Dbengine.Create(&articleLike)
	if result.Error != nil {
		tolog.Log().Infof("Error while CreateArticleLike %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func DeleteArticleLike(articleID, userID int) error {
	err := Dbengine.Where("article_id = ? and user_id = ?", articleID, userID).Delete(&models.ArticleLikeTable{}).Error
	if err != nil {
		tolog.Log().Infof("Error while DeleteArticleLike %e", err).PrintAndWriteSafe()
		return err
	}
	return nil
}

func GetArticleByIntList(list []int) ([]models.ArticleInfo, error) {
	var articles []models.ArticleInfo
	for _, articleID := range list {
		var article models.ArticleTable
		var articleTag models.ArticleTagTable
		var likesNumber, commentNumber, collectionNumber int64
		result := Dbengine.First(&article, articleID)
		if result.Error != nil {
			tolog.Log().Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		result = Dbengine.First(&articleTag, articleID)
		if result.Error != nil {
			tolog.Log().Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		result = Dbengine.Model(&models.ArticleLikeTable{}).Where("article_id = ?", articleID).Count(&likesNumber)
		if result.Error != nil {
			tolog.Log().Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		result = Dbengine.Model(&models.CommentTable{}).Where("article_id = ?", articleID).Count(&commentNumber)
		if result.Error != nil {
			tolog.Log().Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		result = Dbengine.Model(&models.CollectionTable{}).Where("article_id = ?", articleID).Count(&collectionNumber)
		if result.Error != nil {
			tolog.Log().Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		articleInfo := models.ArticleInfo{
			ArticleTable:     article,
			ArticleTagTable:  articleTag,
			LikesNumber:      int(likesNumber),
			CollectionNumber: int(collectionNumber),
			CommentNumber:    int(commentNumber),
		}
		articles = append(articles, articleInfo)
	}
	return articles, nil
}

func CreateArticleTag(articleID, tagID int) (bool, error) {
	articleTag := &models.ArticleTagTable{
		ArticleID: articleID,
		TagID:     tagID,
	}
	result := Dbengine.Create(&articleTag)
	if result.Error != nil {
		tolog.Log().Infof("Error while CreateArticleTag %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	return true, nil
}

func DeleteArticleTag(articleID, tagID int) (bool, error) {
	err := Dbengine.Where("article_id = ? and tag_id = ?", articleID, tagID).Delete(&models.ArticleTagTable{}).Error
	if err != nil {
		tolog.Log().Infof("Error while DeleteArticleTag %e", err).PrintAndWriteSafe()
		return false, err
	}
	return true, nil
}

func CheckArticleAuthor(articleID, author int) (bool, error) {
	var article models.ArticleTable
	result := Dbengine.Where("article_id = ? and author = ?", articleID, author).First(&article)
	if result.Error != nil {
		tolog.Log().Infof("Error while CheckArticleAuthor %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
