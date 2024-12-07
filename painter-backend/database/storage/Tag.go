package storage

import (
	"github.com/callme-taota/painter/painter-backend/database/repository"
)

func CreateTag(name, description string) (int, error) {
	_, id, err := CreatorWithIDReturn(repository.TagTableName, dbValue{
		"TagName":     name,
		"Description": description,
	}, "TagID")
	return id, err
}

func UpdateTag(id int, name, description string) error {
	err := Updater(repository.TagTableName,
		dbValue{
			"tag_id": id,
		},
		dbValue{
			"TagName":     name,
			"Description": description,
		})
	if err != nil {
		return err
	}
	return nil
}
