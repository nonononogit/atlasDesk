package database_test

import (
	"context"
	"regexp"
	"testing"

	"atlasdesk/internal/platform/database"
	"github.com/DATA-DOG/go-sqlmock"
)

// TestMigrator_UpAndDown 验证迁移管理器 Up 与 Down 的事务管理、版本记录与幂等性
func TestMigrator_UpAndDown(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("初始化 sqlmock 失败: %v", err)
	}
	defer db.Close()

	migrator := database.NewMigrator(db)
	ctx := context.Background()

	// 1. 模拟 Up 阶段：初始化表 -> 检查是否已应用 (未应用) -> 开启事务 -> 执行 SQL -> 插入记录 -> 提交事务
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)")).
		WithArgs("000001_auth_schema").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	mock.ExpectBegin()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS organizations").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO schema_migrations (version) VALUES ($1)")).
		WithArgs("000001_auth_schema").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Migrator.Up 预期执行成功，实际报错: %v", err)
	}

	// 2. 模拟 Up 幂等性：再次运行已存在时跳过
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)")).
		WithArgs("000001_auth_schema").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Migrator.Up 幂等重复执行预期成功，实际报错: %v", err)
	}

	// 3. 模拟 Down 回滚阶段
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version FROM schema_migrations ORDER BY applied_at DESC, version DESC LIMIT $1")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("000001_auth_schema"))

	mock.ExpectBegin()
	mock.ExpectExec("DROP TABLE IF EXISTS refresh_sessions;").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM schema_migrations WHERE version = $1")).
		WithArgs("000001_auth_schema").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := migrator.Down(ctx, 1); err != nil {
		t.Fatalf("Migrator.Down 预期回滚成功，实际报错: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("未满足的 SQL Mock 预期: %v", err)
	}
}
