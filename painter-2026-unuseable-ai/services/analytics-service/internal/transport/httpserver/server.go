package httpserver

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"painter-2026/analytics-service/internal/infrastructure/repository"
)

type envelope struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	TraceID string      `json:"traceId"`
	Data    interface{} `json:"data"`
}

type Server struct {
	engine *gin.Engine
	repo   *repository.MySQLRepo
}

func NewServer() *Server {
	engine := gin.Default()
	s := &Server{
		engine: engine,
	}
	dsn := os.Getenv("ANALYTICS_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/painter_analytics?charset=utf8mb4&parseTime=True&loc=Local"
	}
	repo, err := repository.NewMySQLRepo(dsn)
	if err != nil {
		panic(err)
	}
	s.repo = repo
	s.registerRoutes()
	return s
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) registerRoutes() {
	s.engine.Use(func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = "analytics-local"
		}
		c.Set("traceID", traceID)
		c.Next()
	})

	s.engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"service": "analytics-service"}})
	})

	s.engine.GET("/analytics/overview", func(c *gin.Context) {
		data, err := s.repo.GetOverview()
		if err != nil {
			c.JSON(http.StatusInternalServerError, envelope{Code: 4500, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: data})
	})
	s.engine.GET("/analytics/history/list", func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.Query("limit"))
		items, err := s.repo.ListHistory(limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, envelope{Code: 4501, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items}})
	})
}

func getTraceID(c *gin.Context) string {
	traceID, ok := c.Get("traceID")
	if !ok {
		return "analytics-unknown"
	}
	return traceID.(string)
}
