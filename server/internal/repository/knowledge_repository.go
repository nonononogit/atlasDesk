package repository

import (
	"context"
	"errors"
	"time"

	"atlasdesk/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// KnowledgeRepository 知识库与文档数据仓储接口
type KnowledgeRepository interface {
	// 知识库管理
	CreateKnowledgeBase(ctx context.Context, kb *domain.KnowledgeBase) error
	ListKnowledgeBases(ctx context.Context, orgID uuid.UUID) ([]domain.KnowledgeBase, error)
	GetKnowledgeBaseByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*domain.KnowledgeBase, error)
	GetKnowledgeBaseByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.KnowledgeBase, error)
	UpdateKnowledgeBase(ctx context.Context, kb *domain.KnowledgeBase) error
	DeleteKnowledgeBase(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error

	// 文档管理
	CreateDocument(ctx context.Context, doc *domain.Document, ver *domain.DocumentVersion) error
	ListDocuments(ctx context.Context, orgID uuid.UUID, filter domain.DocumentFilter) (*domain.CursorPage[domain.Document], error)
	GetDocumentByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*domain.Document, error)
	UpdateDocument(ctx context.Context, doc *domain.Document) error
	DeleteDocument(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error
	GetDocumentVersion(ctx context.Context, docID uuid.UUID, version int) (*domain.DocumentVersion, error)
}

// GormKnowledgeRepository 基于 GORM 实现的知识库仓储
type GormKnowledgeRepository struct {
	db *gorm.DB
}

// NewKnowledgeRepository 构造 GormKnowledgeRepository 实例
func NewKnowledgeRepository(db *gorm.DB) *GormKnowledgeRepository {
	return &GormKnowledgeRepository{db: db}
}

// CreateKnowledgeBase 创建知识库
func (r *GormKnowledgeRepository) CreateKnowledgeBase(ctx context.Context, kb *domain.KnowledgeBase) error {
	return r.db.WithContext(ctx).Create(kb).Error
}

// ListKnowledgeBases 列出租户组织下的所有未删除知识库
func (r *GormKnowledgeRepository) ListKnowledgeBases(ctx context.Context, orgID uuid.UUID) ([]domain.KnowledgeBase, error) {
	var kbs []domain.KnowledgeBase
	// 强制按 organization_id 过滤
	err := r.db.WithContext(ctx).
		Where("organization_id = ?", orgID).
		Order("created_at DESC").
		Find(&kbs).Error
	return kbs, err
}

// GetKnowledgeBaseByID 按 ID 获取知识库详情（显式限定租户组织）
func (r *GormKnowledgeRepository) GetKnowledgeBaseByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*domain.KnowledgeBase, error) {
	var kb domain.KnowledgeBase
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND id = ?", orgID, id).
		First(&kb).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &kb, nil
}

// GetKnowledgeBaseByName 查询同组织内同名知识库（用于重名检查）
func (r *GormKnowledgeRepository) GetKnowledgeBaseByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.KnowledgeBase, error) {
	var kb domain.KnowledgeBase
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND name = ?", orgID, name).
		First(&kb).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &kb, nil
}

// UpdateKnowledgeBase 更新知识库基本信息
func (r *GormKnowledgeRepository) UpdateKnowledgeBase(ctx context.Context, kb *domain.KnowledgeBase) error {
	return r.db.WithContext(ctx).
		Model(kb).
		Where("organization_id = ? AND id = ?", kb.OrganizationID, kb.ID).
		Updates(map[string]any{
			"name":        kb.Name,
			"description": kb.Description,
			"visibility":  kb.Visibility,
			"updated_at":  time.Now().UTC(),
		}).Error
}

// DeleteKnowledgeBase 软删除知识库
func (r *GormKnowledgeRepository) DeleteKnowledgeBase(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("organization_id = ? AND id = ?", orgID, id).
		Delete(&domain.KnowledgeBase{}).Error
}

// CreateDocument 在事务中创建初始文档与物理版本记录
func (r *GormKnowledgeRepository) CreateDocument(ctx context.Context, doc *domain.Document, ver *domain.DocumentVersion) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(doc).Error; err != nil {
			return err
		}
		if ver != nil {
			ver.DocumentID = doc.ID
			if err := tx.Create(ver).Error; err != nil {
				return err
			}
			// 关联当前版本 ID
			doc.CurrentVersionID = &ver.ID
			if err := tx.Model(doc).Update("current_version_id", ver.ID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListDocuments 文档游标分页与多条件筛选 (搜索、格式、状态)
func (r *GormKnowledgeRepository) ListDocuments(ctx context.Context, orgID uuid.UUID, filter domain.DocumentFilter) (*domain.CursorPage[domain.Document], error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	query := r.db.WithContext(ctx).
		Preload("CurrentVersion").
		Where("organization_id = ?", orgID)

	if filter.KnowledgeBaseID != "" {
		query = query.Where("knowledge_base_id = ?", filter.KnowledgeBaseID)
	}
	if filter.Query != "" {
		query = query.Where("name ILIKE ?", "%"+filter.Query+"%")
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Type != "" {
		query = query.Where("mime_type ILIKE ?", "%"+filter.Type+"%")
	}

	// 游标分页逻辑：按 created_at 倒序排列，若有游标则查询早于游标时间的数据
	if filter.Cursor != "" {
		if cursorTime, err := time.Parse(time.RFC3339Nano, filter.Cursor); err == nil {
			query = query.Where("created_at < ?", cursorTime)
		}
	}

	// 多查 1 条以精准判断是否有下一页
	var docs []domain.Document
	err := query.Order("created_at DESC").Limit(limit + 1).Find(&docs).Error
	if err != nil {
		return nil, err
	}

	hasMore := len(docs) > limit
	if hasMore {
		docs = docs[:limit]
	}

	nextCursor := ""
	if hasMore && len(docs) > 0 {
		nextCursor = docs[len(docs)-1].CreatedAt.Format(time.RFC3339Nano)
	}

	return &domain.CursorPage[domain.Document]{
		Items:      docs,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// GetDocumentByID 根据 ID 获取文档详情（强制限制租户组织）
func (r *GormKnowledgeRepository) GetDocumentByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*domain.Document, error) {
	var doc domain.Document
	err := r.db.WithContext(ctx).
		Preload("CurrentVersion").
		Where("organization_id = ? AND id = ?", orgID, id).
		First(&doc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// UpdateDocument 更新文档属性、状态机与解析进度
func (r *GormKnowledgeRepository) UpdateDocument(ctx context.Context, doc *domain.Document) error {
	doc.UpdatedAt = time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(doc).
		Where("organization_id = ? AND id = ?", doc.OrganizationID, doc.ID).
		Updates(map[string]any{
			"name":               doc.Name,
			"size":               doc.Size,
			"checksum":           doc.Checksum,
			"current_version_id": doc.CurrentVersionID,
			"status":             doc.Status,
			"progress":           doc.Progress,
			"error_code":         doc.ErrorCode,
			"error_message":      doc.ErrorMessage,
			"updated_at":         doc.UpdatedAt,
		}).Error
}

// DeleteDocument 软删除文档
func (r *GormKnowledgeRepository) DeleteDocument(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("organization_id = ? AND id = ?", orgID, id).
		Delete(&domain.Document{}).Error
}

// GetDocumentVersion 获取文档特定物理版本
func (r *GormKnowledgeRepository) GetDocumentVersion(ctx context.Context, docID uuid.UUID, version int) (*domain.DocumentVersion, error) {
	var ver domain.DocumentVersion
	err := r.db.WithContext(ctx).
		Where("document_id = ? AND version = ?", docID, version).
		First(&ver).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ver, nil
}
