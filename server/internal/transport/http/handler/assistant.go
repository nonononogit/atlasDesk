package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/platform/rag"
	"atlasdesk/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AssistantHandler RAG 智能助手 HTTP 控制器
type AssistantHandler struct {
	convService service.ConversationService
	ragEngine   *rag.Engine
}

// NewAssistantHandler 构造 AssistantHandler 实例
func NewAssistantHandler(convService service.ConversationService, ragEngine *rag.Engine) *AssistantHandler {
	return &AssistantHandler{
		convService: convService,
		ragEngine:   ragEngine,
	}
}

// ListConversations 获取当前租户下当前用户的会话历史列表 (游标分页)
// GET /api/v1/conversations
func (h *AssistantHandler) ListConversations(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}
	userUID, ok := getUserUUID(c)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	cursor := c.Query("cursor")

	page, err := h.convService.ListConversations(c.Request.Context(), orgUID, userUID, cursor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": page, "request_id": reqID})
}

// CreateConversation 创建新会话
// POST /api/v1/conversations
func (h *AssistantHandler) CreateConversation(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}
	userUID, ok := getUserUUID(c)
	if !ok {
		return
	}

	var req domain.CreateConversationReq
	_ = c.ShouldBindJSON(&req)

	conv, err := h.convService.GetOrCreateConversation(c.Request.Context(), orgUID, userUID, nil, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": conv, "request_id": reqID})
}

// GetConversation 获取会话详情
// GET /api/v1/conversations/:id
func (h *AssistantHandler) GetConversation(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的会话 ID"}, "request_id": reqID})
		return
	}

	conv, err := h.convService.GetConversation(c.Request.Context(), orgUID, convID)
	if err != nil {
		if errors.Is(err, service.ErrConversationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": conv, "request_id": reqID})
}

// DeleteConversation 软删除会话
// DELETE /api/v1/conversations/:id
func (h *AssistantHandler) DeleteConversation(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的会话 ID"}, "request_id": reqID})
		return
	}

	if err := h.convService.DeleteConversation(c.Request.Context(), orgUID, convID); err != nil {
		if errors.Is(err, service.ErrConversationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "会话已成功删除"}, "request_id": reqID})
}

// ListMessages 获取指定会话的历史消息列表
// GET /api/v1/conversations/:id/messages
func (h *AssistantHandler) ListMessages(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	convID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的会话 ID"}, "request_id": reqID})
		return
	}

	msgs, err := h.convService.ListMessages(c.Request.Context(), orgUID, convID)
	if err != nil {
		if errors.Is(err, service.ErrConversationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": msgs, "request_id": reqID})
}

// SendMessageStream 发起流式问答对话 (符合 A-03 规范)
// POST /api/v1/conversations/:id/messages/stream
func (h *AssistantHandler) SendMessageStream(c *gin.Context) {
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}
	userUID, ok := getUserUUID(c)
	if !ok {
		return
	}

	paramID := c.Param("id")
	var targetConvID *uuid.UUID
	if paramID != "" && paramID != "new" {
		if uid, err := uuid.Parse(paramID); err == nil {
			targetConvID = &uid
		}
	}

	var req domain.SendMessageStreamReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_REQUEST", "message": "提问内容不能为空且不能超过 2000 字"}})
		return
	}

	// 1. 获取或首次提问懒创建会话记录 (A-02)
	conv, err := h.convService.GetOrCreateConversation(c.Request.Context(), orgUID, userUID, targetConvID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "CONVERSATION_CREATE_FAILED", "message": err.Error()}})
		return
	}

	// 解析可选限定的知识库 ID 列表
	var kbUUIDs []uuid.UUID
	for _, idStr := range req.KnowledgeBaseIDs {
		if kbUID, err := uuid.Parse(idStr); err == nil {
			kbUUIDs = append(kbUUIDs, kbUID)
		}
	}

	// 2. 配置标准 HTTP chunked SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no") // 禁用 Nginx 反向代理缓冲

	eventChan := make(chan rag.StreamEvent, 100)

	// 3. 后台启动 RAG 引擎
	go func() {
		_ = h.ragEngine.StreamChat(c.Request.Context(), orgUID, userUID, conv.ID, req.Content, kbUUIDs, eventChan)
	}()

	// 4. 读取事件通道并实时推送到客户端
	c.Stream(func(w io.Writer) bool {
		event, open := <-eventChan
		if !open {
			return false
		}
		c.SSEvent(event.Event, event.Data)
		c.Writer.Flush()
		return true
	})
}

// GetCitation 获取单条引用溯源详情
// GET /api/v1/citations/:id
func (h *AssistantHandler) GetCitation(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}

	citationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的引用 ID"}, "request_id": reqID})
		return
	}

	citation, err := h.convService.GetCitation(c.Request.Context(), orgUID, citationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": citation, "request_id": reqID})
}

// SubmitFeedback 提交回答反馈 (有帮助/无帮助)
// POST /api/v1/messages/:id/feedback
func (h *AssistantHandler) SubmitFeedback(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	orgUID, ok := getOrgUUID(c)
	if !ok {
		return
	}
	userUID, ok := getUserUUID(c)
	if !ok {
		return
	}

	msgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "无效的消息 ID"}, "request_id": reqID})
		return
	}

	var req domain.MessageFeedbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_REQUEST", "message": "参数格式错误: rating 必须为 1 或 -1"}, "request_id": reqID})
		return
	}

	if err := h.convService.SubmitFeedback(c.Request.Context(), orgUID, userUID, msgID, req.Rating, req.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "FEEDBACK_FAILED", "message": err.Error()}, "request_id": reqID})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "反馈提交成功"}, "request_id": reqID})
}

func getUserUUID(c *gin.Context) (uuid.UUID, bool) {
	userVal, exists := c.Get("CurrentUserID")
	if !exists {
		reqID, _ := c.Get("RequestID")
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "用户未登录"}, "request_id": reqID})
		return uuid.Nil, false
	}
	u, err := uuid.Parse(userVal.(string))
	if err != nil {
		reqID, _ := c.Get("RequestID")
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_USER_ID", "message": "用户 ID 格式错误"}, "request_id": reqID})
		return uuid.Nil, false
	}
	return u, true
}
