package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"painter-server-new/database"
	"painter-server-new/models"
	"painter-server-new/utils"
	"strings"
	"time"
)

// upload
func FileUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}

	if file.Size > 10<<20 { // 10MB
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}

	fileSize := file.Size
	dir, err := utils.GetProjectDirRoot()
	fileType := GetFileExtension(file.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}

	fileName := time.Now().Format("20060102150405") + "_" + file.Filename

	err = database.CreateFileRecord(fileName, dir+fileName, fileSize, fileType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}

	dst := dir + "/server/static/upload/" + fileName
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{
		"filePath": "/fs/",
		"fileName": fileName,
	}))
	return
}

func GetFileExtension(fileName string) string {
	parts := strings.Split(fileName, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

func FileUploadDot() {

}

func FileUpload_tecent_cos() {

}

// getter
func FileGetter(c *gin.Context) {
	fileName := c.Param("filename")

	dir, err := utils.GetProjectDirRoot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}

	filePath := dir + "/static/upload/" + fileName

	_, err = os.Stat(filePath)
	if os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, models.R(models.KErrorNotFound, models.KReturnFalse, models.RDC{}))
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Type", "application/octet-stream")

	c.File(filePath)
}

func FileGetter_tecent_cos() {

}
