.PHONY: help dev dev-server dev-web test test-server test-web docker-up docker-down clean

# 默认展示帮助
help:
	@echo "AtlasDesk 常用管理指令:"
	@echo "  make dev          - 同时启动后端 API 与前端开发服务"
	@echo "  make dev-server   - 启动后端 Go API 服务"
	@echo "  make dev-worker   - 启动后端 Go Worker 服务"
	@echo "  make dev-web      - 启动前端 Vite 开发服务器"
	@echo "  make test-server  - 运行后端单元测试与健康检查验证"
	@echo "  make docker-up    - 启动本地 PostgreSQL+pgvector, Redis, MinIO 容器"
	@echo "  make docker-down  - 停止本地基础设施容器"
	@echo "  make clean        - 清理构建产物与临时文件"

# 启动基础设施 (Docker Compose)
docker-up:
	docker compose -f deploy/docker-compose.yml up -d

# 停止基础设施
docker-down:
	docker compose -f deploy/docker-compose.yml down

# 启动后端 API 服务
dev-server:
	cd server && go run cmd/api/main.go

# 启动后端 Worker 服务
dev-worker:
	cd server && go run cmd/worker/main.go

# 启动前端开发服务
dev-web:
	cd web && npm run dev

# 运行后端测试
test-server:
	cd server && go test -v ./...

# 运行所有测试
test: test-server

# 清理临时文件
clean:
	rm -rf web/dist server/tmp
