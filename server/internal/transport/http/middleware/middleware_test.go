package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"atlasdesk/internal/domain"
	"atlasdesk/internal/service"
	"atlasdesk/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockAuthSvc 用于中间件测试的轻量 Auth 桩
type mockAuthSvc struct {
	tokenMap map[string]*domain.UserClaims
}

func (m *mockAuthSvc) Login(ctx context.Context, email, password string) (*service.LoginResult, error) {
	return nil, nil
}
func (m *mockAuthSvc) Refresh(ctx context.Context, rawRefreshToken string) (*service.RefreshResult, error) {
	return nil, nil
}
func (m *mockAuthSvc) Logout(ctx context.Context, rawRefreshToken string) error {
	return nil
}
func (m *mockAuthSvc) GetMe(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) (*domain.AuthUserDTO, error) {
	return nil, nil
}
func (m *mockAuthSvc) ParseToken(tokenStr string) (*domain.UserClaims, error) {
	if claims, ok := m.tokenMap[tokenStr]; ok {
		return claims, nil
	}
	return nil, service.ErrInvalidCredentials
}

// TestMiddleware_Chain 验证未登录 401、跨组织越权 403、权限不足 403 及通过场景
func TestMiddleware_Chain(t *testing.T) {
	authSvc := &mockAuthSvc{
		tokenMap: make(map[string]*domain.UserClaims),
	}

	org1 := "org_11111111"
	org2 := "org_22222222"

	// 普通客服坐席
	authSvc.tokenMap["agent_token"] = &domain.UserClaims{
		UserID:         "user_agent",
		Email:          "agent@test.com",
		OrganizationID: org1,
		RoleName:       "客服坐席",
		Permissions:    []string{"ticket:read"},
	}

	// 超级管理员
	authSvc.tokenMap["admin_token"] = &domain.UserClaims{
		UserID:         "user_admin",
		Email:          "admin@test.com",
		OrganizationID: org1,
		RoleName:       "超级管理员",
		Permissions:    []string{"*"},
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Recovery())
	r.Use(middleware.SecurityHeaders())

	protected := r.Group("/api/v1")
	protected.Use(middleware.Authentication(authSvc))
	protected.Use(middleware.OrganizationContext())
	{
		// 读工单接口 (需要 ticket:read)
		protected.GET("/tickets", middleware.RequirePermission("ticket:read"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// 写工单接口 (需要 ticket:create)
		protected.POST("/tickets", middleware.RequirePermission("ticket:create"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "created"})
		})
	}

	// 1. 验证未登录访问受保护接口 -> 401 UNAUTHORIZED
	t.Run("未登录返回 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/tickets", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("预期 401，实际返回: %d", w.Code)
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Errorf("预期响应包含 X-Request-ID")
		}
	})

	// 2. 验证合法 Token 访问已有权限接口 -> 200 OK
	t.Run("合法 Token 访问已有权限接口返回 200", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/tickets", nil)
		req.Header.Set("Authorization", "Bearer agent_token")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("预期 200，实际返回: %d", w.Code)
		}
	})

	// 3. 验证无权操作被拦截 -> 403 FORBIDDEN
	t.Run("无权创建工单返回 403", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/tickets", nil)
		req.Header.Set("Authorization", "Bearer agent_token")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("预期 403，实际返回: %d, body: %s", w.Code, w.Body.String())
		}
	})

	// 4. 验证跨组织越权拦截 -> 403 CROSS_ORGANIZATION_FORBIDDEN
	t.Run("普通坐席尝试跨租户访问返回 403", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/tickets", nil)
		req.Header.Set("Authorization", "Bearer agent_token")
		req.Header.Set("X-Organization-ID", org2) // 尝试访问 org2
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("跨租户访问预期 403，实际返回: %d", w.Code)
		}
	})

	// 5. 验证超级管理员跨租户管理允许访问
	t.Run("超级管理员跨租户允许访问", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/tickets", nil)
		req.Header.Set("Authorization", "Bearer admin_token")
		req.Header.Set("X-Organization-ID", org2)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("超级管理员预期 200，实际返回: %d", w.Code)
		}
	})
}
