package APIs

type CreateCommentJSON struct {
	ArticleID int    `json:"articleID"`
	Content   string `json:"content"`
}

type CommentJSON struct {
	CommentID int `json:"commentID"`
}
