// MCP 代理核心 — 透明中间人代理，支持 stdio 和 SSE 双模式
package proxy

import (
	"context"

	"github.com/Samantha09/mcpguard/internal/detector"
	"github.com/Samantha09/mcpguard/internal/models"
)

// Proxy MCP 代理接口
type Proxy interface {
	// Start 启动代理
	Start(ctx context.Context) error
	// Stop 停止代理
	Stop() error
}

// Config 代理配置
type Config struct {
	Transport models.Transport  `json:"transport"`
	Servers   []models.ServerConfig `json:"servers"`
	Listen    string            `json:"listen"` // SSE 模式监听地址，如 ":8080"
}

// MCProxy MCP 代理实现
type MCProxy struct {
	config    Config
	pipeline  *detector.Pipeline
}

// New 创建 MCP 代理
func New(cfg Config, pipeline *detector.Pipeline) *MCProxy {
	return &MCProxy{
		config:   cfg,
		pipeline: pipeline,
	}
}

func (p *MCProxy) Start(ctx context.Context) error {
	// TODO: 根据 Transport 类型启动 stdio 或 SSE 代理
	return nil
}

func (p *MCProxy) Stop() error {
	// TODO: 清理连接
	return nil
}

// interceptRequest 拦截并检测请求
func (p *MCProxy) interceptRequest(ctx context.Context, req *models.InterceptedRequest) (*models.DetectResult, error) {
	// TODO: 调用 pipeline.Run
	return &models.DetectResult{Action: models.ActionAllow}, nil
}

// interceptResponse 拦截并检测响应
func (p *MCProxy) interceptResponse(ctx context.Context, resp *models.InterceptedResponse) (*models.DetectResult, error) {
	// TODO: 响应检测逻辑
	return &models.DetectResult{Action: models.ActionAllow}, nil
}
