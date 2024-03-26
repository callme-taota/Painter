package mid

import (
	"github.com/gin-contrib/cors"
	"time"
)

var CorsMid = cors.New(cors.Config{ //conf
	AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"},
	AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "session"},
	AllowCredentials: true,
	MaxAge:           12 * time.Hour, //time
	ExposeHeaders:    []string{"Content-Length"},
	AllowOriginFunc: func(origin string) bool { //allow
		return true //all
	},
})
