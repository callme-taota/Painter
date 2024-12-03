package cache

import (
	"fmt"

	"github.com/callme-taota/painter/painter-backend/conf"
	"github.com/callme-taota/painter/painter-backend/models"

	"time"

	"github.com/callme-taota/tolog"
)

func AddVisRecord2Set(record models.VisitorRecord) error {
	timezone := conf.Conf.Server.Timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return err
	}

	serverName := conf.Conf.Server.Name
	todayKey := fmt.Sprintf("%s-visitors-%s", serverName, time.Now().In(loc).Format("2006-01-02"))
	flag := CheckRecordExistInSet(record)
	if flag {
		return nil
	}
	visitorKey := fmt.Sprintf("%s:%s", record.UA, record.IP)
	tolog.Infof("New visitor : %s", visitorKey).PrintAndWriteSafe()

	added, err := Cache.client.SAdd(todayKey, visitorKey).Result()
	if err != nil {
		tolog.Warningf("Failed to add visitor record to set: %v", err).PrintAndWriteSafe()
		return err
	}

	if added == 0 {
		tolog.Infof("Visitor record already exists").PrintAndWriteSafe()
	}

	// Set expiration time for the key
	expiration := 48 * time.Hour
	_, err = Cache.client.Expire(todayKey, expiration).Result()
	if err != nil {
		tolog.Warningf("Failed to set expiration for key: %v", err).PrintAndWriteSafe()
		return err
	}

	return nil
}

func CheckRecordExistInSet(record models.VisitorRecord) bool {
	timezone := conf.Conf.Server.Timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return false
	}

	serverName := conf.Conf.Server.Name
	todayKey := fmt.Sprintf("%s-visitors-%s", serverName, time.Now().In(loc).Format("2006-01-02"))
	visitorKey := fmt.Sprintf("%s:%s", record.UA, record.IP)

	exists, err := Cache.client.SIsMember(todayKey, visitorKey).Result()
	if err != nil {
		tolog.Warningf("Failed to check visitor record existence: %v", err).PrintAndWriteSafe()
		return false
	}

	return exists
}

func GetVisitorsByDate(date string) (int, error) {
	serverName := conf.Conf.Server.Name
	key := fmt.Sprintf("%s-visitors-%s", serverName, date)

	members, err := Cache.client.SMembers(key).Result()
	if err != nil {
		return -1, err
	}

	return len(members), nil
}
