package domain

import (
	"time"

	"github.com/google/uuid"
)

// Organization 租户组织聚合根
type Organization struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Slug      string    `gorm:"type:varchar(100);not null;uniqueIndex" json:"slug"`
	Settings  string    `gorm:"type:jsonb;not null;default:'{}'" json:"settings"`
	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// User 系统用户实体
type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email        string     `gorm:"type:varchar(255);not null;uniqueIndex" json:"email"`
	PasswordHash string     `gorm:"type:varchar(255);not null" json:"-"`
	Name         string     `gorm:"type:varchar(255);not null" json:"name"`
	Status       string     `gorm:"type:varchar(50);not null;default:'active'" json:"status"` // active, suspended
	LastActiveAt *time.Time `json:"last_active_at,omitempty"`
	CreatedAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// Role 用户角色实体
type Role struct {
	ID             uuid.UUID    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID *uuid.UUID   `gorm:"type:uuid;index" json:"organization_id,omitempty"` // 为空代表系统内置全局角色
	Name           string       `gorm:"type:varchar(100);not null" json:"name"`
	IsSystem       bool         `gorm:"not null;default:false" json:"is_system"`
	Permissions    []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
	CreatedAt      time.Time    `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt      time.Time    `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// Permission 权限项实体
type Permission struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Code      string    `gorm:"type:varchar(100);not null;uniqueIndex" json:"code"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// RolePermission 角色与权限关联中间表
type RolePermission struct {
	RoleID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"role_id"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey" json:"permission_id"`
}

// Membership 用户与组织租户成员关系
type Membership struct {
	ID             uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID uuid.UUID     `gorm:"type:uuid;not null;index;uniqueIndex:uq_membership_org_user" json:"organization_id"`
	UserID         uuid.UUID     `gorm:"type:uuid;not null;index;uniqueIndex:uq_membership_org_user" json:"user_id"`
	RoleID         uuid.UUID     `gorm:"type:uuid;not null" json:"role_id"`
	Status         string        `gorm:"type:varchar(50);not null;default:'active'" json:"status"` // active, suspended
	Organization   *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Role           *Role         `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	CreatedAt      time.Time     `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt      time.Time     `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// RefreshSession 刷新凭证会话记录
type RefreshSession struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash string     `gorm:"type:varchar(255);not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

// UserClaims JWT 令牌中的荷载声明
type UserClaims struct {
	UserID         string   `json:"user_id"`
	Email          string   `json:"email"`
	OrganizationID string   `json:"org_id"`
	RoleID         string   `json:"role_id"`
	RoleName       string   `json:"role_name"`
	Permissions    []string `json:"permissions"`
}

// AuthUserDTO 认证后返回的用户概要信息
type AuthUserDTO struct {
	ID             string   `json:"id"`
	Email          string   `json:"email"`
	Name           string   `json:"name"`
	OrganizationID string   `json:"organization_id"`
	OrgName        string   `json:"organization_name"`
	RoleName       string   `json:"role_name"`
	Permissions    []string `json:"permissions"`
}
