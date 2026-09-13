# AtlasDesk API 清单

本文件固化 AtlasDesk 项目全量核心 API 契约（详见 `docs/AGENT_IMPLEMENTATION_SPEC.md` 第 13 节）。

## 1. 基础与健康检查
- `GET /health/live` - 存活探针
- `GET /health/ready` - 就绪探针 (检查 DB 与 Redis 连通性)

## 2. 认证与用户
- `POST /api/v1/auth/login` - 账号密码登录
- `POST /api/v1/auth/refresh` - 刷新 Access Token
- `POST /api/v1/auth/logout` - 注销登录
- `GET /api/v1/auth/me` - 获取当前登录用户信息与权限

## 3. 工作台与全局
- `GET /api/v1/dashboard/summary` - 工作台汇总指标
- `GET /api/v1/dashboard/trends` - 趋势分析
- `GET /api/v1/dashboard/top-knowledge` - 热门知识榜
- `GET /api/v1/dashboard/activities` - 最近活动流水
- `GET /api/v1/dashboard/sla-health` - SLA 健康状况
- `GET /api/v1/search` - 全局综合检索
- `GET /api/v1/notifications` - 通知列表
- `POST /api/v1/notifications/:id/read` - 标记通知已读
- `POST /api/v1/notifications/read-all` - 全部标记已读

## 4. 知识库与文档
- `GET /api/v1/knowledge-bases` - 知识库列表
- `POST /api/v1/knowledge-bases` - 创建知识库
- `GET /api/v1/knowledge-bases/:id` - 知识库详情
- `PATCH /api/v1/knowledge-bases/:id` - 更新知识库
- `DELETE /api/v1/knowledge-bases/:id` - 删除知识库
- `GET /api/v1/documents` - 文档列表
- `POST /api/v1/documents/uploads` - 请求预签名上传凭证
- `POST /api/v1/documents/:id/complete-upload` - 完成并确认上传
- `GET /api/v1/documents/:id` - 文档详情
- `GET /api/v1/documents/:id/status` - 文档解析与向量化状态
- `PATCH /api/v1/documents/:id` - 更新文档属性
- `POST /api/v1/documents/:id/reprocess` - 重新触发解析切片
- `POST /api/v1/documents/:id/versions` - 上传新版本
- `GET /api/v1/documents/:id/download-url` - 获取下载预签名链接
- `DELETE /api/v1/documents/:id` - 软删除文档

## 5. RAG 会话与消息
- `GET /api/v1/conversations` - 会话历史列表
- `POST /api/v1/conversations` - 新建会话
- `GET /api/v1/conversations/:id` - 会话详情
- `DELETE /api/v1/conversations/:id` - 删除会话
- `GET /api/v1/conversations/:id/messages` - 获取消息列表
- `POST /api/v1/conversations/:id/messages/stream` - 发送消息并流式响应 (SSE)
- `POST /api/v1/messages/:id/feedback` - 消息满意度反馈
- `POST /api/v1/conversations/:id/exports` - 导出对话记录

## 6. 工单流转
- `GET /api/v1/tickets` - 工单列表
- `POST /api/v1/tickets` - 新建工单
- `GET /api/v1/tickets/:id` - 工单详情
- `PATCH /api/v1/tickets/:id` - 更新工单信息
- `POST /api/v1/tickets/:id/assign` - 分配工单处理人
- `POST /api/v1/tickets/:id/transitions` - 状态流转推进
- `GET /api/v1/tickets/:id/comments` - 获取工单评论与处理记录
- `POST /api/v1/tickets/:id/comments` - 添加工单评论
- `GET /api/v1/tickets/:id/events` - 工单流转审计轨迹
- `POST /api/v1/ticket-exports` - 导出工单数据

## 7. 组织、成员与权限
- `GET /api/v1/members` - 成员列表
- `GET /api/v1/members/summary` - 成员概况汇总
- `PATCH /api/v1/members/:id/role` - 修改成员角色
- `POST /api/v1/members/:id/suspend` - 停用成员账号
- `POST /api/v1/members/:id/restore` - 恢复成员账号
- `DELETE /api/v1/members/:id` - 移除成员
- `GET /api/v1/roles` - 系统与自定义角色列表
- `POST /api/v1/roles` - 新增自定义角色
- `PATCH /api/v1/roles/:id` - 更新角色与权限映射
- `DELETE /api/v1/roles/:id` - 删除自定义角色
- `POST /api/v1/invitations` - 发送团队邀请
- `POST /api/v1/invitations/:token/accept` - 接受邀请

## 8. AI 配置与系统审计
- `GET /api/v1/ai-config` - 获取当前模型与检索配置 (密钥掩码)
- `PATCH /api/v1/ai-config` - 更新 AI 配置 (生成新版本)
- `POST /api/v1/ai-config/test` - 连通性测试 (防 SSRF)
- `GET /api/v1/ai-config/versions` - 配置历史版本
- `GET /api/v1/ai-usage` - AI Token 用量统计
- `GET /api/v1/audit-logs` - 系统审计日志
