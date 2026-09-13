package job_test

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/job"
	"atlasdesk/internal/platform/embedding"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// ----------------------------------------------------------------------------
// 模拟仓储与存储实现
// ----------------------------------------------------------------------------

type mockDocRepo struct {
	doc     *domain.Document
	version *domain.DocumentVersion
}

func (m *mockDocRepo) CreateKnowledgeBase(ctx context.Context, kb *domain.KnowledgeBase) error {
	return nil
}
func (m *mockDocRepo) ListKnowledgeBases(ctx context.Context, orgID uuid.UUID) ([]domain.KnowledgeBase, error) {
	return nil, nil
}
func (m *mockDocRepo) GetKnowledgeBaseByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*domain.KnowledgeBase, error) {
	return nil, nil
}
func (m *mockDocRepo) GetKnowledgeBaseByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.KnowledgeBase, error) {
	return nil, nil
}
func (m *mockDocRepo) UpdateKnowledgeBase(ctx context.Context, kb *domain.KnowledgeBase) error {
	return nil
}
func (m *mockDocRepo) DeleteKnowledgeBase(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	return nil
}
func (m *mockDocRepo) CreateDocument(ctx context.Context, doc *domain.Document, ver *domain.DocumentVersion) error {
	return nil
}
func (m *mockDocRepo) ListDocuments(ctx context.Context, orgID uuid.UUID, filter domain.DocumentFilter) (*domain.CursorPage[domain.Document], error) {
	return nil, nil
}
func (m *mockDocRepo) GetDocumentByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*domain.Document, error) {
	return m.doc, nil
}
func (m *mockDocRepo) UpdateDocument(ctx context.Context, doc *domain.Document) error {
	m.doc = doc
	return nil
}
func (m *mockDocRepo) DeleteDocument(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	return nil
}
func (m *mockDocRepo) GetDocumentVersion(ctx context.Context, docID uuid.UUID, version int) (*domain.DocumentVersion, error) {
	return m.version, nil
}
func (m *mockDocRepo) GetLatestDocumentVersion(ctx context.Context, docID uuid.UUID) (*domain.DocumentVersion, error) {
	return m.version, nil
}
func (m *mockDocRepo) CreateDocumentVersion(ctx context.Context, ver *domain.DocumentVersion) error {
	m.version = ver
	return nil
}

type mockChunkRepo struct {
	savedChunks []domain.DocumentChunk
}

func (m *mockChunkRepo) SaveChunksInTx(ctx context.Context, versionID uuid.UUID, chunks []domain.DocumentChunk) error {
	m.savedChunks = chunks
	return nil
}
func (m *mockChunkRepo) GetChunksByVersion(ctx context.Context, versionID uuid.UUID) ([]domain.DocumentChunk, error) {
	return m.savedChunks, nil
}
func (m *mockChunkRepo) CountChunksByVersion(ctx context.Context, versionID uuid.UUID) (int64, error) {
	return int64(len(m.savedChunks)), nil
}

type mockStorage struct {
	content string
}

func (m *mockStorage) PresignUploadURL(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	return "", nil
}
func (m *mockStorage) PresignDownloadURL(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	return "", nil
}
func (m *mockStorage) StatObject(ctx context.Context, objectKey string) (int64, string, error) {
	return int64(len(m.content)), "etag", nil
}
func (m *mockStorage) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, int64, error) {
	return io.NopCloser(strings.NewReader(m.content)), int64(len(m.content)), nil
}

// ----------------------------------------------------------------------------
// 单元测试
// ----------------------------------------------------------------------------

func TestTaskProcessor_ProcessDocumentTask(t *testing.T) {
	ctx := context.Background()

	orgID := uuid.New()
	kbID := uuid.New()
	docID := uuid.New()
	versionID := uuid.New()

	doc := &domain.Document{
		ID:              docID,
		OrganizationID:  orgID,
		KnowledgeBaseID: kbID,
		Name:            "用户手册.md",
		MimeType:        "text/markdown",
		Status:          domain.DocStatusUploaded,
		Progress:        0,
		CurrentVersion: &domain.DocumentVersion{
			ID:         versionID,
			DocumentID: docID,
			Version:    1,
			ObjectKey:  "org/kb/doc.md",
		},
	}

	version := doc.CurrentVersion
	docRepo := &mockDocRepo{doc: doc, version: version}
	chunkRepo := &mockChunkRepo{}
	storageClient := &mockStorage{
		content: "# 欢迎使用 AtlasDesk\n\nAtlasDesk 提供了智能知识库问答功能。\n\n## 功能亮点\n支持多格式解析与高维向量切片检索。",
	}
	embedder := embedding.NewMockEmbedder()

	processor := job.NewTaskProcessor(
		docRepo,
		chunkRepo,
		storageClient,
		embedder,
		domain.ChunkOptions{TargetTokens: 100, OverlapTokens: 20},
	)

	payload := domain.DocumentProcessPayload{
		OrganizationID: orgID.String(),
		DocumentID:     docID.String(),
		VersionID:      versionID.String(),
		TraceID:        "trace-123456",
	}
	payloadBytes, _ := json.Marshal(payload)
	task := asynq.NewTask(job.TypeDocumentProcess, payloadBytes)

	// 执行任务
	err := processor.ProcessDocumentTask(ctx, task)
	if err != nil {
		t.Fatalf("预期任务执行成功，实际报错: %v", err)
	}

	// 校验状态流转结果
	if doc.Status != domain.DocStatusReady {
		t.Errorf("文档状态预期为 READY，实际为: %s", doc.Status)
	}
	if doc.Progress != 100 {
		t.Errorf("文档进度预期为 100，实际为: %d", doc.Progress)
	}
	if doc.CurrentVersionID == nil || *doc.CurrentVersionID != versionID {
		t.Errorf("文档当前版本指针未更新")
	}

	// 校验切片持久化结果
	if len(chunkRepo.savedChunks) == 0 {
		t.Fatalf("预期保存切片，实际为空")
	}
	for _, chunk := range chunkRepo.savedChunks {
		if chunk.Embedding.Slice() == nil || len(chunk.Embedding.Slice()) != 1536 {
			t.Errorf("切片向量未正确填充 1536 维数据")
		}
	}
}

func TestTaskProcessor_OCRRequired(t *testing.T) {
	ctx := context.Background()

	orgID := uuid.New()
	kbID := uuid.New()
	docID := uuid.New()
	versionID := uuid.New()

	doc := &domain.Document{
		ID:              docID,
		OrganizationID:  orgID,
		KnowledgeBaseID: kbID,
		Name:            "扫描合同.pdf",
		MimeType:        "application/pdf",
		Status:          domain.DocStatusUploaded,
		Progress:        0,
		CurrentVersion: &domain.DocumentVersion{
			ID:         versionID,
			DocumentID: docID,
			Version:    1,
			ObjectKey:  "org/kb/scan.pdf",
		},
	}

	docRepo := &mockDocRepo{doc: doc, version: doc.CurrentVersion}
	chunkRepo := &mockChunkRepo{}
	// 空白或无文字层的 PDF 模拟损坏或空文件
	storageClient := &mockStorage{
		content: "",
	}
	embedder := embedding.NewMockEmbedder()

	processor := job.NewTaskProcessor(
		docRepo,
		chunkRepo,
		storageClient,
		embedder,
		domain.ChunkOptions{TargetTokens: 100, OverlapTokens: 20},
	)

	payload := domain.DocumentProcessPayload{
		OrganizationID: orgID.String(),
		DocumentID:     docID.String(),
		VersionID:      versionID.String(),
		TraceID:        "trace-ocr",
	}
	payloadBytes, _ := json.Marshal(payload)
	task := asynq.NewTask(job.TypeDocumentProcess, payloadBytes)

	// 执行任务，应当捕获不可恢复错误并标记 FAILED
	_ = processor.ProcessDocumentTask(ctx, task)

	if doc.Status != domain.DocStatusFailed {
		t.Errorf("异常文档状态预期为 FAILED，实际为: %s", doc.Status)
	}
	if doc.ErrorCode == "" {
		t.Errorf("应当记录具体的错误代码 ErrorCode")
	}
}
