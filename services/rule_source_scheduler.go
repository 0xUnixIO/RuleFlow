package services

import (
	"context"
	"log"
	"time"
)

type RuleSourceScheduler struct {
	syncService   *RuleSourceSyncService
	checkInterval time.Duration
}

func NewRuleSourceScheduler(syncService *RuleSourceSyncService) *RuleSourceScheduler {
	return &RuleSourceScheduler{
		syncService:   syncService,
		checkInterval: time.Minute,
	}
}

func (s *RuleSourceScheduler) Start(ctx context.Context) {
	runPeriodicTask(ctx, s.checkInterval, false, s.syncService.SyncDueSources)
	log.Println("[rule-sync] 规则源调度器已启动")
}
