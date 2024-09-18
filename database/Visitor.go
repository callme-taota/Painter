package database

import (
	"painter-server-new/conf"
	"painter-server-new/models"
	"painter-server-new/tolog"
	"time"
)

func SaveVisitorStats(v models.VisitorRecordTable) error {
	res := DbEngine.Create(&v)
	if res.Error != nil {
		tolog.Log().Warningf("Can't saving visitor status %e", res.Error).PrintAndWriteSafe()
		return res.Error
	}
	return nil
}

func GetMonthlyVisitors() (int, error) {
	// Load the time zone
	timezone := conf.Server.Timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return 0, err
	}

	// Get the current year and month
	year, month, _ := time.Now().In(loc).Date()

	// Build the query conditions
	startDate := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	endDate := startDate.AddDate(0, 1, 0)

	var total int64
	DbEngine.Model(&models.VisitorRecordTable{}).
		Where("date >= ? AND date < ?", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")).
		Select("COALESCE(SUM(total), 0)").
		Scan(&total)
	return int(total), nil
}

func GetPreDayVisitors() (int, error) {
	// Load the time zone
	timezone := conf.Server.Timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return 0, err
	}

	// Get the current year, month, and day
	year, month, day := time.Now().In(loc).Date()

	// Calculate the date of the previous day
	currentDay := time.Date(year, month, day, 0, 0, 0, 0, loc)
	previousDay := currentDay.AddDate(0, 0, -1)

	// Construct the query conditions
	startDate := time.Date(previousDay.Year(), previousDay.Month(), previousDay.Day(), 0, 0, 0, 0, loc)
	endDate := time.Date(year, month, day, 0, 0, 0, 0, loc)

	var total int64
	DbEngine.Model(&models.VisitorRecordTable{}).
		Where("date >= ? AND date < ?", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")).
		Select("COALESCE(SUM(total), 0)").
		Scan(&total)
	return int(total), nil
}
