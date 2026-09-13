-- ==============================================================================
-- 迁移 000002 回滚脚本
-- 逆序删除文档版本、文档与知识库表
-- ==============================================================================

DROP TABLE IF EXISTS document_versions;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS knowledge_bases;
