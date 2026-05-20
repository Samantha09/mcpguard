// MCP 代理核心 — 透明中间人代理，支持 stdio 和 SSE 双模式
package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/Samantha09/mcpguard/internal/detector"
	"github.com/Samantha09/mcpguard/internal/models"
	"github.com/Samantha09/mcpguard/internal/probe"
	"github.com/Samantha09/mcpguard/internal/store"
	"github.com/Samantha09/mcpguard/pkg/jsonrpc"
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
	Transport models.Transport      `json:"transport"`
	Servers   []models.ServerConfig `json:"servers"`
	Listen    string                `json:"listen"` // SSE 模式监听地址，如 ":8080"
}

// MCProxy MCP 代理实现
type MCProxy struct {
	config      Config
	pipeline    *detector.Pipeline
	store       store.Store
	probeClient *probe.Client // 探针客户端（可选）
	probeID     string        // 探针 ID

	// 运行时，测试中可注入
	agentIn   io.Reader
	agentOut  io.Writer
	serverIn  io.Writer
	serverOut io.Reader
	cmd       *exec.Cmd
}

// New 创建 MCP 代理
func New(cfg Config, pipeline *detector.Pipeline, s store.Store, pc *probe.Client) *MCProxy {
	p := &MCProxy{
		config:      cfg,
		pipeline:    pipeline,
		store:       s,
		probeClient: pc,
	}
	if pc != nil {
		p.probeID = pc.ProbeID()
	}
	return p
}

func (p *MCProxy) Start(ctx context.Context) error {
	if len(p.config.Servers) == 0 {
		return fmt.Errorf("没有配置 MCP Server")
	}
	srv := p.config.Servers[0]

	if p.agentIn == nil {
		p.agentIn = os.Stdin
	}
	if p.agentOut == nil {
		p.agentOut = os.Stdout
	}

	p.cmd = exec.CommandContext(ctx, srv.Command, srv.Args...)
	p.cmd.Env = append(os.Environ(), srv.Env...)

	stdin, err := p.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建子进程 stdin 管道失败: %w", err)
	}
	p.serverIn = stdin

	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建子进程 stdout 管道失败: %w", err)
	}
	p.serverOut = stdout

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("启动 MCP Server 失败: %w", err)
	}

	return p.runLoop(ctx)
}

func (p *MCProxy) Stop() error {
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Process.Kill()
	}
	return nil
}

// runLoop 核心 I/O 循环：读取 Agent 请求 → 检测 → 转发/拦截 → 返回响应
func (p *MCProxy) runLoop(ctx context.Context) error {
	agentReader := bufio.NewReader(p.agentIn)
	serverReader := bufio.NewReader(p.serverOut)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := agentReader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		resultLine, err := p.handleLine(ctx, line)
		if err != nil {
			return err
		}

		if resultLine != nil {
			if _, err := p.agentOut.Write(resultLine); err != nil {
				return err
			}
			continue
		}

		// 转发给 Server，等待响应
		if _, err := p.serverIn.Write(line); err != nil {
			return err
		}

		respLine, err := serverReader.ReadBytes('\n')
		if err != nil {
			return err
		}
		if _, err := p.agentOut.Write(respLine); err != nil {
			return err
		}
	}
}

// handleLine 处理单行 JSON-RPC 消息
// 返回非 nil 表示已构造响应（拦截），返回 nil 表示需要转发给 Server
func (p *MCProxy) handleLine(ctx context.Context, line []byte) ([]byte, error) {
	var req jsonrpc.Request
	if err := json.Unmarshal(line, &req); err != nil {
		// JSON 解析失败，原样转发
		return nil, nil
	}

	intercepted := parseInterceptedRequest(&req)

	if intercepted.Method == models.MethodToolsCall && intercepted.ToolCall != nil {
		result, _ := p.pipeline.Run(ctx, intercepted)
		p.logRequest(string(line), intercepted, result)

		if result.Action == models.ActionBlock {
			resp := jsonrpc.NewErrorResponse(req.ID, -32000, "请求被安全策略拦截: "+result.Reason)
			out, _ := json.Marshal(resp)
			return append(out, '\n'), nil
		}
	}

	return nil, nil
}

// parseInterceptedRequest 将 jsonrpc.Request 解析为内部模型
func parseInterceptedRequest(raw *jsonrpc.Request) *models.InterceptedRequest {
	req := &models.InterceptedRequest{
		ID:     raw.ID,
		Method: models.MCPRequestMethod(raw.Method),
		Params: make(map[string]any),
	}

	if m, ok := raw.Params.(map[string]any); ok {
		req.Params = m
		if req.Method == models.MethodToolsCall {
			if name, ok := m["name"].(string); ok {
				req.ToolCall = &models.ToolCallInfo{
					Name:      name,
					Arguments: make(map[string]any),
				}
				if args, ok := m["arguments"].(map[string]any); ok {
					req.ToolCall.Arguments = args
				}
			}
		}
	}

	return req
}

// logRequest 记录审计日志
func (p *MCProxy) logRequest(raw string, req *models.InterceptedRequest, result *models.DetectResult) {
	if p.store == nil {
		return
	}
	entry := &models.LogEntry{
		Direction: "request",
		Method:    string(req.Method),
		Action:    result.Action,
		Reason:    result.Reason,
		Request:   raw,
		Detector:  result.Detector,
		ProbeID:   p.probeID,
	}
	if req.ToolCall != nil {
		entry.ToolName = req.ToolCall.Name
	}
	if err := p.store.InsertLog(context.Background(), entry); err != nil {
		// 日志写入失败不应阻断请求
	}

	// 上报到平台（异步，不阻塞）
	if p.probeClient != nil {
		go func() {
			_ = p.probeClient.SendLog(entry)
		}()
	}
}

func (p *MCProxy) interceptRequest(ctx context.Context, req *models.InterceptedRequest) (*models.DetectResult, error) {
	return p.pipeline.Run(ctx, req)
}

func (p *MCProxy) interceptResponse(ctx context.Context, resp *models.InterceptedResponse) (*models.DetectResult, error) {
	return &models.DetectResult{Action: models.ActionAllow}, nil
}
