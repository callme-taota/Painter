package database

import (
	"github.com/callme-taota/painter/painter-backend/database/repository"
	"github.com/callme-taota/painter/painter-backend/models"

	"github.com/callme-taota/tolog"
	"gorm.io/gorm"

	"time"
)

func SaveVisitorStats(v models.VisitorRecordTable) error {
	return SaveVisitorStatsV2(repository.GetDBImplement(), repository.VisitorRecord{
		BaseTableImplement: repository.BaseTableImplement{},
		Date:               v.Date,
		Total:              v.Total,
	})
}

func SaveVisitorStatsV2(db *repository.DBImplement, v repository.VisitorRecord) error {
	tx := db.GetTransaction()
	tb := db.UseTable(repository.VisitorRecordTableName)
	_, err := tb.Create(tx, &v)
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

func GetVisitors(y, m, d int) (int, error) {
	db := repository.GetDBImplement()
	timeZone := db.Config.Server.Timezone
	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		tolog.Infof("Location load error: %v", err)
		return 0, err
	}

	// Get the current year and month
	year, month, _ := time.Now().In(loc).Date()

	// Build the query conditions
	startDate := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	endDate := startDate.AddDate(y*-1, m*-1, d*-1)
	var total int64
	tx := db.GetTransaction()
	tb := db.UseTable(repository.VisitorRecordTableName)
	_, _, err = tb.Select(tx, func(g *gorm.DB) *gorm.DB {
		return g.Where("date >= ? AND date < ?", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")).
			Select("COALESCE(SUM(total), 0)").
			Scan(&total)
	})
	if err != nil {
		return 0, err
	}
	return int(total), nil
}

func GetMonthlyVisitors() (int, error) {
	return GetVisitors(0, 1, 0)
}

func GetPreDayVisitors() (int, error) {
	return GetVisitors(0, 0, 1)
}
