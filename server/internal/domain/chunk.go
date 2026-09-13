package domain

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

// ChunkMetadata 文档切片溯源元数据 (符合规格 5.3 节)
type ChunkMetadata struct {
	DocumentName  string `json:"document_name,omitempty"`  // 原始文档名称
	HeadingPath   string `json:"heading_path,omitempty"`   // 标题层级路径 (例如: "第一章 > 1.2 认证规范")
	PageNumber    int    `json:"page_number,omitempty"`    // 对应原始页码 (PDF等支持，从1开始)
	SourceVersion int    `json:"source_version,omitempty"` // 文档物理版本号
}

// Value 实现 driver.Valuer 接口，将元数据转为 JSON 字符串存入 JSONB 列
func (m ChunkMetadata) Value() (driver.Value, error) {
	return json.Marshal(m)
}

// Scan 实现 sql.Scanner 接口，从数据库 JSONB 反序列化
func (m *ChunkMetadata) Scan(value any) error {
	if value == nil {
		*m = ChunkMetadata{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		if s, ok := value.(string); ok {
			bytes = []byte(s)
		} else {
			return errors.New("无法解析 ChunkMetadata: 非字节数组或字符串")
		}
	}
	return json.Unmarshal(bytes, m)
}

// DocumentChunk 文档切片实体模型
type DocumentChunk struct {
	ID                uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID    uuid.UUID       `gorm:"type:uuid;not null;index" json:"organization_id"`
	KnowledgeBaseID   uuid.UUID       `gorm:"type:uuid;not null;index" json:"knowledge_base_id"`
	DocumentID        uuid.UUID       `gorm:"type:uuid;not null;index" json:"document_id"`
	DocumentVersionID uuid.UUID       `gorm:"type:uuid;not null;index" json:"document_version_id"`
	ChunkIndex        int             `gorm:"type:int;not null" json:"chunk_index"`
	Content           string          `gorm:"type:text;not null" json:"content"`
	TokenCount        int             `gorm:"type:int;not null;default:0" json:"token_count"`
	Metadata          ChunkMetadata   `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	Embedding         pgvector.Vector `gorm:"type:vector(1536)" json:"embedding,omitempty"`
	CreatedAt         time.Time       `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

// ParsedBlock 解析器输出的基础语义块
type ParsedBlock struct {
	HeadingPath string // 所在标题层级
	PageNumber  int    // 页码
	Content     string // 文本内容
}

// ChunkOptions 切片参数配置
type ChunkOptions struct {
	TargetTokens  int // 目标切片大小 (默认 500)
	OverlapTokens int // 重叠大小 (默认 80)
}

// DocumentProcessPayload 异步文档处理任务负载 (轻量仅传标识，Worker 实时查库)
type DocumentProcessPayload struct {
	OrganizationID string `json:"organization_id"`
	DocumentID     string `json:"document_id"`
	VersionID      string `json:"version_id"`
	TraceID        string `json:"trace_id"`
}
