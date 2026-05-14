package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ablate-ai/RuleFlow/cache"
	"github.com/ablate-ai/RuleFlow/database"
)

// NodeDeleter 节点删除接口（用于解耦和测试）
type NodeDeleter interface {
	DeleteBySourceID(ctx context.Context, sourceID int64) (int64, error)
}

// SubscriptionService 订阅服务
type SubscriptionService struct {
	repo      *database.SubscriptionRepo
	cache     *cache.SubscriptionCache
	nodeDeleter NodeDeleter
}

// NewSubscriptionService 创建订阅服务
func NewSubscriptionService(repo *database.SubscriptionRepo, cache *cache.SubscriptionCache, nodeDeleter NodeDeleter) *SubscriptionService {
	return &SubscriptionService{
		repo:        repo,
		cache:       cache,
		nodeDeleter: nodeDeleter,
	}
}

// CreateSubscription 创建订阅
func (s *SubscriptionService) CreateSubscription(ctx context.Context, sub *database.Subscription) error {
	sub.Name = strings.TrimSpace(sub.Name)
	sub.Description = strings.TrimSpace(sub.Description)
	if sub.URL != nil {
		trimmedURL := strings.TrimSpace(*sub.URL)
		sub.URL = &trimmedURL
	}

	// 检查名称是否已存在
	exists, err := s.repo.Exists(ctx, sub.Name)
	if err != nil {
		return fmt.Errorf("检查订阅名称失败: %w", err)
	}
	if exists {
		return fmt.Errorf("订阅名称已存在: %s", sub.Name)
	}

	// 验证 URL
	if sub.URL == nil || strings.TrimSpace(*sub.URL) == "" {
		return fmt.Errorf("订阅必须提供 URL")
	}

	// 设置默认值
	if sub.RefreshInterval == 0 {
		sub.RefreshInterval = 3600 // 默认 1 小时
	}

	return s.repo.Create(ctx, sub)
}

// GetSubscription 获取订阅信息
func (s *SubscriptionService) GetSubscription(ctx context.Context, name string) (*database.Subscription, error) {
	return s.repo.GetByName(ctx, name)
}

// GetSubscriptionByID 获取订阅信息
func (s *SubscriptionService) GetSubscriptionByID(ctx context.Context, id int64) (*database.Subscription, error) {
	return s.repo.GetByID(ctx, id)
}

// ListSubscriptions 列出所有订阅（附带流量信息）
func (s *SubscriptionService) ListSubscriptions(ctx context.Context) ([]database.Subscription, error) {
	return s.repo.List(ctx)
}

// UpdateSubscription 更新订阅
func (s *SubscriptionService) UpdateSubscription(ctx context.Context, sub *database.Subscription) error {
	sub.Name = strings.TrimSpace(sub.Name)
	sub.Description = strings.TrimSpace(sub.Description)
	if sub.URL != nil {
		trimmedURL := strings.TrimSpace(*sub.URL)
		sub.URL = &trimmedURL
	}

	// 验证 URL
	if sub.URL == nil || strings.TrimSpace(*sub.URL) == "" {
		return fmt.Errorf("订阅必须提供 URL")
	}

	// 查旧记录，获取原名称用于清缓存和判断是否改名
	old, err := s.repo.GetByID(ctx, sub.ID)
	if err != nil {
		return err
	}

	// 改名时检查新名称是否冲突
	if old.Name != sub.Name {
		exists, err := s.repo.Exists(ctx, sub.Name)
		if err != nil {
			return fmt.Errorf("检查订阅名称失败: %w", err)
		}
		if exists {
			return fmt.Errorf("订阅名称已存在: %s", sub.Name)
		}
	}

	// 订阅变更后让所有策略配置缓存失效
	if s.cache != nil {
		_ = s.cache.DeleteAllByPattern(ctx, "ruleflow:policy:config:*")
	}

	return s.repo.Update(ctx, sub)
}

// DeleteSubscriptionByID 删除订阅
func (s *SubscriptionService) DeleteSubscriptionByID(ctx context.Context, id int64) error {
	// 先删除关联的节点
	if s.nodeDeleter != nil {
		_, err := s.nodeDeleter.DeleteBySourceID(ctx, id)
		if err != nil {
			return fmt.Errorf("删除关联节点失败: %w", err)
		}
	}

	return s.repo.DeleteByID(ctx, id)
}

// Health 健康检查
func (s *SubscriptionService) Health(ctx context.Context) map[string]interface{} {
	status := make(map[string]interface{})

	// 数据库健康检查
	dbHealth := make(chan map[string]string, 1)
	go func() {
		dbHealth <- s.repo.GetDB().Health()
	}()

	// Redis 健康检查
	cacheHealth := make(chan map[string]string, 1)
	if s.cache != nil {
		go func() {
			cacheHealth <- s.cache.GetClient().Health()
		}()
	} else {
		cacheHealth <- map[string]string{"status": "disabled"}
	}

	// 收集结果
	select {
	case h := <-dbHealth:
		status["database"] = h
	case <-time.After(2 * time.Second):
		status["database"] = map[string]string{"status": "timeout"}
	}

	select {
	case h := <-cacheHealth:
		status["redis"] = h
	case <-time.After(2 * time.Second):
		status["redis"] = map[string]string{"status": "timeout"}
	}

	// 整体状态
	allHealthy := true
	for _, component := range []string{"database", "redis"} {
		if compStatus, ok := status[component].(map[string]string); ok {
			if component == "redis" && compStatus["status"] == "disabled" {
				continue
			}
			if compStatus["status"] != "healthy" {
				allHealthy = false
				break
			}
		} else {
			allHealthy = false
			break
		}
	}

	if allHealthy {
		status["status"] = "healthy"
	} else {
		status["status"] = "degraded"
	}

	return status
}
