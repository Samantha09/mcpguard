# CLAUDE.md

MCPGuard — Go 1.24+

## 关于本文件

`CLAUDE.md` 为 Claude Code 提供项目级指导。

## 开发工作流（Superpowers Skills）

本项目全流程使用 Superpowers skills 套件，所有开发任务必须遵循以下工作流：

| 场景 | Skill |
|------|-------|
| 任何创造性工作（新功能、新组件、修改行为） | `/superpowers:brainstorming` |
| 多步骤实现任务（有规格或需求文档） | `/superpowers:writing-plans` |
| 执行已有实现计划 | `/superpowers:executing-plans` |
| 功能开发或 Bug 修复 | `/superpowers:test-driven-development` |
| 遇到 Bug、测试失败、异常行为 | `/superpowers:systematic-debugging` |
| 即将声称工作完成/通过 | `/superpowers:verification-before-completion` |
| 完成实现，需要集成 | `/superpowers:finishing-a-development-branch` |
| 完成任务后请求审查 | `/superpowers:requesting-code-review` |
| 收到代码审查反馈 | `/superpowers:receiving-code-review` |
| 2+ 个独立任务可并行 | `/superpowers:dispatching-parallel-agents` |

关键规则：
- **编码前必须先 brainstorming**
- **TDD 优先**：先写测试，再写实现
- **证据优先于断言**：声称完成前必须有验证命令的输出作为证据
- **收到审查反馈时保持严谨**：不盲目同意，技术上验证每条反馈

## 提交规范

遵循 Conventional Commits：

```
<类型>(<范围>): <描述>
```

**类型**：`feat` / `fix` / `hotfix` / `perf` / `build` / `ci` / `chore` / `docs` / `refactor` / `revert` / `style` / `test`

**范围（括号内容）**：
- `feat` / `fix`：括号内**必须是纯数字**（需求/缺陷 ID），如 `feat(10565): ...`
- 其他类型：括号内使用英文，如 `docs(doc): ...`、`refactor(core): ...`

**描述**：至少 5 个字符

**分支名规范**：`master` | `dev` | `feature` | `master_xxx` | `dev_xxx` | `feature_xxx` | `maintenance_xxx` | `bugfix`，只能使用英文字母、数字和下划线

**开发分支**：所有开发工作均在 `dev` 分支进行，**修改时不更新 `main` 分支**。`main` 仅用于稳定发布或最终集成。

注意：commit message 中**不要**添加 `Co-Authored-By` 行。

## 项目特定规范

- **技术栈**：Go 1.24+，React，MCP Protocol
- **构建**：`go build -o mcpguard ./cmd/mcpguard`
- **模块名**：`github.com/Samantha09/mcpguard`
- **目录结构**：
  - `cmd/mcpguard/` — 程序入口
  - `internal/proxy/` — MCP Proxy 拦截层（JSON-RPC 请求/响应拦截）
  - `internal/detectors/` — 检测引擎（规则 + LLM 检测）
  - `internal/rules/` — 规则定义（文件路径、命令关键词、正则等）
  - `internal/api/` — HTTP API（配置管理、日志查询、安全报告）
  - `internal/models/` — 数据模型
  - `internal/config/` — 配置管理
  - `pkg/` — 可导出的公共库
  - `web/` — React 前端
  - `docs/` — 文档
- **`.claude/` 目录**：Claude Code 本地配置目录，**必须**加入 `.gitignore`，禁止提交到仓库
- **注释语言**：代码注释使用中文
- **Commit 语言**：Commit message 的描述部分使用中文（类型/范围仍遵循 Conventional Commits 英文规范）

## 开发自测流程（改完必须自测）

**核心原则**：任何代码修改完成后，必须在提交前自行验证，不能依赖用户当测试员。

### 1. 编译检查

```bash
go build ./...
```

### 2. 运行全部测试

```bash
go test ./... -v
```

### 3. 代码风格检查

```bash
# 格式化
go fmt ./...

# 静态检查（如已安装 golangci-lint）
golangci-lint run ./...
```

### 4. 竞态检测

```bash
go test -race ./...
```

### 5. CLI 基础功能验证

```bash
# 构建
go build -o mcpguard ./cmd/mcpguard

# 验证可正常运行
./mcpguard version
./mcpguard serve
```

### 6. 常见问题自诊

| 现象 | 排查方向 |
|------|---------|
| go build 失败 | 检查 Go 版本是否 >= 1.24；运行 `go mod tidy` 整理依赖 |
| 测试失败 | 使用 `-v` 查看详细输出；检查是否有未 mock 的外部依赖 |
| 竞态检测报错 | 检查共享状态的并发访问，必要时加锁或使用 channel |
| import 循环 | Go 禁止循环导入，将共享类型提取到 `internal/models/` 或 `pkg/` |
| 前端构建失败 | 检查 `web/` 目录下 `npm install` 是否完成 |

## 参考文档

| 主题 | 位置 |
|------|------|
| **项目简介与快速开始** | `README.md` |
| **需求与设计文档** | `docs/` 目录 |
| **Go 编码规范** | memory: `go-coding-standards.md` |
| **MCP 协议参考** | memory: `mcp-protocol-reference.md` |
