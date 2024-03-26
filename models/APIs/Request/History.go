package Request

type CreateHistoryJSON struct {
	ArticleID int `json:"articleID"`
}

type GetUserHistoryJSON struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
