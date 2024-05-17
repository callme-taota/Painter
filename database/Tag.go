package database

import (
	"painter-server-new/models"
	"painter-server-new/models/APIs/Response"
	"painter-server-new/tolog"
)

func CreateTag(name, description string) (int, error) {
	tag := models.TagTable{
		TagName:     name,
		Description: description,
	}
	err := DbEngine.Create(&tag).Error
	if err != nil {
		tolog.Log().Infof("Error while create tag %e", err).PrintAndWriteSafe()
		return -1, err
	}
	id := tag.TagID
	return id, nil
}

func UpdateTagName(id int, name string) error {
	tag := &models.TagTable{}
	res := DbEngine.First(&tag, id)
	if res.Error != nil {
		tolog.Log().Infof("Error while update tag name %e", res.Error)
		return res.Error
	}
	tag.TagName = name
	res = DbEngine.Save(&tag)
	if res.Error != nil {
		tolog.Log().Infof("Error while update tag name %e", res.Error)
		return res.Error
	}
	return nil
}

func UpdateTagDesc(id int, description string) error {
	tag := &models.TagTable{}
	res := DbEngine.First(&tag, id)
	if res.Error != nil {
		tolog.Log().Infof("Error while update tag description %e", res.Error)
		return res.Error
	}
	tag.Description = description
	res = DbEngine.Save(&tag)
	if res.Error != nil {
		tolog.Log().Infof("Error while update tag description %e", res.Error)
		return res.Error
	}
	return nil
}

func UpdateTag(id int, name, description string) error {
	tag := &models.TagTable{}
	res := DbEngine.First(&tag, id)
	if res.Error != nil {
		tolog.Log().Infof("Error while update tag description %e", res.Error)
		return res.Error
	}
	tag.Description = description
	tag.TagName = name
	res = DbEngine.Where("tag_id = ?", id).Save(&tag)
	if res.Error != nil {
		tolog.Log().Infof("Error while update tag description %e", res.Error)
		return res.Error
	}
	return nil
}
func CheckTagExist(name string) bool {
	var tag models.TagTable
	result := DbEngine.Where("tag_name = ?", name).First(&tag)
	return result.RowsAffected > 0
}

func GetTagID(name string) (int, error) {
	var tag models.TagTable
	result := DbEngine.Where("tag_name = ?", name).First(&tag)
	if result.Error != nil {
		tolog.Log().Infof("Error while get tag id %e", result.Error)
		return -1, result.Error
	}
	return tag.TagID, nil
}

func GetTag(id int) (models.TagTable, error) {
	var tag models.TagTable
	result := DbEngine.First(&tag, id)
	if result.Error != nil {
		return tag, result.Error
	}
	return tag, nil
}

func GetTags(limit, offset int) ([]models.TagTable, error) {
	var tags []models.TagTable
	result := DbEngine.Limit(limit).Offset(offset).Find(&tags)
	if result.Error != nil {
		return nil, result.Error
	}
	return tags, nil
}

func GetTagsWithCount(limit, offset int) ([]Response.TagWithCount, error) {
	var tags []models.TagTable
	var tagsWithCount []Response.TagWithCount

	result := DbEngine.Limit(limit).Offset(offset).Find(&tags)
	if result.Error != nil {
		return nil, result.Error
	}

	for _, tag := range tags {
		var count int64
		DbEngine.Model(&models.ArticleTagTable{}).Where("tag_id = ?", tag.TagID).Count(&count)

		tagWithCount := Response.TagWithCount{
			TagTable:     tag,
			ArticleCount: int(count),
		}
		tagsWithCount = append(tagsWithCount, tagWithCount)
	}

	return tagsWithCount, nil
}

func GetTagTotalNumber() int {
	var count int64
	DbEngine.Model(&models.TagTable{}).Count(&count)
	return int(count)
}

func GetTagListByArticleTagTable(articleTagTable []models.ArticleTagTable) ([]models.TagTable, error) {
	var tags []models.TagTable
	for _, item := range articleTagTable {
		tag, err := GetTag(item.TagID)
		if err != nil {
			tolog.Log().Infof("Error while GetTagListByArticleTagTable %e ", err).PrintAndWriteSafe()
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}
