package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildSingBoxFromTemplateContent(t *testing.T) {
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
		{
			Protocol: "vless",
			Name:     "HK Node 1",
			Server:   "hk1.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"uuid": "11111111-1111-1111-1111-111111111111",
				"tls":  true,
				"sni":  "hk1.example.com",
			},
		},
	}

	templateWithDNS := `{
  "dns": {
    "servers": [
      {"tag": "dns_proxy", "domain_resolver": "dns_direct"},
      {"tag": "dns_direct"}
    ]
  },
  "outbounds": [
    {"type": "urltest", "tag": "Auto", "outbounds": ["__AUTO_NODES__"]},
    "__OUTBOUNDS__",
    {"type": "direct", "tag": "DIRECT"}
  ]
}`
	config, err := BuildSingBoxFromTemplateContent(nodes, templateWithDNS)
	if err != nil {
		t.Fatalf("生成 sing-box 配置失败: %v", err)
	}

	if strings.Contains(config, "__OUTBOUNDS__") || strings.Contains(config, "__AUTO_NODES__") || strings.Contains(config, "\"filter\":") {
		t.Fatalf("生成后的配置不应保留节点占位符，实际配置为:\n%s", config)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		t.Fatalf("生成的 sing-box 配置不是合法 JSON: %v", err)
	}

	dns, ok := cfg["dns"].(map[string]interface{})
	if !ok {
		t.Fatalf("生成的 sing-box 配置缺少 dns")
	}
	servers, ok := dns["servers"].([]interface{})
	if !ok {
		t.Fatalf("生成的 sing-box 配置缺少 dns.servers")
	}
	foundDNSProxy := false
	for _, item := range servers {
		server, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if server["tag"] == "dns_proxy" {
			foundDNSProxy = true
			if server["domain_resolver"] != "dns_direct" {
				t.Fatalf("期望 dns_proxy 配置 domain_resolver=dns_direct，实际为: %#v", server["domain_resolver"])
			}
		}
	}
	if !foundDNSProxy {
		t.Fatalf("生成的 sing-box 配置缺少 dns_proxy 服务器")
	}

	outbounds, ok := cfg["outbounds"].([]interface{})
	if !ok || len(outbounds) == 0 {
		t.Fatalf("生成的 sing-box 配置缺少 outbounds")
	}

	foundUS := false
	foundHK := false
	for _, item := range outbounds {
		outbound, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		tag, _ := outbound["tag"].(string)
		if strings.Contains(tag, "US Node 1") {
			foundUS = true
		}
		if strings.Contains(tag, "HK Node 1") {
			foundHK = true
		}
	}

	if !foundUS || !foundHK {
		t.Fatalf("生成的 sing-box 配置缺少节点出站，US=%v HK=%v", foundUS, foundHK)
	}
}

func TestBuildSingBoxSupportsFilterExpansion(t *testing.T) {
	templateContent := `{
  "outbounds": [
    {
      "type": "urltest",
      "tag": "🇸🇬 SG",
      "outbounds": ["__NODES__"],
      "filter": "新加坡|SG"
    },
    {
      "type": "urltest",
      "tag": "🛰 Auto",
      "outbounds": ["__NODES__"],
      "filter": "^((?!v2).)*$"
    },
    "__OUTBOUNDS__",
    {
      "type": "direct",
      "tag": "DIRECT"
    }
  ]
}`
	nodes := []*ProxyNode{
		{
			Protocol: "trojan",
			Name:     "SG Node 1",
			Server:   "sg1.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"password": "password123",
				"sni":      "sg1.example.com",
			},
		},
		{
			Protocol: "trojan",
			Name:     "v2 SG Node",
			Server:   "sg2.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"password": "password456",
				"sni":      "sg2.example.com",
			},
		},
	}

	config, err := BuildSingBoxFromTemplateContent(nodes, templateContent)
	if err != nil {
		t.Fatalf("按 filter 展开 sing-box 组失败: %v", err)
	}
	if strings.Contains(config, "\"filter\":") || strings.Contains(config, "__NODES__") {
		t.Fatalf("sing-box 输出中不应保留 filter 或 __NODES__ 占位符，实际配置为:\n%s", config)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		t.Fatalf("生成的 sing-box 配置不是合法 JSON: %v", err)
	}
	outbounds, ok := cfg["outbounds"].([]interface{})
	if !ok {
		t.Fatalf("生成的 sing-box 配置缺少 outbounds")
	}

	foundSG := false
	foundAuto := false
	for _, item := range outbounds {
		outbound, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if outbound["tag"] == "🇸🇬 SG" {
			foundSG = true
			members, ok := outbound["outbounds"].([]interface{})
			if !ok {
				t.Fatalf("SG 组缺少 outbounds，实际配置为:\n%s", config)
			}
			if len(members) != 2 {
				t.Fatalf("期望 SG 组按 filter 命中两个新加坡节点，实际配置为:\n%s", config)
			}
			continue
		}
		if outbound["tag"] == "🛰 Auto" {
			foundAuto = true
			members, ok := outbound["outbounds"].([]interface{})
			if !ok {
				t.Fatalf("Auto 组缺少 outbounds，实际配置为:\n%s", config)
			}
			if len(members) != 1 || members[0] != "🇸🇬 SG Node 1" {
				t.Fatalf("期望 Auto 组通过负向前瞻排除 v2 节点，实际配置为:\n%s", config)
			}
		}
	}
	if !foundSG || !foundAuto {
		t.Fatalf("未找到期望的 SG/Auto 组，实际配置为:\n%s", config)
	}
}

func TestBuildSingBoxTrojanWebSocket(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol: "trojan",
			Name:     "WS Node",
			Server:   "ws.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"password": "password123",
				"tls": map[string]interface{}{
					"enabled":     true,
					"server_name": "ws.example.com",
				},
				"transport": map[string]interface{}{
					"type": "ws",
					"path": "/ws",
					"host": "edge.example.com",
					"headers": map[string]interface{}{
						"Host": "edge.example.com",
					},
				},
			},
		},
	}

	minimalTemplate := `{"outbounds": ["__OUTBOUNDS__", {"type": "direct", "tag": "DIRECT"}]}`
	config, err := BuildSingBoxFromTemplateContent(nodes, minimalTemplate)
	if err != nil {
		t.Fatalf("生成 sing-box 配置失败: %v", err)
	}

	if !strings.Contains(config, "\"type\": \"ws\"") || !strings.Contains(config, "\"path\": \"/ws\"") {
		t.Fatalf("期望生成 WebSocket transport，实际配置为:\n%s", config)
	}
	if strings.Contains(config, "\"host\": \"edge.example.com\"") {
		t.Fatalf("期望 sing-box WebSocket transport 不输出 host 字段，实际配置为:\n%s", config)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		t.Fatalf("解析生成配置失败: %v", err)
	}

	outbounds, ok := parsed["outbounds"].([]interface{})
	if !ok {
		t.Fatalf("期望 outbounds 为数组，实际配置为:\n%s", config)
	}

	var transport map[string]interface{}
	for _, outboundRaw := range outbounds {
		outbound, ok := outboundRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if outbound["tag"] == "WS Node" {
			transport, _ = outbound["transport"].(map[string]interface{})
			break
		}
	}
	if transport == nil {
		t.Fatalf("未找到 WS Node 的 transport，实际配置为:\n%s", config)
	}
	if _, exists := transport["host"]; exists {
		t.Fatalf("期望 sing-box WebSocket transport 不包含 host 字段，实际配置为:\n%s", config)
	}
	headers, ok := transport["headers"].(map[string]interface{})
	if !ok || headers["Host"] != "edge.example.com" {
		t.Fatalf("期望 sing-box WebSocket transport 保留 Host header，实际配置为:\n%s", config)
	}
}

func TestBuildSingBoxVLESSWebSocketFiltersH2ALPN(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol: "vless",
			Name:     "TW Node",
			Server:   "origin.mcmtn.net",
			Port:     2083,
			Options: map[string]interface{}{
				"uuid": "1d09bd75-a4ba-4103-a20c-28019af3ab82",
				"tls": map[string]interface{}{
					"enabled":     true,
					"server_name": "twhn.1391399.xyz",
					"alpn":        []interface{}{"h2", "http/1.1"},
				},
				"transport": map[string]interface{}{
					"type": "ws",
					"path": "/pan_transportation",
					"headers": map[string]interface{}{
						"Host": "twhn.1391399.xyz",
					},
				},
			},
		},
	}

	minimalTemplate := `{"outbounds": ["__OUTBOUNDS__", {"type": "direct", "tag": "DIRECT"}]}`
	config, err := BuildSingBoxFromTemplateContent(nodes, minimalTemplate)
	if err != nil {
		t.Fatalf("生成 sing-box 配置失败: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		t.Fatalf("解析生成配置失败: %v", err)
	}

	outbounds := parsed["outbounds"].([]interface{})
	var tlsObj map[string]interface{}
	for _, raw := range outbounds {
		ob, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if ob["uuid"] == "1d09bd75-a4ba-4103-a20c-28019af3ab82" {
			tlsObj, _ = ob["tls"].(map[string]interface{})
			break
		}
	}
	if tlsObj == nil {
		t.Fatalf("未找到 VLESS 节点的 tls 配置，实际配置为:\n%s", config)
	}

	// WebSocket 节点不应包含 h2，否则 TLS 协商到 h2 后 WebSocket 握手失败
	alpnRaw, hasALPN := tlsObj["alpn"]
	if hasALPN {
		alpn, _ := alpnRaw.([]interface{})
		for _, a := range alpn {
			if a == "h2" {
				t.Fatalf("WebSocket 节点的 sing-box 配置不应包含 h2 ALPN，实际配置为:\n%s", config)
			}
		}
		// 保留 http/1.1
		found := false
		for _, a := range alpn {
			if a == "http/1.1" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("WebSocket 节点应保留 http/1.1 ALPN，实际配置为:\n%s", config)
		}
	}
}

func TestBuildSingBoxUsesNestedTLSAndTransport(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol: "vless",
			Name:     "Nested Node",
			Server:   "nested.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"uuid": "11111111-1111-1111-1111-111111111111",
				"tls": map[string]interface{}{
					"enabled":     true,
					"server_name": "tls.example.com",
					"utls": map[string]interface{}{
						"enabled":     true,
						"fingerprint": "chrome",
					},
				},
				"transport": map[string]interface{}{
					"type": "ws",
					"path": "/nested",
				},
			},
		},
	}

	minimalTemplate := `{"outbounds": ["__OUTBOUNDS__", {"type": "direct", "tag": "DIRECT"}]}`
	config, err := BuildSingBoxFromTemplateContent(nodes, minimalTemplate)
	if err != nil {
		t.Fatalf("生成 sing-box 配置失败: %v", err)
	}

	if !strings.Contains(config, "\"server_name\": \"tls.example.com\"") {
		t.Fatalf("期望读取嵌套 tls.server_name，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "\"fingerprint\": \"chrome\"") {
		t.Fatalf("期望读取嵌套 tls.utls.fingerprint，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "\"type\": \"ws\"") || !strings.Contains(config, "\"path\": \"/nested\"") {
		t.Fatalf("期望读取嵌套 transport，实际配置为:\n%s", config)
	}
}

func TestBuildSingBoxWireGuard(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol: "wireguard",
			Name:     "WG Node",
			Server:   "vpn.example.com",
			Port:     51820,
			Options: map[string]interface{}{
				"ip":          "10.0.0.2/32",
				"private-key": "private-key",
				"dns":         []interface{}{"1.1.1.1", "8.8.8.8"},
				"mtu":         1420,
				"public-key":  "peer-public-key",
				"allowed-ips": []interface{}{"0.0.0.0/0", "::/0"},
				"reserved":    []interface{}{1, 2, 3},
			},
		},
	}

	minimalTemplate := `{"endpoints": ["__ENDPOINTS__"], "outbounds": [{"type": "direct", "tag": "DIRECT"}]}`
	config, err := BuildSingBoxFromTemplateContent(nodes, minimalTemplate)
	if err != nil {
		t.Fatalf("生成 sing-box 配置失败: %v", err)
	}

	if !strings.Contains(config, "\"type\": \"wireguard\"") {
		t.Fatalf("期望生成 wireguard 端点，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "\"private_key\": \"private-key\"") {
		t.Fatalf("期望生成 private_key，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "\"public_key\": \"peer-public-key\"") {
		t.Fatalf("期望生成 peers.public_key，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "\"address\": [") {
		t.Fatalf("期望生成 endpoint address，实际配置为:\n%s", config)
	}
	if strings.Contains(config, "\"local_address\": [") || strings.Contains(config, "\"peer_public_key\":") {
		t.Fatalf("sing-box wireguard 不应再使用旧 outbound 字段，实际配置为:\n%s", config)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		t.Fatalf("生成的 sing-box 配置不是合法 JSON: %v", err)
	}
	endpoints, ok := cfg["endpoints"].([]interface{})
	if !ok || len(endpoints) == 0 {
		t.Fatalf("生成的 sing-box 配置缺少 endpoints")
	}
}

func TestBuildSingBoxSupportsLegacyQuotedEndpointsPlaceholder(t *testing.T) {
	templateContent := `{
  "endpoints": [
    "__ENDPOINTS__"
  ],
  "outbounds": [
    "__OUTBOUNDS__",
    {
      "type": "direct",
      "tag": "DIRECT"
    }
  ]
}`
	nodes := []*ProxyNode{
		{
			Protocol: "wireguard",
			Name:     "WG Node",
			Server:   "vpn.example.com",
			Port:     51820,
			Options: map[string]interface{}{
				"ip":          "10.0.0.2/32",
				"private-key": "private-key",
				"public-key":  "peer-public-key",
				"allowed-ips": []interface{}{"0.0.0.0/0"},
			},
		},
	}

	config, err := BuildSingBoxFromTemplateContent(nodes, templateContent)
	if err != nil {
		t.Fatalf("旧模板占位符兼容失败: %v", err)
	}
	if strings.Contains(config, "__ENDPOINTS__") {
		t.Fatalf("旧模板中的 __ENDPOINTS__ 不应保留，实际配置为:\n%s", config)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		t.Fatalf("生成的 sing-box 配置不是合法 JSON: %v", err)
	}
	endpoints, ok := cfg["endpoints"].([]interface{})
	if !ok || len(endpoints) != 1 {
		t.Fatalf("期望旧模板生成 1 个 endpoint，实际为: %#v", cfg["endpoints"])
	}
}

func TestBuildSingBoxWireGuardNormalizesAddressPrefix(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol: "wireguard",
			Name:     "WG Bare IP",
			Server:   "vpn.example.com",
			Port:     51820,
			Options: map[string]interface{}{
				"ip":          "10.0.10.3",
				"private-key": "private-key",
				"public-key":  "peer-public-key",
				"allowed-ips": []interface{}{"8.8.8.8"},
			},
		},
	}

	minimalTemplate := `{"endpoints": ["__ENDPOINTS__"], "outbounds": [{"type": "direct", "tag": "DIRECT"}]}`
	config, err := BuildSingBoxFromTemplateContent(nodes, minimalTemplate)
	if err != nil {
		t.Fatalf("生成 sing-box 配置失败: %v", err)
	}
	if !strings.Contains(config, "\"10.0.10.3/32\"") {
		t.Fatalf("期望自动补全 wireguard address 前缀，实际配置为:\n%s", config)
	}
	if !strings.Contains(config, "\"8.8.8.8/32\"") {
		t.Fatalf("期望自动补全 wireguard allowed_ips 前缀，实际配置为:\n%s", config)
	}
}

func TestBuildSingBoxSupportsBooleanTLSOption(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol: "vless",
			Name:     "Bool TLS Node",
			Server:   "tls.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"uuid": "11111111-1111-1111-1111-111111111111",
				"tls":  true,
				"sni":  "tls.example.com",
			},
		},
	}

	minimalTemplate := `{"outbounds": ["__OUTBOUNDS__", {"type": "direct", "tag": "DIRECT"}]}`
	config, err := BuildSingBoxFromTemplateContent(nodes, minimalTemplate)
	if err != nil {
		t.Fatalf("生成 sing-box 配置失败: %v", err)
	}
	if !strings.Contains(config, "\"enabled\": true") || !strings.Contains(config, "\"server_name\": \"tls.example.com\"") {
		t.Fatalf("期望从布尔 tls 生成 sing-box tls 对象，实际配置为:\n%s", config)
	}
}

func TestBuildSingBoxSupportsFlatSecurityTLSOption(t *testing.T) {
	nodes := []*ProxyNode{
		{
			Protocol: "vless",
			Name:     "Security TLS Node",
			Server:   "security.example.com",
			Port:     443,
			Options: map[string]interface{}{
				"uuid":     "11111111-1111-1111-1111-111111111111",
				"security": "tls",
				"sni":      "security.example.com",
			},
		},
	}

	minimalTemplate := `{"outbounds": ["__OUTBOUNDS__", {"type": "direct", "tag": "DIRECT"}]}`
	config, err := BuildSingBoxFromTemplateContent(nodes, minimalTemplate)
	if err != nil {
		t.Fatalf("生成 sing-box 配置失败: %v", err)
	}
	if !strings.Contains(config, "\"enabled\": true") || !strings.Contains(config, "\"server_name\": \"security.example.com\"") {
		t.Fatalf("期望从 security=tls 生成 sing-box tls 对象，实际配置为:\n%s", config)
	}
}
