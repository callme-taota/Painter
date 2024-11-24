package database

import "gorm.io/gorm"

type Table interface {
	Migrate(db *gorm.DB) error
	TableName() string

	Create(db *gorm.DB, row Row) error
	Update(db *gorm.DB, condition func(Row) bool, updater func(Row) error) error
	Delete(db *gorm.DB, condition func(Row) bool) error
	Select(db *gorm.DB, condition func(Row) bool) ([]Row, error)
}

// Row interface defines operations for a single row in a table.
type Row interface {
	// GetValue retrieves the value of a specific column by name.
	GetValue(column string) (interface{}, error)

	// SetValue sets the value of a specific column by name.
	SetValue(column string, value interface{}) error

	// GetColumns retrieves all column names for the row.
	GetColumns() []string

	// ToMap converts the row into a map of column names to values.
	ToMap() map[string]interface{}
}
