package api

import (
	"backendQucikStart/cache"
	"backendQucikStart/database"
	"backendQucikStart/models"
	"github.com/gin-gonic/gin"
)

func LinkUser() {
	userGroup := Server.Group("/user")
	userGroup.POST("/test", TestPost)
}

func TestPost(c *gin.Context) {
	// create empty post data struct
	var postData models.PostData

	// 使用ShouldBindJSON将前端发送的JSON数据绑定到PostData对象
	if err := c.ShouldBindJSON(&postData); err != nil {
		// 如果绑定失败，返回错误信息
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	cache.AddUser(postData.Username, postData.Password)
	database.SetUser(postData.Username, postData.Password)

	// 返回成功的响应
	c.JSON(200, gin.H{"message": "Data received successfully", "Username:": postData.Username, "Password:": postData.Password})
}
