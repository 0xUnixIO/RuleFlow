package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/ablate-ai/RuleFlow/database"
)

// ConfigPolicyService 配置策略服务
type ConfigPolicyService struct {
	policyRepo    *database.ConfigPolicyRepo
	accessLogRepo *database.ConfigAccessLogRepo
	subRepo       *database.SubscriptionRepo
	nodeRepo      *database.NodeRepo
}

// NewConfigPolicyService 创建配置策略服务
func NewConfigPolicyService(
	policyRepo *database.ConfigPolicyRepo,
	accessLogRepo *database.ConfigAccessLogRepo,
	subRepo *database.SubscriptionRepo,
	nodeRepo *database.NodeRepo,
) *ConfigPolicyService {
	return &ConfigPolicyService{
		policyRepo:    policyRepo,
		accessLogRepo: accessLogRepo,
		subRepo:       subRepo,
		nodeRepo:      nodeRepo,
	}
}

// Create 创建配置策略
func (s *ConfigPolicyService) Create(ctx context.Context, policy *database.ConfigPolicy) error {
	s.sanitizePolicyReferences(ctx, policy)
	if err := s.ValidateConfig(policy); err != nil {
		return err
	}

	// 验证目标类型
	if policy.Target != "clash-mihomo" && policy.Target != "stash" && policy.Target != "surge" && policy.Target != "sing-box" {
		return fmt.Errorf("不支持的目标类型: %s", policy.Target)
	}

	return s.policyRepo.Create(ctx, policy)
}

// GetByName 根据名称获取配置策略
func (s *ConfigPolicyService) GetByName(ctx context.Context, name string) (*database.ConfigPolicy, error) {
	return s.policyRepo.GetByName(ctx, name)
}

// GetByID 根据 ID 获取配置策略
func (s *ConfigPolicyService) GetByID(ctx context.Context, id int64) (*database.ConfigPolicy, error) {
	return s.policyRepo.GetByID(ctx, id)
}

// GetByToken 根据 token 获取配置策略
func (s *ConfigPolicyService) GetByToken(ctx context.Context, token string) (*database.ConfigPolicy, error) {
	return s.policyRepo.GetByToken(ctx, token)
}

// List 获取所有配置策略
func (s *ConfigPolicyService) List(ctx context.Context) ([]*database.ConfigPolicy, error) {
	return s.policyRepo.List(ctx)
}

// Update 更新配置策略
func (s *ConfigPolicyService) Update(ctx context.Context, policy *database.ConfigPolicy) error {
	s.sanitizePolicyReferences(ctx, policy)
	if err := s.ValidateConfig(policy); err != nil {
		return err
	}

	// 验证目标类型
	if policy.Target != "clash-mihomo" && policy.Target != "stash" && policy.Target != "surge" && policy.Target != "sing-box" {
		return fmt.Errorf("不支持的目标类型: %s", policy.Target)
	}

	return s.policyRepo.Update(ctx, policy)
}

// Delete 删除配置策略
func (s *ConfigPolicyService) Delete(ctx context.Context, id int64) error {
	return s.policyRepo.Delete(ctx, id)
}

// GetEnabled 获取所有启用的配置策略
func (s *ConfigPolicyService) GetEnabled(ctx context.Context) ([]*database.ConfigPolicy, error) {
	return s.policyRepo.GetEnabled(ctx)
}

// RecordAccess 记录配置访问日志，成功时同步刷新最近访问时间
func (s *ConfigPolicyService) RecordAccess(ctx context.Context, log *database.ConfigAccessLog) error {
	if s.accessLogRepo != nil {
		if err := s.accessLogRepo.Create(ctx, log); err != nil {
			return err
		}
	}

	if log.Success && log.PolicyID != nil {
		return s.policyRepo.TouchAccess(ctx, *log.PolicyID)
	}

	return nil
}

// ListAccessLogs 获取指定策略最近访问日志
func (s *ConfigPolicyService) ListAccessLogs(ctx context.Context, policyID int64) ([]*database.ConfigAccessLog, error) {
	if _, err := s.policyRepo.GetByID(ctx, policyID); err != nil {
		return nil, err
	}
	if s.accessLogRepo == nil {
		return []*database.ConfigAccessLog{}, nil
	}
	return s.accessLogRepo.ListByPolicy(ctx, policyID)
}

// ListAllAccessLogs 获取全局访问日志
func (s *ConfigPolicyService) ListAllAccessLogs(ctx context.Context, filter database.ConfigAccessLogFilter) ([]*database.ConfigAccessLog, error) {
	if filter.PolicyID != nil {
		if _, err := s.policyRepo.GetByID(ctx, *filter.PolicyID); err != nil {
			return nil, err
		}
	}
	if s.accessLogRepo == nil {
		return []*database.ConfigAccessLog{}, nil
	}
	return s.accessLogRepo.List(ctx, filter)
}

// ValidateConfig 验证配置策略
func (s *ConfigPolicyService) ValidateConfig(policy *database.ConfigPolicy) error {
	// 验证名称
	if policy.Name == "" {
		return fmt.Errorf("配置策略名称不能为空")
	}

	// 验证数据源（至少选一个订阅源或手动节点）
	if len(policy.SubscriptionIDs) == 0 && len(policy.NodeIDs) == 0 {
		return fmt.Errorf("至少需要选择一个订阅源或手动节点")
	}

	// 验证目标类型
	if policy.Target != "clash-mihomo" && policy.Target != "stash" && policy.Target != "surge" && policy.Target != "sing-box" {
		return fmt.Errorf("不支持的目标类型: %s (支持: clash-mihomo, stash, surge, sing-box)", policy.Target)
	}

	return nil
}

func (s *ConfigPolicyService) sanitizePolicyReferences(ctx context.Context, policy *database.ConfigPolicy) {
	policy.SubscriptionIDs = sanitizeExistingIDs(policy.SubscriptionIDs, func(id int64) bool {
		if s.subRepo == nil {
			return true
		}
		_, err := s.subRepo.GetByID(ctx, id)
		return err == nil
	})
	policy.NodeIDs = sanitizeExistingIDs(policy.NodeIDs, func(id int64) bool {
		if s.nodeRepo == nil {
			return true
		}
		_, err := s.nodeRepo.GetByID(ctx, id)
		return err == nil
	})
}

func sanitizeExistingIDs(ids []int64, exists func(int64) bool) []int64 {
	if len(ids) == 0 {
		return []int64{}
	}

	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if exists != nil && !exists(id) {
			continue
		}
		out = append(out, id)
	}
	if out == nil {
		return []int64{}
	}
	return out
}

// GetNodesForPolicy 获取策略对应的节点
func (s *ConfigPolicyService) GetNodesForPolicy(ctx context.Context, policy *database.ConfigPolicy) ([]database.Node, error) {
	// 收集所有节点
	allNodes := make([]database.Node, 0)

	// 从订阅源获取节点，并应用该订阅的过滤规则
	for _, subID := range policy.SubscriptionIDs {
		id := subID
		nodes, err := s.nodeRepo.List(ctx, database.NodeFilter{SourceID: &id})
		if err != nil {
			return nil, fmt.Errorf("获取节点失败 (订阅 ID: %d): %w", subID, err)
		}
		// 加载订阅过滤规则（失败不阻塞）
		if sub, err := s.subRepo.GetByID(ctx, subID); err == nil && sub.FilterRules != nil {
			nodes = applySubscriptionFilter(nodes, sub.FilterRules)
		}
		allNodes = append(allNodes, nodes...)
	}

	// 获取指定的手动节点
	if len(policy.NodeIDs) > 0 {
		nodes, err := s.nodeRepo.List(ctx, database.NodeFilter{IDs: policy.NodeIDs})
		if err != nil {
			return nil, fmt.Errorf("获取手动节点失败: %w", err)
		}
		allNodes = append(allNodes, nodes...)
	}

	// 应用节点过滤条件
	if policy.NodeFilters != nil && len(policy.NodeFilters) > 0 {
		allNodes = s.applyNodeFilters(allNodes, policy.NodeFilters)
	}

	return allNodes, nil
}

// GetUserInfoForPolicy 汇总策略关联的所有订阅流量信息
func (s *ConfigPolicyService) GetUserInfoForPolicy(ctx context.Context, policy *database.ConfigPolicy) *database.UserInfo {
	if s.subRepo == nil || len(policy.SubscriptionIDs) == 0 {
		return nil
	}
	var hasInfo bool
	result := &database.UserInfo{}
	for _, subID := range policy.SubscriptionIDs {
		sub, err := s.subRepo.GetByID(ctx, subID)
		if err != nil || sub.UserInfo == nil {
			continue
		}
		hasInfo = true
		result.Upload += sub.UserInfo.Upload
		result.Download += sub.UserInfo.Download
		result.Total += sub.UserInfo.Total
		// 取最早到期时间
		if sub.UserInfo.Expire != nil {
			if result.Expire == nil || *sub.UserInfo.Expire < *result.Expire {
				exp := *sub.UserInfo.Expire
				result.Expire = &exp
			}
		}
	}
	if !hasInfo {
		return nil
	}
	return result
}

// applyNodeFilters 应用节点过滤条件
func (s *ConfigPolicyService) applyNodeFilters(nodes []database.Node, filters map[string]interface{}) []database.Node {
	filtered := make([]database.Node, 0, len(nodes))

	for _, node := range nodes {
		// 默认包含节点
		include := true

		// 按协议筛选
		if protocols, ok := filters["protocols"].([]interface{}); ok {
			protoSet := make(map[string]bool)
			for _, p := range protocols {
				if protoStr, ok := p.(string); ok {
					protoSet[protoStr] = true
				}
			}
			if len(protoSet) > 0 && !protoSet[node.Protocol] {
				include = false
			}
		}

		// 按关键词筛选
		if keywords, ok := filters["keywords"].([]interface{}); ok && include {
			matches := false
			nodeNameLower := strings.ToLower(node.Name)
			for _, kw := range keywords {
				if keywordStr, ok := kw.(string); ok {
					if strings.Contains(nodeNameLower, strings.ToLower(keywordStr)) {
						matches = true
						break
					}
				}
			}
			if len(keywords) > 0 && !matches {
				include = false
			}
		}

		// 按标签筛选
		if tags, ok := filters["tags"].([]interface{}); ok && include {
			tagSet := make(map[string]bool)
			for _, t := range tags {
				if tagStr, ok := t.(string); ok {
					tagSet[tagStr] = true
				}
			}
			if len(tagSet) > 0 {
				hasTag := false
				for _, nodeTag := range node.Tags {
					if tagSet[nodeTag] {
						hasTag = true
						break
					}
				}
				if !hasTag {
					include = false
				}
			}
		}

		// 只包含启用的节点
		if include && !node.Enabled {
			include = false
		}

		if include {
			filtered = append(filtered, node)
		}
	}

	return filtered
}

// ApplyNodeFilters 应用节点过滤条件（保留向后兼容）
func (s *ConfigPolicyService) ApplyNodeFilters(nodes []*database.ConfigPolicy, filters map[string]interface{}) []*database.ConfigPolicy {
	// 这里保留原有的方法签名以向后兼容
	// 实际的过滤逻辑现在使用 applyNodeFilters 方法处理 Node 类型
	return nodes
}

// GetPolicyWithNodes 获取策略及其节点
func (s *ConfigPolicyService) GetPolicyWithNodes(ctx context.Context, policyName string) (*database.ConfigPolicy, []database.Node, error) {
	policy, err := s.GetByName(ctx, policyName)
	if err != nil {
		return nil, nil, err
	}

	nodes, err := s.GetNodesForPolicy(ctx, policy)
	if err != nil {
		return nil, nil, err
	}

	return policy, nodes, nil
}

// applySubscriptionFilter 按订阅级过滤规则筛选节点
func applySubscriptionFilter(nodes []database.Node, f *database.SubscriptionFilter) []database.Node {
	if f == nil {
		return nodes
	}

	// 预编译正则（为空则跳过）
	var excludeRe *regexp.Regexp
	if f.ExcludeRegex != "" {
		if re, err := regexp.Compile(f.ExcludeRegex); err == nil {
			excludeRe = re
		}
	}

	// 协议白名单集合
	protoWhitelist := make(map[string]bool, len(f.IncludeProtocols))
	for _, p := range f.IncludeProtocols {
		protoWhitelist[p] = true
	}

	filtered := make([]database.Node, 0, len(nodes))
	for _, node := range nodes {
		nameLower := strings.ToLower(node.Name)

		// 排除关键词（命中任一即排除）
		excluded := false
		for _, kw := range f.ExcludeKeywords {
			if strings.Contains(nameLower, strings.ToLower(kw)) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		// 排除正则
		if excludeRe != nil && excludeRe.MatchString(node.Name) {
			continue
		}

		// 协议白名单（非空时过滤）
		if len(protoWhitelist) > 0 && !protoWhitelist[node.Protocol] {
			continue
		}

		filtered = append(filtered, node)
	}
	return filtered
}
