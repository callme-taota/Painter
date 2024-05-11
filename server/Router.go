package server

import (
	"painter-server-new/conf"
	"painter-server-new/server/api"
)

func LinkUser() {
	userGroup := Server.Group("/user")
	userLoginGroup := LoginGroup.Group("/user")

	userGroup.POST("/create", api.UserSignUp)
	userGroup.POST("/login/email", api.EmailLogin)
	userGroup.POST("/login/email/pass", api.EmailPasswordCheck)
	//userGroup.POST("/login/phone", api.PhoneLogin)
	userGroup.POST("/login/uname", api.UserNameLogin)
	userGroup.POST("/login/uname/pass", api.UserNamePasswordCheck)
	userGroup.POST("/login/check", api.CheckLogin)
	userGroup.POST("/login/exist", api.CheckUserExist)
	userGroup.POST("/login/send", api.SendUserMail)
	userGroup.POST("/login/mailcheck", api.CheckMail)
	userLoginGroup.GET("/self", api.GetSelfData)
	userLoginGroup.GET("/self/full", api.GetUserSelfData)
	userLoginGroup.POST("/logout", api.LogOut)
	userLoginGroup.POST("/update/name", api.UserNameUpdate)
	userLoginGroup.POST("/update/email", api.UserEmailUpdate)
	userLoginGroup.POST("/update/nickname", api.UserNickNameUpdate)
	userLoginGroup.POST("/update/phone", api.UserPhoneUpdate)
	userLoginGroup.POST("/update/headerfield", api.UserHeaderFieldUpdate)
	userLoginGroup.POST("/update/passwd", api.UserResetPassword)
	userLoginGroup.POST("/update", api.UserProfileUpdate)

	userGroup.GET("/info", api.GetUserFullData)
}

func LinkTag() {
	TagGroup := Server.Group("/tag")
	//TagLoginGroup := LoginGroup.Group("/tag")
	TagAdminGroup := AdminGroup.Group("/tag")

	TagGroup.GET("/suggest", api.SuggestTags)
	TagGroup.GET("/list", api.TagsList)
	TagGroup.GET("/list/full", api.FullTagsListWithCount)
	TagAdminGroup.POST("/create", api.NewTag)
	TagAdminGroup.POST("/update/name", api.UpdateTagName)
	TagAdminGroup.POST("/update/desc", api.UpdateTagDesc)
}

func LinkHistory() {
	HistoryLoginGroup := LoginGroup.Group("/history")

	HistoryLoginGroup.GET("/list", api.GetHistories)
	//HistoryLoginGroup.POST("/create", api.CreateHistory)
}

func LinkFollow() {
	FollowLoginGroup := LoginGroup.Group("/follow")

	FollowLoginGroup.GET("/followers", api.GetFollowers)
	FollowLoginGroup.GET("/followings", api.GetFollowings)

	FollowLoginGroup.POST("/follow", api.CreateFollow)
	FollowLoginGroup.POST("/unfollow", api.UnFollow)
}

func LinkComment() {
	CommentGroup := Server.Group("/comment")
	CommentLoginGroup := LoginGroup.Group("/comment")

	CommentGroup.GET("/list", api.GetCommentsByArticleID)
	CommentLoginGroup.GET("/list/l", api.GetCommentsByArticleIDWithLiked)
	CommentLoginGroup.POST("/create", api.CreateComment)
	CommentLoginGroup.POST("/delete", api.DeleteComment)
	CommentLoginGroup.POST("/like", api.CreateCommentLike)
	CommentLoginGroup.POST("/dislike", api.DeleteCommentLike)
}

func LinkCollection() {
	CollectionLoginGroup := LoginGroup.Group("/collection")

	CollectionLoginGroup.POST("/delete", api.DeleteCollection)
	CollectionLoginGroup.POST("/list", api.GetCollections)
	CollectionLoginGroup.POST("/check", api.CheckCollectionArticle)
}

func LinkCategory() {
	CategoryGroup := Server.Group("/category")
	//CategoryLoginGroup := LoginGroup.Group("/category")
	CategoryAdminGroup := AdminGroup.Group("/category")

	CategoryGroup.GET("/list", api.GetCategories)
	CategoryGroup.GET("/get", api.GetCategory)
	CategoryGroup.GET("/get/fulllist", api.GetCategoryList)

	CategoryAdminGroup.POST("/create", api.CreateCategory)
	CategoryAdminGroup.POST("/update/name", api.UpdateCategoryName)
	CategoryAdminGroup.POST("/update/desc", api.UpdateCategoryDesc)

}

func LinkArticle() {
	ArticleGroup := Server.Group("/article")
	ArticleLoginGroup := LoginGroup.Group("/article")

	ArticleGroup.GET("/get/author", api.GetArticleByAuthor)
	ArticleGroup.GET("/get/title", api.GetArticlesByTitle)
	ArticleGroup.GET("/get/content", api.GetArticlesByContent)
	ArticleGroup.GET("/get/collection", api.GetArticlesByCollection)
	ArticleGroup.GET("/get/category", api.GetArticlesByCategory)
	ArticleGroup.GET("/get/tag", api.GetArticlesByTag)
	ArticleGroup.GET("/get/time", api.GetArticlesByTime)
	ArticleGroup.GET("/get/count", api.GetArticlesCount)
	ArticleLoginGroup.GET("/get/self", api.GetArticleSelf)
	ArticleGroup.GET("/get", api.GetFullArticle)

	ArticleLoginGroup.POST("/create", api.CreateArticle)
	ArticleLoginGroup.POST("/update/content", api.UpdateArticleContent)
	ArticleLoginGroup.POST("/update/summary", api.UpdateArticleSummary)
	ArticleLoginGroup.POST("/update/category", api.UpdateArticleCategory)
	//ArticleLoginGroup.POST("/update/readcount", api.UpdateArticleReadCount)
	ArticleLoginGroup.POST("/update/title", api.UpdateArticleTitle)
	ArticleLoginGroup.POST("/update/status", api.UpdateArticleStatus)
	//ArticleLoginGroup.POST("/update/istop", api.UpdateArticleIsTop)
	ArticleLoginGroup.POST("/delete", api.DeleteArticle)

	ArticleLoginGroup.POST("/like/create", api.CreateArticleLike)
	ArticleLoginGroup.POST("/like/delete", api.DeleteArticleLike)
	ArticleLoginGroup.POST("/like", api.ArticleLike)
	ArticleLoginGroup.POST("/like/check", api.CheckArticleLike)

	ArticleLoginGroup.POST("/collection", api.CollectionArticle)

	ArticleLoginGroup.POST("/tag/create", api.CreateArticleTag)
	ArticleLoginGroup.POST("/tag/update", api.UpdateArticleTag)
	ArticleLoginGroup.POST("/tag/delete", api.DeleteArticleTag)
}

func LinkFile() {
	FileGroup := LoginGroup.Group("/file")

	FileGroup.GET("/get", api.FileGetter)
	FileGroup.POST("/upload", api.FileUpload)
}

func LinkCommon() {
	CommonGroup := Server.Group("/common")

	CommonGroup.GET("/starttime", api.GetServerRunningTime)
	CommonGroup.GET("/vis/preday", api.GetServerPreDayVis)
	CommonGroup.GET("/vis/currmonth", api.GetServerCurrentMonthVis)
}

func LinkDebug() {
	if conf.Server.Model == "debug" {
		Server.GET("/"+conf.RandomKey, api.SetDebugKeyInCookie)
	}
}
