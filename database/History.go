package database

import (
	"painter-server-new/models"

	"github.com/callme-taota/tolog"

	"time"
)

func CreateHistory(userID, articleID int) (int, error) {
	history := models.HistoryTable{
		UserID:      userID,
		ArticleID:   articleID,
		HistoryTime: time.Now(),
	}

	result := DbEngine.Create(&history)
	if result.Error != nil {
		tolog.Infof("Error while create history %e", result.Error).PrintAndWriteSafe()
		return -1, result.Error
	}
	return history.HistoryID, nil
}

func GetUserHistories(userID, limit, offset int) ([]models.HistoryTable, error) {
	var history []models.HistoryTable
	result := DbEngine.Where("user_id = ?", userID).Limit(limit).Offset(offset).Find(&history)
	if result.Error != nil {
		tolog.Infof("Error while get history %e", result.Error).PrintAndWriteSafe()
		return nil, result.Error
	}
	return history, nil
}

func UpdateHistoryTime(historyID int) error {
	result := DbEngine.Model(&models.HistoryTable{}).Where("history_id = ?", historyID).
		Update("history_time", time.Now())
	return result.Error
}

func GetHistoryByUserIDAndArticleID(userID, articleID int) (models.HistoryTable, error) {
	var history models.HistoryTable
	result := DbEngine.Where("user_id = ? AND article_id = ?", userID, articleID).First(&history)
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
	id := history.HistoryID
	err = UpdateHistoryTime(id)
	if err != nil {
		return err
	}
	return nil
}

func CheckHistoryExist(userID, articleID int) (bool, error) {
	var history models.HistoryTable
	result := DbEngine.Where("user_id = ? AND article_id = ?", userID, articleID).First(&history)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func AutoHistory(userID, articleID int) (bool, error) {
	flag, err := CheckHistoryExist(userID, articleID)
	if flag == false {
		_, err = CreateHistory(userID, articleID)
		if err != nil {
			return false, err
		}
	} else {
		err = UpdateHistoryTimeByUserIDAndArticleID(userID, articleID)
		if err != nil {
			return false, err
		}
	}
	return true, nil
}
