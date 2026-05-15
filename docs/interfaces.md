# MCPGuard 接口文档

## 架构总览

```
Agent ←→ MCProxy ←→ MCP Server
              │
         Detector Pipeline
         ┌────┴────┐
     RuleDetector  LLMDetector
                      │
                  LLMBackend (OpenAI / Ollama / ...)
```

代理拦截 JSON-RPC 消息，经检测流水线判断后放行、拦截或告警。审计日志写入 SQLite，管理界面通过 HTTP API 暴露。

---

## 核心接口

### Detector（检测器）

```go
// internal/detector/detector.go
type Detector interface {
    Detect(ctx context.Context, req *models.InterceptedRequest) (*models.DetectResult, error)
    Name() string
}
```

所有检测器统一实现此接口。Pipeline 按注册顺序依次调用，任一返回 Block 则拦截。

**已知实现：**
| 实现类型 | 包 | 说明 |
|---------|-----|------|
| `RuleDetector` | `internal/rules` | 基于规则匹配（关键词/正则/路径/工具名） |
| `LLMDetector` | 待建 | 通过 LLM 判断模糊场景 |

### Pipeline（检测流水线）

```go
// internal/detector/detector.go
type Pipeline struct { detectors []Detector }

func NewPipeline(detectors ...Detector) *Pipeline
func (p *Pipeline) AddDetector(d Detector)
func (p *Pipeline) Run(ctx context.Context, req *models.InterceptedRequest) (*models.DetectResult, error)
```

Run 执行逻辑：顺序调用所有 Detector，返回最严重的 Action（Block > Warn > Allow）。

### Proxy（代理）

```go
// internal/proxy/proxy.go
type Proxy interface {
    Start(ctx context.Context) error
    Stop() error
}
```

**实现：** `MCProxy`，根据 Config.Transport 启动 stdio 或 SSE 模式。

```go
type MCProxy struct { ... }

func New(cfg Config, pipeline *detector.Pipeline) *MCProxy
func (p *MCProxy) Start(ctx context.Context) error
func (p *MCProxy) Stop() error
```

### LLMBackend（LLM 后端）

```go
// internal/llm/llm.go
type Backend interface {
    Complete(ctx context.Context, prompt string) (string, error)
    Name() string
}
```

**已知实现：**
| 实现类型 | Name() | 说明 |
|---------|--------|------|
| `OpenAIBackend` | `"openai"` | OpenAI 兼容 API（支持自定义 BaseURL） |

后续扩展只需实现 Backend 接口并注册。

### Store（存储）

```go
// internal/store/store.go
type Store interface {
    Init(ctx context.Context) error
    Close() error

    // 审计日志
    InsertLog(ctx context.Context, entry *models.LogEntry) error
    QueryLogs(ctx context.Context, filter LogFilter) ([]*models.LogEntry, error)

    // 策略管理
    UpsertPolicy(ctx context.Context, policy *models.Policy) error
    GetPolicy(ctx context.Context, id string) (*models.Policy, error)
    ListPolicies(ctx context.Context) ([]*models.Policy, error)
    DeletePolicy(ctx context.Context, id string) error
}
```

**实现：** `SQLiteStore`，基于 `database/sql` + `modernc.org/sqlite`（纯 Go，无 CGO）。

---

## 数据模型

### 枚举类型

```go
// internal/models/models.go

type Action string   // "allow" | "block" | "warn"
type Transport string // "stdio" | "sse"
type MCPRequestMethod string // "initialize" | "tools/list" | "tools/call" | ...
```

### 检测相关

```go
type ToolCallInfo struct {
    Name      string         `json:"name"`
    Arguments map[string]any `json:"arguments"`
}

type InterceptedRequest struct {
    ID       any               `json:"id"`
    Method   MCPRequestMethod  `json:"method"`
    Params   map[string]any    `json:"params"`
    ToolCall *ToolCallInfo     `json:"tool_call,omitempty"` // tools/call 时填充
}

type InterceptedResponse struct {
    ID     any       `json:"id"`
    Result any       `json:"result,omitempty"`
    Error  *RPCError `json:"error,omitempty"`
}

type DetectResult struct {
    Action     Action  `json:"action"`
    Reason     string  `json:"reason"`
    Confidence float64 `json:"confidence"`
    Detector   string  `json:"detector"` // 产生此结果的检测器名
}
```

### 策略与规则

```go
type Policy struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Enabled     bool     `json:"enabled"`
    RuleIDs     []string `json:"rule_ids"`
    LLMEnabled  bool     `json:"llm_enabled"`
}

// internal/rules/rules.go
type RuleType string // "file_path" | "keyword" | "regex" | "tool_name"

type Rule struct {
    ID          string        `json:"id"`
    Name        string        `json:"name"`
    Type        RuleType      `json:"type"`
    Pattern     string        `json:"pattern"`
    Action      models.Action `json:"action"`
    Enabled     bool          `json:"enabled"`
    Description string        `json:"description"`
}
```

### 审计日志

```go
type LogEntry struct {
    ID        int64     `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    Direction string    `json:"direction"` // "request" | "response"
    Method    string    `json:"method"`
    ToolName  string    `json:"tool_name,omitempty"`
    Action    Action    `json:"action"`
    Reason    string    `json:"reason,omitempty"`
    Request   string    `json:"request"`            // 原始 JSON
    Response  string    `json:"response,omitempty"` // 原始 JSON
    ClientID  string    `json:"client_id,omitempty"`
}
```

### 配置

```go
// internal/config/config.go
type AppConfig struct {
    LogLevel string      `json:"log_level"` // debug|info|warn|error
    DBPath   string      `json:"db_path"`
    Proxy    proxy.Config `json:"proxy"`
    LLM      LLMConfig   `json:"llm"`
    API      APIConfig   `json:"api"`
}

// internal/proxy/proxy.go
type Config struct {  // proxy.Config
    Transport models.Transport      `json:"transport"`
    Servers   []models.ServerConfig `json:"servers"`
    Listen    string                `json:"listen"` // SSE 模式监听地址
}

// internal/llm/llm.go
type OpenAIConfig struct {
    APIKey  string `json:"api_key"`
    BaseURL string `json:"base_url"` // 默认 https://api.openai.com/v1
    Model   string `json:"model"`    // 默认 gpt-4o
}
```

---

## HTTP API

基础路径：`http://<host>:9090`

### 健康检查

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 服务健康状态 |

### 审计日志

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/logs` | 查询日志列表 |

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| start_time | ISO8601 | 起始时间 |
| end_time | ISO8601 | 结束时间 |
| action | string | `allow` / `block` / `warn` |
| method | string | MCP 方法名 |
| tool_name | string | 工具名 |
| limit | int | 分页大小 |
| offset | int | 分页偏移 |

**响应：**
```json
[
  {
    "id": 1,
    "timestamp": "2026-05-14T10:00:00Z",
    "direction": "request",
    "method": "tools/call",
    "tool_name": "execute_command",
    "action": "block",
    "reason": "匹配规则: 危险命令 rm -rf",
    "request": "{...}",
    "client_id": "agent-1"
  }
]
```

### 策略管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/policies` | 列出所有策略 |
| POST | `/api/policies` | 创建策略 |
| GET | `/api/policies/:id` | 获取单个策略 |
| PUT | `/api/policies/:id` | 更新策略 |
| DELETE | `/api/policies/:id` | 删除策略 |

**Policy 请求体（POST/PUT）：**
```json
{
  "id": "policy-1",
  "name": "禁止文件删除",
  "description": "拦截所有删除文件的工具调用",
  "enabled": true,
  "rule_ids": ["rule-1", "rule-2"],
  "llm_enabled": false
}
```

### 规则管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/rules` | 列出所有规则 |
| POST | `/api/rules` | 创建规则 |

**Rule 请求体（POST）：**
```json
{
  "id": "rule-1",
  "name": "拦截 rm -rf",
  "type": "keyword",
  "pattern": "rm -rf",
  "action": "block",
  "enabled": true,
  "description": "禁止递归强制删除"
}
```

### 安全报告

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/reports/summary` | 安全摘要统计 |

**响应：**
```json
{
  "total_requests": 1500,
  "blocked": 23,
  "warned": 45,
  "allowed": 1432,
  "top_blocked_tools": ["execute_command", "file_write"],
  "top_triggered_rules": ["rule-1", "rule-5"]
}
```

---

## JSON-RPC 2.0

```go
// pkg/jsonrpc/jsonrpc.go
type Request  struct { JSONRPC, ID, Method, Params }
type Response struct { JSONRPC, ID, Result, Error }
type Error    struct { Code, Message, Data }
type Notification struct { JSONRPC, Method, Params }
```

**标准错误码：** `CodeParseError(-32700)` `CodeInvalidRequest(-32600)` `CodeMethodNotFound(-32601)` `CodeInvalidParams(-32602)` `CodeInternalError(-32603)`

**工厂函数：** `NewRequest(id, method, params)` `NewResponse(id, result)` `NewErrorResponse(id, code, message)`

---

## 数据流

```
1. Agent 发送 JSON-RPC 请求
2. MCProxy 接收，解析为 InterceptedRequest
3. 若是 tools/call，提取 ToolCallInfo
4. Pipeline.Run() 依次调用所有 Detector
5. 若 Action=Block，返回错误响应，记录 LogEntry
6. 若 Action=Allow/Warn，转发给 MCP Server
7. MCP Server 返回响应
8. MCProxy 拦截响应，解析为 InterceptedResponse
9. 响应检测（可选）
10. 返回给 Agent，记录 LogEntry
```
