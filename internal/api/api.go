// HTTP API — Gin 路由定义，配置管理、日志查询、安全报告
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/Samantha09/mcpguard/internal/store"
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

func (s *Server) handleHealth(c *gin.Context)           {}
func (s *Server) handleListLogs(c *gin.Context)         {}
func (s *Server) handleListPolicies(c *gin.Context)     {}
func (s *Server) handleCreatePolicy(c *gin.Context)     {}
func (s *Server) handleGetPolicy(c *gin.Context)        {}
func (s *Server) handleUpdatePolicy(c *gin.Context)     {}
func (s *Server) handleDeletePolicy(c *gin.Context)     {}
func (s *Server) handleListRules(c *gin.Context)        {}
func (s *Server) handleCreateRule(c *gin.Context)       {}
func (s *Server) handleReportSummary(c *gin.Context)    {}
