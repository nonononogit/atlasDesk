-- ==============================================================================
-- 迁移 000001 回滚脚本
-- 逆序删除表结构
-- ==============================================================================

DROP TABLE IF EXISTS refresh_sessions;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
