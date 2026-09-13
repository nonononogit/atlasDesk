-- ==============================================================================
-- 迁移 000004: RAG 智能助手会话、消息、引用与反馈架构
-- 创建表: conversations, messages, message_citations, message_feedback
-- ==============================================================================

-- 1. 会话表 (Conversations)
CREATE TABLE IF NOT EXISTS conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL DEFAULT '新对话',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_conversations_org_user ON conversations(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_conversations_updated ON conversations(updated_at DESC);

-- 2. 消息表 (Messages)
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL, -- user, assistant, system
    content TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'sent', -- sending, sent, cancelled, failed
    model VARCHAR(100) NOT NULL DEFAULT '',
    usage JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_conv_created ON messages(conversation_id, created_at ASC);

-- 3. 消息切片引用溯源表 (Message Citations)
CREATE TABLE IF NOT EXISTS message_citations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    chunk_id UUID REFERENCES document_chunks(id) ON DELETE SET NULL,
    citation_index INT NOT NULL,
    quote TEXT NOT NULL DEFAULT '',
    document_title VARCHAR(255) NOT NULL DEFAULT '',
    page_number INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_msg_citation_index UNIQUE (message_id, citation_index)
);

CREATE INDEX IF NOT EXISTS idx_citations_message_id ON message_citations(message_id);

-- 4. 问答反馈表 (Message Feedback)
CREATE TABLE IF NOT EXISTS message_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating INT NOT NULL, -- 1: 赞 (有帮助), -1: 踩 (没帮助)
    reason VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_feedback_message_user UNIQUE (message_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_feedback_message_id ON message_feedback(message_id);
