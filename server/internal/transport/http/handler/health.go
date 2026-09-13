package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Checker 定义统一的健康探测接口
type Checker interface {
	PingContext(ctx context.Context) error
}

// HealthHandler 健康检查处理器
type HealthHandler struct {
	dbChecker    Checker // 数据库健康检查器
	redisChecker Checker // 缓存健康检查器
}

// NewHealthHandler 构造健康检查处理器实例
func NewHealthHandler(dbChecker Checker, redisChecker Checker) *HealthHandler {
	return &HealthHandler{
		dbChecker:    dbChecker,
		redisChecker: redisChecker,
	}
}

// Live 存活探针 (/health/live)
// 只要服务进程存活且 Gin 正常处理请求，即返回 200 OK
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Ready 就绪探针 (/health/ready)
// 校验数据库和 Redis 连通性；若有组件失败，返回 503 Service Unavailable 并记录日志
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	components := make(map[string]string)
	isHealthy := true

	// 1. 检查数据库连通性
	if h.dbChecker == nil {
		components["database"] = "unconfigured"
		isHealthy = false
	} else if err := h.dbChecker.PingContext(ctx); err != nil {
		slog.Error("就绪检查失败: 数据库不可用", "error", err.Error())
		components["database"] = "down"
		isHealthy = false
	} else {
		components["database"] = "up"
	}

	// 2. 检查 Redis 连通性
	if h.redisChecker == nil {
		components["redis"] = "unconfigured"
		isHealthy = false
	} else if err := h.redisChecker.PingContext(ctx); err != nil {
		slog.Error("就绪检查失败: Redis 不可用", "error", err.Error())
		components["redis"] = "down"
		isHealthy = false
	} else {
		components["redis"] = "up"
	}

	// 3. 构建响应
	now := time.Now().UTC().Format(time.RFC3339)
	if !isHealthy {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":     "unhealthy",
			"timestamp":  now,
			"components": components,
			"error": gin.H{
				"code":    "SERVICE_NOT_READY",
				"message": "依赖的基础设施服务尚未就绪",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "ready",
		"timestamp":  now,
		"components": components,
	})
}
