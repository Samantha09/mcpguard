// LLM 检测 — 可插拔 LLM 后端接口，先实现 OpenAI 兼容格式
package llm

import "context"

// Backend LLM 后端接口（可插拔）
type Backend interface {
	// Complete 发送 prompt 并获取回复
	Complete(ctx context.Context, prompt string) (string, error)
	// Name 后端名称
	Name() string
}

// OpenAIConfig OpenAI 兼容后端配置
type OpenAIConfig struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"` // 默认 https://api.openai.com/v1
	Model   string `json:"model"`    // 默认 gpt-4o
}

// OpenAIBackend OpenAI 兼容后端实现
type OpenAIBackend struct {
	config OpenAIConfig
}

// NewOpenAIBackend 创建 OpenAI 兼容后端
func NewOpenAIBackend(cfg OpenAIConfig) *OpenAIBackend {
	return &OpenAIBackend{config: cfg}
}

func (b *OpenAIBackend) Complete(ctx context.Context, prompt string) (string, error) {
	// TODO: 实现 HTTP 调用
	return "", nil
}

func (b *OpenAIBackend) Name() string {
	return "openai"
}
