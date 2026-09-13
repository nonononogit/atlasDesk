package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/platform/storage"
	"atlasdesk/internal/repository"
	"github.com/google/uuid"
)

// 知识库与文档业务错误定义
var (
	ErrKnowledgeBaseNotFound      = errors.New("知识库不存在或已被删除")
	ErrKnowledgeBaseNameDuplicate = errors.New("同组织下已存在相同名称的知识库")
	ErrDocumentNotFound           = errors.New("文档不存在或已被删除")
	ErrUnsupportedFileType        = errors.New("暂不支持该文件类型，仅支持 PDF、DOCX、Markdown 与 TXT")
	ErrFileSizeExceeded           = errors.New("文件大小超出上限（最大限制 50MB）")
)

const MaxDocumentSizeBytes = 50 * 1024 * 1024 // 50MB

// KnowledgeService 知识库与文档业务服务接口
type KnowledgeService interface {
	// 知识库管理
	CreateKnowledgeBase(ctx context.Context, orgID uuid.UUID, req *domain.CreateKnowledgeBaseReq) (*domain.KnowledgeBase, error)
	ListKnowledgeBases(ctx context.Context, orgID uuid.UUID) ([]domain.KnowledgeBase, error)
	GetKnowledgeBase(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*domain.KnowledgeBase, error)
	UpdateKnowledgeBase(ctx context.Context, orgID uuid.UUID, id uuid.UUID, req *domain.UpdateKnowledgeBaseReq) (*domain.KnowledgeBase, error)
	DeleteKnowledgeBase(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error

	// 文档上传与列表
	ListDocuments(ctx context.Context, orgID uuid.UUID, filter domain.DocumentFilter) (*domain.CursorPage[domain.Document], error)
	GetDocument(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*domain.Document, error)
	RequestUpload(ctx context.Context, orgID uuid.UUID, req *domain.DocumentUploadReq) (*domain.DocumentUploadResp, error)
	CompleteUpload(ctx context.Context, orgID uuid.UUID, docID uuid.UUID, req *domain.CompleteUploadReq) (*domain.Document, error)
	DeleteDocument(ctx context.Context, orgID uuid.UUID, docID uuid.UUID) error
}

// DefaultKnowledgeService 知识库业务服务实现
type DefaultKnowledgeService struct {
	repo    repository.KnowledgeRepository
	storage storage.StorageClient
}

// NewKnowledgeService 构造 DefaultKnowledgeService 实例
func NewKnowledgeService(repo repository.KnowledgeRepository, storage storage.StorageClient) *DefaultKnowledgeService {
	return &DefaultKnowledgeService{
		repo:    repo,
		storage: storage,
	}
}

// CreateKnowledgeBase 创建知识库（强制组织内重名校验）
func (s *DefaultKnowledgeService) CreateKnowledgeBase(ctx context.Context, orgID uuid.UUID, req *domain.CreateKnowledgeBaseReq) (*domain.KnowledgeBase, error) {
	// 1. 同组织重名排查
	existing, err := s.repo.GetKnowledgeBaseByName(ctx, orgID, req.Name)
	if err != nil {
		return nil, fmt.Errorf("检查知识库重名失败: %w", err)
	}
	if existing != nil {
		return nil, ErrKnowledgeBaseNameDuplicate
	}

	visibility := req.Visibility
	if visibility == "" {
		visibility = "private"
	}

	kb := &domain.KnowledgeBase{
		OrganizationID: orgID,
		Name:           req.Name,
		Description:    req.Description,
		Visibility:     visibility,
	}

	if err := s.repo.CreateKnowledgeBase(ctx, kb); err != nil {
		return nil, fmt.Errorf("创建知识库失败: %w", err)
	}

	return kb, nil
}

// ListKnowledgeBases 列出组织下的全部知识库
func (s *DefaultKnowledgeService) ListKnowledgeBases(ctx context.Context, orgID uuid.UUID) ([]domain.KnowledgeBase, error) {
	return s.repo.ListKnowledgeBases(ctx, orgID)
}

// GetKnowledgeBase 查询知识库详情
func (s *DefaultKnowledgeService) GetKnowledgeBase(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*domain.KnowledgeBase, error) {
	kb, err := s.repo.GetKnowledgeBaseByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if kb == nil {
		return nil, ErrKnowledgeBaseNotFound
	}
	return kb, nil
}

// UpdateKnowledgeBase 更新知识库信息
func (s *DefaultKnowledgeService) UpdateKnowledgeBase(ctx context.Context, orgID uuid.UUID, id uuid.UUID, req *domain.UpdateKnowledgeBaseReq) (*domain.KnowledgeBase, error) {
	kb, err := s.GetKnowledgeBase(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil && *req.Name != kb.Name {
		// 检查修改后的名称是否已被同组织占用
		existing, err := s.repo.GetKnowledgeBaseByName(ctx, orgID, *req.Name)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != kb.ID {
			return nil, ErrKnowledgeBaseNameDuplicate
		}
		kb.Name = *req.Name
	}

	if req.Description != nil {
		kb.Description = *req.Description
	}
	if req.Visibility != nil {
		kb.Visibility = *req.Visibility
	}

	if err := s.repo.UpdateKnowledgeBase(ctx, kb); err != nil {
		return nil, fmt.Errorf("更新知识库失败: %w", err)
	}

	return kb, nil
}

// DeleteKnowledgeBase 软删除知识库
func (s *DefaultKnowledgeService) DeleteKnowledgeBase(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	kb, err := s.GetKnowledgeBase(ctx, orgID, id)
	if err != nil {
		return err
	}
	return s.repo.DeleteKnowledgeBase(ctx, orgID, kb.ID)
}

// RequestUpload 申请预签名直传凭证（前后端双重白名单与文件大小校验）
func (s *DefaultKnowledgeService) RequestUpload(ctx context.Context, orgID uuid.UUID, req *domain.DocumentUploadReq) (*domain.DocumentUploadResp, error) {
	kbUID, err := uuid.Parse(req.KnowledgeBaseID)
	if err != nil {
		return nil, errors.New("无效的知识库 ID")
	}

	// 1. 验证目标知识库归属
	kb, err := s.GetKnowledgeBase(ctx, orgID, kbUID)
	if err != nil {
		return nil, err
	}

	// 2. 校验文件大小配额
	if req.Size > MaxDocumentSizeBytes {
		return nil, ErrFileSizeExceeded
	}

	// 3. 校验 MIME 类型白名单与文件扩展名
	if !domain.SupportedMimeTypes[req.MimeType] {
		// 兼容通过后缀名判断
		ext := filepath.Ext(req.Name)
		if ext != ".pdf" && ext != ".docx" && ext != ".md" && ext != ".txt" {
			return nil, ErrUnsupportedFileType
		}
	}

	docUID := uuid.New()
	verUID := uuid.New()
	// 生成规范对象存储键: orgs/{org_id}/kbs/{kb_id}/docs/{doc_id}/v1/{filename}
	objectKey := fmt.Sprintf("orgs/%s/kbs/%s/docs/%s/v1/%s", orgID.String(), kb.ID.String(), docUID.String(), req.Name)

	// 4. 签发 MinIO 预签名 PUT URL (15 分钟)
	uploadURL, err := s.storage.PresignUploadURL(ctx, objectKey, 15*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("生成直传凭证失败: %w", err)
	}

	// 5. 初始化 UPLOADING 态文档记录与版本记录
	doc := &domain.Document{
		ID:              docUID,
		OrganizationID:  orgID,
		KnowledgeBaseID: kb.ID,
		Name:            req.Name,
		MimeType:        req.MimeType,
		Size:            req.Size,
		Status:          domain.DocStatusUploading,
		Progress:        0,
	}

	ver := &domain.DocumentVersion{
		ID:          verUID,
		DocumentID:  docUID,
		Version:     1,
		ObjectKey:   objectKey,
		ParseStatus: "PENDING",
	}

	if err := s.repo.CreateDocument(ctx, doc, ver); err != nil {
		return nil, fmt.Errorf("保存文档元数据失败: %w", err)
	}

	return &domain.DocumentUploadResp{
		DocumentID: docUID.String(),
		UploadURL:  uploadURL,
		ObjectKey:  objectKey,
		ExpiresIn:  900,
	}, nil
}

// CompleteUpload 确认上传完成并流转状态至 UPLOADED，支持重复确认幂等控制
func (s *DefaultKnowledgeService) CompleteUpload(ctx context.Context, orgID uuid.UUID, docID uuid.UUID, req *domain.CompleteUploadReq) (*domain.Document, error) {
	doc, err := s.repo.GetDocumentByID(ctx, orgID, docID)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrDocumentNotFound
	}

	// 幂等处理：若已处于 UPLOADED 或后续解析中阶段，直接返回成功，不重复触发任务
	if doc.Status != domain.DocStatusUploading {
		return doc, nil
	}

	ver, err := s.repo.GetDocumentVersion(ctx, doc.ID, 1)
	if err != nil || ver == nil {
		return nil, errors.New("未找到文档版本元数据")
	}

	// 核验对象存储中文件是否存在与大小
	actualSize, _, err := s.storage.StatObject(ctx, ver.ObjectKey)
	if err != nil {
		return nil, fmt.Errorf("对象存储中未找到上传文件，无法确认: %w", err)
	}

	// 状态机流转：UPLOADING -> UPLOADED
	doc.Status = domain.DocStatusUploaded
	doc.Progress = 10
	if req.Checksum != "" {
		doc.Checksum = req.Checksum
	}
	if actualSize > 0 {
		doc.Size = actualSize
	}

	if err := s.repo.UpdateDocument(ctx, doc); err != nil {
		return nil, fmt.Errorf("更新文档上传状态失败: %w", err)
	}

	return doc, nil
}

// ListDocuments 文档游标列表与条件筛选
func (s *DefaultKnowledgeService) ListDocuments(ctx context.Context, orgID uuid.UUID, filter domain.DocumentFilter) (*domain.CursorPage[domain.Document], error) {
	return s.repo.ListDocuments(ctx, orgID, filter)
}

// GetDocument 获取文档详情
func (s *DefaultKnowledgeService) GetDocument(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*domain.Document, error) {
	doc, err := s.repo.GetDocumentByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, ErrDocumentNotFound
	}
	return doc, nil
}

// DeleteDocument 软删除文档
func (s *DefaultKnowledgeService) DeleteDocument(ctx context.Context, orgID uuid.UUID, docID uuid.UUID) error {
	doc, err := s.GetDocument(ctx, orgID, docID)
	if err != nil {
		return err
	}
	return s.repo.DeleteDocument(ctx, orgID, doc.ID)
}
