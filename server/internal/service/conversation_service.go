package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/repository"
	"github.com/google/uuid"
)

// 业务错误定义
var (
	ErrConversationNotFound = errors.New("会话不存在或已被删除")
	ErrMessageNotFound      = errors.New("消息不存在")
)

// ConversationService 会话与历史消息业务服务接口
type ConversationService interface {
	// 会话管理
	GetOrCreateConversation(ctx context.Context, orgID, userID uuid.UUID, convID *uuid.UUID, firstQuestion string) (*domain.Conversation, error)
	GetConversation(ctx context.Context, orgID, convID uuid.UUID) (*domain.Conversation, error)
	ListConversations(ctx context.Context, orgID, userID uuid.UUID, cursor string, limit int) (*domain.CursorPage[domain.Conversation], error)
	DeleteConversation(ctx context.Context, orgID, convID uuid.UUID) error

	// 消息与引用
	ListMessages(ctx context.Context, orgID, convID uuid.UUID) ([]domain.Message, error)
	GetCitation(ctx context.Context, orgID, citationID uuid.UUID) (*domain.MessageCitation, error)
	SubmitFeedback(ctx context.Context, orgID, userID, messageID uuid.UUID, rating int, reason string) error
}

// DefaultConversationService 会话服务实现
type DefaultConversationService struct {
	repo repository.ConversationRepository
}

// NewConversationService 构造 DefaultConversationService 实例
func NewConversationService(repo repository.ConversationRepository) *DefaultConversationService {
	return &DefaultConversationService{repo: repo}
}

// GetOrCreateConversation 获取已有会话，或在首次提问时懒创建会话 (规范 A-02: 空会话不落库，首问落库)
func (s *DefaultConversationService) GetOrCreateConversation(
	ctx context.Context,
	orgID, userID uuid.UUID,
	convID *uuid.UUID,
	firstQuestion string,
) (*domain.Conversation, error) {
	if convID != nil && *convID != uuid.Nil {
		conv, err := s.repo.GetConversationByID(ctx, orgID, *convID)
		if err != nil {
			return nil, err
		}
		if conv != nil {
			return conv, nil
		}
	}

	// 截取首个问题前 20 字符作为临时标题 (符合 A-02 规范)
	title := generateTempTitle(firstQuestion)

	newConv := &domain.Conversation{
		OrganizationID: orgID,
		UserID:         userID,
		Title:          title,
	}

	if err := s.repo.CreateConversation(ctx, newConv); err != nil {
		return nil, err
	}

	return newConv, nil
}

// GetConversation 获取单条会话详情
func (s *DefaultConversationService) GetConversation(ctx context.Context, orgID, convID uuid.UUID) (*domain.Conversation, error) {
	conv, err := s.repo.GetConversationByID(ctx, orgID, convID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, ErrConversationNotFound
	}
	return conv, nil
}

// ListConversations 列出用户的会话历史列表 (游标分页)
func (s *DefaultConversationService) ListConversations(ctx context.Context, orgID, userID uuid.UUID, cursor string, limit int) (*domain.CursorPage[domain.Conversation], error) {
	return s.repo.ListConversations(ctx, orgID, userID, cursor, limit)
}

// DeleteConversation 软删除会话
func (s *DefaultConversationService) DeleteConversation(ctx context.Context, orgID, convID uuid.UUID) error {
	conv, err := s.GetConversation(ctx, orgID, convID)
	if err != nil {
		return err
	}
	return s.repo.DeleteConversation(ctx, orgID, conv.ID)
}

// ListMessages 列出指定会话内的历史问答记录
func (s *DefaultConversationService) ListMessages(ctx context.Context, orgID, convID uuid.UUID) ([]domain.Message, error) {
	// 校验当前会话组织归属
	_, err := s.GetConversation(ctx, orgID, convID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListMessagesByConversation(ctx, convID)
}

// GetCitation 查询单条引用详情
func (s *DefaultConversationService) GetCitation(ctx context.Context, orgID, citationID uuid.UUID) (*domain.MessageCitation, error) {
	citation, err := s.repo.GetCitationByID(ctx, citationID)
	if err != nil {
		return nil, err
	}
	if citation == nil {
		return nil, errors.New("引用记录不存在")
	}
	return citation, nil
}

// SubmitFeedback 提交对回答的满意度评价 (符合 A-07 规范)
func (s *DefaultConversationService) SubmitFeedback(ctx context.Context, orgID, userID, messageID uuid.UUID, rating int, reason string) error {
	msg, err := s.repo.GetMessageByID(ctx, messageID)
	if err != nil {
		return err
	}
	if msg == nil {
		return ErrMessageNotFound
	}

	feedback := &domain.MessageFeedback{
		MessageID: messageID,
		UserID:    userID,
		Rating:    rating,
		Reason:    reason,
	}

	return s.repo.SaveFeedback(ctx, feedback)
}

// generateTempTitle 提取前 20 字生成清晰标题
func generateTempTitle(question string) string {
	trimmed := strings.TrimSpace(question)
	if trimmed == "" {
		return "新对话"
	}
	runes := []rune(trimmed)
	if utf8.RuneCountInString(trimmed) > 20 {
		return string(runes[:20]) + "..."
	}
	return trimmed
}
