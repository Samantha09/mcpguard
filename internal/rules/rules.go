// 规则定义 — 文件路径、命令关键词、正则等匹配规则
package rules

import (
	"context"

	"github.com/Samantha09/mcpguard/internal/detector"
	"github.com/Samantha09/mcpguard/internal/models"
)

// RuleType 规则类型
type RuleType string

const (
	RuleTypeFilePath   RuleType = "file_path"   // 文件路径匹配
	RuleTypeKeyword    RuleType = "keyword"     // 命令关键词匹配
	RuleTypeRegex      RuleType = "regex"       // 正则表达式匹配
	RuleTypeToolName   RuleType = "tool_name"   // 工具名匹配
)

// Rule 规则定义
type Rule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        RuleType `json:"type"`
	Pattern     string   `json:"pattern"`     // 匹配模式（路径、关键词或正则）
	Action      models.Action `json:"action"` // 匹配时的动作
	Enabled     bool     `json:"enabled"`
	Description string   `json:"description"`
}

// RuleDetector 基于规则的检测器，实现 Detector 接口
type RuleDetector struct {
	rules []Rule
}

// NewRuleDetector 创建规则检测器
func NewRuleDetector(rules []Rule) *RuleDetector {
	return &RuleDetector{rules: rules}
}

// LoadFromFile 从 JSON 文件加载规则
func LoadFromFile(path string) ([]Rule, error) {
	// TODO: 实现文件加载
	return nil, nil
}

func (d *RuleDetector) Detect(ctx context.Context, req *models.InterceptedRequest) (*models.DetectResult, error) {
	// TODO: 遍历规则匹配
	return &models.DetectResult{Action: models.ActionAllow}, nil
}

func (d *RuleDetector) Name() string {
	return "rule"
}

// 编译时检查接口实现
var _ detector.Detector = (*RuleDetector)(nil)
