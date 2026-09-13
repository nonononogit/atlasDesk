# AtlasDesk 系统架构设计文档

本文档描述 AtlasDesk 智能服务工作台的总体系统架构、分层模型与核心数据流。

## 1. 总体物理架构

```mermaid
flowchart LR
  Browser[Web 客户端] -->|HTTPS / REST / SSE| Nginx[反向代理 (Nginx)]
  Nginx --> Web[前端单页应用 (React+Vite)]
  Nginx --> API[后端 API (Go+Gin)]
  API --> PG[(PostgreSQL 16 + pgvector)]
  API --> Redis[(Redis 7 缓存与 Asynq)]
  API --> Store[(MinIO S3 对象存储)]
  API --> LLM[OpenAI 兼容 LLM / Embedding]
  Redis --> Worker[异步任务 Worker (Go+Asynq)]
  Worker --> PG
  Worker --> Store
  Worker --> LLM
```

## 2. 后端分层架构原则

后端 Go 服务严格遵循以下分层调用约束（详见 `AGENT_IMPLEMENTATION_SPEC.md` 第 3 节）：

```
transport/http → application → domain
                         ↓
                   repository interface
(repository implementation / platform / service 由 main 入口层注入)
```

- **transport/http**：负责 HTTP 请求解析、参数验证、统一响应封装，不写 SQL，不处理业务状态转移。
- **application**：编排用例流，调度领域模型与外部服务。
- **domain**：定义实体、聚合根、领域事件、状态转移规则（如工单状态机、文档状态机）。
- **repository**：数据持久化接口与 GORM/pgx 实现，强制显式接收 `organization_id` 进行多租户过滤。
- **platform**：外部基础设施适配（数据库连接池、Redis 客户端、MinIO 存储客户端、LLM SDK）。
- **job**：Asynq 异步任务定义与处理逻辑（文档解析切片、工单智能打标等）。

## 3. 多租户与安全约束

1. **租户隔离**：所有业务表结构均带有 `organization_id`。所有 Repository 查询先按租户 ID 过滤，杜绝越权。
2. **审计与合规**：所有关键状态变更均落库 `audit_logs`，敏感配置（如 AI API Key）应用级加密存储且接口返回掩码。
3. **分权 RBAC**：基于固定的四角色设计（超级管理员、知识管理员、客服主管、客服坐席），由后端中间件进行权限终审。
