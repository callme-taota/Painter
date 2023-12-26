package cache

import (
	"backendQucikStart/utils"
)

func AddUser(username, password string) (string, error) {
	// Generate a new session for the user.
	session, err := utils.CreateSession(username)
	if err != nil {
		return "", err
	}
	RedisClient.Set(username, password, 0)
	return session, err
}
