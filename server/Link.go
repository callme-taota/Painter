package server

import (
	"painter-server-new/api"
)

func LinkUser() {
	LoginMid := Server.Group("")
	LoginMid.Use(api.CheckLoginMid())

	userGroup := Server.Group("/user")
	userLoginGroup := LoginMid.Group("/user")

	userGroup.POST("/create", api.UserSignUp)
	userGroup.POST("/login/email", api.EmailLogin)
	userGroup.POST("/login/phone", api.PhoneLogin)
	userGroup.POST("/login/uname", api.UserNameLogin)
	userGroup.POST("/login/check", api.CheckLogin)
	userLoginGroup.POST("/logout", api.LogOut)
	userLoginGroup.POST("/update/name", api.UserNameUpdate)
	userLoginGroup.POST("/update/email", api.UserEmailUpdate)
	userLoginGroup.POST("/update/nickname", api.UserNickNameUpdate)
	userLoginGroup.POST("/update/phone", api.UserPhoneUpdate)
	userLoginGroup.POST("/update/headerfield", api.UserHeaderFieldUpdate)
	userLoginGroup.POST("/update", api.UserProfileUpdate)
}
