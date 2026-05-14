package app

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestVLESSRealityYAMLOutput(t *testing.T) {
	proxy := &Proxy{
		Name:        "东京",
		Type:        "vless",
		Server:      "154.31.116.16",
		Port:        45478,
		UUID:        "700229f2-3709-4fc5-8d8e-ae1af6ed8d58",
		Flow:        "xtls-rprx-vision",
		Network:     "tcp",
		TLS:         true,
		SNI:         "music.apple.com",
		Fingerprint: "random",
		Reality: &RealityCfg{
			PublicKey: "Fnu3wR5hEeonakgRDrgG9yRG9XyM9KScbZlmPzrUXwM",
			ShortID:   "0892831900b76d85",
		},
	}

	out, err := yaml.Marshal(proxy)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	yamlStr := string(out)
	if !strings.Contains(yamlStr, "reality-opts:") {
		t.Errorf("YAML 输出缺少 reality-opts 字段，实际输出:\n%s", yamlStr)
	}
	if strings.Contains(yamlStr, "\nreality:") {
		t.Errorf("YAML 输出不应包含 reality: 字段（应为 reality-opts:），实际输出:\n%s", yamlStr)
	}
}

func TestAddVLESSFieldsFromNestedTLS(t *testing.T) {
	proxy := &Proxy{Name: "东京", Type: "vless", Server: "154.31.116.16", Port: 45478, UDP: true}
	opts := map[string]interface{}{
		"uuid": "700229f2-3709-4fc5-8d8e-ae1af6ed8d58",
		"flow": "xtls-rprx-vision",
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": "music.apple.com",
			"utls": map[string]interface{}{
				"enabled":     true,
				"fingerprint": "random",
			},
			"reality": map[string]interface{}{
				"public_key": "Fnu3wR5hEeonakgRDrgG9yRG9XyM9KScbZlmPzrUXwM",
				"short_id":   "0892831900b76d85",
			},
		},
	}
	addVLESSFields(proxy, opts)

	if proxy.Fingerprint != "random" {
		t.Errorf("utls fingerprint 未映射，got %q", proxy.Fingerprint)
	}
	if proxy.Reality == nil {
		t.Fatal("nested reality 未映射，proxy.Reality 为 nil")
	}
	if proxy.Reality.PublicKey != "Fnu3wR5hEeonakgRDrgG9yRG9XyM9KScbZlmPzrUXwM" {
		t.Errorf("public-key 错误，got %q", proxy.Reality.PublicKey)
	}
	if proxy.Reality.ShortID != "0892831900b76d85" {
		t.Errorf("short-id 错误，got %q", proxy.Reality.ShortID)
	}
}

func TestAddTrojanFieldsWithWebSocketOptions(t *testing.T) {
	proxy := &Proxy{Name: "US WS", Type: "trojan", Server: "cdn.wwm.app", Port: 443, UDP: true}
	opts := map[string]interface{}{
		"password": "test-password",
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": "cdn.wwm.app",
		},
		"transport": map[string]interface{}{
			"type": "ws",
			"path": "/test-path",
		},
	}

	addTrojanFields(proxy, opts)

	if proxy.Network != "ws" {
		t.Fatalf("期望 trojan network=ws，实际为 %q", proxy.Network)
	}
	if proxy.WSOpts == nil {
		t.Fatal("期望 trojan 生成 ws-opts，实际为 nil")
	}
	if proxy.WSOpts.Path != "/test-path" {
		t.Fatalf("期望 ws path 为 /test-path，实际为 %q", proxy.WSOpts.Path)
	}
}

func TestAddTrojanFieldsWithNestedTransportHost(t *testing.T) {
	proxy := &Proxy{Name: "US WS", Type: "trojan", Server: "cdn.wwm.app", Port: 443, UDP: true}
	opts := map[string]interface{}{
		"password": "test-password",
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": "cdn.wwm.app",
			"alpn":        []string{"h2", "http/1.1"},
		},
		"transport": map[string]interface{}{
			"type": "ws",
			"path": "/test-path",
			"host": "edge.example.com",
			"headers": map[string]interface{}{
				"Host": "edge.example.com",
			},
		},
	}

	addTrojanFields(proxy, opts)

	if proxy.WSOpts == nil {
		t.Fatal("期望 trojan 生成 ws-opts，实际为 nil")
	}
	if proxy.WSOpts.Headers["Host"] != "edge.example.com" {
		t.Fatalf("期望 ws host 为 edge.example.com，实际为 %#v", proxy.WSOpts.Headers)
	}
	if len(proxy.Alpn) != 2 || proxy.Alpn[0] != "h2" {
		t.Fatalf("期望从嵌套 tls 读取 alpn，实际为 %#v", proxy.Alpn)
	}
}

func TestAddTrojanFieldsWithClashStyleWebSocketOptions(t *testing.T) {
	proxy := &Proxy{Name: "JP WS", Type: "trojan", Server: "89.121189.xyz", Port: 443, UDP: true}
	opts := map[string]interface{}{
		"password": "test-password",
		"tls":      true,
		"sni":      "89.121189.xyz",
		"alpn":     []interface{}{"h3"},
		"network":  "ws",
		"ws-opts": map[string]interface{}{
			"path": "/",
			"headers": map[string]interface{}{
				"Host":       "89.121189.xyz",
				"User-Agent": "Safari",
			},
		},
	}

	addTrojanFields(proxy, opts)

	if proxy.Network != "ws" {
		t.Fatalf("期望 trojan network=ws，实际为 %q", proxy.Network)
	}
	if proxy.WSOpts == nil {
		t.Fatal("期望 trojan 生成 ws-opts，实际为 nil")
	}
	if proxy.WSOpts.Path != "/" {
		t.Fatalf("期望 ws path 为 /，实际为 %q", proxy.WSOpts.Path)
	}
	if proxy.WSOpts.Headers["Host"] != "89.121189.xyz" {
		t.Fatalf("期望 ws host 为 89.121189.xyz，实际为 %#v", proxy.WSOpts.Headers)
	}
	if len(proxy.Alpn) != 1 || proxy.Alpn[0] != "h3" {
		t.Fatalf("期望读取平铺 alpn，实际为 %#v", proxy.Alpn)
	}
}

func TestAddVLESSFieldsWithNestedGRPCTransport(t *testing.T) {
	proxy := &Proxy{Name: "东京", Type: "vless", Server: "154.31.116.16", Port: 45478, UDP: true}
	opts := map[string]interface{}{
		"uuid": "700229f2-3709-4fc5-8d8e-ae1af6ed8d58",
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": "music.apple.com",
		},
		"transport": map[string]interface{}{
			"type":         "grpc",
			"service_name": "grpc-svc",
		},
	}
	addVLESSFields(proxy, opts)

	if proxy.Network != "grpc" {
		t.Fatalf("期望 network=grpc，实际为 %q", proxy.Network)
	}
	if proxy.GRPCOpts == nil || proxy.GRPCOpts.GrpcServiceName != "grpc-svc" {
		t.Fatalf("期望 grpc-opts.grpc-service-name=grpc-svc，实际为 %#v", proxy.GRPCOpts)
	}
}

func TestVLESSManualImportFullPath(t *testing.T) {
	nodeURL := "vless://700229f2-3709-4fc5-8d8e-ae1af6ed8d58@154.31.116.16:45478?type=tcp&security=reality&pbk=Fnu3wR5hEeonakgRDrgG9yRG9XyM9KScbZlmPzrUXwM&fp=random&sni=music.apple.com&sid=0892831900b76d85&spx=%2F&flow=xtls-rprx-vision#%E4%B8%9C%E4%BA%AC"

	node, err := parseVLESSNode(nodeURL)
	if err != nil {
		t.Fatalf("parseVLESSNode error: %v", err)
	}

	jsonBytes, _ := json.Marshal(node.Options)
	var dbConfig map[string]interface{}
	json.Unmarshal(jsonBytes, &dbConfig)

	proxyNodes := []*ProxyNode{{
		Protocol: node.Protocol,
		Name:     node.Name,
		Server:   node.Server,
		Port:     node.Port,
		Options:  dbConfig,
	}}

	proxies, _ := buildProxies(proxyNodes)
	p := proxies[0]

	if p.Reality == nil {
		t.Fatal("proxy.Reality 为 nil，DB 路径修复失败")
	}
	if p.Reality.PublicKey != "Fnu3wR5hEeonakgRDrgG9yRG9XyM9KScbZlmPzrUXwM" {
		t.Errorf("PublicKey 错误: %q", p.Reality.PublicKey)
	}
	if p.Reality.ShortID != "0892831900b76d85" {
		t.Errorf("ShortID 错误: %q", p.Reality.ShortID)
	}
}

func TestBuildProxiesSupportsAnyTLS(t *testing.T) {
	nodes := []*ProxyNode{
		{
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
				"idle-session-timeout": 30,
			},
		},
	}

	proxies, _ := buildProxies(nodes)
	if len(proxies) != 1 {
		t.Fatalf("buildProxies() 返回代理数 = %d, want 1", len(proxies))
	}

	proxy := proxies[0]
	if proxy.Type != "anytls" {
		t.Fatalf("proxy.Type = %s, want anytls", proxy.Type)
	}
	if proxy.Password != "secret" {
		t.Fatalf("proxy.Password = %s, want secret", proxy.Password)
	}
	if proxy.Fingerprint != "chrome" {
		t.Fatalf("proxy.Fingerprint = %s, want chrome", proxy.Fingerprint)
	}
	if !proxy.SkipCertVerify {
		t.Fatal("proxy.SkipCertVerify = false, want true")
	}
	if proxy.IdleSessionTimeout != 30 {
		t.Fatalf("proxy.IdleSessionTimeout = %d, want 30", proxy.IdleSessionTimeout)
	}
	if len(proxy.Alpn) != 2 || proxy.Alpn[0] != "h2" || proxy.Alpn[1] != "http/1.1" {
		t.Fatalf("proxy.Alpn = %#v, want [h2 http/1.1]", proxy.Alpn)
	}
}
