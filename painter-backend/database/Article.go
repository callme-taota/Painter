package database

import (
	"painter-server-new/models"
	"painter-server-new/models/APIs/Response"

	"time"

	"github.com/callme-taota/tolog"
	"gorm.io/gorm"
)

func CreateArticle(title string, author int, summary string, categoryID int, content string, tags []int) (int, error) {
	tx := DbEngine.Begin()

	article := &models.ArticleTable{
		Title:      title,
		Author:     author,
		Summary:    summary,
		CategoryID: categoryID,
		CreatedAt:  time.Now(),
		Status:     0,
	}
	if err := tx.Create(&article).Error; err != nil {
		tx.Rollback()
		tolog.Infof("Error while CreateArticle %e", err).PrintAndWriteSafe()
		return -1, err
	}

	articleID := article.ArticleID
	articleContent := &models.ArticleContentTable{
		ArticleID: articleID,
		Content:   content,
	}
	if err := tx.Create(&articleContent).Error; err != nil {
		tx.Rollback()
		tolog.Infof("Error while CreateArticle %e", err).PrintAndWriteSafe()
		return -2, err
	}

	for _, tag := range tags {
		articleTag := models.ArticleTagTable{
			ArticleID: articleID,
			TagID:     tag,
		}
		if err := tx.Create(&articleTag).Error; err != nil {
			tx.Rollback()
			tolog.Infof("Error while CreateArticle %e", err).PrintAndWriteSafe()
			return -3, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		tolog.Infof("Error while committing transaction %e", err).PrintAndWriteSafe()
		return -4, err
	}

	return articleID, nil
}

func UpdateArticleContent(articleID int, content string) error {
	var articleContent models.ArticleContentTable
	result := DbEngine.First(&articleContent, articleID)
	if result.Error != nil {
		tolog.Infof("Error while UpdateArticleContent %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	articleContent.Content = content
	result = DbEngine.Where("article_id = ?", articleID).Save(&articleContent)
	if result.Error != nil {
		tolog.Infof("Error while UpdateArticleContent %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	err := UpdateArticleUpdateTime(articleID)
	if err != nil {
		tolog.Infof("Error while UpdateArticleContent %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func UpdateArticleSummary(articleID int, summary string) error {
	var article models.ArticleTable
	result := DbEngine.First(&article, articleID)
	if result.Error != nil {
		tolog.Infof("Error while UpdateArticleSummary %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.Summary = summary
	result = DbEngine.Save(&article)
	if result.Error != nil {
		tolog.Infof("Error while UpdateArticleSummary %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	err := UpdateArticleUpdateTime(articleID)
	if err != nil {
		tolog.Infof("Error while UpdateArticleSummary %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func UpdateArticleReadCount(articleID, count int) error {
	result := DbEngine.Model(&models.ArticleTable{}).Where("article_id = ?", articleID).UpdateColumn("read_count", count)
	if result.Error != nil {
		tolog.Infof("Error while UpdateArticleReadCount %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func UpdateArticle(article models.ArticleTable, content models.ArticleContentTable, tagList []int) error {
	tx := DbEngine.Begin()
	result := tx.Model(&models.ArticleTable{}).Where("article_id = ?", article.ArticleID).Select("title", "summary", "category_id").UpdateColumns(article)
	if result.Error != nil {
		tolog.Infof("Error while UpdateArticle %e", result.Error).PrintAndWriteSafe()
		tx.Rollback()
		return result.Error
	}
	result = tx.Model(&models.ArticleContentTable{}).Where("article_id = ?", article.ArticleID).UpdateColumn("content", content.Content)
	if result.Error != nil {
		tolog.Infof("Error while UpdateArticle %e", result.Error).PrintAndWriteSafe()
		tx.Rollback()
		return result.Error
	}
	err := UpdateArticleTagsWithDB(tx, article.ArticleID, tagList)
	if err != nil {
		tolog.Infof("Error while UpdateArticle %e", result.Error).PrintAndWriteSafe()
		tx.Rollback()
		return err
	}
	tx.Commit()
	return nil
}

func ArticleReadCountAdd(articleID int) error {
	var readCount int
	result := DbEngine.Model(&models.ArticleTable{}).Where("article_id = ?", articleID).Pluck("read_count", &readCount)
	if result.Error != nil {
		tolog.Infof("Error while updating article read count: %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	readCount += 1
	err := UpdateArticleReadCount(articleID, readCount)
	if err != nil {
		tolog.Infof("Error while updating article read count: %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func UpdateArticleTitle(articleID int, title string) error {
	var article models.ArticleTable
	result := DbEngine.First(&article, articleID)
	if result.Error != nil {
		tolog.Infof("Error while UpdateArticleTitle %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.Title = title
	err := UpdateArticleUpdateTime(articleID)
	if err != nil {
		tolog.Infof("Error while UpdateArticleTitle %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func UpdateArticleStatus(articleID int, status int) error {
	var article models.ArticleTable
	result := DbEngine.First(&article, articleID)
	if result.Error != nil {
		tolog.Infof("Error while UpdateArticleStatus %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.Status = status
	result = DbEngine.Where("article_id = ?", articleID).Save(&article)
	if result.Error != nil {
		tolog.Infof("Error while UpdateArticleStatus %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	err := UpdateArticleUpdateTime(articleID)
	if err != nil {
		tolog.Infof("Error while UpdateArticleStatus %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func UpdateArticleUpdateTime(articleID int) error {
	var article models.ArticleTable
	result := DbEngine.First(&article, articleID)
	if result.Error != nil {
		tolog.Infof("Error while UpdateArticleUpdateTime %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	article.UpdatedAt = time.Now()
	result = DbEngine.Save(&article)
	if result.Error != nil {
		tolog.Infof("Error while UpdateArticleUpdateTime %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func DeleteArticle(articleID, author int) error {
	err := DbEngine.Where("article_id = ? and author = ?", articleID, author).Delete(&models.ArticleTable{}).Error
	if err != nil {
		tolog.Infof("Error while delete article %e", err).PrintAndWriteSafe()
		return err
	}
	return nil
}

func GetArticlesByAuthor(userID, limit, offset int) ([]int, error) {
	var articles []models.ArticleTable
	result := DbEngine.
		Where("author = ? and status = 1", userID).
		Limit(limit).
		Offset(offset).
		Order("updated_at DESC").
		Find(&articles)
	if result.Error != nil {
		tolog.Infof("Error while GetArticlesByAuthor %e", result.Error).PrintAndWriteSafe()
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
		tolog.Infof("Error while GetArticleCountByAuthor %e", result.Error).PrintAndWriteSafe()
		return 0, result.Error
	}
	return int(count), nil
}

func GetArticlesByTitle(title string, limit, offset int) ([]int, error) {
	var articles []models.ArticleTable
	result := DbEngine.Where("title like and status = 1", "%"+title+"%").Limit(limit).Offset(offset).
		Order("updated_at DESC").
		Find(&articles)
	if result.Error != nil {
		tolog.Infof("Error while GetArticlesByTitle %e", result.Error).PrintAndWriteSafe()
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
	result := DbEngine.Where("content like and status = 1", "%"+content+"%").Limit(limit).Offset(offset).
		Order("updated_at DESC").
		Find(&articles)
	if result.Error != nil {
		tolog.Infof("Error while GetArticlesByTitle %e", result.Error).PrintAndWriteSafe()
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
	result := DbEngine.Where("category_id = ? and status = 1", category).Limit(limit).Offset(offset).
		Order("updated_at DESC").
		Find(&articles)
	if result.Error != nil {
		tolog.Infof("Error while GetArticlesByCategory %e", result.Error).PrintAndWriteSafe()
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
	result := DbEngine.Model(&models.ArticleTable{}).Where("category_id = ? and status = 1", category).Count(&count)
	if result.Error != nil {
		tolog.Infof("Error while GetArticleCountByCategory %e", result.Error).PrintAndWriteSafe()
		return -1, result.Error
	}
	return int(count), nil
}

func GetArticlesByCollection(userID, limit, offset int) ([]int, error) {
	var collections []models.CollectionTable
	result := DbEngine.Where("user_id = ? and status = 1", userID).Limit(limit).Offset(offset).
		Find(&collections)
	if result.Error != nil {
		tolog.Infof("Error while GetArticlesByCollection %e", result.Error).PrintAndWriteSafe()
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
		tolog.Infof("Error while GetCollectionCountByUser %e", result.Error).PrintAndWriteSafe()
		return -1, result.Error
	}
	return int(count), nil
}

func GetArticlesByTag(tagID, limit, offset int) ([]int, error) {
	var articles []models.ArticleTagTable
	result := DbEngine.Where("tag_id = ?", tagID).Limit(limit).Offset(offset).
		Find(&articles)
	if result.Error != nil {
		tolog.Infof("Error while GetArticleByTag %e", result.Error).PrintAndWriteSafe()
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
		Where("status = 1").
		Order("updated_at DESC").
		Limit(limit).
		Offset(offset).
		Pluck("article_id", &articleIDs)
	if result.Error != nil {
		tolog.Infof("Error while GetArticleByTag %e", result.Error).PrintAndWriteSafe()
		return nil, result.Error
	}
	return articleIDs, nil
}

func GetArticleCount() (int, error) {
	var count int64
	result := DbEngine.Model(&models.ArticleTable{}).Where("status = 1").Count(&count)
	if result.Error != nil {
		tolog.Infof("Error while GetArticleCount %e", result.Error).PrintAndWriteSafe()
		return 0, result.Error
	}
	return int(count), nil
}

func GetArticlesCountByTag(tagID int) (int, error) {
	var count int64
	result := DbEngine.Model(&models.ArticleTagTable{}).Where("tag_id = ? and status = 1", tagID).Count(&count)
	if result.Error != nil {
		tolog.Infof("Error while counting articles %e", result.Error).PrintAndWriteSafe()
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
		tolog.Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = DbEngine.First(&articleContent, articleID)
	if result.Error != nil {
		tolog.Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = DbEngine.Find(&articleTag, articleID)
	if result.Error != nil {
		tolog.Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	tagList, err := GetTagListByArticleTagTable(articleTag)
	if err != nil {
		tolog.Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = DbEngine.Model(&models.ArticleLikeTable{}).Where("article_id = ?", articleID).Count(&likesNumber)
	if result.Error != nil {
		tolog.Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = DbEngine.Model(&models.CommentTable{}).Where("article_id = ?", articleID).Count(&commentNumber)
	if result.Error != nil {
		tolog.Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	result = DbEngine.Model(&models.CollectionTable{}).Where("article_id = ?", articleID).Count(&collectionNumber)
	if result.Error != nil {
		tolog.Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	var miniUserInfo Response.MiniUserInfo
	result = DbEngine.Table("user").Select("id, email, nick_name, header_field, created_at").Where("id = ?", article.Author).First(&miniUserInfo)
	if result.Error != nil {
		tolog.Infof("Error while GetFullArticle %e", result.Error).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	category, err := GetCategory(article.CategoryID)
	if err != nil {
		tolog.Infof("Error while GetArticleByIntList %e", err).PrintAndWriteSafe()
		return fullArticle, result.Error
	}
	_ = ArticleReadCountAdd(articleID)
	categoryName := category.CategoryName
	fullArticle = Response.FullArticle{
		ArticleTable:        article,
		ArticleContentTable: articleContent,
		ArticleTagTable:     tagList,
		User:                miniUserInfo,
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
		tolog.Infof("Error while CreateArticleLike %e", result.Error).PrintAndWriteSafe()
		return result.Error
	}
	return nil
}

func DeleteArticleLike(articleID, userID int) error {
	err := DbEngine.Where("article_id = ? and user_id = ?", articleID, userID).Delete(&models.ArticleLikeTable{}).Error
	if err != nil {
		tolog.Infof("Error while DeleteArticleLike %e", err).PrintAndWriteSafe()
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
		tolog.Infof("Error while checking existing like %e", res.Error).PrintAndWriteSafe()
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
			tolog.Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		result = DbEngine.Where("article_id = ?", articleID).Find(&articleTag)
		if result.Error != nil {
			tolog.Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		tagList, err := GetTagListByArticleTagTable(articleTag)
		if err != nil {
			tolog.Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		result = DbEngine.Model(&models.ArticleLikeTable{}).Where("article_id = ?", articleID).Count(&likesNumber)
		if result.Error != nil {
			tolog.Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		result = DbEngine.Model(&models.CommentTable{}).Where("article_id = ?", articleID).Count(&commentNumber)
		if result.Error != nil {
			tolog.Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
			return nil, result.Error
		}
		result = DbEngine.Model(&models.CollectionTable{}).Where("article_id = ?", articleID).Count(&collectionNumber)
		if result.Error != nil {
			tolog.Infof("Error while GetArticleByIntList %e", result.Error).PrintAndWriteSafe()
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
		tolog.Infof("Error while CreateArticleTag %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	return true, nil
}

func DeleteArticleTag(articleID, tagID int) (bool, error) {
	err := DbEngine.Where("article_id = ? and tag_id = ?", articleID, tagID).Delete(&models.ArticleTagTable{}).Error
	if err != nil {
		tolog.Infof("Error while DeleteArticleTag %e", err).PrintAndWriteSafe()
		return false, err
	}
	return true, nil
}

func UpdateArticleTagsWithDB(db *gorm.DB, articleID int, tagIDs []int) error {
	// 查询当前文章已有的标签列表
	var existingTagIDs []int
	err := db.Model(&models.ArticleTagTable{}).Where("article_id = ?", articleID).Pluck("tag_id", &existingTagIDs).Error
	if err != nil {
		tolog.Infof("Error while querying existing tags for article %d: %e", articleID, err).PrintAndWriteSafe()
		return err
	}

	// 找出需要删除的标签
	var tagsToDelete []int
	for _, existingTagID := range existingTagIDs {
		found := false
		for _, tagID := range tagIDs {
			if existingTagID == tagID {
				found = true
				break
			}
		}
		if !found {
			tagsToDelete = append(tagsToDelete, existingTagID)
		}
	}

	// 找出需要新增的标签
	var tagsToAdd []int
	for _, tagID := range tagIDs {
		found := false
		for _, existingTagID := range existingTagIDs {
			if tagID == existingTagID {
				found = true
				break
			}
		}
		if !found {
			tagsToAdd = append(tagsToAdd, tagID)
		}
	}

	// 删除需要删除的标签记录
	if len(tagsToDelete) > 0 {
		err = db.Where("article_id = ? and tag_id in (?)", articleID, tagsToDelete).Delete(&models.ArticleTagTable{}).Error
		if err != nil {
			tolog.Infof("Error while deleting tags for article %d: %e", articleID, err).PrintAndWriteSafe()
			return err
		}
	}

	// 新增需要新增的标签记录
	for _, tagID := range tagsToAdd {
		articleTag := &models.ArticleTagTable{
			ArticleID: articleID,
			TagID:     tagID,
		}
		err = db.Create(&articleTag).Error
		if err != nil {
			tolog.Infof("Error while creating tag for article %d: %e", articleID, err).PrintAndWriteSafe()
			return err
		}
	}

	return nil
}

func UpdateArticleTags(articleID int, tagIDs []int) error {
	return UpdateArticleTagsWithDB(DbEngine, articleID, tagIDs)
}

func CheckArticleAuthor(articleID, author int) (bool, error) {
	var article models.ArticleTable
	result := DbEngine.Where("article_id = ? and author = ?", articleID, author).First(&article)
	if result.Error != nil {
		tolog.Infof("Error while CheckArticleAuthor %e", result.Error).PrintAndWriteSafe()
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
