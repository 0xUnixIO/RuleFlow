package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client Redis 客户端封装
type Client struct {
	*redis.Client
}

// New 创建新的 Redis 客户端
func New(addr, password string, db int) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	// 验证连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("Redis 连接失败: %w", err)
	}

	return &Client{rdb}, nil
}

// Close 关闭 Redis 客户端
func (c *Client) Close() error {
	if c.Client != nil {
		return c.Client.Close()
	}
	return nil
}

// Ping 检查 Redis 连接状态
func (c *Client) Ping(ctx context.Context) error {
	return c.Client.Ping(ctx).Err()
}

// Health 检查 Redis 健康状态
func (c *Client) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	status := map[string]string{}
	if err := c.Ping(ctx); err != nil {
		status["status"] = "unhealthy"
		status["error"] = err.Error()
		return status
	}

	status["status"] = "healthy"
	status["connected"] = "true"

	return status
}

// Get 获取缓存值
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.Client.Get(ctx, key).Result()
}

// Set 设置缓存值
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.Client.Set(ctx, key, value, expiration).Err()
}

// Delete 删除缓存值
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.Client.Del(ctx, keys...).Err()
}

// Exists 检查键是否存在
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	return c.Client.Exists(ctx, keys...).Result()
}

// AcquireLock 获取分布式锁
func (c *Client) AcquireLock(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	// 使用 SET NX EX 命令实现分布式锁
	result, err := c.Client.SetNX(ctx, key, "1", expiration).Result()
	if err != nil {
		return false, fmt.Errorf("获取锁失败: %w", err)
	}
	return result, nil
}

// ReleaseLock 释放分布式锁
func (c *Client) ReleaseLock(ctx context.Context, key string) error {
	return c.Client.Del(ctx, key).Err()
}
