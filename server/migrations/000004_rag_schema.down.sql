-- ==============================================================================
-- 迁移 000004 回滚: 删除 RAG 会话、消息、引用与反馈表
-- ==============================================================================

DROP TABLE IF EXISTS message_feedback;
DROP TABLE IF EXISTS message_citations;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;
