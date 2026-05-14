package app

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// --- yaml.Node 操作辅助函数 ---

// yamlMappingNode 从文档节点取出顶层 MappingNode
func yamlMappingNode(doc *yaml.Node) *yaml.Node { return YAMLRootMappingNode(doc) }

// yamlFindInMapping 在 MappingNode 中查找 key，返回 value 节点（未找到返回 nil）
func yamlFindInMapping(mapping *yaml.Node, key string) *yaml.Node {
	return YAMLLookupMappingValue(mapping, key)
}

// yamlHasKey 检查 MappingNode 中是否存在某个 key
func yamlHasKey(mapping *yaml.Node, key string) bool {
	return yamlFindInMapping(mapping, key) != nil
}

// yamlDeleteFromMapping 从 MappingNode 中删除指定 key（及其 value）
func yamlDeleteFromMapping(mapping *yaml.Node, key string) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

// yamlSetInMapping 在 MappingNode 中设置或新增 key-value
func yamlSetInMapping(mapping *yaml.Node, key string, value *yaml.Node) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"}
	mapping.Content = append(mapping.Content, keyNode, value)
}

// yamlValueToNode 将任意 Go 值序列化为 yaml.Node（用于将修改后的数据写回文档树）
func yamlValueToNode(v interface{}) (*yaml.Node, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0], nil
	}
	return &doc, nil
}

func flattenWireGuardProxyForStash(proxy Proxy) Proxy {
	if proxy.Type != "wireguard" || len(proxy.Peers) == 0 {
		return proxy
	}

	flattened := proxy
	peer := proxy.Peers[0]
	flattened.Server = peer.Server
	flattened.Port = peer.Port
	flattened.PublicKey = peer.PublicKey
	flattened.PreSharedKey = peer.PreSharedKey
	flattened.Reserved = append([]int(nil), peer.Reserved...)
	flattened.PersistentKeepalive = peer.PersistentKeepalive
	flattened.Peers = nil
	return flattened
}

func proxyPayloadForTarget(proxies []Proxy, target string) interface{} {
	adapted := make([]Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		switch target {
		case "stash":
			adapted = append(adapted, flattenWireGuardProxyForStash(proxy))
		case "clash-mihomo":
			// Clash Mihomo VLESS 使用 servername 字段，Stash 使用 sni
			if proxy.Type == "vless" && proxy.SNI != "" {
				proxy.Servername = proxy.SNI
				proxy.SNI = ""
			}
			adapted = append(adapted, proxy)
		default:
			adapted = append(adapted, proxy)
		}
	}
	return adapted
}

// adaptConfigForTargetNode 根据目标客户端类型调整 yaml.Node 树中的配置
// 使用 yaml.Node 操作，保留原始 YAML 格式（包括引号风格）
func adaptConfigForTargetNode(mapping *yaml.Node, target string) {
	if target != "stash" {
		return
	}
	// 移除 Clash 专属字段
	for _, key := range []string{"port", "socks-port", "redir-port", "mixed-port", "allow-lan", "external-controller", "secret", "tun"} {
		yamlDeleteFromMapping(mapping, key)
	}
	// 调整 DNS 配置
	dnsNode := yamlFindInMapping(mapping, "dns")
	if dnsNode != nil && dnsNode.Kind == yaml.MappingNode {
		yamlDeleteFromMapping(dnsNode, "prefer-h3")
		yamlDeleteFromMapping(dnsNode, "fake-ip-filter-mode")
		if !yamlHasKey(dnsNode, "fake-ip-filter") {
			filterNode, _ := yamlValueToNode([]string{"*.lan", "*.local"})
			yamlSetInMapping(dnsNode, "fake-ip-filter", filterNode)
		}
	}
}

// adaptConfigForTarget 根据目标客户端类型调整配置（仅用于 BuildYAMLFromDefaultTemplate）
func adaptConfigForTarget(cfg map[string]interface{}, target string) {
	if target == "stash" {
		// 移除 Clash 特定的端口设置
		delete(cfg, "port")
		delete(cfg, "socks-port")
		delete(cfg, "redir-port")
		delete(cfg, "mixed-port")
		delete(cfg, "allow-lan")
		delete(cfg, "external-controller")
		delete(cfg, "secret")
		delete(cfg, "tun")

		if dns, ok := cfg["dns"].(map[string]interface{}); ok {
			delete(dns, "prefer-h3")
			delete(dns, "fake-ip-filter-mode")
			if _, hasFakeIP := dns["fake-ip-filter"]; !hasFakeIP {
				dns["fake-ip-filter"] = []string{"*.lan", "*.local"}
			}
		}
	}
}

func adaptProxyGroupsForTarget(groups []interface{}, target string) {
	for _, item := range groups {
		groupMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		switch target {
		case "stash":
			if url, exists := groupMap["url"]; exists {
				groupMap["benchmark-url"] = url
				delete(groupMap, "url")
			}
		case "clash-mihomo":
			if benchmarkURL, exists := groupMap["benchmark-url"]; exists {
				groupMap["url"] = benchmarkURL
				delete(groupMap, "benchmark-url")
			}
		}
	}
}

func adaptTemplateProxyGroups(raw interface{}, nodeNames []string) ([]interface{}, map[string]string, error) {
	groupList, ok := raw.([]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("proxy-groups 格式无效")
	}

	known := make(map[string]struct{}, len(nodeNames)+len(groupList))
	for _, n := range nodeNames {
		known[n] = struct{}{}
	}

	// 按顺序收集 group 名，用于 dialer-proxy 匹配（group 名优先于节点名）
	groupNames := make([]string, 0, len(groupList))
	for _, item := range groupList {
		groupMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := groupMap["name"].(string)
		if name != "" {
			known[name] = struct{}{}
			groupNames = append(groupNames, name)
		}
	}

	// findRelay 在 groupNames 和 nodeNames 中找到第一个匹配正则的名称
	// 优先匹配 group 名，便于 Stash 通过 group 灵活切换中转节点
	findRelay := func(pattern string) string {
		if pattern == "" {
			return ""
		}
		matcher, err := compileNodeNameMatcher(pattern)
		if err != nil {
			return ""
		}
		for _, n := range groupNames {
			if matcher(n) {
				return n
			}
		}
		for _, n := range nodeNames {
			if matcher(n) {
				return n
			}
		}
		return ""
	}

	// dialerMap: 节点名 -> 中转节点名
	dialerMap := make(map[string]string)

	for i, item := range groupList {
		groupMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		groupType, _ := groupMap["type"].(string)
		rawProxies, ok := groupMap["proxies"].([]interface{})
		if !ok {
			continue
		}

		// 读取并移除 filter 字段，用于服务端按正则过滤节点
		filterPattern, _ := groupMap["filter"].(string)
		delete(groupMap, "filter")
		var filterMatcher func(string) bool
		if filterPattern != "" {
			filterMatcher, _ = compileNodeNameMatcher(filterPattern)
		}

		// 读取并移除 dialer-proxy 字段，找到第一个匹配的中转名（group 优先于节点）
		dialerPattern, _ := groupMap["dialer-proxy"].(string)
		delete(groupMap, "dialer-proxy")
		relayName := findRelay(dialerPattern)

		// filterNodes 根据正则过滤节点列表
		filterNodes := func(names []string) []string {
			if filterMatcher == nil {
				return names
			}
			result := make([]string, 0, len(names))
			for _, n := range names {
				if filterMatcher(n) {
					result = append(result, n)
				}
			}
			return result
		}

		filtered := make([]string, 0, len(rawProxies))
		for _, p := range rawProxies {
			name, ok := p.(string)
			if !ok || strings.TrimSpace(name) == "" {
				continue
			}

			if name == "__NODES__" {
				filtered = append(filtered, filterNodes(nodeNames)...)
				continue
			}
			if _, exists := known[name]; exists || builtInProxyName(name) {
				filtered = append(filtered, name)
			}
		}

		if len(filtered) == 0 {
			switch strings.ToLower(groupType) {
			case "select", "url-test", "fallback", "load-balance":
				candidates := filterNodes(nodeNames)
				if len(candidates) > 0 {
					filtered = append(filtered, candidates...)
				} else {
					filtered = append(filtered, "DIRECT")
				}
			default:
				filtered = append(filtered, "DIRECT")
			}
		}

		filtered = DedupeStrings(filtered)

		// 收集 dialer-proxy 映射：该 group 内的节点 -> 中转节点
		if relayName != "" {
			for _, nodeName := range filtered {
				// 只对真实代理节点（非内置名）注入
				if !builtInProxyName(nodeName) {
					dialerMap[nodeName] = relayName
				}
			}
		}

		converted := make([]interface{}, 0, len(filtered))
		for _, p := range filtered {
			converted = append(converted, p)
		}
		groupMap["proxies"] = converted
		groupList[i] = groupMap
	}

	return groupList, dialerMap, nil
}

// BuildYAMLFromTemplateContent 从模板内容（字符串）构建 YAML 配置
func BuildYAMLFromTemplateContent(nodes []*ProxyNode, templateContent string, target string) (string, error) {
	if target != "clash-mihomo" && target != "stash" {
		return "", fmt.Errorf("不支持的目标类型: %s (支持: clash-mihomo, stash)", target)
	}

	// 用 yaml.Node 解析，保留原始格式（含引号风格）
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(templateContent), &doc); err != nil {
		return "", fmt.Errorf("解析模板内容失败: %w", err)
	}
	mapping := yamlMappingNode(&doc)
	if mapping == nil {
		return "", fmt.Errorf("模板内容格式无效：根节点不是映射")
	}

	adaptConfigForTargetNode(mapping, target)

	proxies, nodeNames := buildProxies(nodes)
	proxiesNode, err := yamlValueToNode(proxyPayloadForTarget(proxies, target))
	if err != nil {
		return "", fmt.Errorf("生成 proxies 失败: %w", err)
	}
	yamlSetInMapping(mapping, "proxies", proxiesNode)

	rawGroupsNode := yamlFindInMapping(mapping, "proxy-groups")
	if rawGroupsNode != nil {
		var rawGroups interface{}
		if err := rawGroupsNode.Decode(&rawGroups); err == nil {
			adaptedGroups, dialerMap, err := adaptTemplateProxyGroups(rawGroups, nodeNames)
			if err != nil {
				return "", err
			}
			adaptProxyGroupsForTarget(adaptedGroups, target)
			// 将 dialer-proxy 注入到对应 proxies 条目
			if len(dialerMap) > 0 {
				for i := range proxies {
					if relay, ok := dialerMap[proxies[i].Name]; ok {
						proxies[i].DialerProxy = relay
					}
				}
				proxiesNode, err = yamlValueToNode(proxyPayloadForTarget(proxies, target))
				if err != nil {
					return "", fmt.Errorf("生成 proxies 失败: %w", err)
				}
				yamlSetInMapping(mapping, "proxies", proxiesNode)
			}
			groupsNode, err := yamlValueToNode(adaptedGroups)
			if err != nil {
				return "", fmt.Errorf("生成 proxy-groups 失败: %w", err)
			}
			yamlSetInMapping(mapping, "proxy-groups", groupsNode)
		}
	} else {
		defaultGroups := []Group{
			{Name: "🚀 节点选择", Type: "select", Proxies: append([]string{"♻️ 自动选择", "DIRECT"}, nodeNames...)},
			{Name: "♻️ 自动选择", Type: "url-test", Proxies: nodeNames},
		}
		groupsNode, err := yamlValueToNode(defaultGroups)
		if err != nil {
			return "", fmt.Errorf("生成 proxy-groups 失败: %w", err)
		}
		yamlSetInMapping(mapping, "proxy-groups", groupsNode)
	}

	if !yamlHasKey(mapping, "rules") {
		rulesNode, err := yamlValueToNode(cloneRules(defaultRules))
		if err != nil {
			return "", fmt.Errorf("生成 rules 失败: %w", err)
		}
		yamlSetInMapping(mapping, "rules", rulesNode)
	}
	pruneYAMLRulesWithMissingPolicies(mapping)

	yamlData, err := yaml.Marshal(&doc)
	if err != nil {
		return "", fmt.Errorf("生成配置失败")
	}
	return string(yamlData), nil
}
func BuildYAMLFromDefaultTemplate(nodes []*ProxyNode, target string) (string, error) {
	if target != "clash-mihomo" && target != "stash" {
		return "", fmt.Errorf("不支持的目标类型: %s (支持: clash-mihomo, stash)", target)
	}

	cfg := map[string]interface{}{}
	adaptConfigForTarget(cfg, target)

	proxies, nodeNames := buildProxies(nodes)
	cfg["proxies"] = proxyPayloadForTarget(proxies, target)
	cfg["proxy-groups"] = []Group{
		{
			Name:    "🚀 节点选择",
			Type:    "select",
			Proxies: append([]string{"♻️ 自动选择", "DIRECT"}, nodeNames...),
		},
		{
			Name:    "♻️ 自动选择",
			Type:    "url-test",
			Proxies: nodeNames,
		},
	}
	cfg["rules"] = cloneRules(defaultRules)
	pruneYAMLRuleStrings(cfg)

	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("生成配置失败")
	}
	return string(yamlData), nil
}

func pruneYAMLRulesWithMissingPolicies(mapping *yaml.Node) {
	validPolicies := yamlKnownPolicyNames(mapping)
	if len(validPolicies) == 0 {
		return
	}

	rulesNode := yamlFindInMapping(mapping, "rules")
	if rulesNode == nil {
		return
	}

	var rules []string
	if err := rulesNode.Decode(&rules); err != nil {
		return
	}

	pruned := pruneYAMLRuleStringsWithPolicies(rules, validPolicies)
	newRulesNode, err := yamlValueToNode(pruned)
	if err != nil {
		return
	}
	yamlSetInMapping(mapping, "rules", newRulesNode)
}

func pruneYAMLRuleStrings(cfg map[string]interface{}) {
	validPolicies := make(map[string]struct{})

	if proxies, ok := cfg["proxies"].([]Proxy); ok {
		for _, proxy := range proxies {
			if proxy.Name != "" {
				validPolicies[proxy.Name] = struct{}{}
			}
		}
	}
	if groups, ok := cfg["proxy-groups"].([]Group); ok {
		for _, group := range groups {
			if group.Name != "" {
				validPolicies[group.Name] = struct{}{}
			}
		}
	}
	if len(validPolicies) == 0 {
		return
	}

	rules, ok := cfg["rules"].([]string)
	if !ok {
		return
	}
	cfg["rules"] = pruneYAMLRuleStringsWithPolicies(rules, validPolicies)
}

func pruneYAMLRuleStringsWithPolicies(rules []string, validPolicies map[string]struct{}) []string {
	pruned := make([]string, 0, len(rules))
	for _, rule := range rules {
		policy := yamlRulePolicy(rule)
		if policy == "" || builtInProxyName(policy) {
			pruned = append(pruned, rule)
			continue
		}
		if _, exists := validPolicies[policy]; exists {
			pruned = append(pruned, rule)
		}
	}
	return pruned
}

func yamlKnownPolicyNames(mapping *yaml.Node) map[string]struct{} {
	known := make(map[string]struct{})

	proxiesNode := yamlFindInMapping(mapping, "proxies")
	if proxiesNode != nil {
		var proxies []map[string]interface{}
		if err := proxiesNode.Decode(&proxies); err == nil {
			for _, proxy := range proxies {
				if name, _ := proxy["name"].(string); name != "" {
					known[name] = struct{}{}
				}
			}
		}
	}

	groupsNode := yamlFindInMapping(mapping, "proxy-groups")
	if groupsNode != nil {
		var groups []map[string]interface{}
		if err := groupsNode.Decode(&groups); err == nil {
			for _, group := range groups {
				if name, _ := group["name"].(string); name != "" {
					known[name] = struct{}{}
				}
			}
		}
	}

	return known
}

func yamlRulePolicy(rule string) string {
	parts := strings.Split(rule, ",")
	if len(parts) < 2 {
		return ""
	}

	ruleType := strings.ToUpper(strings.TrimSpace(parts[0]))
	switch ruleType {
	case "MATCH":
		return strings.TrimSpace(parts[1])
	default:
		return strings.TrimSpace(parts[len(parts)-1])
	}
}

