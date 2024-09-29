package cache

import (
	"encoding/json"
	"errors"
	"net/url"
	conf "painter-server-new/conf"
	"strings"
	"sync"
	"time"

	"github.com/callme-taota/tolog"
	"github.com/gin-gonic/gin"
)

type AutoCacheStatus struct {
	Status uint
}

const (
	KCacheStatusOn = iota
	KCacheStatusOutdated
	KCacheStatusOff
	KCacheStatusUpdating
	KCacheStatusLocked
	KCacheStatusCreated
)

type Param struct {
	Key   string
	Value string
}
type Params []Param

type Context struct {
	Type AutoCacheStatus

	FullRequest string
	Request     string
	SubRequest  string
	Params      Params

	LastUpdate time.Time
}

// ContextMapManager 管理 Context 对象的存储
type ContextMapManager struct {
	mu       sync.RWMutex
	contexts map[string]map[string]Context
}

var contextMapManager *ContextMapManager

func GetStatusName(t AutoCacheStatus) string {
	switch t.Status {
	case 0:
		return "on"
	case 1:
		return "outdated"
	case 2:
		return "off"
	case 3:
		return "updating"
	case 4:
		return "locked"
	}
	return ""
}

func (t AutoCacheStatus) CanStatus() bool {
	if t.Status == 0 {
		return true
	}
	return false
}

func (c *Context) StatusName() string {
	return GetStatusName(c.Type)
}

func AutoCache(c *gin.Context) (bool, error) {
	ok := CheckContextMapExist()
	if !ok {
		err := errors.New("ContextMapManager doesn't exist. ")
		tolog.Errorf("Error while AutoCache: %e", err).PrintAndWriteSafe()
		return false, err
	}
	context := CreateContext(c)
	contextMapManager.AddContext(*context)
	return true, nil
}

func InitCacheContext() {
	mapManager := NewContextMapManager()
	contextMapManager = mapManager
}

func CreateContext(c *gin.Context) *Context {
	fullRequest := c.Request.URL.String()

	parsedURL, err := url.Parse(fullRequest)
	if err != nil {
		tolog.Errorf("URL formatting error : %e", err).PrintAndWriteSafe()
	}
	request := parsedURL.Path
	subRequest := parsedURL.RawQuery
	subRequest = strings.TrimLeft(subRequest, "/")
	params := SubRequest2Params(subRequest)
	return &Context{
		Type:        AutoCacheStatus{KCacheStatusCreated},
		FullRequest: fullRequest,
		Request:     request,
		SubRequest:  subRequest,
		Params:      params,
		LastUpdate:  time.Now(),
	}
}

func SubRequest2Params(subRequest string) []Param {
	subParts := strings.Split(subRequest, "&")
	arrLen := len(subParts)
	params := make([]Param, arrLen)

	for i, item := range subParts {
		paramParts := strings.Split(item, "=")
		key := paramParts[0]
		value := paramParts[1]
		param := Param{Key: key, Value: value}
		params[i] = param
	}

	return params
}

func NewContextMapManager() *ContextMapManager {
	return &ContextMapManager{
		contexts: make(map[string]map[string]Context),
	}
}

func CheckContextMapExist() bool {
	return contextMapManager != nil
}

func (c *ContextMapManager) AddContext(context Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	request := context.Request
	subRequest := context.SubRequest
	// 如果 request 对应的 map 不存在，则创建
	if _, ok := c.contexts[request]; !ok {
		c.contexts[request] = make(map[string]Context)
	}

	// 添加 subRequest 和对应的 Context
	c.contexts[request][subRequest] = context
}

func (c *ContextMapManager) CheckContext(context Context) bool {
	request := context.Request
	subRequest := context.SubRequest

	if _, ok := c.contexts[request][subRequest]; !ok {
		return false
	}
	return true
}

func (c *ContextMapManager) DeleteContext(context Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	request := context.Request
	subRequest := context.SubRequest

	if _, ok := c.contexts[request]; ok {
		delete(c.contexts[request], subRequest)
	}
}

func (c *ContextMapManager) ContextShouldUpdate(context Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	request := context.Request
	subRequest := context.SubRequest
	if ctx, ok := c.contexts[request][subRequest]; ok {
		ctx.Type = AutoCacheStatus{KCacheStatusOutdated}
		c.contexts[request][subRequest] = ctx
	}
}

func (c *ContextMapManager) ContextOn(context Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	request := context.Request
	subRequest := context.SubRequest
	if ctx, ok := c.contexts[request][subRequest]; ok {
		ctx.Type = AutoCacheStatus{KCacheStatusOn}
		c.contexts[request][subRequest] = ctx
	}
}

func (c *ContextMapManager) ContextOff(context Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	request := context.Request
	subRequest := context.SubRequest
	if ctx, ok := c.contexts[request][subRequest]; ok {
		ctx.Type = AutoCacheStatus{KCacheStatusOff}
		c.contexts[request][subRequest] = ctx
	}
}

func (c *ContextMapManager) ContextLock(context Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	request := context.Request
	subRequest := context.SubRequest
	if ctx, ok := c.contexts[request][subRequest]; ok {
		ctx.Type = AutoCacheStatus{KCacheStatusLocked}
		c.contexts[request][subRequest] = ctx
	}
}

func (c *ContextMapManager) ContextUnlock(context Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	request := context.Request
	subRequest := context.SubRequest
	if ctx, ok := c.contexts[request][subRequest]; ok {
		ctx.Type = AutoCacheStatus{KCacheStatusOutdated}
		c.contexts[request][subRequest] = ctx
	}
}

func (c *Context) PutContext2Redis(m any) error {
	key := conf.Server.Name + c.FullRequest
	value, err := json.Marshal(m)
	if err != nil {
		return err
	}
	err = RedisClient.Set(key, value, 0).Err()
	if err != nil {
		return err
	}
	contextMapManager.ContextOn(*c)
	return nil
}

func (c *Context) DelContext2Redis() error {
	key := conf.Server.Name + c.FullRequest
	err := RedisClient.Del(key).Err()
	if err != nil {
		return err
	}
	contextMapManager.ContextOff(*c)
	return nil
}

func (c *Context) GetContextContentFromRedis() (string, error) {
	key := conf.Server.Name + c.FullRequest
	value, err := RedisClient.Get(key).Result()
	if err != nil {
		return "", err
	}
	return value, nil
}

func (c *Context) GetContextContentByJSONFromRedis() (any, error) {
	key := conf.Server.Name + c.FullRequest
	value, err := RedisClient.Get(key).Result()
	if err != nil {
		return "", err
	}
	var data map[string]interface{}
	err = json.Unmarshal([]byte(value), &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}
