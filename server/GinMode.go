package server

import (
	"github.com/gin-gonic/gin"
	"painter-server-new/conf"
)

const (
	// DebugMode indicates gin mode is debug.
	DebugMode = "debug"
	// ReleaseMode indicates gin mode is release.
	ReleaseMode = "release"
	// TestMode indicates gin mode is test.
	TestMode = "test"
)

var serverMode = conf.Server.Model

func SetGinMode() {
	switch serverMode {
	case DebugMode:
		gin.SetMode(DebugMode)
		return
	case ReleaseMode:
		gin.SetMode(ReleaseMode)
		return
	case TestMode:
		gin.SetMode(TestMode)
		return
	default:
		gin.SetMode(ReleaseMode)
	}
	return
}
