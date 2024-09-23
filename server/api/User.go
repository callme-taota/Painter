package api

import (
	"fmt"
	"net/http"
	"painter-server-new/cache"
	"painter-server-new/database"
	"painter-server-new/models"
	"painter-server-new/models/APIs/Request"
	"painter-server-new/server/mail"
	"painter-server-new/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

func CheckLogin(c *gin.Context) {
	var json Request.CheckUsingSessionJSON
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

func UserSignUp(c *gin.Context) {
	var json Request.UserSignUpJSON

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

func UserResetPassword(c *gin.Context) {
	var json Request.UserResetPasswordJSON

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"userid": ""}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"ID", "OldPassword", "NewPassword"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.ResetPassWord(json.ID, json.OldPassword, json.NewPassword)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"userid": ""}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgOK, models.KReturnTrue, models.RDC{}))
	return
}

func EmailLogin(c *gin.Context) {
	var json Request.LoginUsingEmailJson

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
	session, err := cache.GetUserSessionByID(fmt.Sprintf("%d", id))
	if !utils.StringIsEmpty(session) {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
		return
	}
	session, err = cache.AddUser(fmt.Sprintf("%d", id))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	c.SetCookie("painter-session", session, 3600*24*30, "/", "", false, true)
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
	return
}

func EmailPasswordCheck(c *gin.Context) {
	var json Request.LoginUsingEmailJson

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Correct": false}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Email", "Password"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{"Correct": false}))
		return
	}
	email, password := json.Email, json.Password
	if !utils.CheckMissings(email, password) {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{"Correct": false}))
		return
	}
	id, err := database.GetUserIdUsingEmail(email)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorNotFound, models.KReturnFalse, models.RDC{"Correct": false}))
		return
	}
	ok, _ = database.CheckUserPassword(id, password)
	if ok == false {
		c.JSON(http.StatusOK, models.R(models.KErrorPassword, models.KReturnFalse, models.RDC{"Correct": false}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Correct": true}))
	return
}

func PhoneLogin(c *gin.Context) {
	var json Request.LoginUsingPhoneJson

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
	session, err := cache.GetUserSessionByID(fmt.Sprintf("%d", id))
	if !utils.StringIsEmpty(session) {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
		return
	}
	session, err = cache.AddUser(fmt.Sprintf("%d", id))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	c.SetCookie("painter-session", session, 3600*24*30, "/", "", false, true)
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
	return
}

func UserNameLogin(c *gin.Context) {
	var json Request.LoginUsingUserNameJson

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
	session, err := cache.GetUserSessionByID(fmt.Sprintf("%d", id))
	if !utils.StringIsEmpty(session) {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
		return
	}
	session, err = cache.AddUser(fmt.Sprintf("%d", id))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	c.SetCookie("painter-session", session, 3600*24*30, "/", "", false, true)
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
	return
}

func UserNamePasswordCheck(c *gin.Context) {
	var json Request.LoginUsingUserNameJson

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Correct": false}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"UserName", "Password"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{"Correct": false}))
		return
	}
	username, password := json.UserName, json.Password
	if !utils.CheckMissings(username, password) {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{"Correct": false}))
		return
	}
	id, err := database.GetUserIDUsingUserName(username)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorNotFound, models.KReturnFalse, models.RDC{"Correct": false}))
		return
	}
	ok, _ = database.CheckUserPassword(id, password)
	if ok == false {
		c.JSON(http.StatusOK, models.R(models.KErrorPassword, models.KReturnFalse, models.RDC{"Correct": false}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Correct": true}))
	return
}

func LogOut(c *gin.Context) {
	var json Request.LogOutJson
	headSession, _ := c.Cookie("painter-session")
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
	c.SetCookie("painter-session", "", 1, "/", "", false, true)
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{}))
	return
}

func UserNameUpdate(c *gin.Context) {
	userid, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusBadRequest, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{}))
		return
	}
	var json Request.UserNameUpdateJson
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
	var json Request.UserEmailUpdateJson
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
	var json Request.UserNickNameUpdateJson
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
	var json Request.UserPhoneUpdateJson
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
	var json Request.UserHeaderFieldJson
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
	var json Request.UserProfileJson
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok = models.ShouldCheckJSON(json, []string{"Name", "Email", "NickName", "PhoneNum"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	err := database.UpdateUserProfile(userid.(int), json.Name, json.Email, json.NickName, json.PhoneNum)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	//err = database.UpdateUserHeaderField(userid.(int), json.HeaderField)
	//if err != nil {
	//	c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
	//	return
	//}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"name": json.Name, "email": json.Email, "nickname": json.NickName, "phonenum": json.PhoneNum}))
	return
}

func GetSelfData(c *gin.Context) {
	userid, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusBadRequest, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{}))
		return
	}
	user, err := database.GetUserInfo(userid.(int))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, user))
	return
}

func GetUserSelfData(c *gin.Context) {
	userid, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusBadRequest, models.R(models.KErrorNoUser, models.KReturnFalse, models.RDC{}))
		return
	}
	user, err := database.GetUserSelfInfo(userid.(int))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, user))
	return
}

func GetUserFullData(c *gin.Context) {
	var json Request.UserIDJSON
	json.ID, _ = strconv.Atoi(c.DefaultQuery("ID", ""))
	ok := models.ShouldCheckJSON(json, []string{"ID"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	user, err := database.GetUserFullInfo(json.ID)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	isLogin, _ := c.Get("isLogin")
	if isLogin.(bool) {
		userID, _ := c.Get("userID")
		following, err := database.CheckFollow(user.UserInfo.ID, userID.(int))
		if err != nil {
			c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
			return
		}
		user.Following = following
		if userID == json.ID {
			user.Self = true
		}
	}
	c.JSON(http.StatusOK, models.Rs(models.KReturnMsgSuccess, models.KReturnTrue, user))
	return
}

func CheckUserExist(c *gin.Context) {
	var json Request.UserExistJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Key"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	has, err := database.HasUser(json.Key)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Has": has}))
	return
}

func SendUserMail(c *gin.Context) {
	var json Request.UserEmailCodeJSON
	if err := c.ShouldBind(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Email"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	isMail := utils.IsValidEmail(json.Email)
	if !isMail {
		userEmail, err := database.GetUserEmailUsingUserName(json.Email)
		if err != nil {
			c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
			return
		}
		go mail.Sender(userEmail)
		c.JSON(models.SuccessR())
		return
	}
	go mail.Sender(json.Email)
	c.JSON(models.SuccessR())
	return
}

func CheckMail(c *gin.Context) {
	var json Request.UserEmailCodeJSON

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	ok := models.ShouldCheckJSON(json, []string{"Email", "Code"})
	if ok != true {
		c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
		return
	}
	isMail := utils.IsValidEmail(json.Email)
	if !isMail {
		id, err := database.GetUserIDUsingUserName(json.Email)
		if err != nil {
			c.JSON(http.StatusOK, models.R(models.KErrorNotFound, models.KReturnFalse, models.RDC{"Session": ""}))
			return
		}
		userEmail, err := database.GetUserEmailUsingUserName(json.Email)
		if err != nil {
			c.JSON(http.StatusOK, models.R(models.KErrorMissing, models.KReturnFalse, models.RDC{}))
			return
		}
		ok, err = cache.CheckEmailPass(userEmail, json.Code)
		if err != nil || !ok {
			c.JSON(http.StatusOK, models.R(models.KErrorNotFound, models.KReturnFalse, models.RDC{"Session": ""}))
			return
		}
		session, err := cache.GetUserSessionByID(fmt.Sprintf("%d", id))
		if !utils.StringIsEmpty(session) {
			c.SetCookie("painter-session", session, 3600*24*30, "/", "", false, true)
			c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
			return
		}
		session, err = cache.AddUser(fmt.Sprintf("%d", id))
		if err != nil {
			c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Session": ""}))
			return
		}
		c.SetCookie("painter-session", session, 3600*24*30, "/", "", false, true)
		c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
		return
	}
	id, err := database.GetUserIdUsingEmail(json.Email)
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KErrorNotFound, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	ok, err = cache.CheckEmailPass(json.Email, json.Code)
	if err != nil || !ok {
		c.JSON(http.StatusOK, models.R(models.KErrorNotFound, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	session, err := cache.GetUserSessionByID(fmt.Sprintf("%d", id))
	if !utils.StringIsEmpty(session) {
		c.SetCookie("painter-session", session, 3600*24*30, "/", "", false, true)
		c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
		return
	}
	session, err = cache.AddUser(fmt.Sprintf("%d", id))
	if err != nil {
		c.JSON(http.StatusOK, models.R(models.KReturnMsgError, models.KReturnFalse, models.RDC{"Session": ""}))
		return
	}
	c.SetCookie("painter-session", session, 3600*24*30, "/", "", false, true)
	c.JSON(http.StatusOK, models.R(models.KReturnMsgSuccess, models.KReturnTrue, models.RDC{"Session": session}))
	return
}
