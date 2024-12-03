package task

import (
	"time"

	"github.com/callme-taota/painter/painter-backend/cache"
	"github.com/callme-taota/painter/painter-backend/database"
	"github.com/callme-taota/tolog"
)

type userLastLoginUpdateTask struct {
}

func newUserLastLoginUpdateTask() *userLastLoginUpdateTask {
	return &userLastLoginUpdateTask{}
}

func (t *userLastLoginUpdateTask) taskFn() func() {
	taskFn := func() {
		userIDs, err := cache.GetPastHourLoginUserList()
		if err != nil {
			tolog.Infof("Error while UserLastLoginUpdate %e", err).PrintAndWriteSafe()
		}
		// update user last visit time
		for _, userID := range userIDs {
			ok, err := database.UpdateUserLoginTime(userID)
			if err != nil {
				tolog.Infof("Error while UserLastLoginUpdate %e %v", err, ok).PrintAndWriteSafe()
				return
			}
			tolog.Infof("Updating last request for user %s in MySQL", userID).PrintAndWriteSafe()
		}
	}
	return taskFn
}

func (t *userLastLoginUpdateTask) duration() time.Duration {
	return time.Minute * 10
}

func (t *userLastLoginUpdateTask) taskName() string {
	return "userLastLoginUpdateTask"
}

func (t *userLastLoginUpdateTask) withValue(args ...interface{}) error {
	return nil
}
