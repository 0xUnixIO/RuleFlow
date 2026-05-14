package app

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

type RuleSetRule struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	NoResolve bool   `json:"no_resolve,omitempty"`
}

func ParseRuleSet(content string, sourceFormat string) ([]RuleSetRule, error) {
	lines := strings.Split(content, "\n")
	rules := make([]RuleSetRule, 0, len(lines))

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "payload:") {
			line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "payload:"))
			if line == "" {
				continue
			}
		}

		switch sourceFormat {
		case "surge", "clash-classical":
			rule, ok := parseClassicalRule(line)
			if ok {
				rules = append(rules, rule)
			}
		case "clash-domain":
			if rule, ok := parseDomainListRule(line); ok {
				rules = append(rules, rule)
			}
		case "clash-ipcidr", "ip-list":
			if rule, ok := parseIPListRule(line); ok {
				rules = append(rules, rule)
			}
		case "domain-list":
			if rule, ok := parseDomainListRule(line); ok {
				rules = append(rules, rule)
			}
		default:
			return nil, fmt.Errorf("不支持的规则源格式: %s", sourceFormat)
		}
	}

	return dedupeRuleSetRules(rules), nil
}

func parseClassicalRule(line string) (RuleSetRule, bool) {
	parts := SplitCSVLine(line)
	if len(parts) < 2 {
		return RuleSetRule{}, false
	}
	ruleType := strings.ToUpper(strings.TrimSpace(parts[0]))
	value := strings.TrimSpace(parts[1])
	if value == "" {
		return RuleSetRule{}, false
	}

	rule := RuleSetRule{Value: value}
	switch ruleType {
	case "DOMAIN-SUFFIX":
		rule.Type = "domain_suffix"
	case "DOMAIN-KEYWORD":
		rule.Type = "domain_keyword"
	case "DOMAIN":
		rule.Type = "domain"
	case "IP-CIDR", "IP-CIDR6":
		rule.Type = "ip_cidr"
	default:
		return RuleSetRule{}, false
	}

	for _, part := range parts[2:] {
		if strings.EqualFold(strings.TrimSpace(part), "no-resolve") {
			rule.NoResolve = true
		}
	}

	return rule, true
}

func parseDomainListRule(line string) (RuleSetRule, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return RuleSetRule{}, false
	}
	if strings.HasPrefix(line, "+.") {
		return RuleSetRule{Type: "domain_suffix", Value: strings.TrimPrefix(line, "+.")}, true
	}
	if strings.HasPrefix(line, "domain:") {
		return RuleSetRule{Type: "domain", Value: strings.TrimSpace(strings.TrimPrefix(line, "domain:"))}, true
	}
	if strings.HasPrefix(line, "full:") {
		return RuleSetRule{Type: "domain", Value: strings.TrimSpace(strings.TrimPrefix(line, "full:"))}, true
	}
	return RuleSetRule{Type: "domain_suffix", Value: line}, true
}

func parseIPListRule(line string) (RuleSetRule, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return RuleSetRule{}, false
	}
	if _, _, err := net.ParseCIDR(line); err == nil {
		return RuleSetRule{Type: "ip_cidr", Value: line, NoResolve: true}, true
	}
	if ip := net.ParseIP(line); ip != nil {
		if strings.Contains(line, ":") {
			return RuleSetRule{Type: "ip_cidr", Value: line + "/128", NoResolve: true}, true
		}
		return RuleSetRule{Type: "ip_cidr", Value: line + "/32", NoResolve: true}, true
	}
	return RuleSetRule{}, false
}

func ExportRuleSet(rules []RuleSetRule, target string) (string, error) {
	switch target {
	case "surge", "clash-classical":
		lines := make([]string, 0, len(rules))
		for _, rule := range rules {
			switch rule.Type {
			case "domain_suffix":
				lines = append(lines, "DOMAIN-SUFFIX,"+rule.Value)
			case "domain_keyword":
				lines = append(lines, "DOMAIN-KEYWORD,"+rule.Value)
			case "domain":
				lines = append(lines, "DOMAIN,"+rule.Value)
			case "ip_cidr":
				line := "IP-CIDR," + rule.Value
				if rule.NoResolve {
					line += ",no-resolve"
				}
				lines = append(lines, line)
			}
		}
		return strings.Join(lines, "\n"), nil
	case "clash-domain":
		lines := make([]string, 0, len(rules))
		for _, rule := range rules {
			switch rule.Type {
			case "domain_suffix":
				lines = append(lines, "+."+rule.Value)
			case "domain":
				lines = append(lines, rule.Value)
			}
		}
		return strings.Join(DedupeStrings(lines), "\n"), nil
	case "clash-ipcidr":
		lines := make([]string, 0, len(rules))
		for _, rule := range rules {
			if rule.Type == "ip_cidr" {
				lines = append(lines, rule.Value)
			}
		}
		return strings.Join(DedupeStrings(lines), "\n"), nil
	case "sing-box":
		payload := map[string]interface{}{
			"version": 1,
			"rules":   buildSingBoxRuleSetSource(rules),
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", fmt.Errorf("生成 sing-box 规则集失败: %w", err)
		}
		return string(data), nil
	default:
		return "", fmt.Errorf("不支持的导出目标: %s", target)
	}
}

func buildSingBoxRuleSetSource(rules []RuleSetRule) []map[string]interface{} {
	grouped := map[string][]string{}
	for _, rule := range rules {
		grouped[rule.Type] = append(grouped[rule.Type], rule.Value)
	}

	out := make([]map[string]interface{}, 0, len(grouped))
	order := []string{"domain_suffix", "domain_keyword", "domain", "ip_cidr"}
	for _, key := range order {
		values := DedupeStrings(grouped[key])
		if len(values) == 0 {
			continue
		}
		out = append(out, map[string]interface{}{
			key: values,
		})
	}
	return out
}

func dedupeRuleSetRules(rules []RuleSetRule) []RuleSetRule {
	seen := make(map[string]struct{}, len(rules))
	out := make([]RuleSetRule, 0, len(rules))
	for _, rule := range rules {
		key := fmt.Sprintf("%s|%s", rule.Type, rule.Value)
		if rule.Type == "ip_cidr" {
			key = fmt.Sprintf("%s|%s|%t", rule.Type, rule.Value, rule.NoResolve)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, rule)
	}
	return out
}
