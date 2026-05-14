package app

import (
	"strings"
	"testing"
)

func TestBuildSurgeFromTemplateContentAddsUnderlyingProxy(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol:    "trojan",
			Name:        "US Node 1",
			Server:      "us1.example.com",
			Port:        443,
			DialerProxy: "SG Relay",
			Options: map[string]interface{}{
				"password": "password123",
				"sni":      "us1.example.com",
			},
		},
	}

	templateContent := `[General]
loglevel = notify

[Proxy]
__NODES__

[Proxy Group]
Proxy = select, __NODES__
`

	config, err := BuildSurgeFromTemplateContent(nodes, templateContent)
	if err != nil {
		t.Fatalf("生成 Surge 配置失败: %v", err)
	}

	if !strings.Contains(config, "underlying-proxy=SG Relay") {
		t.Fatalf("期望生成 underlying-proxy，实际配置为:\n%s", config)
	}
}

func TestBuildSurgeFromTemplateContentAppliesGroupDialerProxyRule(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol: "trojan",
			Name:     "SG Relay",
			Server:   "sg.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"password": "sg-password",
				"sni":      "sg.example.com",
			},
		},
		{
			Protocol: "trojan",
			Name:     "US Node 1",
			Server:   "us1.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"password": "us-password",
				"sni":      "us1.example.com",
			},
		},
		{
			Protocol: "trojan",
			Name:     "HK Node 1",
			Server:   "hk1.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"password": "hk-password",
				"sni":      "hk1.example.com",
			},
		},
	}

	templateContent := `[General]
loglevel = notify

[Proxy]
__NODES__

[Proxy Group]
US = select, __NODES__, policy-regex-filter=US|美国, dialer-proxy=SG|新加坡
`

	config, err := BuildSurgeFromTemplateContent(nodes, templateContent)
	if err != nil {
		t.Fatalf("生成 Surge 配置失败: %v", err)
	}

	if !strings.Contains(config, "🇺🇸 US Node 1 = trojan, us1.example.com, 443, password=us-password, sni=us1.example.com, skip-cert-verify=false, underlying-proxy=🇸🇬 SG Relay") {
		t.Fatalf("期望 US 节点带 underlying-proxy，实际配置为:\n%s", config)
	}
	if strings.Contains(config, "🇭🇰 HK Node 1 = trojan, hk1.example.com, 443, password=hk-password, sni=hk1.example.com, skip-cert-verify=false, underlying-proxy=🇸🇬 SG Relay") {
		t.Fatalf("HK 节点不应匹配 US 规则，实际配置为:\n%s", config)
	}
	if strings.Contains(config, "dialer-proxy=") {
		t.Fatalf("生成后的 Surge 模板不应保留 dialer-proxy 扩展字段，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "US = select, 🇺🇸 US Node 1") {
		t.Fatalf("期望 Proxy Group 展开过滤后的节点，实际配置为:\n%s", config)
	}
}

func TestBuildSurgeFromTemplateContentAppliesDialerProxyToExplicitNodeMembers(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol: "trojan",
			Name:     "SG Relay",
			Server:   "sg.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"password": "sg-password",
				"sni":      "sg.example.com",
			},
		},
		{
			Protocol: "trojan",
			Name:     "us.lax.dmit",
			Server:   "us1.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"password": "us-password",
				"sni":      "us1.example.com",
			},
		},
		{
			Protocol: "trojan",
			Name:     "us.hnl.qqpw",
			Server:   "us2.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"password": "us2-password",
				"sni":      "us2.example.com",
			},
		},
	}

	templateContent := `[General]
loglevel = notify

[Proxy]
__NODES__

[Proxy Group]
🤖 AI = select, us.lax.dmit, us.hnl.qqpw, url=http://www.gstatic.com/generate_204, interval=1200, dialer-proxy=SG|新加坡
`

	config, err := BuildSurgeFromTemplateContent(nodes, templateContent)
	if err != nil {
		t.Fatalf("生成 Surge 配置失败: %v", err)
	}

	if !strings.Contains(config, "🇺🇸 us.lax.dmit = trojan, us1.example.com, 443, password=us-password, sni=us1.example.com, skip-cert-verify=false, underlying-proxy=🇸🇬 SG Relay") {
		t.Fatalf("期望显式节点 us.lax.dmit 带 underlying-proxy，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "🇺🇸 us.hnl.qqpw = trojan, us2.example.com, 443, password=us2-password, sni=us2.example.com, skip-cert-verify=false, underlying-proxy=🇸🇬 SG Relay") {
		t.Fatalf("期望显式节点 us.hnl.qqpw 带 underlying-proxy，实际配置为:\n%s", config)
	}
	if strings.Contains(config, "dialer-proxy=") {
		t.Fatalf("生成后的 Surge 模板不应保留 dialer-proxy 扩展字段，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "🤖 AI = select, us.lax.dmit, us.hnl.qqpw, url=http://www.gstatic.com/generate_204, interval=1200") {
		t.Fatalf("期望 Proxy Group 保留显式节点并删除扩展字段，实际配置为:\n%s", config)
	}
}

func TestBuildSurgeFromTemplateContentExcludeFilter(t *testing.T) {
	nodes := []*ProxyNode{
		{Protocol: "trojan", Name: "SG IPLC 1", Server: "sg-iplc.example.com", Port: 443, Options: map[string]interface{}{"password": "p1", "sni": "sg.example.com"}},
		{Protocol: "trojan", Name: "SG BGP 1", Server: "sg-bgp.example.com", Port: 443, Options: map[string]interface{}{"password": "p2", "sni": "sg.example.com"}},
		{Protocol: "trojan", Name: "SG 普通 1", Server: "sg.example.com", Port: 443, Options: map[string]interface{}{"password": "p3", "sni": "sg.example.com"}},
	}

	templateContent := `[Proxy]
__NODES__

[Proxy Group]
🇸🇬 SG = url-test, __NODES__, url=http://www.gstatic.com/generate_204, interval=1200, policy-regex-filter=SG|新加坡, exclude-filter=IPLC|BGP
`

	config, err := BuildSurgeFromTemplateContent(nodes, templateContent)
	if err != nil {
		t.Fatalf("生成 Surge 配置失败: %v", err)
	}

	if strings.Contains(config, "SG IPLC 1") && strings.Contains(config, "🇸🇬 SG = url-test") {
		// 检查 Proxy Group 行不含 IPLC/BGP 节点
		for _, line := range strings.Split(config, "\n") {
			if strings.HasPrefix(line, "🇸🇬 SG = url-test") {
				if strings.Contains(line, "IPLC") || strings.Contains(line, "BGP") {
					t.Fatalf("exclude-filter 应排除 IPLC/BGP 节点，实际行:\n%s", line)
				}
				if !strings.Contains(line, "SG 普通 1") {
					t.Fatalf("exclude-filter 不应排除普通 SG 节点，实际行:\n%s", line)
				}
			}
		}
	}

	if strings.Contains(config, "exclude-filter=") {
		t.Fatalf("生成配置中不应保留 exclude-filter 字段:\n%s", config)
	}
}

func TestBuildSurgeFromTemplateContentWireGuard(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol: "wireguard",
			Name:     "WG Private",
			Server:   "vpn.example.com",
			Port:     51820,
			Options: map[string]interface{}{
				"ip":          "10.255.255.2/32",
				"ipv6":        "fd00::2/128",
				"private-key": "private-key",
				"dns":         []interface{}{"192.168.100.1"},
				"test-url":    "http://192.168.100.1",
				"mtu":         1420,
				"public-key":  "peer-public-key",
				"allowed-ips": []interface{}{"192.168.100.0/24", "10.255.0.0/24"},
				"reserved":    []interface{}{1, 2, 3},
			},
		},
	}

	templateContent := `[General]
loglevel = notify

[Proxy]
__NODES__

[Proxy Group]
Proxy = select, __NODES__
`

	config, err := BuildSurgeFromTemplateContent(nodes, templateContent)
	if err != nil {
		t.Fatalf("生成 Surge 配置失败: %v", err)
	}

	if !strings.Contains(config, "WG Private = wireguard, section-name = WG Private, test-url=http://192.168.100.1") {
		t.Fatalf("期望生成 wireguard 代理行，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "[WireGuard WG Private]") {
		t.Fatalf("期望生成 WireGuard section，实际配置为:\n%s", config)
	}
	if strings.Index(config, "WG Private = wireguard, section-name = WG Private, test-url=http://192.168.100.1") >
		strings.Index(config, "[WireGuard WG Private]") {
		t.Fatalf("期望 wireguard 代理行出现在 WireGuard section 之前，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "dns-server=192.168.100.1") {
		t.Fatalf("期望 dns-server 不带引号，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "self-ip=10.255.255.2") || strings.Contains(config, "self-ip=10.255.255.2/32") {
		t.Fatalf("期望 self-ip 为纯 IPv4 地址，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "self-ip-v6=fd00::2") || strings.Contains(config, "self-ip-v6=fd00::2/128") {
		t.Fatalf("期望 self-ip-v6 为纯 IPv6 地址，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "peer=(endpoint=vpn.example.com:51820, public-key=\"peer-public-key\", allowed-ips=\"192.168.100.0/24,10.255.0.0/24\", client-id=\"1/2/3\")\n\n[Proxy Group]") {
		t.Fatalf("期望 WireGuard section 与下一个 section 之间保留空行，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "peer=(endpoint=vpn.example.com:51820, public-key=\"peer-public-key\", allowed-ips=\"192.168.100.0/24,10.255.0.0/24\", client-id=\"1/2/3\")") {
		t.Fatalf("期望生成 peer 配置，实际配置为:\n%s", config)
	}
}

func TestBuildSurgeFromTemplateContentPolicyRegexFilterWithSpaces(t *testing.T) {
	nodes := []*ProxyNode{
		{Name: "HK A", Server: "hk-a.example.com", Port: 443, Protocol: "trojan", Options: map[string]interface{}{"password": "a", "sni": "hk-a.example.com"}},
		{Name: "SG A", Server: "sg-a.example.com", Port: 443, Protocol: "trojan", Options: map[string]interface{}{"password": "b", "sni": "sg-a.example.com"}},
	}

	templateContent := `[General]
loglevel = notify

[Proxy]
__NODES__

[Proxy Group]
HK = url-test, __NODES__, url = http://www.gstatic.com/generate_204, interval = 1200, policy-regex-filter = HK
`

	config, err := BuildSurgeFromTemplateContent(nodes, templateContent)
	if err != nil {
		t.Fatalf("BuildSurgeFromTemplateContent() error = %v", err)
	}
	if strings.Contains(config, "policy-regex-") {
		t.Fatalf("生成配置中不应残留截断字段:\n%s", config)
	}
	if strings.Contains(config, "policy-regex-filter") {
		t.Fatalf("生成配置中不应保留 policy-regex-filter 字段:\n%s", config)
	}
	if !strings.Contains(config, "HK = url-test, ") {
		t.Fatalf("生成配置未正确保留其余参数并应用过滤:\n%s", config)
	}
	if !strings.Contains(config, "url = http://www.gstatic.com/generate_204, interval = 1200") {
		t.Fatalf("生成配置未正确保留其余参数:\n%s", config)
	}
	if !strings.Contains(config, "🇭🇰 HK A") || strings.Contains(config, "HK = url-test, 🇸🇬 SG A") {
		t.Fatalf("生成配置未正确应用 policy-regex-filter:\n%s", config)
	}
}

func TestBuildSurgeFromTemplateContentPolicyRegexFilterSupportsLookahead(t *testing.T) {
	nodes := []*ProxyNode{
		{Name: "SG v1", Server: "sg-v1.example.com", Port: 443, Protocol: "trojan", Options: map[string]interface{}{"password": "a", "sni": "sg-v1.example.com"}},
		{Name: "SG v2", Server: "sg-v2.example.com", Port: 443, Protocol: "trojan", Options: map[string]interface{}{"password": "b", "sni": "sg-v2.example.com"}},
	}

	templateContent := `[General]
loglevel = notify

[Proxy]
__NODES__

[Proxy Group]
SG = select, __NODES__, policy-regex-filter=^((?!v2).)*$
`

	config, err := BuildSurgeFromTemplateContent(nodes, templateContent)
	if err != nil {
		t.Fatalf("BuildSurgeFromTemplateContent() error = %v", err)
	}
	if !strings.Contains(config, "SG = select, 🇸🇬 SG v1") {
		t.Fatalf("前瞻表达式未正确保留非 v2 节点:\n%s", config)
	}
	if strings.Contains(config, "SG = select, 🇸🇬 SG v2") || strings.Contains(config, "SG = select, 🇸🇬 SG v1, 🇸🇬 SG v2") {
		t.Fatalf("前瞻表达式未正确排除 v2 节点:\n%s", config)
	}
}

func TestBuildSurgePrunesRulesForMissingPolicy(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol: "trojan",
			Name:     "US Node 1",
			Server:   "us1.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"password": "password123",
				"sni":      "us1.example.com",
			},
		},
	}

	templateContent := `[General]
loglevel = notify

[Proxy]
__NODES__

[Proxy Group]
🌐 Proxy = select, __NODES__

[Rule]
RULE-SET,/rulesets/wireguard-home?target=surge,🏠 wireguard-home
RULE-SET,/rulesets/need_direct?target=surge,DIRECT
FINAL,🌐 Proxy
`

	config, err := BuildSurgeFromTemplateContent(nodes, templateContent)
	if err != nil {
		t.Fatalf("生成 Surge 配置失败: %v", err)
	}

	if strings.Contains(config, "RULE-SET,/rulesets/wireguard-home?target=surge,🏠 wireguard-home") {
		t.Fatalf("未生成对应 policy 时，不应保留规则，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "RULE-SET,/rulesets/need_direct?target=surge,DIRECT") {
		t.Fatalf("内置 policy 规则不应被裁掉，实际配置为:\n%s", config)
	}
}

func TestBuildSurgeKeepsRulesForExistingPolicy(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol: "wireguard",
			Name:     "🏠 wireguard-home",
			Server:   "vpn.example.com",
			Port:     51820,
			Options: map[string]interface{}{
				"ip":          "10.0.10.3/32",
				"private-key": "private-key",
				"public-key":  "peer-public-key",
				"allowed-ips": []interface{}{"0.0.0.0/0"},
			},
		},
	}

	templateContent := `[General]
loglevel = notify

[Proxy]
__NODES__

[Proxy Group]
🌐 Proxy = select, __NODES__

[Rule]
RULE-SET,/rulesets/wireguard-home?target=surge,🏠 wireguard-home
FINAL,🌐 Proxy
`

	config, err := BuildSurgeFromTemplateContent(nodes, templateContent)
	if err != nil {
		t.Fatalf("生成 Surge 配置失败: %v", err)
	}

	if !strings.Contains(config, "RULE-SET,/rulesets/wireguard-home?target=surge,🏠 wireguard-home") {
		t.Fatalf("已存在对应 policy 时，应保留规则，实际配置为:\n%s", config)
	}
}

func TestSurgeProxyLineSupportsAnyTLS(t *testing.T) {
	node := &ProxyNode{
		Protocol: "anytls",
		Name:     "SG AnyTLS",
		Server:   "sg.example.com",
		Port:     443,
		Options: map[string]interface{}{
			"password": "secret",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "sg.example.com",
				"insecure":    true,
				"alpn":        []string{"h2", "http/1.1"},
				"utls": map[string]interface{}{
					"enabled":     true,
					"fingerprint": "chrome",
				},
			},
			"idle-session-check-interval": 15,
			"idle-session-timeout":        30,
			"min-idle-session":            2,
		},
	}

	line := surgeProxyLine(node, "SG AnyTLS")
	if !strings.Contains(line, "= anytls, sg.example.com, 443, password=secret") {
		t.Fatalf("缺少 anytls 基础字段，实际输出:\n%s", line)
	}
	if !strings.Contains(line, "client-fingerprint=chrome") {
		t.Fatalf("缺少 client-fingerprint，实际输出:\n%s", line)
	}
	if !strings.Contains(line, "alpn=h2|http/1.1") {
		t.Fatalf("缺少 alpn，实际输出:\n%s", line)
	}
	if !strings.Contains(line, "idle-session-timeout=30") {
		t.Fatalf("缺少 idle-session-timeout，实际输出:\n%s", line)
	}
	if !strings.Contains(line, "skip-cert-verify=true") {
		t.Fatalf("缺少 skip-cert-verify，实际输出:\n%s", line)
	}
}

func TestSurgeProxyLineSupportsTUICUUID(t *testing.T) {
	node := &ProxyNode{
		Protocol: "tuic",
		Name:     "JP TUIC",
		Server:   "jp.example.com",
		Port:     443,
		Options: map[string]interface{}{
			"uuid":     "949862c6-2854-475d-bf74-c73eda541d22",
			"password": "949862c6-2854-475d-bf74-c73eda541d22",
			"sni":      "jp.example.com",
		},
	}

	line := surgeProxyLine(node, "JP TUIC")
	if !strings.Contains(line, " = tuic-v5, jp.example.com, 443, uuid=949862c6-2854-475d-bf74-c73eda541d22") {
		t.Fatalf("缺少 tuic uuid 字段，实际输出:\n%s", line)
	}
	if strings.Contains(line, "token=") {
		t.Fatalf("tuic-v5 不应输出 token 字段，实际输出:\n%s", line)
	}
}

func TestSurgeProxyLinePrefersExplicitUnderlyingProxyOption(t *testing.T) {
	node := &ProxyNode{
		Protocol:    "ss",
		Name:        "HK Node 1",
		Server:      "hk1.example.com",
		Port:        443,
		DialerProxy: "SG Relay",
		Options: map[string]interface{}{
			"cipher":           "aes-128-gcm",
			"password":         "password123",
			"underlying-proxy": "JP Relay",
		},
	}

	line := surgeProxyLine(node, "HK Node 1")
	if !strings.Contains(line, "underlying-proxy=SG Relay") {
		t.Fatalf("期望优先使用 ProxyNode.DialerProxy，实际行为为: %s", line)
	}
}

func TestSurgeProxyLineSupportsTrojanWebSocket(t *testing.T) {
	node := &ProxyNode{
		Protocol: "trojan",
		Name:     "us.lax.dmit",
		Server:   "cdn.wwm.app",
		Port:     443,
		Options: map[string]interface{}{
			"password": "d85ec159-128e-4903-bf8e-b4313c3631c0",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "cdn.wwm.app",
			},
			"transport": map[string]interface{}{
				"type": "ws",
				"path": "/d85ec159-128e-4903-bf8e-b4313c3631c0",
			},
		},
	}

	line := surgeProxyLine(node, "🇺🇸 us.lax.dmit")
	if !strings.Contains(line, "ws=true") {
		t.Fatalf("期望 trojan ws 节点输出 ws=true，实际为: %s", line)
	}
	if !strings.Contains(line, "ws-path=/d85ec159-128e-4903-bf8e-b4313c3631c0") {
		t.Fatalf("期望 trojan ws 节点输出 ws-path，实际为: %s", line)
	}
}

func TestSurgeProxyLineSupportsNestedTransportAndTLS(t *testing.T) {
	node := &ProxyNode{
		Protocol: "trojan",
		Name:     "us.lax.dmit",
		Server:   "cdn.wwm.app",
		Port:     443,
		Options: map[string]interface{}{
			"password": "d85ec159-128e-4903-bf8e-b4313c3631c0",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "cdn.wwm.app",
			},
			"transport": map[string]interface{}{
				"type": "ws",
				"path": "/d85ec159-128e-4903-bf8e-b4313c3631c0",
				"host": "edge.example.com",
				"headers": map[string]interface{}{
					"Host": "edge.example.com",
				},
			},
		},
	}

	line := surgeProxyLine(node, "🇺🇸 us.lax.dmit")
	if !strings.Contains(line, "ws=true") || !strings.Contains(line, "ws-path=/d85ec159-128e-4903-bf8e-b4313c3631c0") {
		t.Fatalf("期望输出 ws 参数，实际为: %s", line)
	}
	if !strings.Contains(line, "ws-headers=Host:edge.example.com") {
		t.Fatalf("期望输出 ws host，实际为: %s", line)
	}
}

func TestSurgeProxyLineSupportsClashStyleTrojanWebSocket(t *testing.T) {
	node := &ProxyNode{
		Protocol: "trojan",
		Name:     "NIO-沪日-JP-trojan",
		Server:   "89.121189.xyz",
		Port:     443,
		Options: map[string]interface{}{
			"password": "HYqoQNlRC0ewOZ6SSb0jIw",
			"tls":      true,
			"sni":      "89.121189.xyz",
			"alpn":     []interface{}{"h3"},
			"network":  "ws",
			"ws-opts": map[string]interface{}{
				"path": "/",
				"headers": map[string]interface{}{
					"Host":       "89.121189.xyz",
					"User-Agent": "Mozilla/5.0",
				},
			},
		},
	}

	line := surgeProxyLine(node, "🇯🇵 NIO-沪日-JP-trojan")
	if !strings.Contains(line, "ws=true") {
		t.Fatalf("期望输出 ws=true，实际为: %s", line)
	}
	if !strings.Contains(line, "ws-path=/") {
		t.Fatalf("期望输出 ws-path=/，实际为: %s", line)
	}
	if !strings.Contains(line, "ws-headers=Host:89.121189.xyz") {
		t.Fatalf("期望输出 ws Host，实际为: %s", line)
	}
}

func TestSurgeProxyLineSupportsGRPCAndALPN(t *testing.T) {
	node := &ProxyNode{
		Protocol: "tuic",
		Name:     "JP TUIC",
		Server:   "jp.example.com",
		Port:     443,
		Options: map[string]interface{}{
			"uuid":     "949862c6-2854-475d-bf74-c73eda541d22",
			"password": "949862c6-2854-475d-bf74-c73eda541d22",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "jp.example.com",
				"alpn":        []string{"h3"},
			},
		},
	}

	line := surgeProxyLine(node, "JP TUIC")
	if !strings.Contains(line, "alpn=h3") {
		t.Fatalf("期望输出嵌套 tls 中的 alpn，实际为: %s", line)
	}

	vlessLine := surgeProxyLine(&ProxyNode{
		Protocol: "vless",
		Name:     "GRPC Node",
		Server:   "grpc.example.com",
		Port:     443,
		Options: map[string]interface{}{
			"uuid": "11111111-1111-1111-1111-111111111111",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "grpc.example.com",
			},
			"transport": map[string]interface{}{
				"type":         "grpc",
				"service_name": "grpc-svc",
			},
		},
	}, "GRPC Node")
	if !strings.Contains(vlessLine, "grpc=true") || !strings.Contains(vlessLine, "grpc-service-name=grpc-svc") {
		t.Fatalf("期望输出 grpc 参数，实际为: %s", vlessLine)
	}
}
