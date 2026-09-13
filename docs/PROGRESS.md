# AtlasDesk Development Progress

## 当前阶段
- Phase: P2
- 当前任务: P2-T04
- 状态: completed

## 已完成
- [x] P0-T01 — 2026-09-13 — 验证：环境审计完成，Git 初始化完成，环境与规格差异明确
- [x] P0-T02 — 2026-09-13 — 验证：目录结构符合第 3 节规范，前后端最小骨架就绪，.env.example 无密钥泄露，Makefile 与 README 完备
- [x] P0-T03 — 2026-09-13 — 验证：deploy/docker-compose.yml 配置完成，覆盖 PostgreSQL+pgvector、Redis、MinIO、持久卷与健康探测
- [x] P0-T04 — 2026-09-13 — 验证：实现 /health/live 与 /health/ready，通过单元测试与实际服务 curl 对抗式验证（DB 不可用时 ready 报 503 且 live 正常为 200，Redis 状态准确上报）
- [x] P1-T01 — 2026-09-13 — 验证：创建 000001_auth_schema 向上与向下 SQL 迁移、嵌入式 Migrator 与幂等 Seed 逻辑，单测验证事务执行、幂等跳过与回滚策略通过
- [x] P1-T02 — 2026-09-13 — 验证：实现 login、refresh、logout、me 接口与服务，单测验证登录成功、密码错误拦截、停用用户拦截、令牌过期失效、注销后令牌废弃通过
- [x] P1-T03 — 2026-09-13 — 验证：实现 RequestID、Recovery、CORS、SecurityHeaders、AccessLog、RateLimit、Authentication、OrganizationContext、RequirePermission 中间件，单测覆盖 401/403/越权拦截通过
- [x] P1-T04 — 2026-09-13 — 验证：实现企业级 LoginPage、AppLayout、ProtectedRoute、Zustand authStore 与原生 fetch 客户端（内存令牌+排队无感刷新），并通过 TypeScript 严格类型校验
- [x] P2-T01 — 2026-09-13 — 验证：创建 000002_knowledge_schema 迁移与知识库 CRUD 接口，单测验证同组织同名拒绝、跨组织同名允许及软删除通过
- [x] P2-T02 — 2026-09-13 — 验证：创建 documents 与 document_versions 模型，实现多租户游标分页、按状态/MIME 筛选与关键字搜索
- [x] P2-T03 — 2026-09-13 — 验证：实现预签名直传凭证申请，严格校验 PDF/DOCX/MD/TXT 白名单与 50MB 大小配额，单测拦截非法格式与超限文件通过
- [x] P2-T04 — 2026-09-13 — 验证：实现确认上传核验、文档状态机流转 (UPLOADING -> UPLOADED)、重复确认幂等控制与前端 3 秒批量轮询机制

## 待完成
- [ ] Phase 3：解析、切片和向量（P3-T01 Worker 和状态）— 阻塞：无

## 技术决策
- ADR-001: 依赖注入与分层设计：遵循第 3 节规范，transport/http -> application -> domain，repository interface 位于 domain，实现置于 repository/platform。
- ADR-002: 环境隔离与降级验证：针对宿主机未安装 Docker 的现状，采用标准化 docker-compose.yml 结合 Go 单元测试 Mock 注入与本地 Redis 服务，保证开发测试闭环。
- ADR-003: 健壮就绪探针：/health/ready 设置 2 秒超时，细粒度上报 components.database 与 components.redis 状态；任一组件故障返回 HTTP 503 与 SERVICE_NOT_READY，而 /health/live 始终维持 200。
- ADR-004: 显式 SQL 迁移与双向幂等种子：使用显式 .sql 迁移文件定义 Schema，编写迁移执行器并记录 schema_migrations 表；系统预置 4 角色与 10 大核心权限，种子执行幂等（存在则更新或跳过）。
- ADR-005: 安全令牌设计：短时 JWT Access Token 存放内存；Refresh Token 采用 32 字节高熵随机串并哈希存储，结合 HttpOnly Cookie 传输与强制会话轮换（Token Rotation）。
- ADR-006: 严格中间件管道链：中间件严格遵循 9 层安全与上下文流转，组织隔离由 OrganizationContext 强化，业务接口通过 RequirePermission 声明式校验。
- ADR-007: 前端认证安全与无感排队：前端 Access Token 仅保存在内存变量中，防止 XSS 窃取；401 时启动单次并发锁排队调用 /api/v1/auth/refresh 换取新令牌并重放等待队列；刷新受保护路由时使用全局加载守卫，杜绝未授权界面闪现。
- ADR-008: 预签名直传与真实进度：规避大文件经过应用服务器带来的带宽瓶颈与网络超时风险，前端通过后端签发的预签名 PUT URL 采用原生 XHR 直传 MinIO/S3，获取真实上传进度；确认上传接口校验对象存储实际元数据并实现幂等保护。

## 已知问题
- ISSUE-001: 宿主机未检测到 Docker 环境，容器编排需在容器运行时就绪或独立测试环境中使用。
