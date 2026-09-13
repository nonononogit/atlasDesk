package service_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/service"
	"github.com/google/uuid"
)

// mockKnowledgeRepo 知识库与文档内存仓储模拟桩
type mockKnowledgeRepo struct {
	kbs      map[uuid.UUID]*domain.KnowledgeBase
	docs     map[uuid.UUID]*domain.Document
	versions map[uuid.UUID]map[int]*domain.DocumentVersion
}

func newMockKnowledgeRepo() *mockKnowledgeRepo {
	return &mockKnowledgeRepo{
		kbs:      make(map[uuid.UUID]*domain.KnowledgeBase),
		docs:     make(map[uuid.UUID]*domain.Document),
		versions: make(map[uuid.UUID]map[int]*domain.DocumentVersion),
	}
}

func (m *mockKnowledgeRepo) CreateKnowledgeBase(ctx context.Context, kb *domain.KnowledgeBase) error {
	if kb.ID == uuid.Nil {
		kb.ID = uuid.New()
	}
	m.kbs[kb.ID] = kb
	return nil
}

func (m *mockKnowledgeRepo) ListKnowledgeBases(ctx context.Context, orgID uuid.UUID) ([]domain.KnowledgeBase, error) {
	var list []domain.KnowledgeBase
	for _, kb := range m.kbs {
		if kb.OrganizationID == orgID && !kb.DeletedAt.Valid {
			list = append(list, *kb)
		}
	}
	return list, nil
}

func (m *mockKnowledgeRepo) GetKnowledgeBaseByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*domain.KnowledgeBase, error) {
	kb, ok := m.kbs[id]
	if !ok || kb.OrganizationID != orgID || kb.DeletedAt.Valid {
		return nil, nil
	}
	return kb, nil
}

func (m *mockKnowledgeRepo) GetKnowledgeBaseByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.KnowledgeBase, error) {
	for _, kb := range m.kbs {
		if kb.OrganizationID == orgID && kb.Name == name && !kb.DeletedAt.Valid {
			return kb, nil
		}
	}
	return nil, nil
}

func (m *mockKnowledgeRepo) UpdateKnowledgeBase(ctx context.Context, kb *domain.KnowledgeBase) error {
	m.kbs[kb.ID] = kb
	return nil
}

func (m *mockKnowledgeRepo) DeleteKnowledgeBase(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	if kb, ok := m.kbs[id]; ok && kb.OrganizationID == orgID {
		kb.DeletedAt.Time = time.Now()
		kb.DeletedAt.Valid = true
	}
	return nil
}

func (m *mockKnowledgeRepo) CreateDocument(ctx context.Context, doc *domain.Document, ver *domain.DocumentVersion) error {
	m.docs[doc.ID] = doc
	if ver != nil {
		if m.versions[doc.ID] == nil {
			m.versions[doc.ID] = make(map[int]*domain.DocumentVersion)
		}
		m.versions[doc.ID][ver.Version] = ver
	}
	return nil
}

func (m *mockKnowledgeRepo) ListDocuments(ctx context.Context, orgID uuid.UUID, filter domain.DocumentFilter) (*domain.CursorPage[domain.Document], error) {
	var items []domain.Document
	for _, doc := range m.docs {
		if doc.OrganizationID == orgID && !doc.DeletedAt.Valid {
			items = append(items, *doc)
		}
	}
	return &domain.CursorPage[domain.Document]{
		Items:   items,
		HasMore: false,
	}, nil
}

func (m *mockKnowledgeRepo) GetDocumentByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*domain.Document, error) {
	doc, ok := m.docs[id]
	if !ok || doc.OrganizationID != orgID || doc.DeletedAt.Valid {
		return nil, nil
	}
	return doc, nil
}

func (m *mockKnowledgeRepo) UpdateDocument(ctx context.Context, doc *domain.Document) error {
	m.docs[doc.ID] = doc
	return nil
}

func (m *mockKnowledgeRepo) DeleteDocument(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	if doc, ok := m.docs[id]; ok && doc.OrganizationID == orgID {
		doc.DeletedAt.Time = time.Now()
		doc.DeletedAt.Valid = true
	}
	return nil
}

func (m *mockKnowledgeRepo) GetDocumentVersion(ctx context.Context, docID uuid.UUID, version int) (*domain.DocumentVersion, error) {
	if vers, ok := m.versions[docID]; ok {
		return vers[version], nil
	}
	return nil, nil
}

func (m *mockKnowledgeRepo) GetLatestDocumentVersion(ctx context.Context, docID uuid.UUID) (*domain.DocumentVersion, error) {
	if vers, ok := m.versions[docID]; ok {
		maxVer := 0
		var latest *domain.DocumentVersion
		for v, ver := range vers {
			if v > maxVer {
				maxVer = v
				latest = ver
			}
		}
		return latest, nil
	}
	return nil, nil
}

func (m *mockKnowledgeRepo) CreateDocumentVersion(ctx context.Context, ver *domain.DocumentVersion) error {
	if m.versions[ver.DocumentID] == nil {
		m.versions[ver.DocumentID] = make(map[int]*domain.DocumentVersion)
	}
	m.versions[ver.DocumentID][ver.Version] = ver
	return nil
}

// mockStorageClient 对象存储模拟桩
type mockStorageClient struct {
	objects map[string]int64
}

func newMockStorageClient() *mockStorageClient {
	return &mockStorageClient{objects: make(map[string]int64)}
}

func (s *mockStorageClient) PresignUploadURL(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	return "https://storage.local/upload/" + objectKey, nil
}

func (s *mockStorageClient) PresignDownloadURL(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	return "https://storage.local/download/" + objectKey, nil
}

func (s *mockStorageClient) StatObject(ctx context.Context, objectKey string) (int64, string, error) {
	if size, ok := s.objects[objectKey]; ok {
		return size, "mock_etag", nil
	}
	return 0, "", errors.New("nosuchkey: object not found")
}

func (s *mockStorageClient) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, int64, error) {
	content := "mock storage content"
	return io.NopCloser(strings.NewReader(content)), int64(len(content)), nil
}

// TestKnowledgeService_Create_And_DuplicateName 验证知识库多租户重名约束
func TestKnowledgeService_Create_And_DuplicateName(t *testing.T) {
	repo := newMockKnowledgeRepo()
	storageMock := newMockStorageClient()
	svc := service.NewKnowledgeService(repo, storageMock, nil, nil)
	ctx := context.Background()

	org1 := uuid.New()
	org2 := uuid.New()

	// 1. 在 Org1 创建知识库 "产品手册"
	kb1, err := svc.CreateKnowledgeBase(ctx, org1, &domain.CreateKnowledgeBaseReq{
		Name:        "产品手册",
		Description: "系统产品功能文档",
	})
	if err != nil {
		t.Fatalf("创建知识库失败: %v", err)
	}
	if kb1.Name != "产品手册" {
		t.Errorf("预期名称为产品手册")
	}

	// 2. 在 Org1 重复创建 "产品手册"，预期被拒绝
	_, err = svc.CreateKnowledgeBase(ctx, org1, &domain.CreateKnowledgeBaseReq{
		Name: "产品手册",
	})
	if err != service.ErrKnowledgeBaseNameDuplicate {
		t.Fatalf("同组织重名预期返回 ErrKnowledgeBaseNameDuplicate，实际返回: %v", err)
	}

	// 3. 在 Org2 创建同名 "产品手册"，多租户隔离预期允许
	kb2, err := svc.CreateKnowledgeBase(ctx, org2, &domain.CreateKnowledgeBaseReq{
		Name: "产品手册",
	})
	if err != nil {
		t.Fatalf("不同组织同名应允许创建，实际报错: %v", err)
	}
	if kb2.OrganizationID != org2 {
		t.Errorf("组织 ID 校验不一致")
	}
}

// TestKnowledgeService_RequestUpload_MimeAndSize 验证上传凭证申请时的白名单与大小拦截
func TestKnowledgeService_RequestUpload_MimeAndSize(t *testing.T) {
	repo := newMockKnowledgeRepo()
	storageMock := newMockStorageClient()
	svc := service.NewKnowledgeService(repo, storageMock, nil, nil)
	ctx := context.Background()

	orgID := uuid.New()
	kb, _ := svc.CreateKnowledgeBase(ctx, orgID, &domain.CreateKnowledgeBaseReq{Name: "技术知识库"})

	// 1. 合法 PDF 文件申请
	pdfResp, err := svc.RequestUpload(ctx, orgID, &domain.DocumentUploadReq{
		KnowledgeBaseID: kb.ID.String(),
		Name:            "manual.pdf",
		MimeType:        "application/pdf",
		Size:            1024 * 1024, // 1MB
	})
	if err != nil {
		t.Fatalf("合法 PDF 预期申请成功，实际报错: %v", err)
	}
	if pdfResp.UploadURL == "" || pdfResp.DocumentID == "" {
		t.Errorf("响应缺少 upload_url 或 document_id")
	}

	// 2. 非法文件类型拦截 (如 .exe)
	_, err = svc.RequestUpload(ctx, orgID, &domain.DocumentUploadReq{
		KnowledgeBaseID: kb.ID.String(),
		Name:            "dangerous.exe",
		MimeType:        "application/x-msdownload",
		Size:            1024,
	})
	if err != service.ErrUnsupportedFileType {
		t.Fatalf("非法类型预期返回 ErrUnsupportedFileType，实际为: %v", err)
	}

	// 3. 超出 50MB 大小配额拦截
	_, err = svc.RequestUpload(ctx, orgID, &domain.DocumentUploadReq{
		KnowledgeBaseID: kb.ID.String(),
		Name:            "huge_file.pdf",
		MimeType:        "application/pdf",
		Size:            60 * 1024 * 1024, // 60MB
	})
	if err != service.ErrFileSizeExceeded {
		t.Fatalf("超限文件预期返回 ErrFileSizeExceeded，实际为: %v", err)
	}
}

// TestKnowledgeService_CompleteUpload_FlowAndIdempotent 验证确认上传流转与幂等性
func TestKnowledgeService_CompleteUpload_FlowAndIdempotent(t *testing.T) {
	repo := newMockKnowledgeRepo()
	storageMock := newMockStorageClient()
	svc := service.NewKnowledgeService(repo, storageMock, nil, nil)
	ctx := context.Background()

	orgID := uuid.New()
	kb, _ := svc.CreateKnowledgeBase(ctx, orgID, &domain.CreateKnowledgeBaseReq{Name: "测试库"})

	// 申请上传
	uploadResp, _ := svc.RequestUpload(ctx, orgID, &domain.DocumentUploadReq{
		KnowledgeBaseID: kb.ID.String(),
		Name:            "guide.md",
		MimeType:        "text/markdown",
		Size:            2048,
	})

	docUID, _ := uuid.Parse(uploadResp.DocumentID)

	// 1. 文件尚未上传到存储时确认 -> 预期失败
	_, err := svc.CompleteUpload(ctx, orgID, docUID, &domain.CompleteUploadReq{})
	if err == nil {
		t.Fatalf("对象存储中不存在文件时预期确认失败")
	}

	// 2. 模拟文件已直传到存储
	storageMock.objects[uploadResp.ObjectKey] = 2048

	// 确认上传 -> 状态转移为 UPLOADED，进度为 10
	doc, err := svc.CompleteUpload(ctx, orgID, docUID, &domain.CompleteUploadReq{Checksum: "abc123hash"})
	if err != nil {
		t.Fatalf("确认上传预期成功，实际报错: %v", err)
	}
	if doc.Status != domain.DocStatusUploaded || doc.Progress != 10 {
		t.Errorf("状态机流转异常: status=%s, progress=%d", doc.Status, doc.Progress)
	}

	// 3. 重复确认上传 -> 幂等安全返回，不报错
	docIdempotent, err := svc.CompleteUpload(ctx, orgID, docUID, &domain.CompleteUploadReq{})
	if err != nil {
		t.Fatalf("重复确认上传预期幂等成功，实际报错: %v", err)
	}
	if docIdempotent.Status != domain.DocStatusUploaded {
		t.Errorf("幂等返回状态异常: %s", docIdempotent.Status)
	}
}

// TestKnowledgeService_DocumentLifecycle 验证重命名、下载链接、新版本与重处理
func TestKnowledgeService_DocumentLifecycle(t *testing.T) {
	repo := newMockKnowledgeRepo()
	storageMock := newMockStorageClient()
	svc := service.NewKnowledgeService(repo, storageMock, nil, nil)
	ctx := context.Background()

	orgID := uuid.New()
	kb, _ := svc.CreateKnowledgeBase(ctx, orgID, &domain.CreateKnowledgeBaseReq{Name: "生命周期库"})

	uploadResp, _ := svc.RequestUpload(ctx, orgID, &domain.DocumentUploadReq{
		KnowledgeBaseID: kb.ID.String(),
		Name:            "doc1.txt",
		MimeType:        "text/plain",
		Size:            100,
	})
	docID, _ := uuid.Parse(uploadResp.DocumentID)
	storageMock.objects[uploadResp.ObjectKey] = 100
	_, _ = svc.CompleteUpload(ctx, orgID, docID, &domain.CompleteUploadReq{})

	// 1. 重命名测试
	renamedDoc, err := svc.RenameDocument(ctx, orgID, docID, "新名称.txt")
	if err != nil {
		t.Fatalf("重命名失败: %v", err)
	}
	if renamedDoc.Name != "新名称.txt" {
		t.Errorf("重命名结果不符合预期: %s", renamedDoc.Name)
	}

	// 2. 签发下载链接测试
	dlURL, err := svc.GetDownloadURL(ctx, orgID, docID)
	if err != nil {
		t.Fatalf("获取下载链接失败: %v", err)
	}
	if !strings.Contains(dlURL, "download") {
		t.Errorf("下载链接格式异常: %s", dlURL)
	}

	// 3. 创建新版本测试 (预期版本号为 2)
	v2Resp, err := svc.CreateNewVersion(ctx, orgID, docID, &domain.DocumentUploadReq{
		KnowledgeBaseID: kb.ID.String(),
		Name:            "新名称_v2.txt",
		MimeType:        "text/plain",
		Size:            120,
	})
	if err != nil {
		t.Fatalf("创建新版本失败: %v", err)
	}
	if !strings.Contains(v2Resp.ObjectKey, "/v2/") {
		t.Errorf("新版本存储路径未递增版本号: %s", v2Resp.ObjectKey)
	}

	// 4. 重处理测试
	err = svc.ReprocessDocument(ctx, orgID, docID)
	if err != nil {
		t.Fatalf("重处理触发失败: %v", err)
	}
}

