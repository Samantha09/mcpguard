// 检测引擎 — 定义检测器接口，规则检测和 LLM 检测统一抽象
package detector

import (
	"context"

	"github.com/Samantha09/mcpguard/internal/models"
)

// Detector 检测器接口
type Detector interface {
	// Detect 对拦截到的请求执行检测
	Detect(ctx context.Context, req *models.InterceptedRequest) (*models.DetectResult, error)
	// Name 检测器名称
	Name() string
}

// Pipeline 检测流水线，串联多个 Detector
type Pipeline struct {
	detectors []Detector
}

// NewPipeline 创建检测流水线
func NewPipeline(detectors ...Detector) *Pipeline {
	return &Pipeline{detectors: detectors}
}

// AddDetector 添加检测器
func (p *Pipeline) AddDetector(d Detector) {
	p.detectors = append(p.detectors, d)
}

// Run 对请求依次执行所有检测器，返回最严重的 Action（Block > Warn > Allow）
func (p *Pipeline) Run(ctx context.Context, req *models.InterceptedRequest) (*models.DetectResult, error) {
	var final *models.DetectResult
	for _, d := range p.detectors {
		result, err := d.Detect(ctx, req)
		if err != nil {
			continue
		}
		if result == nil {
			continue
		}
		if final == nil || severity(result.Action) > severity(final.Action) {
			final = result
		}
		if result.Action == models.ActionBlock {
			break
		}
	}
	if final == nil {
		return &models.DetectResult{Action: models.ActionAllow}, nil
	}
	return final, nil
}

// severity 返回 Action 的严重程度等级
func severity(a models.Action) int {
	switch a {
	case models.ActionBlock:
		return 3
	case models.ActionWarn:
		return 2
	case models.ActionAllow:
		return 1
	default:
		return 0
	}
}
