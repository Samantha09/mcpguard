# MCPGuard 架构设计文档

> 版本：v0.1 MVP
> 日期：2026-05-15
> 状态：草案

---

## 1. 设计目标

MCPGuard 是一个 MCP（Model Context Protocol）安全护栏，在 Agent 与 MCP Server 之间充当不可绕过的透明代理，检测并拦截不安全的工具调用。

**MVP 目标**：
- 能代理 stdio MCP Server
- 能用内置规则检测危险工具调用
- 能把每一次拦截记录到 SQLite
- 能通过 HTTP API 查询日志

**非目标**（后续迭代）：
- SSE / HTTP 传输模式
- LLM 智能检测
- Web 管理界面
- 响应内容检测
- 策略热重载
- 密码学签名审计

---

## 2. 核心设计决策

### 2.1 传输层所有权

MCPGuard 必须拥有传输协议的控制权。Agent 不直接连接 MCP Server，而是连接 MCPGuard，由 MCPGuard 启动并代理真正的 MCP Server。

**理由**：
- SDK 包装方式可被 Agent 绕过（如果 Agent 被 prompt injection 控制）
- 代理层拥有传输 = 零绕过可能
- 竞品（Pipelock、Agentgateway、ClawShield）均采用此模式

### 2.2 传输协议与检测逻辑解耦

检测引擎只处理标准化的 `InterceptedRequest` / `InterceptedResponse`，不感知底层是 stdio 还是 SSE。

**理由**：
- 未来扩展 SSE 传输时，检测引擎零改动
- 符合 Agentgateway 的多传输抽象设计

### 2.3 规则检测优先，LLM 检测延后

MVP 只实现基于规则的检测（keyword / tool_name / regex），不依赖外部 LLM API。

**理由**：
- Pipelock 和 ClawShield 均用纯本地规则覆盖 80%+ 风险场景
- LLM 检测需要 API Key、有成本、延迟高
- 规则检测零外部依赖，可离线运行

### 2.4 请求检测优先，响应检测延后

MVP 只对 Agent 发出的请求做检测，MCP Server 返回的响应原样透传。

**理由**：
- 危险操作（rm -rf、文件删除）发生在请求阶段
- 响应检测主要用于防御 prompt injection，属于增强功能
- 先闭环请求检测，再扩展响应检测

### 2.5 SQLite 足够支撑 MVP 审计

审计日志使用 SQLite 持久化，不引入外部数据库。

**理由**：
- ClawShield 用 SQLite 做取证日志已证明可行
- 单文件、零配置、与 Go 二进制一起分发
- 日志量可控（MCP 调用频率远低于 HTTP 请求）

---

## 3. 架构概览

```
┌─────────────┐      stdin/stdout      ┌──────────────┐      stdin/stdout      ┌─────────────┐
│             │ ──────────────────────>│              │ ──────────────────────>│             │
│   Agent     │                        │  MCPGuard    │                        │  MCP Server │
│  (Client)   │ <──────────────────────│  (Proxy)     │ <──────────────────────│  (Child)    │
│             │   JSON-RPC Response    │              │   JSON-RPC Response    │             │
└─────────────┘                        └──────┬───────┘                        └─────────────┘
                                              │
                                              │ Detect
                                              │
                                       ┌──────┴───────┐
                                       │   Pipeline   │
                                       │  ┌────────┐  │
                                       │  │ Rules  │  │
                                       │  │Detector│  │
                                       │  └────────┘  │
                                       └──────────────┘
                                              │
                                              │ Log
                                              │
                                       ┌──────┴───────┐
                                       │  SQLiteStore │
                                       └──────────────┘
                                              │
                                              │ Query
                                              │
                                       ┌──────┴───────┐
                                       │  HTTP API    │
                                       │  GET /logs   │
                                       └──────────────┘
```

### 3.1 启动流程

```
1. 加载配置（默认配置 + 命令行参数）
2. 初始化日志（slog）
3. 初始化 SQLite 存储（建表）
4. 加载内置规则 → 创建 RuleDetector → 注入 Pipeline
5. 创建 MCProxy（持有 Pipeline）
6. 启动 HTTP API 服务（后台 goroutine）
7. 启动 MCProxy（阻塞主 goroutine）
8. 等待 SIGINT/SIGTERM，优雅关闭
```

---

## 4. 模块设计

### 4.1 Proxy（代理层）

**职责**：拥有传输协议，在 Agent 与 MCP Server 之间双向转发 JSON-RPC 消息，在转发前调用检测流水线。

**MVP 范围**：仅实现 stdio 模式。

**stdio 代理实现**：

```go
type MCProxy struct {
    command   string        // MCP Server 启动命令，如 "npx -y @server"
    args      []string      // 命令参数
    pipeline  *detector.Pipeline
    store     store.Store

    // 运行时
    cmd       *exec.Cmd
    stdin     io.WriteCloser    // 指向 MCP Server 子进程的 stdin
    stdout    io.ReadCloser     // 指向 MCP Server 子进程的 stdout
    agentStdin io.Reader        // 指向 Agent 的 stdin（os.Stdin）
    agentStdout io.Writer       // 指向 Agent 的 stdout（os.Stdout）
}
```

**数据流**：

```
Agent stdout ──> [MCPGuard 读取] ──> JSON-RPC 解析 ──> Pipeline.Run()
                                              │
                                              ├─ Block ──> 返回错误响应给 Agent
                                              │
                                              └─ Allow ──> 写入 MCP Server stdin

MCP Server stdout ──> [MCPGuard 读取] ──> 直接写入 Agent stdin（MVP 不过滤响应）
```

**关键设计点**：
- 使用 `bufio.Scanner` 或 `json.Decoder` 按行读取 JSON-RPC（每行一条消息）
- Agent → Server 方向：读取 → 解析 → 检测 → 放行/拦截
- Server → Agent 方向：直接透传（MVP 不解析响应）
- 拦截时：不向 Server 转发，直接构造 JSON-RPC Error Response 返回给 Agent
- 拦截后写审计日志（无论 Block 还是 Allow）
- 使用 `context.Context` 管理生命周期，支持优雅关闭

**错误处理**：
- MCP Server 子进程异常退出：记录日志，向 Agent 返回 internal error，尝试重启（MVP 可简化为直接退出）
- JSON 解析失败：原样透传给 Server（避免误拦截合法但格式特殊的请求），但记录 warn 日志

### 4.2 Detector（检测引擎）

**职责**：定义检测器接口，管理检测流水线。

```go
type Detector interface {
    Detect(ctx context.Context, req *models.InterceptedRequest) (*models.DetectResult, error)
    Name() string
}

type Pipeline struct {
    detectors []Detector
}

func (p *Pipeline) Run(ctx context.Context, req *models.InterceptedRequest) (*models.DetectResult, error)
```

**执行逻辑**：
1. 顺序调用所有 Detector
2. 任一返回 `ActionBlock` → 立即返回 Block（短路）
3. 无 Block，但有 `ActionWarn` → 返回 Warn
4. 全部 `ActionAllow` → 返回 Allow
5. 如果某个 Detector 报错 → 记录错误，继续执行下一个（fail-open，避免单点故障导致服务不可用）

**MVP 检测器**：

| 检测器 | 类型 | 说明 |
|--------|------|------|
| RuleDetector | 本地规则 | keyword / tool_name / regex 匹配 |

### 4.3 Rules（规则引擎）

**职责**：基于规则对工具调用做快速匹配。

**规则类型**：

```go
type RuleType string

const (
    RuleTypeKeyword  RuleType = "keyword"   // 参数值中包含指定关键词
    RuleTypeToolName RuleType = "tool_name" // 工具名匹配
    RuleTypeRegex    RuleType = "regex"     // 参数值正则匹配
)
```

**匹配逻辑**：

```go
func (d *RuleDetector) Detect(ctx context.Context, req *models.InterceptedRequest) (*models.DetectResult, error) {
    if req.Method != models.MethodToolsCall || req.ToolCall == nil {
        return allow, nil
    }

    for _, rule := range d.rules {
        if !rule.Enabled {
            continue
        }
        if match(rule, req.ToolCall) {
            return &models.DetectResult{
                Action:   rule.Action,
                Reason:   fmt.Sprintf("规则匹配: %s (%s)", rule.Name, rule.Pattern),
                Detector: d.Name(),
            }, nil
        }
    }
    return allow, nil
}
```

**匹配函数**：

| 规则类型 | 匹配对象 | 匹配方式 |
|---------|---------|---------|
| `keyword` | `ToolCall.Arguments` 的所有字符串值 | `strings.Contains` |
| `tool_name` | `ToolCall.Name` | 精确匹配或 `strings.EqualFold` |
| `regex` | `ToolCall.Arguments` 的所有字符串值 | `regexp.MatchString` |

**MVP 内置规则（10条）**：

| ID | 名称 | 类型 | 模式 | 动作 |
|----|------|------|------|------|
| builtin-001 | 禁止递归删除 | keyword | `rm -rf` | block |
| builtin-002 | 禁止强制删除根目录 | keyword | `rm -rf /` | block |
| builtin-003 | 禁止格式化磁盘 | keyword | `mkfs` | block |
| builtin-004 | 禁止覆盖磁盘 | keyword | `dd if=/dev/zero` | block |
| builtin-005 | 禁止访问 SSH 密钥 | keyword | `~/.ssh` | block |
| builtin-006 | 禁止访问密码文件 | keyword | `/etc/passwd` | block |
| builtin-007 | 禁止 sudo 提权 | keyword | `sudo` | block |
| builtin-008 | 禁止 su 切换用户 | keyword | `su -` | block |
| builtin-009 | 禁止执行命令工具 | tool_name | `execute_command` | warn |
| builtin-010 | 禁止写文件工具 | tool_name | `write_file` | warn |

### 4.4 Store（存储层）

**职责**：审计日志的持久化。

**MVP 表结构**：

```sql
CREATE TABLE IF NOT EXISTS logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    direction TEXT NOT NULL CHECK(direction IN ('request', 'response')),
    method TEXT NOT NULL,
    tool_name TEXT,
    action TEXT NOT NULL CHECK(action IN ('allow', 'block', 'warn')),
    reason TEXT,
    request TEXT NOT NULL,
    response TEXT,
    client_id TEXT,
    detector TEXT,
    rule_id TEXT
);

CREATE INDEX idx_logs_timestamp ON logs(timestamp);
CREATE INDEX idx_logs_action ON logs(action);
CREATE INDEX idx_logs_method ON logs(method);
CREATE INDEX idx_logs_tool_name ON logs(tool_name);
```

**接口**：

```go
type Store interface {
    Init(ctx context.Context) error
    Close() error
    InsertLog(ctx context.Context, entry *models.LogEntry) error
    QueryLogs(ctx context.Context, filter LogFilter) ([]*models.LogEntry, error)
}
```

**实现**：`SQLiteStore`，基于 `modernc.org/sqlite`（纯 Go，无 CGO）。

### 4.5 API（HTTP 服务）

**职责**：提供审计日志查询和健康检查。

**MVP 路由**：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/api/logs` | 查询日志列表 |

**日志查询参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `start_time` | ISO8601 | 起始时间 |
| `end_time` | ISO8601 | 结束时间 |
| `action` | string | `allow` / `block` / `warn` |
| `method` | string | MCP 方法名 |
| `tool_name` | string | 工具名 |
| `limit` | int | 分页大小（默认 100，最大 1000） |
| `offset` | int | 分页偏移 |

**响应示例**：

```json
[
  {
    "id": 1,
    "timestamp": "2026-05-15T10:00:00Z",
    "direction": "request",
    "method": "tools/call",
    "tool_name": "execute_command",
    "action": "block",
    "reason": "规则匹配: 禁止递归删除 (rm -rf)",
    "request": "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"execute_command\",\"arguments\":{\"command\":\"rm -rf /\"}}}",
    "client_id": "",
    "detector": "rule",
    "rule_id": "builtin-001"
  }
]
```

### 4.6 Config（配置）

**MVP 配置项**：

```go
type AppConfig struct {
    LogLevel string      // debug / info / warn / error（默认 info）
    DBPath   string      // SQLite 路径（默认 mcpguard.db）
    Proxy    ProxyConfig // 代理配置
    API      APIConfig   // HTTP API 配置
}

type ProxyConfig struct {
    Command string   // MCP Server 启动命令
    Args    []string // 命令参数
    Env     []string // 环境变量
}

type APIConfig struct {
    Enabled bool   // 是否启用 API（默认 true）
    Listen  string // 监听地址（默认 :9090）
}
```

**配置来源优先级**（从高到低）：
1. 命令行参数（如 `--command`）
2. 环境变量
3. 配置文件（JSON，可选）
4. 硬编码默认值

---

## 5. 数据流详细设计

### 5.1 正常请求（Allow）

```
1. Agent 发送 JSON-RPC Request（stdin）
   {"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"/tmp/test.txt"}}}

2. MCProxy 读取一行 JSON

3. 解析为 jsonrpc.Request

4. 如果是 tools/call，提取 ToolCallInfo
   ToolCallInfo{Name: "read_file", Arguments: {"path": "/tmp/test.txt"}}

5. 构造 InterceptedRequest
   InterceptedRequest{Method: "tools/call", ToolCall: {...}}

6. Pipeline.Run(ctx, req)
   → RuleDetector 遍历规则
   → 无匹配 → ActionAllow

7. 向 MCP Server 子进程 stdin 写入原始 JSON

8. MCP Server 处理，向 stdout 写入 Response

9. MCProxy 读取 Response

10. 直接写入 Agent stdout（透传）

11. 写审计日志（direction=request, action=allow）
```

### 5.2 拦截请求（Block）

```
1-6 同上

7. Pipeline.Run 返回 ActionBlock
   DetectResult{Action: "block", Reason: "规则匹配: 禁止递归删除 (rm -rf)", Detector: "rule"}

8. 不向 MCP Server 转发

9. 构造 JSON-RPC Error Response
   {"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"请求被安全策略拦截: 规则匹配: 禁止递归删除 (rm -rf)"}}

10. 写入 Agent stdout

11. 写审计日志（direction=request, action=block, reason=...）
```

### 5.3 告警请求（Warn）

```
1-6 同上

7. Pipeline.Run 返回 ActionWarn

8. 向 MCP Server 转发请求（同 Allow）

9. MCP Server 返回 Response

10. 透传给 Agent

11. 写审计日志（direction=request, action=warn, reason=...）
12. 同时向日志输出一条 warn 级别的 slog
```

---

## 6. 错误处理策略

| 场景 | 策略 | 理由 |
|------|------|------|
| JSON 解析失败 | 原样透传 + 记录 warn | 避免误拦截非标准 JSON-RPC 消息 |
| Detector 报错 | 跳过该 Detector，继续执行 | fail-open，避免单点故障阻断服务 |
| Pipeline 全部报错 | 返回 Allow | 最坏情况下不阻断合法请求 |
| MCP Server 子进程退出 | 记录 error，向 Agent 返回 error，进程退出 | MVP 不做自动重启 |
| SQLite 写入失败 | 记录 error slog，不阻断请求 | 存储不可用时不影响代理功能 |
| API 服务 panic | 单独 goroutine，不影响代理 | 代理是核心，API 是辅助 |

---

## 7. CLI 设计

```
mcpguard serve [flags]

Flags:
  --command string     MCP Server 启动命令（必填）
  --args strings       MCP Server 命令参数
  --env strings        环境变量（KEY=VALUE）
  --db string          SQLite 数据库路径（默认 mcpguard.db）
  --api-addr string    API 监听地址（默认 :9090）
  --no-api             禁用 HTTP API
  --log-level string   日志级别（默认 info）

Examples:
  mcpguard serve --command="npx" --args="-y,@anthropic/mcp-server-filesystem"
  mcpguard serve --command="python" --args="-m,mcp_server_filesystem" --db=/var/lib/mcpguard.db
```

---

## 8. 部署方式

### 8.1 独立运行

```bash
# Agent 配置中指向 mcpguard，而非直接指向 MCP Server
# 例如 Claude Desktop config:
{
  "mcpServers": {
    "filesystem": {
      "command": "mcpguard",
      "args": ["serve", "--command=python", "--args=-m,mcp_server_filesystem"]
    }
  }
}
```

### 8.2 容器化（后续）

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o mcpguard ./cmd/mcpguard

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/mcpguard .
CMD ["./mcpguard"]
```

---

## 9. 安全考量

### 9.1 不可绕过

- Agent 无法直接访问 MCP Server 子进程
- MCPGuard 是 MCP Server 的父进程，子进程的标准 I/O 完全由 MCPGuard 控制
- 即使 Agent 被 prompt injection 控制，也无法跳过 MCPGuard

### 9.2 Fail-Open 策略

- 检测引擎报错时不阻断请求
- 理由：MCPGuard 是安全增强层，不应成为单点故障
- 后续可配置为 `strict` 模式（fail-closed）

### 9.3 日志安全

- 审计日志包含完整请求 JSON，可能含敏感信息
- MVP 阶段日志存储在本地 SQLite，由操作系统权限保护
- 后续可加入敏感字段脱敏（参考 Pipelock 的请求脱敏设计）

### 9.4 资源限制

- MCP Server 子进程可通过 `exec.Cmd` 设置 `SysProcAttr` 限制资源
- MVP 不实现，但架构预留（后续可加入 CPU/内存限制）

---

## 10. 测试策略

### 10.1 单元测试

| 模块 | 测试重点 |
|------|---------|
| `jsonrpc` | Request/Response 序列化与反序列化 |
| `rules` | 规则匹配逻辑（keyword/tool_name/regex） |
| `detector` | Pipeline 执行顺序与结果聚合 |
| `store` | SQLite CRUD（使用内存数据库 `:memory:`） |
| `proxy` | JSON-RPC 解析与转发逻辑（模拟 stdin/stdout） |

### 10.2 集成测试

- 启动一个模拟 MCP Server（echo 服务）
- MCPGuard 代理该 Server
- 发送 tools/call 请求，验证拦截/放行行为
- 验证审计日志是否正确写入

### 10.3 端到端测试

- 使用真实 MCP Server（如 filesystem server）
- 通过 stdio 发送请求，验证完整数据流

---

## 11. 后续迭代路线图

| 版本 | 功能 |
|------|------|
| v0.1 (MVP) | stdio 代理、规则检测、SQLite 审计、HTTP API |
| v0.2 | 配置文件加载、规则文件加载、响应检测 |
| v0.3 | LLM 检测（OpenAI/Ollama）、策略 CRUD API |
| v0.4 | SSE 传输模式、Web 管理界面 |
| v0.5 | 密码学签名审计、MITRE ATT&CK 映射、SIEM 集成 |

---

## 12. 参考

- [MCPGuard 竞品调研报告](./research-report.md)
- [MCPGuard 接口文档](./interfaces.md)
- [Model Context Protocol Specification](https://modelcontextprotocol.io/specification)
