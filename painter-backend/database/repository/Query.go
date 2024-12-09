package repository

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"
)

type Table interface {
	Migrate(db *gorm.DB) error
	TableName() string

	Constructor() Table

	// CRUD
	Create(db *gorm.DB, row Table) (*gorm.DB, error)
	Update(db *gorm.DB, query func(*gorm.DB) *gorm.DB, updater func(Table) error) error
	Delete(db *gorm.DB, query func(*gorm.DB) *gorm.DB) error
	Select(db *gorm.DB, query func(*gorm.DB) *gorm.DB) ([]Table, *gorm.DB, error)

	BaseTable
}

type BaseTable interface {
	// GetValue retrieves the value of a specific column by name.
	GetValue(column string, table Table) (interface{}, error)

	// SetValue sets the value of a specific column by name.
	SetValue(column string, value interface{}, table Table) error

	// SetValues sets the value of some column by name.
	SetValues(values map[string]interface{}, table Table) error

	// GetColumns retrieves all column names for the row.
	GetColumns(table Table) []string

	// CheckColumnsExist check the columns exist in table or not.
	CheckColumnsExist(columnName string, table Table) bool

	// ToMap converts the row into a map of column names to values.
	ToMap(table Table) map[string]interface{}
}

type BaseTableImplement struct {
}

func (b *BaseTableImplement) GetValue(column string, table Table) (interface{}, error) {
	v := reflect.ValueOf(table).Elem()
	field := v.FieldByName(column)
	if !field.IsValid() {
		return nil, errors.New("unknown column")
	}
	return field.Interface(), nil
}

func (b *BaseTableImplement) SetValue(column string, value interface{}, table Table) error {
	v := reflect.ValueOf(table).Elem()
	field, err := findFieldByNameOrTag(v, column)
	if err != nil {
		return err
	}
	if !field.CanSet() {
		return fmt.Errorf("field cannot be set: %s", column)
	}

	val := reflect.ValueOf(value)
	if !val.Type().AssignableTo(field.Type()) {
		convertedVal, err := tryConvertValue(val, field.Type())
		if err != nil {
			return fmt.Errorf("value type mismatch for column %s: %v", column, err)
		}
		val = convertedVal
	}

	field.Set(val)
	return nil
}

func (b *BaseTableImplement) SetValues(values map[string]interface{}, table Table) error {
	var errs []error
	for k, v := range values {
		err := b.SetValue(k, v, table)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...) // 合并所有错误
	}
	return nil
}

func (b *BaseTableImplement) GetColumns(table Table) []string {
	v := reflect.ValueOf(table).Elem()
	t := v.Type()

	columns := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		columns = append(columns, t.Field(i).Name)
	}

	return columns
}

func (b *BaseTableImplement) CheckColumnsExist(columnName string, table Table) bool {
	v := reflect.ValueOf(table).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Name == columnName {
			return true
		}
	}

	return false
}

func (b *BaseTableImplement) ToMap(table Table) map[string]interface{} {
	v := reflect.ValueOf(table).Elem()
	t := v.Type()
	result := make(map[string]interface{})

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		result[field.Name] = v.Field(i).Interface()
	}

	return result
}

func findFieldByNameOrTag(v reflect.Value, column string) (reflect.Value, error) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Name == column {
			return v.Field(i), nil
		}
		tag := field.Tag.Get("gorm")
		if tag != "" {
			tagParts := strings.Split(tag, ";")
			for _, part := range tagParts {
				if strings.HasPrefix(part, "column:") && strings.TrimPrefix(part, "column:") == column {
					return v.Field(i), nil
				}
			}
		}
		tag = field.Tag.Get("json")
		if tag != "" {
			tagParts := strings.Split(tag, ";")
			for _, part := range tagParts {
				if strings.HasPrefix(part, "column:") && strings.TrimPrefix(part, "column:") == column {
					return v.Field(i), nil
				}
			}
		}
	}
	return reflect.Value{}, fmt.Errorf("unknown column: %s", column)
}

func tryConvertValue(val reflect.Value, targetType reflect.Type) (reflect.Value, error) {
	if val.Kind() == reflect.Int && targetType.Kind() == reflect.Float64 {
		return reflect.ValueOf(float64(val.Int())), nil
	}
	if val.Kind() == reflect.Float64 && targetType.Kind() == reflect.Int {
		return reflect.ValueOf(int(val.Float())), nil
	}
	return reflect.Value{}, fmt.Errorf("unsupported conversion from %s to %s", val.Type(), targetType)
}
