package job

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"atlasdesk/internal/domain"
	"github.com/hibiken/asynq"
)

// 任务类型常量定义 (符合规格 5.1 节规范)
const (
	TypeDocumentProcess = "document:process"
)

// TaskDistributor 异步任务分发器接口
type TaskDistributor interface {
	DistributeDocumentProcess(ctx context.Context, payload *domain.DocumentProcessPayload, opts ...asynq.Option) error
}

// RedisTaskDistributor 基于 Asynq 的 Redis 任务派发器实现
type RedisTaskDistributor struct {
	client *asynq.Client
}

// NewRedisTaskDistributor 构造 Asynq 任务派发客户端
func NewRedisTaskDistributor(redisOpt asynq.RedisConnOpt) *RedisTaskDistributor {
	client := asynq.NewClient(redisOpt)
	return &RedisTaskDistributor{client: client}
}

// DistributeDocumentProcess 将文档处理异步任务推入队列
// 默认策略：最大重试 3 次，超时 10 分钟
func (d *RedisTaskDistributor) DistributeDocumentProcess(ctx context.Context, payload *domain.DocumentProcessPayload, opts ...asynq.Option) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化任务负载失败: %w", err)
	}

	// 创建任务实例
	task := asynq.NewTask(TypeDocumentProcess, jsonPayload, opts...)

	// 默认重试与超时控制
	defaultOpts := []asynq.Option{
		asynq.MaxRetry(3),
		asynq.Timeout(10 * time.Minute),
		asynq.Retention(24 * time.Hour), // 任务完成后保留记录供审计
	}

	info, err := d.client.EnqueueContext(ctx, task, defaultOpts...)
	if err != nil {
		return fmt.Errorf("投递异步任务 [%s] 失败: %w", TypeDocumentProcess, err)
	}

	_ = info
	return nil
}
