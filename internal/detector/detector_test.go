// 检测引擎测试 — Pipeline 执行顺序与结果聚合
package detector

import (
	"context"
	"errors"
	"testing"

	"github.com/Samantha09/mcpguard/internal/models"
)

// mockDetector 测试用的模拟检测器
type mockDetector struct {
	name   string
	result *models.DetectResult
	err    error
}

func (m *mockDetector) Detect(ctx context.Context, req *models.InterceptedRequest) (*models.DetectResult, error) {
	return m.result, m.err
}

func (m *mockDetector) Name() string {
	return m.name
}

func TestPipeline_EmptyReturnsAllow(t *testing.T) {
	p := NewPipeline()
	req := &models.InterceptedRequest{Method: models.MethodToolsCall}

	result, err := p.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionAllow {
		t.Fatalf("expected allow, got %s", result.Action)
	}
}

func TestPipeline_SingleAllow(t *testing.T) {
	d := &mockDetector{name: "allow", result: &models.DetectResult{Action: models.ActionAllow}}
	p := NewPipeline(d)

	result, err := p.Run(context.Background(), &models.InterceptedRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionAllow {
		t.Fatalf("expected allow, got %s", result.Action)
	}
}

func TestPipeline_SingleBlock(t *testing.T) {
	d := &mockDetector{name: "blocker", result: &models.DetectResult{Action: models.ActionBlock, Reason: "blocked"}}
	p := NewPipeline(d)

	result, err := p.Run(context.Background(), &models.InterceptedRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionBlock {
		t.Fatalf("expected block, got %s", result.Action)
	}
	if result.Reason != "blocked" {
		t.Fatalf("expected reason 'blocked', got %s", result.Reason)
	}
}

func TestPipeline_SingleWarn(t *testing.T) {
	d := &mockDetector{name: "warner", result: &models.DetectResult{Action: models.ActionWarn, Reason: "warned"}}
	p := NewPipeline(d)

	result, err := p.Run(context.Background(), &models.InterceptedRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionWarn {
		t.Fatalf("expected warn, got %s", result.Action)
	}
}

func TestPipeline_BlockShortCircuits(t *testing.T) {
	allow := &mockDetector{name: "allow", result: &models.DetectResult{Action: models.ActionAllow}}
	block := &mockDetector{name: "block", result: &models.DetectResult{Action: models.ActionBlock, Reason: "blocked"}}
	// 第三个检测器不应该被调用
	neverCalled := &mockDetector{name: "never", result: &models.DetectResult{Action: models.ActionAllow}}

	p := NewPipeline(allow, block, neverCalled)

	result, err := p.Run(context.Background(), &models.InterceptedRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionBlock {
		t.Fatalf("expected block, got %s", result.Action)
	}
}

func TestPipeline_AllowWarnReturnsWarn(t *testing.T) {
	allow := &mockDetector{name: "allow", result: &models.DetectResult{Action: models.ActionAllow}}
	warn := &mockDetector{name: "warn", result: &models.DetectResult{Action: models.ActionWarn, Reason: "warned"}}

	p := NewPipeline(allow, warn)

	result, err := p.Run(context.Background(), &models.InterceptedRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionWarn {
		t.Fatalf("expected warn, got %s", result.Action)
	}
}

func TestPipeline_WarnThenBlockReturnsBlock(t *testing.T) {
	warn := &mockDetector{name: "warn", result: &models.DetectResult{Action: models.ActionWarn, Reason: "warned"}}
	block := &mockDetector{name: "block", result: &models.DetectResult{Action: models.ActionBlock, Reason: "blocked"}}

	p := NewPipeline(warn, block)

	result, err := p.Run(context.Background(), &models.InterceptedRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionBlock {
		t.Fatalf("expected block, got %s", result.Action)
	}
}

func TestPipeline_Severities_BlockGtWarnGtAllow(t *testing.T) {
	cases := []struct {
		name     string
		actions  []models.Action
		expected models.Action
	}{
		{"block_over_warn", []models.Action{models.ActionWarn, models.ActionBlock}, models.ActionBlock},
		{"block_over_allow", []models.Action{models.ActionAllow, models.ActionBlock}, models.ActionBlock},
		{"warn_over_allow", []models.Action{models.ActionAllow, models.ActionWarn}, models.ActionWarn},
		{"allow_only", []models.Action{models.ActionAllow, models.ActionAllow}, models.ActionAllow},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			detectors := make([]Detector, len(tc.actions))
			for i, a := range tc.actions {
				detectors[i] = &mockDetector{name: string(a), result: &models.DetectResult{Action: a}}
			}
			p := NewPipeline(detectors...)

			result, err := p.Run(context.Background(), &models.InterceptedRequest{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Action != tc.expected {
				t.Fatalf("expected %s, got %s", tc.expected, result.Action)
			}
		})
	}
}

func TestPipeline_DetectorErrorContinues(t *testing.T) {
	broken := &mockDetector{name: "broken", err: errors.New("boom")}
	block := &mockDetector{name: "block", result: &models.DetectResult{Action: models.ActionBlock, Reason: "blocked"}}

	p := NewPipeline(broken, block)

	result, err := p.Run(context.Background(), &models.InterceptedRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionBlock {
		t.Fatalf("expected block after skipping broken detector, got %s", result.Action)
	}
}

func TestPipeline_AllDetectorsErrorReturnsAllow(t *testing.T) {
	broken1 := &mockDetector{name: "broken1", err: errors.New("boom1")}
	broken2 := &mockDetector{name: "broken2", err: errors.New("boom2")}

	p := NewPipeline(broken1, broken2)

	result, err := p.Run(context.Background(), &models.InterceptedRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != models.ActionAllow {
		t.Fatalf("expected allow when all detectors fail, got %s", result.Action)
	}
}
