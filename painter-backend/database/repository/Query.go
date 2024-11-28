package repository

import (
	"errors"
	"reflect"

	"gorm.io/gorm"
)

type Table interface {
	Migrate(db *gorm.DB) error
	TableName() string

	// CRUD
	Create(db *gorm.DB, row Table) (*gorm.DB, error)
	Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error
	Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error
	Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error)

	BaseTable
}

type BaseTable interface {
	// GetValue retrieves the value of a specific column by name.
	GetValue(column string) (interface{}, error)

	// SetValue sets the value of a specific column by name.
	SetValue(column string, value interface{}) error

	// SetValues sets the value of some column by name.
	SetValues(values map[string]interface{}) error

	// GetColumns retrieves all column names for the row.
	GetColumns() []string

	// CheckColumnsExist check the columns exist in table or not.
	CheckColumnsExist(columnName string) bool

	// ToMap converts the row into a map of column names to values.
	ToMap() map[string]interface{}
}

type BaseTableImplement struct {
}

func (b *BaseTableImplement) GetValue(column string) (interface{}, error) {
	v := reflect.ValueOf(b).Elem()
	field := v.FieldByName(column)
	if !field.IsValid() {
		return nil, errors.New("unknown column")
	}
	return field.Interface(), nil
}

func (b *BaseTableImplement) SetValue(column string, value interface{}) error {
	v := reflect.ValueOf(b).Elem()
	field := v.FieldByName(column)
	if !field.IsValid() {
		return errors.New("unknown column")
	}
	if !field.CanSet() {
		return errors.New("field cannot be set")
	}

	val := reflect.ValueOf(value)
	if !val.Type().AssignableTo(field.Type()) {
		return errors.New("value type mismatch")
	}

	field.Set(val)
	return nil
}

func (b *BaseTableImplement) SetValues(values map[string]interface{}) error {
	errs := errors.Join()
	for k, v := range values {
		err := b.SetValue(k, v)
		if err != nil {
			errors.Join(errs, err)
		}
	}
	return nil
}

func (b *BaseTableImplement) GetColumns() []string {
	t := reflect.TypeOf(*b)
	columns := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		columns = append(columns, t.Field(i).Name)
	}
	return columns
}

func (b *BaseTableImplement) CheckColumnsExist(columnName string) bool {
	t := reflect.TypeOf(*b)
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Name == columnName {
			return true
		}
	}
	return false
}

func (b *BaseTableImplement) ToMap() map[string]interface{} {
	v := reflect.ValueOf(b).Elem()
	t := v.Type()
	result := make(map[string]interface{})
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		result[field.Name] = v.Field(i).Interface()
	}
	return result
}
