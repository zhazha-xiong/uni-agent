package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	"uni-agent/internal/agent"
	"uni-agent/internal/config"
	"uni-agent/internal/llm"
	"uni-agent/internal/tool"
	"uni-agent/internal/tool/remote/amap"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

// run 启动 agent
func run() error {
	restore := enterTUIMode("../log/agent-debug.log")
	defer restore()

	if err := godotenv.Load(); err != nil {
		slog.Warn("未找到 .env 文件，使用系统环境变量")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("配置加载失败: %w", err)
	}

	// 注册工具
	tool.Registry(amap.NewCityTool(cfg.AMAP.Key, cfg.AMAP.ConfigURL))
	tool.Registry(amap.NewWeatherTool(cfg.AMAP.Key, cfg.AMAP.WeatherURL))

	// 准备 llm client
	client, err := llm.NewChatClient(
		cfg.LLM.APIKey, cfg.LLM.BaseURL, cfg.LLM.Model,
		cfg.LLM.Reason, cfg.LLM.MaxTokens, cfg.LLM.Temperature,
	)
	if err != nil {
		return fmt.Errorf("初始化 LLM client 失败: %w", err)
	}

	// 准备 agent loop
	loop, err := agent.NewLoop(client, tool.Set)
	if err != nil {
		return fmt.Errorf("初始化 agent loop 失败: %w", err)
	}

	if err := loop.Run(context.Background()); err != nil {
		return fmt.Errorf("agent loop: %w", err)
	}
	return nil
}

// enterTUIMode 进入TUI模式
func enterTUIMode(logPath string) (restore func()) {
	previous := slog.Default()

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		// 打不开文件就静默丢弃，绝不能回落到 stderr
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return func() { slog.SetDefault(previous) }
	}

	logger := slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	return func() {
		slog.SetDefault(previous)
		f.Close()
	}
}
