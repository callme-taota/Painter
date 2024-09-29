package cache

import (
	"errors"

	"painter-server-new/utils"
	"strconv"
	"time"

	"github.com/callme-taota/tolog"
	"github.com/redis/go-redis/v9"
)

// Constants for Redis hash keys
const reverseLookupHash = "painter_reverse_lookup"
const forwardHash = "painter_user_sessions"

// AddUser generates a session for a user, associates it with the user ID in Redis, and updates reverse lookup.
func AddUser(userID string) (string, error) {
	// Generate a new session for the user.
	session, err := utils.CreateSession(userID)
	if err != nil {
		return "", err
	}

	// Set the user ID to session mapping in the forward hash.
	err = RedisClient.HSet(forwardHash, userID, session).Err()
	if err != nil {
		return "", err
	}

	// Set the session to user ID mapping in the reverse lookup hash.
	err = RedisClient.HSet(reverseLookupHash, session, userID).Err()
	if err != nil {
		return "", err
	}

	return session, err
}

// GetUserSessionByID retrieves the session associated with a user ID from Redis.
func GetUserSessionByID(userID string) (string, error) {
	session, err := RedisClient.HGet(forwardHash, userID).Result()
	if err == redis.Nil {
		// Return nil error if the user ID is not found.
		return "", nil
	} else if err != nil {
		return "", err
	}
	return session, nil
}

// GetUserIDByUserSession retrieves the user ID associated with a session from Redis.
func GetUserIDByUserSession(session string) (string, error) {
	userID, err := RedisClient.HGet(reverseLookupHash, session).Result()
	if err == redis.Nil {
		// Return nil error if the session is not found.
		return "", errors.New("no user")
	} else if err != nil {
		return "", err
	}
	return userID, nil
}

// DeleteUserBySession removes user-related data from Redis using the provided session.
func DeleteUserBySession(session string) (bool, error) {
	// Retrieve user ID associated with the session.
	userID, err := GetUserIDByUserSession(session)
	if err != nil {
		tolog.Errorf("DeleteUserBySession: %e", err)
		return false, err
	}

	// Remove session to user ID mapping.
	err = RedisClient.HDel(reverseLookupHash, session).Err()
	if err != nil {
		tolog.Errorf("DeleteUserBySession: %e", err)
		return false, err
	}

	// Remove user ID to session mapping.
	err = RedisClient.HDel(forwardHash, userID).Err()
	if err != nil {
		tolog.Errorf("DeleteUserBySession: %e", err)
		return false, err
	}

	// Delete the session key.
	err = RedisClient.Del(session).Err()
	if err != nil {
		tolog.Errorf("DeleteUserBySession: %e", err)
		return false, err
	}

	return true, nil
}

func CreateEmailCheck(mail string, code int) error {
	res := RedisClient.Set(
		"painter_verification_code:"+mail,
		code,
		5*time.Minute,
	)
	if res.Err() != nil {
		return res.Err()
	}
	return nil
}

func CheckEmailPass(mail string, code int) (bool, error) {
	key := "painter_verification_code:" + mail
	val, err := RedisClient.Get(key).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil // 验证码不存在
		}
		return false, err
	}
	codeStr := strconv.Itoa(code)
	if val == codeStr {
		return true, nil // 验证码正确
	}
	return false, nil // 验证码不正确
}

func CreateUserAccess(userID int) error {
	// get current time
	now := time.Now()
	// redis key，with yyyy-MM-dd-HH
	redisKey := "painter-connect" + now.Format("2006-01-02-15")

	// put user ID into redis set
	_, err := RedisClient.SAdd(redisKey, userID).Result()
	if err != nil {
		return err
	}

	expire := time.Hour * 2

	// set key expire
	_, err = RedisClient.Expire(redisKey, expire).Result()
	if err != nil {
		return err
	}

	return nil
}

func GetPastHourLoginUserList() ([]int, error) {
	// get current time
	now := time.Now().Add(-time.Hour)
	// redis key，with yyyy-MM-dd-HH
	redisKey := "painter-connect" + now.Format("2006-01-02-15")
	//get list
	userIDs, err := RedisClient.SMembers(redisKey).Result()
	if err != nil {
		tolog.Infof("Error getting user IDs from Redis: %e", err).PrintAndWriteSafe()
		return nil, err
	}
	var userList []int
	for _, userID := range userIDs {
		userIntID, _ := strconv.Atoi(userID)
		userList = append(userList, userIntID)
	}
	return userList, nil
}
