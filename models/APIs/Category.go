package APIs

type CategoryJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateCategoryNameJSON struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}
