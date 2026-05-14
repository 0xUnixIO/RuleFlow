package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/ablate-ai/RuleFlow/database"
	"github.com/ablate-ai/RuleFlow/internal/app"
)

type RuleSourceSyncService struct {
	repo *database.RuleSourceRepo
}

func NewRuleSourceSyncService(repo *database.RuleSourceRepo) *RuleSourceSyncService {
	return &RuleSourceSyncService{repo: repo}
}

func (s *RuleSourceSyncService) SyncRuleSource(ctx context.Context, id int64) (int, error) {
	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return 0, err
	}
	if !source.Enabled {
		return 0, fmt.Errorf("规则源已禁用: %s", source.Name)
	}

	log.Printf("[rule-sync] 开始同步规则源: id=%d name=%s", source.ID, source.Name)

	// 如果 URL 为空，说明是手动维护的规则源
	if source.URL == "" {
		if source.RawContent == "" {
			return 0, fmt.Errorf("手动规则源未配置规则内容")
		}

		log.Printf("[rule-sync] 解析手动规则源内容: id=%d name=%s", source.ID, source.Name)
		parsed, ruleCount, err := database.ParseRuleSourceContent(source.RawContent, source.SourceFormat)
		if err != nil {
			_ = s.repo.UpdateSyncResult(ctx, id, source.RawContent, json.RawMessage("[]"), 0, err)
			return 0, fmt.Errorf("解析规则源失败: %w", err)
		}

		if err := s.repo.UpdateSyncResult(ctx, id, source.RawContent, parsed, ruleCount, nil); err != nil {
			return 0, err
		}

		return ruleCount, nil
	}

	// 远程规则源同步逻辑
	content, _, err := app.FetchSubscriptionContent(ctx, source.URL)
	if err != nil {
		_ = s.repo.UpdateSyncResult(ctx, id, "", json.RawMessage("[]"), 0, err)
		return 0, fmt.Errorf("拉取规则源失败: %w", err)
	}

	parsed, ruleCount, err := database.ParseRuleSourceContent(content, source.SourceFormat)
	if err != nil {
		_ = s.repo.UpdateSyncResult(ctx, id, content, json.RawMessage("[]"), 0, err)
		return 0, fmt.Errorf("解析规则源失败: %w", err)
	}

	if err := s.repo.UpdateSyncResult(ctx, id, content, parsed, ruleCount, nil); err != nil {
		return 0, err
	}

	return ruleCount, nil
}

func (s *RuleSourceSyncService) loadSourceRules(ctx context.Context, name string) ([]app.RuleSetRule, error) {
	source, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(source.ParsedRules) == 0 {
		return nil, fmt.Errorf("规则源尚未同步: %s", source.Name)
	}
	var rules []app.RuleSetRule
	if err := json.Unmarshal(source.ParsedRules, &rules); err != nil {
		return nil, fmt.Errorf("读取已同步规则失败: %w", err)
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("规则源没有可导出的规则: %s", source.Name)
	}
	return rules, nil
}

func (s *RuleSourceSyncService) ExportRuleSource(ctx context.Context, name string, target string) (string, error) {
	rules, err := s.loadSourceRules(ctx, name)
	if err != nil {
		return "", err
	}
	return app.ExportRuleSet(rules, target)
}

// ExportRuleSourceSRS 将规则源编译为 sing-box SRS 二进制格式写入 w
func (s *RuleSourceSyncService) ExportRuleSourceSRS(ctx context.Context, name string, w io.Writer) error {
	rules, err := s.loadSourceRules(ctx, name)
	if err != nil {
		return err
	}
	return app.WriteSRS(rules, w)
}

func (s *RuleSourceSyncService) SyncDueSources(ctx context.Context) {
	sources, err := s.repo.List(ctx)
	if err != nil {
		log.Printf("[rule-sync] 获取规则源列表失败: %v", err)
		return
	}

	now := time.Now()
	for _, source := range sources {
		if !source.Enabled || !source.AutoRefresh {
			continue
		}
		if source.LastSyncedAt != nil && now.Sub(*source.LastSyncedAt) < time.Duration(source.RefreshInterval)*time.Second {
			continue
		}
		syncCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		if _, err := s.SyncRuleSource(syncCtx, source.ID); err != nil {
			log.Printf("[rule-sync] 同步规则源失败: id=%d err=%v", source.ID, err)
		}
		cancel()
	}
}
