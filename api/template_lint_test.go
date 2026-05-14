package api

import (
	"context"
	"strings"
	"testing"

	"github.com/ablate-ai/RuleFlow/internal/app"
)

func TestExtractSurgeRules(t *testing.T) {
	content := `
[General]
loglevel = notify

[Rule]
DOMAIN,example.com,DIRECT
FINAL,Proxy

[Host]
`

	rules := extractSurgeRules(content)
	if len(rules) != 2 {
		t.Fatalf("期望提取 2 条规则，实际为 %d: %#v", len(rules), rules)
	}
	if rules[0] != "DOMAIN,example.com,DIRECT" || rules[1] != "FINAL,Proxy" {
		t.Fatalf("提取的规则不符合预期: %#v", rules)
	}
}

func TestExtractYAMLRules(t *testing.T) {
	content := `
proxies: []
rules:
  - DOMAIN,example.com,DIRECT
  - MATCH,Proxy
`

	rules, err := extractYAMLRules(content)
	if err != nil {
		t.Fatalf("extractYAMLRules() 返回错误: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("期望提取 2 条规则，实际为 %d: %#v", len(rules), rules)
	}
}

func TestParseRulesToLintRules(t *testing.T) {
	got := parseRulesToLintRules([]string{
		"DOMAIN-SUFFIX,example.com,Proxy",
		"MATCH,DIRECT",
	})

	if len(got) != 2 {
		t.Fatalf("期望解析 2 条规则，实际为 %d", len(got))
	}
	if got[0].Type != "DOMAIN-SUFFIX" || got[0].Payload != "example.com" || got[0].Policy != "Proxy" {
		t.Fatalf("第一条解析结果不符合预期: %#v", got[0])
	}
	if got[1].Type != "MATCH" || got[1].Policy != "DIRECT" {
		t.Fatalf("第二条解析结果不符合预期: %#v", got[1])
	}
}

func TestExpandRuleSetRules(t *testing.T) {
	ref := ruleSetReference{
		ParentIndex: 3,
		Policy:      "Proxy",
		Source:      "/rulesets/demo?target=surge",
	}
	rules := []app.RuleSetRule{
		{Type: "domain_suffix", Value: "example.com"},
		{Type: "ip_cidr", Value: "1.1.1.0/24", NoResolve: true},
	}

	got := expandRuleSetRules(ref, rules)
	if len(got) != 2 {
		t.Fatalf("期望展开 2 条规则，实际为 %d: %#v", len(got), got)
	}
	if got[0].Line != 3 || got[0].Policy != "Proxy" || got[0].Type != "DOMAIN-SUFFIX" {
		t.Fatalf("第一条展开结果不符合预期: %#v", got[0])
	}
	if got[1].Type != "IP-CIDR" || !got[1].NoResolve {
		t.Fatalf("第二条展开结果不符合预期: %#v", got[1])
	}
}

func TestExpandRuleSetReferencesNoRefs(t *testing.T) {
	got, warnings := expandRuleSetReferences(context.Background(), &Handlers{}, nil)
	if len(got) != 0 || len(warnings) != 0 {
		t.Fatalf("空引用不应产生结果，got=%#v warnings=%v", got, warnings)
	}
}

func TestCheckTerminalRules(t *testing.T) {
	warnings := checkTerminalRules([]lintRule{
		{Type: "DOMAIN", Payload: "example.com", Policy: "DIRECT", Line: 1},
		{Type: "MATCH", Policy: "Proxy", Line: 2},
		{Type: "DOMAIN", Payload: "google.com", Policy: "DIRECT", Line: 3},
	})

	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "不会生效") {
		t.Fatalf("期望检测到兜底规则后还有规则，实际 warnings=%v", warnings)
	}
}

func TestCheckDuplicateRules(t *testing.T) {
	warnings := checkDuplicateRules([]lintRule{
		{Type: "DOMAIN", Payload: "example.com", Policy: "DIRECT", Line: 1, Source: "template"},
		{Type: "DOMAIN", Payload: "example.com", Policy: "DIRECT", Line: 2, Source: "template"},
	})

	if joined := strings.Join(warnings, "\n"); !strings.Contains(joined, "重复") {
		t.Fatalf("期望检测到重复规则，实际 warnings=%v", warnings)
	}
}

func TestCheckDuplicateRulesIncludesExpandedSources(t *testing.T) {
	warnings := checkDuplicateRules([]lintRule{
		{Type: "DOMAIN-SUFFIX", Payload: "anthropic.com", Policy: "🤖 AI", Line: 14, Source: "/rulesets/ai_a?target=surge"},
		{Type: "DOMAIN-SUFFIX", Payload: "anthropic.com", Policy: "🤖 AI", Line: 26, Source: "/rulesets/ai_b?target=surge"},
	})

	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "/rulesets/ai_a?target=surge") || !strings.Contains(joined, "/rulesets/ai_b?target=surge") {
		t.Fatalf("期望重复规则告警包含来源，实际 warnings=%v", warnings)
	}
}

func TestCheckRuleOrder(t *testing.T) {
	warnings := checkRuleOrder([]lintRule{
		{Type: "IP-CIDR", Payload: "1.1.1.0/24", Policy: "Proxy", Line: 1},
		{Type: "DOMAIN", Payload: "google.com", Policy: "Proxy", Line: 2},
	})

	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "第 2 条规则应放到第 1 条之前") || !strings.Contains(joined, "IP-CIDR,1.1.1.0/24 -> Proxy") {
		t.Fatalf("期望检测到顺序风险，实际 warnings=%v", warnings)
	}
}

func TestCheckRuleShadowingByDomainSuffix(t *testing.T) {
	warnings := checkRuleShadowing([]lintRule{
		{Type: "DOMAIN-SUFFIX", Payload: "google.com", Policy: "Proxy", Line: 1, Source: "/rulesets/proxy?target=surge"},
		{Type: "DOMAIN", Payload: "mail.google.com", Policy: "DIRECT", Line: 2, Source: "/rulesets/direct?target=surge"},
	})

	if joined := strings.Join(warnings, "\n"); !strings.Contains(joined, "覆盖") || !strings.Contains(joined, "/rulesets/proxy?target=surge") {
		t.Fatalf("期望检测到域名覆盖，实际 warnings=%v", warnings)
	}
}

func TestCheckRuleShadowingByCIDR(t *testing.T) {
	warnings := checkRuleShadowing([]lintRule{
		{Type: "IP-CIDR", Payload: "10.0.0.0/8", Policy: "DIRECT", Line: 1},
		{Type: "IP-CIDR", Payload: "10.0.0.0/16", Policy: "Proxy", Line: 2},
	})

	if joined := strings.Join(warnings, "\n"); !strings.Contains(joined, "覆盖") {
		t.Fatalf("期望检测到 CIDR 覆盖，实际 warnings=%v", warnings)
	}
}

func TestRunLintChecksMissingTerminalRule(t *testing.T) {
	warnings := runLintChecks([]lintRule{
		{Type: "DOMAIN", Payload: "example.com", Policy: "DIRECT", Line: 1},
	}, nil)

	if joined := strings.Join(warnings, "\n"); !strings.Contains(joined, "缺少兜底规则") {
		t.Fatalf("期望检测到缺少兜底规则，实际 warnings=%v", warnings)
	}
}

func TestRunLintChecksDoesNotReportOrderForExpandedRulesAfterTerminal(t *testing.T) {
	warnings := runLintChecks([]lintRule{
		{Type: "DOMAIN-SUFFIX", Payload: "example.com", Policy: "Proxy", Line: 1},
		{Type: "MATCH", Policy: "DIRECT", Line: 36},
	}, []lintRule{
		{Type: "DOMAIN", Payload: "later.example.com", Policy: "REJECT", Line: 2, Source: "/rulesets/demo?target=surge"},
	})

	joined := strings.Join(warnings, "\n")
	if strings.Contains(joined, "应放到第 36 条之前") {
		t.Fatalf("展开规则不应因为顶层兜底规则产生顺序风险，实际 warnings=%v", warnings)
	}
}

func TestCheckExpandedRuleSetOrderAggregatesByRuleSet(t *testing.T) {
	warnings := checkExpandedRuleSetOrder([]lintRule{
		{Type: "IP-CIDR", Payload: "1.1.1.0/24", Policy: "Proxy", Line: 6, Source: "/rulesets/ip?target=surge"},
		{Type: "IP-CIDR", Payload: "8.8.8.0/24", Policy: "Proxy", Line: 6, Source: "/rulesets/ip?target=surge"},
		{Type: "DOMAIN-SUFFIX", Payload: "gpt.com", Policy: "🤖 AI", Line: 10, Source: "/rulesets/ai?target=surge"},
		{Type: "DOMAIN", Payload: "openai.com", Policy: "🤖 AI", Line: 10, Source: "/rulesets/ai?target=surge"},
	})

	if len(warnings) != 1 {
		t.Fatalf("期望按 ruleset 聚合为 1 条告警，实际为 %d: %v", len(warnings), warnings)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "第 10 条 ruleset（/rulesets/ai?target=surge）应放到第 6 条 ruleset（/rulesets/ip?target=surge）之前") {
		t.Fatalf("期望包含 ruleset 聚合顺序告警，实际 warnings=%v", warnings)
	}
	if !strings.Contains(joined, "有 2 条展开规则存在顺序风险") {
		t.Fatalf("期望包含聚合数量，实际 warnings=%v", warnings)
	}
}
