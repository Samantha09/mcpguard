# CLAUDE.md

MCPGuard — Go 1.24+

## 开发工作流

使用 Superpowers Skills 套件，核心规则：

- **编码前必须先 brainstorming**
- **TDD 优先**：先写测试，再写实现
- **证据优先于断言**：声称完成前必须有验证命令输出作为证据
- **收到审查反馈时保持严谨**：不盲目同意，技术上验证每条反馈

| 场景 | Skill |
|------|-------|
| 创造性工作（新功能、修改行为） | `/superpowers:brainstorming` |
| 多步骤实现任务 | `/superpowers:writing-plans` |
| 执行已有计划 | `/superpowers:executing-plans` |
| 功能/Bug 开发 | `/superpowers:test-driven-development` |
| Bug/测试失败/异常 | `/superpowers:systematic-debugging` |
| 声称完成前 | `/superpowers:verification-before-completion` |
| 需要集成 | `/superpowers:finishing-a-development-branch` |
| 请求审查 | `/superpowers:requesting-code-review` |
| 收到审查反馈 | `/superpowers:receiving-code-review` |
| 2+ 独立任务并行 | `/superpowers:dispatching-parallel-agents` |

## 提交规范

Conventional Commits：`<类型>(<范围>): <描述>`

- **类型**：`feat` / `fix` / `hotfix` / `perf` / `build` / `ci` / `chore` / `docs` / `refactor` / `revert` / `style` / `test`
- **范围**：`feat`/`fix` 括号内必须是纯数字（需求/缺陷 ID）；其他类型用英文
- **描述**：至少 5 个字符，使用中文
- **不要**添加 `Co-Authored-By` 行

## 项目规范

- **模块名**：`github.com/Samantha09/mcpguard`
- **构建**：`go build -o mcpguard ./cmd/mcpguard`
- **开发分支**：`dev`，不直接修改 `main`
- **`.claude/` 目录**：禁止提交到仓库
- **注释语言**：中文
- **Commit 语言**：描述用中文，类型/范围用英文

## 参考索引

| 主题 | 位置 |
|------|------|
| 技术栈选型 | memory: `tech-stack.md` |
| 项目结构 | memory: `project-structure.md` |
| 自测流程 | memory: `self-test-checklist.md` |
| 项目简介 | `README.md` |
| 设计文档 | `docs/` |
