package database

import (
	"painter-server-new/models"
	"time"
)

func CreateFileRecord(fileName, filePath string, fileSize int64, fileType string) error {
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
