# API / Store 空实现补全实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补全 API 和 Store 层的 Policy / Rule / Report 空实现，并同步补齐单元测试。

**Architecture:** TDD 纵向切分，按模块逐个闭环（Policy → Rule → Report）。每个模块内部遵循"先写测试 → 跑失败 → 实现代码 → 跑通过 → 提交"的循环。

**Tech Stack:** Go 1.24, Gin, modernc.org/sqlite

---

## 文件结构

| 文件 | 责任 |
|------|------|
| `internal/models/models.go` | 新增 `ReportSummary`、`ToolCount`、`RuleCount` 数据结构 |
| `internal/store/store.go` | 新增 `policies` 表 Schema；补全 `UpsertPolicy/GetPolicy/ListPolicies/DeletePolicy`；新增 `QueryReportSummary` |
| `internal/api/api.go` | 补全 `handleListPolicies/handleCreatePolicy/handleGetPolicy/handleUpdatePolicy/handleDeletePolicy/handleListRules/handleCreateRule/handleReportSummary` |
| `internal/store/store_test.go` | 新增 Policy CRUD 测试、ReportSummary 测试 |
| `internal/api/api_test.go` | 新增 Policy / Rule / Report HTTP handler 测试 |

---

### Task 1: 新增 ReportSummary 模型

**Files:**
- Modify: `internal/models/models.go:137`（在 Rule 结构体之后追加）

- [ ] **Step 1: 追加 ReportSummary 相关结构体**

```go
// ReportSummary 安全报告汇总
type ReportSummary struct {
	TotalRequests     int64       `json:"total_requests"`
	Blocked           int64       `json:"blocked"`
	Warned            int64       `json:"warned"`
	Allowed           int64       `json:"allowed"`
	TopBlockedTools   []ToolCount `json:"top_blocked_tools"`
	TopTriggeredRules []RuleCount `json:"top_triggered_rules"`
}

// ToolCount 工具统计项
type ToolCount struct {
	ToolName string `json:"tool_name"`
	Count    int64  `json:"count"`
}

// RuleCount 规则统计项
type RuleCount struct {
	RuleID string `json:"rule_id"`
	Count  int64  `json:"count"`
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add internal/models/models.go
git commit -m "feat(2): 新增 ReportSummary 数据模型"
```

---

### Task 2: Policy Store（TDD）

**Files:**
- Modify: `internal/store/store.go:76`（Schema 中追加 policies 表）
- Modify: `internal/store/store.go:341`（Policy 空方法实现区）
- Modify: `internal/store/store_test.go:326`（追加 Policy 测试）

- [ ] **Step 1: 写 Policy Store 失败测试**

在 `internal/store/store_test.go` 末尾追加：

```go
// --- Policy Tests ---

func TestSQLiteStore_UpsertAndGetPolicy(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	policy := &models.Policy{
		ID:          "policy-1",
		Name:        "禁止删除",
		Description: "拦截所有删除操作",
		Enabled:     true,
		RuleIDs:     []string{"rule-1", "rule-2"},
		LLMEnabled:  false,
	}
	if err := s.UpsertPolicy(context.Background(), policy); err != nil {
		t.Fatalf("upsert policy failed: %v", err)
	}

	got, err := s.GetPolicy(context.Background(), "policy-1")
	if err != nil {
		t.Fatalf("get policy failed: %v", err)
	}
	if got.Name != "禁止删除" {
		t.Fatalf("expected name 禁止删除, got %s", got.Name)
	}
	if len(got.RuleIDs) != 2 {
		t.Fatalf("expected 2 rule ids, got %d", len(got.RuleIDs))
	}
}

func TestSQLiteStore_ListPolicies(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_ = s.UpsertPolicy(context.Background(), &models.Policy{ID: "p1", Name: "a"})
	_ = s.UpsertPolicy(context.Background(), &models.Policy{ID: "p2", Name: "b"})

	policies, err := s.ListPolicies(context.Background())
	if err != nil {
		t.Fatalf("list policies failed: %v", err)
	}
	if len(policies) != 2 {
		t.Fatalf("expected 2 policies, got %d", len(policies))
	}
}

func TestSQLiteStore_DeletePolicy(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_ = s.UpsertPolicy(context.Background(), &models.Policy{ID: "p1", Name: "a"})
	_ = s.DeletePolicy(context.Background(), "p1")

	_, err := s.GetPolicy(context.Background(), "p1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/store -run TestSQLiteStore_UpsertAndGetPolicy -v`
Expected: FAIL — `UpsertPolicy` 返回 nil（空实现），但 GetPolicy 可能也返回 nil 导致后续断言失败；或者编译报错如果方法签名不匹配。

- [ ] **Step 3: 在 Schema 中新增 policies 表**

在 `internal/store/store.go` 的 `schema` 字符串中，在 `rules` 表之后追加：

```sql
CREATE TABLE IF NOT EXISTS policies (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    rule_ids TEXT,
    llm_enabled BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 4: 实现 Policy Store 方法**

替换 `internal/store/store.go` 中 Policy 的空方法区（约 341-351 行）：

```go
func (s *SQLiteStore) UpsertPolicy(ctx context.Context, policy *models.Policy) error {
	ruleIDs, _ := json.Marshal(policy.RuleIDs)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO policies (id, name, description, enabled, rule_ids, llm_enabled, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			 name=excluded.name, description=excluded.description, enabled=excluded.enabled,
			 rule_ids=excluded.rule_ids, llm_enabled=excluded.llm_enabled, updated_at=excluded.updated_at`,
		policy.ID, policy.Name, policy.Description, policy.Enabled, string(ruleIDs), policy.LLMEnabled, time.Now(),
	)
	return err
}

func (s *SQLiteStore) GetPolicy(ctx context.Context, id string) (*models.Policy, error) {
	var p models.Policy
	var ruleIDsRaw string
	var createdAt, updatedAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, enabled, rule_ids, llm_enabled, created_at, updated_at FROM policies WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Enabled, &ruleIDsRaw, &p.LLMEnabled, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(ruleIDsRaw), &p.RuleIDs)
	return &p, nil
}

func (s *SQLiteStore) ListPolicies(ctx context.Context) ([]*models.Policy, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, enabled, rule_ids, llm_enabled, created_at, updated_at FROM policies ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []*models.Policy
	for rows.Next() {
		var p models.Policy
		var ruleIDsRaw string
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Enabled, &ruleIDsRaw, &p.LLMEnabled, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(ruleIDsRaw), &p.RuleIDs)
		policies = append(policies, &p)
	}
	return policies, rows.Err()
}

func (s *SQLiteStore) DeletePolicy(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM policies WHERE id = ?`, id)
	return err
}
```

注意：需要在 `internal/store/store.go` 顶部 import 中加入 `"encoding/json"`。

- [ ] **Step 5: 跑测试确认通过**

Run: `go test ./internal/store -run TestSQLiteStore_ -v`
Expected: PASS（所有 Store 测试通过）

- [ ] **Step 6: Commit**

```bash
git add internal/store/store.go internal/store/store_test.go
git commit -m "feat(2): 实现 Policy Store CRUD"
```

---

### Task 3: Policy API（TDD）

**Files:**
- Modify: `internal/api/api.go:108-112`（Policy handler 空实现区）
- Modify: `internal/api/api_test.go:130`（追加 Policy 测试）

- [ ] **Step 1: 写 Policy API 失败测试**

在 `internal/api/api_test.go` 末尾追加：

```go
// --- Policy Tests ---

func TestHandleListPolicies(t *testing.T) {
	srv, s := newTestServer(t)
	_ = s.UpsertPolicy(context.Background(), &models.Policy{ID: "p1", Name: "a"})
	_ = s.UpsertPolicy(context.Background(), &models.Policy{ID: "p2", Name: "b"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/policies", nil)
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var policies []models.Policy
	if err := json.Unmarshal(w.Body.Bytes(), &policies); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(policies) != 2 {
		t.Fatalf("expected 2 policies, got %d", len(policies))
	}
}

func TestHandleCreatePolicy(t *testing.T) {
	srv, _ := newTestServer(t)

	body := `{"id":"p1","name":"禁止删除","enabled":true,"rule_ids":["r1"],"llm_enabled":false}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/policies", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestHandleGetPolicy(t *testing.T) {
	srv, s := newTestServer(t)
	_ = s.UpsertPolicy(context.Background(), &models.Policy{ID: "p1", Name: "a"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/policies/p1", nil)
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var p models.Policy
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.ID != "p1" {
		t.Fatalf("expected p1, got %s", p.ID)
	}
}

func TestHandleGetPolicy_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/policies/notexist", nil)
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleUpdatePolicy(t *testing.T) {
	srv, s := newTestServer(t)
	_ = s.UpsertPolicy(context.Background(), &models.Policy{ID: "p1", Name: "old"})

	body := `{"id":"p1","name":"new","enabled":true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/policies/p1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	got, _ := s.GetPolicy(context.Background(), "p1")
	if got.Name != "new" {
		t.Fatalf("expected new name, got %s", got.Name)
	}
}

func TestHandleDeletePolicy(t *testing.T) {
	srv, s := newTestServer(t)
	_ = s.UpsertPolicy(context.Background(), &models.Policy{ID: "p1", Name: "a"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/policies/p1", nil)
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	_, err := s.GetPolicy(context.Background(), "p1")
	if err == nil {
		t.Fatal("expected policy deleted")
	}
}
```

注意：需要在 `internal/api/api_test.go` 顶部 import 中加入 `"strings"`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/api -run TestHandleListPolicies -v`
Expected: FAIL — 空 handler 返回 200 空体，但断言期望非空数组

- [ ] **Step 3: 实现 Policy API handler**

替换 `internal/api/api.go` 中 Policy 空方法区：

```go
func (s *Server) handleListPolicies(c *gin.Context) {
	policies, err := s.store.ListPolicies(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policies)
}

func (s *Server) handleCreatePolicy(c *gin.Context) {
	var p models.Policy
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.store.UpsertPolicy(c.Request.Context(), &p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, p)
}

func (s *Server) handleGetPolicy(c *gin.Context) {
	id := c.Param("id")
	p, err := s.store.GetPolicy(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "policy not found"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (s *Server) handleUpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	var p models.Policy
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if p.ID != "" && p.ID != id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id mismatch"})
		return
	}
	p.ID = id
	if err := s.store.UpsertPolicy(c.Request.Context(), &p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (s *Server) handleDeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := s.store.DeletePolicy(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/api -run TestHandle -v`
Expected: PASS（所有 API 测试通过）

- [ ] **Step 5: Commit**

```bash
git add internal/api/api.go internal/api/api_test.go
git commit -m "feat(2): 实现 Policy API CRUD"
```

---

### Task 4: Rule API（TDD）

**Files:**
- Modify: `internal/api/api.go:113-114`（Rule handler 空实现区）
- Modify: `internal/api/api_test.go`（追加 Rule 测试）

- [ ] **Step 1: 写 Rule API 失败测试**

在 `internal/api/api_test.go` 末尾追加：

```go
// --- Rule Tests ---

func TestHandleListRules(t *testing.T) {
	srv, s := newTestServer(t)
	_ = s.CreateRule(context.Background(), &models.Rule{ID: "r1", Name: "a", Type: "keyword", Pattern: "x", Action: models.ActionBlock})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/rules", nil)
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var rules []models.Rule
	if err := json.Unmarshal(w.Body.Bytes(), &rules); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

func TestHandleCreateRule(t *testing.T) {
	srv, _ := newTestServer(t)

	body := `{"id":"r1","name":"拦截 rm","type":"keyword","pattern":"rm -rf","action":"block","enabled":true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestHandleCreateRule_Conflict(t *testing.T) {
	srv, s := newTestServer(t)
	_ = s.CreateRule(context.Background(), &models.Rule{ID: "r1", Name: "a", Type: "keyword", Pattern: "x", Action: models.ActionBlock})

	body := `{"id":"r1","name":"b","type":"keyword","pattern":"y","action":"block","enabled":true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/api -run TestHandleListRules -v`
Expected: FAIL — 空 handler 返回 200 空体

- [ ] **Step 3: 实现 Rule API handler**

替换 `internal/api/api.go` 中 Rule 空方法区：

```go
func (s *Server) handleListRules(c *gin.Context) {
	rules, err := s.store.ListRules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (s *Server) handleCreateRule(c *gin.Context) {
	var r models.Rule
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 检查 ID 是否已存在
	if _, err := s.store.GetRule(c.Request.Context(), r.ID); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "rule already exists"})
		return
	}
	if err := s.store.CreateRule(c.Request.Context(), &r); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, r)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/api -run TestHandleListRules -v`
Run: `go test ./internal/api -run TestHandleCreateRule -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/api.go internal/api/api_test.go
git commit -m "feat(2): 实现 Rule API"
```

---

### Task 5: Report Store（TDD）

**Files:**
- Modify: `internal/store/store.go`（追加 `QueryReportSummary` 方法）
- Modify: `internal/store/store_test.go`（追加 ReportSummary 测试）

- [ ] **Step 1: 写 Report Store 失败测试**

在 `internal/store/store_test.go` 末尾追加：

```go
// --- Report Tests ---

func TestSQLiteStore_QueryReportSummary(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// 构造日志数据
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", ToolName: "execute_command", Action: models.ActionBlock, Reason: "规则匹配", Request: "{}", RuleID: "rule-1"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", ToolName: "execute_command", Action: models.ActionBlock, Reason: "规则匹配", Request: "{}", RuleID: "rule-1"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", ToolName: "write_file", Action: models.ActionBlock, Request: "{}", RuleID: "rule-2"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", ToolName: "read_file", Action: models.ActionWarn, Request: "{}"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", ToolName: "read_file", Action: models.ActionAllow, Request: "{}"})

	summary, err := s.QueryReportSummary(context.Background())
	if err != nil {
		t.Fatalf("query summary failed: %v", err)
	}
	if summary.TotalRequests != 5 {
		t.Fatalf("expected total 5, got %d", summary.TotalRequests)
	}
	if summary.Blocked != 3 {
		t.Fatalf("expected blocked 3, got %d", summary.Blocked)
	}
	if summary.Warned != 1 {
		t.Fatalf("expected warned 1, got %d", summary.Warned)
	}
	if summary.Allowed != 1 {
		t.Fatalf("expected allowed 1, got %d", summary.Allowed)
	}
	if len(summary.TopBlockedTools) == 0 || summary.TopBlockedTools[0].ToolName != "execute_command" || summary.TopBlockedTools[0].Count != 2 {
		t.Fatalf("unexpected top blocked tools: %+v", summary.TopBlockedTools)
	}
	if len(summary.TopTriggeredRules) == 0 || summary.TopTriggeredRules[0].RuleID != "rule-1" || summary.TopTriggeredRules[0].Count != 2 {
		t.Fatalf("unexpected top triggered rules: %+v", summary.TopTriggeredRules)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/store -run TestSQLiteStore_QueryReportSummary -v`
Expected: FAIL — `QueryReportSummary` 方法不存在或返回 nil

- [ ] **Step 3: 实现 QueryReportSummary**

在 `internal/store/store.go` 末尾（DeletePolicy 之后）追加：

```go
func (s *SQLiteStore) QueryReportSummary(ctx context.Context) (*models.ReportSummary, error) {
	summary := &models.ReportSummary{}

	// action 统计
	rows, err := s.db.QueryContext(ctx, `SELECT action, COUNT(*) FROM logs GROUP BY action`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var action string
		var count int64
		if err := rows.Scan(&action, &count); err != nil {
			rows.Close()
			return nil, err
		}
		summary.TotalRequests += count
		switch models.Action(action) {
		case models.ActionBlock:
			summary.Blocked = count
		case models.ActionWarn:
			summary.Warned = count
		case models.ActionAllow:
			summary.Allowed = count
		}
	}
	rows.Close()

	// top blocked tools
	toolRows, err := s.db.QueryContext(ctx,
		`SELECT tool_name, COUNT(*) FROM logs WHERE action = 'block' AND tool_name IS NOT NULL AND tool_name != '' GROUP BY tool_name ORDER BY COUNT(*) DESC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	for toolRows.Next() {
		var tc models.ToolCount
		if err := toolRows.Scan(&tc.ToolName, &tc.Count); err != nil {
			toolRows.Close()
			return nil, err
		}
		summary.TopBlockedTools = append(summary.TopBlockedTools, tc)
	}
	toolRows.Close()

	// top triggered rules
	ruleRows, err := s.db.QueryContext(ctx,
		`SELECT rule_id, COUNT(*) FROM logs WHERE rule_id IS NOT NULL AND rule_id != '' GROUP BY rule_id ORDER BY COUNT(*) DESC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	for ruleRows.Next() {
		var rc models.RuleCount
		if err := ruleRows.Scan(&rc.RuleID, &rc.Count); err != nil {
			ruleRows.Close()
			return nil, err
		}
		summary.TopTriggeredRules = append(summary.TopTriggeredRules, rc)
	}
	ruleRows.Close()

	return summary, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/store -run TestSQLiteStore_QueryReportSummary -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/store.go internal/store/store_test.go
git commit -m "feat(2): 实现 ReportSummary Store 查询"
```

---

### Task 6: Report API（TDD）

**Files:**
- Modify: `internal/api/api.go:115`（Report handler 空实现）
- Modify: `internal/api/api_test.go`（追加 Report 测试）

- [ ] **Step 1: 写 Report API 失败测试**

在 `internal/api/api_test.go` 末尾追加：

```go
// --- Report Tests ---

func TestHandleReportSummary(t *testing.T) {
	srv, s := newTestServer(t)
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", ToolName: "execute_command", Action: models.ActionBlock, Request: "{}", RuleID: "rule-1"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", ToolName: "read_file", Action: models.ActionAllow, Request: "{}"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/reports/summary", nil)
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var summary models.ReportSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if summary.TotalRequests != 2 {
		t.Fatalf("expected total 2, got %d", summary.TotalRequests)
	}
	if summary.Blocked != 1 {
		t.Fatalf("expected blocked 1, got %d", summary.Blocked)
	}
	if summary.Allowed != 1 {
		t.Fatalf("expected allowed 1, got %d", summary.Allowed)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/api -run TestHandleReportSummary -v`
Expected: FAIL — 空 handler 返回 200 空体

- [ ] **Step 3: 实现 Report API handler**

替换 `internal/api/api.go` 中 `handleReportSummary` 空实现：

```go
func (s *Server) handleReportSummary(c *gin.Context) {
	summary, err := s.store.QueryReportSummary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/api -run TestHandleReportSummary -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/api.go internal/api/api_test.go
git commit -m "feat(2): 实现 Report Summary API"
```

---

### Task 7: 全量回归验证

- [ ] **Step 1: 运行全部测试**

Run: `go test ./...`
Expected: 所有包 PASS

- [ ] **Step 2: 构建验证**

Run: `go build -o mcpguard ./cmd/mcpguard`
Expected: 编译成功

- [ ] **Step 3: 提交（如测试全部通过则无需额外提交）**

---

## Self-Review Checklist

**Spec coverage:**
- [x] Policy Store CRUD — Task 2
- [x] Policy API CRUD — Task 3
- [x] Rule API — Task 4
- [x] Report Store 汇总查询 — Task 5
- [x] Report API — Task 6

**Placeholder scan:**
- [x] 无 TBD/TODO/"实现 later"/"add appropriate error handling"
- [x] 每个步骤包含完整代码或确切命令

**Type consistency:**
- [x] `ReportSummary` / `ToolCount` / `RuleCount` 字段名与模型定义一致
- [x] `QueryReportSummary` 返回 `*models.ReportSummary, error`
- [x] `UpsertPolicy` 使用 `INSERT OR REPLACE` / `ON CONFLICT` 语义一致
