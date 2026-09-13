package rag_test

import (
	"context"
	"testing"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/platform/embedding"
	"atlasdesk/internal/platform/rag"
	"atlasdesk/internal/platform/retrieval"
	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

type mockConvRepoForRAG struct {
	messages  []*domain.Message
	citations []domain.MessageCitation
}

func (m *mockConvRepoForRAG) CreateConversation(ctx context.Context, conv *domain.Conversation) error {
	return nil
}
func (m *mockConvRepoForRAG) GetConversationByID(ctx context.Context, orgID, convID uuid.UUID) (*domain.Conversation, error) {
	return nil, nil
}
func (m *mockConvRepoForRAG) ListConversations(ctx context.Context, orgID, userID uuid.UUID, cursor string, limit int) (*domain.CursorPage[domain.Conversation], error) {
	return nil, nil
}
func (m *mockConvRepoForRAG) UpdateConversationTitle(ctx context.Context, orgID, convID uuid.UUID, title string) error {
	return nil
}
func (m *mockConvRepoForRAG) DeleteConversation(ctx context.Context, orgID, convID uuid.UUID) error {
	return nil
}
func (m *mockConvRepoForRAG) CreateMessage(ctx context.Context, msg *domain.Message) error {
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	m.messages = append(m.messages, msg)
	return nil
}
func (m *mockConvRepoForRAG) UpdateMessage(ctx context.Context, msg *domain.Message) error {
	for i, existing := range m.messages {
		if existing.ID == msg.ID {
			m.messages[i] = msg
			break
		}
	}
	return nil
}
func (m *mockConvRepoForRAG) GetMessageByID(ctx context.Context, msgID uuid.UUID) (*domain.Message, error) {
	return nil, nil
}
func (m *mockConvRepoForRAG) ListMessagesByConversation(ctx context.Context, convID uuid.UUID) ([]domain.Message, error) {
	return nil, nil
}
func (m *mockConvRepoForRAG) SaveCitations(ctx context.Context, citations []domain.MessageCitation) error {
	m.citations = append(m.citations, citations...)
	return nil
}
func (m *mockConvRepoForRAG) GetCitationByID(ctx context.Context, citationID uuid.UUID) (*domain.MessageCitation, error) {
	return nil, nil
}
func (m *mockConvRepoForRAG) SaveFeedback(ctx context.Context, feedback *domain.MessageFeedback) error {
	return nil
}

func TestRAGEngine_StreamChat(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()
	convID := uuid.New()

	embedder := embedding.NewMockEmbedder()
	vecs, _ := embedder.EmbedTexts(ctx, []string{"数据权限如何配置"})

	chunkID := uuid.New()
	chunks := []domain.DocumentChunk{
		{
			ID:             chunkID,
			OrganizationID: orgID,
			Content:        "数据权限在团队管理界面中配置，可通过为成员指派不同角色来控制可见范围。",
			Embedding:      pgvector.NewVector(vecs[0]),
			Metadata: domain.ChunkMetadata{
				DocumentName: "权限管理指南.md",
				HeadingPath:  "第二章 数据权限",
				PageNumber:   1,
			},
		},
	}

	retriever := retrieval.NewInMemoryRetriever(chunks)
	repo := &mockConvRepoForRAG{}
	engine := rag.NewEngine(embedder, retriever, repo, nil)

	eventChan := make(chan rag.StreamEvent, 100)

	t.Run("流式问答事件序列与引用对齐", func(t *testing.T) {
		go func() {
			_ = engine.StreamChat(ctx, orgID, userID, convID, "数据权限如何配置", nil, eventChan)
		}()

		var eventsReceived []string
		var deltaCollected string
		var receivedCitations []domain.MessageCitation

		for event := range eventChan {
			eventsReceived = append(eventsReceived, event.Event)
			if event.Event == domain.EventAnswerDelta {
				if m, ok := event.Data.(map[string]string); ok {
					deltaCollected += m["delta"]
				}
			}
			if event.Event == domain.EventCitations {
				if cits, ok := event.Data.([]domain.MessageCitation); ok {
					receivedCitations = cits
				}
			}
		}

		// 校验事件顺序包含 retrieval_started, answer_delta, citations, done
		if len(eventsReceived) == 0 {
			t.Fatalf("未接收到任何 SSE 事件")
		}
		if eventsReceived[0] != domain.EventRetrievalStarted {
			t.Errorf("首个事件必须为 retrieval_started，实际: %s", eventsReceived[0])
		}
		if eventsReceived[len(eventsReceived)-1] != domain.EventDone {
			t.Errorf("末尾事件必须为 done，实际: %s", eventsReceived[len(eventsReceived)-1])
		}
		if deltaCollected == "" {
			t.Errorf("未接收到流式回答文本")
		}

		// 校验引用生成与白名单过滤
		if len(receivedCitations) == 0 {
			t.Fatalf("预期产生切片引用，实际为空")
		}
		if receivedCitations[0].CitationIndex != 1 || receivedCitations[0].DocumentTitle != "权限管理指南.md" {
			t.Errorf("引用溯源字段错误: %+v", receivedCitations[0])
		}
	})

	t.Run("无命中切片时礼貌拒答", func(t *testing.T) {
		emptyRetriever := retrieval.NewInMemoryRetriever([]domain.DocumentChunk{})
		emptyRepo := &mockConvRepoForRAG{}
		emptyEngine := rag.NewEngine(embedder, emptyRetriever, emptyRepo, nil)

		emptyEventChan := make(chan rag.StreamEvent, 100)
		go func() {
			_ = emptyEngine.StreamChat(ctx, orgID, userID, convID, "完全不相关的超纲问题", nil, emptyEventChan)
		}()

		hasDelta := false
		isDone := false
		for event := range emptyEventChan {
			if event.Event == domain.EventAnswerDelta {
				hasDelta = true
			}
			if event.Event == domain.EventDone {
				isDone = true
			}
		}

		if !hasDelta || !isDone {
			t.Errorf("拒答流式事件未完整输出")
		}
	})
}
