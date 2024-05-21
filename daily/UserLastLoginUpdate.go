package daily

import (
	"painter-server-new/cache"
	"painter-server-new/database"
	"painter-server-new/tolog"
)

func UserLastLoginUpdate() {
	userIDs, err := cache.GetPastHourLoginUserList()
	if err != nil {
		tolog.Log().Infof("Error while UserLastLoginUpdate %e", err).PrintAndWriteSafe()
	}
	// update user last visit time
	for _, userID := range userIDs {
		ok, err := database.UpdateUserLoginTime(userID)
		if err != nil {
			tolog.Log().Infof("Error while UserLastLoginUpdate %e %v", err, ok).PrintAndWriteSafe()
			return
		}
		tolog.Log().Infof("Updating last request for user %s in MySQL", userID).PrintAndWriteSafe()
	}
}
