package job

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/platform/chunker"
	"atlasdesk/internal/platform/embedding"
	"atlasdesk/internal/platform/parser"
	"atlasdesk/internal/platform/storage"
	"atlasdesk/internal/repository"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/pgvector/pgvector-go"
)

// TaskProcessor 异步任务处理器，负责驱动文档处理流水线
type TaskProcessor struct {
	docRepo       repository.KnowledgeRepository
	chunkRepo     repository.ChunkRepository
	storageClient storage.StorageClient
	embedder      embedding.Embedder
	chunker       *chunker.Chunker
}

// NewTaskProcessor 构造异步任务处理器
func NewTaskProcessor(
	docRepo repository.KnowledgeRepository,
	chunkRepo repository.ChunkRepository,
	storageClient storage.StorageClient,
	embedder embedding.Embedder,
	chunkOptions domain.ChunkOptions,
) *TaskProcessor {
	return &TaskProcessor{
		docRepo:       docRepo,
		chunkRepo:     chunkRepo,
		storageClient: storageClient,
		embedder:      embedder,
		chunker:       chunker.NewChunker(chunkOptions),
	}
}

// ProcessDocumentTask 处理文档解析、切片、向量化异步任务 (符合规格 5.1-5.5 状态机规范)
func (p *TaskProcessor) ProcessDocumentTask(ctx context.Context, t *asynq.Task) error {
	var payload domain.DocumentProcessPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		slog.Error("反序列化任务负载失败", "error", err)
		return fmt.Errorf("反序列化任务负载失败: %w", asynq.SkipRetry)
	}

	orgID, err := uuid.Parse(payload.OrganizationID)
	if err != nil {
		return fmt.Errorf("组织 ID 无效: %w", asynq.SkipRetry)
	}
	docID, err := uuid.Parse(payload.DocumentID)
	if err != nil {
		return fmt.Errorf("文档 ID 无效: %w", asynq.SkipRetry)
	}
	versionID, err := uuid.Parse(payload.VersionID)
	if err != nil {
		return fmt.Errorf("版本 ID 无效: %w", asynq.SkipRetry)
	}

	logger := slog.With(
		"trace_id", payload.TraceID,
		"org_id", orgID.String(),
		"document_id", docID.String(),
		"version_id", versionID.String(),
	)
	logger.Info("开始执行文档异步处理流水线")

	// 1. 查询文档与物理版本记录
	doc, err := p.docRepo.GetDocumentByID(ctx, orgID, docID)
	if err != nil {
		logger.Warn("目标文档不存在或已被删除，结束任务", "error", err)
		return nil
	}

	// 查找对应版本
	var targetVersion *domain.DocumentVersion
	if doc.CurrentVersion != nil && doc.CurrentVersion.ID == versionID {
		targetVersion = doc.CurrentVersion
	} else {
		// 尝试查询对应版本
		v, err := p.docRepo.GetDocumentVersion(ctx, docID, 1)
		if err == nil && v.ID == versionID {
			targetVersion = v
		}
	}

	if targetVersion == nil {
		logger.Warn("目标版本不存在，结束任务")
		return nil
	}

	// 标记失败辅助函数
	markFailed := func(code, message string, skipRetry bool) error {
		logger.Error("文档处理失败", "code", code, "message", message)
		doc.Status = domain.DocStatusFailed
		doc.Progress = 0
		doc.ErrorCode = code
		doc.ErrorMessage = message
		_ = p.docRepo.UpdateDocument(ctx, doc)

		if skipRetry {
			return fmt.Errorf("%s: %s: %w", code, message, asynq.SkipRetry)
		}
		return fmt.Errorf("%s: %s", code, message)
	}

	// 2. 状态转移 -> PARSING (进度: 20%)
	doc.Status = domain.DocStatusParsing
	doc.Progress = 20
	doc.ErrorCode = ""
	doc.ErrorMessage = ""
	if err := p.docRepo.UpdateDocument(ctx, doc); err != nil {
		return fmt.Errorf("更新文档状态为 PARSING 失败: %w", err)
	}

	// 3. 从对象存储读取文件
	body, size, err := p.storageClient.GetObject(ctx, targetVersion.ObjectKey)
	if err != nil {
		return markFailed("STORAGE_READ_FAILED", fmt.Sprintf("读取对象存储失败: %v", err), false)
	}
	defer body.Close()

	fileBytes, err := io.ReadAll(body)
	if err != nil {
		return markFailed("FILE_READ_ERROR", fmt.Sprintf("读取文件流错误: %v", err), false)
	}
	if size <= 0 {
		size = int64(len(fileBytes))
	}

	// 4. 调用对应格式解析器
	docParser, err := parser.GetParser(doc.MimeType)
	if err != nil {
		return markFailed("UNSUPPORTED_FORMAT", err.Error(), true)
	}

	readerAt := bytes.NewReader(fileBytes)
	blocks, err := docParser.Parse(ctx, readerAt, size)
	if err != nil {
		if errors.Is(err, parser.ErrOCRRequired) {
			// 扫描版 PDF 无文本层，规范指定返回 OCR_REQUIRED
			return markFailed("OCR_REQUIRED", "PDF 文件为扫描件或图片，缺少文本层，需 OCR 支持", true)
		}
		if errors.Is(err, parser.ErrEmptyFile) {
			return markFailed("EMPTY_FILE", "文档内容为空", true)
		}
		if errors.Is(err, parser.ErrCorruptedFile) {
			return markFailed("CORRUPTED_FILE", "文档格式损坏无法解析", true)
		}
		return markFailed("PARSE_ERROR", fmt.Sprintf("文档解析失败: %v", err), false)
	}

	// 5. 状态转移 -> CHUNKING (进度: 50%)
	doc.Status = domain.DocStatusChunking
	doc.Progress = 50
	if err := p.docRepo.UpdateDocument(ctx, doc); err != nil {
		return fmt.Errorf("更新文档状态为 CHUNKING 失败: %w", err)
	}

	// 6. 语义分块
	chunks := p.chunker.SplitBlocksToChunks(
		blocks,
		doc.OrganizationID,
		doc.KnowledgeBaseID,
		doc.ID,
		targetVersion.ID,
		doc.Name,
		targetVersion.Version,
	)

	if len(chunks) == 0 {
		return markFailed("NO_CHUNKS_GENERATED", "未产生有效切片内容", true)
	}

	// 7. 状态转移 -> EMBEDDING (进度: 80%)
	doc.Status = domain.DocStatusEmbedding
	doc.Progress = 80
	if err := p.docRepo.UpdateDocument(ctx, doc); err != nil {
		return fmt.Errorf("更新文档状态为 EMBEDDING 失败: %w", err)
	}

	// 8. 批量向量计算 (每批最多 16 条)
	batchSize := 16
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		texts := make([]string, end-i)
		for j := i; j < end; j++ {
			texts[j-i] = chunks[j].Content
		}

		vecs, err := p.embedder.EmbedTexts(ctx, texts)
		if err != nil {
			return markFailed("EMBEDDING_FAILED", fmt.Sprintf("计算文本向量失败: %v", err), false)
		}

		for j := i; j < end; j++ {
			chunks[j].Embedding = pgvector.NewVector(vecs[j-i])
		}
	}

	// 9. 事务幂等入库切片
	if err := p.chunkRepo.SaveChunksInTx(ctx, targetVersion.ID, chunks); err != nil {
		return markFailed("DB_SAVE_CHUNKS_FAILED", fmt.Sprintf("保存切片事务失败: %v", err), false)
	}

	// 10. 状态转移 -> READY (进度: 100%)，原子切换当前有效版本
	doc.Status = domain.DocStatusReady
	doc.Progress = 100
	doc.CurrentVersionID = &targetVersion.ID
	doc.ErrorCode = ""
	doc.ErrorMessage = ""
	if err := p.docRepo.UpdateDocument(ctx, doc); err != nil {
		return fmt.Errorf("更新文档状态为 READY 失败: %w", err)
	}

	logger.Info("文档处理流水线顺利完成", "chunks_count", len(chunks))
	return nil
}
