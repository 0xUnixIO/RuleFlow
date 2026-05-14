package cache

import (
	"context"
	"fmt"
	"time"
)

// SubscriptionCache 策略配置缓存管理
type SubscriptionCache struct {
	client *Client
	ttl    time.Duration
}

// NewSubscriptionCache 创建策略配置缓存
func NewSubscriptionCache(client *Client, ttl time.Duration) *SubscriptionCache {
	return &SubscriptionCache{
		client: client,
		ttl:    ttl,
	}
}

// 键名格式常量
const (
	// CacheKeyPolicyConfig 策略配置缓存键格式：ruleflow:policy:config:<token>
	CacheKeyPolicyConfig = "ruleflow:policy:config:%s"
)

// DeleteAllByPattern 按模式删除所有缓存
func (c *SubscriptionCache) DeleteAllByPattern(ctx context.Context, pattern string) error {
	// 检查客户端是否为 nil
	if c == nil || c.client == nil || c.client.Client == nil {
		return nil // 如果没有 Redis 连接，直接返回成功
	}

	// 使用 Keys 命令查找匹配的键
	keys, err := c.client.Client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("查找键失败: %w", err)
	}

	if len(keys) > 0 {
		return c.client.Delete(ctx, keys...)
	}

	return nil
}

// GetPolicyConfig 获取策略配置缓存（按 token）
func (c *SubscriptionCache) GetPolicyConfig(ctx context.Context, token string) (string, error) {
	key := fmt.Sprintf(CacheKeyPolicyConfig, token)
	return c.client.Get(ctx, key)
}

// SetPolicyConfig 设置策略配置缓存
func (c *SubscriptionCache) SetPolicyConfig(ctx context.Context, token, yaml string) error {
	key := fmt.Sprintf(CacheKeyPolicyConfig, token)
	return c.client.Set(ctx, key, yaml, c.ttl)
}

// DeletePolicyConfig 删除策略配置缓存
func (c *SubscriptionCache) DeletePolicyConfig(ctx context.Context, token string) error {
	key := fmt.Sprintf(CacheKeyPolicyConfig, token)
	return c.client.Delete(ctx, key)
}

// FlushAll 清空所有缓存（谨慎使用）
func (c *SubscriptionCache) FlushAll(ctx context.Context) error {
	return c.client.Client.FlushDB(ctx).Err()
}

// GetClient 获取 Redis 客户端
func (c *SubscriptionCache) GetClient() *Client {
	return c.client
}
