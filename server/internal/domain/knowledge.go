package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 文档流转状态机常量 (符合规格第 5.1 节)
const (
	DocStatusUploading = "UPLOADING"
	DocStatusUploaded  = "UPLOADED"
	DocStatusParsing   = "PARSING"
	DocStatusChunking  = "CHUNKING"
	DocStatusEmbedding = "EMBEDDING"
	DocStatusReady     = "READY"
	DocStatusFailed    = "FAILED"
)

// 支持的文件 MIME 类型与扩展名定义 (符合规格 1.2 节: PDF, DOCX, Markdown, TXT)
var SupportedMimeTypes = map[string]bool{
	"application/pdf": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"text/markdown": true,
	"text/plain":    true,
}

// KnowledgeBase 知识库实体聚合根
type KnowledgeBase struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;not null;index" json:"organization_id"`
	Name           string         `gorm:"type:varchar(255);not null" json:"name"`
	Description    string         `gorm:"type:text;not null;default:''" json:"description"`
	Visibility     string         `gorm:"type:varchar(50);not null;default:'private'" json:"visibility"` // public, team, private
	CreatedAt      time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// Document 知识库文档实体
type Document struct {
	ID               uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID   uuid.UUID        `gorm:"type:uuid;not null;index" json:"organization_id"`
	KnowledgeBaseID  uuid.UUID        `gorm:"type:uuid;not null;index" json:"knowledge_base_id"`
	Name             string           `gorm:"type:varchar(255);not null" json:"name"`
	MimeType         string           `gorm:"type:varchar(100);not null" json:"mime_type"`
	Size             int64            `gorm:"type:bigint;not null;default:0" json:"size"`
	Checksum         string           `gorm:"type:varchar(64);not null;default:''" json:"checksum"`
	CurrentVersionID *uuid.UUID       `gorm:"type:uuid" json:"current_version_id,omitempty"`
	Status           string           `gorm:"type:varchar(50);not null;default:'UPLOADING'" json:"status"`
	Progress         int              `gorm:"type:int;not null;default:0" json:"progress"`
	ErrorCode        string           `gorm:"type:varchar(100)" json:"error_code,omitempty"`
	ErrorMessage     string           `gorm:"type:text" json:"error_message,omitempty"`
	CurrentVersion   *DocumentVersion `gorm:"foreignKey:CurrentVersionID" json:"current_version,omitempty"`
	CreatedAt        time.Time        `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt        time.Time        `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt        gorm.DeletedAt   `gorm:"index" json:"-"`
}

// DocumentVersion 文档物理版本记录
type DocumentVersion struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DocumentID     uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	Version        int       `gorm:"type:int;not null;default:1" json:"version"`
	ObjectKey      string    `gorm:"type:varchar(500);not null" json:"object_key"`
	ParseStatus    string    `gorm:"type:varchar(50);not null;default:'PENDING'" json:"parse_status"`
	ParserVersion  string    `gorm:"type:varchar(50);not null;default:'1.0'" json:"parser_version"`
	EmbeddingModel string    `gorm:"type:varchar(100);not null;default:''" json:"embedding_model"`
	CreatedAt      time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

// CreateKnowledgeBaseReq 创建知识库参数体
type CreateKnowledgeBaseReq struct {
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description" binding:"max=500"`
	Visibility  string `json:"visibility" binding:"omitempty,oneof=public team private"`
}

// UpdateKnowledgeBaseReq 更新知识库参数体
type UpdateKnowledgeBaseReq struct {
	Name        *string `json:"name" binding:"omitempty,max=100"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	Visibility  *string `json:"visibility" binding:"omitempty,oneof=public team private"`
}

// DocumentUploadReq 请求预签名直传凭据参数体
type DocumentUploadReq struct {
	KnowledgeBaseID string `json:"knowledge_base_id" binding:"required,uuid"`
	Name            string `json:"name" binding:"required,max=255"`
	MimeType        string `json:"mime_type" binding:"required"`
	Size            int64  `json:"size" binding:"required,gt=0"`
}

// DocumentUploadResp 返回前端直传凭据
type DocumentUploadResp struct {
	DocumentID string `json:"document_id"`
	UploadURL  string `json:"upload_url"`
	ObjectKey  string `json:"object_key"`
	ExpiresIn  int    `json:"expires_in"` // 秒
}

// CompleteUploadReq 完成并核验上传请求体
type CompleteUploadReq struct {
	Checksum string `json:"checksum" binding:"omitempty,max=64"`
}

// DocumentFilter 文档筛选与游标查询参数
type DocumentFilter struct {
	KnowledgeBaseID string
	Query           string
	Type            string
	Status          string
	Cursor          string
	Limit           int
}

// CursorPage 通用游标分页结果封装 (符合规范第 4.5 节)
type CursorPage[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}
