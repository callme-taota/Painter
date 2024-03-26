package Request

type CreateArticleJSON struct {
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	CategoryID int    `json:"categoryID"`
	Content    string `json:"content"`
	Tags       []int  `json:"tags"`
}

type UpdateArticleJSON struct {
	ArticleID int    `json:"articleID"`
	Content   string `json:"content"`
	Summary   string `json:"summary"`
	ReadCount int    `json:"readCount"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	IsTop     bool   `json:"isTop"`
}

type ArticleJSON struct {
	ArticleID int `json:"articleID"`
}

type GetArticleJSON struct {
	Author     int    `json:"author"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	CategoryID int    `json:"categoryID"`
	TagID      int    `json:"tagID"`
	UserID     int    `json:"userID"`
}

type ArticleTagJSON struct {
	ArticleID int `json:"articleID"`
	TagID     int `json:"tagID"`
}
