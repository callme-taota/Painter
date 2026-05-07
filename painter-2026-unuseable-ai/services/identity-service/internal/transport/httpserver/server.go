package httpserver

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"painter-2026/identity-service/internal/application"
	"painter-2026/identity-service/internal/domain"
	"painter-2026/identity-service/internal/infrastructure/cache"
	"painter-2026/identity-service/internal/infrastructure/repository"
	"painter-2026/identity-service/internal/infrastructure/token"
)

type envelope struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	TraceID string      `json:"traceId"`
	Data    interface{} `json:"data"`
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Server struct {
	engine *gin.Engine
	auth   *application.AuthService
	repo   *repository.MySQLRepo
	cache  *cache.RedisStore
}

func NewServer() *Server {
	engine := gin.Default()
	s := &Server{
		engine: engine,
		auth:   application.NewAuthService(token.NewSimpleIssuer()),
	}
	dsn := os.Getenv("IDENTITY_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/painter_identity?charset=utf8mb4&parseTime=True&loc=Local"
	}
	repo, err := repository.NewMySQLRepo(dsn)
	if err != nil {
		panic(err)
	}
	redisAddr := os.Getenv("IDENTITY_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	redisStore := cache.NewRedisStore(redisAddr, os.Getenv("IDENTITY_REDIS_PASSWORD"), 0)
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
			traceID = "identity-local"
		}
		c.Set("traceID", traceID)
		c.Next()
	})

	s.engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"service": "identity-service"}})
	})

	s.engine.POST("/identity/auth/login", s.loginHandler)
	s.engine.POST("/identity/auth/register", s.registerUser)
	s.engine.POST("/identity/auth/check", s.checkLogin)
	s.engine.GET("/identity/follows", s.listFollows)
	s.engine.GET("/identity/followers", s.listFollowers)
	s.engine.POST("/identity/follows", s.createFollow)
	s.engine.POST("/identity/auth/exist", s.userExist)
	s.engine.POST("/identity/auth/send-code", s.sendCode)
	s.engine.POST("/identity/auth/check-code", s.checkCode)
	s.engine.GET("/identity/users/self", s.getSelf)
	s.engine.GET("/identity/users/self/full", s.getSelfFull)
	s.engine.GET("/identity/users/info", s.getUserInfo)
	s.engine.POST("/identity/users/logout", s.logout)
	s.engine.POST("/identity/users/update", s.updateProfile)
	s.engine.POST("/identity/users/update/name", s.updateSingle("userName"))
	s.engine.POST("/identity/users/update/email", s.updateSingle("email"))
	s.engine.POST("/identity/users/update/nickname", s.updateSingle("nickName"))
	s.engine.POST("/identity/users/update/phone", s.updateSingle("phone"))
	s.engine.POST("/identity/users/update/headerfield", s.updateSingle("headerField"))
	s.engine.POST("/identity/users/update/passwd", s.updateSingle("password"))
	s.engine.GET("/identity/admin/userlist", s.getUserList)
	s.engine.POST("/identity/admin/user/permission", s.setUserPermission)
}

func (s *Server) loginHandler(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 1001, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	user, err := s.repo.GetUserByCredential(req.Username)
	if err != nil || user.Password != req.Password {
		c.JSON(http.StatusUnauthorized, envelope{Code: 1004, Message: "invalid credentials", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	expireAt := time.Now().Add(2 * time.Hour)
	result, loginErr := s.auth.Login(domain.LoginCommand{Username: user.UserName, Password: req.Password})
	if loginErr != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1500, Message: "internal error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	result.UserID = user.UserID
	result.ExpiresAt = expireAt.Format(time.RFC3339)
	if err := s.cache.SetLoginSession(c.Request.Context(), result.AccessToken, result.UserID, time.Until(expireAt)); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1513, Message: "redis error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: result})
}

func getTraceID(c *gin.Context) string {
	traceID, ok := c.Get("traceID")
	if !ok {
		return "identity-unknown"
	}
	return traceID.(string)
}

func (s *Server) listFollows(c *gin.Context) {
	userID, err := s.currentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 1101, Message: "login required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	rows, err := s.repo.ListFollowings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1501, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{"userId": row.TargetUser, "nickname": row.TargetUser})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items}})
}

func (s *Server) createFollow(c *gin.Context) {
	var req struct {
		TargetUserID string `json:"targetUserId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.TargetUserID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 1102, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	userID, userErr := s.currentUserID(c)
	if userErr != nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 1103, Message: "login required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	legacyPath := c.GetHeader("X-Legacy-Path")
	if strings.Contains(legacyPath, "/api/follow/unfollow") {
		if err := s.repo.DeleteFollow(userID, req.TargetUserID); err != nil {
			c.JSON(http.StatusInternalServerError, envelope{Code: 1502, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"success": true}})
		return
	}
	if err := s.repo.CreateFollow(userID, req.TargetUserID); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1502, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"success": true}})
}

func (s *Server) registerUser(c *gin.Context) {
	var req struct {
		UserID   string `json:"UserID"`
		UserName string `json:"UserName"`
		Email    string `json:"Email"`
		Passwd   string `json:"Passwd"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == "" || req.UserName == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 1203, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	exists, err := s.repo.UserExists(req.UserID, "", "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1503, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, envelope{Code: 1204, Message: "user exists", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	if err := s.repo.CreateUser(repository.User{
		UserID:      req.UserID,
		UserName:    req.UserName,
		Email:       req.Email,
		NickName:    req.UserName,
		Password:    req.Passwd,
		Group:       3,
		HeaderField: "",
	}); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1504, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"created": true}})
}

func (s *Server) checkLogin(c *gin.Context) {
	token := extractToken(c)
	if token == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 1003, Message: "token required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	userID, err := s.cache.GetLoginSession(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"loggedIn": false}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"loggedIn": true, "userId": userID}})
}

func (s *Server) listFollowers(c *gin.Context) {
	userID := c.Query("userId")
	if userID == "" {
		current, err := s.currentUserID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, envelope{Code: 1505, Message: "login required", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		userID = current
	}
	rows, err := s.repo.ListFollowers(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1505, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	items := make([]gin.H, 0)
	for _, row := range rows {
		items = append(items, gin.H{"userId": row.UserID, "nickname": row.UserID})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items}})
}

func (s *Server) userExist(c *gin.Context) {
	var req struct {
		UserID   string `json:"UserID"`
		UserName string `json:"UserName"`
		Email    string `json:"Email"`
	}
	_ = c.ShouldBindJSON(&req)
	exists, err := s.repo.UserExists(req.UserID, req.UserName, req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1506, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"exist": exists}})
}

func (s *Server) sendCode(c *gin.Context) {
	var req struct {
		Email string `json:"Email"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Email == "" {
		req.Email = c.Query("Email")
	}
	if req.Email == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 1601, Message: "email required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	code, codeErr := generateLoginCode()
	if codeErr != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1602, Message: "code generation failed", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	if err := s.cache.SetLoginCode(c.Request.Context(), req.Email, code, 5*time.Minute); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1602, Message: "redis error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"sent": true}})
}

func (s *Server) checkCode(c *gin.Context) {
	var req struct {
		Email string `json:"Email"`
		Code  string `json:"Code"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Email == "" {
		req.Email = c.Query("Email")
	}
	if req.Code == "" {
		req.Code = c.Query("Code")
	}
	if req.Email == "" || req.Code == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 1603, Message: "email/code required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	valid, err := s.cache.VerifyLoginCode(c.Request.Context(), req.Email, req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1604, Message: "redis error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"valid": valid}})
}

func (s *Server) getSelf(c *gin.Context) {
	userID, err := s.currentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 1507, Message: "login required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	user, err := s.repo.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1507, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: toUserJSON(user)})
}

func (s *Server) getSelfFull(c *gin.Context) {
	userID, err := s.currentUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 1508, Message: "login required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	user, err := s.repo.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1508, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	rows, _ := s.repo.ListFollowers(userID)
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{
		"profile":   toUserJSON(user),
		"followers": len(rows),
		"articles":  0,
	}})
}

func (s *Server) getUserInfo(c *gin.Context) {
	userID := c.Query("UserID")
	if userID == "" {
		current, err := s.currentUserID(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, envelope{Code: 1404, Message: "login required", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		userID = current
	}
	user, err := s.repo.GetUser(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, envelope{Code: 1404, Message: "user not found", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: toUserJSON(user)})
}

func (s *Server) logout(c *gin.Context) {
	token := extractToken(c)
	if token != "" {
		_ = s.cache.DeleteLoginSession(c.Request.Context(), token)
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"logout": true}})
}

func (s *Server) updateProfile(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 1201, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	userID, userErr := s.currentUserID(c)
	if userErr != nil {
		c.JSON(http.StatusUnauthorized, envelope{Code: 1509, Message: "login required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	if err := s.repo.UpdateUser(userID, req); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1509, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	user, _ := s.repo.GetUser(userID)
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: toUserJSON(user)})
}

func (s *Server) updateSingle(field string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req map[string]interface{}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, envelope{Code: 1202, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		value := req[field]
		if value == nil {
			for _, v := range req {
				value = v
				break
			}
		}
		userID, userErr := s.currentUserID(c)
		if userErr != nil {
			c.JSON(http.StatusUnauthorized, envelope{Code: 1510, Message: "login required", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		if err := s.repo.UpdateUser(userID, map[string]interface{}{field: value}); err != nil {
			c.JSON(http.StatusInternalServerError, envelope{Code: 1510, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		user, _ := s.repo.GetUser(userID)
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: toUserJSON(user)})
	}
}

func (s *Server) getUserList(c *gin.Context) {
	users, err := s.repo.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1511, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		items = append(items, toUserJSON(u))
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items}})
}

func (s *Server) setUserPermission(c *gin.Context) {
	var req struct {
		UserID string `json:"UserID"`
		Group  int    `json:"Group"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 1301, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	if err := s.repo.SetUserGroup(req.UserID, req.Group); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 1512, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"success": true}})
}

func toUserJSON(u repository.User) gin.H {
	return gin.H{
		"userId":      u.UserID,
		"userName":    u.UserName,
		"email":       u.Email,
		"nickName":    u.NickName,
		"phone":       u.Phone,
		"headerField": u.HeaderField,
		"group":       u.Group,
	}
}

func extractToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	var req struct {
		AccessToken string `json:"accessToken"`
		Token       string `json:"token"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.AccessToken != "" {
		return req.AccessToken
	}
	return req.Token
}

func (s *Server) currentUserID(c *gin.Context) (string, error) {
	token := extractToken(c)
	if token == "" {
		return "", http.ErrNoCookie
	}
	return s.cache.GetLoginSession(c.Request.Context(), token)
}

func generateLoginCode() (string, error) {
	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	n := int(buf[0])<<16 | int(buf[1])<<8 | int(buf[2])
	return fmt.Sprintf("%06d", n%1000000), nil
}
