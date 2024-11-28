package database

import (
	"time"

	"github.com/callme-taota/painter/painter-backend/database/repository"
	"github.com/callme-taota/painter/painter-backend/models"
)

func CreateFileRecord(fileName, filePath string, fileSize int64, fileType string) error {
	DbEngine := repository.GetDBImplement().GetDB()
	fileRecord := models.FileStorageTable{
		FileName:  fileName,
		FilePath:  filePath,
		FileSize:  fileSize,
		FileType:  fileType,
		CreatedAt: time.Now(),
	}

	result := DbEngine.Create(&fileRecord)
	if result.Error != nil {
		return result.Error
	}

	return nil
}
