package services

import (
	"context"
	"log"
	"time"
)

// BackupScheduler 每 6 小时自动备份一次
type BackupScheduler struct {
	svc *BackupService
}

func NewBackupScheduler(svc *BackupService) *BackupScheduler {
	return &BackupScheduler{svc: svc}
}

func (s *BackupScheduler) Start(ctx context.Context) {
	// 启动时不立即执行，等第一个 6 小时周期再跑，避免重启时重复备份
	runPeriodicTask(ctx, 6*time.Hour, false, func(ctx context.Context) {
		log.Println("[backup] 开始定时备份")
		if err := s.svc.RunBackup(ctx); err != nil {
			log.Printf("[backup] 定时备份失败: %v", err)
		}
	})
	log.Println("[backup] 备份调度器已启动（每 6 小时执行一次，保留最近 6 份）")
}
