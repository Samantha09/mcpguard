package probe

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Samantha09/mcpguard/internal/models"
	"github.com/Samantha09/mcpguard/internal/platform"
	"github.com/Samantha09/mcpguard/internal/store"
	"github.com/gin-gonic/gin"
)

func TestClient_Register(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := store.NewSQLiteStore(":memory:")
	_ = s.Init(context.Background())
	defer s.Close()

	plat := platform.NewServer(s)
	server := httptest.NewServer(plat.Router())
	defer server.Close()

	client := NewClient(server.URL, "")
	probeID, token, err := client.Register(context.Background(), "test", "localhost", "127.0.0.1")
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if probeID == "" || token == "" {
		t.Fatalf("expected probe_id and token")
	}
}

func TestClient_PullRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := store.NewSQLiteStore(":memory:")
	_ = s.Init(context.Background())
	defer s.Close()

	_ = s.CreateRule(context.Background(), &models.Rule{
		ID: "r1", Name: "ban rm", Type: "keyword", Pattern: "rm", Action: models.ActionBlock,
	})

	plat := platform.NewServer(s)
	server := httptest.NewServer(plat.Router())
	defer server.Close()

	client := NewClient(server.URL, "")
	_, token, _ := client.Register(context.Background(), "test", "localhost", "127.0.0.1")
	client.token = token

	rules, err := client.PullRules(context.Background())
	if err != nil {
		t.Fatalf("pull rules failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}
