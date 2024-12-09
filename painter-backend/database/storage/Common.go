package storage

import (
	"errors"
	"strings"

	"github.com/callme-taota/painter/painter-backend/database/repository"
	"gorm.io/gorm"
)

type dbValue map[string]interface{}

func PageFilter(tableName string, limit, offset int, cause ...dbValue) ([]repository.Table, error) {
	tb := repository.GetDBImplement().UseTable(tableName)
	if tb == nil {
		return nil, errors.New("can't find table")
	}

	dbI := repository.GetDBImplement().GetDB()
	if dbI == nil {
		return nil, errors.New("can't find db")
	}

	query, values := genQueryClause(cause...)

	res, _, err := tb.Select(dbI, func(db *gorm.DB) *gorm.DB {
		return db.Offset(offset).Limit(limit).Where(query, values)
	})
	return res, err
}

func PageFilterWithCount(tableName string, limit, offset int, cause ...dbValue) ([]repository.Table, int64, error) {
	dbI := repository.GetDBImplement()

	tb := repository.GetDBImplement().UseTable(tableName)
	if tb == nil {
		return nil, 0, errors.New("can't find table")
	}

	db := repository.GetDBImplement().GetDB()
	if dbI == nil {
		return nil, 0, errors.New("can't find db")
	}

	query, values := genQueryClause(cause...)

	res, _, err := tb.Select(db, func(db *gorm.DB) *gorm.DB {
		return db.Offset(offset).Limit(limit).Where(query, values)
	})

	var count int64
	db.Model(dbI.UseTable(tableName)).Count(&count)

	return res, count, err
}

func Counter(tableName string, cause ...dbValue) (int64, error) {
	dbI := repository.GetDBImplement()

	db := repository.GetDBImplement().GetDB()
	if dbI == nil {
		return 0, errors.New("can't find db")
	}

	query, values := genQueryClause(cause...)

	var count int64
	err := db.Model(dbI.UseTable(tableName)).Where(query, values).Count(&count).Error

	return count, err
}

func Creator(tableName string, kv dbValue) (*gorm.DB, int, error) {
	tx := repository.GetDBImplement().GetTransaction()
	tb := repository.GetDBImplement().UseTable(tableName)
	row := repository.GetDBImplement().TableConstructor(tableName)

	err := row.SetValues(kv, row)
	if err != nil {
		tx.Rollback()
		return tx, -1, err
	}

	db, err := tb.Create(tx, row)
	if err != nil {
		return db, -1, err
	}
	err = tx.Commit().Error
	return db, int(db.RowsAffected), err
}

func CreatorWithIDReturn(tableName string, kv dbValue, autoIncrementColumn ...string) (*gorm.DB, int, error) {
	tx := repository.GetDBImplement().GetTransaction()
	tb := repository.GetDBImplement().UseTable(tableName)
	row := repository.GetDBImplement().TableConstructor(tableName)

	err := row.SetValues(kv, row)
	if err != nil {
		tx.Rollback()
		return tx, -1, err
	}

	db, err := tb.Create(tx, row)
	if err != nil {
		return db, -1, err
	}
	err = tx.Commit().Error

	idColumn := "id"
	if len(autoIncrementColumn) == 1 {
		idColumn = autoIncrementColumn[0]
	}

	idI, err := row.GetValue(idColumn, row)
	if err != nil {
		return nil, 0, err
	}
	id, ok := idI.(int)
	if !ok {
		return nil, 0, errors.New("can't parse id to int, please check id type")
	}

	return db, id, err
}

func Updater(tableName string, cause, updateRow dbValue) error {
	dbI := repository.GetDBImplement()
	tx := dbI.GetTransaction()
	tb := dbI.UseTable(tableName)

	query, values := genQueryClause(cause)
	
	err := tb.Update(tx, func(db *gorm.DB) *gorm.DB {
		return db.Where(query, values)
	}, func(table repository.Table) error {
		return table.SetValues(updateRow, table)
	})

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func genQueryClause(cause ...dbValue) (string, []interface{}) {
	query := ""
	var values []interface{}
	if len(cause) == 1 {
		var keys []string
		for k, v := range cause[0] {
			keys = append(keys, k+" = ?")
			values = append(values, v)
		}
		query = strings.Join(keys, " ,")
	}
	return query, values
}
