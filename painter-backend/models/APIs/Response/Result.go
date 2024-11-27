package Response

import "github.com/callme-taota/painter/painter-backend/models"

type CategoryWithCount struct {
	models.CategoryTable
	ArticleCount int
}

type TagWithCount struct {
	models.TagTable
	ArticleCount int
}
