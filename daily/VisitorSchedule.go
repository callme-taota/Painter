package daily

import (
	"painter-server-new/cache"
	"painter-server-new/database"
	"painter-server-new/models"
	"painter-server-new/tolog"
	"time"
)

func ScheduleDailyVisitorAggregation() {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		tolog.Log().Errorf("Failed to load location: %v", err).PrintAndWriteSafe()
		return
	}
	// 获取昨天的日期
	yesterday := time.Now().In(location).AddDate(0, 0, -1)
	yesterdayStr := yesterday.Format("2006-01-02")

	// 获取昨天的访客记录
	totalVisitors, err := cache.GetVisitorsByDate(yesterdayStr)
	if err != nil {
		tolog.Log().Errorf("Failed to get visitors for date %s: %v", yesterdayStr, err).PrintAndWriteSafe()
		return
	}

	// 创建访客统计数据结构
	visitorStats := models.VisitorRecordTable{
		Date:  yesterday,
		Total: totalVisitors,
	}

	// 将访客统计数据存入MySQL数据库
	err = database.SaveVisitorStats(visitorStats)
	if err != nil {
		tolog.Log().Errorf("Failed to save visitor stats for date %s: %v", yesterdayStr, err).PrintAndWriteSafe()
		return
	}

	tolog.Log().Infof("Visitor stats for date %s saved successfully. Total visitors: %d", yesterdayStr, totalVisitors).PrintAndWriteSafe()
}
