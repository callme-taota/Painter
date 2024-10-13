package daily

import (
	"painter-server-new/cache"
	"painter-server-new/conf"
	"painter-server-new/database"
	"painter-server-new/models"

	"time"

	"github.com/callme-taota/tolog"
)

func ScheduleDailyVisitorAggregation() {
	location, err := time.LoadLocation(conf.Server.Timezone)
	if err != nil {
		tolog.Errorf("Failed to load location: %v", err).PrintAndWriteSafe()
		return
	}
	// Get the preview date.
	yesterday := time.Now().In(location).AddDate(0, 0, -1)
	yesterdayStr := yesterday.Format("2006-01-02")

	tolog.Debugf("Current time: %s", time.Now().In(location).Format("2006-01-02 15:04:05")).PrintAndWriteSafe()
	tolog.Debugf("Calculated yesterday's date: %s", yesterdayStr).PrintAndWriteSafe()

	// Get yesterday visitor record.
	totalVisitors, err := cache.GetVisitorsByDate(yesterdayStr)
	if err != nil {
		tolog.Errorf("Failed to get visitors for date %s: %v", yesterdayStr, err).PrintAndWriteSafe()
		return
	}

	// Create table row.
	visitorStats := models.VisitorRecordTable{
		Date:  yesterday,
		Total: totalVisitors,
	}

	// Save row into db.
	err = database.SaveVisitorStats(visitorStats)
	if err != nil {
		tolog.Errorf("Failed to save visitor stats for date %s: %v", yesterdayStr, err).PrintAndWriteSafe()
		return
	}

	tolog.Infof("Visitor stats for date %s saved successfully. Total visitors: %d", yesterdayStr, totalVisitors).PrintAndWriteSafe()
}
