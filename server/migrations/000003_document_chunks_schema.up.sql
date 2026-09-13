-- ==============================================================================
-- 迁移 000003: 文档切片与向量存储架构
-- 创建表: document_chunks
-- ==============================================================================

-- 启用 pgvector 扩展 (支持高维向量检索)
CREATE EXTENSION IF NOT EXISTS vector;

-- 创建文档切片表 (Document Chunks)
CREATE TABLE IF NOT EXISTS document_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    document_version_id UUID NOT NULL REFERENCES document_versions(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    token_count INT NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    embedding vector(1536),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_chunk_version_index UNIQUE (document_version_id, chunk_index)
);

-- 索引：按组织与知识库过滤
CREATE INDEX IF NOT EXISTS idx_chunks_org_kb ON document_chunks(organization_id, knowledge_base_id);
-- 索引：按文档版本查询切片
CREATE INDEX IF NOT EXISTS idx_chunks_doc_version ON document_chunks(document_version_id);
