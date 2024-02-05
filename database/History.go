package database

import (
	"painter-server-new/models"
	"painter-server-new/tolog"
	"time"
)

func CreateHistory(userID, articleID int) (int, error) {
	history := models.HistoryTable{
		UserId:      userID,
		ArticleId:   articleID,
		HistoryTime: time.Now(),
	}

	result := Dbengine.Create(&history)
	if result.Error != nil {
		tolog.Log().Infof("Error while create history %e", result.Error).PrintAndWriteSafe()
		return -1, result.Error
	}
	return history.HistoryId, nil
}

func GetUserHistory(userID, limit, offset int) ([]models.HistoryTable, error) {
	var history []models.HistoryTable
	result := Dbengine.Where("user_id = ?", userID).Limit(limit).Offset(offset).Find(&history)
	if result.Error != nil {
		tolog.Log().Infof("Error while get history %e", result.Error).PrintAndWriteSafe()
		return nil, result.Error
	}
	return history, nil
}

func UpdateHistoryTime(historyID int) error {
	result := Dbengine.Model(&models.HistoryTable{}).Where("history_id = ?", historyID).
		Update("history_time", time.Now())
	return result.Error
}

func GetHistoryByUserIDAndArticleID(userID, articleID int) (models.HistoryTable, error) {
	var history models.HistoryTable
	result := Dbengine.Where("user_id = ? AND article_id = ?", userID, articleID).First(&history)
	if result.Error != nil {
		return history, result.Error
	}
	return history, nil
}

func UpdateHistoryTimeByUserIDAndArticleID(userID, articleID int) error {
	history, err := GetHistoryByUserIDAndArticleID(userID, articleID)
	if err != nil {
		return err
	}
	id := history.HistoryId
	err = UpdateHistoryTime(id)
	if err != nil {
		return err
	}
	return nil
}
