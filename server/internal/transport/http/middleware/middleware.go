package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/service"
	"github.com/gin-gonic/gin"
)

// RequestID 中间件: 生成或透传 X-Request-ID 并注入上下文与响应头
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			b := make([]byte, 12)
			_, _ = rand.Read(b)
			reqID = "req_" + hex.EncodeToString(b)
		}

		c.Set("RequestID", reqID)
		c.Header("X-Request-ID", reqID)
		c.Next()
	}
}

// Recovery 中间件: 捕获全局 panic，输出结构化告警日志并返回标准 500 错误
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				reqID, _ := c.Get("RequestID")
				slog.Error("处理请求发生严重恐慌 (panic recovered)", "request_id", reqID, "panic", r)

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{
						"code":    "INTERNAL_SERVER_ERROR",
						"message": "服务器发生内部错误，请稍后重试",
					},
					"request_id": reqID,
				})
			}
		}()
		c.Next()
	}
}

// CORS 中间件: 跨域资源共享配置
func CORS(allowedOrigins []string) gin.HandlerFunc {
	originMap := make(map[string]bool)
	for _, o := range allowedOrigins {
		originMap[strings.TrimSpace(o)] = true
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		// 如果在允许名单中或允许全部
		if origin != "" && (originMap[origin] || originMap["*"] || len(allowedOrigins) == 0) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Request-ID, X-Organization-ID")
			c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type, X-Request-ID")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// SecurityHeaders 中间件: 注入基础安全响应头
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

// AccessLog 中间件: 结构化输出 API 访问日志
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		reqID, _ := c.Get("RequestID")

		if raw != "" {
			path = path + "?" + raw
		}

		slog.Info("HTTP 请求",
			"request_id", reqID,
			"status", statusCode,
			"method", method,
			"path", path,
			"ip", clientIP,
			"latency_ms", latency.Milliseconds(),
		)
	}
}

// simpleRateLimiter 简单内存漏桶限流器（按 IP 或 Token 限流）
type simpleRateLimiter struct {
	mu      sync.Mutex
	records map[string][]time.Time
	limit   int
	window  time.Duration
}

var globalLimiter = &simpleRateLimiter{
	records: make(map[string][]time.Time),
	limit:   100, // 默认每分钟 100 次
	window:  time.Minute,
}

// RateLimit 中间件: 限流防护
func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		reqID, _ := c.Get("RequestID")

		globalLimiter.mu.Lock()
		now := time.Now()
		times, exists := globalLimiter.records[key]
		if !exists {
			globalLimiter.records[key] = []time.Time{now}
			globalLimiter.mu.Unlock()
			c.Next()
			return
		}

		// 移除超出窗口的历史请求
		threshold := now.Add(-globalLimiter.window)
		var valid []time.Time
		for _, t := range times {
			if t.After(threshold) {
				valid = append(valid, t)
			}
		}

		if len(valid) >= globalLimiter.limit {
			globalLimiter.records[key] = valid
			globalLimiter.mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "请求过于频繁，请稍后再试",
				},
				"request_id": reqID,
			})
			return
		}

		valid = append(valid, now)
		globalLimiter.records[key] = valid
		globalLimiter.mu.Unlock()

		c.Next()
	}
}

// Authentication 中间件: 解析 Bearer JWT Token 并校验
func Authentication(authService service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID, _ := c.Get("RequestID")
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "未登录或缺少身份令牌",
				},
				"request_id": reqID,
			})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := authService.ParseToken(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "INVALID_TOKEN",
					"message": "身份令牌已失效或非法",
				},
				"request_id": reqID,
			})
			return
		}

		// 将已认证信息注入上下文
		c.Set("CurrentUserID", claims.UserID)
		c.Set("CurrentOrgID", claims.OrganizationID)
		c.Set("CurrentUserClaims", claims)

		c.Next()
	}
}

// OrganizationContext 中间件: 校验并锁定租户多组织上下文，杜绝跨租户越权
func OrganizationContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID, _ := c.Get("RequestID")
		claimsVal, exists := c.Get("CurrentUserClaims")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "未登录",
				},
				"request_id": reqID,
			})
			return
		}

		claims := claimsVal.(*domain.UserClaims)

		// 检查是否有外部传入的目标组织请求头 X-Organization-ID
		targetOrgID := c.GetHeader("X-Organization-ID")
		if targetOrgID != "" && targetOrgID != claims.OrganizationID {
			// 超级管理员可跨租户访问，否则严禁跨租户越权
			if claims.RoleName != "超级管理员" && claims.RoleName != "super_admin" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": gin.H{
						"code":    "CROSS_ORGANIZATION_FORBIDDEN",
						"message": "无权访问其他企业租户的数据",
					},
					"request_id": reqID,
				})
				return
			}
		}

		c.Next()
	}
}

// RequirePermission 中间件: 声明式权限检查
func RequirePermission(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID, _ := c.Get("RequestID")
		claimsVal, exists := c.Get("CurrentUserClaims")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "未登录",
				},
				"request_id": reqID,
			})
			return
		}

		claims := claimsVal.(*domain.UserClaims)

		// 超级管理员自动具备所有权限
		if claims.RoleName == "超级管理员" || claims.RoleName == "super_admin" {
			c.Next()
			return
		}

		hasPerm := false
		for _, p := range claims.Permissions {
			if p == code {
				hasPerm = true
				break
			}
		}

		if !hasPerm {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"code":    "FORBIDDEN",
					"message": "当前角色缺少该操作权限: " + code,
				},
				"request_id": reqID,
			})
			return
		}

		c.Next()
	}
}
