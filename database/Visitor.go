package database

import (
	"painter-server-new/models"
	"painter-server-new/tolog"
	"time"
)

func SaveVisitorStats(v models.VisitorRecordTable) error {
	res := Dbengine.Create(&v)
	if res.Error != nil {
		tolog.Log().Warningf("Can't saving visitor status %e", res.Error).PrintAndWriteSafe()
		return res.Error
	}
	return nil
}

func GetMonthlyVisitors() (int, error) {
	// Get the current year and month
	year, month, _ := time.Now().Date()

	// Build the query conditions
	startDate := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	endDate := startDate.AddDate(0, 1, 0)

	var total int64
	res := Dbengine.Model(&models.VisitorRecordTable{}).
		Where("date >= ? AND date < ?", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")).
		Select("SUM(total)").
		Scan(&total)
	if res.Error != nil {
		tolog.Log().Warningf("Failed to get monthly visitors total: %v", res.Error).PrintAndWriteSafe()
		return 0, res.Error
	}

	return int(total), nil
}

func GetPreDayVisitors() (int, error) {
	// Get the current year, month, and day
	year, month, day := time.Now().Date()

	// Calculate the date of the previous day
	previousDay := time.Date(year, month, day-1, 0, 0, 0, 0, time.Local)

	// Construct the query conditions
	startDate := time.Date(previousDay.Year(), previousDay.Month(), previousDay.Day(), 0, 0, 0, 0, time.Local)
	endDate := time.Date(year, month, day, 0, 0, 0, 0, time.Local)

	var total int64
	res := Dbengine.Model(&models.VisitorRecordTable{}).
		Where("date >= ? AND date < ?", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")).
		Select("SUM(total)").
		Scan(&total)
	if res.Error != nil {
		tolog.Log().Warningf("Failed to get previous day's visitors total: %v", res.Error).PrintAndWriteSafe()
		return 0, res.Error
	}

	return int(total), nil
}
