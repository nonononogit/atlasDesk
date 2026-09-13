package http

import (
	"atlasdesk/internal/config"
	"atlasdesk/internal/service"
	"atlasdesk/internal/transport/http/handler"
	"atlasdesk/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// RouterDeps 定义路由引擎组装所需的依赖项
type RouterDeps struct {
	Config           *config.Config
	HealthHandler    *handler.HealthHandler
	AuthHandler      *handler.AuthHandler
	AuthService      service.AuthService
	KnowledgeHandler *handler.KnowledgeHandler
}

// NewRouter 创建并初始化符合规范第 6.2 节的 Gin 路由引擎
func NewRouter(deps *RouterDeps) *gin.Engine {
	if deps.Config.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 1-6 层核心中间件按序链式挂载 (RequestID -> Recovery -> CORS -> SecurityHeaders -> AccessLog -> RateLimit)
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS(deps.Config.App.AllowedOrigins))
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.AccessLog())
	r.Use(middleware.RateLimit())

	// 注册全局健康检查探针 (开放访问)
	if deps.HealthHandler != nil {
		r.GET("/health/live", deps.HealthHandler.Live)
		r.GET("/health/ready", deps.HealthHandler.Ready)
	}

	// API 统一前缀分组 (/api/v1)
	apiV1 := r.Group("/api/v1")
	{
		// 基础连通性心跳
		apiV1.GET("/ping", func(c *gin.Context) {
			reqID, _ := c.Get("RequestID")
			c.JSON(200, gin.H{
				"message":    "pong",
				"request_id": reqID,
			})
		})

		// 认证公开端点
		if deps.AuthHandler != nil {
			authGroup := apiV1.Group("/auth")
			{
				authGroup.POST("/login", deps.AuthHandler.Login)
				authGroup.POST("/refresh", deps.AuthHandler.Refresh)
				authGroup.POST("/logout", deps.AuthHandler.Logout)
			}
		}

		// 受保护接口分组: 挂载第 7-8 层中间件 (Authentication -> OrganizationContext)
		if deps.AuthService != nil {
			protected := apiV1.Group("")
			protected.Use(middleware.Authentication(deps.AuthService))
			protected.Use(middleware.OrganizationContext())
			{
				// 获取当前用户身份与组织权限
				if deps.AuthHandler != nil {
					protected.GET("/auth/me", deps.AuthHandler.Me)
				}

				// 知识库与文档路由 (受 RBAC 权限控制)
				if deps.KnowledgeHandler != nil {
					// 知识库只读路由
					kbRead := protected.Group("/knowledge-bases", middleware.RequirePermission("knowledge:read"))
					{
						kbRead.GET("", deps.KnowledgeHandler.ListKnowledgeBases)
						kbRead.GET("/:id", deps.KnowledgeHandler.GetKnowledgeBase)
					}

					// 知识库管理写路由
					kbWrite := protected.Group("/knowledge-bases", middleware.RequirePermission("knowledge:write"))
					{
						kbWrite.POST("", deps.KnowledgeHandler.CreateKnowledgeBase)
						kbWrite.PATCH("/:id", deps.KnowledgeHandler.UpdateKnowledgeBase)
						kbWrite.DELETE("/:id", deps.KnowledgeHandler.DeleteKnowledgeBase)
					}

					// 文档只读路由
					docRead := protected.Group("/documents", middleware.RequirePermission("knowledge:read"))
					{
						docRead.GET("", deps.KnowledgeHandler.ListDocuments)
						docRead.GET("/:id", deps.KnowledgeHandler.GetDocument)
						docRead.GET("/:id/status", deps.KnowledgeHandler.GetDocumentStatus)
					}

					// 文档直传与管理写路由
					docWrite := protected.Group("/documents", middleware.RequirePermission("knowledge:write"))
					{
						docWrite.POST("/uploads", deps.KnowledgeHandler.RequestUpload)
						docWrite.POST("/:id/complete-upload", deps.KnowledgeHandler.CompleteUpload)
						docWrite.DELETE("/:id", deps.KnowledgeHandler.DeleteDocument)
					}
				}
			}
		}
	}

	return r
}
