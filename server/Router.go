package server

import (
	api2 "painter-server-new/server/api"
)

func LinkUser() {
	userGroup := Server.Group("/user")
	userLoginGroup := LoginGroup.Group("/user")

	userGroup.POST("/create", api2.UserSignUp)
	userGroup.POST("/login/email", api2.EmailLogin)
	userGroup.POST("/login/phone", api2.PhoneLogin)
	userGroup.POST("/login/uname", api2.UserNameLogin)
	userGroup.POST("/login/check", api2.CheckLogin)
	userLoginGroup.POST("/self", api2.GetSelfData)
	userLoginGroup.POST("/self/full", api2.GetUserData)
	userLoginGroup.POST("/logout", api2.LogOut)
	userLoginGroup.POST("/update/name", api2.UserNameUpdate)
	userLoginGroup.POST("/update/email", api2.UserEmailUpdate)
	userLoginGroup.POST("/update/nickname", api2.UserNickNameUpdate)
	userLoginGroup.POST("/update/phone", api2.UserPhoneUpdate)
	userLoginGroup.POST("/update/headerfield", api2.UserHeaderFieldUpdate)
	userLoginGroup.POST("/update", api2.UserProfileUpdate)
}

func LinkTag() {
	TagGroup := Server.Group("/tag")
	TagLoginGroup := LoginGroup.Group("/tag")

	TagGroup.GET("/suggest", api2.SuggestTags)
	TagGroup.GET("/list", api2.TagsList)
	TagGroup.GET("/list/full", api2.FullTagsList)
	TagLoginGroup.POST("/create", api2.NewTag)
	TagLoginGroup.POST("/update/name", api2.UpdateTagName)
	TagLoginGroup.POST("/update/desc", api2.UpdateTagDesc)
}

func LinkHistory() {
	HistoryLoginGroup := LoginGroup.Group("/history")

	HistoryLoginGroup.GET("/list", api2.GetHistories)
	HistoryLoginGroup.POST("/create", api2.CreateHistory)
}

func LinkFollow() {
	FollowLoginGroup := LoginGroup.Group("/follow")

	FollowLoginGroup.GET("/followers", api2.GetFollowers)
	FollowLoginGroup.GET("/followings", api2.GetFollowings)

	FollowLoginGroup.POST("/follow", api2.CreateFollow)
	FollowLoginGroup.POST("/unfollow", api2.UnFollow)
}

func LinkComment() {
	CommentGroup := Server.Group("/comment")
	CommentLoginGroup := LoginGroup.Group("/comment")

	CommentGroup.GET("/list", api2.GetCommentsByArticleID)
	CommentLoginGroup.GET("/list/l", api2.GetCommentsByArticleIDWithLiked)
	CommentLoginGroup.POST("/create", api2.CreateComment)
	CommentLoginGroup.POST("/delete", api2.DeleteComment)
	CommentLoginGroup.POST("/like", api2.CreateCommentLike)
	CommentLoginGroup.POST("/dislike", api2.DeleteCommentLike)
}

func LinkCollection() {
	CollectionLoginGroup := LoginGroup.Group("/collection")

	CollectionLoginGroup.POST("/create", api2.CreateCollection)
	CollectionLoginGroup.POST("/delete", api2.DeleteCollection)
	CollectionLoginGroup.POST("/list", api2.GetCollections)
	CollectionLoginGroup.POST("/check", api2.CheckCollectionArticle)
}

func LinkCategory() {
	CategoryGroup := Server.Group("/category")
	CategoryLoginGroup := LoginGroup.Group("/category")

	CategoryGroup.GET("/list", api2.GetCategories)
	CategoryGroup.GET("/get", api2.GetCategory)
	CategoryGroup.GET("/get/fulllist", api2.GetCategoryList)

	CategoryLoginGroup.POST("/create", api2.CreateCategory)
	CategoryLoginGroup.POST("/update/name", api2.UpdateCategoryName)
	CategoryLoginGroup.POST("/update/desc", api2.UpdateCategoryDesc)

}

func LinkArticle() {
	ArticleGroup := Server.Group("/article")
	ArticleLoginGroup := LoginGroup.Group("/article")

	ArticleGroup.GET("/get/author", api2.GetArticleByAuthor)
	ArticleGroup.GET("/get/title", api2.GetArticlesByTitle)
	ArticleGroup.GET("/get/content", api2.GetArticlesByContent)
	ArticleGroup.GET("/get/collection", api2.GetArticlesByCollection)
	ArticleGroup.GET("/get/category", api2.GetArticlesByCategory)
	ArticleGroup.GET("/get/tag", api2.GetArticlesByTag)
	ArticleLoginGroup.GET("/get/self", api2.GetArticleSelf)
	ArticleGroup.GET("/get", api2.GetFullArticle)

	ArticleLoginGroup.POST("/create", api2.CreateArticle)
	ArticleLoginGroup.POST("/update/content", api2.UpdateArticleContent)
	ArticleLoginGroup.POST("/update/summary", api2.UpdateArticleSummary)
	ArticleLoginGroup.POST("/update/readcount", api2.UpdateArticleReadCount)
	ArticleLoginGroup.POST("/update/title", api2.UpdateArticleTitle)
	ArticleLoginGroup.POST("/update/status", api2.UpdateArticleStatus)
	ArticleLoginGroup.POST("/update/istop", api2.UpdateArticleIsTop)
	ArticleLoginGroup.POST("/delete", api2.DeleteArticle)

	ArticleLoginGroup.POST("/like/create", api2.CreateArticleLike)
	ArticleLoginGroup.POST("/like/delete", api2.DeleteArticleLike)
	ArticleLoginGroup.POST("/like", api2.ArticleLike)
	ArticleLoginGroup.POST("/like/check", api2.CheckArticleLike)

	ArticleLoginGroup.POST("/collection", api2.CollectionArticle)

	ArticleLoginGroup.POST("/tag/create", api2.CreateArticleTag)
	ArticleLoginGroup.POST("/tag/delete", api2.DeleteArticleTag)
}
