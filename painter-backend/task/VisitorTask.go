package task

import (
	"errors"
	"time"

	"github.com/callme-taota/painter/painter-backend/cache"
	"github.com/callme-taota/painter/painter-backend/database"
	"github.com/callme-taota/painter/painter-backend/models"
	"github.com/callme-taota/tolog"
)

type visitorTask struct {
	timeZone string
}

func newVisitorTask() *visitorTask {
	return &visitorTask{}
}

func (v *visitorTask) taskFn() func() {
	taskFn := func() {
		location, err := time.LoadLocation(v.timeZone)
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
	return taskFn
}

func (v *visitorTask) taskName() string {
	return "visitorTask"
}

func (v *visitorTask) duration() time.Duration {
	return time.Hour * 24
}

func (v *visitorTask) withValue(args ...interface{}) error {
	if len(args) != 1 {
		return errors.New("with wrong value")
	}
	if timezone, ok := args[0].(string); ok {
		v.timeZone = timezone
		return nil
	}
	return errors.New("with wrong value")
}
