package repository

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func newCRUDTestDB(t *testing.T, rows []Category, queryErr error) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(localhost:9910)/gorm",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun:                 true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	err = db.Callback().Query().Replace("gorm:query", func(tx *gorm.DB) {
		if queryErr != nil {
			tx.AddError(queryErr)
			return
		}
		*tx.Statement.Dest.(*[]Category) = append([]Category(nil), rows...)
		tx.RowsAffected = int64(len(rows))
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestUpdateRowsPreservesMutationsAndQueryScope(t *testing.T) {
	db := newCRUDTestDB(t, []Category{{CategoryID: 1}, {CategoryID: 2}}, nil)
	var saved []Category
	if err := db.Callback().Update().After("gorm:update").Register("test:capture", func(tx *gorm.DB) {
		saved = append(saved, *tx.Statement.Dest.(*Category))
		if sql := tx.Statement.SQL.String(); strings.Contains(sql, "only_this_query") || strings.Contains(sql, "LIMIT") {
			t.Errorf("query scope leaked into save: %s", sql)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Query().After("gorm:query").Register("test:scope", func(tx *gorm.DB) {
		where := tx.Statement.Clauses["WHERE"].Expression.(clause.Where)
		expr := where.Exprs[0].(clause.Expr)
		if expr.SQL != "only_this_query = ?" || !reflect.DeepEqual(expr.Vars, []interface{}{"chosen"}) {
			t.Errorf("query lost filter: %#v", where)
		}
		limit := tx.Statement.Clauses["LIMIT"].Expression.(clause.Limit)
		if limit.Limit == nil || *limit.Limit != 2 || limit.Offset != 3 {
			t.Errorf("query lost pagination: %#v", limit)
		}
	}); err != nil {
		t.Fatal(err)
	}

	queryCalls := 0
	err := NewEmptyCategory().Update(db, func(tx *gorm.DB) *gorm.DB {
		queryCalls++
		return tx.Where("only_this_query = ?", "chosen").Limit(2).Offset(3)
	}, func(row Table) error {
		return row.SetValue("CategoryName", "updated", row)
	})
	if err != nil {
		t.Fatal(err)
	}
	if queryCalls != 1 || len(saved) != 2 {
		t.Fatalf("query calls = %d, saved rows = %d", queryCalls, len(saved))
	}
	for i, row := range saved {
		if row.CategoryID != i+1 || row.CategoryName != "updated" {
			t.Errorf("saved row %d = %#v", i, row)
		}
	}
}

func TestUpdateRowsStopsOnErrors(t *testing.T) {
	failure := errors.New("operation failed")
	rows := []Category{{CategoryID: 1}, {CategoryID: 2}, {CategoryID: 3}}
	for _, tc := range []struct {
		name       string
		rows       []Category
		queryErr   error
		updaterErr int
		saveErr    int
		wantCalls  []int
		wantSaves  []int
		wantErr    error
	}{
		{name: "empty"},
		{name: "query", rows: rows, queryErr: failure, wantErr: failure},
		{name: "updater", rows: rows, updaterErr: 2, wantCalls: []int{1, 2}, wantSaves: []int{1}, wantErr: failure},
		{name: "save", rows: rows, saveErr: 2, wantCalls: []int{1, 2}, wantSaves: []int{1, 2}, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newCRUDTestDB(t, tc.rows, tc.queryErr)
			var calls, saves []int
			if err := db.Callback().Update().After("gorm:update").Register("test:save", func(tx *gorm.DB) {
				id := tx.Statement.Dest.(*Category).CategoryID
				saves = append(saves, id)
				if id == tc.saveErr {
					tx.AddError(failure)
				}
			}); err != nil {
				t.Fatal(err)
			}
			err := NewEmptyCategory().Update(db, func(tx *gorm.DB) *gorm.DB { return tx }, func(row Table) error {
				id := row.(*Category).CategoryID
				calls = append(calls, id)
				if id == tc.updaterErr {
					return failure
				}
				return nil
			})
			if !errors.Is(err, tc.wantErr) || !reflect.DeepEqual(calls, tc.wantCalls) || !reflect.DeepEqual(saves, tc.wantSaves) {
				t.Fatalf("error = %v, updater calls = %v, save attempts = %v", err, calls, saves)
			}
		})
	}
}

func TestDeleteRowsPreservesOrderAndStopsOnErrors(t *testing.T) {
	failure := errors.New("operation failed")
	rows := []Category{{CategoryID: 1}, {CategoryID: 2}, {CategoryID: 3}}
	for _, tc := range []struct {
		name      string
		rows      []Category
		queryErr  error
		deleteErr int
		want      []int
		wantErr   error
	}{
		{name: "empty"},
		{name: "query", rows: rows, queryErr: failure, wantErr: failure},
		{name: "all", rows: rows, want: []int{1, 2, 3}},
		{name: "delete", rows: rows, deleteErr: 2, want: []int{1, 2}, wantErr: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newCRUDTestDB(t, tc.rows, tc.queryErr)
			var deleted []int
			if err := db.Callback().Delete().After("gorm:delete").Register("test:delete", func(tx *gorm.DB) {
				id := tx.Statement.Dest.(*Category).CategoryID
				deleted = append(deleted, id)
				if strings.Contains(tx.Statement.SQL.String(), "only_this_query") {
					t.Error("query filter leaked into delete")
				}
				if id == tc.deleteErr {
					tx.AddError(failure)
				}
			}); err != nil {
				t.Fatal(err)
			}
			queryCalls := 0
			err := NewEmptyCategory().Delete(db, func(tx *gorm.DB) *gorm.DB {
				queryCalls++
				return tx.Where("only_this_query = ?", "chosen")
			})
			if !errors.Is(err, tc.wantErr) || !reflect.DeepEqual(deleted, tc.want) || queryCalls != 1 {
				t.Fatalf("error = %v, delete attempts = %v, query calls = %d", err, deleted, queryCalls)
			}
		})
	}
}
