package repository

import "gorm.io/gorm"

func updateRows(table Table, db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error {
	rows, _, err := table.Select(db, query)
	if err != nil {
		return err
	}

	for _, row := range rows {
		if err := updater(row); err != nil {
			return err
		}
		if err := db.Save(row).Error; err != nil {
			return err
		}
	}
	return nil
}

func deleteRows[T any](db *gorm.DB, query func(*gorm.DB) *gorm.DB) error {
	var rows []T
	if err := query(db).Find(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		if err := db.Delete(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
