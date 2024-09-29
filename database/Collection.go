package database

import (
	"painter-server-new/models"

	"time"

	"github.com/callme-taota/tolog"
)

func CreateCollection(userId, articleId int) error {
	collection := models.CollectionTable{
		UserID:         userId,
		ArticleID:      articleId,
		CollectionTime: int(time.Now().Unix()),
	}

	result := DbEngine.Create(&collection)
	if result.Error != nil {
		tolog.Infof("Error while create collection %e", result.Error)
		return result.Error
	}
	return nil
}

func DeleteCollection(userId, articleId int) error {
	result := DbEngine.Where("user_id = ? AND article_id = ?", userId, articleId).Delete(&models.CollectionTable{})
	if result.Error != nil {
		tolog.Infof("Error while create collection %e", result.Error)
		return result.Error
	}
	return nil
}

func GetCollections(userId, limit, offset int) ([]models.CollectionTable, error) {
	var collections []models.CollectionTable
	result := DbEngine.Where("user_id = ?", userId).Limit(limit).Offset(offset).Find(&collections)
	if result.Error != nil {
		return nil, result.Error
	}
	return collections, nil
}

func GetCollectionsNumber(userId int) (int, error) {
	var count int64
	result := DbEngine.Model(&models.CollectionTable{}).Where("user_id = ?", userId).Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(count), nil
}

func CollectionArticle(articleID, userID int) error {
	flag, _ := HasCollection(articleID, userID)
	if flag {
		err := DeleteCollection(userID, articleID)
		if err != nil {
			return err
		}
		return nil
	}
	err := CreateCollection(userID, articleID)
	if err != nil {
		return err
	}
	return nil
}

func HasCollection(articleID, userID int) (bool, error) {
	var existingCollection models.CollectionTable
	res := DbEngine.Where("article_id = ? and user_id = ?", articleID, userID).First(&existingCollection)
	if res.RowsAffected <= 0 {
		return false, nil
	}
	return true, nil
}
