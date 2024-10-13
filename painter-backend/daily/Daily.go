package daily

import (
	"time"

	"github.com/callme-taota/tolog"
)

func InitDaily() error {
	ticker := time.NewTicker(1 * time.Hour)
	done := make(chan bool)
	tolog.Infof("Init daily schedule").PrintAndWriteSafe()
	for {
		select {
		case <-ticker.C:
			if time.Now().Hour() == 1 {
				ScheduleDailyVisitorAggregation()
			}
			UserLastLoginUpdate()
		case <-done:
			return nil
		}
	}
	return nil
}
