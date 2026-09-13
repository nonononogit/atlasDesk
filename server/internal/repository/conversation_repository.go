package repository

import (
	"context"
	"errors"
	"time"

	"atlasdesk/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConversationRepository 会话与消息数据仓储接口
type ConversationRepository interface {
	// 会话管理
	CreateConversation(ctx context.Context, conv *domain.Conversation) error
	GetConversationByID(ctx context.Context, orgID, convID uuid.UUID) (*domain.Conversation, error)
	ListConversations(ctx context.Context, orgID, userID uuid.UUID, cursor string, limit int) (*domain.CursorPage[domain.Conversation], error)
	UpdateConversationTitle(ctx context.Context, orgID, convID uuid.UUID, title string) error
	DeleteConversation(ctx context.Context, orgID, convID uuid.UUID) error

	// 消息与引用管理
	CreateMessage(ctx context.Context, msg *domain.Message) error
	UpdateMessage(ctx context.Context, msg *domain.Message) error
	GetMessageByID(ctx context.Context, msgID uuid.UUID) (*domain.Message, error)
	ListMessagesByConversation(ctx context.Context, convID uuid.UUID) ([]domain.Message, error)
	SaveCitations(ctx context.Context, citations []domain.MessageCitation) error
	GetCitationByID(ctx context.Context, citationID uuid.UUID) (*domain.MessageCitation, error)

	// 反馈管理
	SaveFeedback(ctx context.Context, feedback *domain.MessageFeedback) error
}

// GormConversationRepository 基于 GORM 的会话仓储实现
type GormConversationRepository struct {
	db *gorm.DB
}

// NewConversationRepository 构造 GormConversationRepository 实例
func NewConversationRepository(db *gorm.DB) *GormConversationRepository {
	return &GormConversationRepository{db: db}
}

// CreateConversation 创建新会话
func (r *GormConversationRepository) CreateConversation(ctx context.Context, conv *domain.Conversation) error {
	return r.db.WithContext(ctx).Create(conv).Error
}

// GetConversationByID 查询会话详情 (强制多租户组织隔离)
func (r *GormConversationRepository) GetConversationByID(ctx context.Context, orgID, convID uuid.UUID) (*domain.Conversation, error) {
	var conv domain.Conversation
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND id = ?", orgID, convID).
		First(&conv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &conv, nil
}

// ListConversations 基于游标分页列出用户的会话 (按更新时间倒序)
func (r *GormConversationRepository) ListConversations(ctx context.Context, orgID, userID uuid.UUID, cursor string, limit int) (*domain.CursorPage[domain.Conversation], error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	query := r.db.WithContext(ctx).
		Where("organization_id = ? AND user_id = ?", orgID, userID)

	if cursor != "" {
		if cursorTime, err := time.Parse(time.RFC3339Nano, cursor); err == nil {
			query = query.Where("updated_at < ?", cursorTime)
		}
	}

	var items []domain.Conversation
	// 查询 limit + 1 条判断是否有下一页
	err := query.Order("updated_at DESC").Limit(limit + 1).Find(&items).Error
	if err != nil {
		return nil, err
	}

	hasMore := len(items) > limit
	var nextCursor string

	if hasMore {
		items = items[:limit]
		nextCursor = items[len(items)-1].UpdatedAt.Format(time.RFC3339Nano)
	}

	return &domain.CursorPage[domain.Conversation]{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// UpdateConversationTitle 更新会话标题
func (r *GormConversationRepository) UpdateConversationTitle(ctx context.Context, orgID, convID uuid.UUID, title string) error {
	return r.db.WithContext(ctx).
		Model(&domain.Conversation{}).
		Where("organization_id = ? AND id = ?", orgID, convID).
		Updates(map[string]any{
			"title":      title,
			"updated_at": time.Now().UTC(),
		}).Error
}

// DeleteConversation 软删除会话
func (r *GormConversationRepository) DeleteConversation(ctx context.Context, orgID, convID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("organization_id = ? AND id = ?", orgID, convID).
		Delete(&domain.Conversation{}).Error
}

// CreateMessage 保存单条消息
func (r *GormConversationRepository) CreateMessage(ctx context.Context, msg *domain.Message) error {
	return r.db.WithContext(ctx).Create(msg).Error
}

// UpdateMessage 更新消息状态、内容或用量
func (r *GormConversationRepository) UpdateMessage(ctx context.Context, msg *domain.Message) error {
	return r.db.WithContext(ctx).
		Model(msg).
		Where("id = ?", msg.ID).
		Updates(map[string]any{
			"content": msg.Content,
			"status":  msg.Status,
			"model":   msg.Model,
			"usage":   msg.Usage,
		}).Error
}

// GetMessageByID 按 ID 查询单条消息
func (r *GormConversationRepository) GetMessageByID(ctx context.Context, msgID uuid.UUID) (*domain.Message, error) {
	var msg domain.Message
	err := r.db.WithContext(ctx).
		Preload("Citations").
		Preload("Feedback").
		Where("id = ?", msgID).
		First(&msg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &msg, nil
}

// ListMessagesByConversation 按时间正序读取会话中的全部消息 (包含预加载引用与反馈)
func (r *GormConversationRepository) ListMessagesByConversation(ctx context.Context, convID uuid.UUID) ([]domain.Message, error) {
	var msgs []domain.Message
	err := r.db.WithContext(ctx).
		Preload("Citations", func(db *gorm.DB) *gorm.DB {
			return db.Order("citation_index ASC")
		}).
		Preload("Feedback").
		Where("conversation_id = ?", convID).
		Order("created_at ASC").
		Find(&msgs).Error
	return msgs, err
}

// SaveCitations 批量插入引用溯源记录
func (r *GormConversationRepository) SaveCitations(ctx context.Context, citations []domain.MessageCitation) error {
	if len(citations) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&citations).Error
}

// GetCitationByID 查询单条引用记录 (附带引用片段原文)
func (r *GormConversationRepository) GetCitationByID(ctx context.Context, citationID uuid.UUID) (*domain.MessageCitation, error) {
	var citation domain.MessageCitation
	err := r.db.WithContext(ctx).
		Where("id = ?", citationID).
		First(&citation).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &citation, nil
}

// SaveFeedback 保存或更新用户对回答的反馈评价 (支持多次修改)
func (r *GormConversationRepository) SaveFeedback(ctx context.Context, feedback *domain.MessageFeedback) error {
	var existing domain.MessageFeedback
	err := r.db.WithContext(ctx).
		Where("message_id = ? AND user_id = ?", feedback.MessageID, feedback.UserID).
		First(&existing).Error

	if err == nil {
		existing.Rating = feedback.Rating
		existing.Reason = feedback.Reason
		return r.db.WithContext(ctx).Save(&existing).Error
	}

	return r.db.WithContext(ctx).Create(feedback).Error
}
