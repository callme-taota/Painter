package httpserver

import (
	"bytes"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"painter-2026/api-gateway/internal/application"
	"painter-2026/api-gateway/internal/infrastructure/proxy"
)

type envelope struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	TraceID string      `json:"traceId"`
	Data    interface{} `json:"data"`
}

type Server struct {
	engine *gin.Engine
	router *application.RouterService
	proxy  *proxy.Client
}

func NewServer() *Server {
	engine := gin.Default()
	s := &Server{
		engine: engine,
		router: application.NewRouterService(),
		proxy:  proxy.NewClient(),
	}
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
			traceID = "gw-" + time.Now().Format("20060102150405.000")
		}
		c.Set("traceID", traceID)
		c.Next()
	})

	s.engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"service": "api-gateway"}})
	})

	s.registerLegacyCompatRoutes()

	v1 := s.engine.Group("/api/v1")
	v1.Any("/:domain/*path", s.proxyHandler)
}

func (s *Server) registerLegacyCompatRoutes() {
	legacyToV1 := map[string]string{
		"POST /api/user/login/email":             "/api/v1/identity/auth/login",
		"POST /api/user/login/email/pass":        "/api/v1/identity/auth/login",
		"POST /api/user/login/uname":             "/api/v1/identity/auth/login",
		"POST /api/user/login/uname/pass":        "/api/v1/identity/auth/login",
		"GET /api/follow/followings":             "/api/v1/identity/follows",
		"POST /api/follow/follow":                "/api/v1/identity/follows",
		"POST /api/follow/unfollow":              "/api/v1/identity/follows",
		"GET /api/comment/list":                  "/api/v1/content/comments",
		"POST /api/comment/create":               "/api/v1/content/comments",
		"POST /api/comment/delete":               "/api/v1/content/comment/delete",
		"POST /api/comment/like":                 "/api/v1/content/comment/like",
		"POST /api/comment/dislike":              "/api/v1/content/comment/dislike",
		"GET /api/article/get/time":              "/api/v1/content/articles",
		"GET /api/article/get":                   "/api/v1/content/articles/get",
		"GET /api/article/get/count":             "/api/v1/content/articles/count",
		"GET /api/article/search":                "/api/v1/content/articles/search",
		"GET /api/article/get/author":            "/api/v1/content/articles/by-author",
		"GET /api/article/get/title":             "/api/v1/content/articles/by-title",
		"GET /api/article/get/content":           "/api/v1/content/articles/by-content",
		"GET /api/article/get/category":          "/api/v1/content/articles/by-category",
		"GET /api/article/get/tag":               "/api/v1/content/articles/by-tag",
		"GET /api/article/get/collection":        "/api/v1/content/articles/by-collection",
		"GET /api/article/get/self":              "/api/v1/content/articles/self",
		"POST /api/article/create":               "/api/v1/content/articles",
		"POST /api/article/update":               "/api/v1/content/articles/update",
		"POST /api/article/update/title":         "/api/v1/content/articles/update/title",
		"POST /api/article/update/content":       "/api/v1/content/articles/update/content",
		"POST /api/article/update/summary":       "/api/v1/content/articles/update/summary",
		"POST /api/article/update/category":      "/api/v1/content/articles/update/category",
		"POST /api/article/update/status/dart":   "/api/v1/content/articles/update/status/draft",
		"POST /api/article/update/status/public": "/api/v1/content/articles/update/status/public",
		"POST /api/article/delete":               "/api/v1/content/articles/delete",
		"POST /api/article/like":                 "/api/v1/content/interactions/like",
		"POST /api/article/like/check":           "/api/v1/content/interactions/like/check",
		"POST /api/article/collection":           "/api/v1/content/interactions/collect",
		"POST /api/article/tag/create":           "/api/v1/content/tags/create",
		"POST /api/article/tag/update":           "/api/v1/content/tags/update",
		"POST /api/article/tag/delete":           "/api/v1/content/tags/delete",
		"GET /api/tag/suggest":                   "/api/v1/content/tags/suggest",
		"GET /api/tag/list":                      "/api/v1/content/tags/list",
		"GET /api/tag/list/full":                 "/api/v1/content/tags/list/full",
		"POST /api/tag/create":                   "/api/v1/content/tags/create",
		"POST /api/tag/update/name":              "/api/v1/content/tags/update",
		"POST /api/tag/update/desc":              "/api/v1/content/tags/update",
		"POST /api/tag/update/":                  "/api/v1/content/tags/update",
		"GET /api/category/list":                 "/api/v1/content/categories/list",
		"GET /api/category/get":                  "/api/v1/content/categories/get",
		"GET /api/category/get/fulllist":         "/api/v1/content/categories/full",
		"POST /api/category/create":              "/api/v1/content/categories/create",
		"POST /api/category/update/name":         "/api/v1/content/categories/update/name",
		"POST /api/category/update/desc":         "/api/v1/content/categories/update/desc",
		"POST /api/category/update/":             "/api/v1/content/categories/update",
		"POST /api/collection/delete":            "/api/v1/content/collection/delete",
		"POST /api/collection/list":              "/api/v1/content/collection/list",
		"POST /api/collection/check":             "/api/v1/content/collection/check",
		"GET /api/user/self":                     "/api/v1/identity/users/self",
		"GET /api/user/self/full":                "/api/v1/identity/users/self/full",
		"POST /api/user/logout":                  "/api/v1/identity/users/logout",
		"POST /api/user/update/name":             "/api/v1/identity/users/update/name",
		"POST /api/user/update/email":            "/api/v1/identity/users/update/email",
		"POST /api/user/update/nickname":         "/api/v1/identity/users/update/nickname",
		"POST /api/user/update/phone":            "/api/v1/identity/users/update/phone",
		"POST /api/user/update/headerfield":      "/api/v1/identity/users/update/headerfield",
		"POST /api/user/update/passwd":           "/api/v1/identity/users/update/passwd",
		"POST /api/user/update":                  "/api/v1/identity/users/update",
		"GET /api/user/info":                     "/api/v1/identity/users/info",
		"POST /api/user/login/exist":             "/api/v1/identity/auth/exist",
		"POST /api/user/login/send":              "/api/v1/identity/auth/send-code",
		"POST /api/user/login/mailcheck":         "/api/v1/identity/auth/check-code",
		"GET /api/setting/userlist":              "/api/v1/identity/admin/userlist",
		"POST /api/setting/user/permission":      "/api/v1/identity/admin/user/permission",
		"GET /api/common/starttime":              "/api/v1/system/common/starttime",
		"GET /api/common/vis/preday":             "/api/v1/system/common/vis/preday",
		"GET /api/common/vis/currmonth":          "/api/v1/system/common/vis/currmonth",
		"GET /api/common/info":                   "/api/v1/system/common/info",
		"GET /api/common/isadmin":                "/api/v1/system/common/isadmin",
		"GET /api/common/entry":                  "/api/v1/system/common/entry",
		"POST /api/file/upload":                  "/api/v1/system/file/upload",
		"GET /api/history/list":                  "/api/v1/analytics/history/list",
		"GET /api/setting/":                      "/api/v1/system/admin/settings",
		"POST /api/setting/":                     "/api/v1/system/admin/settings",
		"POST /api/user/create":                  "/api/v1/identity/auth/register",
		"POST /api/user/login/check":             "/api/v1/identity/auth/check",
		"POST /api/article/like/create":          "/api/v1/content/interactions/like/create",
		"POST /api/article/like/delete":          "/api/v1/content/interactions/like/delete",
		"GET /api/follow/followers":              "/api/v1/identity/followers",
		"GET /api/comment/list/l":                "/api/v1/content/comments/liked",
	}

	legacyCompatOnly := []struct {
		method string
		path   string
	}{}

	for key, target := range legacyToV1 {
		method := strings.Split(key, " ")[0]
		path := strings.TrimPrefix(key, method+" ")
		s.engine.Handle(method, path, s.legacyProxyHandler(target))
	}
	for _, route := range legacyCompatOnly {
		s.engine.Handle(route.method, route.path, s.legacyCompatHandler(route.method, route.path))
	}
}

func (s *Server) legacyProxyHandler(target string) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if target == "/api/v1/system/admin/settings" && method == http.MethodPost {
			method = http.MethodPatch
		}
		body := []byte{}
		if c.Request.Body != nil {
			raw, err := proxy.ReadBody(c.Request.Body)
			if err == nil {
				body = raw
			}
		}
		req, err := http.NewRequest(method, target+"?"+c.Request.URL.RawQuery, bytes.NewReader(body))
		if err != nil {
			c.JSON(http.StatusBadGateway, envelope{Code: 9001, Message: "proxy build failed", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		req.Header = c.Request.Header.Clone()
		req.Header.Set("X-Legacy-Path", c.FullPath())
		c.Request = req
		parts := strings.Split(strings.TrimPrefix(target, "/api/v1/"), "/")
		if len(parts) > 0 {
			c.Params = append(c.Params, gin.Param{Key: "domain", Value: parts[0]})
			c.Params = append(c.Params, gin.Param{Key: "path", Value: "/" + strings.Join(parts[1:], "/")})
		}
		s.proxyHandler(c)
	}
}

func (s *Server) legacyCompatHandler(method string, path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, envelope{
			Code:    0,
			Message: "compat handler",
			TraceID: getTraceID(c),
			Data: gin.H{
				"legacyMethod": method,
				"legacyPath":   path,
				"status":       "mapped-to-compat-layer",
			},
		})
	}
}

func (s *Server) proxyHandler(c *gin.Context) {
	domain := c.Param("domain")
	baseURL := s.router.Resolve(domain)
	if baseURL == "" {
		c.JSON(http.StatusBadGateway, envelope{Code: 9003, Message: "unknown upstream domain", TraceID: getTraceID(c), Data: gin.H{"domain": domain}})
		return
	}
	path := c.Param("path")
	if path == "" {
		path = "/"
	}
	url := baseURL + "/" + domain + path
	if query := c.Request.URL.RawQuery; query != "" {
		url += "?" + query
	}

	body := []byte{}
	if c.Request.Body != nil {
		raw, err := proxy.ReadBody(c.Request.Body)
		if err == nil {
			body = raw
		}
	}
	resp, err := s.proxy.Forward(c.Request.Method, url, getTraceID(c), c.Request.Header, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, envelope{Code: 9002, Message: "upstream unavailable", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	defer resp.Body.Close()
	payload, _ := proxy.ReadBody(resp.Body)
	contentType := resp.Header.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, payload)
}

func getTraceID(c *gin.Context) string {
	traceID, ok := c.Get("traceID")
	if !ok {
		return "gw-unknown"
	}
	return traceID.(string)
}
