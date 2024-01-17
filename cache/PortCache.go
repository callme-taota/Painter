package cache

import (
	"github.com/gin-gonic/gin"
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
)

type Param struct {
	Key   string
	Value string
}
type Params []Param

type Context struct {
	Type       AutoCacheStatus
	Request    string
	SubRequest string
	Params     Params

	LastUpdate time.Time
	Duration   time.Time
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

func SetContext(c *gin.Context) {
	fullRequest := c.Request.URL.String()

}
