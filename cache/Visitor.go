package cache

import (
	"fmt"
	"painter-server-new/conf"
	"painter-server-new/models"
	"painter-server-new/tolog"
	"time"
)

func AddVisRecord2Set(record models.VisitorRecord) error {
	serverName := conf.Server.Name
	todayKey := fmt.Sprintf("%s-visitors-%s", serverName, time.Now().Format("2006-01-02"))
	flag := CheckRecordExistInSet(record)
	if flag {
		return nil
	}
	visitorKey := fmt.Sprintf("%s:%s", record.UA, record.IP)
	tolog.Log().Infof("New visitor : %s", visitorKey).PrintAndWriteSafe()

	added, err := RedisClient.SAdd(todayKey, visitorKey).Result()
	if err != nil {
		tolog.Log().Warningf("Failed to add visitor record to set: %v", err).PrintAndWriteSafe()
		return err
	}

	if added == 0 {
		tolog.Log().Info("Visitor record already exists").PrintAndWriteSafe()
	}

	return nil
}

func CheckRecordExistInSet(record models.VisitorRecord) bool {
	serverName := conf.Server.Name
	todayKey := fmt.Sprintf("%s-visitors-%s", serverName, time.Now().Format("2006-01-02"))
	visitorKey := fmt.Sprintf("%s:%s", record.UA, record.IP)

	exists, err := RedisClient.SIsMember(todayKey, visitorKey).Result()
	if err != nil {
		tolog.Log().Warningf("Failed to check visitor record existence: %v", err).PrintAndWriteSafe()
		return false
	}

	return exists
}

func GetVisitorsByDate(date string) (int, error) {
	serverName := conf.Server.Name
	key := fmt.Sprintf("%s-visitors-%s", serverName, date)

	members, err := RedisClient.SMembers(key).Result()
	if err != nil {
		return -1, err
	}

	return len(members), nil
}
