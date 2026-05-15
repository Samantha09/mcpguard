// 数据模型 — 定义 MCPGuard 全局共享的数据结构
package models

import "time"

// Action 检测策略动作
type Action string

const (
	ActionAllow Action = "allow" // 放行
	ActionBlock Action = "block" // 拦截
	ActionWarn  Action = "warn"  // 告警但放行
)

// Transport MCP 传输协议类型
type Transport string

const (
	TransportStdio Transport = "stdio" // 标准输入输出
	TransportSSE   Transport = "sse"   // Server-Sent Events / Streamable HTTP
)

// MCPRequestMethod MCP 协议方法名
type MCPRequestMethod string

const (
	MethodInitialize      MCPRequestMethod = "initialize"
	MethodToolsList       MCPRequestMethod = "tools/list"
	MethodToolsCall       MCPRequestMethod = "tools/call"
	MethodResourcesList   MCPRequestMethod = "resources/list"
	MethodResourcesRead   MCPRequestMethod = "resources/read"
	MethodPromptsList     MCPRequestMethod = "prompts/list"
	MethodPromptsGet      MCPRequestMethod = "prompts/get"
	MethodNotification    MCPRequestMethod = "notifications/*"
)

// ToolCallInfo 工具调用信息（从 tools/call 请求中提取）
type ToolCallInfo struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// DetectResult 检测结果
type DetectResult struct {
	Action     Action  `json:"action"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
	Detector   string  `json:"detector"` // 产生此结果的检测器名称
}

// InterceptedRequest 被拦截的请求（用于检测流水线输入）
type InterceptedRequest struct {
	ID     any             `json:"id"`
	Method MCPRequestMethod `json:"method"`
	Params map[string]any  `json:"params"`
	// 如果是 tools/call，解析后的工具信息
	ToolCall *ToolCallInfo `json:"tool_call,omitempty"`
}

// InterceptedResponse 被拦截的响应
type InterceptedResponse struct {
	ID     any    `json:"id"`
	Result any    `json:"result,omitempty"`
	Error  *RPCError `json:"error,omitempty"`
}

// RPCError JSON-RPC 错误
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Policy 拦截策略
type Policy struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
	RuleIDs     []string `json:"rule_ids"`  // 关联的规则 ID
	LLMEnabled  bool     `json:"llm_enabled"` // 是否启用 LLM 检测
}

// LogEntry 审计日志条目
type LogEntry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Direction string    `json:"direction"` // "request" 或 "response"
	Method    string    `json:"method"`
	ToolName  string    `json:"tool_name,omitempty"`
	Action    Action    `json:"action"`
	Reason    string    `json:"reason,omitempty"`
	Request   string    `json:"request"`  // 原始 JSON
	Response  string    `json:"response,omitempty"` // 原始 JSON
	ClientID  string    `json:"client_id,omitempty"`
}

// ServerConfig 上游 MCP Server 配置
type ServerConfig struct {
	Name      string    `json:"name"`
	Transport Transport `json:"transport"`
	// stdio 模式
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
	// SSE 模式
	URL string `json:"url,omitempty"`
}
