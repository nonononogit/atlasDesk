package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"atlasdesk/migrations"
)

// Migrator 负责管理与执行显式 SQL 数据库版本迁移
type Migrator struct {
	db *sql.DB
}

// NewMigrator 构造迁移执行器
func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

// InitSchemaTable 创建迁移版本记录表 schema_migrations (如果不存在)
func (m *Migrator) InitSchemaTable(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := m.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("创建迁移历史表 schema_migrations 失败: %w", err)
	}
	return nil
}

// Up 执行所有未应用的向上迁移文件 (*.up.sql)
func (m *Migrator) Up(ctx context.Context) error {
	if err := m.InitSchemaTable(ctx); err != nil {
		return err
	}

	// 1. 读取嵌入的迁移文件清单
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		return fmt.Errorf("读取迁移文件系统失败: %w", err)
	}

	var upFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upFiles = append(upFiles, entry.Name())
		}
	}
	sort.Strings(upFiles)

	// 2. 逐一检查并应用未执行的迁移
	for _, filename := range upFiles {
		version := strings.TrimSuffix(filename, ".up.sql")

		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`
		if err := m.db.QueryRowContext(ctx, checkQuery, version).Scan(&exists); err != nil {
			return fmt.Errorf("检查迁移状态失败 (%s): %w", version, err)
		}

		if exists {
			slog.Debug("迁移已应用，跳过", "version", version)
			continue
		}

		slog.Info("正在执行数据库向上迁移", "file", filename)
		content, err := fs.ReadFile(migrations.Files, filename)
		if err != nil {
			return fmt.Errorf("读取迁移内容失败 (%s): %w", filename, err)
		}

		// 在事务中执行迁移与版本记录
		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("开启事务失败: %w", err)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("执行迁移 SQL 失败 (%s): %w", filename, err)
		}

		recordQuery := `INSERT INTO schema_migrations (version) VALUES ($1)`
		if _, err := tx.ExecContext(ctx, recordQuery, version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("记录迁移版本失败 (%s): %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交迁移事务失败: %w", err)
		}

		slog.Info("数据库向上迁移执行成功", "version", version)
	}

	return nil
}

// Down 回滚最新应用的一个或多个迁移版本 (*.down.sql)
func (m *Migrator) Down(ctx context.Context, steps int) error {
	if err := m.InitSchemaTable(ctx); err != nil {
		return err
	}

	// 1. 查询已应用的最新版本列表
	rows, err := m.db.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY applied_at DESC, version DESC LIMIT $1`, steps)
	if err != nil {
		return fmt.Errorf("查询已应用版本失败: %w", err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return err
		}
		versions = append(versions, v)
	}

	// 2. 依次执行回滚脚本
	for _, version := range versions {
		downFilename := version + ".down.sql"
		slog.Info("正在执行数据库回滚", "file", downFilename)

		content, err := fs.ReadFile(migrations.Files, downFilename)
		if err != nil {
			return fmt.Errorf("读取回滚文件失败 (%s): %w", downFilename, err)
		}

		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("执行回滚 SQL 失败 (%s): %w", downFilename, err)
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("删除迁移记录失败 (%s): %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		slog.Info("数据库版本回滚成功", "version", version)
	}

	return nil
}
