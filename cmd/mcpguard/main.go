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
	"time"

	"github.com/Samantha09/mcpguard/internal/api"
	"github.com/Samantha09/mcpguard/internal/config"
	"github.com/Samantha09/mcpguard/internal/detector"
	"github.com/Samantha09/mcpguard/internal/logger"
	"github.com/Samantha09/mcpguard/internal/models"
	"github.com/Samantha09/mcpguard/internal/platform"
	"github.com/Samantha09/mcpguard/internal/probe"
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
		case "platform":
			if err := runPlatform(); err != nil {
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

func runPlatform() error {
	var (
		dbPath   = flag.String("db", "mcpguard.db", "SQLite 数据库路径")
		listen   = flag.String("listen", ":8080", "监听地址")
		logLevel = flag.String("log-level", "info", "日志级别")
	)
	flag.CommandLine.Parse(os.Args[2:])

	logger.Setup(logger.Config{Level: *logLevel})

	s := store.NewSQLiteStore(*dbPath)
	if err := s.Init(context.Background()); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	defer s.Close()

	plat := platform.NewServer(s)
	fmt.Printf("平台服务启动于 %s\n", *listen)
	return plat.Run(*listen)
}

func runServe() error {
	// CLI flags
	var (
		command      = flag.String("command", "", "MCP Server 启动命令（必填）")
		argsStr      = flag.String("args", "", "MCP Server 命令参数（逗号分隔）")
		dbPath       = flag.String("db", "mcpguard.db", "SQLite 数据库路径")
		apiAddr      = flag.String("api-addr", ":9090", "API 监听地址")
		noAPI        = flag.Bool("no-api", false, "禁用 HTTP API")
		logLevel     = flag.String("log-level", "info", "日志级别: debug/info/warn/error")
		platformAddr = flag.String("platform-addr", "", "平台地址（如 http://localhost:8080）")
		token        = flag.String("token", "", "探针认证 token")
		probeID      = flag.String("probe-id", "", "探针 ID（注册后获得）")
		register     = flag.Bool("register", false, "首次注册模式（获取 token 后退出）")
		probeName    = flag.String("probe-name", "", "探针名称（默认 hostname）")
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
	cfg.Probe.PlatformAddr = *platformAddr
	cfg.Probe.Token = *token
	cfg.Probe.ProbeID = *probeID
	cfg.Probe.ProbeName = *probeName

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

	// 探针客户端
	var probeClient *probe.Client
	if cfg.Probe.PlatformAddr != "" {
		probeClient = probe.NewClient(cfg.Probe.PlatformAddr, cfg.Probe.Token, cfg.Probe.ProbeID)
		if *register {
			name := cfg.Probe.ProbeName
			if name == "" {
				name, _ = os.Hostname()
				if name == "" {
					name = "unnamed"
				}
			}
			_, newToken, err := probeClient.Register(context.Background(), name, "", "")
			if err != nil {
				return fmt.Errorf("探针注册失败: %w", err)
			}
			fmt.Printf("探针注册成功，token: %s\n", newToken)
			return nil
		}
		// 拉取平台规则
		_, _ = probeClient.PullRules(context.Background())
	}

	// 构建检测流水线（内置规则）
	pipeline := detector.NewPipeline(rules.NewRuleDetector(rules.BuiltInRules()))

	// 创建代理
	pxy := proxy.New(cfg.Proxy, pipeline, s, probeClient)

	// 启动 API 服务（异步）
	if cfg.API.Enabled {
		apiSrv := api.NewServer(s)
		go func() {
			if err := apiSrv.Run(cfg.API.Listen); err != nil {
				fmt.Fprintf(os.Stderr, "API 服务错误: %v\n", err)
			}
		}()
	}

	// 启动信号监听
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 启动心跳（如果配置了平台）
	if probeClient != nil {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					_ = probeClient.Heartbeat(ctx)
				}
			}
		}()
	}

	// 启动代理
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
	fmt.Println("  mcpguard serve [flags]     启动 MCPGuard 探针服务")
	fmt.Println("  mcpguard platform [flags]  启动 MCPGuard 平台服务")
	fmt.Println("  mcpguard version           显示版本号")
	fmt.Println()
	fmt.Println("serve 参数:")
	fmt.Println("  --command string       MCP Server 启动命令（必填）")
	fmt.Println("  --args string          命令参数（逗号分隔，如 \"-y,@server\"）")
	fmt.Println("  --db string            SQLite 数据库路径（默认 mcpguard.db）")
	fmt.Println("  --api-addr string      API 监听地址（默认 :9090）")
	fmt.Println("  --no-api               禁用 HTTP API")
	fmt.Println("  --log-level string     日志级别（默认 info）")
	fmt.Println("  --platform-addr string 平台地址（如 http://localhost:8080）")
	fmt.Println("  --token string         探针认证 token")
	fmt.Println("  --register             首次注册模式（获取 token 后退出）")
	fmt.Println("  --probe-name string    探针名称（默认 hostname）")
	fmt.Println()
	fmt.Println("platform 参数:")
	fmt.Println("  --db string            SQLite 数据库路径（默认 mcpguard.db）")
	fmt.Println("  --listen string        监听地址（默认 :8080）")
	fmt.Println("  --log-level string     日志级别（默认 info）")
}
