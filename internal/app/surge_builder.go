package app

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"

	"github.com/dlclark/regexp2"
)

// BuildSurgeFromTemplateContent 从模板内容（字符串）构建 Surge INI 配置
func BuildSurgeFromTemplateContent(nodes []*ProxyNode, templateContent string) (string, error) {
	clonedNodes := cloneProxyNodes(nodes)

	// Surge 不支持 vless 协议，过滤掉
	filtered := clonedNodes[:0]
	for _, n := range clonedNodes {
		if n.Protocol != "vless" {
			filtered = append(filtered, n)
		}
	}
	clonedNodes = filtered

	// 收集节点名称
	nodeNames := make([]string, 0, len(clonedNodes))
	nodeNameSet := make(map[string]struct{}, len(clonedNodes))
	nodeAliases := make(map[string]string, len(clonedNodes)*2)
	for i, node := range clonedNodes {
		name := ensureNodeName(node, i)
		nodeNames = append(nodeNames, name)
		nodeNameSet[name] = struct{}{}
		nodeAliases[name] = name
		rawName := strings.TrimSpace(node.Name)
		if rawName != "" {
			nodeAliases[rawName] = name
		}
	}

	relayMap := surgeTemplateDialerMap(templateContent, nodeNames, nodeNameSet, nodeAliases)
	for i, node := range clonedNodes {
		name := ensureNodeName(node, i)
		if relay, ok := relayMap[name]; ok {
			node.DialerProxy = relay
		}
	}

	// 渲染最终节点行
	nodeLines := make([]string, 0, len(clonedNodes))
	wireGuardSections := make([]string, 0)
	for i, node := range clonedNodes {
		name := ensureNodeName(node, i)
		nodeLines = append(nodeLines, surgeProxyLine(node, name))
		if section := surgeWireGuardSection(node, name); section != "" {
			wireGuardSections = append(wireGuardSections, section)
		}
	}

	lines := strings.Split(templateContent, "\n")
	var out []string

	// 当前所在的 section
	section := ""
	// 用于 [Proxy] section：记录是否已找到 __NODES__ 占位行
	proxyNodesInserted := false
	wireGuardSectionsInserted := false
	// 缓存 [Proxy] section 末尾插入位置
	pendingProxyLines := []string(nil)

	for idx, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")

		// 识别 section 头
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			// 离开 [Proxy] section 时，若未找到 __NODES__ 则在末尾追加节点
			if section == "[Proxy]" && !proxyNodesInserted {
				out = append(out, pendingProxyLines...)
				out = append(out, nodeLines...)
				out = append(out, wireGuardSections...)
				proxyNodesInserted = true
				wireGuardSectionsInserted = true
			} else if section == "[Proxy]" && len(wireGuardSections) > 0 && !wireGuardSectionsInserted {
				out = append(out, wireGuardSections...)
				wireGuardSectionsInserted = true
			}
			pendingProxyLines = nil

			section = trimmed
			out = append(out, line)
			continue
		}

		_ = idx // 消除 unused 警告

		switch section {
		case "[Proxy]":
			if strings.TrimSpace(line) == "__NODES__" {
				// 替换占位符为所有节点行
				out = append(out, nodeLines...)
				proxyNodesInserted = true
			} else {
				// 暂存非占位行，以便在 section 结束时判断是否需要追加
				if !proxyNodesInserted {
					pendingProxyLines = append(pendingProxyLines, line)
				}
				out = append(out, line)
			}

		case "[Proxy Group]":
			_, filterPattern, excludeFilterPattern, _, sanitizedLine := parseSurgeProxyGroupLine(line)
			if strings.Contains(line, "__NODES__") {
				filtered := filterNodesByPattern(nodeNames, filterPattern, excludeFilterPattern)
				filtered = filterExistingNodeNames(filtered, nodeNameSet)
				expansion := strings.Join(filtered, ", ")
				line = strings.ReplaceAll(sanitizedLine, "__NODES__", expansion)
				line = cleanCommas(line)
				out = append(out, line)
				continue
			}
			out = append(out, cleanCommas(sanitizedLine))

		default:
			out = append(out, line)
		}
	}

	// 文件末尾若仍在 [Proxy] section 且未插入节点
	if section == "[Proxy]" && !proxyNodesInserted {
		out = append(out, pendingProxyLines...)
		out = append(out, nodeLines...)
		out = append(out, wireGuardSections...)
		wireGuardSectionsInserted = true
	}

	out = pruneSurgeRulesWithMissingPolicies(out)
	return strings.Join(out, "\n"), nil
}

func cloneProxyNodes(nodes []*ProxyNode) []*ProxyNode {
	cloned := make([]*ProxyNode, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		copyNode := *node
		cloned = append(cloned, &copyNode)
	}
	return cloned
}

func surgeTemplateDialerMap(templateContent string, nodeNames []string, nodeNameSet map[string]struct{}, nodeAliases map[string]string) map[string]string {
	lines := strings.Split(templateContent, "\n")
	groupNames := collectSurgeProxyGroupNames(lines)
	relayMap := make(map[string]string)

	section := ""
	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = trimmed
			continue
		}
		if section != "[Proxy Group]" {
			continue
		}

		_, filterPattern, excludeFilterPattern, relayPattern, sanitizedLine := parseSurgeProxyGroupLine(line)
		if relayPattern == "" {
			continue
		}
		relayName := findSurgeRelay(relayPattern, groupNames, nodeNames)
		if relayName == "" {
			continue
		}
		members := surgeGroupNodeMembers(sanitizedLine, nodeNames, nodeNameSet, nodeAliases, filterPattern, excludeFilterPattern)
		for _, nodeName := range members {
			relayMap[nodeName] = relayName
		}
	}

	return relayMap
}

func collectSurgeProxyGroupNames(lines []string) []string {
	groupNames := make([]string, 0)
	seen := make(map[string]struct{})
	section := ""
	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = trimmed
			continue
		}
		if section != "[Proxy Group]" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if name, ok := surgePolicyName(line); ok {
			if _, exists := seen[name]; !exists {
				seen[name] = struct{}{}
				groupNames = append(groupNames, name)
			}
		}
	}
	return groupNames
}

func parseSurgeProxyGroupLine(line string) (groupName string, filterPattern string, excludeFilterPattern string, relayPattern string, sanitized string) {
	sanitized = line
	groupName, _ = surgePolicyName(line)
	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:^|,)\s*exclude-filter\s*=\s*([^,\r\n]+)`),
		regexp.MustCompile(`(?i)(?:^|,)\s*policy-regex-filter\s*=\s*([^,\r\n]+)`),
		regexp.MustCompile(`(?i)(?:^|,)\s*dialer-proxy\s*=\s*([^,\r\n]+)`),
		regexp.MustCompile(`(?i)(?:^|,)\s*underlying-proxy\s*=\s*([^,\r\n]+)`),
	} {
		match := pattern.FindStringSubmatch(sanitized)
		if match == nil {
			continue
		}
		value := strings.TrimSpace(match[1])
		key := strings.ToLower(strings.TrimSpace(strings.SplitN(match[0], "=", 2)[0]))
		switch {
		case strings.Contains(key, "exclude-filter"):
			excludeFilterPattern = value
		case strings.Contains(key, "policy-regex-filter"):
			filterPattern = value
		case strings.Contains(key, "dialer-proxy"), strings.Contains(key, "underlying-proxy"):
			relayPattern = value
		}
		sanitized = pattern.ReplaceAllString(sanitized, "")
	}
	return groupName, filterPattern, excludeFilterPattern, relayPattern, sanitized
}

func surgeGroupNodeMembers(line string, nodeNames []string, nodeNameSet map[string]struct{}, nodeAliases map[string]string, filterPattern string, excludeFilterPattern string) []string {
	members := make([]string, 0)
	if strings.Contains(line, "__NODES__") {
		members = append(members, filterNodesByPattern(nodeNames, filterPattern, excludeFilterPattern)...)
	}

	right, ok := surgePolicyRightPart(line)
	if !ok {
		return DedupeStrings(filterExistingNodeNames(members, nodeNameSet))
	}
	parts := strings.Split(right, ",")
	for i, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" || i == 0 || token == "__NODES__" || strings.Contains(token, "=") {
			continue
		}
		if finalName, exists := nodeAliases[token]; exists {
			members = append(members, finalName)
		}
	}

	return DedupeStrings(filterExistingNodeNames(members, nodeNameSet))
}

func surgePolicyName(line string) (string, bool) {
	left, _, found := strings.Cut(line, "=")
	if !found {
		return "", false
	}
	name := strings.TrimSpace(left)
	if name == "" {
		return "", false
	}
	return name, true
}

func pruneSurgeRulesWithMissingPolicies(lines []string) []string {
	validPolicies := collectSurgeKnownPolicies(lines)
	if len(validPolicies) == 0 {
		return lines
	}

	out := make([]string, 0, len(lines))
	section := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = trimmed
			out = append(out, line)
			continue
		}

		if section == "[Rule]" {
			if policy, ok := surgeRulePolicy(line); ok {
				if !builtInProxyName(policy) {
					if _, exists := validPolicies[policy]; !exists {
						continue
					}
				}
			}
		}

		out = append(out, line)
	}

	return out
}

func collectSurgeKnownPolicies(lines []string) map[string]struct{} {
	known := make(map[string]struct{})
	section := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = trimmed
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}

		switch section {
		case "[Proxy]", "[Proxy Group]":
			if name, ok := surgePolicyName(line); ok {
				known[name] = struct{}{}
			}
		}
	}
	return known
}

func surgeRulePolicy(line string) (string, bool) {
	trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
		return "", false
	}

	parts := strings.Split(trimmed, ",")
	if len(parts) < 2 {
		return "", false
	}
	if strings.EqualFold(strings.TrimSpace(parts[0]), "FINAL") {
		return "", false
	}

	policy := strings.TrimSpace(parts[2-1])
	if len(parts) >= 3 {
		policy = strings.TrimSpace(parts[2])
	}
	return policy, policy != ""
}

func surgePolicyRightPart(line string) (string, bool) {
	_, right, found := strings.Cut(line, "=")
	if !found {
		return "", false
	}
	right = strings.TrimSpace(right)
	if right == "" {
		return "", false
	}
	return right, true
}

func findSurgeRelay(pattern string, groupNames []string, nodeNames []string) string {
	if pattern == "" {
		return ""
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ""
	}
	for _, name := range groupNames {
		if re.MatchString(name) {
			return name
		}
	}
	for _, name := range nodeNames {
		if re.MatchString(name) {
			return name
		}
	}
	return ""
}

func filterExistingNodeNames(names []string, known map[string]struct{}) []string {
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if _, ok := known[name]; ok {
			filtered = append(filtered, name)
		}
	}
	return filtered
}

// BuildSurgeFromDefaultTemplate 使用内置默认结构生成最小 Surge 配置
func BuildSurgeFromDefaultTemplate(nodes []*ProxyNode) (string, error) {
	nodeNames := make([]string, 0, len(nodes))
	nodeLines := make([]string, 0, len(nodes))
	for i, node := range nodes {
		if node.Protocol == "vless" {
			continue
		}
		name := ensureNodeName(node, i)
		nodeNames = append(nodeNames, name)
		nodeLines = append(nodeLines, surgeProxyLine(node, name))
	}

	var sb strings.Builder

	sb.WriteString("[General]\n")
	sb.WriteString("loglevel = notify\n")
	sb.WriteString("dns-server = 8.8.8.8, 114.114.114.114\n")
	sb.WriteString("\n")

	sb.WriteString("[Proxy]\n")
	for _, l := range nodeLines {
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	sb.WriteString("[Proxy Group]\n")
	selectProxies := append([]string{"♻️ 自动选择", "DIRECT"}, nodeNames...)
	sb.WriteString(fmt.Sprintf("🚀 节点选择 = select, %s\n", strings.Join(selectProxies, ", ")))
	sb.WriteString(fmt.Sprintf("♻️ 自动选择 = url-test, %s, url=http://cp.cloudflare.com/generate_204, interval=300\n",
		strings.Join(nodeNames, ", ")))
	sb.WriteString("\n")

	sb.WriteString("[Rule]\n")
	sb.WriteString("DOMAIN-SUFFIX,local,DIRECT\n")
	sb.WriteString("IP-CIDR,127.0.0.0/8,DIRECT\n")
	sb.WriteString("IP-CIDR,172.16.0.0/12,DIRECT\n")
	sb.WriteString("IP-CIDR,192.168.0.0/16,DIRECT\n")
	sb.WriteString("IP-CIDR,10.0.0.0/8,DIRECT\n")
	sb.WriteString("DOMAIN-SUFFIX,cn,DIRECT\n")
	sb.WriteString("GEOIP,CN,DIRECT\n")
	sb.WriteString("FINAL,🚀 节点选择\n")

	return sb.String(), nil
}

// surgeProxyLine 将 ProxyNode 转换为 Surge 代理行
func surgeProxyLine(node *ProxyNode, name string) string {
	opts := node.Options
	if opts == nil {
		opts = map[string]interface{}{}
	}

	strOpt := func(key string) string {
		v, _ := opts[key].(string)
		return v
	}
	intOpt := func(key string) int {
		switch v := opts[key].(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		default:
			return 0
		}
	}
	underlyingProxy := strings.TrimSpace(node.DialerProxy)
	if underlyingProxy == "" {
		if v, ok := opts["underlying-proxy"].(string); ok {
			underlyingProxy = strings.TrimSpace(v)
		}
	}
	if underlyingProxy == "" {
		if v, ok := opts["dialer-proxy"].(string); ok {
			underlyingProxy = strings.TrimSpace(v)
		}
	}
	appendUnderlyingProxy := func(line string) string {
		if underlyingProxy == "" {
			return line
		}
		return fmt.Sprintf("%s, underlying-proxy=%s", line, underlyingProxy)
	}
	tlsOptions, _ := extractTLSOptions(opts)
	transportOptions, _ := extractTransportOptions(opts)
	tlsEnabled := func() bool {
		if tlsOptions != nil {
			return tlsOptions.Enabled || tlsOptions.ServerName != "" || tlsOptions.Reality != nil
		}
		return false
	}
	tlsSNI := func() string {
		if tlsOptions != nil && tlsOptions.ServerName != "" {
			return tlsOptions.ServerName
		}
		return node.Server
	}
	tlsInsecure := func() bool {
		if tlsOptions != nil {
			return tlsOptions.Insecure
		}
		return false
	}
	transportType := func() string {
		if transportOptions != nil {
			return transportOptions.Type
		}
		return ""
	}
	transportPath := func() string {
		if transportOptions != nil && transportOptions.Path != "" {
			return transportOptions.Path
		}
		return ""
	}
	transportHost := func() string {
		if transportOptions != nil {
			if transportOptions.Host != "" {
				return transportOptions.Host
			}
			if host, ok := transportOptions.Headers["Host"]; ok {
				return host
			}
		}
		return ""
	}
	transportServiceName := func() string {
		if transportOptions != nil {
			return transportOptions.ServiceName
		}
		return ""
	}
	alpnValues := func() []string {
		if tlsOptions != nil && len(tlsOptions.ALPN) > 0 {
			return append([]string(nil), tlsOptions.ALPN...)
		}
		if values, ok := stringSliceOption(opts, "alpn"); ok {
			return values
		}
		return nil
	}
	switch node.Protocol {
	case "trojan":
		password := strOpt("password")
		sni := tlsSNI()
		skipVerify := tlsInsecure()
		parts := []string{
			fmt.Sprintf("%s = trojan", name),
			node.Server,
			fmt.Sprintf("%d", node.Port),
			fmt.Sprintf("password=%s", password),
			fmt.Sprintf("sni=%s", sni),
			fmt.Sprintf("skip-cert-verify=%v", skipVerify),
		}
		network := transportType()
		if network == "ws" {
			parts = append(parts, "ws=true")
			if wsPath := transportPath(); wsPath != "" {
				parts = append(parts, fmt.Sprintf("ws-path=%s", wsPath))
			}
			if wsHost := transportHost(); wsHost != "" {
				parts = append(parts, fmt.Sprintf("ws-headers=Host:%s", wsHost))
			}
		}
		return appendUnderlyingProxy(strings.Join(parts, ", "))

	case "vmess":
		uuid := strOpt("uuid")
		security := strOpt("security")
		if security == "" {
			security = "auto"
		}
		tls := tlsEnabled()
		sni := tlsSNI()
		parts := []string{
			fmt.Sprintf("%s = vmess", name),
			node.Server,
			fmt.Sprintf("%d", node.Port),
			fmt.Sprintf("username=%s", uuid),
			fmt.Sprintf("encrypt-method=%s", security),
			fmt.Sprintf("tls=%v", tls),
			fmt.Sprintf("sni=%s", sni),
		}
		if network := transportType(); network == "ws" {
			parts = append(parts, "ws=true")
			if path := transportPath(); path != "" {
				parts = append(parts, fmt.Sprintf("ws-path=%s", path))
			}
			if host := transportHost(); host != "" {
				parts = append(parts, fmt.Sprintf("ws-headers=Host:%s", host))
			}
		} else if network == "grpc" {
			parts = append(parts, "grpc=true")
			if serviceName := transportServiceName(); serviceName != "" {
				parts = append(parts, fmt.Sprintf("grpc-service-name=%s", serviceName))
			}
		}
		return appendUnderlyingProxy(strings.Join(parts, ", "))

	case "vless":
		uuid := strOpt("uuid")
		tls := tlsEnabled()
		sni := tlsSNI()
		parts := []string{
			fmt.Sprintf("%s = vless", name),
			node.Server,
			fmt.Sprintf("%d", node.Port),
			fmt.Sprintf("uuid=%s", uuid),
			fmt.Sprintf("tls=%v", tls),
			fmt.Sprintf("sni=%s", sni),
		}
		if network := transportType(); network == "ws" {
			parts = append(parts, "ws=true")
			if path := transportPath(); path != "" {
				parts = append(parts, fmt.Sprintf("ws-path=%s", path))
			}
			if host := transportHost(); host != "" {
				parts = append(parts, fmt.Sprintf("ws-headers=Host:%s", host))
			}
		} else if network == "grpc" {
			parts = append(parts, "grpc=true")
			if serviceName := transportServiceName(); serviceName != "" {
				parts = append(parts, fmt.Sprintf("grpc-service-name=%s", serviceName))
			}
		}
		return appendUnderlyingProxy(strings.Join(parts, ", "))

	case "ss":
		cipher := strOpt("cipher")
		password := strOpt("password")
		return appendUnderlyingProxy(fmt.Sprintf("%s = ss, %s, %d, encrypt-method=%s, password=%s",
			name, node.Server, node.Port, cipher, password))

	case "hysteria2", "hy2":
		password := strOpt("password")
		sni := tlsSNI()
		skipVerify := tlsInsecure()
		return appendUnderlyingProxy(fmt.Sprintf("%s = hysteria2, %s, %d, password=%s, sni=%s, skip-cert-verify=%v",
			name, node.Server, node.Port, password, sni, skipVerify))

	case "anytls":
		password := strOpt("password")
		sni := tlsSNI()
		skipVerify := tlsInsecure()
		parts := []string{
			fmt.Sprintf("%s = anytls", name),
			node.Server,
			fmt.Sprintf("%d", node.Port),
			fmt.Sprintf("password=%s", password),
		}
		if sni != "" {
			parts = append(parts, fmt.Sprintf("sni=%s", sni))
		}
		if tlsOptions != nil && tlsOptions.UTLS != nil && tlsOptions.UTLS.Fingerprint != "" {
			parts = append(parts, fmt.Sprintf("client-fingerprint=%s", tlsOptions.UTLS.Fingerprint))
		}
		if alpn := alpnValues(); len(alpn) > 0 {
			parts = append(parts, fmt.Sprintf("alpn=%s", strings.Join(alpn, "|")))
		}
		if interval := intOpt("idle-session-check-interval"); interval > 0 {
			parts = append(parts, fmt.Sprintf("idle-session-check-interval=%d", interval))
		}
		if timeout := intOpt("idle-session-timeout"); timeout > 0 {
			parts = append(parts, fmt.Sprintf("idle-session-timeout=%d", timeout))
		}
		if minIdle := intOpt("min-idle-session"); minIdle > 0 {
			parts = append(parts, fmt.Sprintf("min-idle-session=%d", minIdle))
		}
		parts = append(parts, fmt.Sprintf("skip-cert-verify=%v", skipVerify))
		return appendUnderlyingProxy(strings.Join(parts, ", "))

	case "tuic":
		uuid := strOpt("uuid")
		password := strOpt("password")
		sni := tlsSNI()
		parts := []string{
			fmt.Sprintf("%s = tuic-v5", name),
			node.Server,
			fmt.Sprintf("%d", node.Port),
			fmt.Sprintf("uuid=%s", uuid),
			fmt.Sprintf("password=%s", password),
			fmt.Sprintf("sni=%s", sni),
		}
		if alpn := alpnValues(); len(alpn) > 0 {
			parts = append(parts, fmt.Sprintf("alpn=%s", strings.Join(alpn, "|")))
		}
		return appendUnderlyingProxy(strings.Join(parts, ", "))

	case "wireguard":
		parts := []string{
			fmt.Sprintf("%s = wireguard", name),
			fmt.Sprintf("section-name = %s", name),
		}
		if testURL := strOpt("test-url"); testURL != "" {
			parts = append(parts, fmt.Sprintf("test-url=%s", testURL))
		}
		return appendUnderlyingProxy(strings.Join(parts, ", "))

	default:
		// 未知协议，生成注释行
		return fmt.Sprintf("# 不支持的协议: %s (%s)", node.Protocol, name)
	}
}

func surgeWireGuardSection(node *ProxyNode, name string) string {
	if node.Protocol != "wireguard" {
		return ""
	}

	cfg := extractWireGuardConfig(node)
	if cfg.PrivateKey == "" || len(cfg.Peers) == 0 {
		return ""
	}

	lines := []string{
		"",
		fmt.Sprintf("[WireGuard %s]", name),
		fmt.Sprintf("private-key=%s", cfg.PrivateKey),
	}
	if len(cfg.LocalAddresses) > 0 {
		lines = append(lines, fmt.Sprintf("self-ip=%s", surgeWireGuardSelfIP(cfg.LocalAddresses[0])))
	}
	for _, addr := range cfg.LocalAddresses {
		if strings.Contains(addr, ":") {
			lines = append(lines, fmt.Sprintf("self-ip-v6=%s", surgeWireGuardSelfIP(addr)))
			break
		}
	}
	if cfg.MTU > 0 {
		lines = append(lines, fmt.Sprintf("mtu=%d", cfg.MTU))
	}
	if len(cfg.DNS) > 0 {
		lines = append(lines, fmt.Sprintf("dns-server=%s", strings.Join(cfg.DNS, ", ")))
	}
	for _, peer := range cfg.Peers {
		parts := []string{
			fmt.Sprintf("endpoint=%s:%d", peer.Server, peer.Port),
			fmt.Sprintf("public-key=\"%s\"", peer.PublicKey),
		}
		if peer.PreSharedKey != "" {
			parts = append(parts, fmt.Sprintf("pre-shared-key=\"%s\"", peer.PreSharedKey))
		}
		if len(peer.AllowedIPs) > 0 {
			parts = append(parts, fmt.Sprintf("allowed-ips=\"%s\"", strings.Join(peer.AllowedIPs, ",")))
		}
		if len(peer.Reserved) > 0 {
			clientID := make([]string, 0, len(peer.Reserved))
			for _, value := range peer.Reserved {
				clientID = append(clientID, fmt.Sprintf("%d", value))
			}
			parts = append(parts, fmt.Sprintf("client-id=\"%s\"", strings.Join(clientID, "/")))
		}
		if peer.PersistentKeepalive > 0 {
			parts = append(parts, fmt.Sprintf("keepalive=%d", peer.PersistentKeepalive))
		}
		lines = append(lines, fmt.Sprintf("peer=(%s)", strings.Join(parts, ", ")))
	}
	return strings.Join(lines, "\n") + "\n"
}

func surgeWireGuardSelfIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Addr().String()
	}
	return value
}

// filterNodesByPattern 按正则模式过滤节点名列表，支持包含（policy-regex-filter）和排除（exclude-filter）
func filterNodesByPattern(nodeNames []string, filterPattern, excludePattern string) []string {
	result := nodeNames
	if filterPattern != "" {
		matcher, err := compileNodeNameMatcher(filterPattern)
		if err == nil {
			included := make([]string, 0, len(result))
			for _, n := range result {
				if matcher(n) {
					included = append(included, n)
				}
			}
			result = included
		}
	}
	if excludePattern != "" {
		matcher, err := compileNodeNameMatcher(excludePattern)
		if err == nil {
			excluded := make([]string, 0, len(result))
			for _, n := range result {
				if !matcher(n) {
					excluded = append(excluded, n)
				}
			}
			result = excluded
		}
	}
	return result
}

func compileNodeNameMatcher(pattern string) (func(string) bool, error) {
	if re, err := regexp.Compile(pattern); err == nil {
		return re.MatchString, nil
	}

	re2, err := regexp2.Compile(pattern, 0)
	if err != nil {
		return nil, err
	}
	return func(value string) bool {
		matched, err := re2.MatchString(value)
		return err == nil && matched
	}, nil
}

// cleanCommas 清理多余的逗号（如连续逗号或行尾逗号）
func cleanCommas(s string) string {
	// 替换连续逗号
	multiComma := regexp.MustCompile(`,\s*,`)
	for multiComma.MatchString(s) {
		s = multiComma.ReplaceAllString(s, ",")
	}
	// 去掉尾随逗号（在可能的空格后）
	s = regexp.MustCompile(`,\s*$`).ReplaceAllString(s, "")
	return s
}
