package services

import (
	"context"
	"log"
	"time"

	"github.com/ablate-ai/RuleFlow/database"
)

// SubscriptionScheduler 订阅自动刷新调度器
type SubscriptionScheduler struct {
	subRepo     *database.SubscriptionRepo
	syncService *SubscriptionSyncService
	// checkInterval 轮询间隔，默认 1 分钟
	checkInterval time.Duration
}

// NewSubscriptionScheduler 创建调度器
func NewSubscriptionScheduler(subRepo *database.SubscriptionRepo, syncService *SubscriptionSyncService) *SubscriptionScheduler {
	return &SubscriptionScheduler{
		subRepo:       subRepo,
		syncService:   syncService,
		checkInterval: time.Minute,
	}
}

// Start 在后台启动调度循环，ctx 取消时退出
func (s *SubscriptionScheduler) Start(ctx context.Context) {
	runPeriodicTask(ctx, s.checkInterval, true, s.run)
	log.Println("[scheduler] 订阅自动刷新调度器已启动")
}

// run 检查所有需要刷新的订阅并触发同步
func (s *SubscriptionScheduler) run(ctx context.Context) {
	subs, err := s.subRepo.List(ctx)
	if err != nil {
		log.Printf("[scheduler] 获取订阅列表失败: %v", err)
		return
	}

	now := time.Now()
	for _, sub := range subs {
		if !sub.AutoRefresh || !sub.Enabled {
			continue
		}
		interval := time.Duration(sub.RefreshInterval) * time.Second
		if interval <= 0 {
			interval = time.Hour // 兜底默认 1 小时
		}
		// 从未同步过，或距上次同步已超过 interval
		if sub.LastFetchedAt == nil || now.Sub(*sub.LastFetchedAt) >= interval {
			log.Printf("[scheduler] 触发自动同步: %s（间隔 %v）", sub.Name, interval)
			go func(id int64, name string) {
				syncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				if _, err := s.syncService.SyncSubscription(syncCtx, id); err != nil {
					log.Printf("[scheduler] 自动同步失败 %s: %v", name, err)
				}
			}(sub.ID, sub.Name)
		}
	}
}
