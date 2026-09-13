package handler

import (
	"errors"
	"net/http"
	"strconv"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// KnowledgeHandler 知识库与文档 HTTP 控制器
type KnowledgeHandler struct {
	knowledgeService service.KnowledgeService
}

// NewKnowledgeHandler 构造 KnowledgeHandler 实例
func NewKnowledgeHandler(knowledgeService service.KnowledgeService) *KnowledgeHandler {
	return &KnowledgeHandler{knowledgeService: knowledgeService}
}

// ListKnowledgeBases 获取知识库列表
// GET /api/v1/knowledge-bases
func (h *KnowledgeHandler) ListKnowledgeBases(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	kbs, err := h.knowledgeService.ListKnowledgeBases(c.Request.Context(), orgUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": kbs, "request_id": reqID})
}

// CreateKnowledgeBase 创建知识库
// POST /api/v1/knowledge-bases
func (h *KnowledgeHandler) CreateKnowledgeBase(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	var req domain.CreateKnowledgeBaseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_REQUEST", "message": "参数格式错误: 知识库名称必填"}, "request_id": reqID})
		return
	}

	kb, err := h.knowledgeService.CreateKnowledgeBase(c.Request.Context(), orgUID, &req)
	if err != nil {
		if errors.Is(err, service.ErrKnowledgeBaseNameDuplicate) {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "KB_NAME_DUPLICATE", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": kb, "request_id": reqID})
}

// GetKnowledgeBase 获取知识库详情
// GET /api/v1/knowledge-bases/:id
func (h *KnowledgeHandler) GetKnowledgeBase(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	idStr := c.Param("id")
	kbID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的知识库 ID"}, "request_id": reqID})
		return
	}

	kb, err := h.knowledgeService.GetKnowledgeBase(c.Request.Context(), orgUID, kbID)
	if err != nil {
		if errors.Is(err, service.ErrKnowledgeBaseNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": kb, "request_id": reqID})
}

// UpdateKnowledgeBase 更新知识库
// PATCH /api/v1/knowledge-bases/:id
func (h *KnowledgeHandler) UpdateKnowledgeBase(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	kbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的知识库 ID"}, "request_id": reqID})
		return
	}

	var req domain.UpdateKnowledgeBaseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_REQUEST", "message": "参数格式错误"}, "request_id": reqID})
		return
	}

	kb, err := h.knowledgeService.UpdateKnowledgeBase(c.Request.Context(), orgUID, kbID, &req)
	if err != nil {
		if errors.Is(err, service.ErrKnowledgeBaseNameDuplicate) {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "KB_NAME_DUPLICATE", "message": err.Error()}, "request_id": reqID})
			return
		}
		if errors.Is(err, service.ErrKnowledgeBaseNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": kb, "request_id": reqID})
}

// DeleteKnowledgeBase 软删除知识库
// DELETE /api/v1/knowledge-bases/:id
func (h *KnowledgeHandler) DeleteKnowledgeBase(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	kbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的知识库 ID"}, "request_id": reqID})
		return
	}

	if err := h.knowledgeService.DeleteKnowledgeBase(c.Request.Context(), orgUID, kbID); err != nil {
		if errors.Is(err, service.ErrKnowledgeBaseNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "知识库已成功删除"}, "request_id": reqID})
}

// ListDocuments 获取文档列表 (支持筛选与游标分页)
// GET /api/v1/documents
func (h *KnowledgeHandler) ListDocuments(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	filter := domain.DocumentFilter{
		KnowledgeBaseID: c.Query("knowledge_base_id"),
		Query:           c.Query("q"),
		Type:            c.Query("type"),
		Status:          c.Query("status"),
		Cursor:          c.Query("cursor"),
		Limit:           limit,
	}

	page, err := h.knowledgeService.ListDocuments(c.Request.Context(), orgUID, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": page, "request_id": reqID})
}

// RequestUpload 申请预签名直传凭证
// POST /api/v1/documents/uploads
func (h *KnowledgeHandler) RequestUpload(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	var req domain.DocumentUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_REQUEST", "message": "参数格式错误"}, "request_id": reqID})
		return
	}

	resp, err := h.knowledgeService.RequestUpload(c.Request.Context(), orgUID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUnsupportedFileType):
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "DOCUMENT_TYPE_NOT_SUPPORTED", "message": err.Error()}, "request_id": reqID})
		case errors.Is(err, service.ErrFileSizeExceeded):
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "FILE_TOO_LARGE", "message": err.Error()}, "request_id": reqID})
		case errors.Is(err, service.ErrKnowledgeBaseNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "KB_NOT_FOUND", "message": err.Error()}, "request_id": reqID})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": resp, "request_id": reqID})
}

// CompleteUpload 确认上传完成
// POST /api/v1/documents/:id/complete-upload
func (h *KnowledgeHandler) CompleteUpload(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的文档 ID"}, "request_id": reqID})
		return
	}

	var req domain.CompleteUploadReq
	_ = c.ShouldBindJSON(&req)

	doc, err := h.knowledgeService.CompleteUpload(c.Request.Context(), orgUID, docID, &req)
	if err != nil {
		if errors.Is(err, service.ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "COMPLETE_UPLOAD_FAILED", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": doc, "request_id": reqID})
}

// GetDocument 获取文档详情
// GET /api/v1/documents/:id
func (h *KnowledgeHandler) GetDocument(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的文档 ID"}, "request_id": reqID})
		return
	}

	doc, err := h.knowledgeService.GetDocument(c.Request.Context(), orgUID, docID)
	if err != nil {
		if errors.Is(err, service.ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": doc, "request_id": reqID})
}

// GetDocumentStatus 查询文档处理状态 (用于轮询)
// GET /api/v1/documents/:id/status
func (h *KnowledgeHandler) GetDocumentStatus(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的文档 ID"}, "request_id": reqID})
		return
	}

	doc, err := h.knowledgeService.GetDocument(c.Request.Context(), orgUID, docID)
	if err != nil {
		if errors.Is(err, service.ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":            doc.ID,
			"status":        doc.Status,
			"progress":      doc.Progress,
			"error_code":    doc.ErrorCode,
			"error_message": doc.ErrorMessage,
			"updated_at":    doc.UpdatedAt,
		},
		"request_id": reqID,
	})
}

// DeleteDocument 软删除文档
// DELETE /api/v1/documents/:id
func (h *KnowledgeHandler) DeleteDocument(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的文档 ID"}, "request_id": reqID})
		return
	}

	if err := h.knowledgeService.DeleteDocument(c.Request.Context(), orgUID, docID); err != nil {
		if errors.Is(err, service.ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "文档已成功删除"}, "request_id": reqID})
}

// ReprocessDocument 重新触发文档解析与向量化处理
// POST /api/v1/documents/:id/reprocess
func (h *KnowledgeHandler) ReprocessDocument(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的文档 ID"}, "request_id": reqID})
		return
	}

	if err := h.knowledgeService.ReprocessDocument(c.Request.Context(), orgUID, docID); err != nil {
		if errors.Is(err, service.ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "REPROCESS_FAILED", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "已重新提交处理任务"}, "request_id": reqID})
}

// GetDownloadURL 生成原文档只读临时下载链接
// GET /api/v1/documents/:id/download
func (h *KnowledgeHandler) GetDownloadURL(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的文档 ID"}, "request_id": reqID})
		return
	}

	downloadURL, err := h.knowledgeService.GetDownloadURL(c.Request.Context(), orgUID, docID)
	if err != nil {
		if errors.Is(err, service.ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DOWNLOAD_URL_FAILED", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"download_url": downloadURL}, "request_id": reqID})
}

// RenameDocument 重命名文档
// PATCH /api/v1/documents/:id/rename
func (h *KnowledgeHandler) RenameDocument(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的文档 ID"}, "request_id": reqID})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required,max=255"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_REQUEST", "message": "参数格式错误: name 必填且不超过 255 字符"}, "request_id": reqID})
		return
	}

	doc, err := h.knowledgeService.RenameDocument(c.Request.Context(), orgUID, docID, req.Name)
	if err != nil {
		if errors.Is(err, service.ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "RENAME_FAILED", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": doc, "request_id": reqID})
}

// CreateNewVersion 创建新物理版本
// POST /api/v1/documents/:id/versions
func (h *KnowledgeHandler) CreateNewVersion(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的文档 ID"}, "request_id": reqID})
		return
	}

	var req domain.DocumentUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_REQUEST", "message": "参数格式错误"}, "request_id": reqID})
		return
	}

	resp, err := h.knowledgeService.CreateNewVersion(c.Request.Context(), orgUID, docID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFileSizeExceeded):
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "FILE_TOO_LARGE", "message": err.Error()}, "request_id": reqID})
		case errors.Is(err, service.ErrDocumentNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": resp, "request_id": reqID})
}

// GetDocumentChunks 查询文档当前版本的切片列表
// GET /api/v1/documents/:id/chunks
func (h *KnowledgeHandler) GetDocumentChunks(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的文档 ID"}, "request_id": reqID})
		return
	}

	chunks, err := h.knowledgeService.GetDocumentChunks(c.Request.Context(), orgUID, docID)
	if err != nil {
		if errors.Is(err, service.ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "GET_CHUNKS_FAILED", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": chunks, "request_id": reqID})
}


func getOrgUUID(c *gin.Context) (uuid.UUID, bool) {
	orgVal, exists := c.Get("CurrentOrgID")
	if !exists {
		reqID, _ := c.Get("RequestID")
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "缺少租户组织上下文"}, "request_id": reqID})
		return uuid.Nil, false
	}
	u, err := uuid.Parse(orgVal.(string))
	if err != nil {
		reqID, _ := c.Get("RequestID")
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ORG_ID", "message": "租户组织 ID 格式错误"}, "request_id": reqID})
		return uuid.Nil, false
	}
	return u, true
}
