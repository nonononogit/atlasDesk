package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"atlasdesk/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Pinger 定义通用的健康检查接口
type Pinger interface {
	PingContext(ctx context.Context) error
}

// DB 封装 GORM 实例与底层 sql.DB
type DB struct {
	GormDB *gorm.DB
	SqlDB  *sql.DB
}

// New 初始化 PostgreSQL 数据库连接池
func New(cfg *config.DatabaseConfig) (*DB, error) {
	// 配置 GORM 日志级别
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	gormDB, err := gorm.Open(postgres.Open(cfg.DSN()), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 sql.DB 失败: %w", err)
	}

	// 连接池参数设置
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &DB{
		GormDB: gormDB,
		SqlDB:  sqlDB,
	}, nil
}

// PingContext 检查数据库健康连通性
func (d *DB) PingContext(ctx context.Context) error {
	if d == nil || d.SqlDB == nil {
		return fmt.Errorf("数据库连接未就绪")
	}
	return d.SqlDB.PingContext(ctx)
}

// Close 安全关闭数据库连接
func (d *DB) Close() error {
	if d != nil && d.SqlDB != nil {
		return d.SqlDB.Close()
	}
	return nil
}
