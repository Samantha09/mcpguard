# MCPGuard 平台 + 探针架构设计

> 日期：2026-05-20
> 状态：已批准

---

## 1. 设计目标

将 MCPGuard 从单体代理重构为**平台 + 探针**架构：

- **平台（Platform）**：集中管理规则、聚合日志、提供全局安全视图
- **探针（Probe）**：部署在每台机器上，本地完成 MCP 代理、检测和拦截

核心原则：**检测在本地（低延迟、离线可用），管控在平台（集中规则、全局视图）**。

## 2. 架构概览

```
┌──────────────────────────────────────────────────┐
│             mcpguard (单二进制)                    │
│                                                    │
│  mcpguard platform          mcpguard serve          │
│  ┌────────────────────┐    ┌──────────────────┐   │
│  │ Platform Server    │    │ Probe (探针)      │   │
│  │                    │    │                  │   │
│  │ ┌──────────────┐  │    │ ┌──────────────┐ │   │
│  │ │ REST API     │  │    │ │ MCP Proxy    │ │   │
│  │ │ - 探针注册   │◄─┼────┼─┤ │ (现有逻辑)  │ │   │
│  │ │ - 规则 CRUD  │  │    │ └──────────────┘ │   │
│  │ │ - 日志查询   │  │    │ ┌──────────────┐ │   │
│  │ │ - 安全报告   │  │    │ │ Platform     │ │   │
│  │ └──────────────┘  │    │ │ Client       │ │   │
│  │ ┌──────────────┐  │    │ │ - 注册       │ │   │
│  │ │ WebSocket    │  │    │ │ - 拉取规则   │ │   │
│  │ │ - 日志接收   │◄─┼────┼─┤ │ - 上报日志   │ │   │
│  │ │ - 即时指令   │  │    │ │ - 接收指令   │ │   │
│  │ └──────────────┘  │    │ └──────────────┘ │   │
│  │ ┌──────────────┐  │    │ ┌──────────────┐ │   │
│  │ │ SQLite       │  │    │ │ SQLite       │ │   │
│  │ │ (聚合存储)   │  │    │ │ (本地存储)   │ │   │
│  │ └──────────────┘  │    │ └──────────────┘ │   │
│  └────────────────────┘    └──────────────────┘   │
└──────────────────────────────────────────────────┘
```

## 3. 关键设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 通信方式 | REST + WebSocket | 规则拉取用 REST（简单可靠），日志告警用 WebSocket（实时推送） |
| 认证方式 | Token（Bearer） | 简单实用，适合内部部署 |
| 部署形态 | 单二进制 + 子命令 | 分发简单，`mcpguard platform` / `mcpguard serve` |
| 数据库 | MVP 用 SQLite，后续可切 PostgreSQL | 无外部依赖，渐进式 |
| MVP 范围 | 纯 API | 不含 Web UI，聚焦核心链路 |

## 4. 通信协议设计

### 4.1 REST API（探针 → 平台）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/probes/register` | 探针注册，返回 token |
| GET | `/api/v1/rules` | 拉取最新规则列表 |
| GET | `/api/v1/rules/:id` | 获取单条规则 |
| POST | `/api/v1/logs/batch` | 批量上报日志 |
| GET | `/api/v1/policies` | 拉取策略列表 |

平台管理 API（供管理员使用）：

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/probes` | 列出所有在线探针 |
| GET | `/api/v1/probes/:id` | 查看探针详情 |
| POST | `/api/v1/rules` | 创建规则 |
| PUT | `/api/v1/rules/:id` | 更新规则 |
| DELETE | `/api/v1/rules/:id` | 删除规则 |
| GET | `/api/v1/logs` | 查询全局日志 |
| GET | `/api/v1/reports/summary` | 安全报告 |
| POST | `/api/v1/probes/:id/command` | 向探针下发即时指令 |

### 4.2 WebSocket 通道

探针通过 WebSocket 连接到 `ws://platform:8080/api/v1/ws`。

**探针 → 平台消息**：

```json
{"type": "log", "data": {<LogEntry>}}
{"type": "alert", "data": {<DetectResult>}}
{"type": "heartbeat", "data": {"status": "ok"}}
```

**平台 → 探针消息**：

```json
{"type": "rules_updated", "data": {"version": 42}}
{"type": "reload_rules", "data": {}}
{"type": "ping", "data": {}}
```

## 5. 数据模型扩展

### 5.1 探针注册信息

```sql
CREATE TABLE IF NOT EXISTS probes (
    id TEXT PRIMARY KEY,           -- UUID
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL,       -- bcrypt hash
    hostname TEXT,
    ip TEXT,
    status TEXT DEFAULT 'offline', -- online / offline
    last_heartbeat DATETIME,
    registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    metadata TEXT                   -- JSON，可扩展
);
```

### 5.2 规则表（新增，替代内置硬编码）

```sql
CREATE TABLE IF NOT EXISTS rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('keyword', 'tool_name', 'regex', 'file_path')),
    pattern TEXT NOT NULL,
    action TEXT NOT NULL CHECK(action IN ('allow', 'block', 'warn')),
    enabled BOOLEAN DEFAULT TRUE,
    description TEXT,
    version INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 5.3 平台日志表（带探针来源）

在现有 logs 表增加 `probe_id TEXT` 字段。

## 6. 模块改造

### 6.1 新增模块

| 模块 | 职责 |
|------|------|
| `internal/platform` | 平台服务：REST API + WebSocket + 探针管理 + 规则存储 |
| `internal/probe/client` | 探针客户端：连接平台、注册、拉取规则、上报日志 |

### 6.2 现有模块改造

| 模块 | 改动 |
|------|------|
| `cmd/mcpguard` | 新增 `platform` 子命令 |
| `internal/proxy` | 检测结果同时写入本地日志和推送到平台客户端 |
| `internal/rules` | 支持从平台拉取的规则覆盖内置规则 |
| `internal/store` | 新增 probes 表、rules 表；日志表加 probe_id |
| `internal/api` | 拆分为平台 API（管理端）和探针 API（上报端） |
| `internal/config` | 新增平台地址、token 等配置项 |

### 6.3 现有模块保持不变

| 模块 | 说明 |
|------|------|
| `internal/detector` | Pipeline 逻辑不变 |
| `internal/llm` | 接口不变，后续扩展 |
| `internal/logger` | 不变 |
| `pkg/jsonrpc` | 不变 |

## 7. 数据流

### 7.1 探针启动流程

```
1. 解析配置（--platform-addr, --token 或 --register）
2. 连接平台 → POST /api/v1/probes/register（首次）或验证 token
3. 拉取规则 → GET /api/v1/rules
4. 合并规则：平台规则覆盖同名内置规则
5. 创建 Pipeline（RuleDetector 使用合并后的规则）
6. 启动 MCP Proxy（同现有逻辑）
7. 建立 WebSocket 连接，开始心跳
8. 后台 goroutine：定时拉取规则刷新（默认 60s）
```

### 7.2 请求检测流程（同现有，增加上报）

```
1. Agent 发送 tools/call
2. 探针本地检测 → Allow/Block/Warn
3. 本地写入审计日志（SQLite）
4. 异步通过 WebSocket 推送日志到平台
5. 如果 WebSocket 断开，缓存到本地队列，重连后补发
```

### 7.3 规则更新流程

```
1. 管理员通过平台 API 创建/修改规则
2. 平台通过 WebSocket 通知所有探针 rules_updated
3. 探针收到通知后立即拉取最新规则
4. 热更新本地 RuleDetector（原子替换规则切片）
```

## 8. 离线容错

- 探针断网时仍能正常代理和检测（使用本地缓存的规则）
- 日志先写入本地 SQLite，WebSocket 重连后自动补发
- 规则刷新失败时继续使用上次成功的规则版本
- 心跳超时后平台标记探针为 offline，但不影响探针本地运行

## 9. CLI 命令

```bash
# 平台模式
mcpguard platform [flags]
  --db string         数据库路径（默认 mcpguard.db）
  --listen string     监听地址（默认 :8080）
  --log-level string  日志级别（默认 info）

# 探针模式（增强现有 serve）
mcpguard serve [flags]
  --command string      MCP Server 启动命令（必填）
  --args string         命令参数
  --platform-addr string  平台地址（如 http://platform:8080）
  --token string        探针认证 token
  --register            首次注册模式（获取 token）
  --probe-name string   探针名称（默认 hostname）
  --db string           本地数据库路径（默认 mcpguard.db）
  --log-level string    日志级别（默认 info）
```

## 10. 安全考量

- Token 使用 bcrypt hash 存储，不明文保存
- WebSocket 连接需要先通过 REST 获取有效 token
- 平台 API 管理端需要认证（后续可扩展 RBAC）
- 探针与平台间通信建议在生产环境使用 TLS

## 11. 后续迭代

| 版本 | 功能 |
|------|------|
| v0.2 (本次) | 平台 + 探针核心链路、规则管理 API、日志聚合 API |
| v0.3 | Web 管理界面 |
| v0.4 | LLM 检测 |
| v0.5 | SSE 传输、策略热重载 |
| v0.6 | PostgreSQL 支持、RBAC、TLS |
