package mid

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"painter-server-new/cache"
	"painter-server-new/database"
	"painter-server-new/models"
	"painter-server-new/tolog"
	"strconv"
)

func SessionCheckMid() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, _ := c.Cookie("session")
		if session == "" {
			c.JSON(http.StatusBadRequest, models.R(models.KErrorInvalid, models.KReturnFalse, models.RDC{"userid": ""}))
			c.Abort()
			return
		}
		userid, err := cache.GetUserIDByUserSession(session)
		if err != nil {
			c.JSON(http.StatusOK, models.R(models.KErrorSessionInvalid, models.KReturnFalse, models.RDC{"userid": ""}))
			c.Abort()
			return
		}
		if userid == "" {
			c.JSON(http.StatusOK, models.R(models.KErrorSessionInvalid, models.KReturnFalse, models.RDC{"userid": ""}))
			c.Abort()
			return
		}
		userID, err := strconv.Atoi(userid)
		if err != nil {
			c.JSON(http.StatusOK, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{"userid": ""}))
			c.Abort()
			return
		}
		go func() {
			_, err := database.UpdateUserLoginTime(userID)
			if err != nil {
				tolog.Log().Infof("Can't update user login time %e", err).PrintAndWriteSafe()
			}
		}()
		c.Set("userID", userID)
		c.Next()
	}
}

func CheckAdminMid() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, _ := c.Cookie("session")
		if session == "" {
			c.JSON(http.StatusBadRequest, models.R(models.KErrorInvalid, models.KReturnFalse, models.RDC{"userid": ""}))
			c.Abort()
			return
		}
		userid, err := cache.GetUserIDByUserSession(session)
		if err != nil {
			c.JSON(http.StatusOK, models.R(models.KErrorSessionInvalid, models.KReturnFalse, models.RDC{"userid": ""}))
			c.Abort()
			return
		}
		if userid == "" {
			c.JSON(http.StatusOK, models.R(models.KErrorSessionInvalid, models.KReturnFalse, models.RDC{"userid": ""}))
			c.Abort()
			return
		}
		userID, err := strconv.Atoi(userid)
		if err != nil {
			c.JSON(http.StatusOK, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{"userid": ""}))
			c.Abort()
			return
		}
		adminFlag, err := database.GetAdminFlag(userID)
		if err != nil {
			c.JSON(http.StatusForbidden, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{"userid": ""}))
			c.Abort()
			return
		}
		go func() {
			_, err := database.UpdateUserLoginTime(userID)
			if err != nil {
				tolog.Log().Infof("Can't update user login time %e", err).PrintAndWriteSafe()
			}
		}()
		c.Set("userID", userID)
		c.Set("adminFlag", adminFlag)
		c.Next()
	}
}

func BetterLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		session, _ := c.Cookie("session")
		if session == "" {
			c.Set("isLogin", false)
			c.Next()
		}
		userid, err := cache.GetUserIDByUserSession(session)
		if err != nil {
			c.Set("isLogin", false)
			c.Next()
		}
		if userid == "" {
			c.Set("isLogin", false)
			c.Next()
		}
		userID, err := strconv.Atoi(userid)
		if err != nil {
			c.Set("isLogin", false)
			c.Next()
		}
		go func() {
			_, err := database.UpdateUserLoginTime(userID)
			if err != nil {
				tolog.Log().Infof("Can't update user login time %e", err).PrintAndWriteSafe()
			}
		}()
		c.Set("isLogin", true)
		c.Set("userID", userID)
		c.Next()
	}
}
