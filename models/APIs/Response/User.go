package Response

import "painter-server-new/models"

type FullUser struct {
	ArticleNumber    int
	ArticleList      []models.ArticleTable
	CollectionNumber int
	FollowingNumber  int
	FollowerNumber   int
	UserInfo         models.UserTable
}
