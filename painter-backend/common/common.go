package common

import (
	"github.com/callme-taota/painter/painter-backend/conf"

	"github.com/callme-taota/tolog"
)

var Modules = map[string]Module{}

const (
	DB_MODULE    = "db"
	CACHE_MODULE = "cache"
	TASK_MODULE  = "task"
)

type Module interface {
	Name() string
	Register(conf conf.Config) (Module, error)
	Start() error
}

func StartModule() error {
	for _, m := range Modules {
		go func(module Module) {
			module.Register(conf.Conf)
			err := module.Start()
			if err != nil {
				tolog.Errorf("start module %s error: %v", module.Name(), err).PrintAndWriteSafe()
			}
		}(m)
	}
	return nil
}

func Register(m Module) {
	Modules[m.Name()] = m
}

func UseModule(moduleName string) Module {
	return Modules[moduleName]
}
