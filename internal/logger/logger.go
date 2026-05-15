// 日志封装 — 基于 log/slog 的结构化日志
package logger

import (
	"log/slog"
	"os"
)

// Config 日志配置
type Config struct {
	Level string `json:"level"` // debug / info / warn / error
	JSON  bool   `json:"json"`  // 是否 JSON 格式输出
}

// Setup 初始化全局 logger
func Setup(cfg Config) {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.JSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
