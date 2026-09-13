package redis

import (
	"context"
	"fmt"
	"time"

	"atlasdesk/internal/config"
	goredis "github.com/redis/go-redis/v9"
)

// Client 封装 Redis 客户端
type Client struct {
	Rdb *goredis.Client
}

// New 初始化 Redis 客户端连接
func New(cfg *config.RedisConfig) (*Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	return &Client{Rdb: rdb}, nil
}

// PingContext 检查 Redis 连通性
func (c *Client) PingContext(ctx context.Context) error {
	if c == nil || c.Rdb == nil {
		return fmt.Errorf("redis 客户端未就绪")
	}
	return c.Rdb.Ping(ctx).Err()
}

// Close 安全关闭 Redis 客户端
func (c *Client) Close() error {
	if c != nil && c.Rdb != nil {
		return c.Rdb.Close()
	}
	return nil
}
