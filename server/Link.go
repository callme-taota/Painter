package server

import (
	"painter-server-new/api"
)

func LinkUser() {
	LoginMid := Server.Group("")
	LoginMid.Use(api.CheckLoginMid())

	userGroup := Server.Group("/user")
	userLoginedGroup := LoginMid.Group("/user")

	userGroup.POST("/create", api.UserSignUp)
	userGroup.POST("/login/email", api.EmailLogin)
	userGroup.POST("/login/phone", api.PhoneLogin)
	userGroup.POST("/login/uname", api.UserNameLogin)
	userGroup.POST("/login/check", api.CheckLogin)
	userLoginedGroup.POST("/logout", api.LogOut)
	userLoginedGroup.POST("/update/name", api.UserNameUpdate)
	userLoginedGroup.POST("/update/email", api.UserEmailUpdate)
	userLoginedGroup.POST("/update/nickname", api.UserNickNameUpdate)
	userLoginedGroup.POST("/update/phone", api.UserPhoneUpdate)
	userLoginedGroup.POST("/update/headerfield", api.UserHeaderFieldUpdate)
	userLoginedGroup.POST("/update", api.UserProfileUpdate)
}
