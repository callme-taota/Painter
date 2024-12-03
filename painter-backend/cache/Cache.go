package cache

import (
	"fmt"

	"github.com/callme-taota/painter/painter-backend/common"
	conf "github.com/callme-taota/painter/painter-backend/conf"

	"strconv"
	"time"

	"github.com/callme-taota/tolog"
	"github.com/go-redis/redis"
)

const maxRetries = 5
const retryDelay = 2 * time.Second

var Cache *cache

type cache struct {
	client *redis.Client
	opt    *redis.Options
}

func init() {
	Cache = newCache()
	common.Register(Cache)
}
func newCache() *cache {
	return &cache{}
}

func (c *cache) Name() string {
	return common.CACHE_MODULE
}

func (c *cache) Register(conf conf.Config) (common.Module, error) {
	db, _ := strconv.Atoi(conf.Redis.DB)

	opt := &redis.Options{
		Addr:     fmt.Sprintf("%s:%s", conf.Redis.Host, conf.Redis.Port),
		Password: conf.Redis.Password,
		DB:       db,
	}

	c.opt = opt
	return c, nil
}

func (c *cache) Start() error {
	c.client = redis.NewClient(c.opt)

	var err error
	for i := range maxRetries {
		pong, err := c.client.Ping().Result()
		if err == nil {
			// Log a message indicating a successful connection to Redis.
			tolog.Infof("Connected to Redis: %s", pong).PrintLog()
			conf.RunningStatus.Cache = true
			return nil
		}
		tolog.Errorf("Failed to connect to Redis, attempt %d: %e", i+1, err).PrintAndWriteSafe()
		time.Sleep(retryDelay)
	}
	tolog.Errorf("Could not connect to Redis after %d attempts: %e", maxRetries, err).PrintAndWriteSafe()
	return err
}
