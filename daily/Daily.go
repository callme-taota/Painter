package daily

import "time"

func InitDaily() error {
	ticker := time.Tick(1 * time.Hour)
	done := make(chan bool)
	for {
		select {
		case <-ticker:
			if time.Now().Hour() == 1 {
				ScheduleDailyVisitorAggregation()
			}
		case <-done:
			return nil
		}
	}
	return nil
}
