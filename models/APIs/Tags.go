package APIs

type SuggestionTagsJSON struct {
	Limit  int `json:"limit"`
	OffSet int `json:"offset"`
}

type CreateTagJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateTagNameJSON struct {
	TagID int    `json:"tagID"`
	Name  string `json:"name"`
}

type UpdateTagDescJSON struct {
	TagID       int    `json:"tagID"`
	Description string `json:"description"`
}
