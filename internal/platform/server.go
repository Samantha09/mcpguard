// 平台服务 — REST API + WebSocket
package platform

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/Samantha09/mcpguard/internal/models"
	"github.com/Samantha09/mcpguard/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server 平台服务器
type Server struct {
	store   store.Store
	router  *gin.Engine
	clients map[string]*websocket.Conn // probe_id -> conn
}

// NewServer 创建平台服务器
func NewServer(s store.Store) *Server {
	srv := &Server{
		store:   s,
		clients: make(map[string]*websocket.Conn),
	}
	srv.setupRoutes()
	return srv
}

func (s *Server) setupRoutes() {
	r := gin.Default()

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 探针 API（探针调用）
	v1 := r.Group("/api/v1")
	{
		v1.POST("/probes/register", s.handleRegisterProbe)
		v1.GET("/rules", s.handleListRules)
		v1.GET("/rules/:id", s.handleGetRule)
		v1.POST("/logs/batch", s.handleBatchLogs)
		v1.GET("/ws", s.handleWebSocket)
	}

	// 管理 API（管理员调用）
	admin := r.Group("/api/v1")
	{
		admin.GET("/probes", s.handleListProbes)
		admin.GET("/probes/:id", s.handleGetProbe)
		admin.POST("/rules", s.handleCreateRule)
		admin.PUT("/rules/:id", s.handleUpdateRule)
		admin.DELETE("/rules/:id", s.handleDeleteRule)
		admin.GET("/logs", s.handleListLogs)
		admin.GET("/reports/summary", s.handleReportSummary)
	}

	s.router = r
}

// Run 启动平台服务
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}

// Router 返回 gin.Engine（测试用）
func (s *Server) Router() *gin.Engine {
	return s.router
}

// --- 探针注册 ---

func (s *Server) handleRegisterProbe(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		Hostname string `json:"hostname"`
		IP       string `json:"ip"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 生成 token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generate token failed"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	probe := &models.Probe{
		ID:            generateID(),
		Name:          req.Name,
		Hostname:      req.Hostname,
		IP:            req.IP,
		Status:        "offline",
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
	}

	if err := s.store.RegisterProbe(c.Request.Context(), probe, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"probe_id": probe.ID,
		"token":    token,
	})
}

// --- 规则管理 ---

func (s *Server) handleListRules(c *gin.Context) {
	rules, err := s.store.ListRules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (s *Server) handleGetRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := s.store.GetRule(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (s *Server) handleCreateRule(c *gin.Context) {
	var rule models.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if rule.ID == "" {
		rule.ID = generateID()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	if err := s.store.CreateRule(c.Request.Context(), &rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (s *Server) handleUpdateRule(c *gin.Context) {
	id := c.Param("id")
	var rule models.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.ID = id
	rule.UpdatedAt = time.Now()
	if err := s.store.UpdateRule(c.Request.Context(), &rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (s *Server) handleDeleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := s.store.DeleteRule(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// --- 探针管理 ---

func (s *Server) handleListProbes(c *gin.Context) {
	probes, err := s.store.ListProbes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, probes)
}

func (s *Server) handleGetProbe(c *gin.Context) {
	id := c.Param("id")
	probe, err := s.store.GetProbe(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "probe not found"})
		return
	}
	c.JSON(http.StatusOK, probe)
}

// --- 日志查询 ---

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
	logs, err := s.store.QueryLogs(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, logs)
}

func (s *Server) handleBatchLogs(c *gin.Context) {
	var entries []models.LogEntry
	if err := c.ShouldBindJSON(&entries); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	for _, entry := range entries {
		_ = s.store.InsertLog(ctx, &entry)
	}
	c.JSON(http.StatusOK, gin.H{"inserted": len(entries)})
}

// --- 安全报告 ---

func (s *Server) handleReportSummary(c *gin.Context) {
	logs, err := s.store.QueryLogs(c.Request.Context(), store.LogFilter{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var blockCount, warnCount, allowCount int
	for _, log := range logs {
		switch log.Action {
		case models.ActionBlock:
			blockCount++
		case models.ActionWarn:
			warnCount++
		case models.ActionAllow:
			allowCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total": len(logs),
		"block": blockCount,
		"warn":  warnCount,
		"allow": allowCount,
	})
}

// --- WebSocket ---

func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	clientID := c.Request.RemoteAddr
	s.clients[clientID] = conn
	defer delete(s.clients, clientID)

	for {
		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		if msgType, ok := msg["type"].(string); ok && msgType == "heartbeat" {
			_ = conn.WriteJSON(map[string]any{"type": "pong"})
		}
	}
}

// Broadcast 向所有 WebSocket 客户端广播消息
func (s *Server) Broadcast(msg map[string]any) {
	for _, conn := range s.clients {
		_ = conn.WriteJSON(msg)
	}
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
