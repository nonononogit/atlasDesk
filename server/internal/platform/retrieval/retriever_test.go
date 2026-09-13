package retrieval_test

import (
	"context"
	"testing"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/platform/embedding"
	"atlasdesk/internal/platform/retrieval"
	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

func TestInMemoryRetriever(t *testing.T) {
	ctx := context.Background()
	embedder := embedding.NewMockEmbedder()

	org1 := uuid.New()
	org2 := uuid.New()
	kb1 := uuid.New()
	kb2 := uuid.New()
	doc1 := uuid.New()
	doc2 := uuid.New()

	texts := []string{
		"AtlasDesk 支持高维向量与全文混合检索融合技术",
		"系统架构采用 Go 语言与 React 前后端分离设计",
		"组织内可以通过成员邀请分配不同的角色和权限",
		"同一个文档的第 1 个切片内容",
		"同一个文档的第 2 个切片内容",
		"同一个文档的第 3 个切片内容",
		"同一个文档的第 4 个切片内容，测试多样性过滤",
	}

	vecs, _ := embedder.EmbedTexts(ctx, texts)

	chunks := []domain.DocumentChunk{
		{
			ID:              uuid.New(),
			OrganizationID:  org1,
			KnowledgeBaseID: kb1,
			DocumentID:      doc1,
			Content:         texts[0],
			Embedding:       pgvector.NewVector(vecs[0]),
		},
		{
			ID:              uuid.New(),
			OrganizationID:  org1,
			KnowledgeBaseID: kb1,
			DocumentID:      doc1,
			Content:         texts[1],
			Embedding:       pgvector.NewVector(vecs[1]),
		},
		{
			ID:              uuid.New(),
			OrganizationID:  org2, // 归属 Org2
			KnowledgeBaseID: kb2,
			DocumentID:      uuid.New(),
			Content:         "Org2 的机密混合检索数据",
			Embedding:       pgvector.NewVector(vecs[0]),
		},
		// doc2 的 4 个连续切片
		{
			ID:              uuid.New(),
			OrganizationID:  org1,
			KnowledgeBaseID: kb1,
			DocumentID:      doc2,
			Content:         texts[3],
			Embedding:       pgvector.NewVector(vecs[3]),
		},
		{
			ID:              uuid.New(),
			OrganizationID:  org1,
			KnowledgeBaseID: kb1,
			DocumentID:      doc2,
			Content:         texts[4],
			Embedding:       pgvector.NewVector(vecs[4]),
		},
		{
			ID:              uuid.New(),
			OrganizationID:  org1,
			KnowledgeBaseID: kb1,
			DocumentID:      doc2,
			Content:         texts[5],
			Embedding:       pgvector.NewVector(vecs[5]),
		},
		{
			ID:              uuid.New(),
			OrganizationID:  org1,
			KnowledgeBaseID: kb1,
			DocumentID:      doc2,
			Content:         texts[6],
			Embedding:       pgvector.NewVector(vecs[6]),
		},
	}

	retriever := retrieval.NewInMemoryRetriever(chunks)

	t.Run("组织隔离性校验", func(t *testing.T) {
		qVec, _ := embedder.EmbedTexts(ctx, []string{"混合检索"})
		results, err := retriever.Retrieve(ctx, org1, nil, "混合检索", qVec[0], 5)
		if err != nil {
			t.Fatalf("检索失败: %v", err)
		}
		for _, r := range results {
			if r.Chunk.OrganizationID != org1 {
				t.Errorf("检索泄漏其他组织数据: %+v", r.Chunk)
			}
		}
	})

	t.Run("文档多样性控制 (同文档不超过 3 个切片)", func(t *testing.T) {
		qVec, _ := embedder.EmbedTexts(ctx, []string{"同一个文档"})
		results, err := retriever.Retrieve(ctx, org1, nil, "同一个文档", qVec[0], 8)
		if err != nil {
			t.Fatalf("检索失败: %v", err)
		}

		doc2Count := 0
		for _, r := range results {
			if r.Chunk.DocumentID == doc2 {
				doc2Count++
			}
		}
		if doc2Count > 3 {
			t.Errorf("同一文档候选数量超出限制 3，实际为: %d", doc2Count)
		}
	})
}
