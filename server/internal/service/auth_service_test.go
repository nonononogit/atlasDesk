package service_test

import (
	"context"
	"testing"
	"time"

	"atlasdesk/internal/config"
	"atlasdesk/internal/domain"
	"atlasdesk/internal/repository"
	"atlasdesk/internal/service"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// mockAuthRepo 内存认证仓储模拟桩
type mockAuthRepo struct {
	users       map[string]*domain.User
	usersByID   map[uuid.UUID]*domain.User
	memberships map[uuid.UUID]*domain.Membership
	roles       map[uuid.UUID]*domain.Role
	permissions map[uuid.UUID][]domain.Permission
	sessions    map[string]*domain.RefreshSession
}

func newMockAuthRepo() *mockAuthRepo {
	return &mockAuthRepo{
		users:       make(map[string]*domain.User),
		usersByID:   make(map[uuid.UUID]*domain.User),
		memberships: make(map[uuid.UUID]*domain.Membership),
		roles:       make(map[uuid.UUID]*domain.Role),
		permissions: make(map[uuid.UUID][]domain.Permission),
		sessions:    make(map[string]*domain.RefreshSession),
	}
}

func (m *mockAuthRepo) FindUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return m.users[email], nil
}

func (m *mockAuthRepo) FindUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return m.usersByID[id], nil
}

func (m *mockAuthRepo) FindMembershipByUserID(ctx context.Context, userID uuid.UUID) (*domain.Membership, error) {
	return m.memberships[userID], nil
}

func (m *mockAuthRepo) GetRoleWithPermissions(ctx context.Context, roleID uuid.UUID) (*domain.Role, []domain.Permission, error) {
	return m.roles[roleID], m.permissions[roleID], nil
}

func (m *mockAuthRepo) CreateRefreshSession(ctx context.Context, session *domain.RefreshSession) error {
	m.sessions[session.TokenHash] = session
	return nil
}

func (m *mockAuthRepo) FindRefreshSessionByHash(ctx context.Context, tokenHash string) (*domain.RefreshSession, error) {
	return m.sessions[tokenHash], nil
}

func (m *mockAuthRepo) RevokeRefreshSession(ctx context.Context, tokenHash string) error {
	if s, ok := m.sessions[tokenHash]; ok {
		now := time.Now().UTC()
		s.RevokedAt = &now
	}
	return nil
}

func (m *mockAuthRepo) UpdateUserLastActive(ctx context.Context, userID uuid.UUID) error {
	if u, ok := m.usersByID[userID]; ok {
		now := time.Now().UTC()
		u.LastActiveAt = &now
	}
	return nil
}

func setupTestAuthService() (*service.DefaultAuthService, *mockAuthRepo) {
	repo := newMockAuthRepo()
	cfg := &config.JWTConfig{
		AccessSecret:       "test_jwt_access_secret_key_12345",
		RefreshSecret:      "test_jwt_refresh_secret_key_12345",
		AccessExpiresMin:   15,
		RefreshExpiresDays: 7,
	}

	// 预置组织、角色、权限与正常用户
	orgID := uuid.New()
	roleID := uuid.New()
	userID := uuid.New()

	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)
	user := &domain.User{
		ID:           userID,
		Email:        "test@example.com",
		PasswordHash: string(passwordHash),
		Name:         "测试用户",
		Status:       "active",
	}
	repo.users[user.Email] = user
	repo.usersByID[user.ID] = user

	// 预置禁用用户
	suspendedUserID := uuid.New()
	suspendedUser := &domain.User{
		ID:           suspendedUserID,
		Email:        "suspended@example.com",
		PasswordHash: string(passwordHash),
		Name:         "已停用用户",
		Status:       "suspended",
	}
	repo.users[suspendedUser.Email] = suspendedUser
	repo.usersByID[suspendedUser.ID] = suspendedUser

	role := &domain.Role{
		ID:   roleID,
		Name: "系统管理员",
	}
	repo.roles[roleID] = role

	perms := []domain.Permission{
		{ID: uuid.New(), Code: "dashboard:read", Name: "查看控制台"},
	}
	repo.permissions[roleID] = perms

	membership := &domain.Membership{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		RoleID:         roleID,
		Status:         "active",
		Organization:   &domain.Organization{ID: orgID, Name: "测试组织"},
		Role:           role,
	}
	repo.memberships[userID] = membership
	repo.memberships[suspendedUserID] = membership

	svc := service.NewAuthService(repo, cfg)
	return svc, repo
}

// TestAuth_Login_Success 验证正常邮箱密码登录成功
func TestAuth_Login_Success(t *testing.T) {
	svc, _ := setupTestAuthService()
	ctx := context.Background()

	res, err := svc.Login(ctx, "test@example.com", "Password123!")
	if err != nil {
		t.Fatalf("预期登录成功，实际返回错误: %v", err)
	}

	if res.AccessToken == "" {
		t.Errorf("预期返回 AccessToken")
	}
	if res.RawRefreshToken == "" {
		t.Errorf("预期返回 RawRefreshToken")
	}
	if res.User.Email != "test@example.com" {
		t.Errorf("预期返回用户 Email 一致")
	}

	// 验证 AccessToken 能被成功解析且 Payload 完整
	claims, err := svc.ParseToken(res.AccessToken)
	if err != nil {
		t.Fatalf("解析生成的 AccessToken 失败: %v", err)
	}
	if claims.Email != "test@example.com" || len(claims.Permissions) != 1 {
		t.Errorf("Claims 内容不匹配: %+v", claims)
	}
}

// TestAuth_Login_PasswordError 验证密码错误时拦截
func TestAuth_Login_PasswordError(t *testing.T) {
	svc, _ := setupTestAuthService()
	ctx := context.Background()

	_, err := svc.Login(ctx, "test@example.com", "WrongPassword!")
	if err != service.ErrInvalidCredentials {
		t.Fatalf("预期返回 ErrInvalidCredentials，实际返回: %v", err)
	}
}

// TestAuth_Login_SuspendedUser 验证已停用用户登录时拦截
func TestAuth_Login_SuspendedUser(t *testing.T) {
	svc, _ := setupTestAuthService()
	ctx := context.Background()

	_, err := svc.Login(ctx, "suspended@example.com", "Password123!")
	if err != service.ErrUserSuspended {
		t.Fatalf("预期返回 ErrUserSuspended，实际返回: %v", err)
	}
}

// TestAuth_Refresh_Success 验证 Refresh 令牌轮换机制
func TestAuth_Refresh_Success(t *testing.T) {
	svc, _ := setupTestAuthService()
	ctx := context.Background()

	loginRes, _ := svc.Login(ctx, "test@example.com", "Password123!")
	oldRefreshToken := loginRes.RawRefreshToken

	refreshRes, err := svc.Refresh(ctx, oldRefreshToken)
	if err != nil {
		t.Fatalf("刷新令牌预期成功，实际返回错误: %v", err)
	}

	if refreshRes.AccessToken == "" || refreshRes.NewRawRefreshToken == "" {
		t.Fatalf("预期返回新 AccessToken 与新 RefreshToken")
	}
	if refreshRes.NewRawRefreshToken == oldRefreshToken {
		t.Errorf("安全轮换机制失效：新旧 RefreshToken 相同")
	}

	// 验证旧 RefreshToken 已被废弃，再次使用应失败
	_, err = svc.Refresh(ctx, oldRefreshToken)
	if err != service.ErrTokenRevoked {
		t.Errorf("使用轮换后的旧 RefreshToken 预期返回 ErrTokenRevoked，实际为: %v", err)
	}
}

// TestAuth_Refresh_Expired 验证过期刷新令牌被拒绝
func TestAuth_Refresh_Expired(t *testing.T) {
	svc, repo := setupTestAuthService()
	ctx := context.Background()

	loginRes, _ := svc.Login(ctx, "test@example.com", "Password123!")
	tokenHash := repository.HashToken(loginRes.RawRefreshToken)

	// 人为调整过期时间到过去
	if session, ok := repo.sessions[tokenHash]; ok {
		session.ExpiresAt = time.Now().UTC().Add(-1 * time.Hour)
	}

	_, err := svc.Refresh(ctx, loginRes.RawRefreshToken)
	if err != service.ErrTokenExpired {
		t.Fatalf("过期令牌预期返回 ErrTokenExpired，实际返回: %v", err)
	}
}

// TestAuth_Logout_And_OldRefresh 验证退出登录后旧会话完全失效
func TestAuth_Logout_And_OldRefresh(t *testing.T) {
	svc, _ := setupTestAuthService()
	ctx := context.Background()

	loginRes, _ := svc.Login(ctx, "test@example.com", "Password123!")
	rawToken := loginRes.RawRefreshToken

	// 执行注销
	if err := svc.Logout(ctx, rawToken); err != nil {
		t.Fatalf("注销登录失败: %v", err)
	}

	// 注销后尝试使用旧 RefreshToken
	_, err := svc.Refresh(ctx, rawToken)
	if err != service.ErrTokenRevoked {
		t.Fatalf("注销后使用旧刷新令牌预期返回 ErrTokenRevoked，实际为: %v", err)
	}
}
