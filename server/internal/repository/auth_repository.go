package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"atlasdesk/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuthRepository 认证相关数据仓储接口
type AuthRepository interface {
	FindUserByEmail(ctx context.Context, email string) (*domain.User, error)
	FindUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	FindMembershipByUserID(ctx context.Context, userID uuid.UUID) (*domain.Membership, error)
	GetRoleWithPermissions(ctx context.Context, roleID uuid.UUID) (*domain.Role, []domain.Permission, error)
	CreateRefreshSession(ctx context.Context, session *domain.RefreshSession) error
	FindRefreshSessionByHash(ctx context.Context, tokenHash string) (*domain.RefreshSession, error)
	RevokeRefreshSession(ctx context.Context, tokenHash string) error
	UpdateUserLastActive(ctx context.Context, userID uuid.UUID) error
}

// GormAuthRepository 基于 GORM 的认证数据仓储实现
type GormAuthRepository struct {
	db *gorm.DB
}

// NewAuthRepository 构造 GormAuthRepository 实例
func NewAuthRepository(db *gorm.DB) *GormAuthRepository {
	return &GormAuthRepository{db: db}
}

// HashToken 计算 Refresh Token 的 SHA-256 哈希
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// FindUserByEmail 根据邮箱查询用户
func (r *GormAuthRepository) FindUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindUserByID 根据主键 ID 查询用户
func (r *GormAuthRepository) FindUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// FindMembershipByUserID 查询用户的租户组织成员关联
func (r *GormAuthRepository) FindMembershipByUserID(ctx context.Context, userID uuid.UUID) (*domain.Membership, error) {
	var membership domain.Membership
	err := r.db.WithContext(ctx).
		Preload("Organization").
		Preload("Role").
		Where("user_id = ?", userID).
		First(&membership).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &membership, nil
}

// GetRoleWithPermissions 获取角色及其包含的所有权限编码
func (r *GormAuthRepository) GetRoleWithPermissions(ctx context.Context, roleID uuid.UUID) (*domain.Role, []domain.Permission, error) {
	var role domain.Role
	err := r.db.WithContext(ctx).Preload("Permissions").Where("id = ?", roleID).First(&role).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	return &role, role.Permissions, nil
}

// CreateRefreshSession 创建刷新凭证会话记录
func (r *GormAuthRepository) CreateRefreshSession(ctx context.Context, session *domain.RefreshSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

// FindRefreshSessionByHash 根据 Token 哈希查找有效会话
func (r *GormAuthRepository) FindRefreshSessionByHash(ctx context.Context, tokenHash string) (*domain.RefreshSession, error) {
	var session domain.RefreshSession
	err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

// RevokeRefreshSession 标记会话撤销 (退出登录或轮换)
func (r *GormAuthRepository) RevokeRefreshSession(ctx context.Context, tokenHash string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&domain.RefreshSession{}).
		Where("token_hash = ?", tokenHash).
		Update("revoked_at", now).Error
}

// UpdateUserLastActive 更新用户最后活跃时间戳
func (r *GormAuthRepository) UpdateUserLastActive(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&domain.User{}).
		Where("id = ?", userID).
		Update("last_active_at", now).Error
}
