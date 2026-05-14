package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// 多协议 URL 匹配模式
var (
	trojanURLPattern    = regexp.MustCompile(`trojan://[^\s]+`)
	vmessURLPattern     = regexp.MustCompile(`vmess://[a-zA-Z0-9_=]+`)
	vlessURLPattern     = regexp.MustCompile(`vless://[^\s]+`)
	ssURLPattern        = regexp.MustCompile(`ss://[^\s]+`)
	anytlsURLPattern    = regexp.MustCompile(`anytls://[^\s]+`)
	hysteria2URLPattern = regexp.MustCompile(`(?:hysteria2|hy2)://[^\s]+`)
	tuicURLPattern      = regexp.MustCompile(`tuic://[^\s]+`)
	allProtocolPattern  = regexp.MustCompile(`(trojan|vmess|vless|ss|anytls|hysteria2|hy2|tuic)://[^\s]+`)
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;:]*[A-Za-z]`)

// ParseNodeURL 解析任意协议的节点 URL（导出版本）
func ParseNodeURL(nodeURL string) (*ProxyNode, error) {
	return parseNodeURL(nodeURL)
}

// parseNodeURL 解析任意协议的节点 URL
func parseNodeURL(nodeURL string) (*ProxyNode, error) {
	nodeURL = strings.TrimSpace(nodeURL)
	if nodeURL == "" {
		return nil, fmt.Errorf("空的节点 URL")
	}

	u, err := url.Parse(nodeURL)
	if err != nil {
		return nil, fmt.Errorf("解析 URL 失败: %w", err)
	}

	switch u.Scheme {
	case "trojan":
		return parseTrojanNode(nodeURL)
	case "vmess":
		return parseVMessNode(nodeURL)
	case "vless":
		return parseVLESSNode(nodeURL)
	case "ss":
		return parseShadowsocksNode(nodeURL)
	case "anytls":
		return parseAnyTLSNode(nodeURL)
	case "hysteria2", "hy2":
		return parseHysteria2Node(nodeURL)
	case "tuic":
		return parseTUICNode(nodeURL)
	default:
		return nil, fmt.Errorf("不支持的协议: %s", u.Scheme)
	}
}

func parseAnyTLSNode(nodeURL string) (*ProxyNode, error) {
	nodeURL = strings.TrimSpace(nodeURL)
	if !strings.HasPrefix(nodeURL, "anytls://") {
		return nil, fmt.Errorf("无效的 AnyTLS 链接格式")
	}

	u, err := url.Parse(nodeURL)
	if err != nil {
		return nil, fmt.Errorf("解析 URL 失败: %w", err)
	}

	password := u.User.Username()
	if password == "" {
		return nil, fmt.Errorf("未找到密码")
	}

	server := u.Hostname()
	if server == "" {
		return nil, fmt.Errorf("未找到服务器地址")
	}

	port, err := parsePortWithDefault(u.Port(), 443)
	if err != nil {
		return nil, fmt.Errorf("无效端口: %w", err)
	}

	query := u.Query()
	name := query.Get("name")
	if name == "" {
		name = decodeURLFragment(u)
	}
	if name == "" {
		name = server
	}

	sni := query.Get("sni")
	if sni == "" {
		sni = server
	}

	insecure := queryBool(query, "insecure", "allowInsecure")

	opts := map[string]interface{}{
		"password": password,
		"tls":      buildTLSOptions(true, sni, insecure, parseALPN(query), query.Get("fp")),
	}

	return &ProxyNode{
		Protocol: "anytls",
		Name:     name,
		Server:   server,
		Port:     port,
		Options:  opts,
	}, nil
}

// parseTrojanNode 解析 Trojan 节点
func parseTrojanNode(nodeURL string) (*ProxyNode, error) {
	nodeURL = strings.TrimSpace(nodeURL)
	if !strings.HasPrefix(nodeURL, "trojan://") {
		return nil, fmt.Errorf("无效的 Trojan 链接格式")
	}

	u, err := url.Parse(nodeURL)
	if err != nil {
		return nil, fmt.Errorf("解析 URL 失败: %w", err)
	}

	password := u.User.Username()
	if password == "" {
		return nil, fmt.Errorf("未找到密码")
	}

	server := u.Hostname()
	port, _ := parsePortWithDefault(u.Port(), 443)

	query := u.Query()
	name := query.Get("name")
	if name == "" {
		name = query.Get("hash")
	}
	if name == "" {
		name = decodeURLFragment(u)
	}
	if name == "" {
		name = server
	}

	sni := query.Get("sni")
	if sni == "" {
		sni = server
	}

	skipCertVerify := queryBool(query, "allowInsecure")

	opts := map[string]interface{}{
		"password": password,
		"tls":      buildTLSOptions(true, sni, skipCertVerify, parseALPN(query), ""),
	}
	if transport := parseTransportOptions(query); transport != nil {
		opts["transport"] = transport
	}

	return &ProxyNode{
		Protocol: "trojan",
		Name:     name,
		Server:   server,
		Port:     port,
		Options:  opts,
	}, nil
}

// parseVMessNode 解析 VMess 节点
func parseVMessNode(nodeURL string) (*ProxyNode, error) {
	nodeURL = strings.TrimSpace(nodeURL)
	if !strings.HasPrefix(nodeURL, "vmess://") {
		return nil, fmt.Errorf("无效的 VMess 链接格式")
	}

	// 移除 vmess:// 前缀
	base64Part := strings.TrimPrefix(nodeURL, "vmess://")

	// 尝试直接解析为 JSON 格式
	jsonStr, err := decodeVMessBase64(base64Part)
	if err != nil {
		return nil, fmt.Errorf("解码 VMess 配置失败: %w", err)
	}

	// 解析 JSON（简化版，实际应使用 json.Unmarshal）
	config, err := parseVMessJSON(jsonStr)
	if err != nil {
		return nil, fmt.Errorf("解析 VMess JSON 失败: %w", err)
	}

	return config, nil
}

// parseVLESSNode 解析 VLESS 节点
func parseVLESSNode(nodeURL string) (*ProxyNode, error) {
	nodeURL = strings.TrimSpace(nodeURL)
	if !strings.HasPrefix(nodeURL, "vless://") {
		return nil, fmt.Errorf("无效的 VLESS 链接格式")
	}

	u, err := url.Parse(nodeURL)
	if err != nil {
		return nil, fmt.Errorf("解析 URL 失败: %w", err)
	}

	// VLESS 格式: vless://uuid@server:port?params#name
	if u.User == nil {
		return nil, fmt.Errorf("未找到 UUID")
	}

	uuid := u.User.Username()
	server := u.Hostname()
	port, _ := parsePortWithDefault(u.Port(), 443)

	query := u.Query()
	name := decodeURLFragment(u)
	if name == "" {
		name = server
	}

	// 解析参数
	network := query.Get("type")
	if network == "" {
		network = "tcp"
	}

	security := query.Get("security")
	tls := security == "tls" || security == "reality"

	opts := map[string]interface{}{
		"uuid": uuid,
	}

	if flow := query.Get("flow"); flow != "" {
		opts["flow"] = flow
	}
	alpn := parseALPN(query)
	tlsObj := map[string]interface{}{}
	if fingerprint := query.Get("fp"); fingerprint != "" {
		tlsObj = buildTLSOptions(tls, query.Get("sni"), false, alpn, fingerprint)
	} else if tls || len(alpn) > 0 || query.Get("sni") != "" {
		tlsObj = buildTLSOptions(tls, query.Get("sni"), false, alpn, "")
	}
	if len(tlsObj) > 0 {
		opts["tls"] = tlsObj
	} else {
		opts["tls"] = false
	}
	if transport := parseTransportOptions(query); transport != nil {
		opts["transport"] = transport
	}

	// Reality 配置
	if security == "reality" {
		reality := &RealityConfig{
			PublicKey: query.Get("pbk"),
			ShortID:   query.Get("sid"),
		}
		if tlsMap, ok := opts["tls"].(map[string]interface{}); ok {
			tlsMap["enabled"] = true
			tlsMap["reality"] = map[string]interface{}{
				"enabled":    true,
				"public_key": reality.PublicKey,
				"short_id":   reality.ShortID,
			}
		}
	}

	return &ProxyNode{
		Protocol: "vless",
		Name:     name,
		Server:   server,
		Port:     port,
		Options:  opts,
	}, nil
}

// parseShadowsocksNode 解析 Shadowsocks 节点
func parseShadowsocksNode(nodeURL string) (*ProxyNode, error) {
	nodeURL = strings.TrimSpace(nodeURL)
	if !strings.HasPrefix(nodeURL, "ss://") {
		return nil, fmt.Errorf("无效的 Shadowsocks 链接格式")
	}

	u, err := url.Parse(nodeURL)
	if err != nil {
		return nil, fmt.Errorf("解析 URL 失败: %w", err)
	}

	// 查找 @ 分隔符
	base64Part := strings.TrimPrefix(nodeURL, "ss://")
	if hashIndex := strings.Index(base64Part, "#"); hashIndex > 0 {
		base64Part = base64Part[:hashIndex]
	}
	if queryIndex := strings.Index(base64Part, "?"); queryIndex > 0 {
		base64Part = base64Part[:queryIndex]
	}

	atIndex := strings.Index(base64Part, "@")
	if atIndex <= 0 {
		return nil, fmt.Errorf("无效的 Shadowsocks 格式：缺少 @ 分隔符")
	}

	userInfo := base64Part[:atIndex]

	// 尝试解码用户信息（可能是 Base64）
	var cipher, password string
	if decoded, err := decodeSSBase64(userInfo); err == nil {
		// Base64 解码成功，解析 cipher:password
		colonIndex := strings.Index(decoded, ":")
		if colonIndex > 0 {
			cipher = decoded[:colonIndex]
			password = decoded[colonIndex+1:]
		} else {
			// 可能整个就是密码
			cipher = "aes-256-gcm" // 默认加密方式
			password = decoded
		}
	} else {
		// 不是 Base64，直接解析 cipher:password
		colonIndex := strings.Index(userInfo, ":")
		if colonIndex <= 0 {
			return nil, fmt.Errorf("无效的 Shadowsocks 用户信息格式")
		}
		cipher = userInfo[:colonIndex]
		password = userInfo[colonIndex+1:]
	}

	server := u.Hostname()
	if server == "" {
		return nil, fmt.Errorf("未找到服务器地址")
	}
	port, err := parsePortWithDefault(u.Port(), 8388)
	if err != nil {
		return nil, fmt.Errorf("无效端口: %w", err)
	}

	// 处理名称
	name := decodeURLFragment(u)
	if name == "" {
		name = server
	}

	return &ProxyNode{
		Protocol: "ss",
		Name:     name,
		Server:   server,
		Port:     port,
		Options: map[string]interface{}{
			"cipher":   cipher,
			"password": password,
		},
	}, nil
}

// parseHysteria2Node 解析 Hysteria2 节点
func parseHysteria2Node(nodeURL string) (*ProxyNode, error) {
	nodeURL = strings.TrimSpace(nodeURL)
	if !strings.HasPrefix(nodeURL, "hysteria2://") && !strings.HasPrefix(nodeURL, "hy2://") {
		return nil, fmt.Errorf("无效的 Hysteria2 链接格式")
	}

	// 标准化为 hysteria2://
	nodeURL = strings.Replace(nodeURL, "hy2://", "hysteria2://", 1)

	u, err := url.Parse(nodeURL)
	if err != nil {
		return nil, fmt.Errorf("解析 URL 失败: %w", err)
	}

	password := u.User.Username()
	if password == "" {
		return nil, fmt.Errorf("未找到密码")
	}

	server := u.Hostname()
	port, _ := parsePortWithDefault(u.Port(), 443)

	query := u.Query()
	sni := query.Get("sni")
	skipCertVerify := queryBool(query, "allow_insecure", "insecure")

	name := decodeURLFragment(u)
	if name == "" {
		name = server
	}

	return &ProxyNode{
		Protocol: "hysteria2",
		Name:     name,
		Server:   server,
		Port:     port,
		Options: map[string]interface{}{
			"password": password,
			"tls":      buildTLSOptions(true, sni, skipCertVerify, parseALPN(query), ""),
		},
	}, nil
}

// parseTUICNode 解析 TUIC 节点
func parseTUICNode(nodeURL string) (*ProxyNode, error) {
	nodeURL = strings.TrimSpace(nodeURL)
	if !strings.HasPrefix(nodeURL, "tuic://") {
		return nil, fmt.Errorf("无效的 TUIC 链接格式")
	}

	u, err := url.Parse(nodeURL)
	if err != nil {
		return nil, fmt.Errorf("解析 URL 失败: %w", err)
	}

	// TUIC 格式: tuic://uuid:password@server:port?sni=xxx#name
	// 需要从原始 URL 中提取用户信息，因为 url.User 对于包含冒号的值处理不一致
	// 提取 tuic:// 后面的部分直到 @
	userPart := strings.TrimPrefix(nodeURL, "tuic://")
	atIndex := strings.Index(userPart, "@")
	if atIndex <= 0 {
		return nil, fmt.Errorf("无效的 TUIC 用户信息格式")
	}

	userInfo := userPart[:atIndex]
	colonIndex := strings.Index(userInfo, ":")
	if colonIndex <= 0 {
		return nil, fmt.Errorf("无效的 TUIC 用户信息格式：缺少密码")
	}

	uuid := userInfo[:colonIndex]
	password := userInfo[colonIndex+1:]

	server := u.Hostname()
	if server == "" {
		return nil, fmt.Errorf("未找到服务器地址")
	}

	port, _ := parsePortWithDefault(u.Port(), 443)

	query := u.Query()
	sni := query.Get("sni")
	skipCertVerify := queryBool(query, "allow_insecure", "insecure")

	name := decodeURLFragment(u)
	if name == "" {
		name = server
	}

	return &ProxyNode{
		Protocol: "tuic",
		Name:     name,
		Server:   server,
		Port:     port,
		Options: map[string]interface{}{
			"uuid":     uuid,
			"password": password,
			"tls":      buildTLSOptions(true, sni, skipCertVerify, parseALPN(query), ""),
		},
	}, nil
}

// ========== 辅助函数 ==========

func parseTransportOptions(query url.Values) map[string]interface{} {
	transportType := strings.TrimSpace(query.Get("type"))
	if transportType == "" {
		return nil
	}

	transport := map[string]interface{}{
		"type": transportType,
	}
	if path := query.Get("path"); path != "" {
		transport["path"] = path
	}
	host := query.Get("host")
	if host == "" {
		host = query.Get("authority")
	}
	if host != "" {
		transport["host"] = host
		transport["headers"] = map[string]string{"Host": host}
	}
	if serviceName := query.Get("serviceName"); serviceName != "" {
		transport["service_name"] = serviceName
	}

	return transport
}

func parseVMessTransportOptions(network, path, host, serviceName string) map[string]interface{} {
	network = strings.TrimSpace(network)
	if network == "" || network == "tcp" {
		return nil
	}

	transport := map[string]interface{}{
		"type": network,
	}
	if path != "" {
		transport["path"] = path
	}
	if host != "" {
		transport["host"] = host
		transport["headers"] = map[string]string{"Host": host}
	}
	if serviceName != "" {
		transport["service_name"] = serviceName
	}
	return transport
}

func parseALPN(query url.Values) []string {
	return splitAndTrim(query.Get("alpn"))
}

func decodeURLFragment(u *url.URL) string {
	if u == nil || u.Fragment == "" {
		return ""
	}
	// 优先用 RawFragment 解码，支持订阅源用 + 代替空格的编码方式
	raw := u.RawFragment
	if raw == "" {
		raw = u.Fragment
	}
	if decoded, err := url.QueryUnescape(raw); err == nil && decoded != "" {
		return decoded
	}
	return u.Fragment
}

func parsePortWithDefault(portStr string, defaultPort int) (int, error) {
	if portStr == "" {
		return defaultPort, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return defaultPort, err
	}
	return port, nil
}

func queryBool(query url.Values, keys ...string) bool {
	for _, key := range keys {
		if value := strings.TrimSpace(query.Get(key)); value != "" {
			return value == "1" || strings.EqualFold(value, "true")
		}
	}
	return false
}

func splitAndTrim(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '|'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildTLSOptions(enabled bool, serverName string, insecure bool, alpn []string, fingerprint string) map[string]interface{} {
	tlsObj := map[string]interface{}{
		"enabled": enabled,
	}
	if serverName != "" {
		tlsObj["server_name"] = serverName
	}
	if insecure {
		tlsObj["insecure"] = true
	}
	if len(alpn) > 0 {
		tlsObj["alpn"] = alpn
	}
	if fingerprint != "" {
		tlsObj["utls"] = map[string]interface{}{
			"enabled":     true,
			"fingerprint": fingerprint,
		}
	}
	return tlsObj
}

func decodeURLSafeBase64String(s string) (string, error) {
	// URL 安全的 Base64 可能包含 - 和 _
	s = strings.NewReplacer("-", "+", "_", "/").Replace(s)

	if rem := len(s) % 4; rem != 0 {
		s += strings.Repeat("=", 4-rem)
	}

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", fmt.Errorf("Base64 解码失败: %w", err)
	}

	return string(decoded), nil
}

// decodeVMessBase64 解码 VMess Base64
func decodeVMessBase64(s string) (string, error) {
	return decodeURLSafeBase64String(s)
}

// decodeSSBase64 解码 Shadowsocks Base64
func decodeSSBase64(s string) (string, error) {
	// 移除可能存在的 fragment
	if idx := strings.Index(s, "#"); idx > 0 {
		s = s[:idx]
	}

	return decodeURLSafeBase64String(s)
}

// parseVMessJSON 解析 VMess JSON 配置（简化版）
func parseVMessJSON(jsonStr string) (*ProxyNode, error) {
	type vmessConfig struct {
		ID      string `json:"id"`
		Add     string `json:"add"`
		Port    int    `json:"port"`
		PS      string `json:"ps"`
		Aid     int    `json:"aid"`
		Net     string `json:"net"`
		TLS     string `json:"tls"`
		SNI     string `json:"sni"`
		Path    string `json:"path"`
		Host    string `json:"host"`
		ALPN    string `json:"alpn"`
		Service string `json:"serviceName"`
	}

	var cfg vmessConfig
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return nil, fmt.Errorf("解析 VMess JSON 失败: %w", err)
	}

	if cfg.ID == "" {
		return nil, fmt.Errorf("未找到 UUID")
	}
	if cfg.Add == "" {
		return nil, fmt.Errorf("未找到服务器地址")
	}

	name := cfg.PS
	if name == "" {
		name = cfg.Add
	}

	network := cfg.Net
	if network == "" {
		network = "tcp"
	}

	tls := cfg.TLS == "tls" || cfg.TLS == "true"

	opts := map[string]interface{}{
		"uuid":    cfg.ID,
		"alterID": cfg.Aid,
	}
	alpn := splitAndTrim(cfg.ALPN)
	if tls {
		opts["tls"] = buildTLSOptions(true, cfg.SNI, false, alpn, "")
	} else {
		opts["tls"] = false
	}
	if transport := parseVMessTransportOptions(cfg.Net, cfg.Path, cfg.Host, cfg.Service); transport != nil {
		opts["transport"] = transport
	}

	return &ProxyNode{
		Protocol: "vmess",
		Name:     name,
		Server:   cfg.Add,
		Port:     cfg.Port,
		Options:  opts,
	}, nil
}

// extractJSONField 从 JSON 字符串中提取字段值（简化版）
func extractJSONField(jsonStr, field string) string {
	pattern := fmt.Sprintf(`"%s"\s*:\s*"([^"]*)"`, field)
	re := regexp.MustCompile(pattern)
	if matches := re.FindStringSubmatch(jsonStr); len(matches) > 1 {
		return matches[1]
	}

	// 尝试数字格式
	pattern = fmt.Sprintf(`"%s"\s*:\s*(\d+)`, field)
	re = regexp.MustCompile(pattern)
	if matches := re.FindStringSubmatch(jsonStr); len(matches) > 1 {
		return matches[1]
	}

	// 尝试布尔值
	pattern = fmt.Sprintf(`"%s"\s*:\s*(true|false)`, field)
	re = regexp.MustCompile(pattern)
	if matches := re.FindStringSubmatch(jsonStr); len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// parseServerPart 解析服务器部分
func parseServerPart(serverPart string) (server string, port int, fragment string, err error) {
	// 解析 fragment
	fragmentIndex := strings.Index(serverPart, "#")
	if fragmentIndex > 0 {
		fragment = serverPart[fragmentIndex+1:]
		if decoded, err := url.QueryUnescape(fragment); err == nil {
			fragment = decoded
		}
		serverPart = serverPart[:fragmentIndex]
	} else if fragmentIndex == 0 {
		return "", 0, "", fmt.Errorf("无效的服务器格式")
	}

	// 解析端口
	colonIndex := strings.LastIndex(serverPart, ":")
	if colonIndex <= 0 {
		return "", 0, "", fmt.Errorf("无效的服务器格式：缺少端口")
	}

	server = serverPart[:colonIndex]
	portStr := serverPart[colonIndex+1:]
	if fragmentIndex > 0 && strings.Contains(portStr, "#") {
		portStr = portStr[:strings.Index(portStr, "#")]
	}

	fmt.Sscanf(portStr, "%d", &port)

	if server == "" {
		return "", 0, "", fmt.Errorf("服务器地址为空")
	}

	return server, port, fragment, nil
}

// parseTrojanURL 解析单个 Trojan URL（保留以向后兼容）。
func parseTrojanURL(trojanURL string) (*TrojanNode, error) {
	trojanURL = strings.TrimSpace(trojanURL)
	if !strings.HasPrefix(trojanURL, "trojan://") {
		return nil, fmt.Errorf("无效的 Trojan 链接格式")
	}

	u, err := url.Parse(trojanURL)
	if err != nil {
		return nil, fmt.Errorf("解析 URL 失败: %w", err)
	}

	password := u.User.Username()
	if password == "" {
		return nil, fmt.Errorf("未找到密码")
	}

	server := u.Hostname()
	port := 443
	if portStr := u.Port(); portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	query := u.Query()
	name := query.Get("name")
	if name == "" {
		name = query.Get("hash")
	}
	if name == "" && u.Fragment != "" {
		raw := u.RawFragment
		if raw == "" {
			raw = u.Fragment
		}
		if decoded, err := url.QueryUnescape(raw); err == nil && decoded != "" {
			name = decoded
		} else {
			name = u.Fragment
		}
	}
	if name == "" {
		name = server
	}

	sni := query.Get("sni")
	if sni == "" {
		sni = server
	}

	return &TrojanNode{
		Name:     name,
		Server:   server,
		Port:     port,
		Password: password,
		SNI:      sni,
	}, nil
}

// parseClashYAML 解析 Clash YAML 格式的订阅内容（含 proxies: 段）
func parseClashYAML(content string) ([]*ProxyNode, error) {
	var cfg struct {
		Proxies []map[string]interface{} `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("YAML 解析失败: %w", err)
	}
	if len(cfg.Proxies) == 0 {
		return nil, fmt.Errorf("YAML 中未找到 proxies 字段")
	}

	nodes := make([]*ProxyNode, 0, len(cfg.Proxies))
	for _, p := range cfg.Proxies {
		name, _ := p["name"].(string)
		rawProxyType, _ := p["type"].(string)
		proxyType, ok := normalizeProxyProtocol(rawProxyType)
		server, _ := p["server"].(string)
		if !ok {
			if name == "" {
				name = server
			}
			log.Printf("[parse] 跳过不支持的 YAML 节点协议: name=%q type=%q server=%q", name, rawProxyType, server)
			continue
		}

		var port int
		switch v := p["port"].(type) {
		case int:
			port = v
		case float64:
			port = int(v)
		}

		if proxyType == "" || server == "" || port == 0 {
			continue
		}
		if name == "" {
			name = server
		}

		options := make(map[string]interface{})
		for k, v := range p {
			if k != "type" && k != "name" && k != "server" && k != "port" {
				options[k] = v
			}
		}

		nodes = append(nodes, &ProxyNode{
			Protocol: proxyType,
			Name:     name,
			Server:   server,
			Port:     port,
			Options:  options,
		})
	}
	return nodes, nil
}

func normalizeProxyProtocol(protocol string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "trojan", "vmess", "vless", "ss", "wireguard", "anytls", "hysteria2", "tuic":
		return strings.ToLower(strings.TrimSpace(protocol)), true
	case "hy2":
		return "hysteria2", true
	default:
		return "", false
	}
}

// ParseSubscription 解析订阅内容，支持多种协议
func ParseSubscription(content string) ([]*ProxyNode, error) {
	lowerContent := strings.ToLower(content)
	if strings.Contains(lowerContent, "<!doctype html>") || strings.Contains(lowerContent, "<html") {
		return nil, fmt.Errorf("订阅服务器返回了 HTML 页面（可能是访问被拒绝或错误），请检查订阅地址是否正确")
	}

	originalContent := strings.TrimSpace(content)
	if originalContent == "" {
		return nil, fmt.Errorf("订阅服务器返回了空内容")
	}

	// 优先尝试 Clash YAML 格式
	if strings.Contains(originalContent, "proxies:") {
		if nodes, err := parseClashYAML(originalContent); err == nil && len(nodes) > 0 {
			return nodes, nil
		}
	}

	candidates := []string{originalContent}
	// 如果没有直接找到协议链接，尝试 Base64 解码
	if !allProtocolPattern.MatchString(originalContent) {
		compact := strings.NewReplacer("\n", "", "\r", "", "\t", "", " ", "").Replace(originalContent)
		if decoded, ok := decodeSubscriptionBase64(compact); ok {
			candidates = append(candidates, decoded)
			// base64 解码结果也可能是 Clash YAML
			if strings.Contains(decoded, "proxies:") {
				if nodes, err := parseClashYAML(decoded); err == nil && len(nodes) > 0 {
					return nodes, nil
				}
			}
		}
		if decoded, ok := decodeSubscriptionBase64ByLine(originalContent); ok {
			candidates = append(candidates, decoded)
		}
	}

	// 收集所有协议的链接
	allURLs := []string{}
	for _, candidate := range candidates {
		matches := allProtocolPattern.FindAllString(candidate, -1)
		for _, m := range matches {
			link := strings.TrimSpace(m)
			if link != "" {
				allURLs = append(allURLs, link)
			}
		}
	}

	// 去重
	allURLs = DedupeStrings(allURLs)

	if len(allURLs) == 0 {
		lines := strings.Split(originalContent, "\n")
		return nil, fmt.Errorf("未找到有效的代理链接（共 %d 行内容，请确认订阅地址是否有效）", len(lines))
	}

	// 解析为 ProxyNode
	nodes := make([]*ProxyNode, 0, len(allURLs))
	parseErrors := []string{}

	for _, u := range allURLs {
		node, err := parseNodeURL(u)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("%s: %v", u, err))
			continue
		}
		nodes = append(nodes, node)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("未能成功解析任何节点（%d 个链接均失败，第一个错误: %s）",
			len(parseErrors), firstOrEmpty(parseErrors))
	}

	return nodes, nil
}

func firstOrEmpty(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

func decodeSubscriptionBase64(content string) (string, bool) {
	content = stripANSIEscape(content)
	content = sanitizeBase64Content(content)
	if content == "" {
		return "", false
	}

	tryDecode := func(enc *base64.Encoding, s string) (string, bool) {
		if decoded, err := enc.DecodeString(s); err == nil {
			return string(decoded), true
		}
		return "", false
	}

	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range encodings {
		if decoded, ok := tryDecode(enc, content); ok {
			return decoded, true
		}
	}

	if rem := len(content) % 4; rem != 0 {
		padded := content + strings.Repeat("=", 4-rem)
		for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding} {
			if decoded, ok := tryDecode(enc, padded); ok {
				return decoded, true
			}
		}
	}

	return "", false
}

func decodeSubscriptionBase64ByLine(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	decodedLines := make([]string, 0, len(lines))
	successCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if decoded, ok := decodeSubscriptionBase64(line); ok {
			decodedLines = append(decodedLines, decoded)
			successCount++
		}
	}

	if successCount == 0 {
		return "", false
	}

	return strings.Join(decodedLines, "\n"), true
}

func sanitizeBase64Content(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '+' || c == '/' || c == '=' || c == '-' || c == '_' {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func stripANSIEscape(s string) string {
	return ansiEscapePattern.ReplaceAllString(s, "")
}
