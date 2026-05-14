package services

import (
	"context"
	"fmt"

	"github.com/ablate-ai/RuleFlow/database"
)

// NodeService 节点服务
type NodeService struct {
	repo *database.NodeRepo
}

// NewNodeService 创建节点服务
func NewNodeService(repo *database.NodeRepo) *NodeService {
	return &NodeService{repo: repo}
}

// AddManualNode 手动添加节点
func (s *NodeService) AddManualNode(ctx context.Context, node *database.Node) error {
	// 验证协议类型
	if !isValidProtocol(node.Protocol) {
		return fmt.Errorf("不支持的协议类型: %s", node.Protocol)
	}

	// 手动节点不关联订阅
	node.SourceID = nil

	// 默认启用
	if !node.Enabled {
		node.Enabled = true
	}

	// 如果没有标签，初始化为空数组
	if node.Tags == nil {
		node.Tags = []string{}
	}

	return s.repo.Create(ctx, node)
}

// ImportManualNode 手动导入节点；若节点已存在则更新协议与配置。
func (s *NodeService) ImportManualNode(ctx context.Context, node *database.Node) (bool, error) {
	// 验证协议类型
	if !isValidProtocol(node.Protocol) {
		return false, fmt.Errorf("不支持的协议类型: %s", node.Protocol)
	}

	node.SourceID = nil
	if !node.Enabled {
		node.Enabled = true
	}
	if node.Tags == nil {
		node.Tags = []string{}
	}

	return s.repo.UpsertManualImported(ctx, node)
}

// ListNodes 列出节点
func (s *NodeService) ListNodes(ctx context.Context, filter database.NodeFilter) ([]database.Node, error) {
	return s.repo.List(ctx, filter)
}

// GetNode 获取节点详情
func (s *NodeService) GetNode(ctx context.Context, id int64) (*database.Node, error) {
	return s.repo.GetByID(ctx, id)
}

// UpdateNode 更新节点
func (s *NodeService) UpdateNode(ctx context.Context, id int64, node *database.Node) error {
	// 验证协议类型
	if !isValidProtocol(node.Protocol) {
		return fmt.Errorf("不支持的协议类型: %s", node.Protocol)
	}

	// 确保不能修改来源
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	node.SourceID = existing.SourceID
	node.ID = id

	return s.repo.Update(ctx, node)
}

// DeleteNode 删除节点
func (s *NodeService) DeleteNode(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// BatchEnable 批量启用/禁用节点
func (s *NodeService) BatchEnable(ctx context.Context, ids []int64, enabled bool) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	return s.repo.BatchUpdateEnabled(ctx, ids, enabled)
}

// GetNodesBySubscription 获取指定订阅的所有节点
func (s *NodeService) GetNodesBySubscription(ctx context.Context, subscriptionName string) ([]database.Node, error) {
	nodes, err := s.repo.List(ctx, database.NodeFilter{})
	if err != nil {
		return nil, err
	}

	result := make([]database.Node, 0)
	for _, node := range nodes {
		if node.SourceID != nil && node.SourceName == subscriptionName {
			result = append(result, node)
		}
	}

	return result, nil
}

// GetManualNodes 获取所有手动添加的节点
func (s *NodeService) GetManualNodes(ctx context.Context) ([]database.Node, error) {
	filter := database.NodeFilter{
		ManualOnly: true,
	}

	return s.repo.List(ctx, filter)
}

// GetEnabledNodes 获取所有启用的节点
func (s *NodeService) GetEnabledNodes(ctx context.Context) ([]database.Node, error) {
	enabled := true
	filter := database.NodeFilter{
		Enabled: &enabled,
	}

	return s.repo.List(ctx, filter)
}

// GetNodesByProtocol 获取指定协议的节点
func (s *NodeService) GetNodesByProtocol(ctx context.Context, protocol string) ([]database.Node, error) {
	filter := database.NodeFilter{
		Protocol: protocol,
	}

	return s.repo.List(ctx, filter)
}

// isValidProtocol 验证协议类型是否有效
func isValidProtocol(protocol string) bool {
	validProtocols := map[string]bool{
		"trojan":    true,
		"vmess":     true,
		"vless":     true,
		"ss":        true,
		"wireguard": true,
		"anytls":    true,
		"hysteria2": true,
		"tuic":      true,
	}

	return validProtocols[protocol]
}

// ValidateNode 验证节点数据
func (s *NodeService) ValidateNode(node *database.Node) error {
	// 验证名称
	if node.Name == "" {
		return fmt.Errorf("节点名称不能为空")
	}

	// 验证协议
	if !isValidProtocol(node.Protocol) {
		return fmt.Errorf("不支持的协议类型: %s", node.Protocol)
	}

	// 验证服务器地址
	if node.Server == "" {
		return fmt.Errorf("服务器地址不能为空")
	}

	// 验证端口
	if node.Port <= 0 || node.Port > 65535 {
		return fmt.Errorf("端口号无效: %d", node.Port)
	}

	// 验证配置
	if node.Config == nil {
		return fmt.Errorf("节点配置不能为空")
	}

	return nil
}

// GetNodeStats 获取节点统计信息
func (s *NodeService) GetNodeStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总节点数
	allNodes, err := s.repo.List(ctx, database.NodeFilter{})
	if err != nil {
		return nil, fmt.Errorf("获取节点列表失败: %w", err)
	}
	stats["total"] = len(allNodes)

	// 启用的节点数
	enabled := true
	enabledNodes, err := s.repo.List(ctx, database.NodeFilter{Enabled: &enabled})
	if err != nil {
		return nil, fmt.Errorf("获取启用节点失败: %w", err)
	}
	stats["enabled"] = len(enabledNodes)

	// 按协议统计
	protocolCounts := make(map[string]int)
	for _, node := range allNodes {
		protocolCounts[node.Protocol]++
	}
	stats["by_protocol"] = protocolCounts

	// 按来源统计
	sourceCounts := make(map[string]int)
	for _, node := range allNodes {
		if node.SourceID == nil {
			sourceCounts["manual"]++
			continue
		}
		key := node.SourceName
		if key == "" {
			key = "subscription"
		}
		sourceCounts[key]++
	}
	stats["by_source"] = sourceCounts

	return stats, nil
}
