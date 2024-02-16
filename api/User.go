package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"painter-server-new/cache"
	"painter-server-new/database"
	"painter-server-new/models"
	"painter-server-new/models/APIs"
	"painter-server-new/utils"
	"strconv"
)

func CheckLogin(c *gin.Context) {
	var json APIs.CheckUsingSessionJSON
	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(400, gin.H{"msg": "post data error", "ok": "false", "userid": ""})
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Session"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	session := json.Session
	userid, err := cache.GetUserIDByUserSession(session)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorSessionInvalid, models.KReturnTrue, models.RDC{"userid": ""}))
		return
	}
	if userid == "" {
		c.JSON(http.StatusOK, models.R(models.KErrorSessionInvalid, models.KReturnFalse, models.RDC{"userid": ""}))
		c.Abort()
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"userid": userid}))
	return
}

func CheckLoginMid() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := c.Request.Header.Get("Session")
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
		c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"userid": userID}))
		c.Set("userID", userID)
		c.Next()
	}
}

func UserSignUp(c *gin.Context) {
	var json APIs.UserSignUpJSON

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"userid": ""}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"UserName", "Email", "NickName", "PhoneNum", "Password"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	id, err := database.CreateUser(json.UserName, json.Email, json.NickName, json.PhoneNum, json.HeaderField, json.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"userid": ""}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgOK, models.KReturnTrue, models.RDC{"userid": id}))
	return
}

func EmailLogin(c *gin.Context) {
	var json APIs.LoginUsingEmailJson

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Email", "Password"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	email, password := json.Email, json.Password
	if !utils.CheckMissings(email, password) {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	id, err := database.GetUserIdUsingEmail(email)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorNotFound, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	ok, _ = database.CheckUserPassword(id, password)
	if ok == false {
		c.JSON(http.StatusOK, models.R(models.KErrorPassword, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	session, err := cache.GetUserSessionByID(fmt.Sprintf("%d %s", id, email))
	if !utils.StringIsEmpty(session) {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
		return
	}
	session, err = cache.AddUser(fmt.Sprintf("%d %s", id, email))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
	return
}

func PhoneLogin(c *gin.Context) {
	var json APIs.LoginUsingPhoneJson

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Phone", "Password"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	phone, password := json.Phone, json.Password
	if !utils.CheckMissings(phone, password) {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	id, err := database.GetUserIdUsingPhoneNum(phone)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorNotFound, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	ok, _ = database.CheckUserPassword(id, password)
	if ok == false {
		c.JSON(http.StatusOK, models.R(models.KErrorPassword, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	session, err := cache.GetUserSessionByID(fmt.Sprintf("%d %s", id, phone))
	if !utils.StringIsEmpty(session) {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
		return
	}
	session, err = cache.AddUser(fmt.Sprintf("%d %s", id, phone))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
	return
}

func UserNameLogin(c *gin.Context) {
	var json APIs.LoginUsingUserNameJson

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"UserName", "Password"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	username, password := json.UserName, json.Password
	if !utils.CheckMissings(username, password) {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	id, err := database.GetUserIDUsingUserName(username)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorNotFound, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	ok, _ = database.CheckUserPassword(id, password)
	if ok == false {
		c.JSON(http.StatusOK, models.R(models.KErrorPassword, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	session, err := cache.GetUserSessionByID(fmt.Sprintf("%d %s", id, username))
	if !utils.StringIsEmpty(session) {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
		return
	}
	session, err = cache.AddUser(fmt.Sprintf("%d %s", id, username))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
	return
}

func LogOut(c *gin.Context) {
	var json APIs.LogOutJson
	headSession := c.Request.Header.Get("Session")
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Session"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	session := json.Session
	if session != headSession {
		c.JSON(http.StatusOK, models.R(models.KErrorSessionInvalid, models.KReturnFalse, models.RDC{}))
		return
	}
	if !utils.CheckMissings(session) {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	ok, err := cache.DeleteUserBySession(session)
	if ok != true || err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorSessionInvalid, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func UserNameUpdate(c *gin.Context) {
	userid, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusBadRequest, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{}))
		return
	}
	var json APIs.UserNameUpdateJson
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok = models.ShouldCheckJSON(json, []string{"Name"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.UpdateUserName(userid.(int), json.Name)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"name": json.Name}))
	return
}

func UserEmailUpdate(c *gin.Context) {
	userid, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusBadRequest, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{}))
		return
	}
	var json APIs.UserEmailUpdateJson
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok = models.ShouldCheckJSON(json, []string{"Email"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.UpdateUserEmail(userid.(int), json.Email)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"email": json.Email}))
	return
}

func UserNickNameUpdate(c *gin.Context) {
	userid, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusBadRequest, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{}))
		return
	}
	var json APIs.UserNickNameUpdateJson
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok = models.ShouldCheckJSON(json, []string{"NickName"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.UpdateUserNickName(userid.(int), json.NickName)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"nickname": json.NickName}))
	return
}

func UserPhoneUpdate(c *gin.Context) {
	userid, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusBadRequest, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{}))
		return
	}
	var json APIs.UserPhoneUpdateJson
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok = models.ShouldCheckJSON(json, []string{"PhoneNum"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.UpdateUserPhoneNum(userid.(int), json.PhoneNum)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"phonenum": json.PhoneNum}))
	return
}

func UserHeaderFieldUpdate(c *gin.Context) {
	userid, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusBadRequest, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{}))
		return
	}
	var json APIs.UserHeaderFieldJson
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok = models.ShouldCheckJSON(json, []string{"HeaderField"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.UpdateUserHeaderField(userid.(int), json.HeaderField)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"headerField": json.HeaderField}))
	return
}

func UserProfileUpdate(c *gin.Context) {
	userid, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusBadRequest, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{}))
		return
	}
	var json APIs.UserProfileJson
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok = models.ShouldCheckJSON(json, []string{"Name", "Email", "NickName", "PhoneNum", "HeaderField"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.UpdateUserProfile(userid.(int), json.Name, json.Email, json.NickName, json.PhoneNum)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	err = database.UpdateUserHeaderField(userid.(int), json.HeaderField)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"name": json.Name, "email": json.Email, "nickname": json.NickName, "phonenum": json.PhoneNum, "headerField": json.HeaderField}))
	return
}
