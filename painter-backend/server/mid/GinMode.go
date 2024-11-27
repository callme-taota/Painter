package mid

import (
	"github.com/callme-taota/painter/painter-backend/conf"

	"github.com/gin-gonic/gin"
)

const (
	// DebugMode indicates gin mode is debug.
	DebugMode = "debug"
	// ReleaseMode indicates gin mode is release.
	ReleaseMode = "release"
	// TestMode indicates gin mode is test.
	TestMode = "test"
)

func SetGinMode() {
	switch conf.Conf.Server.Model {
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
