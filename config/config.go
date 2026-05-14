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
	LogKeepDays      int
	LogMaxRecords    int
	LogCheckInterval int
}

// Load 从环境变量加载配置
func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		Port:               getEnv("PORT", "8080"),
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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
