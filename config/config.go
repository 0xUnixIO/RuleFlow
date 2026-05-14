package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config 应用程序配置
type Config struct {
	// 服务器配置
	Port string

	// 管理员密码（用于 Web 控制台 Basic Auth，为空则不启用鉴权）
	AdminPassword string

	// 允许的 CORS 来源（逗号分隔，默认 "*"）
	CORSAllowedOrigins string

	// PostgreSQL 配置
	DatabaseURL string

	// Redis 配置
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// 缓存配置
	CacheTTLSeconds int

	// 日志清理配置
	LogKeepDays      int // 保留日志天数，默认 30 天
	LogMaxRecords    int // 最大保留日志记录数，默认 10000 条
	LogCheckInterval int // 日志检查间隔（小时），默认 1 小时
}

// Load 从环境变量加载配置
func Load() *Config {
	// 尝试加载 .env 文件（如果存在）
	_ = godotenv.Load()

	return &Config{
		Port:               getEnv("PORT", "8080"),
		AdminPassword:      getEnv("ADMIN_PASSWORD", ""),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "*"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgresql://ruleflow:password@localhost:5432/ruleflow?sslmode=disable"),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            getEnvInt("REDIS_DB", 0),
		CacheTTLSeconds:    getEnvInt("CACHE_TTL_SECONDS", 3600),
		LogKeepDays:        getEnvInt("LOG_KEEP_DAYS", 30),
		LogMaxRecords:      getEnvInt("LOG_MAX_RECORDS", 10000),
		LogCheckInterval:   getEnvInt("LOG_CHECK_INTERVAL", 1),
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt 获取整数环境变量，如果不存在或无效则返回默认值
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
