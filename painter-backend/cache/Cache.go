package cache

import (
	"fmt"
	conf "painter-server-new/conf"

	"strconv"
	"time"

	"github.com/callme-taota/tolog"
	"github.com/go-redis/redis"
)

// RedisClient is a global variable representing the Redis client.
var RedisClient *redis.Client

// InitCache initializes the Redis cache connection.
func InitCache() error {
	// Convert the cache DB string to an integer.
	db, _ := strconv.Atoi(conf.CacheConf.DB)

	// Create a new Redis client using the configuration from the 'conf' package.
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", conf.CacheConf.Host, conf.CacheConf.Port),
		Password: conf.CacheConf.Password,
		DB:       db,
	})

	// Set the global 'RedisClient' variable to the created client.
	RedisClient = client

	// Define the maximum number of retries and the delay between retries.
	maxRetries := 5
	retryDelay := 2 * time.Second

	// Attempt to ping the Redis server with retries.
	var err error
	for i := 0; i < maxRetries; i++ {
		pong, err := client.Ping().Result()
		if err == nil {
			// Log a message indicating a successful connection to Redis.
			tolog.Infof("Connected to Redis: %s", pong).PrintLog()
			conf.RunningStatus.Cache = true
			return nil
		}
		tolog.Errorf("Failed to connect to Redis, attempt %d: %e", i+1, err).PrintAndWriteSafe()
		time.Sleep(retryDelay)
	}

	// Log an error if all attempts fail.
	tolog.Errorf("Could not connect to Redis after %d attempts: %e", maxRetries, err).PrintAndWriteSafe()
	return err
}
