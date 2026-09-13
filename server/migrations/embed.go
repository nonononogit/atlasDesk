package migrations

import "embed"

// Files 嵌入全量 SQL 迁移文件，随二进制一同分发
//
//go:embed *.sql
var Files embed.FS
