package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Samantha09/mcpguard/internal/models"
	"github.com/Samantha09/mcpguard/internal/store"
	"github.com/gin-gonic/gin"
)

func newTestPlatform(t *testing.T) (*Server, store.Store) {
	s := store.NewSQLiteStore(":memory:")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return NewServer(s), s
}

func TestPlatform_Health(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv, _ := newTestPlatform(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestPlatform_RegisterProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv, _ := newTestPlatform(t)
	body, _ := json.Marshal(map[string]string{
		"name":     "test-probe",
		"hostname": "localhost",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/probes/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["probe_id"] == "" || resp["token"] == "" {
		t.Fatalf("expected probe_id and token, got %+v", resp)
	}
}

func TestPlatform_CreateAndGetRule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv, _ := newTestPlatform(t)
	rule := models.Rule{
		ID:      "r1",
		Name:    "ban rm",
		Type:    "keyword",
		Pattern: "rm -rf",
		Action:  models.ActionBlock,
		Enabled: true,
	}
	body, _ := json.Marshal(rule)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// 获取规则
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/rules/r1", nil)
	srv.Router().ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
}

func TestPlatform_ListRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv, _ := newTestPlatform(t)
	_ = createRule(t, srv, &models.Rule{ID: "r1", Name: "a", Type: "keyword", Pattern: "x", Action: models.ActionBlock})
	_ = createRule(t, srv, &models.Rule{ID: "r2", Name: "b", Type: "keyword", Pattern: "y", Action: models.ActionAllow})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/rules", nil)
	srv.Router().ServeHTTP(w, req)

	var rules []models.Rule
	json.Unmarshal(w.Body.Bytes(), &rules)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func createRule(t *testing.T, srv *Server, rule *models.Rule) *models.Rule {
	body, _ := json.Marshal(rule)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create rule failed: %d %s", w.Code, w.Body.String())
	}
	var result models.Rule
	json.Unmarshal(w.Body.Bytes(), &result)
	return &result
}
