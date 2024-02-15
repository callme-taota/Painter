package server

import (
	"painter-server-new/api"
)

func LinkUser() {
	userGroup := Server.Group("/user")
	userLoginGroup := LoginGroup.Group("/user")

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

func LinkTag() {
	TagGroup := Server.Group("/tag")
	TagLoginGroup := LoginGroup.Group("/tag")

	TagGroup.GET("/suggest", api.SuggestTags)
	TagGroup.GET("/list", api.TagsList)
	TagLoginGroup.POST("/create", api.NewTag)
	TagLoginGroup.POST("/update/name", api.UpdateTagName)
	TagLoginGroup.POST("/update/desc", api.UpdateTagDesc)
}

func LinkHistory() {
	HistoryLoginGroup := LoginGroup.Group("/history")

	HistoryLoginGroup.GET("/list", api.GetHistories)
	HistoryLoginGroup.POST("/create", api.CreateHistory)
}

func LinkFollow() {
	FollowLoginGroup := LoginGroup.Group("/follow")

	FollowLoginGroup.GET("/followers", api.GetFollowers)
	FollowLoginGroup.GET("/followings", api.GetFollowings)

	FollowLoginGroup.POST("/follow", api.CreateFollow)
	FollowLoginGroup.POST("/unfollow", api.UnFollow)
}

func LinkComment() {
	CommentLoginGroup := LoginGroup.Group("/comment")

	CommentLoginGroup.POST("/create", api.CreateComment)
	CommentLoginGroup.POST("/delete", api.DeleteComment)
	CommentLoginGroup.POST("/like", api.CreateCommentLike)
	CommentLoginGroup.POST("/dislike", api.DeleteCommentLike)
}

func LinkCollection() {
	CollectionLoginGroup := LoginGroup.Group("/collection")

	CollectionLoginGroup.POST("/create", api.CreateCollection)
	CollectionLoginGroup.POST("/delete", api.DeleteCollection)
	CollectionLoginGroup.POST("/list", api.GetCollections)
}

func LinkCategory() {
	CategoryGroup := Server.Group("/category")
	CategoryLoginGroup := LoginGroup.Group("/category")

	CategoryGroup.Group("/list", api.GetCategories)

	CategoryLoginGroup.POST("/create", api.CreateCategory)
	CategoryLoginGroup.POST("/update/name", api.UpdateCategoryName)
	CategoryLoginGroup.POST("/update/desc", api.UpdateCategoryDesc)

}
