package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"painter-2026/system-service/internal/domain"
	"painter-2026/system-service/internal/infrastructure/cache"
	"painter-2026/system-service/internal/infrastructure/repository"
)

type envelope struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	TraceID string      `json:"traceId"`
	Data    interface{} `json:"data"`
}

type Server struct {
	engine    *gin.Engine
	repo      *repository.MySQLRepo
	cache     *cache.RedisStore
	startedAt time.Time
}

func NewServer() *Server {
	engine := gin.Default()
	s := &Server{
		engine:    engine,
		startedAt: time.Now(),
	}
	dsn := os.Getenv("SYSTEM_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/painter_system?charset=utf8mb4&parseTime=True&loc=Local"
	}
	repo, err := repository.NewMySQLRepo(dsn)
	if err != nil {
		panic(err)
	}
	redisAddr := os.Getenv("SYSTEM_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	redisStore := cache.NewRedisStore(redisAddr, os.Getenv("SYSTEM_REDIS_PASSWORD"), 1)
	if err := redisStore.Ping(context.Background()); err != nil {
		panic(err)
	}
	s.repo = repo
	s.cache = redisStore
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
			traceID = "system-local"
		}
		c.Set("traceID", traceID)
		c.Next()
	})

	s.engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"service": "system-service"}})
	})

	s.engine.GET("/system/configs", s.listConfigs)
	s.engine.GET("/system/admin/settings", s.getAdminSettings)
	s.engine.PATCH("/system/admin/settings", s.updateAdminSettings)
	s.engine.GET("/system/common/starttime", s.commonStartTime)
	s.engine.GET("/system/common/vis/preday", s.commonPreDayVis)
	s.engine.GET("/system/common/vis/currmonth", s.commonCurrentMonthVis)
	s.engine.GET("/system/common/info", s.getAdminSettings)
	s.engine.GET("/system/common/isadmin", s.commonIsAdmin)
	s.engine.GET("/system/common/entry", s.commonEntry)
	s.engine.POST("/system/file/upload", s.fileUpload)
}

func (s *Server) listConfigs(c *gin.Context) {
	namespace := c.Query("namespace")
	if err := (domain.Query{Namespace: namespace}).Validate(); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 3001, Message: "namespace required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	cacheKey := "system:configs:" + namespace
	if cached, err := s.cache.Get(c.Request.Context(), cacheKey); err == nil {
		items := make([]domain.ConfigItem, 0)
		if json.Unmarshal([]byte(cached), &items) == nil {
			c.JSON(http.StatusOK, envelope{
				Code:    0,
				Message: "ok",
				TraceID: getTraceID(c),
				Data: gin.H{
					"namespace": namespace,
					"items":     items,
				},
			})
			return
		}
	}
	records, err := s.repo.ListConfigs(namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 3500, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	items := make([]domain.ConfigItem, 0, len(records))
	for _, it := range records {
		items = append(items, domain.ConfigItem{
			Key: it.Key, Value: it.Value, ValueType: it.ValueType, UpdatedAt: it.UpdatedAt,
		})
	}
	if payload, err := json.Marshal(items); err == nil {
		_ = s.cache.Set(c.Request.Context(), cacheKey, string(payload), 2*time.Minute)
	}
	c.JSON(http.StatusOK, envelope{
		Code:    0,
		Message: "ok",
		TraceID: getTraceID(c),
		Data: gin.H{
			"namespace": namespace,
			"items":     items,
		},
	})
}

func getTraceID(c *gin.Context) string {
	traceID, ok := c.Get("traceID")
	if !ok {
		return "system-unknown"
	}
	return traceID.(string)
}

func (s *Server) getAdminSettings(c *gin.Context) {
	cacheKey := "system:settings:admin"
	if cached, err := s.cache.Get(c.Request.Context(), cacheKey); err == nil {
		var item gin.H
		if json.Unmarshal([]byte(cached), &item) == nil {
			c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: item})
			return
		}
	}
	setting, err := s.repo.GetSetting()
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 3501, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	data := gin.H{
		"siteName":    setting.SiteName,
		"icpCode":     setting.ICPCode,
		"github":      setting.Github,
		"canRegister": setting.CanRegister,
	}
	if payload, err := json.Marshal(data); err == nil {
		_ = s.cache.Set(c.Request.Context(), cacheKey, string(payload), 2*time.Minute)
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: data})
}

func (s *Server) updateAdminSettings(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 3101, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	updated, err := s.repo.UpdateSetting(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 3502, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	_ = s.cache.Delete(c.Request.Context(), "system:settings:admin")
	_ = s.cache.Delete(c.Request.Context(), "system:configs:public")
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{
		"siteName":    updated.SiteName,
		"icpCode":     updated.ICPCode,
		"github":      updated.Github,
		"canRegister": updated.CanRegister,
	}})
}

func (s *Server) commonStartTime(c *gin.Context) {
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"startTime": s.startedAt.Format(time.RFC3339)}})
}

func (s *Server) commonPreDayVis(c *gin.Context) {
	total, err := s.repo.CountFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 3503, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"visits": total}})
}

func (s *Server) commonCurrentMonthVis(c *gin.Context) {
	total, err := s.repo.CountConfigs("public")
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 3504, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"visits": total}})
}

func (s *Server) commonIsAdmin(c *gin.Context) {
	role := c.Query("role")
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"isAdmin": role == "admin"}})
}

func (s *Server) commonEntry(c *gin.Context) {
	setting, err := s.repo.GetSetting()
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 3505, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{
		"siteName": setting.SiteName,
		"slogan":   "Painter 2026",
	}})
}

func (s *Server) fileUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	filename := "mock.png"
	if err == nil && file != nil && file.Filename != "" {
		filename = file.Filename
		if mkErr := os.MkdirAll("uploads", 0o755); mkErr != nil {
			c.JSON(http.StatusInternalServerError, envelope{Code: 3506, Message: "storage init failed", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		dst := filepath.Join("uploads", filename)
		if saveErr := c.SaveUploadedFile(file, dst); saveErr != nil {
			c.JSON(http.StatusInternalServerError, envelope{Code: 3507, Message: "save file failed", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
	}
	if dbErr := s.repo.SaveFile(filename, "/static/upload/"+filename); dbErr != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 3508, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{
		"url":      "/static/upload/" + filename,
		"fileName": filename,
	}})
}
