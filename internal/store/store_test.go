// 存储层测试 — SQLite 审计日志 CRUD
package store

import (
	"context"
	"testing"
	"time"

	"github.com/Samantha09/mcpguard/internal/models"
)

func newTestStore(t *testing.T) *SQLiteStore {
	s := NewSQLiteStore(":memory:")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("init store failed: %v", err)
	}
	return s
}

func TestSQLiteStore_InitCreatesTable(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// 验证表存在：尝试插入一条记录
	entry := &models.LogEntry{
		Direction: "request",
		Method:    "tools/call",
		Action:    models.ActionAllow,
		Request:   "{}",
	}
	if err := s.InsertLog(context.Background(), entry); err != nil {
		t.Fatalf("insert after init failed: %v", err)
	}
}

func TestSQLiteStore_InsertAndQuery(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	entry := &models.LogEntry{
		Direction: "request",
		Method:    "tools/call",
		ToolName:  "read_file",
		Action:    models.ActionAllow,
		Reason:    "",
		Request:   `{"jsonrpc":"2.0"}`,
		ClientID:  "agent-1",
	}
	if err := s.InsertLog(context.Background(), entry); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	logs, err := s.QueryLogs(context.Background(), LogFilter{})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Method != "tools/call" {
		t.Fatalf("expected method tools/call, got %s", logs[0].Method)
	}
}

func TestSQLiteStore_QueryByAction(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", Action: models.ActionAllow, Request: "{}"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", Action: models.ActionBlock, Request: "{}"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", Action: models.ActionWarn, Request: "{}"})

	blockAction := models.ActionBlock
	logs, err := s.QueryLogs(context.Background(), LogFilter{Action: &blockAction})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 blocked log, got %d", len(logs))
	}
	if logs[0].Action != models.ActionBlock {
		t.Fatalf("expected block action, got %s", logs[0].Action)
	}
}

func TestSQLiteStore_QueryByMethod(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", Action: models.ActionAllow, Request: "{}"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "initialize", Action: models.ActionAllow, Request: "{}"})

	logs, err := s.QueryLogs(context.Background(), LogFilter{Method: "initialize"})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Method != "initialize" {
		t.Fatalf("expected initialize, got %s", logs[0].Method)
	}
}

func TestSQLiteStore_QueryByToolName(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", ToolName: "execute_command", Action: models.ActionAllow, Request: "{}"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", ToolName: "read_file", Action: models.ActionAllow, Request: "{}"})

	logs, err := s.QueryLogs(context.Background(), LogFilter{ToolName: "read_file"})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].ToolName != "read_file" {
		t.Fatalf("expected read_file, got %s", logs[0].ToolName)
	}
}

func TestSQLiteStore_QueryByTimeRange(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	now := time.Now()
	_ = s.InsertLog(context.Background(), &models.LogEntry{Timestamp: now.Add(-2 * time.Hour), Direction: "request", Method: "tools/call", Action: models.ActionAllow, Request: "{}"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Timestamp: now, Direction: "request", Method: "tools/call", Action: models.ActionAllow, Request: "{}"})

	start := now.Add(-30 * time.Minute)
	end := now.Add(30 * time.Minute)
	logs, err := s.QueryLogs(context.Background(), LogFilter{StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log in range, got %d", len(logs))
	}
}

func TestSQLiteStore_QueryPagination(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	for i := 0; i < 5; i++ {
		_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", Action: models.ActionAllow, Request: "{}"})
	}

	logs, err := s.QueryLogs(context.Background(), LogFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}

	logs2, err := s.QueryLogs(context.Background(), LogFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(logs2) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs2))
	}

	// 验证两组 ID 不同
	if logs[0].ID == logs2[0].ID {
		t.Fatal("pagination returned same results")
	}
}

func TestSQLiteStore_QueryCombinedFilter(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", ToolName: "execute_command", Action: models.ActionBlock, Request: "{}"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", ToolName: "read_file", Action: models.ActionBlock, Request: "{}"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", ToolName: "execute_command", Action: models.ActionAllow, Request: "{}"})

	blockAction := models.ActionBlock
	logs, err := s.QueryLogs(context.Background(), LogFilter{
		Action:   &blockAction,
		ToolName: "execute_command",
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].ToolName != "execute_command" || logs[0].Action != models.ActionBlock {
		t.Fatalf("unexpected log: %+v", logs[0])
	}
}

func TestSQLiteStore_QueryEmptyReturnsEmpty(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	logs, err := s.QueryLogs(context.Background(), LogFilter{})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected 0 logs, got %d", len(logs))
	}
}

// --- Probe Tests ---

func TestSQLiteStore_RegisterAndGetProbe(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	probe := &models.Probe{
		ID:       "probe-1",
		Name:     "test-probe",
		Hostname: "localhost",
		IP:       "127.0.0.1",
		Status:   "online",
	}
	if err := s.RegisterProbe(context.Background(), probe, "hash123"); err != nil {
		t.Fatalf("register probe failed: %v", err)
	}

	got, err := s.GetProbe(context.Background(), "probe-1")
	if err != nil {
		t.Fatalf("get probe failed: %v", err)
	}
	if got.Name != "test-probe" {
		t.Fatalf("expected name test-probe, got %s", got.Name)
	}
}

func TestSQLiteStore_ListProbes(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_ = s.RegisterProbe(context.Background(), &models.Probe{ID: "p1", Name: "a"}, "h1")
	_ = s.RegisterProbe(context.Background(), &models.Probe{ID: "p2", Name: "b"}, "h2")

	probes, err := s.ListProbes(context.Background())
	if err != nil {
		t.Fatalf("list probes failed: %v", err)
	}
	if len(probes) != 2 {
		t.Fatalf("expected 2 probes, got %d", len(probes))
	}
}

func TestSQLiteStore_UpdateProbeStatus(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_ = s.RegisterProbe(context.Background(), &models.Probe{ID: "p1", Name: "a", Status: "online"}, "h1")
	_ = s.UpdateProbeStatus(context.Background(), "p1", "offline")

	got, _ := s.GetProbe(context.Background(), "p1")
	if got.Status != "offline" {
		t.Fatalf("expected offline, got %s", got.Status)
	}
}

// --- Rule Tests ---

func TestSQLiteStore_CreateAndGetRule(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	rule := &models.Rule{
		ID:      "r1",
		Name:    "ban rm",
		Type:    "keyword",
		Pattern: "rm -rf",
		Action:  models.ActionBlock,
		Enabled: true,
	}
	if err := s.CreateRule(context.Background(), rule); err != nil {
		t.Fatalf("create rule failed: %v", err)
	}

	got, err := s.GetRule(context.Background(), "r1")
	if err != nil {
		t.Fatalf("get rule failed: %v", err)
	}
	if got.Name != "ban rm" {
		t.Fatalf("expected name ban rm, got %s", got.Name)
	}
}

func TestSQLiteStore_ListRules(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_ = s.CreateRule(context.Background(), &models.Rule{ID: "r1", Name: "a", Type: "keyword", Pattern: "x", Action: models.ActionBlock})
	_ = s.CreateRule(context.Background(), &models.Rule{ID: "r2", Name: "b", Type: "keyword", Pattern: "y", Action: models.ActionAllow})

	rules, err := s.ListRules(context.Background())
	if err != nil {
		t.Fatalf("list rules failed: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestSQLiteStore_UpdateAndDeleteRule(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_ = s.CreateRule(context.Background(), &models.Rule{ID: "r1", Name: "old", Type: "keyword", Pattern: "x", Action: models.ActionBlock})
	_ = s.UpdateRule(context.Background(), &models.Rule{ID: "r1", Name: "new", Type: "keyword", Pattern: "y", Action: models.ActionBlock})

	got, _ := s.GetRule(context.Background(), "r1")
	if got.Name != "new" {
		t.Fatalf("expected new name, got %s", got.Name)
	}

	_ = s.DeleteRule(context.Background(), "r1")
	_, err := s.GetRule(context.Background(), "r1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}
