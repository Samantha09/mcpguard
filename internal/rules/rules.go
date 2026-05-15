// 规则定义 — 文件路径、命令关键词、正则等匹配规则
package rules

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Samantha09/mcpguard/internal/detector"
	"github.com/Samantha09/mcpguard/internal/models"
)

// RuleType 规则类型
type RuleType string

const (
	RuleTypeFilePath RuleType = "file_path" // 文件路径匹配
	RuleTypeKeyword  RuleType = "keyword"   // 命令关键词匹配
	RuleTypeRegex    RuleType = "regex"     // 正则表达式匹配
	RuleTypeToolName RuleType = "tool_name" // 工具名匹配
)

// Rule 规则定义
type Rule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        RuleType      `json:"type"`
	Pattern     string        `json:"pattern"` // 匹配模式（路径、关键词或正则）
	Action      models.Action `json:"action"`  // 匹配时的动作
	Enabled     bool          `json:"enabled"`
	Description string        `json:"description"`
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
	if req.Method != models.MethodToolsCall || req.ToolCall == nil {
		return &models.DetectResult{Action: models.ActionAllow}, nil
	}

	for _, rule := range d.rules {
		if !rule.Enabled {
			continue
		}

		matched := false
		switch rule.Type {
		case RuleTypeToolName:
			matched = strings.EqualFold(req.ToolCall.Name, rule.Pattern)
		case RuleTypeKeyword:
			matched = matchKeyword(req.ToolCall.Arguments, rule.Pattern)
		case RuleTypeRegex:
			matched = matchRegex(req.ToolCall.Arguments, rule.Pattern)
		}

		if matched {
			return &models.DetectResult{
				Action:   rule.Action,
				Reason:   fmt.Sprintf("规则匹配: %s (%s)", rule.Name, rule.Pattern),
				Detector: d.Name(),
			}, nil
		}
	}

	return &models.DetectResult{Action: models.ActionAllow}, nil
}

// matchStringArg 遍历参数中所有字符串值，任一满足 fn 则返回 true
func matchStringArg(args map[string]any, fn func(string) bool) bool {
	for _, v := range args {
		if s, ok := v.(string); ok {
			if fn(s) {
				return true
			}
		}
	}
	return false
}

// matchKeyword 在参数的所有字符串值中搜索关键词
func matchKeyword(args map[string]any, pattern string) bool {
	return matchStringArg(args, func(s string) bool {
		return strings.Contains(s, pattern)
	})
}

// matchRegex 在参数的所有字符串值中执行正则匹配
func matchRegex(args map[string]any, pattern string) bool {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return matchStringArg(args, re.MatchString)
}

func (d *RuleDetector) Name() string {
	return "rule"
}

// 编译时检查接口实现
var _ detector.Detector = (*RuleDetector)(nil)
