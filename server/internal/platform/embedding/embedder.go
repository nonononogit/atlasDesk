package embedding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"
)

// 标准向量维度 (符合规格 5.4 节，采用主流 1536 维标准)
const DefaultDimension = 1536

// 错误定义
var (
	ErrDimensionMismatch = errors.New("EMBEDDING_DIMENSION_MISMATCH")
	ErrBatchTooLarge     = errors.New("EMBEDDING_BATCH_TOO_LARGE")
)

// Embedder 文本向量化接口
type Embedder interface {
	EmbedTexts(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

// ----------------------------------------------------------------------------
// 1. MockEmbedder: 本地测试与脱机环境向量生成器 (确定性、单位长度归一化)
// ----------------------------------------------------------------------------

type MockEmbedder struct {
	dim int
}

// NewMockEmbedder 构造脱机 Mock Embedder
func NewMockEmbedder() *MockEmbedder {
	return &MockEmbedder{dim: DefaultDimension}
}

func (m *MockEmbedder) Dimension() int {
	return m.dim
}

// EmbedTexts 为每个输入文本生成确定性、符合余弦计算特性的 1536 维归一化向量
func (m *MockEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) > 64 {
		return nil, fmt.Errorf("%w: 单批不能超过 64 条", ErrBatchTooLarge)
	}

	results := make([][]float32, len(texts))
	for idx, text := range texts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 基于 sha256 种子伪随机扩散生成 1536 维向量
		hash := sha256.Sum256([]byte(text))
		seed := binary.BigEndian.Uint64(hash[:8])

		vec := make([]float32, m.dim)
		var sumSq float64

		for i := 0; i < m.dim; i++ {
			// 线性同余发生器 (LCG)
			seed = seed*6364136223846793005 + 1442695040888963407
			val := float32((seed>>33)&0x7FFFFFFF)/float32(0x7FFFFFFF) - 0.5
			vec[i] = val
			sumSq += float64(val * val)
		}

		// L2 归一化 (使得向量模长为 1，满足余弦距离运算)
		norm := math.Sqrt(sumSq)
		if norm > 0 {
			for i := 0; i < m.dim; i++ {
				vec[i] = float32(float64(vec[i]) / norm)
			}
		}

		results[idx] = vec
	}

	return results, nil
}

// ----------------------------------------------------------------------------
// 2. OpenAIEmbedder: 生产级兼容 OpenAI / v1/embeddings 接口的客户端
// ----------------------------------------------------------------------------

type OpenAIEmbedder struct {
	baseURL    string
	apiKey     string
	model      string
	dim        int
	httpClient *http.Client
}

type openAIEmbedRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type openAIEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewOpenAIEmbedder 构造 OpenAI 兼容的 Embedding 客户端
func NewOpenAIEmbedder(baseURL, apiKey, model string) *OpenAIEmbedder {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAIEmbedder{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		dim:     DefaultDimension,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (o *OpenAIEmbedder) Dimension() int {
	return o.dim
}

func (o *OpenAIEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	if len(texts) > 32 {
		return nil, fmt.Errorf("%w: 单批上限 32 条", ErrBatchTooLarge)
	}

	reqBody, err := json.Marshal(openAIEmbedRequest{
		Input: texts,
		Model: o.model,
	})
	if err != nil {
		return nil, fmt.Errorf("序列化 Embedding 请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("构建 HTTP 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用 Embedding 接口网络异常: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Embedding 服务端返回错误状态码: %d", resp.StatusCode)
	}

	var res openAIEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("解析 Embedding 响应 JSON 失败: %w", err)
	}

	if res.Error != nil {
		return nil, fmt.Errorf("Embedding 服务返回业务错误: %s", res.Error.Message)
	}

	output := make([][]float32, len(texts))
	for _, item := range res.Data {
		if len(item.Embedding) != o.dim {
			return nil, fmt.Errorf("%w: 预期维度 %d, 实际收到 %d", ErrDimensionMismatch, o.dim, len(item.Embedding))
		}
		if item.Index >= 0 && item.Index < len(output) {
			output[item.Index] = item.Embedding
		}
	}

	return output, nil
}
