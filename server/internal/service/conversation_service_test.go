package service_test

import (
	"context"
	"testing"
	"time"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/service"
	"github.com/google/uuid"
)

// mockConversationRepo 会话仓储模拟桩
type mockConversationRepo struct {
	conversations map[uuid.UUID]*domain.Conversation
	messages      map[uuid.UUID]*domain.Message
	feedbacks     map[uuid.UUID]*domain.MessageFeedback
	citations     map[uuid.UUID]*domain.MessageCitation
}

func newMockConversationRepo() *mockConversationRepo {
	return &mockConversationRepo{
		conversations: make(map[uuid.UUID]*domain.Conversation),
		messages:      make(map[uuid.UUID]*domain.Message),
		feedbacks:     make(map[uuid.UUID]*domain.MessageFeedback),
		citations:     make(map[uuid.UUID]*domain.MessageCitation),
	}
}

func (m *mockConversationRepo) CreateConversation(ctx context.Context, conv *domain.Conversation) error {
	if conv.ID == uuid.Nil {
		conv.ID = uuid.New()
	}
	conv.CreatedAt = time.Now()
	conv.UpdatedAt = time.Now()
	m.conversations[conv.ID] = conv
	return nil
}

func (m *mockConversationRepo) GetConversationByID(ctx context.Context, orgID, convID uuid.UUID) (*domain.Conversation, error) {
	c, ok := m.conversations[convID]
	if !ok || c.OrganizationID != orgID || c.DeletedAt.Valid {
		return nil, nil
	}
	return c, nil
}

func (m *mockConversationRepo) ListConversations(ctx context.Context, orgID, userID uuid.UUID, cursor string, limit int) (*domain.CursorPage[domain.Conversation], error) {
	var list []domain.Conversation
	for _, c := range m.conversations {
		if c.OrganizationID == orgID && c.UserID == userID && !c.DeletedAt.Valid {
			list = append(list, *c)
		}
	}
	return &domain.CursorPage[domain.Conversation]{Items: list, HasMore: false}, nil
}

func (m *mockConversationRepo) UpdateConversationTitle(ctx context.Context, orgID, convID uuid.UUID, title string) error {
	if c, ok := m.conversations[convID]; ok && c.OrganizationID == orgID {
		c.Title = title
		c.UpdatedAt = time.Now()
	}
	return nil
}

func (m *mockConversationRepo) DeleteConversation(ctx context.Context, orgID, convID uuid.UUID) error {
	if c, ok := m.conversations[convID]; ok && c.OrganizationID == orgID {
		c.DeletedAt.Valid = true
		c.DeletedAt.Time = time.Now()
	}
	return nil
}

func (m *mockConversationRepo) CreateMessage(ctx context.Context, msg *domain.Message) error {
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	m.messages[msg.ID] = msg
	return nil
}

func (m *mockConversationRepo) UpdateMessage(ctx context.Context, msg *domain.Message) error {
	m.messages[msg.ID] = msg
	return nil
}

func (m *mockConversationRepo) GetMessageByID(ctx context.Context, msgID uuid.UUID) (*domain.Message, error) {
	msg, ok := m.messages[msgID]
	if !ok {
		return nil, nil
	}
	return msg, nil
}

func (m *mockConversationRepo) ListMessagesByConversation(ctx context.Context, convID uuid.UUID) ([]domain.Message, error) {
	var list []domain.Message
	for _, msg := range m.messages {
		if msg.ConversationID == convID {
			list = append(list, *msg)
		}
	}
	return list, nil
}

func (m *mockConversationRepo) SaveCitations(ctx context.Context, citations []domain.MessageCitation) error {
	for _, c := range citations {
		if c.ID == uuid.Nil {
			c.ID = uuid.New()
		}
		m.citations[c.ID] = &c
	}
	return nil
}

func (m *mockConversationRepo) GetCitationByID(ctx context.Context, citationID uuid.UUID) (*domain.MessageCitation, error) {
	return m.citations[citationID], nil
}

func (m *mockConversationRepo) SaveFeedback(ctx context.Context, feedback *domain.MessageFeedback) error {
	m.feedbacks[feedback.MessageID] = feedback
	return nil
}

func TestConversationService(t *testing.T) {
	ctx := context.Background()
	repo := newMockConversationRepo()
	svc := service.NewConversationService(repo)

	orgID := uuid.New()
	userID := uuid.New()

	t.Run("首问懒创建会话并截断 20 字符标题", func(t *testing.T) {
		longQuestion := "这是一段非常长的问题描述，用来测试首问自动截取前二十个字符作为会话临时标题的逻辑"
		conv, err := svc.GetOrCreateConversation(ctx, orgID, userID, nil, longQuestion)
		if err != nil {
			t.Fatalf("懒创建会话失败: %v", err)
		}
		if conv == nil || conv.ID == uuid.Nil {
			t.Fatalf("预期生成新会话")
		}
		if len([]rune(conv.Title)) > 24 { // 20 + "..."
			t.Errorf("标题截断未生效: %s", conv.Title)
		}

		// 再次传入相同 convID，应返回已有会话
		convAgain, err := svc.GetOrCreateConversation(ctx, orgID, userID, &conv.ID, "新问题")
		if err != nil || convAgain.ID != conv.ID {
			t.Errorf("获取已有会话失败")
		}
	})

	t.Run("多租户隔离与软删除校验", func(t *testing.T) {
		conv, _ := svc.GetOrCreateConversation(ctx, orgID, userID, nil, "租户测试问题")

		otherOrg := uuid.New()
		_, err := svc.GetConversation(ctx, otherOrg, conv.ID)
		if err != service.ErrConversationNotFound {
			t.Errorf("跨租户访问未被拦截")
		}

		// 软删除
		err = svc.DeleteConversation(ctx, orgID, conv.ID)
		if err != nil {
			t.Fatalf("软删除失败: %v", err)
		}

		_, err = svc.GetConversation(ctx, orgID, conv.ID)
		if err != service.ErrConversationNotFound {
			t.Errorf("软删除后不应能再次查询到")
		}
	})

	t.Run("回答点赞反馈提交", func(t *testing.T) {
		msgID := uuid.New()
		_ = repo.CreateMessage(ctx, &domain.Message{ID: msgID, Content: "AI 回答内容"})

		err := svc.SubmitFeedback(ctx, orgID, userID, msgID, 1, "回答非常清晰准确")
		if err != nil {
			t.Fatalf("提交反馈失败: %v", err)
		}
		if repo.feedbacks[msgID] == nil || repo.feedbacks[msgID].Rating != 1 {
			t.Errorf("反馈数据未保存")
		}
	})
}
