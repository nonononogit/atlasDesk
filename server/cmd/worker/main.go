package main

import (
	"context"
	"log/slog"
	"os"

	"atlasdesk/internal/config"
	"atlasdesk/internal/domain"
	"atlasdesk/internal/job"
	"atlasdesk/internal/platform/database"
	"atlasdesk/internal/platform/embedding"
	"atlasdesk/internal/platform/storage"
	"atlasdesk/internal/repository"
	"github.com/hibiken/asynq"
)

func main() {
	// 1. 初始化结构化日志
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("正在启动 AtlasDesk 异步任务 Worker 服务...")

	// 2. 加载配置
	cfg := config.Load()

	// 3. 初始化数据库连接
	db, err := database.New(&cfg.Database)
	if err != nil {
		slog.Warn("数据库连接失败，使用离线模式启动 Worker", "error", err)
	}

	// 4. 初始化存储客户端
	storageClient, err := storage.NewMinIOClient(&cfg.Storage)
	if err != nil {
		slog.Warn("MinIO 对象存储未就绪，使用离线兼容模式", "error", err)
	}

	// 5. 初始化仓储层
	var docRepo repository.KnowledgeRepository
	var chunkRepo repository.ChunkRepository
	if db != nil && db.GormDB != nil {
		docRepo = repository.NewKnowledgeRepository(db.GormDB)
		chunkRepo = repository.NewChunkRepository(db.GormDB)
	}

	// 6. 初始化 Embedder (默认采用 1536 维归一化 Embedder)
	embedder := embedding.NewMockEmbedder()

	// 7. 初始化任务处理器
	processor := job.NewTaskProcessor(
		docRepo,
		chunkRepo,
		storageClient,
		embedder,
		domain.ChunkOptions{TargetTokens: 500, OverlapTokens: 80},
	)

	// 8. 配置 Asynq Server 并监听队列
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 10,
			Queues: map[string]int{
				"default": 1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				slog.Error("Asynq 任务处理抛出异常", "type", task.Type(), "error", err)
			}),
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(job.TypeDocumentProcess, processor.ProcessDocumentTask)

	slog.Info("AtlasDesk Worker 开始监听任务队列...", "redis", cfg.Redis.Addr())
	if err := srv.Run(mux); err != nil {
		slog.Error("Worker 运行异常退出", "error", err)
		os.Exit(1)
	}
}
