package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"

	"uni-agent/internal/config"
	"uni-agent/internal/tool"
	"uni-agent/internal/tool/remote/amap"
)

func main() {
	// 加载配置
	if err := godotenv.Load(); err != nil {
		slog.Warn("未找到 .env 文件，使用系统环境变量")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("配置加载失败", "err", err)
		os.Exit(1)
	}

	// 注册工具
	tool.Registry(amap.NewCityTool(cfg.AMAP.Key, cfg.AMAP.ConfigURL))

	slog.Info("配置加载完成",
		"llm_base_url", cfg.LLM.BaseURL,
		"amap_configured", cfg.AMAP.Key != "",
	)
}
