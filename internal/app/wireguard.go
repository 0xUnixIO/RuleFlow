package app

import (
	"fmt"
	"net/netip"
	"strings"
)

type wireGuardPeerConfig struct {
	Server              string
	Port                int
	PublicKey           string
	PreSharedKey        string
	AllowedIPs          []string
	Reserved            []int
	PersistentKeepalive int
}

type wireGuardConfig struct {
	LocalAddresses   []string
	PrivateKey       string
	DNS              []string
	MTU              int
	RemoteDNSResolve bool
	UDP              bool
	Peers            []wireGuardPeerConfig
}

func extractWireGuardConfig(node *ProxyNode) wireGuardConfig {
	opts := node.Options
	if opts == nil {
		opts = map[string]interface{}{}
	}

	cfg := wireGuardConfig{
		LocalAddresses: normalizeWireGuardPrefixes(DedupeStrings(appendStringSlice(
			flexibleStringSliceOption(opts, "local-address"),
			stringValueOption(opts, "ip", "self-ip"),
			stringValueOption(opts, "ipv6", "self-ip-v6"),
		))),
		PrivateKey:       stringValueOption(opts, "private-key", "privateKey"),
		DNS:              flexibleStringSliceOption(opts, "dns", "dns-server"),
		MTU:              intValueOption(opts, "mtu"),
		RemoteDNSResolve: boolValueOption(opts, "remote-dns-resolve", "remoteDNSResolve"),
		UDP:              true,
	}
	if udp, ok := boolOption(opts, "udp"); ok {
		cfg.UDP = udp
	}

	if rawPeers, ok := opts["peers"].([]interface{}); ok {
		for _, item := range rawPeers {
			peerMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if peer := extractWireGuardPeerFromMap(peerMap, node.Server, node.Port); peer.PublicKey != "" {
				cfg.Peers = append(cfg.Peers, peer)
			}
		}
	}

	if len(cfg.Peers) == 0 {
		peer := extractWireGuardPeerFromMap(opts, node.Server, node.Port)
		if peer.PublicKey != "" {
			cfg.Peers = append(cfg.Peers, peer)
		}
	}

	return cfg
}

func extractWireGuardPeerFromMap(raw map[string]interface{}, fallbackServer string, fallbackPort int) wireGuardPeerConfig {
	peer := wireGuardPeerConfig{
		Server:              stringValueOption(raw, "server", "host"),
		Port:                intValueOption(raw, "port"),
		PublicKey:           stringValueOption(raw, "public-key", "publicKey"),
		PreSharedKey:        stringValueOption(raw, "pre-shared-key", "preSharedKey"),
		AllowedIPs:          flexibleStringSliceOption(raw, "allowed-ips", "allowedIPs"),
		Reserved:            flexibleIntSliceOption(raw, "reserved"),
		PersistentKeepalive: intValueOption(raw, "persistent-keepalive", "persistentKeepalive"),
	}
	if peer.Server == "" {
		peer.Server = fallbackServer
	}
	if peer.Port == 0 {
		peer.Port = fallbackPort
	}
	if len(peer.Reserved) == 0 {
		peer.Reserved = flexibleIntSliceOption(raw, "client-id", "clientId")
	}
	peer.AllowedIPs = normalizeWireGuardPrefixes(peer.AllowedIPs)
	return peer
}

func flexibleStringSliceOption(opts map[string]interface{}, keys ...string) []string {
	for _, key := range keys {
		value, exists := opts[key]
		if !exists || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			parts := strings.FieldsFunc(typed, func(r rune) bool {
				return r == ',' || r == '\n'
			})
			out := make([]string, 0, len(parts))
			for _, part := range parts {
				part = strings.TrimSpace(strings.Trim(part, `"`))
				if part != "" {
					out = append(out, part)
				}
			}
			if len(out) > 0 {
				return out
			}
		case []string:
			return append([]string(nil), typed...)
		case []interface{}:
			out := make([]string, 0, len(typed))
			for _, item := range typed {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					out = append(out, strings.TrimSpace(s))
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func flexibleIntSliceOption(opts map[string]interface{}, keys ...string) []int {
	for _, key := range keys {
		value, exists := opts[key]
		if !exists || value == nil {
			continue
		}
		switch typed := value.(type) {
		case []int:
			return append([]int(nil), typed...)
		case []interface{}:
			out := make([]int, 0, len(typed))
			for _, item := range typed {
				switch v := item.(type) {
				case int:
					out = append(out, v)
				case int64:
					out = append(out, int(v))
				case float64:
					out = append(out, int(v))
				}
			}
			if len(out) > 0 {
				return out
			}
		case string:
			parts := strings.FieldsFunc(typed, func(r rune) bool {
				return r == ',' || r == '/' || r == ' '
			})
			out := make([]int, 0, len(parts))
			for _, part := range parts {
				var value int
				if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &value); err == nil {
					out = append(out, value)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func stringValueOption(opts map[string]interface{}, keys ...string) string {
	value, _ := stringOption(opts, keys...)
	return value
}

func intValueOption(opts map[string]interface{}, keys ...string) int {
	value, _ := intOption(opts, keys...)
	return value
}

func boolValueOption(opts map[string]interface{}, keys ...string) bool {
	value, _ := boolOption(opts, keys...)
	return value
}

func appendStringSlice(base []string, extra ...string) []string {
	out := append([]string(nil), base...)
	for _, item := range extra {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func normalizeWireGuardPrefixes(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.Contains(value, "/") {
			out = append(out, value)
			continue
		}
		if addr, err := netip.ParseAddr(value); err == nil {
			bits := 32
			if addr.Is6() {
				bits = 128
			}
			out = append(out, fmt.Sprintf("%s/%d", addr.String(), bits))
			continue
		}
		out = append(out, value)
	}
	return out
}
