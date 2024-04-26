package Response

import "painter-server-new/models"

type FullArticle struct {
	ArticleTable        models.ArticleTable
	ArticleTagTable     []models.TagTable
	ArticleContentTable models.ArticleContentTable
	User                MiniUserInfo
	LikesNumber         int
	CollectionNumber    int
	CommentNumber       int
	CategoryName        string
	Liked               bool
	Collected           bool
}

type ArticleInfo struct {
	ArticleTable     models.ArticleTable
	ArticleTagTable  []models.TagTable
	LikesNumber      int
	CollectionNumber int
	CommentNumber    int
}
