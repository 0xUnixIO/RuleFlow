package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/ablate-ai/RuleFlow/internal/app"
	"gopkg.in/yaml.v3"
)

const (
	remoteRuleSetCacheKeyPrefix = "ruleflow:template-lint:ruleset:"
)

var (
	remoteRuleSetCacheTTL   = 5 * time.Minute
	remoteRuleSetFailureTTL = 30 * time.Second
)

type remoteRuleSetCachePayload struct {
	Rules []app.RuleSetRule `json:"rules,omitempty"`
	Error string            `json:"error,omitempty"`
}

type templateValidationResult struct {
	Message        string         `json:"message"`
	Warnings       []string       `json:"warnings,omitempty"`
	WarningSummary map[string]int `json:"warning_summary,omitempty"`
}

type lintRule struct {
	Type      string
	Payload   string
	Policy    string
	Line      int
	Source    string
	NoResolve bool
}

type ruleSetReference struct {
	ParentIndex int
	Kind        string
	Policy      string
	Source      string
	Format      string
	Raw         string
}

type yamlRuleProvider struct {
	URL    string
	Format string
}

type lintRuleSetOrderIssue struct {
	blockedLine   int
	blockedSource string
	blockerLine   int
	blockerSource string
	count         int
	sample        lintRule
}

func lintGeneratedTemplate(ctx context.Context, h *Handlers, target, configContent string) []string {
	rules, refs, warnings := collectLintInputs(target, configContent)
	expanded := []lintRule(nil)
	if len(refs) > 0 {
		var expandWarnings []string
		expanded, expandWarnings = expandRuleSetReferences(ctx, h, refs)
		warnings = append(warnings, expandWarnings...)
	}
	return append(warnings, runLintChecks(rules, expanded)...)
}

func summarizeLintWarnings(warnings []string) map[string]int {
	if len(warnings) == 0 {
		return nil
	}

	summary := map[string]int{
		"terminal":  0,
		"duplicate": 0,
		"order":     0,
		"shadowing": 0,
		"ruleset":   0,
		"other":     0,
	}

	for _, warning := range warnings {
		switch {
		case strings.Contains(warning, "兜底规则") || strings.Contains(warning, "缺少兜底规则"):
			summary["terminal"]++
		case strings.Contains(warning, "重复规则") || strings.Contains(warning, "重复："):
			summary["duplicate"]++
		case strings.Contains(warning, "顺序风险"):
			summary["order"]++
		case strings.Contains(warning, "覆盖"):
			summary["shadowing"]++
		case strings.Contains(warning, "ruleset 无法展开"):
			summary["ruleset"]++
		default:
			summary["other"]++
		}
	}

	for key, value := range summary {
		if value == 0 {
			delete(summary, key)
		}
	}

	return summary
}

func collectLintInputs(target, configContent string) ([]lintRule, []ruleSetReference, []string) {
	switch target {
	case "clash-mihomo", "stash":
		return collectYAMLLintInputs(configContent)
	case "surge":
		return collectSurgeLintInputs(configContent)
	default:
		return nil, nil, nil
	}
}

func collectYAMLLintInputs(configContent string) ([]lintRule, []ruleSetReference, []string) {
	rules, err := extractYAMLRules(configContent)
	if err != nil {
		return nil, nil, []string{fmt.Sprintf("无法分析 rules 顺序: %v", err)}
	}

	providers, err := extractYAMLRuleProviders(configContent)
	if err != nil {
		return parseRulesToLintRules(rules), nil, []string{fmt.Sprintf("无法分析 rule-providers: %v", err)}
	}

	return parseRulesToLintRules(rules), buildYAMLRuleSetRefs(rules, providers), nil
}

func collectSurgeLintInputs(configContent string) ([]lintRule, []ruleSetReference, []string) {
	rules := extractSurgeRules(configContent)
	return parseRulesToLintRules(rules), buildSurgeRuleSetRefs(rules), nil
}

func runLintChecks(templateRules, expandedRules []lintRule) []string {
	if len(templateRules) == 0 {
		return []string{"未检测到任何 rules，无法检查规则顺序与重复项"}
	}

	combinedRules := make([]lintRule, 0, len(templateRules)+len(expandedRules))
	combinedRules = append(combinedRules, templateRules...)
	combinedRules = append(combinedRules, expandedRules...)

	warnings := make([]string, 0)
	warnings = append(warnings, checkTerminalRules(templateRules)...)
	warnings = append(warnings, checkRuleOrder(templateRules)...)
	warnings = append(warnings, checkExpandedRuleSetOrder(expandedRules)...)
	warnings = append(warnings, checkDuplicateRules(combinedRules)...)
	warnings = append(warnings, checkRuleShadowing(combinedRules)...)
	return app.DedupeTrimmedNonEmptyStrings(warnings)
}

func expandRuleSetReferences(ctx context.Context, h *Handlers, refs []ruleSetReference) ([]lintRule, []string) {
	if len(refs) == 0 {
		return nil, nil
	}
	if h == nil {
		return nil, []string{"规则展开检查不可用：处理器未初始化"}
	}

	expanded := make([]lintRule, 0)
	warnings := make([]string, 0)

	for _, ref := range refs {
		rules, err := loadRuleSetRules(ctx, h, ref)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("第 %d 条规则引用的 ruleset 无法展开：%s（%v）", ref.ParentIndex, ref.Source, err))
			continue
		}
		expanded = append(expanded, expandRuleSetRules(ref, rules)...)
	}

	return expanded, warnings
}

func loadRuleSetRules(ctx context.Context, h *Handlers, ref ruleSetReference) ([]app.RuleSetRule, error) {
	if strings.HasPrefix(ref.Source, "/rulesets/") {
		return loadLocalRuleSetRules(ctx, h, ref.Source)
	}
	if strings.HasPrefix(ref.Source, "http://") || strings.HasPrefix(ref.Source, "https://") {
		return loadRemoteRuleSetRules(ctx, h, ref)
	}
	return nil, fmt.Errorf("暂不支持的 ruleset 来源")
}

func loadLocalRuleSetRules(ctx context.Context, h *Handlers, source string) ([]app.RuleSetRule, error) {
	if h.ruleSourceService == nil {
		return nil, fmt.Errorf("规则源服务不可用")
	}

	u, err := url.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("ruleset 路径无效: %w", err)
	}

	name := strings.TrimPrefix(u.Path, "/rulesets/")
	name, err = url.PathUnescape(name)
	if err != nil {
		return nil, fmt.Errorf("ruleset 名称解码失败: %w", err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("ruleset 名称为空")
	}

	sourceObj, err := h.ruleSourceService.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(sourceObj.ParsedRules) == 0 {
		return nil, fmt.Errorf("规则源尚未同步")
	}

	var rules []app.RuleSetRule
	if err := json.Unmarshal(sourceObj.ParsedRules, &rules); err != nil {
		return nil, fmt.Errorf("读取已同步规则失败: %w", err)
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("规则源没有可用规则")
	}
	return rules, nil
}

func loadRemoteRuleSetRules(ctx context.Context, h *Handlers, ref ruleSetReference) ([]app.RuleSetRule, error) {
	format := strings.TrimSpace(ref.Format)
	if format == "" {
		return nil, fmt.Errorf("无法判断规则集格式")
	}

	if cached, err := loadRemoteRuleSetRulesFromCache(ctx, h, ref); err == nil {
		return cached, nil
	}

	content, _, err := app.FetchSubscriptionContent(ctx, ref.Source)
	if err != nil {
		storeRemoteRuleSetCache(ctx, h, ref, remoteRuleSetCachePayload{
			Error: err.Error(),
		}, remoteRuleSetFailureTTL)
		return nil, err
	}

	rules, err := app.ParseRuleSet(content, format)
	if err != nil {
		storeRemoteRuleSetCache(ctx, h, ref, remoteRuleSetCachePayload{
			Error: err.Error(),
		}, remoteRuleSetFailureTTL)
		return nil, err
	}
	if len(rules) == 0 {
		storeRemoteRuleSetCache(ctx, h, ref, remoteRuleSetCachePayload{
			Error: "规则集为空",
		}, remoteRuleSetFailureTTL)
		return nil, fmt.Errorf("规则集为空")
	}

	storeRemoteRuleSetCache(ctx, h, ref, remoteRuleSetCachePayload{
		Rules: rules,
	}, remoteRuleSetCacheTTL)
	return rules, nil
}

func expandRuleSetRules(ref ruleSetReference, rules []app.RuleSetRule) []lintRule {
	expanded := make([]lintRule, 0, len(rules))
	for _, rule := range rules {
		item, ok := lintRuleFromRuleSet(ref, rule)
		if ok {
			expanded = append(expanded, item)
		}
	}
	return expanded
}

func lintRuleFromRuleSet(ref ruleSetReference, rule app.RuleSetRule) (lintRule, bool) {
	ruleType := ""
	switch rule.Type {
	case "domain_suffix":
		ruleType = "DOMAIN-SUFFIX"
	case "domain_keyword":
		ruleType = "DOMAIN-KEYWORD"
	case "domain":
		ruleType = "DOMAIN"
	case "ip_cidr":
		ruleType = "IP-CIDR"
	default:
		return lintRule{}, false
	}

	return lintRule{
		Type:      ruleType,
		Payload:   strings.TrimSpace(rule.Value),
		Policy:    strings.TrimSpace(ref.Policy),
		Line:      ref.ParentIndex,
		Source:    ref.Source,
		NoResolve: rule.NoResolve,
	}, true
}

func parseRulesToLintRules(lines []string) []lintRule {
	rules := make([]lintRule, 0, len(lines))
	for i, line := range lines {
		rule, ok := parseRuleLine(line, i+1)
		if ok {
			rules = append(rules, rule)
		}
	}
	return rules
}

func parseRuleLine(line string, lineNo int) (lintRule, bool) {
	parts := app.SplitCSVLine(line)
	if len(parts) < 2 {
		return lintRule{}, false
	}

	ruleType := strings.ToUpper(strings.TrimSpace(parts[0]))
	rule := lintRule{
		Type:    ruleType,
		Line:    lineNo,
		Source:  "template",
		Payload: "",
		Policy:  "",
	}

	switch ruleType {
	case "MATCH", "FINAL":
		if len(parts) >= 2 {
			rule.Policy = strings.TrimSpace(parts[1])
		}
		return rule, true
	case "RULE-SET", "DOMAIN-SET":
		if len(parts) >= 3 {
			rule.Payload = strings.TrimSpace(parts[1])
			rule.Policy = strings.TrimSpace(parts[2])
		}
		return rule, true
	default:
		if len(parts) < 3 {
			return lintRule{}, false
		}
		rule.Payload = strings.TrimSpace(parts[1])
		rule.Policy = strings.TrimSpace(parts[2])
		for _, part := range parts[3:] {
			if strings.EqualFold(strings.TrimSpace(part), "no-resolve") {
				rule.NoResolve = true
			}
		}
		return rule, true
	}
}

func checkTerminalRules(rules []lintRule) []string {
	terminalIndexes := make([]int, 0, 2)
	for i, rule := range rules {
		if isTerminalRuleType(rule.Type) {
			terminalIndexes = append(terminalIndexes, i)
		}
	}

	if len(terminalIndexes) == 0 {
		return []string{"缺少兜底规则，建议最后加上 MATCH 或 FINAL"}
	}

	warnings := make([]string, 0, 2)
	if len(terminalIndexes) > 1 {
		indexes := make([]int, 0, len(terminalIndexes))
		for _, idx := range terminalIndexes {
			indexes = append(indexes, rules[idx].Line)
		}
		warnings = append(warnings, fmt.Sprintf("检测到多个兜底规则：第 %d、%s 条", indexes[0], joinRuleIndexes(indexes[1:])))
	}

	lastTerminal := terminalIndexes[len(terminalIndexes)-1]
	if lastTerminal != len(rules)-1 {
		warnings = append(warnings, fmt.Sprintf("兜底规则位于第 %d 条，后面还有 %d 条规则不会生效", rules[lastTerminal].Line, len(rules)-1-lastTerminal))
	}
	return warnings
}

func checkDuplicateRules(rules []lintRule) []string {
	warnings := make([]string, 0)
	seen := make(map[string]lintRule, len(rules))

	for _, rule := range rules {
		key := duplicateKey(rule)
		if prev, ok := seen[key]; ok {
			if rule.Source == "template" && prev.Source == "template" {
				warnings = append(warnings, fmt.Sprintf("第 %d 条规则与第 %d 条重复：%s,%s -> %s", rule.Line, prev.Line, rule.Type, rule.Payload, rule.Policy))
			} else {
				warnings = append(warnings, fmt.Sprintf(
					"展开后发现重复规则：第 %d 条（%s）与第 %d 条（%s）都包含 %s,%s -> %s",
					rule.Line, ruleSourceLabel(rule),
					prev.Line, ruleSourceLabel(prev),
					rule.Type, rule.Payload, rule.Policy,
				))
			}
			continue
		}
		seen[key] = rule
	}

	return warnings
}

func checkRuleOrder(rules []lintRule) []string {
	warnings := make([]string, 0)
	var blockingRule *lintRule

	for i := range rules {
		rule := rules[i]
		priority := ruleOrderPriority(rule.Type)
		if priority == 0 {
			continue
		}

		if blockingRule != nil {
			blockingPriority := ruleOrderPriority(blockingRule.Type)
			if blockingPriority > 0 && priority < blockingPriority {
				warnings = append(warnings, fmt.Sprintf(
					"第 %d 条规则应放到第 %d 条之前：%s,%s -> %s 当前位于更宽泛规则 %s,%s -> %s 之后",
					rule.Line, blockingRule.Line,
					rule.Type, rule.Payload, rule.Policy,
					blockingRule.Type, blockingRule.Payload, blockingRule.Policy,
				))
			}
		}

		if blockingRule == nil || priority > ruleOrderPriority(blockingRule.Type) {
			blockingRule = &rules[i]
		}
	}

	return warnings
}

func checkExpandedRuleSetOrder(rules []lintRule) []string {
	if len(rules) == 0 {
		return nil
	}

	var blocker *lintRule
	issues := make(map[string]*lintRuleSetOrderIssue)

	for i := range rules {
		rule := rules[i]
		priority := ruleOrderPriority(rule.Type)
		if priority == 0 {
			continue
		}

		if blocker != nil {
			blockerPriority := ruleOrderPriority(blocker.Type)
			if blockerPriority > 0 && priority < blockerPriority && !sameRuleSetRef(*blocker, rule) {
				key := fmt.Sprintf("%d|%s|%d|%s", rule.Line, rule.Source, blocker.Line, blocker.Source)
				issue := issues[key]
				if issue == nil {
					issue = &lintRuleSetOrderIssue{
						blockedLine:   rule.Line,
						blockedSource: rule.Source,
						blockerLine:   blocker.Line,
						blockerSource: blocker.Source,
						sample:        rule,
					}
					issues[key] = issue
				}
				issue.count++
			}
		}

		if blocker == nil || priority > ruleOrderPriority(blocker.Type) {
			blocker = &rules[i]
		}
	}

	if len(issues) == 0 {
		return nil
	}

	warnings := make([]string, 0, len(issues))
	for _, issue := range issues {
		warnings = append(warnings, fmt.Sprintf(
			"第 %d 条 ruleset（%s）应放到第 %d 条 ruleset（%s）之前：有 %d 条展开规则存在顺序风险，示例 %s,%s -> %s",
			issue.blockedLine, issue.blockedSource,
			issue.blockerLine, issue.blockerSource,
			issue.count,
			issue.sample.Type, issue.sample.Payload, issue.sample.Policy,
		))
	}
	return warnings
}

func checkRuleShadowing(rules []lintRule) []string {
	warnings := make([]string, 0)

	for i := 0; i < len(rules); i++ {
		left := rules[i]
		if isTerminalRuleType(left.Type) {
			break
		}

		for j := i + 1; j < len(rules); j++ {
			right := rules[j]
			if isTerminalRuleType(right.Type) {
				continue
			}
			if shadows(left, right) {
				if strings.EqualFold(left.Policy, right.Policy) {
					warnings = append(warnings, fmt.Sprintf(
						"第 %d 条规则（%s）会覆盖第 %d 条同策略规则（%s）：%s,%s -> %s",
						left.Line, ruleSourceLabel(left),
						right.Line, ruleSourceLabel(right),
						right.Type, right.Payload, right.Policy,
					))
				} else {
					warnings = append(warnings, fmt.Sprintf(
						"第 %d 条规则（%s）会覆盖第 %d 条规则（%s）：%s,%s -> %s 将先于 %s 生效",
						left.Line, ruleSourceLabel(left),
						right.Line, ruleSourceLabel(right),
						right.Type, right.Payload, left.Policy, right.Policy,
					))
				}
			}
		}
	}

	return warnings
}

func shadows(left, right lintRule) bool {
	if left.Type == right.Type && strings.EqualFold(strings.TrimSpace(left.Payload), strings.TrimSpace(right.Payload)) {
		return true
	}

	switch left.Type {
	case "DOMAIN-SUFFIX":
		switch right.Type {
		case "DOMAIN", "DOMAIN-SUFFIX":
			return domainMatchesSuffix(right.Payload, left.Payload)
		}
	case "DOMAIN":
		return right.Type == "DOMAIN" && strings.EqualFold(strings.TrimSpace(left.Payload), strings.TrimSpace(right.Payload))
	case "DOMAIN-KEYWORD":
		switch right.Type {
		case "DOMAIN", "DOMAIN-SUFFIX", "DOMAIN-KEYWORD":
			return strings.Contains(strings.ToLower(strings.TrimSpace(right.Payload)), strings.ToLower(strings.TrimSpace(left.Payload)))
		}
	case "IP-CIDR":
		if right.Type != "IP-CIDR" {
			return false
		}
		return prefixContains(left.Payload, right.Payload)
	}

	return false
}

func domainMatchesSuffix(candidate, suffix string) bool {
	candidate = strings.ToLower(strings.Trim(candidate, ". "))
	suffix = strings.ToLower(strings.Trim(suffix, ". "))
	return candidate == suffix || strings.HasSuffix(candidate, "."+suffix)
}

func prefixContains(parentRaw, childRaw string) bool {
	parent, err := netip.ParsePrefix(strings.TrimSpace(parentRaw))
	if err != nil {
		return false
	}
	child, err := netip.ParsePrefix(strings.TrimSpace(childRaw))
	if err != nil {
		return false
	}
	return parent.Contains(child.Addr()) && parent.Bits() <= child.Bits()
}

func duplicateKey(rule lintRule) string {
	return fmt.Sprintf("%s|%s|%s|%t", strings.ToUpper(rule.Type), strings.ToLower(strings.TrimSpace(rule.Payload)), strings.ToLower(strings.TrimSpace(rule.Policy)), rule.NoResolve)
}

func ruleSourceLabel(rule lintRule) string {
	source := strings.TrimSpace(rule.Source)
	if source == "" || source == "template" {
		return fmt.Sprintf("模板第 %d 条", rule.Line)
	}
	return source
}

func sameRuleSetRef(left, right lintRule) bool {
	return left.Line == right.Line && strings.EqualFold(strings.TrimSpace(left.Source), strings.TrimSpace(right.Source))
}

func ruleOrderPriority(ruleType string) int {
	switch strings.ToUpper(strings.TrimSpace(ruleType)) {
	case "DOMAIN", "DOMAIN-SUFFIX", "DOMAIN-KEYWORD":
		return 1
	case "RULE-SET", "DOMAIN-SET":
		return 2
	case "IP-CIDR", "IP-CIDR6":
		return 3
	case "GEOIP":
		return 4
	case "MATCH", "FINAL":
		return 5
	default:
		return 0
	}
}

func extractYAMLRules(content string) ([]string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil, err
	}

	mapping := app.YAMLRootMappingNode(&doc)
	if mapping == nil {
		return nil, fmt.Errorf("根节点不是映射")
	}

	rulesNode := app.YAMLLookupMappingValue(mapping, "rules")
	if rulesNode == nil {
		return nil, nil
	}
	if rulesNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("rules 不是序列")
	}

	rules := make([]string, 0, len(rulesNode.Content))
	for _, item := range rulesNode.Content {
		if item.Kind != yaml.ScalarNode {
			continue
		}
		rule := strings.TrimSpace(item.Value)
		if rule != "" {
			rules = append(rules, rule)
		}
	}
	return rules, nil
}

func extractYAMLRuleProviders(content string) (map[string]yamlRuleProvider, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return nil, err
	}

	mapping := app.YAMLRootMappingNode(&doc)
	if mapping == nil {
		return nil, fmt.Errorf("根节点不是映射")
	}

	providersNode := app.YAMLLookupMappingValue(mapping, "rule-providers")
	if providersNode == nil || providersNode.Kind != yaml.MappingNode {
		return map[string]yamlRuleProvider{}, nil
	}

	providers := make(map[string]yamlRuleProvider)
	for i := 0; i+1 < len(providersNode.Content); i += 2 {
		name := strings.TrimSpace(providersNode.Content[i].Value)
		valueNode := providersNode.Content[i+1]
		if name == "" || valueNode.Kind != yaml.MappingNode {
			continue
		}
		urlNode := app.YAMLLookupMappingValue(valueNode, "url")
		behaviorNode := app.YAMLLookupMappingValue(valueNode, "behavior")
		providers[name] = yamlRuleProvider{
			URL:    scalarNodeValue(urlNode),
			Format: providerBehaviorToFormat(scalarNodeValue(behaviorNode)),
		}
	}

	return providers, nil
}

func extractSurgeRules(content string) []string {
	lines := strings.Split(content, "\n")
	rules := make([]string, 0)
	inRuleSection := false

	for _, rawLine := range lines {
		line := strings.TrimSpace(strings.TrimRight(rawLine, "\r"))
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inRuleSection = line == "[Rule]"
			continue
		}
		if !inRuleSection || line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		rules = append(rules, line)
	}

	return rules
}

func buildYAMLRuleSetRefs(rules []string, providers map[string]yamlRuleProvider) []ruleSetReference {
	refs := make([]ruleSetReference, 0)
	for i, rule := range rules {
		parts := app.SplitCSVLine(rule)
		if len(parts) < 3 || !strings.EqualFold(strings.TrimSpace(parts[0]), "RULE-SET") {
			continue
		}

		providerName := strings.TrimSpace(parts[1])
		policy := strings.TrimSpace(parts[2])
		provider, ok := providers[providerName]
		if !ok || strings.TrimSpace(provider.URL) == "" {
			continue
		}

		refs = append(refs, ruleSetReference{
			ParentIndex: i + 1,
			Kind:        "RULE-SET",
			Policy:      policy,
			Source:      strings.TrimSpace(provider.URL),
			Format:      provider.Format,
			Raw:         rule,
		})
	}
	return refs
}

func buildSurgeRuleSetRefs(rules []string) []ruleSetReference {
	refs := make([]ruleSetReference, 0)
	for i, rule := range rules {
		parts := app.SplitCSVLine(rule)
		if len(parts) < 3 {
			continue
		}

		kind := strings.ToUpper(strings.TrimSpace(parts[0]))
		if kind != "RULE-SET" && kind != "DOMAIN-SET" {
			continue
		}

		source := strings.TrimSpace(parts[1])
		policy := strings.TrimSpace(parts[2])
		refs = append(refs, ruleSetReference{
			ParentIndex: i + 1,
			Kind:        kind,
			Policy:      policy,
			Source:      source,
			Format:      inferRuleSetFormat(source, kind),
			Raw:         rule,
		})
	}
	return refs
}

func inferRuleSetFormat(source, kind string) string {
	if u, err := url.Parse(source); err == nil {
		target := strings.TrimSpace(u.Query().Get("target"))
		switch target {
		case "surge":
			return "surge"
		case "clash-classical":
			return "clash-classical"
		case "clash-domain":
			return "clash-domain"
		case "clash-ipcidr":
			return "clash-ipcidr"
		case "domain-list":
			return "domain-list"
		case "ip-list":
			return "ip-list"
		}
	}

	if strings.EqualFold(kind, "DOMAIN-SET") {
		return "domain-list"
	}
	return "surge"
}

func providerBehaviorToFormat(behavior string) string {
	switch strings.ToLower(strings.TrimSpace(behavior)) {
	case "domain":
		return "clash-domain"
	case "ipcidr":
		return "clash-ipcidr"
	case "classical":
		return "clash-classical"
	default:
		return ""
	}
}

func isTerminalRuleType(ruleType string) bool {
	switch strings.ToUpper(strings.TrimSpace(ruleType)) {
	case "MATCH", "FINAL":
		return true
	default:
		return false
	}
}

func scalarNodeValue(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	return strings.TrimSpace(node.Value)
}

func joinRuleIndexes(indexes []int) string {
	values := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		values = append(values, fmt.Sprintf("%d", idx))
	}
	return strings.Join(values, "、")
}

func loadRemoteRuleSetRulesFromCache(ctx context.Context, h *Handlers, ref ruleSetReference) ([]app.RuleSetRule, error) {
	if h == nil || h.redisClient == nil {
		return nil, fmt.Errorf("redis 不可用")
	}

	raw, err := h.redisClient.Get(ctx, remoteRuleSetCacheKey(ref))
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("缓存未命中")
	}

	var payload remoteRuleSetCachePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	if strings.TrimSpace(payload.Error) != "" {
		return nil, errors.New(payload.Error)
	}
	if len(payload.Rules) == 0 {
		return nil, fmt.Errorf("缓存中没有规则")
	}

	return payload.Rules, nil
}

func storeRemoteRuleSetCache(ctx context.Context, h *Handlers, ref ruleSetReference, payload remoteRuleSetCachePayload, ttl time.Duration) {
	if h == nil || h.redisClient == nil || ttl <= 0 {
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	_ = h.redisClient.Set(ctx, remoteRuleSetCacheKey(ref), string(data), ttl)
}

func remoteRuleSetCacheKey(ref ruleSetReference) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(ref.Source) + "|" + strings.TrimSpace(ref.Format)))
	return remoteRuleSetCacheKeyPrefix + fmt.Sprintf("%x", sum[:])
}
