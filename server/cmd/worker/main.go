package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"atlasdesk/internal/config"
)

func main() {
	// 1. 初始化结构化日志
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("正在启动 AtlasDesk Asynq Worker 服务...")

	// 2. 加载配置
	cfg := config.Load()
	slog.Info("Worker 配置已加载", "redis_addr", cfg.Redis.Addr(), "env", cfg.App.Env)

	// 3. Worker 骨架循环（将在 Phase 3 接入 Asynq 真实任务调度处理器）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	slog.Info("AtlasDesk Worker 已就绪并等待任务...")
	<-quit

	slog.Info("正在平滑关闭 Worker 服务...")
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	slog.Info("AtlasDesk Worker 服务已安全退出")
}
