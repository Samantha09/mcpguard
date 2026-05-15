// 配置管理 — 应用配置加载和管理
package config

import (
	"github.com/Samantha09/mcpguard/internal/llm"
	"github.com/Samantha09/mcpguard/internal/models"
	"github.com/Samantha09/mcpguard/internal/proxy"
)

// AppConfig 应用全局配置
type AppConfig struct {
	// 基础设置
	LogLevel string `json:"log_level"` // debug / info / warn / error
	DBPath   string `json:"db_path"`   // SQLite 数据库路径

	// 代理配置
	Proxy proxy.Config `json:"proxy"`

	// LLM 配置
	LLM LLMConfig `json:"llm"`

	// API 服务配置
	API APIConfig `json:"api"`
}

// LLMConfig LLM 检测配置
type LLMConfig struct {
	Enabled bool            `json:"enabled"`
	Backend string          `json:"backend"` // "openai" | "ollama" | ...
	OpenAI  llm.OpenAIConfig `json:"openai"`
}

// APIConfig HTTP API 配置
type APIConfig struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"` // 如 ":9090"
}

// DefaultConfig 返回默认配置
func DefaultConfig() *AppConfig {
	return &AppConfig{
		LogLevel: "info",
		DBPath:   "mcpguard.db",
		Proxy: proxy.Config{
			Transport: models.TransportStdio,
		},
		LLM: LLMConfig{
			Enabled: false,
			Backend: "openai",
		},
		API: APIConfig{
			Enabled: true,
			Listen:  ":9090",
		},
	}
}

// Load 从文件加载配置
func Load(path string) (*AppConfig, error) {
	// TODO: 读取 JSON 文件并解析
	return DefaultConfig(), nil
}
