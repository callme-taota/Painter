package Request

type CreateCommentJSON struct {
	ArticleID int    `json:"articleID"`
	Content   string `json:"content"`
}

type CommentJSON struct {
	CommentID int `json:"commentID"`
}

type GetCommentJSON struct {
	ArticleID int `json:"articleID"`
	Limit     int `json:"limit"`
	Offset    int `json:"offset"`
}
