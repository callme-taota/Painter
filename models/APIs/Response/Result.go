package Response

import "painter-server-new/models"

type CategoryWithCount struct {
	models.CategoryTable
	ArticleCount int
}

type TagWithCount struct {
	models.TagTable
	ArticleCount int
}
