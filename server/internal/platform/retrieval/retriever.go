package retrieval

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"atlasdesk/internal/domain"
	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
)

// RRF 平滑常数 (标准采用 k = 60)
const rrfK = 60.0

// 单文档最大切片数量配额 (保证检索多样性)
const maxChunksPerDocument = 3

// Retriever 混合检索接口 (符合规格 14.4 节规范)
type Retriever interface {
	Retrieve(
		ctx context.Context,
		orgID uuid.UUID,
		kbIDs []uuid.UUID,
		query string,
		queryVec []float32,
		topK int,
	) ([]domain.ScoredChunk, error)
}

// HybridRetriever 混合检索实现 (向量召回 + 全文召回 + RRF 融合)
type HybridRetriever struct {
	db *gorm.DB
}

// NewHybridRetriever 构造混合检索器
func NewHybridRetriever(db *gorm.DB) *HybridRetriever {
	return &HybridRetriever{db: db}
}

// Retrieve 执行混合检索、RRF 融合去重与多样性限制
func (r *HybridRetriever) Retrieve(
	ctx context.Context,
	orgID uuid.UUID,
	kbIDs []uuid.UUID,
	query string,
	queryVec []float32,
	topK int,
) ([]domain.ScoredChunk, error) {
	if topK <= 0 {
		topK = 5
	}
	if topK > 8 {
		topK = 8
	}

	// 1. 向量召回 Top 20
	denseHits, err := r.searchDense(ctx, orgID, kbIDs, queryVec, 20)
	if err != nil {
		return nil, fmt.Errorf("向量检索失败: %w", err)
	}

	// 2. 文本召回 Top 20
	sparseHits, err := r.searchSparse(ctx, orgID, kbIDs, query, 20)
	if err != nil {
		return nil, fmt.Errorf("全文检索失败: %w", err)
	}

	// 3. RRF 倒数排名融合 (Reciprocal Rank Fusion)
	fusedMap := make(map[uuid.UUID]*domain.ScoredChunk)

	for rank, chunk := range denseHits {
		denseRank := rank + 1
		rrfScore := 1.0 / (rrfK + float64(denseRank))
		fusedMap[chunk.ID] = &domain.ScoredChunk{
			Chunk:     chunk,
			Score:     rrfScore,
			DenseRank: denseRank,
		}
	}

	for rank, chunk := range sparseHits {
		sparseRank := rank + 1
		rrfScore := 1.0 / (rrfK + float64(sparseRank))
		if existing, ok := fusedMap[chunk.ID]; ok {
			existing.Score += rrfScore
			existing.SparseRank = sparseRank
		} else {
			fusedMap[chunk.ID] = &domain.ScoredChunk{
				Chunk:      chunk,
				Score:      rrfScore,
				SparseRank: sparseRank,
			}
		}
	}

	// 排序
	allScored := make([]domain.ScoredChunk, 0, len(fusedMap))
	for _, sc := range fusedMap {
		allScored = append(allScored, *sc)
	}
	sort.Slice(allScored, func(i, j int) bool {
		return allScored[i].Score > allScored[j].Score
	})

	// 4. 文档多样性过滤 (同文档限额 maxChunksPerDocument)
	docCount := make(map[uuid.UUID]int)
	var finalCandidates []domain.ScoredChunk

	for _, sc := range allScored {
		docID := sc.Chunk.DocumentID
		if docCount[docID] < maxChunksPerDocument {
			finalCandidates = append(finalCandidates, sc)
			docCount[docID]++
			if len(finalCandidates) >= topK {
				break
			}
		}
	}

	return finalCandidates, nil
}

// searchDense 执行 pgvector 向量余弦距离搜索
func (r *HybridRetriever) searchDense(
	ctx context.Context,
	orgID uuid.UUID,
	kbIDs []uuid.UUID,
	queryVec []float32,
	limit int,
) ([]domain.DocumentChunk, error) {
	if len(queryVec) == 0 {
		return nil, nil
	}

	vec := pgvector.NewVector(queryVec)
	query := r.db.WithContext(ctx).
		Table("document_chunks dc").
		Select("dc.*").
		Joins("JOIN documents d ON d.id = dc.document_id").
		Where("dc.organization_id = ?", orgID).
		Where("d.deleted_at IS NULL AND d.status = ?", domain.DocStatusReady)

	if len(kbIDs) > 0 {
		query = query.Where("dc.knowledge_base_id IN ?", kbIDs)
	}

	var chunks []domain.DocumentChunk
	err := query.
		Order(gorm.Expr("dc.embedding <=> ?", vec)).
		Limit(limit).
		Find(&chunks).Error

	return chunks, err
}

// searchSparse 执行文本匹配检索
func (r *HybridRetriever) searchSparse(
	ctx context.Context,
	orgID uuid.UUID,
	kbIDs []uuid.UUID,
	queryText string,
	limit int,
) ([]domain.DocumentChunk, error) {
	trimmed := strings.TrimSpace(queryText)
	if trimmed == "" {
		return nil, nil
	}

	query := r.db.WithContext(ctx).
		Table("document_chunks dc").
		Select("dc.*").
		Joins("JOIN documents d ON d.id = dc.document_id").
		Where("dc.organization_id = ?", orgID).
		Where("d.deleted_at IS NULL AND d.status = ?", domain.DocStatusReady)

	if len(kbIDs) > 0 {
		query = query.Where("dc.knowledge_base_id IN ?", kbIDs)
	}

	// 提取查询中的关键词
	words := strings.Fields(trimmed)
	if len(words) > 0 {
		likePattern := "%" + words[0] + "%"
		query = query.Where("dc.content ILIKE ?", likePattern)
	}

	var chunks []domain.DocumentChunk
	err := query.Limit(limit).Find(&chunks).Error
	return chunks, err
}

// ----------------------------------------------------------------------------
// 内存模拟检索器 (用于单元测试、无数据库脱机环境与评测套件)
// ----------------------------------------------------------------------------

type InMemoryRetriever struct {
	chunks []domain.DocumentChunk
}

// NewInMemoryRetriever 构造内存测试检索器
func NewInMemoryRetriever(chunks []domain.DocumentChunk) *InMemoryRetriever {
	return &InMemoryRetriever{chunks: chunks}
}

func (m *InMemoryRetriever) Retrieve(
	ctx context.Context,
	orgID uuid.UUID,
	kbIDs []uuid.UUID,
	query string,
	queryVec []float32,
	topK int,
) ([]domain.ScoredChunk, error) {
	if topK <= 0 {
		topK = 5
	}
	if topK > 8 {
		topK = 8
	}

	queryLower := strings.ToLower(query)
	var scored []domain.ScoredChunk

	for _, chunk := range m.chunks {
		if chunk.OrganizationID != orgID {
			continue
		}
		if len(kbIDs) > 0 {
			matchKB := false
			for _, k := range kbIDs {
				if chunk.KnowledgeBaseID == k {
					matchKB = true
					break
				}
			}
			if !matchKB {
				continue
			}
		}

		// 计算余弦相似度
		var sim float64 = 0
		if len(queryVec) > 0 && chunk.Embedding.Slice() != nil {
			vec := chunk.Embedding.Slice()
			if len(vec) == len(queryVec) {
				var dot, normA, normB float64
				for i := range vec {
					dot += float64(vec[i] * queryVec[i])
					normA += float64(vec[i] * vec[i])
					normB += float64(queryVec[i] * queryVec[i])
				}
				if normA > 0 && normB > 0 {
					sim = dot / (math.Sqrt(normA) * math.Sqrt(normB))
				}
			}
		}

		// 全文稀疏检索 (模拟 PostgreSQL tsvector/tsquery 与 BM25 词频匹配)
		terms := tokenizeQuery(queryLower)
		termHits := 0
		for _, term := range terms {
			termLower := strings.ToLower(term)
			if strings.Contains(strings.ToLower(chunk.Content), termLower) ||
				strings.Contains(strings.ToLower(chunk.Metadata.DocumentName), termLower) ||
				strings.Contains(strings.ToLower(chunk.Metadata.HeadingPath), termLower) {
				termHits++
			}
		}

		if len(terms) > 0 && termHits > 0 {
			// 稀疏检索贡献打分
			sparseScore := float64(termHits) / float64(len(terms))
			sim += sparseScore
		}

		// 相关度门槛过滤 (符合规格 14.5 要求)
		if sim > 0.15 {
			scored = append(scored, domain.ScoredChunk{
				Chunk: chunk,
				Score: sim,
			})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// 文档多样性过滤
	docCount := make(map[uuid.UUID]int)
	var finalCandidates []domain.ScoredChunk

	for _, sc := range scored {
		docID := sc.Chunk.DocumentID
		if docCount[docID] < maxChunksPerDocument {
			finalCandidates = append(finalCandidates, sc)
			docCount[docID]++
			if len(finalCandidates) >= topK {
				break
			}
		}
	}

	return finalCandidates, nil
}

// tokenizeQuery 针对搜索查询提取检索关键词或 n-gram 分词
// 支持中英文混合切分，用于内存检索模拟全文检索
func tokenizeQuery(query string) []string {
	// 剔除标点符号与特殊字符
	replacer := strings.NewReplacer(
		"？", " ", "?", " ", "！", " ", "!", " ",
		"，", " ", ",", " ", "。", " ", ".", " ",
		"：", " ", ":", " ", "、", " ", "“", " ", "”", " ",
		"（", " ", "）", " ", "(", " ", ")", " ",
		"[", " ", "]", " ", "{", " ", "}", " ",
		"\\", " ", "/", " ", ";", " ", "；", " ",
	)
	cleaned := replacer.Replace(query)
	fields := strings.Fields(cleaned)

	var tokens []string
	seen := make(map[string]bool)

	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len(field) == 0 {
			continue
		}

		// 英文或数字单词直接加入
		if !seen[field] {
			seen[field] = true
			tokens = append(tokens, field)
		}

		// 中文长词生成 2-gram / 3-gram 提升召回率
		runes := []rune(field)
		if len(runes) > 1 {
			for i := 0; i < len(runes)-1; i++ {
				bi := string(runes[i : i+2])
				if !seen[bi] {
					seen[bi] = true
					tokens = append(tokens, bi)
				}
			}
		}
	}

	return tokens
}

