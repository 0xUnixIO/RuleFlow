package services

import (
	"context"
	"log"
	"time"

	"github.com/ablate-ai/RuleFlow/database"
)

// LogCleanupScheduler 日志清理调度器
type LogCleanupScheduler struct {
	accessLogRepo *database.ConfigAccessLogRepo
	// checkInterval 检查间隔，默认 1 小时
	checkInterval time.Duration
	// keepDays 保留天数，默认 30 天
	keepDays int
	// maxRecords 最大保留记录数，默认 10000 条
	maxRecords int
}

// LogCleanupOption 日志清理调度器选项
type LogCleanupOption func(*LogCleanupScheduler)

// WithLogCleanupCheckInterval 设置检查间隔
func WithLogCleanupCheckInterval(interval time.Duration) LogCleanupOption {
	return func(s *LogCleanupScheduler) {
		s.checkInterval = interval
	}
}

// WithLogCleanupKeepDays 设置保留天数
func WithLogCleanupKeepDays(days int) LogCleanupOption {
	return func(s *LogCleanupScheduler) {
		s.keepDays = days
	}
}

// WithLogCleanupMaxRecords 设置最大保留记录数
func WithLogCleanupMaxRecords(max int) LogCleanupOption {
	return func(s *LogCleanupScheduler) {
		s.maxRecords = max
	}
}

// NewLogCleanupScheduler 创建日志清理调度器
func NewLogCleanupScheduler(accessLogRepo *database.ConfigAccessLogRepo, opts ...LogCleanupOption) *LogCleanupScheduler {
	s := &LogCleanupScheduler{
		accessLogRepo: accessLogRepo,
		checkInterval: time.Hour,
		keepDays:      30,
		maxRecords:    10000,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Start 在后台启动调度循环，ctx 取消时退出
func (s *LogCleanupScheduler) Start(ctx context.Context) {
	runPeriodicTask(ctx, s.checkInterval, true, s.run)
	log.Printf("[log-cleanup] 日志清理调度器已启动（保留 %d 天 / %d 条）", s.keepDays, s.maxRecords)
}

// run 执行日志清理
func (s *LogCleanupScheduler) run(ctx context.Context) {
	log.Println("[log-cleanup] 开始清理旧日志")
	deletedCount, err := s.accessLogRepo.CleanupOldLogs(ctx, s.keepDays, s.maxRecords)
	if err != nil {
		log.Printf("[log-cleanup] 清理失败: %v", err)
		return
	}
	if deletedCount > 0 {
		log.Printf("[log-cleanup] 清理完成，删除了 %d 条日志", deletedCount)
	} else {
		log.Println("[log-cleanup] 清理完成，无需删除日志")
	}
}
