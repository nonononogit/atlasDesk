-- ==============================================================================
-- 迁移 000002: 知识库与文档模型架构
-- 创建表: knowledge_bases, documents, document_versions
-- ==============================================================================

-- 1. 知识库表 (Knowledge Bases)
CREATE TABLE IF NOT EXISTS knowledge_bases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    visibility VARCHAR(50) NOT NULL DEFAULT 'private', -- public, team, private
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

-- 同组织内知识库名称唯一索引 (排查已软删除记录)
CREATE UNIQUE INDEX IF NOT EXISTS uq_kb_org_name_active 
ON knowledge_bases (organization_id, name) 
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_kb_org_id ON knowledge_bases(organization_id);

-- 2. 文档表 (Documents)
CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    knowledge_base_id UUID NOT NULL REFERENCES knowledge_bases(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    checksum VARCHAR(64) NOT NULL DEFAULT '',
    current_version_id UUID,
    status VARCHAR(50) NOT NULL DEFAULT 'UPLOADING', -- UPLOADING, UPLOADED, PARSING, CHUNKING, EMBEDDING, READY, FAILED
    progress INT NOT NULL DEFAULT 0,
    error_code VARCHAR(100),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_documents_org_kb ON documents(organization_id, knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);

-- 3. 文档版本表 (Document Versions)
CREATE TABLE IF NOT EXISTS document_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    object_key VARCHAR(500) NOT NULL,
    parse_status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    parser_version VARCHAR(50) NOT NULL DEFAULT '1.0',
    embedding_model VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_doc_version UNIQUE (document_id, version)
);

CREATE INDEX IF NOT EXISTS idx_doc_versions_doc_id ON document_versions(document_id);
