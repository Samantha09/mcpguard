// MCPGuard — MCP 安全护栏
// 透明中间人代理，双向拦截 Agent 工具调用，检测并拦截不安全操作
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Samantha09/mcpguard/internal/api"
	"github.com/Samantha09/mcpguard/internal/config"
	"github.com/Samantha09/mcpguard/internal/detector"
	"github.com/Samantha09/mcpguard/internal/logger"
	"github.com/Samantha09/mcpguard/internal/models"
	"github.com/Samantha09/mcpguard/internal/proxy"
	"github.com/Samantha09/mcpguard/internal/rules"
	"github.com/Samantha09/mcpguard/internal/store"
)

var version = "0.1.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			if err := runServe(); err != nil {
				fmt.Fprintf(os.Stderr, "错误: %v\n", err)
				os.Exit(1)
			}
		case "version", "--version":
			fmt.Printf("mcpguard v%s\n", version)
		default:
			printUsage()
		}
	} else {
		printUsage()
	}
}

func runServe() error {
	// CLI flags
	var (
		command  = flag.String("command", "", "MCP Server 启动命令（必填）")
		argsStr  = flag.String("args", "", "MCP Server 命令参数（逗号分隔）")
		dbPath   = flag.String("db", "mcpguard.db", "SQLite 数据库路径")
		apiAddr  = flag.String("api-addr", ":9090", "API 监听地址")
		noAPI    = flag.Bool("no-api", false, "禁用 HTTP API")
		logLevel = flag.String("log-level", "info", "日志级别: debug/info/warn/error")
	)
	flag.CommandLine.Parse(os.Args[2:])

	if *command == "" {
		return fmt.Errorf("必须指定 --command 参数")
	}

	// 加载配置
	cfg := config.DefaultConfig()
	cfg.LogLevel = *logLevel
	cfg.DBPath = *dbPath
	cfg.API.Enabled = !*noAPI
	cfg.API.Listen = *apiAddr

	var args []string
	if *argsStr != "" {
		args = strings.Split(*argsStr, ",")
	}
	cfg.Proxy.Servers = []models.ServerConfig{
		{
			Name:      "default",
			Transport: models.TransportStdio,
			Command:   *command,
			Args:      args,
		},
	}

	// 初始化日志
	logger.Setup(logger.Config{Level: cfg.LogLevel})

	// 初始化存储
	s := store.NewSQLiteStore(cfg.DBPath)
	if err := s.Init(context.Background()); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	defer s.Close()

	// 构建检测流水线（内置规则）
	pipeline := detector.NewPipeline(rules.NewRuleDetector(rules.BuiltInRules()))

	// 创建代理
	pxy := proxy.New(cfg.Proxy, pipeline, s)

	// 启动 API 服务（异步）
	if cfg.API.Enabled {
		apiSrv := api.NewServer(s)
		go func() {
			if err := apiSrv.Run(cfg.API.Listen); err != nil {
				fmt.Fprintf(os.Stderr, "API 服务错误: %v\n", err)
			}
		}()
	}

	// 启动代理
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := pxy.Start(ctx); err != nil {
		return fmt.Errorf("代理启动失败: %w", err)
	}
	defer pxy.Stop()

	<-ctx.Done()
	fmt.Println("\n正在关闭...")
	return nil
}

func printUsage() {
	fmt.Printf("mcpguard v%s — MCP 安全护栏\n\n", version)
	fmt.Println("用法:")
	fmt.Println("  mcpguard serve [flags]  启动 MCPGuard 服务")
	fmt.Println("  mcpguard version        显示版本号")
	fmt.Println()
	fmt.Println("serve 参数:")
	fmt.Println("  --command string    MCP Server 启动命令（必填）")
	fmt.Println("  --args string       命令参数（逗号分隔，如 \"-y,@server\"）")
	fmt.Println("  --db string         SQLite 数据库路径（默认 mcpguard.db）")
	fmt.Println("  --api-addr string   API 监听地址（默认 :9090）")
	fmt.Println("  --no-api            禁用 HTTP API")
	fmt.Println("  --log-level string  日志级别（默认 info）")
}
