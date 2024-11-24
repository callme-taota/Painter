package common

import (
	"painter-server-new/conf"

	"github.com/callme-taota/tolog"
)

var Modules []Module

const (
	DB_MODULE    = "db"
	CACHE_MODULE = "cache"
)

type Module interface {
	Name() string
	Register(conf conf.Config) (Module, error)
	Start() error
}

func StartModule() error {
	for _, m := range Modules {
		go func(module Module) {
			module.Register(conf.Conf.Conf())
			err := module.Start()
			if err != nil {
				tolog.Errorf("start module %s error: %v", module.Name(), err).PrintAndWriteSafe()
			}
		}(m)
	}
	return nil
}

func Register(m Module) {
	Modules = append(Modules, m)
}
