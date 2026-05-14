package app

import (
	"testing"
)

func TestDecodeURLSafeBase64String(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "标准 Base64",
			input: "YWJjZA==",
			want:  "abcd",
		},
		{
			name:  "缺少 padding",
			input: "YWJjZA",
			want:  "abcd",
		},
		{
			name:  "URL 安全 Base64",
			input: "Pz8_",
			want:  "???",
		},
		{
			name:    "非法 Base64",
			input:   "not-base64!",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeURLSafeBase64String(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("decodeURLSafeBase64String() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("decodeURLSafeBase64String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodeSSBase64IgnoresFragment(t *testing.T) {
	got, err := decodeSSBase64("YWVzLTI1Ni1nY206cGFzc3dvcmQ=#TestNode")
	if err != nil {
		t.Fatalf("decodeSSBase64() error = %v", err)
	}
	if got != "aes-256-gcm:password" {
		t.Fatalf("decodeSSBase64() = %q, want %q", got, "aes-256-gcm:password")
	}
}

func TestParseTrojanNode(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "标准 Trojan 链接",
			url:     "trojan://password@example.com:443?security=tls&sni=example.com#TestNode",
			wantErr: false,
		},
		{
			name:    "带 skipCertVerify 的 Trojan 链接",
			url:     "trojan://password@example.com?allowInsecure=1#InsecureNode",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTrojanNode(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTrojanNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Protocol != "trojan" {
					t.Errorf("parseTrojanNode() Protocol = %v, want trojan", got.Protocol)
				}
				tlsObj, ok := got.Options["tls"].(map[string]interface{})
				if !ok {
					t.Fatalf("parseTrojanNode() 缺少 tls 对象，got=%#v", got.Options["tls"])
				}
				if enabled, _ := tlsObj["enabled"].(bool); !enabled {
					t.Fatalf("parseTrojanNode() tls.enabled = %v, want true", tlsObj["enabled"])
				}
			}
		})
	}
}

func TestParseVLESSNode(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "标准 VLESS 链接",
			url:     "vless://uuid@example.com:443?encryption=none&type=tcp&security=tls&sni=example.com#VLESSNode",
			wantErr: false,
		},
		{
			name:    "VLESS with REALITY",
			url:     "vless://700229f2-3709-4fc5-8d8e-ae1af6ed8d58@154.31.116.16:45478?type=tcp&security=reality&pbk=Fnu3wR5hEeonakgRDrgG9yRG9XyM9KScbZlmPzrUXwM&fp=random&sni=music.apple.com&sid=0892831900b76d85&flow=xtls-rprx-vision#东京",
			wantErr: false,
		},
		{
			name:    "VLESS with WebSocket",
			url:     "vless://uuid@example.com:443?type=ws&path=/ws&host=cdn.example.com#VLESSWS",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVLESSNode(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVLESSNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Protocol != "vless" {
					t.Errorf("parseVLESSNode() Protocol = %v, want vless", got.Protocol)
				}
				if tlsObj, ok := got.Options["tls"].(map[string]interface{}); ok {
					if tt.name == "VLESS with REALITY" {
						if _, ok := tlsObj["reality"].(map[string]interface{}); !ok {
							t.Fatalf("parseVLESSNode() 缺少 reality tls 配置，got=%#v", tlsObj)
						}
					}
				} else if tt.name != "VLESS with WebSocket" {
					t.Fatalf("parseVLESSNode() 缺少 tls 对象，got=%#v", got.Options["tls"])
				}
				if tt.name == "VLESS with WebSocket" {
					transport, ok := got.Options["transport"].(map[string]interface{})
					if !ok {
						t.Fatalf("parseVLESSNode() 缺少 transport 对象，got=%#v", got.Options["transport"])
					}
					if transport["type"] != "ws" || transport["path"] != "/ws" {
						t.Fatalf("parseVLESSNode() transport=%#v, want ws /ws", transport)
					}
					if transport["host"] != "cdn.example.com" {
						t.Fatalf("parseVLESSNode() transport.host=%v, want cdn.example.com", transport["host"])
					}
				}
			}
		})
	}
}

func TestParseVLESSNodeWithGRPCTransport(t *testing.T) {
	got, err := parseVLESSNode("vless://uuid@example.com:443?type=grpc&serviceName=grpc-svc&security=tls&sni=example.com&alpn=h2,http/1.1#VLESSGRPC")
	if err != nil {
		t.Fatalf("parseVLESSNode() error = %v", err)
	}
	transport, ok := got.Options["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("缺少 transport 对象，got=%#v", got.Options["transport"])
	}
	if transport["type"] != "grpc" || transport["service_name"] != "grpc-svc" {
		t.Fatalf("transport = %#v, want grpc/grpc-svc", transport)
	}
	tlsObj, ok := got.Options["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("缺少 tls 对象，got=%#v", got.Options["tls"])
	}
	alpn, ok := tlsObj["alpn"].([]string)
	if !ok || len(alpn) != 2 {
		t.Fatalf("tls.alpn = %#v, want [h2 http/1.1]", tlsObj["alpn"])
	}
}

func TestParseEncodedFragmentNames(t *testing.T) {
	tests := []struct {
		name     string
		nodeURL  string
		protocol string
		wantName string
	}{
		{
			name:     "VLESS 编码名称",
			nodeURL:  "vless://uuid@example.com:443?security=tls#%E4%B8%9C%E4%BA%AC",
			protocol: "vless",
			wantName: "东京",
		},
		{
			name:     "Hysteria2 编码名称",
			nodeURL:  "hysteria2://pass@example.com:443#%E6%96%B0%E5%8A%A0%E5%9D%A1",
			protocol: "hysteria2",
			wantName: "新加坡",
		},
		{
			name:     "TUIC 编码名称",
			nodeURL:  "tuic://uuid:pass@example.com:443#%E9%A6%99%E6%B8%AF",
			protocol: "tuic",
			wantName: "香港",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNodeURL(tt.nodeURL)
			if err != nil {
				t.Fatalf("parseNodeURL() error = %v", err)
			}
			if got.Protocol != tt.protocol {
				t.Fatalf("parseNodeURL() protocol = %s, want %s", got.Protocol, tt.protocol)
			}
			if got.Name != tt.wantName {
				t.Fatalf("parseNodeURL() name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}

func TestParseClashYAMLNormalizesHY2(t *testing.T) {
	clashYAML := `proxies:
  - name: HY2 节点
    type: hy2
    server: example.com
    port: 443
    password: secret
    sni: example.com
`

	nodes, err := parseClashYAML(clashYAML)
	if err != nil {
		t.Fatalf("parseClashYAML() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("parseClashYAML() 节点数 = %d, want 1", len(nodes))
	}
	if nodes[0].Protocol != "hysteria2" {
		t.Fatalf("parseClashYAML() Protocol = %s, want hysteria2", nodes[0].Protocol)
	}
}

func TestParseShadowsocksNode(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    *ProxyNode
		wantErr bool
	}{
		{
			name: "SIP002 格式 Shadowsocks 链接",
			url:  "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ=@example.com:8388#SSNode",
			want: &ProxyNode{
				Protocol: "ss",
				Name:     "SSNode",
				Server:   "example.com",
				Port:     8388,
				Options: map[string]interface{}{
					"cipher":   "aes-256-gcm",
					"password": "password",
				},
			},
			wantErr: false,
		},
		{
			name: "空格分隔格式 Shadowsocks 链接",
			url:  "ss://aes-256-gcm:password@example.com:8388#SSNode2",
			want: &ProxyNode{
				Protocol: "ss",
				Name:     "SSNode2",
				Server:   "example.com",
				Port:     8388,
				Options: map[string]interface{}{
					"cipher":   "aes-256-gcm",
					"password": "password",
				},
			},
			wantErr: false,
		},
		{
			name: "SIP002 带 query 参数的 Shadowsocks 链接",
			url:  "ss://YWVzLTI1Ni1nY206OGI4Z3pnWkpVdXQ3NEtMV0k4ckNtSkpYS2hiNkplN1dqaHgxM0Eyc0tQOD0@72.234.229.126:38280?type=tcp#telegram%40wenwencc-f3lpezxl",
			want: &ProxyNode{
				Protocol: "ss",
				Name:     "telegram@wenwencc-f3lpezxl",
				Server:   "72.234.229.126",
				Port:     38280,
				Options: map[string]interface{}{
					"cipher":   "aes-256-gcm",
					"password": "8b8gzgZJUut74KLWI8rCmJJXKhb6Je7Wjhx13A2sKP8=",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseShadowsocksNode(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseShadowsocksNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.want != nil {
				if got == nil {
					t.Errorf("parseShadowsocksNode() returned nil, want non-nil")
					return
				}
				if got.Protocol != tt.want.Protocol {
					t.Errorf("parseShadowsocksNode() Protocol = %v, want %v", got.Protocol, tt.want.Protocol)
				}
				if got.Server != tt.want.Server {
					t.Errorf("parseShadowsocksNode() Server = %v, want %v", got.Server, tt.want.Server)
				}
				if got.Port != tt.want.Port {
					t.Errorf("parseShadowsocksNode() Port = %v, want %v", got.Port, tt.want.Port)
				}
			}
		})
	}
}

func TestParseHysteria2Node(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    *ProxyNode
		wantErr bool
	}{
		{
			name: "标准 Hysteria2 链接",
			url:  "hysteria2://password@example.com:443?sni=example.com&insecure=1#H2Node",
			want: &ProxyNode{
				Protocol: "hysteria2",
				Name:     "H2Node",
				Server:   "example.com",
				Port:     443,
				Options: map[string]interface{}{
					"password":       "password",
					"sni":            "example.com",
					"skipCertVerify": true,
				},
			},
			wantErr: false,
		},
		{
			name: "hy2 短格式",
			url:  "hy2://password@example.com:443#HY2Node",
			want: &ProxyNode{
				Protocol: "hysteria2",
				Name:     "HY2Node",
				Server:   "example.com",
				Port:     443,
				Options: map[string]interface{}{
					"password":       "password",
					"sni":            "",
					"skipCertVerify": false,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHysteria2Node(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHysteria2Node() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Protocol != tt.want.Protocol {
					t.Errorf("parseHysteria2Node() Protocol = %v, want %v", got.Protocol, tt.want.Protocol)
				}
				if got.Name != tt.want.Name {
					t.Errorf("parseHysteria2Node() Name = %v, want %v", got.Name, tt.want.Name)
				}
				if got.Server != tt.want.Server {
					t.Errorf("parseHysteria2Node() Server = %v, want %v", got.Server, tt.want.Server)
				}
			}
		})
	}
}

func TestParseTUICNode(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    *ProxyNode
		wantErr bool
	}{
		{
			name: "标准 TUIC 链接",
			url:  "tuic://uuid:password@example.com:443?sni=example.com&alpn=h3#TUICNode",
			want: &ProxyNode{
				Protocol: "tuic",
				Name:     "TUICNode",
				Server:   "example.com",
				Port:     443,
				Options: map[string]interface{}{
					"uuid":     "uuid",
					"password": "password",
					"sni":      "example.com",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTUICNode(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTUICNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Protocol != tt.want.Protocol {
					t.Errorf("parseTUICNode() Protocol = %v, want %v", got.Protocol, tt.want.Protocol)
				}
				if got.Server != tt.want.Server {
					t.Errorf("parseTUICNode() Server = %v, want %v", got.Server, tt.want.Server)
				}
				tlsObj, ok := got.Options["tls"].(map[string]interface{})
				if !ok {
					t.Fatalf("parseTUICNode() 缺少 tls 对象，got=%#v", got.Options["tls"])
				}
				if tlsObj["server_name"] != "example.com" {
					t.Fatalf("parseTUICNode() tls.server_name = %v, want example.com", tlsObj["server_name"])
				}
				if alpn, ok := tlsObj["alpn"].([]string); !ok || len(alpn) != 1 || alpn[0] != "h3" {
					t.Fatalf("parseTUICNode() tls.alpn = %#v, want [h3]", tlsObj["alpn"])
				}
			}
		})
	}
}

func TestParseAnyTLSNode(t *testing.T) {
	got, err := parseAnyTLSNode("anytls://secret@example.com:8443/?sni=edge.example.com&insecure=1&alpn=h2,http/1.1&fp=chrome#AnyTLSNode")
	if err != nil {
		t.Fatalf("parseAnyTLSNode() error = %v", err)
	}
	if got.Protocol != "anytls" {
		t.Fatalf("parseAnyTLSNode() Protocol = %v, want anytls", got.Protocol)
	}
	if got.Server != "example.com" || got.Port != 8443 {
		t.Fatalf("parseAnyTLSNode() server/port = %s:%d, want example.com:8443", got.Server, got.Port)
	}
	if got.Name != "AnyTLSNode" {
		t.Fatalf("parseAnyTLSNode() name = %s, want AnyTLSNode", got.Name)
	}
	if got.Options["password"] != "secret" {
		t.Fatalf("parseAnyTLSNode() password = %#v, want secret", got.Options["password"])
	}
	tlsObj, ok := got.Options["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("parseAnyTLSNode() 缺少 tls 对象，got=%#v", got.Options["tls"])
	}
	if tlsObj["server_name"] != "edge.example.com" {
		t.Fatalf("parseAnyTLSNode() tls.server_name = %v, want edge.example.com", tlsObj["server_name"])
	}
	if tlsObj["insecure"] != true {
		t.Fatalf("parseAnyTLSNode() tls.insecure = %v, want true", tlsObj["insecure"])
	}
}

func TestParseNodeURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		check   func(*testing.T, *ProxyNode)
	}{
		{
			name:    "Trojan",
			url:     "trojan://pass@example.com#TrojanNode",
			wantErr: false,
			check: func(t *testing.T, n *ProxyNode) {
				if n.Protocol != "trojan" {
					t.Errorf("Expected protocol trojan, got %s", n.Protocol)
				}
			},
		},
		{
			name:    "VLESS",
			url:     "vless://uuid@example.com#VLESSNode",
			wantErr: false,
			check: func(t *testing.T, n *ProxyNode) {
				if n.Protocol != "vless" {
					t.Errorf("Expected protocol vless, got %s", n.Protocol)
				}
			},
		},
		{
			name:    "Hysteria2",
			url:     "hysteria2://pass@example.com#H2Node",
			wantErr: false,
			check: func(t *testing.T, n *ProxyNode) {
				if n.Protocol != "hysteria2" {
					t.Errorf("Expected protocol hysteria2, got %s", n.Protocol)
				}
			},
		},
		{
			name:    "TUIC",
			url:     "tuic://uuid:pass@example.com#TUICNode",
			wantErr: false,
			check: func(t *testing.T, n *ProxyNode) {
				if n.Protocol != "tuic" {
					t.Errorf("Expected protocol tuic, got %s", n.Protocol)
				}
			},
		},
		{
			name:    "AnyTLS",
			url:     "anytls://secret@example.com:443/?sni=edge.example.com#AnyTLSNode",
			wantErr: false,
			check: func(t *testing.T, n *ProxyNode) {
				if n.Protocol != "anytls" {
					t.Errorf("Expected protocol anytls, got %s", n.Protocol)
				}
			},
		},
		{
			name:    "不支持的协议",
			url:     "unknown://test",
			wantErr: true,
			check:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNodeURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseNodeURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestParseSubscription(t *testing.T) {
	// 混合协议订阅测试
	mixedContent := `trojan://pass1@example.com#Trojan1
vless://uuid@example.com#VLESS1
anytls://secret@example.com:443/?sni=edge.example.com#AnyTLS1
hysteria2://pass2@example.com#H2_1
tuic://uuid:pass@example.com#TUIC1
`

	nodes, err := ParseSubscription(mixedContent)
	if err != nil {
		t.Fatalf("parseSubscription() error = %v", err)
	}

	if len(nodes) != 5 {
		t.Errorf("parseSubscription() returned %d nodes, want 5", len(nodes))
	}

	protocols := make(map[string]bool)
	for _, node := range nodes {
		protocols[node.Protocol] = true
	}

	expectedProtocols := []string{"trojan", "vless", "anytls", "hysteria2", "tuic"}
	for _, proto := range expectedProtocols {
		if !protocols[proto] {
			t.Errorf("parseSubscription() missing protocol %s", proto)
		}
	}
}
