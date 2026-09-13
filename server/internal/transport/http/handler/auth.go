package handler

import (
	"errors"
	"net/http"

	"atlasdesk/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuthHandler 认证路由处理器
type AuthHandler struct {
	authService service.AuthService
}

// NewAuthHandler 构造认证处理器
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// LoginRequest 登录请求参数体
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// Login 账号密码登录
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	reqID, _ := c.Get("RequestID")

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "请提供有效的邮箱与密码",
			},
			"request_id": reqID,
		})
		return
	}

	result, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_CREDENTIALS",
					"message": "邮箱或密码错误",
				},
				"request_id": reqID,
			})
		case errors.Is(err, service.ErrUserSuspended):
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "USER_SUSPENDED",
					"message": "账号已被停用，请联系管理员",
				},
				"request_id": reqID,
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"code":    "INTERNAL_ERROR",
					"message": "登录处理失败，请稍后重试",
				},
				"request_id": reqID,
			})
		}
		return
	}

	// 写入安全的 HttpOnly Cookie
	setRefreshTokenCookie(c, result.RawRefreshToken, 7*24*3600)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"access_token": result.AccessToken,
			"user":         result.User,
		},
		"request_id": reqID,
	})
}

// Refresh 刷新 Access Token 与会话轮换
// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	reqID, _ := c.Get("RequestID")

	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "缺少刷新令牌",
			},
			"request_id": reqID,
		})
		return
	}

	result, err := h.authService.Refresh(c.Request.Context(), refreshToken)
	if err != nil {
		clearRefreshTokenCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "TOKEN_EXPIRED",
				"message": err.Error(),
			},
			"request_id": reqID,
		})
		return
	}

	// 轮换写入新 Refresh Token Cookie
	setRefreshTokenCookie(c, result.NewRawRefreshToken, 7*24*3600)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"access_token": result.AccessToken,
			"user":         result.User,
		},
		"request_id": reqID,
	})
}

// Logout 注销登录会话
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	reqID, _ := c.Get("RequestID")

	refreshToken, _ := c.Cookie("refresh_token")
	if refreshToken != "" {
		_ = h.authService.Logout(c.Request.Context(), refreshToken)
	}

	// 清除 Cookie
	clearRefreshTokenCookie(c)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"message": "已成功退出登录",
		},
		"request_id": reqID,
	})
}

// Me 获取当前登录用户身份详情
// GET /api/v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	reqID, _ := c.Get("RequestID")
	userIDVal, existsUser := c.Get("CurrentUserID")
	orgIDVal, existsOrg := c.Get("CurrentOrgID")

	if !existsUser || !existsOrg {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "请先登录",
			},
			"request_id": reqID,
		})
		return
	}

	userUID, err := uuid.Parse(userIDVal.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_USER_ID", "message": "无效的用户 ID"}, "request_id": reqID})
		return
	}

	orgUID, err := uuid.Parse(orgIDVal.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ORG_ID", "message": "无效的组织 ID"}, "request_id": reqID})
		return
	}

	userDTO, err := h.authService.GetMe(c.Request.Context(), userUID, orgUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"code":    "NOT_FOUND",
				"message": err.Error(),
			},
			"request_id": reqID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"user": userDTO,
		},
		"request_id": reqID,
	})
}

// setRefreshTokenCookie 辅助函数：设置安全 HttpOnly 刷新 Cookie
func setRefreshTokenCookie(c *gin.Context, token string, maxAge int) {
	c.SetCookie(
		"refresh_token",
		token,
		maxAge,
		"/api/v1/auth",
		"",
		false, // 本地开发环境 false，HTTPS 生产环境由 Nginx/TLS 保障
		true,  // HttpOnly: 防止 XSS 脚本窃取
	)
}

// clearRefreshTokenCookie 辅助函数：销毁 Cookie
func clearRefreshTokenCookie(c *gin.Context) {
	c.SetCookie("refresh_token", "", -1, "/api/v1/auth", "", false, true)
}
