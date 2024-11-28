package Response

import "github.com/callme-taota/painter/painter-backend/models"

type FullComment struct {
	models.CommentTable
	NickName    string
	HeaderField string
	LikeCount   int
}

type FullCommentWithLike struct {
	models.CommentTable
	NickName    string
	HeaderField string
	LikeCount   int
	Liked       bool
	IsSelf      bool
}
