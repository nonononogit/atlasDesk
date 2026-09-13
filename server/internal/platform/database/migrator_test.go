package database_test

import (
	"context"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	"atlasdesk/internal/platform/database"
	"atlasdesk/migrations"
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

	// 读取当前所有的 up 文件
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatalf("读取迁移文件失败: %v", err)
	}

	var upVersions []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upVersions = append(upVersions, strings.TrimSuffix(entry.Name(), ".up.sql"))
		}
	}
	sort.Strings(upVersions)

	// 1. 模拟 Up 阶段：依次执行所有未应用的迁移
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	for _, v := range upVersions {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)")).
			WithArgs(v).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		mock.ExpectBegin()
		mock.ExpectExec("CREATE TABLE IF NOT EXISTS").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO schema_migrations (version) VALUES ($1)")).
			WithArgs(v).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
	}

	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Migrator.Up 预期执行成功，实际报错: %v", err)
	}

	// 2. 模拟 Up 幂等性：全部已存在时直接跳过
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	for _, v := range upVersions {
		mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)")).
			WithArgs(v).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	}

	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Migrator.Up 幂等重复执行预期成功，实际报错: %v", err)
	}

	// 3. 模拟 Down 回滚阶段（回滚最新一个版本）
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS schema_migrations")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	latestVersion := upVersions[len(upVersions)-1]
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version FROM schema_migrations ORDER BY applied_at DESC, version DESC LIMIT $1")).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(latestVersion))

	mock.ExpectBegin()
	mock.ExpectExec("DROP TABLE IF EXISTS").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM schema_migrations WHERE version = $1")).
		WithArgs(latestVersion).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := migrator.Down(ctx, 1); err != nil {
		t.Fatalf("Migrator.Down 预期回滚成功，实际报错: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("未满足的 SQL Mock 预期: %v", err)
	}
}
