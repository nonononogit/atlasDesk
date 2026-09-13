package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"atlasdesk/internal/transport/http/handler"
	"github.com/gin-gonic/gin"
)

// mockChecker 用于单元测试的健康检查模拟桩
type mockChecker struct {
	err error
}

// PingContext 模拟连通性探测
func (m *mockChecker) PingContext(ctx context.Context) error {
	return m.err
}

func init() {
	// 设置测试环境为 Gin 纯净测试模式
	gin.SetMode(gin.TestMode)
}

// setupRouter 辅助函数：构建带健康检查端点的测试路由
func setupRouter(dbChecker handler.Checker, redisChecker handler.Checker) *gin.Engine {
	r := gin.New()
	h := handler.NewHealthHandler(dbChecker, redisChecker)
	r.GET("/health/live", h.Live)
	r.GET("/health/ready", h.Ready)
	return r
}

// TestHealth_Live 验证存活探针 /health/live 始终返回 200 OK
func TestHealth_Live(t *testing.T) {
	router := setupRouter(nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health/live", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("预期状态码 200，实际返回: %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("预期 status 为 'ok'，实际返回: %v", resp["status"])
	}
	if resp["timestamp"] == "" {
		t.Errorf("预期包含 timestamp 字段")
	}
}

// TestHealth_Ready_AllHealthy 验证当数据库和 Redis 均正常时，就绪探针返回 200
func TestHealth_Ready_AllHealthy(t *testing.T) {
	dbMock := &mockChecker{err: nil}
	redisMock := &mockChecker{err: nil}
	router := setupRouter(dbMock, redisMock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("预期状态码 200，实际返回: %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Status     string            `json:"status"`
		Components map[string]string `json:"components"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.Status != "ready" {
		t.Errorf("预期 status 为 'ready'，实际为: %s", resp.Status)
	}
	if resp.Components["database"] != "up" || resp.Components["redis"] != "up" {
		t.Errorf("预期组件状态均为 up，实际为: %+v", resp.Components)
	}
}

// TestHealth_Ready_DBDown 验证当数据库不可用时，ready 探针失败 (503) 但 live 探针依然正常 (200)
func TestHealth_Ready_DBDown(t *testing.T) {
	dbMock := &mockChecker{err: errors.New("connection refused: dial tcp 127.0.0.1:5432")}
	redisMock := &mockChecker{err: nil}
	router := setupRouter(dbMock, redisMock)

	// 1. 验证 ready 失败 (HTTP 503)
	wReady := httptest.NewRecorder()
	reqReady, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
	router.ServeHTTP(wReady, reqReady)

	if wReady.Code != http.StatusServiceUnavailable {
		t.Fatalf("数据库不可用时预期状态码 503，实际返回: %d", wReady.Code)
	}

	var respReady struct {
		Status     string            `json:"status"`
		Components map[string]string `json:"components"`
		Error      map[string]any    `json:"error"`
	}
	if err := json.Unmarshal(wReady.Body.Bytes(), &respReady); err != nil {
		t.Fatalf("解析 ready 失败响应异常: %v", err)
	}

	if respReady.Status != "unhealthy" {
		t.Errorf("预期 status 为 unhealthy，实际为: %s", respReady.Status)
	}
	if respReady.Components["database"] != "down" {
		t.Errorf("预期 database 状态为 down，实际为: %s", respReady.Components["database"])
	}
	if respReady.Components["redis"] != "up" {
		t.Errorf("预期 redis 状态为 up，实际为: %s", respReady.Components["redis"])
	}
	if respReady.Error["code"] != "SERVICE_NOT_READY" {
		t.Errorf("预期错误码 SERVICE_NOT_READY，实际为: %v", respReady.Error["code"])
	}

	// 2. 验证 live 探针不受影响，依然返回 200 OK
	wLive := httptest.NewRecorder()
	reqLive, _ := http.NewRequest(http.MethodGet, "/health/live", nil)
	router.ServeHTTP(wLive, reqLive)

	if wLive.Code != http.StatusOK {
		t.Fatalf("数据库不可用时 live 探针应保持 200，实际返回: %d", wLive.Code)
	}
}

// TestHealth_Ready_RedisDown 验证当 Redis 不可用时，ready 探针失败 (503) 但 live 探针依然正常 (200)
func TestHealth_Ready_RedisDown(t *testing.T) {
	dbMock := &mockChecker{err: nil}
	redisMock := &mockChecker{err: errors.New("i/o timeout")}
	router := setupRouter(dbMock, redisMock)

	wReady := httptest.NewRecorder()
	reqReady, _ := http.NewRequest(http.MethodGet, "/health/ready", nil)
	router.ServeHTTP(wReady, reqReady)

	if wReady.Code != http.StatusServiceUnavailable {
		t.Fatalf("Redis 不可用时预期状态码 503，实际返回: %d", wReady.Code)
	}

	var respReady struct {
		Status     string            `json:"status"`
		Components map[string]string `json:"components"`
	}
	_ = json.Unmarshal(wReady.Body.Bytes(), &respReady)

	if respReady.Components["redis"] != "down" {
		t.Errorf("预期 redis 状态为 down，实际为: %s", respReady.Components["redis"])
	}
	if respReady.Components["database"] != "up" {
		t.Errorf("预期 database 状态为 up，实际为: %s", respReady.Components["database"])
	}

	// live 探针依然正常
	wLive := httptest.NewRecorder()
	reqLive, _ := http.NewRequest(http.MethodGet, "/health/live", nil)
	router.ServeHTTP(wLive, reqLive)
	if wLive.Code != http.StatusOK {
		t.Fatalf("Redis 不可用时 live 探针应保持 200，实际返回: %d", wLive.Code)
	}
}
