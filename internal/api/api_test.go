// HTTP API 测试
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Samantha09/mcpguard/internal/models"
	"github.com/Samantha09/mcpguard/internal/store"
)

func newTestServer(t *testing.T) (*Server, store.Store) {
	s := store.NewSQLiteStore(":memory:")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return NewServer(s), s
}

func TestHandleHealth(t *testing.T) {
	srv, _ := newTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body)
	}
}

func TestHandleListLogs(t *testing.T) {
	srv, s := newTestServer(t)

	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", Action: models.ActionBlock, Request: "{}"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", Action: models.ActionAllow, Request: "{}"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/logs", nil)
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var logs []models.LogEntry
	if err := json.Unmarshal(w.Body.Bytes(), &logs); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
}

func TestHandleListLogs_FilterByAction(t *testing.T) {
	srv, s := newTestServer(t)

	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", Action: models.ActionBlock, Request: "{}"})
	_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", Action: models.ActionAllow, Request: "{}"})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/logs?action=block", nil)
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var logs []models.LogEntry
	if err := json.Unmarshal(w.Body.Bytes(), &logs); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Action != models.ActionBlock {
		t.Fatalf("expected block, got %s", logs[0].Action)
	}
}

func TestHandleListLogs_Pagination(t *testing.T) {
	srv, s := newTestServer(t)

	for i := 0; i < 5; i++ {
		_ = s.InsertLog(context.Background(), &models.LogEntry{Direction: "request", Method: "tools/call", Action: models.ActionAllow, Request: "{}"})
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/logs?limit=2&offset=0", nil)
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var logs []models.LogEntry
	if err := json.Unmarshal(w.Body.Bytes(), &logs); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
}

func TestHandleListLogs_Empty(t *testing.T) {
	srv, _ := newTestServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/logs", nil)
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var logs []models.LogEntry
	if err := json.Unmarshal(w.Body.Bytes(), &logs); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if len(logs) != 0 {
		t.Fatalf("expected 0 logs, got %d", len(logs))
	}
}

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
	var resp models.Policy
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID != "p1" || resp.Name != "禁止删除" {
		t.Fatalf("unexpected response: %+v", resp)
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

func TestHandleUpdatePolicy_IDMismatch(t *testing.T) {
	srv, _ := newTestServer(t)

	body := `{"id":"p2","name":"new","enabled":true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/policies/p1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

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
