# MCPGuard 竞品调研报告

> 调研日期：2026-05-15
> 调研范围：开源 AI Agent 安全代理/防火墙/护栏产品
> 调研目的：为 MCPGuard MVP 架构设计提供参考

---

## 执行摘要

2025-2026 年，AI Agent 安全领域出现了多个开源项目，核心架构趋同：**在 Agent 与外部资源之间插入一个不可绕过的代理层，拥有传输协议的所有权**。本次调研选取三个代表性产品：

| 产品 | 语言 | 定位 | 核心差异 |
|------|------|------|---------|
| **Pipelock** | Go | 企业级 Agent 防火墙 | 能力隔离架构、DLP 深度检测、密码学审计 |
| **Agentgateway** | Rust/Go | Agent 通信网关 | MCP/A2A 多协议、CEL 策略引擎、LLM 路由 |
| **ClawShield** | Go | 纵深防御安全代理 | 三层架构（应用/网络/内核）、YAML 策略、热重载 |

共同启示：**MCPGuard 的 stdio 代理模式是正确的方向，但 MVP 需要在"传输层所有权"和"检测引擎"之间找到最小闭环。**

---

## 一、Pipelock

### 1.1 产品定位

Pipelock 是一个面向生产环境的 AI Agent 防火墙，采用**能力隔离（capability separation）**架构——Agent 进程持有密钥但无直接网络访问，Pipelock 有网络访问但不持有 Agent 密钥。

### 1.2 架构设计

```
┌──────────────────────┐         ┌───────────────────────┐
│  PRIVILEGED ZONE     │         │  FIREWALL ZONE        │
│  AI Agent            │  IPC    │  Pipelock             │
│  - Has API keys      │────────>│  - No agent secrets   │
│  - Restricted network│ fetch / │  - Full internet      │
│                      │ CONNECT │  - Returns text       │
│                      │ /ws/MCP │  - URL scanning       │
│                      │<────────│  - Audit logging      │
└──────────────────────┘         └───────────────────────┘
```

**三种代理模式（同端口复用）：**
- **Fetch proxy** (`/fetch?url=...`)：URL 获取、文本提取、注入扫描
- **Forward proxy** (`HTTPS_PROXY`)：标准 HTTP CONNECT 隧道
- **WebSocket proxy** (`/ws?url=ws://...`)：双向帧扫描
- **MCP proxy**：包装 stdio 或 HTTP MCP 服务器
- **A2A proxy**：检查 Google Agent-to-Agent 协议流量

### 1.3 检测引擎

**11 层 URL 扫描管道：**
1. 方案验证
2. CRLF 注入检测
3. 路径遍历阻断
4. 域名黑名单
5. DLP 模式匹配（48 种内置模式）
6. 路径熵分析
7. 子域熵分析
8. SSRF 防护 + DNS 重绑定防护
9. 按域速率限制
10. URL 长度限制
11. 按域数据预算

**响应扫描（提示注入）：**
- 6 遍归一化：NFKC + 不可见字符 + leetspeak + 元音 + 隐写剥离
- 25 种内置模式：越狱短语、指令操纵、凭证索取、内存持久化

**MCP 专用检测：**
- 17 条内置工具策略，阻止危险工具调用
- 10 种工具调用链检测模式
- 双向扫描（请求 + 响应）

### 1.4 策略配置

三种 CLI 预设模式：

| 模式 | 安全级别 | 网络浏览 | 适用场景 |
|-----|---------|---------|---------|
| `strict` | 仅允许列表 | 无 | 受监管行业、高安全 |
| `balanced` | 阻止初级 + 检测高级 | 通过代理 | 大多数开发者（默认） |
| `audit` | 仅日志 | 无限制 | 执行前评估 |

**配置热重载**：文件监视器或 SIGHUP 触发，无需重启。

### 1.5 审计日志

- **飞行记录器**：哈希链 JSONL 证据日志，Ed25519 签名检查点
- **行动收据**：密码学签名的每项调解行动证明
- **事件发射**：转发到 SIEM、webhook、syslog，fire-and-forget 不阻塞代理
- **MITRE ATT&CK 映射**：每项事件包含技术 ID（如 T1048 外泄）

### 1.6 技术栈

| 组件 | 技术 |
|-----|------|
| 核心语言 | Go 97.4% |
| 构建要求 | Go 1.25+ |
| CLI 框架 | Cobra（20+ 命令） |
| 日志 | zerolog（结构化 JSON） |
| 签名算法 | Ed25519 |
| 测试覆盖 | 88%+ 语句覆盖 |

---

## 二、Agentgateway

### 2.1 产品定位

Agentgateway 是一个"Agentic AI 连接解决方案"，定位为开源代理网关，支持 Agent-to-LLM、Agent-to-Tool、Agent-to-Agent 的通信安全。

### 2.2 架构设计

**核心能力：**
- **MCP Gateway**：stdio/HTTP/SSE/Streamable HTTP 多传输支持
- **Tool Federation**：将多个 MCP 服务器的工具聚合为统一端点
- **LLM Routing**：OpenAI 兼容 API，支持负载均衡和故障转移
- **OAuth 认证**：MCP 工具访问的授权层

### 2.3 策略与治理

**Guardrails 多层过滤：**
- 正则表达式过滤
- OpenAI Moderation API
- AWS Bedrock Guardrails
- Google Model Armor
- 自定义 Webhook

**RBAC 与策略引擎：**
- 基于 CEL（Common Expression Language）的细粒度策略
- 支持 JWT、API Key、OAuth 认证
- 速率限制、TLS 终止

### 2.4 传输层设计

Agentgateway 的传输层抽象对 MCPGuard 有直接参考价值：

| 传输协议 | 支持状态 | 用途 |
|---------|---------|------|
| stdio | 支持 | 本地子进程包装 |
| HTTP | 支持 | 远程 MCP 服务器反向代理 |
| SSE | 支持 | 流式响应 |
| Streamable HTTP | 支持 | MCP 最新标准 |

**关键设计**：将传输协议与检测逻辑解耦，检测引擎只处理标准化的 `Request` / `Response` 对象。

### 2.5 技术栈

| 组件 | 技术 |
|-----|------|
| 核心语言 | Rust 59.1%, Go 28.5%, TypeScript 9.9% |
| 部署 | Kubernetes controller / Gateway API |
| 可观测性 | OpenTelemetry |

---

## 三、ClawShield

### 3.1 产品定位

ClawShield 采用**纵深防御（defense-in-depth）**理念，构建三层安全架构，每层可以独立工作也可以协同响应。

### 3.2 架构设计

```
Layer 1: Application (Go Proxy)
    ↓
Layer 2: Network (iptables)
    ↓
Layer 3: Kernel (eBPF)
```

**跨层事件总线**：通过 Unix socket (`/tmp/clawshield-events.sock`) 实现自适应响应。例如：eBPF 检测到特权提升 → Proxy 提升注入检测敏感度。

### 3.3 Go Proxy（Layer 1）

- **HTTP 反向代理**：拦截用户与 AI 网关之间的所有流量
- **流式响应分析**：chunk-by-chunk 扫描 SSE 和 NDJSON 流，无缓冲延迟
- **YAML 策略引擎**：deny-by-default，热重载（修改后 5 秒内生效）
- **影子/金丝雀模式**：策略变更可先观察再生效

### 3.4 检测能力

| 检测类型 | 说明 |
|---------|------|
| Prompt Injection | 越狱尝试、指令覆盖、角色操纵 |
| PII | 邮箱、电话、SSN、信用卡检测与脱敏 |
| Secrets | API Key、Token、密码外泄拦截 |
| 漏洞扫描 | SQL 注入、SSRF、路径遍历、命令注入、XSS |
| 恶意软件 | 可执行文件、脚本、压缩炸弹 |

### 3.5 审计日志

- **SQLite 存储**：结构化取证日志，记录每次 deny/redact 决策的完整上下文
- **OCSF v1.1 格式**：标准化安全事件格式
- **SIEM 集成**：Syslog (RFC 5424 over TCP/TLS) + Webhook

### 3.6 网络与内核层

- **iptables（Layer 2）**：出口防火墙，限制 Agent 可访问的域名/IP，支持从事件总线动态添加临时规则
- **eBPF（Layer 3）**：内核级系统调用监控，检测 fork 炸弹、敏感文件访问、特权提升、异常网络连接

---

## 四、横向对比

### 4.1 架构模式对比

| 维度 | Pipelock | Agentgateway | ClawShield |
|------|---------|-------------|------------|
| **代理位置** | 进程间 IPC | 网络网关 | 应用反向代理 + 网络 + 内核 |
| **传输所有权** | 完全拥有 | 完全拥有 | 完全拥有 |
| **传输协议** | stdio/HTTP/WebSocket | stdio/HTTP/SSE/Streamable | HTTP（MCP 通过 HTTP） |
| **绕过难度** | 高（网络隔离） | 高（独立进程） | 极高（三层防御） |
| **部署复杂度** | 中 | 高（K8s 原生） | 高（需要 root/eBPF） |

### 4.2 检测引擎对比

| 维度 | Pipelock | Agentgateway | ClawShield |
|------|---------|-------------|------------|
| **规则引擎** | 内置模式 + DLP | CEL + 正则 | YAML deny-by-default |
| **LLM 检测** | 无 | 支持（ moderation / guardrails） | 无 |
| **MCP 专用** | 是（17 条工具规则 + 链检测） | 是（工具联邦 + 策略） | 否（通用 HTTP 代理） |
| **检测阶段** | 请求 + 响应 | 请求 + 响应 | 请求 + 响应 |
| **热重载** | 是 | 未明确 | 是（5 秒内） |

### 4.3 审计与治理对比

| 维度 | Pipelock | Agentgateway | ClawShield |
|------|---------|-------------|------------|
| **存储格式** | JSONL + 哈希链 | 未明确 | SQLite + OCSF v1.1 |
| **签名** | Ed25519 | 未明确 | 未明确 |
| **外部集成** | SIEM / webhook / syslog | OpenTelemetry | Syslog / webhook |
| **MITRE 映射** | 是 | 否 | 否 |

---

## 五、对 MCPGuard 的设计启示

### 5.1 架构层面

**1. 传输层所有权是最核心的安全边界**

三个产品都选择拥有传输协议（stdio/HTTP/SSE），而非在 Agent 内部做 SDK 包装。这与 [Akto MCP Proxy](https://www.akto.io/blog/what-is-mcp-proxy) 的观点一致："Every request must pass through it, and no one can bypass it."

> **启示**：MCPGuard 的 stdio 代理方向正确，必须确保 Agent 无法绕过代理直接连接 MCP Server。

**2. 传输协议与检测逻辑解耦**

Agentgateway 将 stdio/HTTP/SSE 统一抽象为 `Request`/`Response`，检测引擎只处理标准化对象。

> **启示**：MCPGuard 当前的 `models.InterceptedRequest` / `models.InterceptedResponse` 已具备此抽象，应继续保持——未来扩展 SSE 传输时，检测引擎零改动。

**3. 策略配置需要预设模式**

Pipelock 的 `strict` / `balanced` / `audit` 三种预设极大降低了用户配置成本。

> **启示**：MCPGuard MVP 可内置一套 `balanced` 默认规则（如禁止 `rm -rf`、禁止 `~/.ssh` 访问），用户零配置即可运行。

### 5.2 检测引擎层面

**4. 先做规则检测，LLM 检测是锦上添花**

三个产品中，只有 Agentgateway 依赖外部 LLM API 做 moderation，Pipelock 和 ClawShield 都是纯本地规则/模式匹配。

> **启示**：MCPGuard MVP 应该先实现 `keyword` + `tool_name` + `regex` 三种规则类型，LLM 检测放到 v0.2。

**5. 规则类型应覆盖常见攻击面**

Pipelock 的 17 条 MCP 工具策略和 10 种调用链检测提供了很好的参考：

| 规则类型 | 示例 | MCPGuard 优先级 |
|---------|------|----------------|
| 危险命令 | `rm -rf`, `mkfs`, `dd if=/dev/zero` | P0 |
| 敏感路径 | `~/.ssh`, `/etc/passwd`, `/var/log` | P0 |
| 网络外联 | `curl`, `wget`, `nc` | P1 |
| 权限提升 | `sudo`, `su`, `chmod 777` | P1 |
| 数据外泄 | 包含 `password`, `secret`, `token` 的参数 | P2 |
| 调用链 | 连续调用 `read_file` + `send_email` | P2（后续） |

**6. 响应检测可以延后**

Pipelock 和 ClawShield 都支持响应检测，但这是为了防御 prompt injection 和数据外泄。MVP 阶段 Agent 向 Server 发起的请求检测已能覆盖 80% 风险。

> **启示**：MCPGuard MVP 只做请求检测（`interceptRequest`），响应检测（`interceptResponse`）标记为 TODO。

### 5.3 工程实践层面

**7. 审计日志需要结构化且不可抵赖**

Pipelock 的 Ed25519 签名和 ClawShield 的 SQLite 取证日志表明：安全产品的日志本身就是证据。

> **启示**：MCPGuard 的 `LogEntry` 应包含完整请求/响应 JSON、检测器名称、规则 ID、时间戳，并考虑未来加入签名字段。

**8. 配置热重载是用户体验的关键**

Pipelock（SIGHUP/文件监视）和 ClawShield（5 秒热重载）都支持不重启更新策略。

> **启示**：MCPGuard MVP 可以先不支持热重载，但架构上预留——规则从 `[]Rule` 改为从 `*RuleManager` 读取，后续容易扩展。

**9. CLI 预设模式降低上手门槛**

Pipelock 的 `pipelock --mode=balanced` 一行启动。

> **启示**：MCPGuard 可以设计 `mcpguard serve --profile=strict`，自动加载对应规则集。

### 5.4 技术选型层面

**10. Go 是 MCP 安全代理的合理选择**

Pipelock（Go 97.4%）和 ClawShield（Go）都选择 Go，原因是：
- stdio/子进程管理天然友好（`os/exec`）
- 单二进制、无依赖部署
- 并发模型适合 I/O 密集型代理

Agentgateway 用 Rust 是为了极致性能和安全内存，但开发成本更高。

> **启示**：MCPGuard 继续用 Go 是正确的，不需要因为性能焦虑而换语言。

**11. SQLite 足够支撑 MVP 审计**

ClawShield 用 SQLite 做取证日志，Pipelock 用 JSONL + 哈希链。

> **启示**：MCPGuard 的 SQLite 选型正确，但 MVP 阶段不需要 OCSF 格式或签名，先保证能写入和查询即可。

---

## 六、MCPGuard MVP 范围修正建议

基于以上调研，MCPGuard MVP 应该**进一步收缩**到真正最小闭环：

### 必须做的（核心闭环）

1. **stdio 代理**：Agent → MCProxy → MCP Server 子进程，stdin/stdout 双向转发
2. **规则检测引擎**：`keyword` + `tool_name` 两种规则类型，内置 10 条默认规则
3. **Pipeline 执行**：顺序调用，Block > Warn > Allow 聚合
4. **SQLite 审计**：`LogEntry` 写入 + `GET /api/logs` 查询
5. **HTTP API**：`/health` + `/api/logs`（分页过滤）
6. **CLI 启动**：`mcpguard serve --command="npx -y @server"`

### 明确不做（等后续迭代）

| 功能 | 竞品实现 | 不做理由 |
|------|---------|---------|
| SSE 传输 | Agentgateway | stdio 覆盖 80% 场景 |
| LLM 检测 | Agentgateway | 规则检测先跑起来 |
| 策略 CRUD API | 全部 | 规则硬编码 + 文件加载 |
| 响应检测 | Pipelock/ClawShield | 请求检测先闭环 |
| 配置热重载 | Pipelock/ClawShield | 架构预留接口 |
| 密码学签名 | Pipelock | SQLite 明文先可用 |
| 前端界面 | 无 | curl 查询足够 |
| eBPF/iptables | ClawShield | 需要 root，MVP 不做 |

---

## 参考来源

- [Pipelock GitHub](https://github.com/luckypipewrench/pipelock)
- [Agentgateway GitHub](https://github.com/agentgateway/agentgateway)
- [ClawShield GitHub](https://github.com/SleuthCo/clawshield-public)
- [Akto: What is MCP Proxy](https://www.akto.io/blog/what-is-mcp-proxy)
- [SOVR MCP Proxy (npm)](https://www.npmjs.com/package/sovr-mcp-proxy)
- [ShieldNet: Network-Level Guardrails (arXiv)](https://arxiv.org/abs/2604.04426)
- [The MCP Security Survival Guide](https://towardsdatascience.com/the-mcp-security-survival-guide-best-practices-pitfalls-and-real-world-lessons/)
- [Best MCP Security Tools 2026](https://mcpmanager.ai/blog/mcp-security-tools/)
- [AEGIS: Open-source AI agent firewall](https://dev.to/justin0504/i-built-an-open-source-firewall-for-ai-agents-it-blocks-dangerous-tool-calls-before-they-4p5f)
