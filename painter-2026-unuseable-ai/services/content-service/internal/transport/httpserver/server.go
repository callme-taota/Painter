package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"painter-2026/content-service/internal/application"
	"painter-2026/content-service/internal/domain"
	"painter-2026/content-service/internal/infrastructure/cache"
	"painter-2026/content-service/internal/infrastructure/repository"
	"painter-2026/content-service/internal/infrastructure/search"
)

type envelope struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	TraceID string      `json:"traceId"`
	Data    interface{} `json:"data"`
}

type createArticleReq struct {
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Content    string   `json:"content"`
	CategoryID string   `json:"categoryId"`
	TagIDs     []string `json:"tagIds"`
}

type Server struct {
	engine       *gin.Engine
	articles     *application.ArticleService
	repo         *repository.MySQLContentRepo
	cache        *cache.RedisStore
	commentMu    sync.RWMutex
	comments     map[string][]gin.H
	interactMu   sync.Mutex
	interactions map[string]gin.H
	tagMu        sync.RWMutex
	tags         []gin.H
	categoryMu   sync.RWMutex
	categories   []gin.H
	collectionMu sync.RWMutex
	collections  map[string]map[string]bool
	likeMu       sync.RWMutex
	articleLikes map[string]map[string]bool
	commentLikes map[string]map[string]bool
	indexer      *search.Indexer
}

func NewServer() *Server {
	engine := gin.Default()
	s := &Server{
		engine:       engine,
		articles:     application.NewArticleService(repository.NewMemoryArticleRepo()),
		comments:     map[string][]gin.H{},
		interactions: map[string]gin.H{},
		tags: []gin.H{
			{"tagId": 1, "name": "Go", "desc": "golang", "count": 1},
			{"tagId": 2, "name": "Vue", "desc": "frontend", "count": 1},
		},
		categories: []gin.H{
			{"categoryId": 1, "name": "默认分类", "desc": "default"},
		},
		collections:  map[string]map[string]bool{},
		articleLikes: map[string]map[string]bool{},
		commentLikes: map[string]map[string]bool{},
	}
	dsn := os.Getenv("CONTENT_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3306)/painter_content?charset=utf8mb4&parseTime=True&loc=Local"
	}
	mysqlRepo, err := repository.NewMySQLContentRepo(dsn)
	if err != nil {
		panic(err)
	}
	redisAddr := os.Getenv("CONTENT_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	redisStore := cache.NewRedisStore(redisAddr, os.Getenv("CONTENT_REDIS_PASSWORD"), 2)
	if err := redisStore.Ping(context.Background()); err != nil {
		panic(err)
	}
	s.repo = mysqlRepo
	s.cache = redisStore
	s.articles = application.NewArticleService(mysqlRepo)
	tok, err := search.NewTokenizer()
	if err != nil {
		panic(err)
	}
	s.indexer = search.NewIndexer(tok, mysqlRepo)
	s.registerRoutes()
	go s.startSearchBackfill()
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
			traceID = "content-local"
		}
		c.Set("traceID", traceID)
		c.Next()
	})

	s.engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"service": "content-service"}})
	})

	s.engine.GET("/content/articles", s.listArticles)
	s.engine.GET("/content/articles/search", s.searchArticles)
	s.engine.GET("/content/articles/get", s.getArticle)
	s.engine.GET("/content/articles/count", s.countArticles)
	s.engine.GET("/content/articles/by-author", s.listArticles)
	s.engine.GET("/content/articles/by-title", s.listArticles)
	s.engine.GET("/content/articles/by-content", s.listArticles)
	s.engine.GET("/content/articles/by-category", s.listArticles)
	s.engine.GET("/content/articles/by-tag", s.listArticles)
	s.engine.GET("/content/articles/by-collection", s.listArticles)
	s.engine.GET("/content/articles/self", s.listArticles)
	s.engine.POST("/content/articles", s.createArticle)
	s.engine.POST("/content/articles/update", s.updateArticle)
	s.engine.POST("/content/articles/update/title", s.updateArticleField("title"))
	s.engine.POST("/content/articles/update/content", s.updateArticleField("content"))
	s.engine.POST("/content/articles/update/summary", s.updateArticleField("summary"))
	s.engine.POST("/content/articles/update/category", s.updateArticleField("categoryId"))
	s.engine.POST("/content/articles/update/status/draft", s.updateArticleField("status"))
	s.engine.POST("/content/articles/update/status/public", s.updateArticleField("status"))
	s.engine.POST("/content/articles/delete", s.deleteArticle)
	s.engine.GET("/content/comments", s.listComments)
	s.engine.POST("/content/comments", s.createComment)
	s.engine.POST("/content/interactions/like", s.likeArticle)
	s.engine.POST("/content/interactions/collect", s.collectArticle)
	s.engine.POST("/content/interactions/like/check", s.checkLike)
	s.engine.POST("/content/interactions/like/create", s.likeArticleCreate)
	s.engine.POST("/content/interactions/like/delete", s.likeArticleDelete)
	s.engine.POST("/content/tags/create", s.createTag)
	s.engine.POST("/content/tags/update", s.updateTag)
	s.engine.POST("/content/tags/delete", s.deleteTag)
	s.engine.POST("/content/comment/delete", s.deleteComment)
	s.engine.POST("/content/comment/like", s.likeComment)
	s.engine.POST("/content/comment/dislike", s.dislikeComment)
	s.engine.POST("/content/collection/delete", s.deleteCollection)
	s.engine.POST("/content/collection/list", s.listCollections)
	s.engine.POST("/content/collection/check", s.checkCollection)
	s.engine.GET("/content/comments/liked", s.listCommentsLiked)
	s.engine.GET("/content/tags/suggest", s.suggestTags)
	s.engine.GET("/content/tags/list", s.listTags)
	s.engine.GET("/content/tags/list/full", s.listTagsFull)
	s.engine.GET("/content/categories/list", s.listCategories)
	s.engine.GET("/content/categories/get", s.getCategory)
	s.engine.GET("/content/categories/full", s.listCategories)
	s.engine.POST("/content/categories/create", s.createCategory)
	s.engine.POST("/content/categories/update/name", s.updateCategoryName)
	s.engine.POST("/content/categories/update/desc", s.updateCategoryDesc)
	s.engine.POST("/content/categories/update", s.updateCategory)
}

func (s *Server) listArticles(c *gin.Context) {
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	items, nextCursor := s.articles.List(c.Query("cursor"), limit)
	c.JSON(http.StatusOK, envelope{
		Code:    0,
		Message: "ok",
		TraceID: getTraceID(c),
		Data: gin.H{
			"items":      items,
			"nextCursor": nextCursor,
		},
	})
}

func (s *Server) searchArticles(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		q = strings.TrimSpace(c.Query("keyword"))
	}
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	hits, tokens, err := s.indexer.Search(q, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2501, Message: "search error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	if len(hits) == 0 {
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": []gin.H{}, "tokens": tokens}})
		return
	}
	ids := make([]string, len(hits))
	scores := make(map[string]int, len(hits))
	for i, h := range hits {
		ids[i] = h.ArticleID
		scores[h.ArticleID] = h.Score
	}
	rows, err := s.repo.GetArticlesByIDsPreserveOrder(ids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2502, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{
			"articleId":   row.ArticleID,
			"title":       row.Title,
			"summary":     row.Summary,
			"content":     row.Content,
			"authorId":    row.AuthorID,
			"categoryId":  row.CategoryID,
			"likeCount":   row.LikeCount,
			"updatedAt":   row.UpdatedAt.Format(time.RFC3339),
			"searchScore": scores[row.ArticleID],
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items, "tokens": tokens}})
}

func (s *Server) reindexArticle(articleID string) {
	if articleID == "" {
		return
	}
	row, err := s.repo.GetArticle(articleID)
	if err != nil {
		return
	}
	_ = s.indexer.IndexArticle(row.ArticleID, row.Title, row.Summary, row.Content)
}

func (s *Server) startSearchBackfill() {
	if os.Getenv("CONTENT_SEARCH_SKIP_BACKFILL") == "1" {
		return
	}
	const batch = 50
	offset := 0
	for {
		rows, err := s.repo.ListArticlesBatch(offset, batch)
		if err != nil || len(rows) == 0 {
			return
		}
		for _, row := range rows {
			_ = s.indexer.IndexArticle(row.ArticleID, row.Title, row.Summary, row.Content)
		}
		offset += len(rows)
		if len(rows) < batch {
			return
		}
	}
}

func (s *Server) createArticle(c *gin.Context) {
	var req createArticleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 2001, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	id, err := s.articles.Create(domain.CreateArticleCommand{
		Title:      req.Title,
		Summary:    req.Summary,
		Content:    req.Content,
		CategoryID: req.CategoryID,
		TagIDs:     req.TagIDs,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidArticleInput) {
			c.JSON(http.StatusBadRequest, envelope{Code: 2002, Message: "missing required fields", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		c.JSON(http.StatusInternalServerError, envelope{Code: 2500, Message: "internal error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	_ = s.indexer.IndexArticle(id, req.Title, req.Summary, req.Content)
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"articleId": id}})
}

func getTraceID(c *gin.Context) string {
	traceID, ok := c.Get("traceID")
	if !ok {
		return "content-unknown"
	}
	return traceID.(string)
}

func (s *Server) listComments(c *gin.Context) {
	articleID := c.Query("articleId")
	rows, err := s.repo.ListComments(articleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2102, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{
			"commentId": row.CommentID,
			"articleId": row.ArticleID,
			"userId":    row.UserID,
			"content":   row.Content,
			"createdAt": row.CreatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items}})
}

func (s *Server) createComment(c *gin.Context) {
	var req struct {
		ArticleID string `json:"articleId"`
		Content   string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ArticleID == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2101, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	userID := currentUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2104, Message: "userId required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	commentID, err := s.repo.CreateComment(req.ArticleID, userID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2103, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"commentId": commentID}})
}

func (s *Server) likeArticle(c *gin.Context) {
	s.genericInteraction(c, "like")
}

func (s *Server) collectArticle(c *gin.Context) {
	s.genericInteraction(c, "collect")
}

func (s *Server) genericInteraction(c *gin.Context, action string) {
	var req struct {
		ArticleID string `json:"articleId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ArticleID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2201, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	userID := currentUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2204, Message: "userId required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	var err error
	switch action {
	case "collect":
		err = s.repo.UpsertUserArticleRelation(userID, req.ArticleID, "collect")
	case "like", "like-create":
		err = s.repo.UpsertUserArticleRelation(userID, req.ArticleID, "like")
	case "like-delete":
		err = s.repo.DeleteUserArticleRelation(userID, req.ArticleID, "like")
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2203, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"success": true}})
}

func (s *Server) getArticle(c *gin.Context) {
	articleID := c.Query("ArticleID")
	if articleID == "" {
		articleID = c.Query("articleId")
	}
	if articleID == "" {
		items, _ := s.articles.List("", 1)
		if len(items) > 0 {
			c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: items[0]})
			return
		}
		c.JSON(http.StatusNotFound, envelope{Code: 2404, Message: "article not found", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	row, err := s.repo.GetArticle(articleID)
	if err == nil {
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{
			"articleId":  row.ArticleID,
			"title":      row.Title,
			"summary":    row.Summary,
			"content":    row.Content,
			"authorId":   row.AuthorID,
			"categoryId": row.CategoryID,
			"likeCount":  row.LikeCount,
			"updatedAt":  row.UpdatedAt,
		}})
		return
	}
	c.JSON(http.StatusNotFound, envelope{Code: 2404, Message: "article not found", TraceID: getTraceID(c), Data: gin.H{}})
}

func (s *Server) countArticles(c *gin.Context) {
	items, _ := s.articles.List("", 2000)
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"count": len(items)}})
}

func (s *Server) updateArticle(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, envelope{Code: 2301, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	articleID := ""
	if v, ok := req["ArticleID"].(string); ok {
		articleID = v
	}
	if articleID == "" {
		if v, ok := req["articleId"].(string); ok {
			articleID = v
		}
	}
	if articleID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2302, Message: "article id required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	delete(req, "ArticleID")
	delete(req, "articleId")
	if err := s.repo.UpdateArticle(articleID, req); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2303, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	s.reindexArticle(articleID)
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"updated": true}})
}

func (s *Server) updateArticleField(field string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req map[string]interface{}
		_ = c.ShouldBindJSON(&req)
		articleID := ""
		if v, ok := req["ArticleID"].(string); ok {
			articleID = v
		}
		if articleID == "" {
			if v, ok := req["articleId"].(string); ok {
				articleID = v
			}
		}
		if articleID == "" {
			c.JSON(http.StatusBadRequest, envelope{Code: 2304, Message: "article id required", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		value := req[field]
		if value == nil {
			for _, v := range req {
				value = v
				break
			}
		}
		if err := s.repo.UpdateArticle(articleID, map[string]interface{}{field: value}); err != nil {
			c.JSON(http.StatusInternalServerError, envelope{Code: 2305, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
		s.reindexArticle(articleID)
		c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"field": field, "updated": true}})
	}
}

func (s *Server) deleteArticle(c *gin.Context) {
	var req struct {
		ArticleID string `json:"ArticleID"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.ArticleID == "" {
		req.ArticleID = c.Query("ArticleID")
	}
	if req.ArticleID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2306, Message: "article id required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	if err := s.repo.DeleteArticle(req.ArticleID); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2307, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"deleted": true}})
}

func (s *Server) checkLike(c *gin.Context) {
	var req struct {
		ArticleID string `json:"ArticleID"`
	}
	_ = c.ShouldBindJSON(&req)
	userID := currentUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2205, Message: "userId required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	liked, err := s.repo.HasUserArticleRelation(userID, req.ArticleID, "like")
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2202, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"liked": liked}})
}

func (s *Server) suggestTags(c *gin.Context) {
	keyword := c.Query("keyword")
	cacheKey := "content:tags:suggest:" + keyword
	if cached, err := s.cache.Get(c.Request.Context(), cacheKey); err == nil {
		items := make([]gin.H, 0)
		if json.Unmarshal([]byte(cached), &items) == nil {
			c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items}})
			return
		}
	}
	rows, err := s.repo.ListTags(keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2602, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	items := make([]gin.H, 0)
	for _, t := range rows {
		items = append(items, gin.H{"tagId": t.TagID, "name": t.Name, "desc": t.Desc, "count": t.Count})
	}
	if payload, err := json.Marshal(items); err == nil {
		_ = s.cache.Set(c.Request.Context(), cacheKey, string(payload), 2*time.Minute)
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items}})
}

func (s *Server) listTags(c *gin.Context) {
	cacheKey := "content:tags:list"
	if cached, err := s.cache.Get(c.Request.Context(), cacheKey); err == nil {
		items := make([]gin.H, 0)
		if json.Unmarshal([]byte(cached), &items) == nil {
			c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items}})
			return
		}
	}
	rows, err := s.repo.ListTags("")
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2603, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, t := range rows {
		items = append(items, gin.H{"tagId": t.TagID, "name": t.Name, "desc": t.Desc, "count": t.Count})
	}
	if payload, err := json.Marshal(items); err == nil {
		_ = s.cache.Set(c.Request.Context(), cacheKey, string(payload), 2*time.Minute)
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items}})
}

func (s *Server) listTagsFull(c *gin.Context) { s.listTags(c) }

func (s *Server) createTag(c *gin.Context) {
	var req struct{ Name, Desc string }
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2601, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	if err := s.repo.CreateTag(req.Name, req.Desc); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2604, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	_ = s.cache.Delete(c.Request.Context(), "content:tags:list")
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"success": true}})
}

func (s *Server) updateTag(c *gin.Context) {
	var req struct {
		TagID int    `json:"TagID"`
		Name  string `json:"Name"`
		Desc  string `json:"Desc"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := s.repo.UpdateTag(req.TagID, req.Name, req.Desc); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2605, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	_ = s.cache.Delete(c.Request.Context(), "content:tags:list")
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"success": true}})
}

func (s *Server) deleteTag(c *gin.Context) {
	var req struct {
		TagID int `json:"TagID"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := s.repo.DeleteTag(req.TagID); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2606, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	_ = s.cache.Delete(c.Request.Context(), "content:tags:list")
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"success": true}})
}

func (s *Server) listCategories(c *gin.Context) {
	rows, err := s.repo.ListCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2702, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{"categoryId": row.CategoryID, "name": row.Name, "desc": row.Desc})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items}})
}

func (s *Server) getCategory(c *gin.Context) {
	rows, err := s.repo.ListCategories()
	if err != nil || len(rows) == 0 {
		c.JSON(http.StatusNotFound, envelope{Code: 2704, Message: "category not found", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{
		"categoryId": rows[0].CategoryID, "name": rows[0].Name, "desc": rows[0].Desc,
	}})
}

func (s *Server) createCategory(c *gin.Context) {
	var req struct{ Name, Desc string }
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2701, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	if err := s.repo.CreateCategory(req.Name, req.Desc); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2705, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"success": true}})
}

func (s *Server) updateCategoryName(c *gin.Context) { s.updateCategory(c) }
func (s *Server) updateCategoryDesc(c *gin.Context) { s.updateCategory(c) }

func (s *Server) updateCategory(c *gin.Context) {
	var req struct {
		CategoryID int    `json:"CategoryID"`
		Name       string `json:"Name"`
		Desc       string `json:"Desc"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := s.repo.UpdateCategory(req.CategoryID, req.Name, req.Desc); err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2706, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"success": true}})
}

func (s *Server) deleteComment(c *gin.Context) {
	var req struct {
		CommentID string `json:"CommentID"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.CommentID != "" {
		if err := s.repo.DeleteComment(req.CommentID); err != nil {
			c.JSON(http.StatusInternalServerError, envelope{Code: 2802, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"success": true}})
}

func (s *Server) likeComment(c *gin.Context) {
	s.commentLike(c, true)
}
func (s *Server) dislikeComment(c *gin.Context) {
	s.commentLike(c, false)
}
func (s *Server) commentLike(c *gin.Context, like bool) {
	var req struct {
		CommentID string `json:"CommentID"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.CommentID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2801, Message: "bad request", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	userID := currentUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2807, Message: "userId required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	if like {
		if err := s.repo.LikeComment(userID, req.CommentID); err != nil {
			c.JSON(http.StatusInternalServerError, envelope{Code: 2803, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
	} else {
		if err := s.repo.DislikeComment(userID, req.CommentID); err != nil {
			c.JSON(http.StatusInternalServerError, envelope{Code: 2804, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"success": true}})
}

func (s *Server) listCommentsLiked(c *gin.Context) {
	userID := currentUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2806, Message: "userId required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	rows, err := s.repo.ListLikedComments(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2805, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{"commentId": row.CommentID})
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items}})
}

func (s *Server) listCollections(c *gin.Context) {
	userID := currentUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2904, Message: "userId required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	rows, err := s.repo.ListUserArticleRelations(userID, "collect")
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2901, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	items := make([]string, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.ArticleID)
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"items": items}})
}

func (s *Server) deleteCollection(c *gin.Context) {
	var req struct {
		ArticleID string `json:"ArticleID"`
	}
	_ = c.ShouldBindJSON(&req)
	userID := currentUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2905, Message: "userId required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	if req.ArticleID != "" {
		if err := s.repo.DeleteUserArticleRelation(userID, req.ArticleID, "collect"); err != nil {
			c.JSON(http.StatusInternalServerError, envelope{Code: 2902, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
			return
		}
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"success": true}})
}

func (s *Server) checkCollection(c *gin.Context) {
	var req struct {
		ArticleID string `json:"ArticleID"`
	}
	_ = c.ShouldBindJSON(&req)
	userID := currentUserID(c)
	if userID == "" {
		c.JSON(http.StatusBadRequest, envelope{Code: 2906, Message: "userId required", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	ok, err := s.repo.HasUserArticleRelation(userID, req.ArticleID, "collect")
	if err != nil {
		c.JSON(http.StatusInternalServerError, envelope{Code: 2903, Message: "db error", TraceID: getTraceID(c), Data: gin.H{}})
		return
	}
	c.JSON(http.StatusOK, envelope{Code: 0, Message: "ok", TraceID: getTraceID(c), Data: gin.H{"collected": ok}})
}

func (s *Server) likeArticleCreate(c *gin.Context) { s.genericInteraction(c, "like-create") }
func (s *Server) likeArticleDelete(c *gin.Context) { s.genericInteraction(c, "like-delete") }

func currentUserID(c *gin.Context) string {
	if userID := c.Query("userId"); userID != "" {
		return userID
	}
	if userID := c.Query("UserID"); userID != "" {
		return userID
	}
	if userID := c.GetHeader("X-User-Id"); userID != "" {
		return userID
	}
	return ""
}
