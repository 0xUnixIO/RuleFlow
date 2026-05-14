package app

import (
	"fmt"
	"strings"
)

// Clash 配置结构
type ClashConfig struct {
	Port            int      `yaml:"port"`
	SocksPort       int      `yaml:"socks-port"`
	RedirPort       int      `yaml:"redir-port"`
	MixedPort       int      `yaml:"mixed-port"`
	AllowLan        bool     `yaml:"allow-lan"`
	Mode            string   `yaml:"mode"`
	LogLevel        string   `yaml:"log-level"`
	ExternalControl string   `yaml:"external-controller"`
	Proxies         []Proxy  `yaml:"proxies"`
	ProxyGroups     []Group  `yaml:"proxy-groups"`
	Rules           []string `yaml:"rules"`
}

type Proxy struct {
	Name                     string          `yaml:"name"`
	Type                     string          `yaml:"type"`
	Server                   string          `yaml:"server,omitempty"`
	Port                     int             `yaml:"port,omitempty"`
	Version                  int             `yaml:"version,omitempty"`
	Password                 string          `yaml:"password,omitempty"`
	UDP                      bool            `yaml:"udp,omitempty"`
	SNI                      string          `yaml:"sni,omitempty"`
	SkipCertVerify           bool            `yaml:"skip-cert-verify,omitempty"`
	Network                  string          `yaml:"network,omitempty"`
	Security                 string          `yaml:"security,omitempty"`
	UUID                     string          `yaml:"uuid,omitempty"`
	AlterID                  int             `yaml:"alterId,omitempty"`
	Cipher                   string          `yaml:"cipher,omitempty"`
	Flow                     string          `yaml:"flow,omitempty"`
	TLS                      bool            `yaml:"tls,omitempty"`
	Alpn                     []string        `yaml:"alpn,omitempty"`
	Servername               string          `yaml:"servername,omitempty"` // Clash Mihomo VLESS 使用 servername，Stash 使用 sni
	Fingerprint              string          `yaml:"client-fingerprint,omitempty"`
	Reality                  *RealityCfg     `yaml:"reality-opts,omitempty"`
	WSOpts                   *WSOpts         `yaml:"ws-opts,omitempty"`
	HTTPOpts                 *HTTPOpts       `yaml:"http-opts,omitempty"`
	GRPCOpts                 *GRPCOpts       `yaml:"grpc-opts,omitempty"`
	DialerProxy              string          `yaml:"dialer-proxy,omitempty"`
	IP                       string          `yaml:"ip,omitempty"`
	IPv6                     string          `yaml:"ipv6,omitempty"`
	PrivateKey               string          `yaml:"private-key,omitempty"`
	PublicKey                string          `yaml:"public-key,omitempty"`
	PreSharedKey             string          `yaml:"pre-shared-key,omitempty"`
	DNS                      []string        `yaml:"dns,omitempty"`
	MTU                      int             `yaml:"mtu,omitempty"`
	RemoteDNSResolve         bool            `yaml:"remote-dns-resolve,omitempty"`
	Reserved                 []int           `yaml:"reserved,omitempty"`
	PersistentKeepalive      int             `yaml:"persistent-keepalive,omitempty"`
	Peers                    []WireGuardPeer `yaml:"peers,omitempty"`
	IdleSessionCheckInterval int             `yaml:"idle-session-check-interval,omitempty"`
	IdleSessionTimeout       int             `yaml:"idle-session-timeout,omitempty"`
	MinIdleSession           int             `yaml:"min-idle-session,omitempty"`
}

type WSOpts struct {
	Path    string            `yaml:"path,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

type HTTPOpts struct {
	Path    string            `yaml:"path,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

type GRPCOpts struct {
	GrpcServiceName string `yaml:"grpc-service-name,omitempty"`
}

type RealityCfg struct {
	PublicKey string `yaml:"public-key,omitempty"`
	ShortID   string `yaml:"short-id,omitempty"`
}

type WireGuardPeer struct {
	Server              string   `yaml:"server,omitempty"`
	Port                int      `yaml:"port,omitempty"`
	PublicKey           string   `yaml:"public-key,omitempty"`
	PreSharedKey        string   `yaml:"pre-shared-key,omitempty"`
	AllowedIPs          []string `yaml:"allowed-ips,omitempty"`
	Reserved            []int    `yaml:"reserved,omitempty"`
	PersistentKeepalive int      `yaml:"persistent-keepalive,omitempty"`
}

type Group struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Proxies []string `yaml:"proxies"`
}

// ProxyNode 通用代理节点
type ProxyNode struct {
	Protocol    string                 // 协议类型: trojan, vmess, vless, ss, wireguard, anytls, hysteria2, tuic
	Name        string                 // 节点名称
	Server      string                 // 服务器地址
	Port        int                    // 端口
	Options     map[string]interface{} // 协议特定选项
	DialerProxy string                 // 链式代理中转名称；Surge 导出时会映射为 underlying-proxy
}

// 各协议的选项结构
type TrojanOptions struct {
	Password       string
	SNI            string
	SkipCertVerify bool
}

type VMessOptions struct {
	UUID        string
	AlterID     int
	Security    string
	Network     string
	WSPath      string
	TLS         bool
	SNI         string
	Alpn        []string
	Fingerprint string
}

type VLESSOptions struct {
	UUID        string
	Flow        string
	Network     string
	WSPath      string
	TLS         bool
	SNI         string
	Fingerprint string
	Reality     *RealityConfig
}

type RealityConfig struct {
	PublicKey string `json:"public-key"`
	ShortID   string `json:"short-id"`
}

type TLSOptions struct {
	Enabled    bool           `json:"enabled"`
	ServerName string         `json:"server_name,omitempty"`
	Insecure   bool           `json:"insecure,omitempty"`
	ALPN       []string       `json:"alpn,omitempty"`
	UTLS       *UTLSOptions   `json:"utls,omitempty"`
	Reality    *RealityConfig `json:"reality,omitempty"`
}

type UTLSOptions struct {
	Enabled     bool   `json:"enabled"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

type TransportOptions struct {
	Type        string            `json:"type"`
	Path        string            `json:"path,omitempty"`
	Host        string            `json:"host,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	ServiceName string            `json:"service_name,omitempty"`
}

type ShadowsocksOptions struct {
	Cipher   string
	Password string
}

type Hysteria2Options struct {
	Password       string
	SNI            string
	SkipCertVerify bool
}

type TUICOptions struct {
	UUID     string
	Password string
	SNI      string
}

// Trojan 节点信息（保留以向后兼容）
type TrojanNode struct {
	Name     string
	Server   string
	Port     int
	Password string
	SNI      string
}

var defaultRules = []string{
	"DOMAIN-SUFFIX,local,DIRECT",
	"IP-CIDR,127.0.0.0/8,DIRECT",
	"IP-CIDR,172.16.0.0/12,DIRECT",
	"IP-CIDR,192.168.0.0/16,DIRECT",
	"IP-CIDR,10.0.0.0/8,DIRECT",
	"DOMAIN-SUFFIX,cn,DIRECT",
	"DOMAIN-KEYWORD,-cn,DIRECT",
	"GEOIP,CN,DIRECT",
	"MATCH,🚀 节点选择",
}

func cloneRules(rules []string) []string {
	return append([]string(nil), rules...)
}

// buildProxies 从通用节点构建代理配置
func buildProxies(nodes []*ProxyNode) ([]Proxy, []string) {
	proxies := make([]Proxy, 0, len(nodes))
	proxyNames := make([]string, 0, len(nodes))

	for i, node := range nodes {
		proxyName := ensureNodeName(node, i)
		proxyNames = append(proxyNames, proxyName)

		proxy := Proxy{
			Name:        proxyName,
			Type:        node.Protocol,
			Server:      node.Server,
			Port:        node.Port,
			UDP:         true,
			DialerProxy: node.DialerProxy,
		}

		// 根据协议类型添加特定字段
		switch node.Protocol {
		case "trojan":
			addTrojanFields(&proxy, node.Options)
		case "vmess":
			addVMessFields(&proxy, node.Options)
		case "vless":
			addVLESSFields(&proxy, node.Options)
		case "ss":
			addShadowsocksFields(&proxy, node.Options)
		case "hysteria2", "hy2":
			addHysteria2Fields(&proxy, node.Options)
		case "wireguard":
			addWireGuardFields(&proxy, node)
		case "anytls":
			addAnyTLSFields(&proxy, node.Options)
		case "tuic":
			addTUICFields(&proxy, node.Options)
		}

		proxies = append(proxies, proxy)
	}

	return proxies, proxyNames
}

// buildProxiesFromTrojan 从 Trojan 节点构建代理配置（向后兼容）
func buildProxiesFromTrojan(nodes []TrojanNode) ([]Proxy, []string) {
	proxies := make([]Proxy, 0, len(nodes))
	proxyNames := make([]string, 0, len(nodes))

	for i, node := range nodes {
		proxyName := fmt.Sprintf("Trojan-%d", i+1)
		if node.Name != "" && node.Name != node.Server {
			proxyName = node.Name
		}

		proxies = append(proxies, Proxy{
			Name:           proxyName,
			Type:           "trojan",
			Server:         node.Server,
			Port:           node.Port,
			Password:       node.Password,
			UDP:            true,
			SNI:            node.SNI,
			SkipCertVerify: true,
		})
		proxyNames = append(proxyNames, proxyName)
	}

	return proxies, proxyNames
}

// ensureNodeName 确保节点有名称，并自动在前面添加国家 emoji
func ensureNodeName(node *ProxyNode, index int) string {
	var name string
	if node.Name != "" && node.Name != node.Server {
		name = node.Name
	} else {
		protocol := strings.ToUpper(node.Protocol)
		name = fmt.Sprintf("%s-%d", protocol, index+1)
	}
	return addCountryEmoji(name)
}

// 协议特定字段添加函数
func addTrojanFields(proxy *Proxy, opts map[string]interface{}) {
	if password, ok := opts["password"].(string); ok {
		proxy.Password = password
	}
	applyTLSFields(proxy, opts)
	applyTransportFields(proxy, opts)
}

func addVMessFields(proxy *Proxy, opts map[string]interface{}) {
	if uuid, ok := opts["uuid"].(string); ok {
		proxy.UUID = uuid
	}
	if alterID, ok := opts["alterID"].(int); ok {
		proxy.AlterID = alterID
	}
	if security, ok := opts["security"].(string); ok {
		proxy.Security = security
	}
	applyTLSFields(proxy, opts)
	applyTransportFields(proxy, opts)
}

func addVLESSFields(proxy *Proxy, opts map[string]interface{}) {
	if uuid, ok := opts["uuid"].(string); ok {
		proxy.UUID = uuid
	}
	if flow, ok := opts["flow"].(string); ok {
		proxy.Flow = flow
	}
	applyTLSFields(proxy, opts)
	applyTransportFields(proxy, opts)
}

func addShadowsocksFields(proxy *Proxy, opts map[string]interface{}) {
	if cipher, ok := opts["cipher"].(string); ok {
		proxy.Cipher = cipher
	}
	if password, ok := opts["password"].(string); ok {
		proxy.Password = password
	}
	// SS 通常不需要 UDP
	proxy.UDP = false
}

func addHysteria2Fields(proxy *Proxy, opts map[string]interface{}) {
	if password, ok := opts["password"].(string); ok {
		proxy.Password = password
	}
	applyTLSFields(proxy, opts)
}

func addWireGuardFields(proxy *Proxy, node *ProxyNode) {
	cfg := extractWireGuardConfig(node)
	for _, addr := range cfg.LocalAddresses {
		if strings.Contains(addr, ":") {
			if proxy.IPv6 == "" {
				proxy.IPv6 = addr
			}
			continue
		}
		if proxy.IP == "" {
			proxy.IP = addr
		}
	}
	proxy.PrivateKey = cfg.PrivateKey
	proxy.DNS = append([]string(nil), cfg.DNS...)
	proxy.MTU = cfg.MTU
	proxy.RemoteDNSResolve = cfg.RemoteDNSResolve
	if len(cfg.Peers) > 0 {
		proxy.Server = ""
		proxy.Port = 0
		proxy.Peers = make([]WireGuardPeer, 0, len(cfg.Peers))
		for _, peer := range cfg.Peers {
			proxy.Peers = append(proxy.Peers, WireGuardPeer{
				Server:              peer.Server,
				Port:                peer.Port,
				PublicKey:           peer.PublicKey,
				PreSharedKey:        peer.PreSharedKey,
				AllowedIPs:          append([]string(nil), peer.AllowedIPs...),
				Reserved:            append([]int(nil), peer.Reserved...),
				PersistentKeepalive: peer.PersistentKeepalive,
			})
		}
	}
}

func addAnyTLSFields(proxy *Proxy, opts map[string]interface{}) {
	if password, ok := opts["password"].(string); ok {
		proxy.Password = password
	}
	applyTLSFields(proxy, opts)
	if v, ok := intOption(opts, "idleSessionCheckInterval", "idle-session-check-interval"); ok {
		proxy.IdleSessionCheckInterval = v
	}
	if v, ok := intOption(opts, "idleSessionTimeout", "idle-session-timeout"); ok {
		proxy.IdleSessionTimeout = v
	}
	if v, ok := intOption(opts, "minIdleSession", "min-idle-session"); ok {
		proxy.MinIdleSession = v
	}
}

func addTUICFields(proxy *Proxy, opts map[string]interface{}) {
	proxy.Version = 5
	if uuid, ok := opts["uuid"].(string); ok {
		proxy.UUID = uuid
	}
	if password, ok := opts["password"].(string); ok {
		proxy.Password = password
	}
	applyTLSFields(proxy, opts)
}

func applyTLSFields(proxy *Proxy, opts map[string]interface{}) {
	if tlsObj, ok := extractTLSOptions(opts); ok {
		proxy.TLS = tlsObj.Enabled
		if tlsObj.ServerName != "" {
			proxy.SNI = tlsObj.ServerName
		}
		proxy.SkipCertVerify = tlsObj.Insecure
		if len(tlsObj.ALPN) > 0 {
			proxy.Alpn = tlsObj.ALPN
		}
		if tlsObj.UTLS != nil && tlsObj.UTLS.Fingerprint != "" {
			proxy.Fingerprint = tlsObj.UTLS.Fingerprint
		}
		if tlsObj.Reality != nil && (tlsObj.Reality.PublicKey != "" || tlsObj.Reality.ShortID != "") {
			proxy.Reality = &RealityCfg{
				PublicKey: tlsObj.Reality.PublicKey,
				ShortID:   tlsObj.Reality.ShortID,
			}
		}
		return
	}
}

func applyTransportFields(proxy *Proxy, opts map[string]interface{}) {
	if transport, ok := extractTransportOptions(opts); ok {
		if transport.Type != "" {
			proxy.Network = transport.Type
		}
		switch transport.Type {
		case "ws":
			proxy.WSOpts = &WSOpts{
				Path:    transport.Path,
				Headers: transport.Headers,
			}
		case "http", "httpupgrade":
			proxy.HTTPOpts = &HTTPOpts{
				Path:    transport.Path,
				Headers: transport.Headers,
			}
		case "grpc":
			if transport.ServiceName != "" {
				proxy.GRPCOpts = &GRPCOpts{GrpcServiceName: transport.ServiceName}
			}
		}
		return
	}
}

func extractTLSOptions(opts map[string]interface{}) (*TLSOptions, bool) {
	raw, exists := opts["tls"]
	if exists && raw != nil {
		switch value := raw.(type) {
		case *TLSOptions:
			return value, true
		case bool:
			tlsObj := buildTLSOptionsFromFlatFields(opts)
			tlsObj.Enabled = value || tlsObj.Enabled
			return tlsObj, true
		case string:
			tlsObj := buildTLSOptionsFromFlatFields(opts)
			tlsObj.Enabled = strings.EqualFold(value, "true") || strings.EqualFold(value, "tls") || strings.EqualFold(value, "reality") || tlsObj.Enabled
			return tlsObj, true
		case map[string]interface{}:
			tlsObj := &TLSOptions{}
			if enabled, ok := boolOption(value, "enabled"); ok {
				tlsObj.Enabled = enabled
			}
			if serverName, ok := stringOption(value, "server_name", "server-name", "sni"); ok {
				if serverName == "" {
					return nil, false
				}
				tlsObj.ServerName = serverName
			}
			if insecure, ok := boolOption(value, "insecure"); ok {
				tlsObj.Insecure = insecure
			}
			if alpn, ok := stringSliceOption(value, "alpn"); ok {
				tlsObj.ALPN = alpn
			}
			if utlsRaw, ok := value["utls"].(map[string]interface{}); ok {
				utlsObj := &UTLSOptions{}
				if enabled, ok := boolOption(utlsRaw, "enabled"); ok {
					utlsObj.Enabled = enabled
				}
				if fingerprint, ok := stringOption(utlsRaw, "fingerprint"); ok {
					utlsObj.Fingerprint = fingerprint
				}
				if utlsObj.Enabled || utlsObj.Fingerprint != "" {
					tlsObj.UTLS = utlsObj
				}
			}
			if realityRaw, ok := value["reality"].(map[string]interface{}); ok {
				realityObj := &RealityConfig{}
				realityObj.PublicKey, _ = stringOption(realityRaw, "public_key")
				realityObj.ShortID, _ = stringOption(realityRaw, "short_id")
				if realityObj.PublicKey != "" || realityObj.ShortID != "" {
					tlsObj.Reality = realityObj
				}
			}
			mergeFlatTLSFields(tlsObj, opts)
			return tlsObj, true
		}
	}

	tlsObj := buildTLSOptionsFromFlatFields(opts)
	if tlsObj == nil {
		return nil, false
	}
	return tlsObj, true
}

func buildTLSOptionsFromFlatFields(opts map[string]interface{}) *TLSOptions {
	tlsObj := &TLSOptions{}
	if security, ok := stringOption(opts, "security"); ok {
		if strings.EqualFold(security, "tls") || strings.EqualFold(security, "reality") {
			tlsObj.Enabled = true
		}
	}
	mergeFlatTLSFields(tlsObj, opts)
	if !tlsObj.Enabled && tlsObj.ServerName == "" && !tlsObj.Insecure && len(tlsObj.ALPN) == 0 && tlsObj.UTLS == nil && tlsObj.Reality == nil {
		return nil
	}
	return tlsObj
}

func mergeFlatTLSFields(tlsObj *TLSOptions, opts map[string]interface{}) {
	if tlsObj == nil {
		return
	}
	// servername 是 Clash YAML 格式的 VLESS SNI 字段名
	if serverName, ok := stringOption(opts, "sni", "server_name", "server-name", "servername"); ok && serverName != "" && tlsObj.ServerName == "" {
		tlsObj.ServerName = serverName
	}
	if insecure, ok := boolOption(opts, "skip-cert-verify", "insecure"); ok {
		tlsObj.Insecure = tlsObj.Insecure || insecure
	}
	if len(tlsObj.ALPN) == 0 {
		if alpn, ok := stringSliceOption(opts, "alpn"); ok {
			tlsObj.ALPN = alpn
		}
	}
	if tlsObj.UTLS == nil {
		if fingerprint, ok := stringOption(opts, "fp", "fingerprint", "client-fingerprint"); ok && fingerprint != "" {
			tlsObj.UTLS = &UTLSOptions{
				Enabled:     true,
				Fingerprint: fingerprint,
			}
		}
	}
	if tlsObj.Reality == nil {
		publicKey, _ := stringOption(opts, "pbk", "public-key")
		shortID, _ := stringOption(opts, "sid", "short-id")
		// 兼容 Clash YAML 的 reality-opts 嵌套格式（订阅二次转换场景）
		if realityOpts, ok := nestedMapOption(opts, "reality-opts"); ok {
			if pk, ok := stringOption(realityOpts, "public-key"); ok && publicKey == "" {
				publicKey = pk
			}
			if si, ok := stringOption(realityOpts, "short-id"); ok && shortID == "" {
				shortID = si
			}
		}
		if publicKey != "" || shortID != "" {
			tlsObj.Enabled = true
			tlsObj.Reality = &RealityConfig{
				PublicKey: publicKey,
				ShortID:   shortID,
			}
		}
	}
}

func extractTransportOptions(opts map[string]interface{}) (*TransportOptions, bool) {
	raw, exists := opts["transport"]
	if exists && raw != nil {
		switch value := raw.(type) {
		case *TransportOptions:
			return value, true
		case map[string]interface{}:
			transport := &TransportOptions{}
			transport.Type, _ = stringOption(value, "type")
			transport.Path, _ = stringOption(value, "path")
			transport.Host, _ = stringOption(value, "host")
			transport.ServiceName, _ = stringOption(value, "service_name")
			if headers := stringMapOption(value, "headers"); len(headers) > 0 {
				transport.Headers = headers
			}
			return transport, true
		default:
			return nil, false
		}
	}

	transport := &TransportOptions{}
	transport.Type, _ = stringOption(opts, "network")
	switch transport.Type {
	case "ws":
		if wsOpts, ok := nestedMapOption(opts, "ws-opts", "ws_opts"); ok {
			transport.Path, _ = stringOption(wsOpts, "path")
			if headers := stringMapOption(wsOpts, "headers"); len(headers) > 0 {
				transport.Headers = headers
				if host, ok := headers["Host"]; ok && host != "" {
					transport.Host = host
				}
			}
		}
	case "http", "httpupgrade":
		if httpOpts, ok := nestedMapOption(opts, "http-opts", "http_opts"); ok {
			transport.Path, _ = stringOption(httpOpts, "path")
			if headers := stringMapOption(httpOpts, "headers"); len(headers) > 0 {
				transport.Headers = headers
				if host, ok := headers["Host"]; ok && host != "" {
					transport.Host = host
				}
			}
		}
	case "grpc":
		if grpcOpts, ok := nestedMapOption(opts, "grpc-opts", "grpc_opts"); ok {
			transport.ServiceName, _ = stringOption(grpcOpts, "grpc-service-name", "grpc_service_name", "service_name")
		}
	}

	if transport.Type == "" {
		return nil, false
	}
	return transport, true
}

func nestedMapOption(opts map[string]interface{}, keys ...string) (map[string]interface{}, bool) {
	for _, key := range keys {
		if raw, ok := opts[key]; ok {
			if value, ok := raw.(map[string]interface{}); ok {
				return value, true
			}
		}
	}
	return nil, false
}

func stringMapOption(opts map[string]interface{}, keys ...string) map[string]string {
	for _, key := range keys {
		raw, ok := opts[key]
		if !ok || raw == nil {
			continue
		}
		switch value := raw.(type) {
		case map[string]string:
			out := make(map[string]string, len(value))
			for k, v := range value {
				if v != "" {
					out[k] = v
				}
			}
			if len(out) > 0 {
				return out
			}
		case map[string]interface{}:
			out := make(map[string]string, len(value))
			for k, v := range value {
				if s, ok := v.(string); ok && s != "" {
					out[k] = s
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func stringOption(opts map[string]interface{}, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := opts[key].(string); ok && value != "" {
			return value, true
		}
	}
	return "", false
}

func boolOption(opts map[string]interface{}, keys ...string) (bool, bool) {
	for _, key := range keys {
		if value, ok := opts[key].(bool); ok {
			return value, true
		}
	}
	return false, false
}

func intOption(opts map[string]interface{}, keys ...string) (int, bool) {
	for _, key := range keys {
		switch value := opts[key].(type) {
		case int:
			return value, true
		case int64:
			return int(value), true
		case float64:
			return int(value), true
		}
	}
	return 0, false
}

func stringSliceOption(opts map[string]interface{}, keys ...string) ([]string, bool) {
	for _, key := range keys {
		switch value := opts[key].(type) {
		case []string:
			if len(value) > 0 {
				return value, true
			}
		case []interface{}:
			out := make([]string, 0, len(value))
			for _, item := range value {
				if s, ok := item.(string); ok && s != "" {
					out = append(out, s)
				}
			}
			if len(out) > 0 {
				return out, true
			}
		}
	}
	return nil, false
}

func builtInProxyName(name string) bool {
	switch name {
	case "DIRECT", "REJECT", "REJECT-DROP", "REJECT-NO-DROP", "PASS", "COMPATIBLE":
		return true
	default:
		return false
	}
}
