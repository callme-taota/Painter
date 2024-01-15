package cache

import "time"

type AutoCacheStatus struct {
	Status uint
}

const (
	KCacheStatusOn       = 1
	KCacheStatusOutdated = 2
	KCacheStatusOff      = 3
	KCacheStatusUpdating = 4
	KCacheStatusLocked   = 5
)

type Param struct {
	Key   string
	Value string
}
type Params []Param

type AutoCacheContext struct {
	Type       AutoCacheStatus
	Request    string
	SubRequest string
	Params     Params

	LastUpdate time.Time
	Duration   time.Time

	Stamp string
}

func SetContextStamp() {

}

func AutoCache(ctx AutoCacheContext) (any, error) {

	return nil, nil
}

func InitContext() {

}

func InitContextPool() {

}

func SetContextDuration() {

}

func ReSetContextDuration() {

}

func FillContextRequest() {

}

func CompleteContextSubRequest() {

}

func DestroyContext() {

}

func SetContextStatus() {

}

func SetContextTime() {

}

func DoInitializeCache() {

}

func GetCache() {

}

func SetCache() {

}
