package app

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// SerializeNodeURL 将存储的节点数据反向生成分享链接
func SerializeNodeURL(name, protocol, server string, port int, config map[string]interface{}) (string, error) {
	switch protocol {
	case "ss":
		return serializeSS(name, server, port, config)
	case "trojan":
		return serializeTrojan(name, server, port, config)
	case "vless":
		return serializeVLESS(name, server, port, config)
	case "vmess":
		return serializeVMess(name, server, port, config)
	case "hysteria2":
		return serializeHysteria2(name, server, port, config)
	case "tuic":
		return serializeTUIC(name, server, port, config)
	case "anytls":
		return serializeAnyTLS(name, server, port, config)
	default:
		return "", fmt.Errorf("不支持序列化协议: %s", protocol)
	}
}

func serializeSS(name, server string, port int, config map[string]interface{}) (string, error) {
	cipher := configStr(config, "cipher")
	password := configStr(config, "password")
	if cipher == "" || password == "" {
		return "", fmt.Errorf("ss 节点缺少 cipher 或 password")
	}
	userInfo := base64.StdEncoding.EncodeToString([]byte(cipher + ":" + password))
	return fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, server, port, url.PathEscape(name)), nil
}

func serializeTrojan(name, server string, port int, config map[string]interface{}) (string, error) {
	password := configStr(config, "password")
	if password == "" {
		return "", fmt.Errorf("trojan 节点缺少 password")
	}
	q := url.Values{}
	applyTLSParams(q, config)
	applyTransportParams(q, config)
	raw := fmt.Sprintf("trojan://%s@%s:%d", url.PathEscape(password), server, port)
	if encoded := q.Encode(); encoded != "" {
		raw += "?" + encoded
	}
	return raw + "#" + url.PathEscape(name), nil
}

func serializeVLESS(name, server string, port int, config map[string]interface{}) (string, error) {
	uuid := configStr(config, "uuid")
	if uuid == "" {
		return "", fmt.Errorf("vless 节点缺少 uuid")
	}
	q := url.Values{}
	if flow := configStr(config, "flow"); flow != "" {
		q.Set("flow", flow)
	}
	applyTLSParams(q, config)
	applyTransportParams(q, config)
	raw := fmt.Sprintf("vless://%s@%s:%d", uuid, server, port)
	if encoded := q.Encode(); encoded != "" {
		raw += "?" + encoded
	}
	return raw + "#" + url.PathEscape(name), nil
}

func serializeVMess(name, server string, port int, config map[string]interface{}) (string, error) {
	uuid := configStr(config, "uuid")
	if uuid == "" {
		return "", fmt.Errorf("vmess 节点缺少 uuid")
	}
	alterID := 0
	if v, ok := config["alterID"]; ok {
		switch n := v.(type) {
		case float64:
			alterID = int(n)
		case int:
			alterID = n
		}
	}

	tlsStr := ""
	sni := ""
	if tls := configMap(config, "tls"); tls != nil {
		if enabled, _ := tls["enabled"].(bool); enabled {
			tlsStr = "tls"
		}
		sni = configStr(tls, "server_name")
	}

	network := "tcp"
	path := ""
	host := ""
	serviceName := ""
	if transport := configMap(config, "transport"); transport != nil {
		if t := configStr(transport, "type"); t != "" {
			network = t
		}
		path = configStr(transport, "path")
		host = configStr(transport, "host")
		serviceName = configStr(transport, "service_name")
	}

	type vmessJSON struct {
		V           string `json:"v"`
		PS          string `json:"ps"`
		Add         string `json:"add"`
		Port        int    `json:"port"`
		ID          string `json:"id"`
		Aid         int    `json:"aid"`
		Net         string `json:"net"`
		Type        string `json:"type"`
		Host        string `json:"host"`
		Path        string `json:"path"`
		TLS         string `json:"tls"`
		SNI         string `json:"sni"`
		ServiceName string `json:"serviceName,omitempty"`
	}

	obj := vmessJSON{
		V:           "2",
		PS:          name,
		Add:         server,
		Port:        port,
		ID:          uuid,
		Aid:         alterID,
		Net:         network,
		Type:        "none",
		Host:        host,
		Path:        path,
		TLS:         tlsStr,
		SNI:         sni,
		ServiceName: serviceName,
	}

	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("序列化 vmess JSON 失败: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(jsonBytes)
	return "vmess://" + encoded, nil
}

func serializeHysteria2(name, server string, port int, config map[string]interface{}) (string, error) {
	password := configStr(config, "password")
	if password == "" {
		return "", fmt.Errorf("hysteria2 节点缺少 password")
	}
	q := url.Values{}
	if tls := configMap(config, "tls"); tls != nil {
		if sni := configStr(tls, "server_name"); sni != "" {
			q.Set("sni", sni)
		}
		if insecure, _ := tls["insecure"].(bool); insecure {
			q.Set("insecure", "1")
		}
	}
	raw := fmt.Sprintf("hysteria2://%s@%s:%d", url.PathEscape(password), server, port)
	if encoded := q.Encode(); encoded != "" {
		raw += "?" + encoded
	}
	return raw + "#" + url.PathEscape(name), nil
}

func serializeTUIC(name, server string, port int, config map[string]interface{}) (string, error) {
	uuid := configStr(config, "uuid")
	password := configStr(config, "password")
	if uuid == "" || password == "" {
		return "", fmt.Errorf("tuic 节点缺少 uuid 或 password")
	}
	q := url.Values{}
	if tls := configMap(config, "tls"); tls != nil {
		if sni := configStr(tls, "server_name"); sni != "" {
			q.Set("sni", sni)
		}
		if insecure, _ := tls["insecure"].(bool); insecure {
			q.Set("insecure", "1")
		}
	}
	raw := fmt.Sprintf("tuic://%s:%s@%s:%d", uuid, password, server, port)
	if encoded := q.Encode(); encoded != "" {
		raw += "?" + encoded
	}
	return raw + "#" + url.PathEscape(name), nil
}

func serializeAnyTLS(name, server string, port int, config map[string]interface{}) (string, error) {
	password := configStr(config, "password")
	if password == "" {
		return "", fmt.Errorf("anytls 节点缺少 password")
	}
	q := url.Values{}
	if tls := configMap(config, "tls"); tls != nil {
		if sni := configStr(tls, "server_name"); sni != "" {
			q.Set("sni", sni)
		}
		if insecure, _ := tls["insecure"].(bool); insecure {
			q.Set("insecure", "1")
		}
	}
	raw := fmt.Sprintf("anytls://%s@%s:%d", url.PathEscape(password), server, port)
	if encoded := q.Encode(); encoded != "" {
		raw += "?" + encoded
	}
	return raw + "#" + url.PathEscape(name), nil
}

// applyTLSParams 将 TLS 配置写入 query 参数
func applyTLSParams(q url.Values, config map[string]interface{}) {
	tls := configMap(config, "tls")
	if tls == nil {
		return
	}
	enabled, _ := tls["enabled"].(bool)
	if !enabled {
		return
	}

	// 检查是否为 reality
	if reality := configMap(tls, "reality"); reality != nil {
		if realityEnabled, _ := reality["enabled"].(bool); realityEnabled {
			q.Set("security", "reality")
			if pk := configStr(reality, "public_key"); pk != "" {
				q.Set("pbk", pk)
			}
			if sid := configStr(reality, "short_id"); sid != "" {
				q.Set("sid", sid)
			}
		}
	} else {
		q.Set("security", "tls")
	}

	if sni := configStr(tls, "server_name"); sni != "" {
		q.Set("sni", sni)
	}
	if insecure, _ := tls["insecure"].(bool); insecure {
		q.Set("allowInsecure", "1")
	}
	if alpnList, ok := tls["alpn"].([]interface{}); ok && len(alpnList) > 0 {
		parts := make([]string, 0, len(alpnList))
		for _, a := range alpnList {
			if s, ok := a.(string); ok {
				parts = append(parts, s)
			}
		}
		if len(parts) > 0 {
			q.Set("alpn", strings.Join(parts, ","))
		}
	}
	if utls := configMap(tls, "utls"); utls != nil {
		if fp := configStr(utls, "fingerprint"); fp != "" {
			q.Set("fp", fp)
		}
	}
}

// applyTransportParams 将 transport 配置写入 query 参数
func applyTransportParams(q url.Values, config map[string]interface{}) {
	transport := configMap(config, "transport")
	if transport == nil {
		return
	}
	transportType := configStr(transport, "type")
	if transportType == "" || transportType == "tcp" {
		return
	}
	q.Set("type", transportType)
	if path := configStr(transport, "path"); path != "" {
		q.Set("path", path)
	}
	if host := configStr(transport, "host"); host != "" {
		q.Set("host", host)
	}
	if sn := configStr(transport, "service_name"); sn != "" {
		q.Set("serviceName", sn)
	}
}

// configStr 从 map 中安全提取字符串
func configStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// configMap 从 map 中安全提取子 map
func configMap(m map[string]interface{}, key string) map[string]interface{} {
	if m == nil {
		return nil
	}
	sub, _ := m[key].(map[string]interface{})
	return sub
}
