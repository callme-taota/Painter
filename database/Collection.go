package database

import (
	"painter-server-new/models"
	"painter-server-new/tolog"
	"time"
)

func CreateCollection(userId, articleId int) error {
	collection := models.CollectionTable{
		UserID:         userId,
		ArticleID:      articleId,
		CollectionTime: int(time.Now().Unix()),
	}

	result := Dbengine.Create(&collection)
	if result.Error != nil {
		tolog.Log().Infof("Error while create collection %e", result.Error)
		return result.Error
	}
	return nil
}

func DeleteCollection(userId, articleId int) error {
	result := Dbengine.Where("user_id = ? AND article_id = ?", userId, articleId).Delete(&models.CollectionTable{})
	if result.Error != nil {
		tolog.Log().Infof("Error while create collection %e", result.Error)
		return result.Error
	}
	return nil
}

func GetCollections(userId, limit, offset int) ([]models.CollectionTable, error) {
	var collections []models.CollectionTable
	result := Dbengine.Where("user_id = ?", userId).Limit(limit).Offset(offset).Find(&collections)
	if result.Error != nil {
		return nil, result.Error
	}
	return collections, nil
}

func GetCollectionsNumber(userId int) (int, error) {
	var count int64
	result := Dbengine.Model(&models.CollectionTable{}).Where("user_id = ?", userId).Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(count), nil
}
