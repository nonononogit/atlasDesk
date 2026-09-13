package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"atlasdesk/internal/config"
	"atlasdesk/internal/job"
	"atlasdesk/internal/platform/database"
	"atlasdesk/internal/platform/redis"
	"atlasdesk/internal/platform/storage"
	"atlasdesk/internal/repository"
	"atlasdesk/internal/service"
	transporthttp "atlasdesk/internal/transport/http"
	"atlasdesk/internal/transport/http/handler"
	"github.com/hibiken/asynq"
)

func main() {
	// 1. 初始化结构化日志
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("正在启动 AtlasDesk API 服务...")

	// 2. 加载系统全局配置
	cfg := config.Load()

	// 3. 初始化 MinIO 对象存储客户端
	storageClient, storageErr := storage.NewMinIOClient(&cfg.Storage)
	if storageErr != nil {
		slog.Warn("初始化 MinIO 对象存储失败 (预签名直传将使用本地模拟模式)", "error", storageErr.Error())
	}

	// 4. 初始化数据库连接
	var dbClient *database.DB
	var dbErr error
	var authRepo repository.AuthRepository
	var authSvc service.AuthService
	var authHandler *handler.AuthHandler
	var knowledgeRepo repository.KnowledgeRepository
	var knowledgeSvc service.KnowledgeService
	var knowledgeHandler *handler.KnowledgeHandler

	dbClient, dbErr = database.New(&cfg.Database)
	if dbErr != nil {
		slog.Warn("初始化数据库连接失败 (健康检查 ready 端点将正确返回未就绪状态)", "error", dbErr.Error())
	} else {
		slog.Info("数据库连接初始化成功")
		defer dbClient.Close()

		// 自动执行显式版本迁移与幂等种子加载
		initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second)
		migrator := database.NewMigrator(dbClient.SqlDB)
		if err := migrator.Up(initCtx); err != nil {
			slog.Error("自动执行数据库迁移失败", "error", err.Error())
		} else {
			if err := database.SeedIdempotent(initCtx, dbClient.GormDB); err != nil {
				slog.Error("自动执行种子数据填充失败", "error", err.Error())
			}
		}
		initCancel()

		// 组装认证业务仓储与服务实例
		authRepo = repository.NewAuthRepository(dbClient.GormDB)
		authSvc = service.NewAuthService(authRepo, &cfg.JWT)
		authHandler = handler.NewAuthHandler(authSvc)

		// 组装切片仓储与异步任务派发器
		chunkRepo := repository.NewChunkRepository(dbClient.GormDB)
		taskDistributor := job.NewRedisTaskDistributor(asynq.RedisClientOpt{
			Addr:     cfg.Redis.Addr(),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})

		// 组装知识库业务仓储与服务实例
		knowledgeRepo = repository.NewKnowledgeRepository(dbClient.GormDB)
		knowledgeSvc = service.NewKnowledgeService(knowledgeRepo, storageClient, taskDistributor, chunkRepo)
		knowledgeHandler = handler.NewKnowledgeHandler(knowledgeSvc)
	}

	// 5. 初始化 Redis 客户端 (用于健康检查与业务缓存)
	var redisClient *redis.Client
	var redisErr error
	redisClient, redisErr = redis.New(&cfg.Redis)
	if redisErr != nil {
		slog.Warn("初始化 Redis 客户端失败", "error", redisErr.Error())
	} else {
		slog.Info("Redis 客户端初始化成功")
		defer redisClient.Close()
	}

	// 6. 初始化健康检查 Handler
	var dbChecker handler.Checker
	if dbClient != nil {
		dbChecker = dbClient
	}
	var redisChecker handler.Checker
	if redisClient != nil {
		redisChecker = redisClient
	}
	healthHandler := handler.NewHealthHandler(dbChecker, redisChecker)

	// 7. 初始化 HTTP 路由，按规范挂载 9 层中间件与认证、知识库路由
	router := transporthttp.NewRouter(&transporthttp.RouterDeps{
		Config:           cfg,
		HealthHandler:    healthHandler,
		AuthHandler:      authHandler,
		AuthService:      authSvc,
		KnowledgeHandler: knowledgeHandler,
	})

	// 8. 配置 HTTP 服务实例
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 9. 异步启动服务监听
	go func() {
		slog.Info("AtlasDesk API 服务已启动", "port", cfg.App.Port, "env", cfg.App.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("API 服务运行异常终止", "error", err.Error())
			os.Exit(1)
		}
	}()

	// 10. 优雅停机信号监听 (Graceful Shutdown)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("收到终止信号，正在平滑关闭服务...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("服务平滑关闭超时，强制退出", "error", err.Error())
	}

	slog.Info("AtlasDesk API 服务已安全退出")
}
