package embedding_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"atlasdesk/internal/platform/embedding"
)

func TestMockEmbedder(t *testing.T) {
	ctx := context.Background()
	embedder := embedding.NewMockEmbedder()

	if embedder.Dimension() != 1536 {
		t.Fatalf("维度应为 1536，实际: %d", embedder.Dimension())
	}

	texts := []string{
		"AtlasDesk 知识库",
		"基于 pgvector 的向量检索",
	}

	vecs, err := embedder.EmbedTexts(ctx, texts)
	if err != nil {
		t.Fatalf("生成向量失败: %v", err)
	}

	if len(vecs) != 2 {
		t.Fatalf("预期 2 个向量，实际: %d", len(vecs))
	}

	for i, vec := range vecs {
		if len(vec) != 1536 {
			t.Errorf("向量 %d 维度错误: %d", i, len(vec))
		}

		// 验证模长约为 1 (L2 归一化)
		var sumSq float64
		for _, v := range vec {
			sumSq += float64(v * v)
		}
		norm := math.Sqrt(sumSq)
		if math.Abs(norm-1.0) > 1e-4 {
			t.Errorf("向量 %d 模长未归一化: 实际 %f", i, norm)
		}
	}

	// 验证确定性：相同文本生成完全相同的向量
	vecsAgain, _ := embedder.EmbedTexts(ctx, []string{"AtlasDesk 知识库"})
	for i := 0; i < 1536; i++ {
		if vecs[0][i] != vecsAgain[0][i] {
			t.Fatalf("相同输入生成的向量不一致: 索引 %d, %f != %f", i, vecs[0][i], vecsAgain[0][i])
		}
	}
}

func TestOpenAIEmbedder(t *testing.T) {
	ctx := context.Background()

	// 构造 Mock HTTP Server 模拟 OpenAI 接口
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			http.NotFound(w, r)
			return
		}

		// 返回 1536 维向量
		dummyEmbedding := make([]float32, 1536)
		resp := map[string]any{
			"data": []map[string]any{
				{
					"index":     0,
					"embedding": dummyEmbedding,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := embedding.NewOpenAIEmbedder(ts.URL, "fake-key", "text-embedding-3-small")
	vecs, err := client.EmbedTexts(ctx, []string{"测试文本"})
	if err != nil {
		t.Fatalf("OpenAIEmbedder 调用失败: %v", err)
	}

	if len(vecs) != 1 || len(vecs[0]) != 1536 {
		t.Fatalf("返回向量结果不符合规范: %+v", vecs)
	}
}
