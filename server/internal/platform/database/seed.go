package database

import (
	"context"
	"fmt"
	"log/slog"

	"atlasdesk/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StandardPermissions 定义规格第 4.2 节要求的 10 大核心权限字典
var StandardPermissions = []struct {
	Code string
	Name string
}{
	{"dashboard:read", "工作台指标与趋势查看"},
	{"knowledge:read", "知识库与文档内容阅读"},
	{"knowledge:write", "知识库管理与文档上传"},
	{"ticket:read", "工单详情与列表查看"},
	{"ticket:create", "创建工单"},
	{"ticket:assign", "工单坐席指派"},
	{"ticket:update", "工单流转与处理状态变更"},
	{"member:manage", "组织成员与权限管理"},
	{"ai_config:manage", "AI 模型与检索参数配置"},
	{"audit_log:read", "系统操作审计日志查看"},
}

// RolePermissionMap 规格第 1.1 节与第 4 节角色与权限映射矩阵
var RolePermissionMap = map[string][]string{
	"super_admin": {
		"dashboard:read", "knowledge:read", "knowledge:write",
		"ticket:read", "ticket:create", "ticket:assign", "ticket:update",
		"member:manage", "ai_config:manage", "audit_log:read",
	},
	"knowledge_admin": {
		"dashboard:read", "knowledge:read", "knowledge:write", "ai_config:manage",
	},
	"support_manager": {
		"dashboard:read", "ticket:read", "ticket:create", "ticket:assign", "ticket:update", "member:manage",
	},
	"support_agent": {
		"ticket:read", "ticket:create", "ticket:update", "knowledge:read",
	},
}

// SeedIdempotent 执行全量幂等种子数据填充
func SeedIdempotent(ctx context.Context, db *gorm.DB) error {
	slog.Info("开始执行数据库幂等种子数据填充...")

	// 1. 填充标准权限 (Permissions)
	permIDMap := make(map[string]uuid.UUID)
	for _, p := range StandardPermissions {
		var perm domain.Permission
		err := db.WithContext(ctx).
			Where(domain.Permission{Code: p.Code}).
			Attrs(domain.Permission{Name: p.Name}).
			FirstOrCreate(&perm).Error
		if err != nil {
			return fmt.Errorf("填充权限失败 (%s): %w", p.Code, err)
		}
		permIDMap[p.Code] = perm.ID
	}

	// 2. 填充系统内置 4 大角色 (Roles)
	systemRoles := []struct {
		RoleKey string
		Name    string
	}{
		{"super_admin", "超级管理员"},
		{"knowledge_admin", "知识管理员"},
		{"support_manager", "客服主管"},
		{"support_agent", "客服坐席"},
	}

	roleIDMap := make(map[string]uuid.UUID)
	for _, r := range systemRoles {
		var role domain.Role
		err := db.WithContext(ctx).
			Where("name = ? AND organization_id IS NULL", r.Name).
			Attrs(domain.Role{
				Name:     r.Name,
				IsSystem: true,
			}).
			FirstOrCreate(&role).Error
		if err != nil {
			return fmt.Errorf("填充角色失败 (%s): %w", r.Name, err)
		}
		roleIDMap[r.RoleKey] = role.ID

		// 绑定角色权限映射 (Role Permissions)
		expectedCodes := RolePermissionMap[r.RoleKey]
		var rolePerms []domain.RolePermission
		for _, code := range expectedCodes {
			if permID, ok := permIDMap[code]; ok {
				rolePerms = append(rolePerms, domain.RolePermission{
					RoleID:       role.ID,
					PermissionID: permID,
				})
			}
		}

		if len(rolePerms) > 0 {
			// 幂等忽略主键冲突
			if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rolePerms).Error; err != nil {
				return fmt.Errorf("绑定角色权限失败 (%s): %w", r.Name, err)
			}
		}
	}

	// 3. 填充默认演示组织 (Organization)
	var defaultOrg domain.Organization
	err := db.WithContext(ctx).
		Where(domain.Organization{Slug: "atlasdesk-demo"}).
		Attrs(domain.Organization{
			Name:     "AtlasDesk 演示企业",
			Slug:     "atlasdesk-demo",
			Settings: "{}",
		}).
		FirstOrCreate(&defaultOrg).Error
	if err != nil {
		return fmt.Errorf("创建默认演示组织失败: %w", err)
	}

	// 4. 填充初始超级管理员账号 (Admin User)
	adminEmail := "admin@atlasdesk.local"
	var adminUser domain.User
	err = db.WithContext(ctx).Where(domain.User{Email: adminEmail}).First(&adminUser).Error
	if err == gorm.ErrRecordNotFound {
		// 生成初始密码密文 (Admin@123456)
		hash, err := bcrypt.GenerateFromPassword([]byte("Admin@123456"), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("加密默认管理员密码失败: %w", err)
		}

		adminUser = domain.User{
			Email:        adminEmail,
			PasswordHash: string(hash),
			Name:         "系统超级管理员",
			Status:       "active",
		}
		if err := db.WithContext(ctx).Create(&adminUser).Error; err != nil {
			return fmt.Errorf("创建默认超级管理员失败: %w", err)
		}
		slog.Info("已初始化超级管理员账号", "email", adminEmail)
	} else if err != nil {
		return fmt.Errorf("查询默认超级管理员失败: %w", err)
	}

	// 5. 绑定超级管理员至演示组织 (Membership)
	if superAdminRoleID, ok := roleIDMap["super_admin"]; ok {
		var membership domain.Membership
		err := db.WithContext(ctx).
			Where(domain.Membership{
				OrganizationID: defaultOrg.ID,
				UserID:         adminUser.ID,
			}).
			Attrs(domain.Membership{
				RoleID: superAdminRoleID,
				Status: "active",
			}).
			FirstOrCreate(&membership).Error
		if err != nil {
			return fmt.Errorf("绑定默认成员从属关系失败: %w", err)
		}
	}

	slog.Info("数据库幂等种子数据填充完成")
	return nil
}
