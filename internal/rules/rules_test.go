// 规则引擎测试 — 规则匹配逻辑
package rules

import (
	"context"
	"testing"

	"github.com/Samantha09/mcpguard/internal/models"
)

func TestRuleDetector_EmptyRulesReturnsAllow(t *testing.T) {
	d := NewRuleDetector(nil)
	req := &models.InterceptedRequest{
		Method:   models.MethodToolsCall,
		ToolCall: &models.ToolCallInfo{Name: "read_file", Arguments: map[string]any{"path": "/tmp/test.txt"}},
	}

	result, err := d.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionAllow {
		t.Fatalf("expected allow, got %s", result.Action)
	}
}

func TestRuleDetector_KeywordMatch(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Name: "ban rm -rf", Type: RuleTypeKeyword, Pattern: "rm -rf", Action: models.ActionBlock, Enabled: true},
	}
	d := NewRuleDetector(rules)

	req := &models.InterceptedRequest{
		Method:   models.MethodToolsCall,
		ToolCall: &models.ToolCallInfo{Name: "execute_command", Arguments: map[string]any{"command": "rm -rf /"}},
	}

	result, err := d.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionBlock {
		t.Fatalf("expected block, got %s", result.Action)
	}
	if result.Detector != "rule" {
		t.Fatalf("expected detector 'rule', got %s", result.Detector)
	}
}

func TestRuleDetector_KeywordNoMatch(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Name: "ban rm -rf", Type: RuleTypeKeyword, Pattern: "rm -rf", Action: models.ActionBlock, Enabled: true},
	}
	d := NewRuleDetector(rules)

	req := &models.InterceptedRequest{
		Method:   models.MethodToolsCall,
		ToolCall: &models.ToolCallInfo{Name: "read_file", Arguments: map[string]any{"path": "/tmp/test.txt"}},
	}

	result, err := d.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionAllow {
		t.Fatalf("expected allow, got %s", result.Action)
	}
}

func TestRuleDetector_ToolNameMatch(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Name: "warn execute", Type: RuleTypeToolName, Pattern: "execute_command", Action: models.ActionWarn, Enabled: true},
	}
	d := NewRuleDetector(rules)

	req := &models.InterceptedRequest{
		Method:   models.MethodToolsCall,
		ToolCall: &models.ToolCallInfo{Name: "execute_command", Arguments: map[string]any{}},
	}

	result, err := d.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionWarn {
		t.Fatalf("expected warn, got %s", result.Action)
	}
}

func TestRuleDetector_ToolNameNoMatch(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Name: "warn execute", Type: RuleTypeToolName, Pattern: "execute_command", Action: models.ActionWarn, Enabled: true},
	}
	d := NewRuleDetector(rules)

	req := &models.InterceptedRequest{
		Method:   models.MethodToolsCall,
		ToolCall: &models.ToolCallInfo{Name: "read_file", Arguments: map[string]any{}},
	}

	result, err := d.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionAllow {
		t.Fatalf("expected allow, got %s", result.Action)
	}
}

func TestRuleDetector_RegexMatch(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Name: "ssh path", Type: RuleTypeRegex, Pattern: `\.ssh/`, Action: models.ActionBlock, Enabled: true},
	}
	d := NewRuleDetector(rules)

	req := &models.InterceptedRequest{
		Method:   models.MethodToolsCall,
		ToolCall: &models.ToolCallInfo{Name: "read_file", Arguments: map[string]any{"path": "/home/user/.ssh/id_rsa"}},
	}

	result, err := d.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionBlock {
		t.Fatalf("expected block, got %s", result.Action)
	}
}

func TestRuleDetector_DisabledRuleIgnored(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Name: "ban rm", Type: RuleTypeKeyword, Pattern: "rm", Action: models.ActionBlock, Enabled: false},
	}
	d := NewRuleDetector(rules)

	req := &models.InterceptedRequest{
		Method:   models.MethodToolsCall,
		ToolCall: &models.ToolCallInfo{Name: "execute_command", Arguments: map[string]any{"command": "rm file"}},
	}

	result, err := d.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionAllow {
		t.Fatalf("expected allow for disabled rule, got %s", result.Action)
	}
}

func TestRuleDetector_FirstMatchWins(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Name: "warn all", Type: RuleTypeToolName, Pattern: "execute_command", Action: models.ActionWarn, Enabled: true},
		{ID: "r2", Name: "block rm", Type: RuleTypeKeyword, Pattern: "rm", Action: models.ActionBlock, Enabled: true},
	}
	d := NewRuleDetector(rules)

	req := &models.InterceptedRequest{
		Method:   models.MethodToolsCall,
		ToolCall: &models.ToolCallInfo{Name: "execute_command", Arguments: map[string]any{"command": "rm file"}},
	}

	result, err := d.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 第一条规则匹配 tool_name=execute_command，返回 warn，不再检查第二条
	if result.Action != models.ActionWarn {
		t.Fatalf("expected warn (first match), got %s", result.Action)
	}
}

func TestRuleDetector_NonToolsCallReturnsAllow(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Name: "ban rm", Type: RuleTypeKeyword, Pattern: "rm", Action: models.ActionBlock, Enabled: true},
	}
	d := NewRuleDetector(rules)

	req := &models.InterceptedRequest{
		Method: models.MethodInitialize,
	}

	result, err := d.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionAllow {
		t.Fatalf("expected allow for non-tools/call, got %s", result.Action)
	}
}

func TestRuleDetector_MatchAnyStringArgument(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Name: "ban dangerous", Type: RuleTypeKeyword, Pattern: "dangerous", Action: models.ActionBlock, Enabled: true},
	}
	d := NewRuleDetector(rules)

	req := &models.InterceptedRequest{
		Method:   models.MethodToolsCall,
		ToolCall: &models.ToolCallInfo{Name: "some_tool", Arguments: map[string]any{"key1": "safe", "key2": "this is dangerous"}},
	}

	result, err := d.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionBlock {
		t.Fatalf("expected block when any argument matches, got %s", result.Action)
	}
}
