package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SSE 流式事件常量定义 (符合规格第 7 节 A-03 规范)
const (
	EventRetrievalStarted = "retrieval_started"
	EventAnswerDelta      = "answer_delta"
	EventCitations        = "citations"
	EventDone             = "done"
	EventError            = "error"
)

// 消息角色常量定义
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// 消息状态常量定义
const (
	MessageStatusSending   = "sending"
	MessageStatusSent      = "sent"
	MessageStatusCancelled = "cancelled"
	MessageStatusFailed    = "failed"
)

// TokenUsage 模型调用用量统计
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Value 实现 driver.Valuer 接口将 TokenUsage 序列化为 JSONB
func (u TokenUsage) Value() (driver.Value, error) {
	return json.Marshal(u)
}

// Scan 实现 sql.Scanner 接口从 JSONB 反序列化
func (u *TokenUsage) Scan(value any) error {
	if value == nil {
		*u = TokenUsage{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		if s, ok := value.(string); ok {
			bytes = []byte(s)
		} else {
			return errors.New("无法解析 TokenUsage: 非字节或字符串")
		}
	}
	return json.Unmarshal(bytes, u)
}

// Conversation RAG 问答会话聚合根
type Conversation struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Title          string         `gorm:"type:varchar(255);not null;default:'新对话'" json:"title"`
	CreatedAt      time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	Messages       []Message      `gorm:"foreignKey:ConversationID" json:"messages,omitempty"`
}

// Message 会话内单条对话消息
type Message struct {
	ID             uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ConversationID uuid.UUID         `gorm:"type:uuid;not null;index" json:"conversation_id"`
	Role           string            `gorm:"type:varchar(50);not null" json:"role"`
	Content        string            `gorm:"type:text;not null" json:"content"`
	Status         string            `gorm:"type:varchar(50);not null;default:'sent'" json:"status"`
	Model          string            `gorm:"type:varchar(100);not null;default:''" json:"model"`
	Usage          TokenUsage        `gorm:"type:jsonb;not null;default:'{}'" json:"usage"`
	CreatedAt      time.Time         `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	Citations      []MessageCitation `gorm:"foreignKey:MessageID" json:"citations,omitempty"`
	Feedback       *MessageFeedback  `gorm:"foreignKey:MessageID" json:"feedback,omitempty"`
}

// MessageCitation 消息切片溯源引用明细
type MessageCitation struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	MessageID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"message_id"`
	ChunkID        *uuid.UUID `gorm:"type:uuid" json:"chunk_id,omitempty"`
	CitationIndex  int        `gorm:"type:int;not null" json:"citation_index"` // 对应正文中的 [1], [2] 序号
	Quote          string     `gorm:"type:text;not null;default:''" json:"quote"`
	DocumentTitle  string     `gorm:"type:varchar(255);not null;default:''" json:"document_title"`
	PageNumber     int        `gorm:"type:int;not null;default:0" json:"page_number"`
	CreatedAt      time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

// MessageFeedback 针对回答的评价反馈
type MessageFeedback struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	MessageID uuid.UUID `gorm:"type:uuid;not null;index" json:"message_id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Rating    int       `gorm:"type:int;not null" json:"rating"` // 1: 有帮助, -1: 无帮助
	Reason    string    `gorm:"type:varchar(255);not null;default:''" json:"reason"`
	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

// ----------------------------------------------------------------------------
// 请求与响应传输对象 (DTO)
// ----------------------------------------------------------------------------

// CreateConversationReq 创建会话请求体 (可选提供初始标题)
type CreateConversationReq struct {
	Title string `json:"title" binding:"omitempty,max=100"`
}

// SendMessageStreamReq 发送问题流式问答请求体 (符合规格 A-03 规范)
type SendMessageStreamReq struct {
	Content          string   `json:"content" binding:"required,max=2000"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"` // 可选限定知识库范围
}

// MessageFeedbackReq 提交回答点赞/点踩参数体 (符合规格 A-07 规范)
type MessageFeedbackReq struct {
	Rating int    `json:"rating" binding:"required,oneof=1 -1"`
	Reason string `json:"reason" binding:"omitempty,max=255"`
}

// ScoredChunk 混合检索召回并计算得分的切片
type ScoredChunk struct {
	Chunk            DocumentChunk
	Score            float64 // 综合融合得分 (RRF / 相关度)
	DenseRank        int     // 向量检索排名
	SparseRank       int     // 全文检索排名
}
