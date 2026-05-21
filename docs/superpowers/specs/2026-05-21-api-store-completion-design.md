# API / Store 空实现补全设计文档

> 日期：2026-05-21
> 范围：阶段一（P0 + P3）

---

## 目标

补全当前代码库中 API 和 Store 层的空实现，使 Policy / Rule / Report 三个模块的功能闭环，并同步补齐单元测试。

---

## 模块一：Policy

### Store 层

在 `store.go` 的 `Init()` 中新增 `policies` 表：

```sql
CREATE TABLE IF NOT EXISTS policies (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    rule_ids TEXT,          -- JSON 数组序列化存储
    llm_enabled BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

实现以下方法（当前为空）：
- `UpsertPolicy`：INSERT OR REPLACE INTO policies
- `GetPolicy`：SELECT 单条，不存在返回 `sql.ErrNoRows`
- `ListPolicies`：SELECT 全部，按 `created_at DESC`
- `DeletePolicy`：DELETE WHERE id = ?

`RuleIDs` 字段在 Go 层用 `json.Marshal/Unmarshal` 与表中的 JSON 字符串互转。

### API 层

补全以下 handler：
- `GET /api/policies` → `ListPolicies`，返回 `[]Policy`
- `POST /api/policies` → 绑定 `Policy` 请求体 → `UpsertPolicy` → 201 Created
- `GET /api/policies/:id` → `GetPolicy`，不存在返回 404
- `PUT /api/policies/:id` → 绑定请求体，校验 URL id 与 body id 一致 → `UpsertPolicy`
- `DELETE /api/policies/:id` → `DeletePolicy` → 204 NoContent

### 测试策略

`api_test.go` 中使用：
- `gin.CreateTestContext` + `httptest.NewRecorder` 构造 HTTP 请求
- 内存 SQLite Store（`:memory:`）作为依赖注入
- 每个 handler 测试覆盖：成功路径、参数错误、资源不存在

---

## 模块二：Rule

Store 层 Rule CRUD 已实现，仅补 API handler。

### API 层

补全以下 handler：
- `GET /api/rules` → `ListRules`，返回 `[]Rule`
- `POST /api/rules` → 绑定 `Rule` 请求体 → `CreateRule` → 201 Created；若 `id` 已存在返回 409 Conflict

### 测试策略

同 Policy，覆盖列表查询和创建，包括 ID 冲突场景。

---

## 模块三：Report

### Store 层

新增方法：

```go
func (s *SQLiteStore) QueryReportSummary(ctx context.Context) (*models.ReportSummary, error)
```

`ReportSummary` 结构体（新增到 `models/models.go`）：

```go
type ReportSummary struct {
    TotalRequests     int64            `json:"total_requests"`
    Blocked           int64            `json:"blocked"`
    Warned            int64            `json:"warned"`
    Allowed           int64            `json:"allowed"`
    TopBlockedTools   []ToolCount      `json:"top_blocked_tools"`
    TopTriggeredRules []RuleCount      `json:"top_triggered_rules"`
}

type ToolCount struct {
    ToolName string `json:"tool_name"`
    Count    int64  `json:"count"`
}

type RuleCount struct {
    RuleID string `json:"rule_id"`
    Count  int64  `json:"count"`
}
```

SQL 查询分 3 条执行：
1. `SELECT action, COUNT(*) FROM logs GROUP BY action`
2. `SELECT tool_name, COUNT(*) FROM logs WHERE action='block' GROUP BY tool_name ORDER BY COUNT(*) DESC LIMIT 5`
3. `SELECT rule_id, COUNT(*) FROM logs WHERE rule_id != '' GROUP BY rule_id ORDER BY COUNT(*) DESC LIMIT 5`

### API 层

补全 handler：
- `GET /api/reports/summary` → `QueryReportSummary`，返回 `ReportSummary`

### 测试策略

在 `api_test.go` 中：
- 构造多条日志数据（allow / block / warn）
- 调用 `QueryReportSummary` 验证统计数字
- 验证 top blocked tools 和 top triggered rules 排序

---

## 错误处理

| 场景 | 响应码 | 说明 |
|------|--------|------|
| 资源不存在 | 404 | Get/Delete/Update 时 |
| 参数绑定失败 | 400 | JSON 格式错误或必填字段缺失 |
| ID 冲突 | 409 | POST /api/rules 时 id 已存在 |
| 内部错误 | 500 | Store 操作失败 |

---

## 文件变更清单

| 文件 | 变更类型 |
|------|---------|
| `internal/models/models.go` | 新增 `ReportSummary` / `ToolCount` / `RuleCount` |
| `internal/store/store.go` | 新增 policies 表、补 Policy CRUD、新增 `QueryReportSummary` |
| `internal/api/api.go` | 补全所有空 handler |
| `internal/api/api_test.go` | 新增 Policy / Rule / Report 测试 |
| `internal/store/store_test.go` | 新增 Policy / ReportSummary 测试 |

---

## 依赖关系

```
models（新增 ReportSummary）
    ↑
store（补 Policy + ReportSummary）
    ↑
api（补 handler）
    ↑
api_test / store_test
```

无外部新依赖，仅使用现有 `gin`、`modernc.org/sqlite`。
