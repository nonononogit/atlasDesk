package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"atlasdesk/internal/config"
	"atlasdesk/internal/domain"
	"atlasdesk/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// 常见认证错误定义
var (
	ErrInvalidCredentials = errors.New("邮箱或密码错误")
	ErrUserSuspended      = errors.New("用户账号已被停用，请联系管理员")
	ErrTokenExpired       = errors.New("刷新令牌已过期，请重新登录")
	ErrTokenRevoked       = errors.New("刷新令牌已注销失效，请重新登录")
	ErrUserNotFound       = errors.New("用户不存在")
	ErrOrgNotFound        = errors.New("用户未加入任何组织")
)

// AuthService 认证业务服务接口
type AuthService interface {
	Login(ctx context.Context, email, password string) (*LoginResult, error)
	Refresh(ctx context.Context, rawRefreshToken string) (*RefreshResult, error)
	Logout(ctx context.Context, rawRefreshToken string) error
	GetMe(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) (*domain.AuthUserDTO, error)
	ParseToken(tokenStr string) (*domain.UserClaims, error)
}

// LoginResult 登录返回结果
type LoginResult struct {
	AccessToken     string              `json:"access_token"`
	RawRefreshToken string              `json:"-"` // 用于写 HttpOnly Cookie
	User            *domain.AuthUserDTO `json:"user"`
}

// RefreshResult 刷新返回结果
type RefreshResult struct {
	AccessToken        string              `json:"access_token"`
	NewRawRefreshToken string              `json:"-"` // 轮换后的新 Refresh Token
	User               *domain.AuthUserDTO `json:"user"`
}

// DefaultAuthService 认证服务实现
type DefaultAuthService struct {
	repo repository.AuthRepository
	cfg  *config.JWTConfig
}

// NewAuthService 构造认证服务实例
func NewAuthService(repo repository.AuthRepository, cfg *config.JWTConfig) *DefaultAuthService {
	return &DefaultAuthService{
		repo: repo,
		cfg:  cfg,
	}
}

// Login 密码认证登录流程
func (s *DefaultAuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	// 1. 查找用户
	user, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	// 2. 校验用户账号状态
	if user.Status != "active" {
		return nil, ErrUserSuspended
	}

	// 3. 校验密码哈希
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// 4. 查询组织与权限信息
	membership, err := s.repo.FindMembershipByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("查询成员关系失败: %w", err)
	}
	if membership == nil || membership.Status != "active" {
		return nil, ErrOrgNotFound
	}

	_, perms, err := s.repo.GetRoleWithPermissions(ctx, membership.RoleID)
	if err != nil {
		return nil, fmt.Errorf("查询角色权限失败: %w", err)
	}

	var permCodes []string
	for _, p := range perms {
		permCodes = append(permCodes, p.Code)
	}

	orgName := ""
	if membership.Organization != nil {
		orgName = membership.Organization.Name
	}
	roleName := ""
	if membership.Role != nil {
		roleName = membership.Role.Name
	}

	// 5. 生成 JWT Access Token
	claims := &domain.UserClaims{
		UserID:         user.ID.String(),
		Email:          user.Email,
		OrganizationID: membership.OrganizationID.String(),
		RoleID:         membership.RoleID.String(),
		RoleName:       roleName,
		Permissions:    permCodes,
	}
	accessToken, err := s.generateAccessToken(claims)
	if err != nil {
		return nil, fmt.Errorf("生成访问令牌失败: %w", err)
	}

	// 6. 生成高熵随机 Refresh Token 并存入数据库
	rawRefreshToken, tokenHash, err := s.generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("生成刷新令牌失败: %w", err)
	}

	expiresAt := time.Now().UTC().Add(time.Duration(s.cfg.RefreshExpiresDays) * 24 * time.Hour)
	session := &domain.RefreshSession{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}
	if err := s.repo.CreateRefreshSession(ctx, session); err != nil {
		return nil, fmt.Errorf("保存刷新会话失败: %w", err)
	}

	// 7. 更新用户最后活跃时间
	_ = s.repo.UpdateUserLastActive(ctx, user.ID)

	return &LoginResult{
		AccessToken:     accessToken,
		RawRefreshToken: rawRefreshToken,
		User: &domain.AuthUserDTO{
			ID:             user.ID.String(),
			Email:          user.Email,
			Name:           user.Name,
			OrganizationID: membership.OrganizationID.String(),
			OrgName:        orgName,
			RoleName:       roleName,
			Permissions:    permCodes,
		},
	}, nil
}

// Refresh 令牌轮换与刷新
func (s *DefaultAuthService) Refresh(ctx context.Context, rawRefreshToken string) (*RefreshResult, error) {
	if rawRefreshToken == "" {
		return nil, ErrTokenRevoked
	}

	tokenHash := repository.HashToken(rawRefreshToken)
	session, err := s.repo.FindRefreshSessionByHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("查询刷新会话失败: %w", err)
	}
	if session == nil {
		return nil, ErrTokenRevoked
	}

	// 检查是否已撤销
	if session.RevokedAt != nil {
		return nil, ErrTokenRevoked
	}

	// 检查是否过期
	if time.Now().UTC().After(session.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// 检查用户有效性
	user, err := s.repo.FindUserByID(ctx, session.UserID)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}
	if user.Status != "active" {
		return nil, ErrUserSuspended
	}

	// 获取最新权限和组织
	membership, err := s.repo.FindMembershipByUserID(ctx, user.ID)
	if err != nil || membership == nil {
		return nil, ErrOrgNotFound
	}

	_, perms, err := s.repo.GetRoleWithPermissions(ctx, membership.RoleID)
	if err != nil {
		return nil, fmt.Errorf("查询权限失败: %w", err)
	}

	var permCodes []string
	for _, p := range perms {
		permCodes = append(permCodes, p.Code)
	}

	orgName := ""
	if membership.Organization != nil {
		orgName = membership.Organization.Name
	}
	roleName := ""
	if membership.Role != nil {
		roleName = membership.Role.Name
	}

	// 撤销旧会话 (Token Rotation 轮换)
	_ = s.repo.RevokeRefreshSession(ctx, tokenHash)

	// 生成新的 Token 对
	claims := &domain.UserClaims{
		UserID:         user.ID.String(),
		Email:          user.Email,
		OrganizationID: membership.OrganizationID.String(),
		RoleID:         membership.RoleID.String(),
		RoleName:       roleName,
		Permissions:    permCodes,
	}
	newAccessToken, err := s.generateAccessToken(claims)
	if err != nil {
		return nil, err
	}

	newRawToken, newTokenHash, err := s.generateRefreshToken()
	if err != nil {
		return nil, err
	}

	newSession := &domain.RefreshSession{
		UserID:    user.ID,
		TokenHash: newTokenHash,
		ExpiresAt: time.Now().UTC().Add(time.Duration(s.cfg.RefreshExpiresDays) * 24 * time.Hour),
	}
	if err := s.repo.CreateRefreshSession(ctx, newSession); err != nil {
		return nil, err
	}

	return &RefreshResult{
		AccessToken:        newAccessToken,
		NewRawRefreshToken: newRawToken,
		User: &domain.AuthUserDTO{
			ID:             user.ID.String(),
			Email:          user.Email,
			Name:           user.Name,
			OrganizationID: membership.OrganizationID.String(),
			OrgName:        orgName,
			RoleName:       roleName,
			Permissions:    permCodes,
		},
	}, nil
}

// Logout 注销登录
func (s *DefaultAuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	if rawRefreshToken == "" {
		return nil
	}
	tokenHash := repository.HashToken(rawRefreshToken)
	return s.repo.RevokeRefreshSession(ctx, tokenHash)
}

// GetMe 获取当前登录用户画像与权限列表
func (s *DefaultAuthService) GetMe(ctx context.Context, userID uuid.UUID, orgID uuid.UUID) (*domain.AuthUserDTO, error) {
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}

	membership, err := s.repo.FindMembershipByUserID(ctx, userID)
	if err != nil || membership == nil {
		return nil, ErrOrgNotFound
	}

	_, perms, err := s.repo.GetRoleWithPermissions(ctx, membership.RoleID)
	if err != nil {
		return nil, err
	}

	var permCodes []string
	for _, p := range perms {
		permCodes = append(permCodes, p.Code)
	}

	orgName := ""
	if membership.Organization != nil {
		orgName = membership.Organization.Name
	}
	roleName := ""
	if membership.Role != nil {
		roleName = membership.Role.Name
	}

	return &domain.AuthUserDTO{
		ID:             user.ID.String(),
		Email:          user.Email,
		Name:           user.Name,
		OrganizationID: membership.OrganizationID.String(),
		OrgName:        orgName,
		RoleName:       roleName,
		Permissions:    permCodes,
	}, nil
}

// ParseToken 解析并校验 JWT Access Token
func (s *DefaultAuthService) ParseToken(tokenStr string) (*domain.UserClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("不支持的签名算法: %v", token.Header["alg"])
		}
		return []byte(s.cfg.AccessSecret), nil
	})
	if err != nil {
		return nil, err
	}

	if claimsMap, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		var perms []string
		if rawPerms, ok := claimsMap["permissions"].([]interface{}); ok {
			for _, p := range rawPerms {
				if ps, ok := p.(string); ok {
					perms = append(perms, ps)
				}
			}
		}

		return &domain.UserClaims{
			UserID:         getStringClaim(claimsMap, "user_id"),
			Email:          getStringClaim(claimsMap, "email"),
			OrganizationID: getStringClaim(claimsMap, "org_id"),
			RoleID:         getStringClaim(claimsMap, "role_id"),
			RoleName:       getStringClaim(claimsMap, "role_name"),
			Permissions:    perms,
		}, nil
	}

	return nil, errors.New("无效的访问令牌")
}

// generateAccessToken 签署 JWT Access Token
func (s *DefaultAuthService) generateAccessToken(claims *domain.UserClaims) (string, error) {
	now := time.Now().UTC()
	exp := now.Add(time.Duration(s.cfg.AccessExpiresMin) * time.Minute)

	jwtClaims := jwt.MapClaims{
		"user_id":     claims.UserID,
		"email":       claims.Email,
		"org_id":      claims.OrganizationID,
		"role_id":     claims.RoleID,
		"role_name":   claims.RoleName,
		"permissions": claims.Permissions,
		"iat":         now.Unix(),
		"exp":         exp.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims)
	return token.SignedString([]byte(s.cfg.AccessSecret))
}

// generateRefreshToken 生成加密安全的随机字节作为 Refresh Token
func (s *DefaultAuthService) generateRefreshToken() (rawToken string, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	rawToken = hex.EncodeToString(b)
	tokenHash = repository.HashToken(rawToken)
	return rawToken, tokenHash, nil
}

func getStringClaim(m jwt.MapClaims, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
