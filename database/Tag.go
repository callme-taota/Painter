package database

import (
	"painter-server-new/models"
	"painter-server-new/tolog"
)

func CreateTag(name, description string) (int, error) {
	tag := models.TagTable{
		TagName:     name,
		Description: description,
	}
	err := Dbengine.Create(&tag).Error
	if err != nil {
		tolog.Log().Infof("Error while create tag %e", err).PrintAndWriteSafe()
		return -1, err
	}
	id := tag.TagID
	return id, nil
}

func UpdateTagName(id int, name string) error {
	tag := &models.TagTable{}
	res := Dbengine.First(&tag, id)
	if res.Error != nil {
		tolog.Log().Infof("Error while update tag name %e", res.Error)
		return res.Error
	}
	tag.TagName = name
	res = Dbengine.Save(&tag)
	if res.Error != nil {
		tolog.Log().Infof("Error while update tag name %e", res.Error)
		return res.Error
	}
	return nil
}

func UpdateTagDesc(id int, description string) error {
	tag := &models.TagTable{}
	res := Dbengine.First(&tag, id)
	if res.Error != nil {
		tolog.Log().Infof("Error while update tag description %e", res.Error)
		return res.Error
	}
	tag.Description = description
	res = Dbengine.Save(&tag)
	if res.Error != nil {
		tolog.Log().Infof("Error while update tag description %e", res.Error)
		return res.Error
	}
	return nil
}

func CheckTagExist(name string) bool {
	var tag models.TagTable
	result := Dbengine.Where("tag_name = ?", name).First(&tag)
	return result.RowsAffected > 0
}

func GetTagID(name string) (int, error) {
	var tag models.TagTable
	result := Dbengine.Where("tag_name = ?", name).First(&tag)
	if result.Error != nil {
		tolog.Log().Infof("Error while get tag id %e", result.Error)
		return -1, result.Error
	}
	return tag.TagID, nil
}

func GetTag(id int) (models.TagTable, error) {
	var tag models.TagTable
	result := Dbengine.First(&tag, id)
	if result.Error != nil {
		return tag, result.Error
	}
	return tag, nil
}

func GetTags(limit, offset int) ([]models.TagTable, error) {
	var tags []models.TagTable
	result := Dbengine.Limit(limit).Offset(offset).Find(&tags)
	if result.Error != nil {
		return nil, result.Error
	}
	return tags, nil
}
