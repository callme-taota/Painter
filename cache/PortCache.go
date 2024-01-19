package cache

import (
	"github.com/gin-gonic/gin"
	"net/url"
	"painter-server-new/tolog"
	"strings"
	"sync"
	"time"
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

func (c *Context) StatusName() string {
	return GetStatusName(c.Type)
}

func AutoCache(ctx Context) (any, error) {

	return nil, nil
}

func SetContext(c *gin.Context) *Context {
	fullRequest := c.Request.URL.String()

	parsedURL, err := url.Parse(fullRequest)
	if err != nil {
		tolog.Log().Errorf("URL formatting error : %e", err).PrintAndWriteSafe()
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
