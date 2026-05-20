// MCP 代理测试 — stdio 双向转发与拦截
package proxy

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/Samantha09/mcpguard/internal/detector"
	"github.com/Samantha09/mcpguard/internal/models"
	"github.com/Samantha09/mcpguard/internal/rules"
	"github.com/Samantha09/mcpguard/internal/store"
)

func newTestProxy(t *testing.T, ruleList []rules.Rule, agentReq, serverResp string) (*MCProxy, *bytes.Buffer, *bytes.Buffer) {
	s := store.NewSQLiteStore(":memory:")
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	agentIn := bytes.NewBufferString(agentReq)
	agentOut := &bytes.Buffer{}
	serverIn := &bytes.Buffer{}
	serverOut := io.NopCloser(bytes.NewBufferString(serverResp))

	p := &MCProxy{
		pipeline:    detector.NewPipeline(rules.NewRuleDetector(ruleList)),
		store:       s,
		probeClient: nil,
		agentIn:     agentIn,
		agentOut:    agentOut,
		serverIn:    serverIn,
		serverOut:   serverOut,
	}

	return p, agentOut, serverIn
}

func TestMCProxy_BlockRequest(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"execute_command","arguments":{"command":"rm -rf /"}}}` + "\n"
	p, agentOut, serverIn := newTestProxy(t, []rules.Rule{
		{ID: "r1", Name: "ban rm", Type: rules.RuleTypeKeyword, Pattern: "rm -rf", Action: models.ActionBlock, Enabled: true},
	}, req, "")

	ctx := context.Background()
	err := p.runLoop(ctx)
	if err != nil && err != io.EOF {
		t.Fatalf("runLoop error: %v", err)
	}

	out := agentOut.String()
	if !strings.Contains(out, `"error"`) {
		t.Fatalf("expected block response with error, got: %s", out)
	}
	if !strings.Contains(out, "rm -rf") {
		t.Fatalf("expected error message to contain rule pattern, got: %s", out)
	}
	if serverIn.Len() != 0 {
		t.Fatalf("request should not be forwarded to server when blocked, got: %s", serverIn.String())
	}
}

func TestMCProxy_AllowRequest(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"/tmp/test.txt"}}}` + "\n"
	resp := `{"jsonrpc":"2.0","id":1,"result":{"content":"hello"}}` + "\n"
	p, agentOut, serverIn := newTestProxy(t, []rules.Rule{
		{ID: "r1", Name: "ban rm", Type: rules.RuleTypeKeyword, Pattern: "rm -rf", Action: models.ActionBlock, Enabled: true},
	}, req, resp)

	ctx := context.Background()
	err := p.runLoop(ctx)
	if err != nil && err != io.EOF {
		t.Fatalf("runLoop error: %v", err)
	}

	out := agentOut.String()
	if !strings.Contains(out, `"result"`) {
		t.Fatalf("expected forwarded response with result, got: %s", out)
	}
	if !strings.Contains(serverIn.String(), "read_file") {
		t.Fatalf("request should be forwarded to server, got: %s", serverIn.String())
	}
}

func TestMCProxy_WarnRequest(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"execute_command","arguments":{"command":"ls"}}}` + "\n"
	resp := `{"jsonrpc":"2.0","id":1,"result":{"content":"ok"}}` + "\n"
	p, agentOut, serverIn := newTestProxy(t, []rules.Rule{
		{ID: "r1", Name: "warn execute", Type: rules.RuleTypeToolName, Pattern: "execute_command", Action: models.ActionWarn, Enabled: true},
	}, req, resp)

	ctx := context.Background()
	err := p.runLoop(ctx)
	if err != nil && err != io.EOF {
		t.Fatalf("runLoop error: %v", err)
	}

	out := agentOut.String()
	if !strings.Contains(out, `"result"`) {
		t.Fatalf("expected forwarded response with result, got: %s", out)
	}
	if serverIn.Len() == 0 {
		t.Fatal("request should be forwarded to server for warn action")
	}

	logs, _ := p.store.QueryLogs(context.Background(), store.LogFilter{})
	if len(logs) != 1 || logs[0].Action != models.ActionWarn {
		t.Fatalf("expected warn log, got: %+v", logs)
	}
}

func TestMCProxy_NonToolsCallForwarded(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}` + "\n"
	resp := `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{}}}` + "\n"
	p, agentOut, serverIn := newTestProxy(t, []rules.Rule{
		{ID: "r1", Name: "ban rm", Type: rules.RuleTypeKeyword, Pattern: "rm -rf", Action: models.ActionBlock, Enabled: true},
	}, req, resp)

	ctx := context.Background()
	err := p.runLoop(ctx)
	if err != nil && err != io.EOF {
		t.Fatalf("runLoop error: %v", err)
	}

	out := agentOut.String()
	if !strings.Contains(out, `"result"`) {
		t.Fatalf("expected forwarded response, got: %s", out)
	}
	if serverIn.Len() == 0 {
		t.Fatal("non-tools/call request should be forwarded")
	}
}

func TestMCProxy_MultipleRequests(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"execute_command","arguments":{"command":"rm -rf /"}}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"/tmp/test.txt"}}}` + "\n"
	resp := `{"jsonrpc":"2.0","id":2,"result":{"content":"hello"}}` + "\n"
	p, agentOut, serverIn := newTestProxy(t, []rules.Rule{
		{ID: "r1", Name: "ban rm", Type: rules.RuleTypeKeyword, Pattern: "rm -rf", Action: models.ActionBlock, Enabled: true},
	}, req, resp)

	ctx := context.Background()
	err := p.runLoop(ctx)
	if err != nil && err != io.EOF {
		t.Fatalf("runLoop error: %v", err)
	}

	out := agentOut.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 response lines, got %d: %s", len(lines), out)
	}
	if !strings.Contains(lines[0], `"error"`) {
		t.Fatalf("first request should be blocked, got: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"result"`) {
		t.Fatalf("second request should be allowed, got: %s", lines[1])
	}
	if !strings.Contains(serverIn.String(), "read_file") {
		t.Fatalf("second request should be forwarded, got: %s", serverIn.String())
	}
	if strings.Contains(serverIn.String(), "rm -rf") {
		t.Fatal("first request should not be forwarded")
	}
}
