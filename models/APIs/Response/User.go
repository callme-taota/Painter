package Response

import (
	"painter-server-new/models"
	"time"
)

type SelfFullUser struct {
	ArticleNumber    int
	ArticleList      []models.ArticleTable
	CollectionNumber int
	FollowingNumber  int
	FollowerNumber   int
	UserInfo         models.UserTable
}

type MiniUserInfo struct {
	ID          int
	Email       string
	NickName    string
	HeaderField string
	CreatedAt   time.Time
}

type FullUser struct {
	ArticleNumber    int
	ArticleList      []ArticleInfo
	CollectionNumber int
	FollowingNumber  int
	FollowerNumber   int
	Following        bool
	UserInfo         MiniUserFullInfo
}

type MiniUserFullInfo struct {
	ID          int
	Email       string
	NickName    string
	HeaderField string
	CreatedAt   time.Time
	LastLogin   time.Time
}
