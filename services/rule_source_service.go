package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/ablate-ai/RuleFlow/database"
)

type RuleSourceService struct {
	repo *database.RuleSourceRepo
}

func NewRuleSourceService(repo *database.RuleSourceRepo) *RuleSourceService {
	return &RuleSourceService{repo: repo}
}

func (s *RuleSourceService) Create(ctx context.Context, source *database.RuleSource) error {
	if err := s.Validate(source); err != nil {
		return err
	}
	exists, err := s.repo.Exists(ctx, source.Name)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("规则源名称已存在: %s", source.Name)
	}
	return s.repo.Create(ctx, source)
}

func (s *RuleSourceService) GetByID(ctx context.Context, id int64) (*database.RuleSource, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RuleSourceService) GetByName(ctx context.Context, name string) (*database.RuleSource, error) {
	return s.repo.GetByName(ctx, name)
}

func (s *RuleSourceService) List(ctx context.Context) ([]database.RuleSource, error) {
	return s.repo.List(ctx)
}

func (s *RuleSourceService) Update(ctx context.Context, source *database.RuleSource) error {
	if err := s.Validate(source); err != nil {
		return err
	}
	current, err := s.repo.GetByID(ctx, source.ID)
	if err != nil {
		return err
	}
	if current.Name != source.Name {
		exists, err := s.repo.Exists(ctx, source.Name)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("规则源名称已存在: %s", source.Name)
		}
	}
	return s.repo.Update(ctx, source)
}

func (s *RuleSourceService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *RuleSourceService) Validate(source *database.RuleSource) error {
	if strings.TrimSpace(source.Name) == "" {
		return fmt.Errorf("规则源名称不能为空")
	}
	switch source.SourceFormat {
	case "surge", "clash-classical", "clash-domain", "clash-ipcidr", "domain-list", "ip-list":
	default:
		return fmt.Errorf("不支持的规则源格式: %s", source.SourceFormat)
	}
	if source.RefreshInterval <= 0 {
		source.RefreshInterval = 43200
	}
	// 对于手动维护的规则源（无 URL），自动禁用自动刷新
	if source.URL == "" {
		source.AutoRefresh = false
	}
	return nil
}
