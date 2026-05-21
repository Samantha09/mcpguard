// HTTP API — Gin 路由定义，配置管理、日志查询、安全报告
package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/Samantha09/mcpguard/internal/models"
	"github.com/Samantha09/mcpguard/internal/store"
	"github.com/gin-gonic/gin"
)

// Server API 服务
type Server struct {
	store  store.Store
	router *gin.Engine
}

// NewServer 创建 API 服务
func NewServer(s store.Store) *Server {
	srv := &Server{store: s}
	srv.setupRoutes()
	return srv
}

// setupRoutes 注册路由
func (s *Server) setupRoutes() {
	r := gin.Default()

	// 健康检查
	r.GET("/health", s.handleHealth)

	// 日志相关
	logs := r.Group("/api/logs")
	{
		logs.GET("", s.handleListLogs)
	}

	// 策略相关
	policies := r.Group("/api/policies")
	{
		policies.GET("", s.handleListPolicies)
		policies.POST("", s.handleCreatePolicy)
		policies.GET("/:id", s.handleGetPolicy)
		policies.PUT("/:id", s.handleUpdatePolicy)
		policies.DELETE("/:id", s.handleDeletePolicy)
	}

	// 规则相关
	rules := r.Group("/api/rules")
	{
		rules.GET("", s.handleListRules)
		rules.POST("", s.handleCreateRule)
	}

	// 安全报告
	reports := r.Group("/api/reports")
	{
		reports.GET("/summary", s.handleReportSummary)
	}

	s.router = r
}

// Run 启动 API 服务
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// 路由处理函数 — 后续实现

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleListLogs(c *gin.Context) {
	filter := store.LogFilter{}

	if action := c.Query("action"); action != "" {
		a := models.Action(action)
		filter.Action = &a
	}
	if method := c.Query("method"); method != "" {
		filter.Method = method
	}
	if toolName := c.Query("tool_name"); toolName != "" {
		filter.ToolName = toolName
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = n
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if n, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = n
		}
	}

	logs, err := s.store.QueryLogs(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, logs)
}

func (s *Server) handleListPolicies(c *gin.Context) {
	policies, err := s.store.ListPolicies(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policies)
}

func (s *Server) handleCreatePolicy(c *gin.Context) {
	var p models.Policy
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.store.UpsertPolicy(c.Request.Context(), &p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (s *Server) handleGetPolicy(c *gin.Context) {
	id := c.Param("id")
	p, err := s.store.GetPolicy(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (s *Server) handleUpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	var p models.Policy
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if p.ID != "" && p.ID != id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id mismatch"})
		return
	}
	p.ID = id
	if err := s.store.UpsertPolicy(c.Request.Context(), &p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (s *Server) handleDeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := s.store.DeletePolicy(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) handleListRules(c *gin.Context) {
	rules, err := s.store.ListRules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (s *Server) handleCreateRule(c *gin.Context) {
	var r models.Rule
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 检查 ID 是否已存在
	if _, err := s.store.GetRule(c.Request.Context(), r.ID); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "rule already exists"})
		return
	}
	if err := s.store.CreateRule(c.Request.Context(), &r); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, r)
}

func (s *Server) handleReportSummary(c *gin.Context) {}
