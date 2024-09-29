package cache

import (
	"fmt"
	"painter-server-new/conf"
	"painter-server-new/models"

	"time"

	"github.com/callme-taota/tolog"
)

func AddVisRecord2Set(record models.VisitorRecord) error {
	timezone := conf.Server.Timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return err
	}

	serverName := conf.Server.Name
	todayKey := fmt.Sprintf("%s-visitors-%s", serverName, time.Now().In(loc).Format("2006-01-02"))
	flag := CheckRecordExistInSet(record)
	if flag {
		return nil
	}
	visitorKey := fmt.Sprintf("%s:%s", record.UA, record.IP)
	tolog.Infof("New visitor : %s", visitorKey).PrintAndWriteSafe()

	added, err := RedisClient.SAdd(todayKey, visitorKey).Result()
	if err != nil {
		tolog.Warningf("Failed to add visitor record to set: %v", err).PrintAndWriteSafe()
		return err
	}

	if added == 0 {
		tolog.Infof("Visitor record already exists").PrintAndWriteSafe()
	}

	// Set expiration time for the key
	expiration := 48 * time.Hour
	_, err = RedisClient.Expire(todayKey, expiration).Result()
	if err != nil {
		tolog.Warningf("Failed to set expiration for key: %v", err).PrintAndWriteSafe()
		return err
	}

	return nil
}

func CheckRecordExistInSet(record models.VisitorRecord) bool {
	timezone := conf.Server.Timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return false
	}

	serverName := conf.Server.Name
	todayKey := fmt.Sprintf("%s-visitors-%s", serverName, time.Now().In(loc).Format("2006-01-02"))
	visitorKey := fmt.Sprintf("%s:%s", record.UA, record.IP)

	exists, err := RedisClient.SIsMember(todayKey, visitorKey).Result()
	if err != nil {
		tolog.Warningf("Failed to check visitor record existence: %v", err).PrintAndWriteSafe()
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
