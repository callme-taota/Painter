package Request

type CategoryJSON struct {
	CategoryID  int    `json:"categoryID"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateCategoryNameJSON struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

type CategoryIDJSON struct {
	CategoryID int `json:"ID"`
}
