package database

import (
	"painter-server-new/models"
	"painter-server-new/models/APIs/Response"
	"painter-server-new/tolog"
	"time"
)

func CreateArticle(title string, author int, summary string, categoryID int, content string, tags []int) (int, error) {
	tx := DbEngine.Begin()

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
	result := DbEngine.First(&articleContent, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleContent %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	articleContent.Content = content
	result = DbEngine.Save(&articleContent)
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
	result := DbEngine.First(&article, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleSummary %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.Summary = summary
	result = DbEngine.Save(&article)
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
	result := DbEngine.First(&article, articleID)
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
	result := DbEngine.First(&article, articleID)
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
	result := DbEngine.First(&article, articleID)
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
	result := DbEngine.First(&article, articleID)
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
	result := DbEngine.First(&article, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleUpdateTime %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.UpdatedAt = time.Now()
	result = DbEngine.Save(&article)
	if result.Error != nil {
		tolog.Log().Infof("Error while UpdateArticleUpdateTime %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func DeleteArticle(articleID, author int) error {
	err := DbEngine.Where("article_id = ? and author = ?", articleID, author).Delete(&models.ArticleTable{}).Error
	if err != nil {
		tolog.Log().Infof("Error while delete article %e", err).PrintAndWriteSafe()
		return err
	}
	return nil
}

func GetArticlesByAuthor(userID, limit, offset int) ([]int, error) {
	var articles []models.ArticleTable
	result := DbEngine.Where("author = ?", userID).Limit(limit).Offset(offset).Find(&articles)
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

func GetArticleCountByAuthor(userID int) (int, error) {
	var count int64
	result := DbEngine.Model(&models.ArticleTable{}).Where("author = ?", userID).Count(&count)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetArticleCountByAuthor %e", result.Error).PrintAndWriteSafe()
		return 0, result.Error
	}
	return int(count), nil
}

func GetArticlesByTitle(title string, limit, offset int) ([]int, error) {
	var articles []models.ArticleTable
	result := DbEngine.Where("title like", "%"+title+"%").Limit(limit).Offset(offset).Find(&articles)
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
	result := DbEngine.Where("content like", "%"+content+"%").Limit(limit).Offset(offset).Find(&articles)
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
	result := DbEngine.Where("category_id = ?", category).Limit(limit).Offset(offset).Find(&articles)
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

func GetArticleCountByCategory(category int) (int, error) {
	var count int64
	result := DbEngine.Model(&models.ArticleTable{}).Where("category_id = ?", category).Count(&count)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetArticleCountByCategory %e", result.Error).PrintAndWriteSafe()
		return -1, result.Error
	}
	return int(count), nil
}

func GetArticlesByCollection(userID, limit, offset int) ([]int, error) {
	var collections []models.CollectionTable
	result := DbEngine.Where("user_id = ?", userID).Limit(limit).Offset(offset).Find(&collections)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetArticlesByCollection %e", result.Error).PrintAndWriteSafe()
		return nil, result.Error
	}

	var articleIDs []int
	for _, collection := range collections {
		articleIDs = append(articleIDs, collection.ArticleID)
	}
	return articleIDs, nil
}

func GetCollectionCountByUser(userID int) (int, error) {
	var count int64
	result := DbEngine.Model(&models.CollectionTable{}).Where("user_id = ?", userID).Count(&count)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetCollectionCountByUser %e", result.Error).PrintAndWriteSafe()
		return -1, result.Error
	}
	return int(count), nil
}

func GetArticlesByTag(tagID, limit, offset int) ([]int, error) {
	var articles []models.ArticleTagTable
	result := DbEngine.Where("tag_id = ?", tagID).Limit(limit).Offset(offset).Find(&articles)
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

func GetArticleIDsByTime(limit, offset int) ([]int, error) {
	var articleIDs []int
	result := DbEngine.Model(&models.ArticleTable{}).
		Select("article_id").
		Order("updated_at DESC").
		Limit(limit).
		Offset(offset).
		Pluck("article_id", &articleIDs)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetArticleByTag %e", result.Error).PrintAndWriteSafe()
		return nil, result.Error
	}
	return articleIDs, nil
}

func GetArticleCount() (int, error) {
	var count int64
	result := DbEngine.Model(&models.ArticleTable{}).Count(&count)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetArticleCount %e", result.Error).PrintAndWriteSafe()
		return 0, result.Error
	}
	return int(count), nil
}

func GetArticlesCountByTag(tagID int) (int, error) {
	var count int64
	result := DbEngine.Model(&models.ArticleTagTable{}).Where("tag_id = ?", tagID).Count(&count)
	if result.Error != nil {
		tolog.Log().Infof("Error while counting articles %e", result.Error).PrintAndWriteSafe()
		return 0, result.Error
	}
	return int(count), nil
}

func GetFullArticle(articleID int) (Response.FullArticle, error) {
	var fullArticle Response.FullArticle
	var article models.ArticleTable
	var articleContent models.ArticleContentTable
	var articleTag []models.ArticleTagTable
	var likesNumber, commentNumber, collectionNumber int64
	result := DbEngine.First(&article, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = DbEngine.First(&articleContent, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = DbEngine.Find(&articleTag, articleID)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	tagList, err := GetTagListByArticleTagTable(articleTag)
	if err != nil {
		tolog.Log().Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = DbEngine.Model(&models.ArticleLikeTable{}).Where("article_id = ?", articleID).Count(&likesNumber)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = DbEngine.Model(&models.CommentTable{}).Where("article_id = ?", articleID).Count(&commentNumber)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = DbEngine.Model(&models.CollectionTable{}).Where("article_id = ?", articleID).Count(&collectionNumber)
	if result.Error != nil {
		tolog.Log().Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	category, err := GetCategory(article.CategoryID)
	if err != nil {
		tolog.Log().Infof("Error while GetArticleByIntList %e", err).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	categoryName := category.CategoryName
	fullArticle = Response.FullArticle{
		ArticleTable:        article,
		ArticleContentTable: articleContent,
		ArticleTagTable:     tagList,
		LikesNumber:         int(likesNumber),
		CommentNumber:       int(commentNumber),
		CollectionNumber:    int(collectionNumber),
		CategoryName:        categoryName,
		Liked:               false,
		Collected:           false,
	}
	return fullArticle, nil
}

func CreateArticleLike(articleID, userID int) error {
	articleLike := &models.ArticleLikeTable{
		ArticleID: articleID,
		UserID:    userID,
		CreatedAt: time.Now(),
	}
	result := DbEngine.Create(&articleLike)
	if result.Error != nil {
		tolog.Log().Infof("Error while CreateArticleLike %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func DeleteArticleLike(articleID, userID int) error {
	err := DbEngine.Where("article_id = ? and user_id = ?", articleID, userID).Delete(&models.ArticleLikeTable{}).Error
	if err != nil {
		tolog.Log().Infof("Error while DeleteArticleLike %e", err).PrintAndWriteSafe()
		return err
	}
	return nil
}

func ToggleArticleLike(articleID, userID int) error {
	var existingLike models.ArticleLikeTable
	res := DbEngine.Where("article_id = ? and user_id = ?", articleID, userID).First(&existingLike)
	if res.RowsAffected > 0 {
		err := DeleteArticleLike(articleID, userID)
		if err != nil {
			return err
		}
		return nil
	}

	err := CreateArticleLike(articleID, userID)
	if err != nil {
		return err
	}

	return nil
}

func HasLikedArticle(articleID, userID int) (bool, error) {
	var existingLike models.ArticleLikeTable
	res := DbEngine.Where("article_id = ? and user_id = ?", articleID, userID).First(&existingLike)
	if res.Error != nil || res.RowsAffected <= 0 {
		tolog.Log().Infof("Error while checking existing like %e", res.Error).PrintAndWriteSafe()
		return false, res.Error
	}
	return true, nil
}

func GetArticleByIntList(list []int) ([]Response.ArticleInfo, error) {
	var articles []Response.ArticleInfo
	for _, articleID := range list {
		var article models.ArticleTable
		var articleTag []models.ArticleTagTable
		var likesNumber, commentNumber, collectionNumber int64
		result := DbEngine.First(&article, articleID)
		if result.Error != nil {
			tolog.Log().Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		result = DbEngine.Where("article_id = ?", articleID).Find(&articleTag)
		if result.Error != nil {
			tolog.Log().Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		tagList, err := GetTagListByArticleTagTable(articleTag)
		if err != nil {
			tolog.Log().Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		result = DbEngine.Model(&models.ArticleLikeTable{}).Where("article_id = ?", articleID).Count(&likesNumber)
		if result.Error != nil {
			tolog.Log().Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		result = DbEngine.Model(&models.CommentTable{}).Where("article_id = ?", articleID).Count(&commentNumber)
		if result.Error != nil {
			tolog.Log().Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		result = DbEngine.Model(&models.CollectionTable{}).Where("article_id = ?", articleID).Count(&collectionNumber)
		if result.Error != nil {
			tolog.Log().Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		articleInfo := Response.ArticleInfo{
			ArticleTable:     article,
			ArticleTagTable:  tagList,
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
	result := DbEngine.Create(&articleTag)
	if result.Error != nil {
		tolog.Log().Infof("Error while CreateArticleTag %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	return true, nil
}

func DeleteArticleTag(articleID, tagID int) (bool, error) {
	err := DbEngine.Where("article_id = ? and tag_id = ?", articleID, tagID).Delete(&models.ArticleTagTable{}).Error
	if err != nil {
		tolog.Log().Infof("Error while DeleteArticleTag %e", err).PrintAndWriteSafe()
		return false, err
	}
	return true, nil
}

func CheckArticleAuthor(articleID, author int) (bool, error) {
	var article models.ArticleTable
	result := DbEngine.Where("article_id = ? and author = ?", articleID, author).First(&article)
	if result.Error != nil {
		tolog.Log().Infof("Error while CheckArticleAuthor %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
