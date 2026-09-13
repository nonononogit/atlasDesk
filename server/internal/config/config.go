package config

import (
	"os"
	"strconv"
	"strings"
)

// Config 系统全局配置结构体
type Config struct {
	App      AppConfig      // 应用基础配置
	Database DatabaseConfig // PostgreSQL 数据库配置
	Redis    RedisConfig    // Redis 缓存配置
	Storage  StorageConfig  // 对象存储配置
	JWT      JWTConfig      // 身份认证配置
}

// AppConfig 应用基础服务配置
type AppConfig struct {
	Env            string   // 运行环境: development | production | test
	Name           string   // 应用名称
	Port           string   // 监听端口，如 "8080"
	AllowedOrigins []string // 允许的跨域来源列表
}

// DatabaseConfig 数据库配置结构
type DatabaseConfig struct {
	Host     string // 数据库主机
	Port     string // 数据库端口
	User     string // 用户名
	Password string // 密码
	DBName   string // 数据库名
	SSLMode  string // SSL 模式: disable | require
}

// DSN 返回 PostgreSQL 连接串
func (d *DatabaseConfig) DSN() string {
	return "host=" + d.Host +
		" port=" + d.Port +
		" user=" + d.User +
		" password=" + d.Password +
		" dbname=" + d.DBName +
		" sslmode=" + d.SSLMode
}

// RedisConfig Redis 服务配置结构
type RedisConfig struct {
	Host     string // Redis 主机
	Port     string // Redis 端口
	Password string // 密码
	DB       int    // 数据库序号
}

// Addr 返回 Redis 地址 "host:port"
func (r *RedisConfig) Addr() string {
	return r.Host + ":" + r.Port
}

// StorageConfig MinIO / S3 对象存储配置
type StorageConfig struct {
	Endpoint  string // S3 兼容 Endpoint
	AccessKey string // 访问密钥
	SecretKey string // 私有密钥
	Bucket    string // 存储桶名称
	UseSSL    bool   // 是否启用 SSL
}

// JWTConfig 身份令牌配置
type JWTConfig struct {
	AccessSecret        string // Access Token 签名密钥
	RefreshSecret       string // Refresh Token 签名密钥
	AccessExpiresMin    int    // Access Token 过期分钟数
	RefreshExpiresDays  int    // Refresh Token 过期天数
	EncryptionKey       string // 敏感配置加密密钥 (32 字节)
}

// Load 从系统环境变量中加载配置并设置合理默认值
func Load() *Config {
	originsStr := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000")
	var origins []string
	for _, o := range strings.Split(originsStr, ",") {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	accessExp, _ := strconv.Atoi(getEnv("JWT_ACCESS_EXPIRES_MINUTES", "15"))
	refreshExp, _ := strconv.Atoi(getEnv("JWT_REFRESH_EXPIRES_DAYS", "7"))
	useSSL := getEnv("STORAGE_USE_SSL", "false") == "true"

	return &Config{
		App: AppConfig{
			Env:            getEnv("APP_ENV", "development"),
			Name:           getEnv("APP_NAME", "atlasdesk"),
			Port:           getEnv("PORT", "8080"),
			AllowedOrigins: origins,
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "atlasdesk_user"),
			Password: getEnv("DB_PASSWORD", "atlasdesk_secret_change_me"),
			DBName:   getEnv("DB_NAME", "atlasdesk_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
		Storage: StorageConfig{
			Endpoint:  getEnv("STORAGE_ENDPOINT", "localhost:9000"),
			AccessKey: getEnv("STORAGE_ACCESS_KEY", "minioadmin"),
			SecretKey: getEnv("STORAGE_SECRET_KEY", "minioadmin"),
			Bucket:    getEnv("STORAGE_BUCKET", "atlasdesk-docs"),
			UseSSL:    useSSL,
		},
		JWT: JWTConfig{
			AccessSecret:       getEnv("JWT_ACCESS_SECRET", "default_access_secret_for_dev"),
			RefreshSecret:      getEnv("JWT_REFRESH_SECRET", "default_refresh_secret_for_dev"),
			AccessExpiresMin:   accessExp,
			RefreshExpiresDays: refreshExp,
			EncryptionKey:      getEnv("ENCRYPTION_KEY", "32_bytes_aes_key_for_api_keys!"),
		},
	}
}

// getEnv 读取指定环境变量，不存在则返回 fallback 默认值
func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
