// HTTP API 测试
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
