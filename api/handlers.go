package api

import (
	"context"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ablate-ai/RuleFlow/cache"
	"github.com/ablate-ai/RuleFlow/database"
	"github.com/ablate-ai/RuleFlow/internal/app"
	"github.com/ablate-ai/RuleFlow/services"
)

// urlParamInt64 从 chi 路由参数中提取整数 ID
func urlParamInt64(r *http.Request, key string) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, key), 10, 64)
}

// policyConfigCache 策略配置缓存接口
type policyConfigCache interface {
	GetPolicyConfig(ctx context.Context, token string) (string, error)
	SetPolicyConfig(ctx context.Context, token, yaml string) error
	DeletePolicyConfig(ctx context.Context, token string) error
	DeleteAllByPattern(ctx context.Context, pattern string) error
}

type manualNodeRequest struct {
	Name     string                 `json:"name"`
	Protocol string                 `json:"protocol"`
	Server   string                 `json:"server"`
	Port     int                    `json:"port"`
	Config   map[string]interface{} `json:"config"`
	Tags     []string               `json:"tags"`
	Enabled  bool                   `json:"enabled"`
	ShareURL string                 `json:"share_url"`
}

type configResponseMeta struct {
	filename    string
	contentType string
}

type convertRequestParams struct {
	subURL     string
	templateID int64
	target     string
}

type validateTemplateRequest struct {
	Content string `json:"content"`
	Target  string `json:"target"`
}

// Handlers API 处理器
type Handlers struct {
	subscriptionService     *services.SubscriptionService
	templateService         *services.TemplateService
	configPolicyService     *services.ConfigPolicyService
	ruleSourceService       *services.RuleSourceService
	nodeService             *services.NodeService
	maintenanceService      *services.MaintenanceService
	subscriptionSyncService *services.SubscriptionSyncService
	ruleSourceSyncService   *services.RuleSourceSyncService
	policyCache             policyConfigCache
	redisClient             *cache.Client
	db                      *database.DB
}

// NewHandlers 创建 API 处理器
func NewHandlers(
	subscriptionService *services.SubscriptionService,
	templateService *services.TemplateService,
	configPolicyService *services.ConfigPolicyService,
	ruleSourceService *services.RuleSourceService,
	nodeService *services.NodeService,
	maintenanceService *services.MaintenanceService,
	subscriptionSyncService *services.SubscriptionSyncService,
	ruleSourceSyncService *services.RuleSourceSyncService,
	policyCache policyConfigCache,
	redisClient *cache.Client,
	db *database.DB,
) *Handlers {
	return &Handlers{
		subscriptionService:     subscriptionService,
		templateService:         templateService,
		configPolicyService:     configPolicyService,
		ruleSourceService:       ruleSourceService,
		nodeService:             nodeService,
		maintenanceService:      maintenanceService,
		subscriptionSyncService: subscriptionSyncService,
		ruleSourceSyncService:   ruleSourceSyncService,
		policyCache:             policyCache,
		redisClient:             redisClient,
		db:                      db,
	}
}

// MigrateSnowflakeIDs 将旧的自增 ID 数据刷成雪花 ID
func (h *Handlers) MigrateSnowflakeIDs(w http.ResponseWriter, r *http.Request) {
	if h.maintenanceService == nil {
		SendError(w, http.StatusServiceUnavailable, "维护服务未启用")
		return
	}

	report, err := h.maintenanceService.MigrateLegacyIDsToSnowflake(r.Context())
	if err != nil {
		SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	SendSuccess(w, report)
}

// CreateSubscription 创建订阅
func (h *Handlers) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	var sub database.Subscription
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	ctx := r.Context()
	if err := h.subscriptionService.CreateSubscription(ctx, &sub); err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	SendSuccess(w, sub)
}

// GetSubscription 获取订阅信息
func (h *Handlers) GetSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的订阅 ID")
		return
	}

	ctx := r.Context()
	sub, err := h.subscriptionService.GetSubscriptionByID(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	SendSuccess(w, sub)
}

// ListSubscriptions 列出所有订阅
func (h *Handlers) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	subs, err := h.subscriptionService.ListSubscriptions(ctx)
	if err != nil {
		SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	SendSuccess(w, subs)
}

// UpdateSubscription 更新订阅
func (h *Handlers) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的订阅 ID")
		return
	}

	var sub database.Subscription
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	sub.ID = id

	ctx := r.Context()
	if err := h.subscriptionService.UpdateSubscription(ctx, &sub); err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	SendSuccess(w, sub)
}

// DeleteSubscription 删除订阅
func (h *Handlers) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的订阅 ID")
		return
	}

	ctx := r.Context()
	if err := h.subscriptionService.DeleteSubscriptionByID(ctx, id); err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	SendSuccess(w, map[string]string{"message": "订阅已删除"})
}

// Health 健康检查
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	health := h.subscriptionService.Health(ctx)

	status := http.StatusOK
	if health["status"] != "healthy" {
		status = http.StatusServiceUnavailable
	}

	SendJSON(w, status, health)
}

// ClearAllPolicyCache 清除所有策略配置缓存
func (h *Handlers) ClearAllPolicyCache(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.policyCache != nil {
		_ = h.policyCache.DeleteAllByPattern(ctx, "ruleflow:policy:config:*")
	}
	SendSuccess(w, map[string]string{"message": "所有策略配置缓存已清除"})
}

// ==================== 模板 API ====================

// CreateTemplate 创建模板
func (h *Handlers) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var tpl database.Template
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	ctx := r.Context()
	if err := h.templateService.CreateTemplate(ctx, &tpl); err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	SendSuccess(w, tpl)
}

// GetTemplate 获取模板信息
func (h *Handlers) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的模板 ID")
		return
	}

	ctx := r.Context()
	tpl, err := h.templateService.GetTemplateByID(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	SendSuccess(w, tpl)
}

// ListTemplates 列出所有模板
func (h *Handlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tpls, err := h.templateService.ListTemplates(ctx)
	if err != nil {
		SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	SendSuccess(w, tpls)
}

// ListPublicTemplates 列出公开模板（无需鉴权，仅返回 id/name/target）
func (h *Handlers) ListPublicTemplates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tpls, err := h.templateService.ListPublicTemplates(ctx)
	if err != nil {
		SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	SendSuccess(w, tpls)
}

// GetPublicTemplate 获取公开模板内容（无需鉴权，仅限 is_public = true）
func (h *Handlers) GetPublicTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的模板 ID")
		return
	}

	ctx := r.Context()
	tpl, err := h.templateService.GetPublicTemplateByID(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	SendSuccess(w, tpl)
}

// ValidateTemplate 校验模板内容
func (h *Handlers) ValidateTemplate(w http.ResponseWriter, r *http.Request) {
	var req validateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		SendError(w, http.StatusBadRequest, "模板内容不能为空")
		return
	}

	target, err := resolveConfigTarget(req.Target, "")
	if err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	configContent, err := buildConfigContent(r, templateValidationNodes(), req.Content, target)
	if err != nil {
		SendError(w, http.StatusBadRequest, "模板检查失败: "+err.Error())
		return
	}

	warnings := lintGeneratedTemplate(r.Context(), h, target, configContent)
	result := templateValidationResult{
		Message: "模板检查通过",
	}
	if len(warnings) > 0 {
		result.Message = fmt.Sprintf("模板可生成，但发现 %d 个规则问题", len(warnings))
		result.Warnings = warnings
		result.WarningSummary = summarizeLintWarnings(warnings)
	}

	SendSuccess(w, result)
}

// UpdateTemplate 更新模板
func (h *Handlers) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的模板 ID")
		return
	}

	var tpl database.Template
	if err := json.NewDecoder(r.Body).Decode(&tpl); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if strings.TrimSpace(tpl.Name) == "" {
		SendError(w, http.StatusBadRequest, "模板名称不能为空")
		return
	}

	ctx := r.Context()
	if err := h.templateService.UpdateTemplate(ctx, id, &tpl); err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 模板更新后清除所有策略配置缓存
	if h.policyCache != nil {
		_ = h.policyCache.DeleteAllByPattern(ctx, "ruleflow:policy:config:*")
	}

	SendSuccess(w, tpl)
}

// DeleteTemplate 删除模板
func (h *Handlers) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的模板 ID")
		return
	}

	ctx := r.Context()
	if err := h.templateService.DeleteTemplate(ctx, id); err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	// 模板删除后清除所有策略配置缓存
	if h.policyCache != nil {
		_ = h.policyCache.DeleteAllByPattern(ctx, "ruleflow:policy:config:*")
	}

	SendSuccess(w, map[string]string{"message": "模板已删除"})
}

// ==================== 规则源 API ====================

func (h *Handlers) CreateRuleSource(w http.ResponseWriter, r *http.Request) {
	var source database.RuleSource
	if err := json.NewDecoder(r.Body).Decode(&source); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	ctx := r.Context()
	if err := h.ruleSourceService.Create(ctx, &source); err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	SendSuccess(w, source)
}

func (h *Handlers) GetRuleSource(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的规则源 ID")
		return
	}

	ctx := r.Context()
	source, err := h.ruleSourceService.GetByID(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	SendSuccess(w, source)
}

func (h *Handlers) ListRuleSources(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sources, err := h.ruleSourceService.List(ctx)
	if err != nil {
		SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	SendSuccess(w, sources)
}

func (h *Handlers) UpdateRuleSource(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的规则源 ID")
		return
	}

	var source database.RuleSource
	if err := json.NewDecoder(r.Body).Decode(&source); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	source.ID = id

	ctx := r.Context()
	if err := h.ruleSourceService.Update(ctx, &source); err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	SendSuccess(w, source)
}

func (h *Handlers) DeleteRuleSource(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的规则源 ID")
		return
	}
	ctx := r.Context()
	if err := h.ruleSourceService.Delete(ctx, id); err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}
	SendSuccess(w, map[string]string{"message": "规则源已删除"})
}

func (h *Handlers) SyncRuleSource(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的规则源 ID")
		return
	}
	ctx := r.Context()
	count, err := h.ruleSourceSyncService.SyncRuleSource(ctx, id)
	if err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}
	SendSuccess(w, map[string]interface{}{
		"message":    "规则源同步完成",
		"rule_count": count,
	})
}

func (h *Handlers) ExportRuleSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if strings.TrimSpace(name) == "" {
		http.Error(w, "缺少规则源名称", http.StatusBadRequest)
		return
	}
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		target = "sing-box"
	}

	ctx := r.Context()

	if target == "sing-box" {
		h.exportRuleSourceSRS(w, ctx, name)
		return
	}

	content, err := h.ruleSourceSyncService.ExportRuleSource(ctx, name, target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, content)
}

// exportRuleSourceSRS 将规则源编译为 sing-box SRS 二进制格式（原生实现，无需外部依赖）
func (h *Handlers) exportRuleSourceSRS(w http.ResponseWriter, ctx context.Context, name string) {
	// 先写入缓冲，确保编译失败时可以返回正确的错误状态码
	var buf bytes.Buffer
	if err := h.ruleSourceSyncService.ExportRuleSourceSRS(ctx, name, &buf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 过滤文件名中可能破坏 header 结构的字符
	safeName := strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(name)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.srs"`, safeName))
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	buf.WriteTo(w) //nolint:errcheck
}

// ConfigResponse 配置响应
type ConfigResponse struct {
	YAML      string `json:"yaml"`
	NodeCount int    `json:"node_count"`
	FromCache bool   `json:"from_cache"`
}

// ==================== 配置策略 API ====================

// CreateConfigPolicy 创建配置策略
func (h *Handlers) CreateConfigPolicy(w http.ResponseWriter, r *http.Request) {
	var policy database.ConfigPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	ctx := r.Context()
	if err := h.configPolicyService.Create(ctx, &policy); err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 新策略创建后不需要清除缓存，因为还没有被请求过

	SendSuccess(w, policy)
}

// GetConfigPolicy 获取配置策略
func (h *Handlers) GetConfigPolicy(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的策略 ID")
		return
	}

	ctx := r.Context()
	policy, err := h.configPolicyService.GetByID(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	SendSuccess(w, policy)
}

// ListConfigPolicies 列出所有配置策略
func (h *Handlers) ListConfigPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	policies, err := h.configPolicyService.List(ctx)
	if err != nil {
		SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	SendSuccess(w, policies)
}

// UpdateConfigPolicy 更新配置策略
func (h *Handlers) UpdateConfigPolicy(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的策略 ID")
		return
	}

	var policy database.ConfigPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	policy.ID = id

	ctx := r.Context()
	// 先获取旧策略以获得 token
	oldPolicy, err := h.configPolicyService.GetByID(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := h.configPolicyService.Update(ctx, &policy); err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 更新策略后清除该策略的缓存
	if h.policyCache != nil && oldPolicy.Token != "" {
		_ = h.policyCache.DeletePolicyConfig(ctx, oldPolicy.Token)
	}

	SendSuccess(w, policy)
}

// DeleteConfigPolicy 删除配置策略
func (h *Handlers) DeleteConfigPolicy(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的策略 ID")
		return
	}

	ctx := r.Context()
	if err := h.configPolicyService.Delete(ctx, id); err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	SendSuccess(w, map[string]string{"message": "配置策略已删除"})
}

// ClearPolicyConfigCache 清除指定策略的生成配置缓存
func (h *Handlers) ClearPolicyConfigCache(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的策略 ID")
		return
	}

	ctx := r.Context()
	policy, err := h.configPolicyService.GetByID(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, "策略不存在")
		return
	}

	if h.policyCache != nil && policy.Token != "" {
		_ = h.policyCache.DeletePolicyConfig(ctx, policy.Token)
	}

	SendSuccess(w, map[string]string{"message": "配置缓存已清除"})
}

// ListConfigPolicyAccessLogs 列出指定策略最近访问日志
func (h *Handlers) ListConfigPolicyAccessLogs(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的策略 ID")
		return
	}

	ctx := r.Context()
	logs, err := h.configPolicyService.ListAccessLogs(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	SendSuccess(w, logs)
}

// ListAllConfigAccessLogs 列出全局访问日志
func (h *Handlers) ListAllConfigAccessLogs(w http.ResponseWriter, r *http.Request) {
	filter := database.ConfigAccessLogFilter{}

	if raw := strings.TrimSpace(r.URL.Query().Get("policy_id")); raw != "" {
		policyID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || policyID <= 0 {
			SendError(w, http.StatusBadRequest, "无效的 policy_id 参数")
			return
		}
		filter.PolicyID = &policyID
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("success")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			SendError(w, http.StatusBadRequest, "无效的 success 参数")
			return
		}
		filter.Success = &value
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("cache_hit")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			SendError(w, http.StatusBadRequest, "无效的 cache_hit 参数")
			return
		}
		filter.CacheHit = &value
	}

	logs, err := h.configPolicyService.ListAllAccessLogs(r.Context(), filter)
	if err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	SendSuccess(w, logs)
}

// ==================== 节点管理 API ====================

// ImportNodes 通过 URL 批量导入节点
func (h *Handlers) ImportNodes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	lines := strings.Split(req.Content, "\n")
	type importErr struct {
		URL   string `json:"url"`
		Error string `json:"error"`
	}
	var created int
	var updated int
	var errors []importErr

	ctx := r.Context()
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		proxyNode, err := app.ParseNodeURL(line)
		if err != nil {
			errors = append(errors, importErr{URL: line, Error: err.Error()})
			continue
		}
		node := &database.Node{
			Name:     proxyNode.Name,
			Protocol: proxyNode.Protocol,
			Server:   proxyNode.Server,
			Port:     proxyNode.Port,
			Config:   proxyNode.Options,
		}
		inserted, err := h.nodeService.ImportManualNode(ctx, node)
		if err != nil {
			errors = append(errors, importErr{URL: line, Error: err.Error()})
			continue
		}
		if inserted {
			created++
		} else {
			updated++
		}
	}

	SendSuccess(w, map[string]interface{}{
		"created": created,
		"updated": updated,
		"errors":  errors,
	})
}

// CreateNode 创建节点（手动添加）
func (h *Handlers) CreateNode(w http.ResponseWriter, r *http.Request) {
	var req manualNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	node, err := h.buildManualNodeFromRequest(req, nil)
	if err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()
	if err := h.nodeService.ValidateNode(node); err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.nodeService.AddManualNode(ctx, node); err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	SendSuccess(w, node)
}

// GetNode 获取节点详情
func (h *Handlers) GetNode(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的节点 ID")
		return
	}

	ctx := r.Context()
	node, err := h.nodeService.GetNode(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	SendSuccess(w, node)
}

// ListNodes 列出节点
func (h *Handlers) ListNodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 构建筛选条件
	filter := database.NodeFilter{}

	// 按来源筛选。兼容保留 source=manual，用 source_id IS NULL 实现。
	if source := r.URL.Query().Get("source"); source != "" {
		if source == "manual" {
			filter.ManualOnly = true
		}
	}

	// 按来源 ID 筛选
	if sourceIDText := r.URL.Query().Get("source_id"); sourceIDText != "" {
		sourceID, err := strconv.ParseInt(sourceIDText, 10, 64)
		if err != nil {
			SendError(w, http.StatusBadRequest, "无效的来源 ID")
			return
		}
		filter.SourceID = &sourceID
	}

	// 按协议筛选
	if protocol := r.URL.Query().Get("protocol"); protocol != "" {
		filter.Protocol = protocol
	}

	// 按启用状态筛选
	if enabled := r.URL.Query().Get("enabled"); enabled != "" {
		enabledBool := enabled == "true" || enabled == "1"
		filter.Enabled = &enabledBool
	}

	// 按标签筛选
	if tags := r.URL.Query().Get("tags"); tags != "" {
		filter.Tags = strings.Split(tags, ",")
	}

	nodes, err := h.nodeService.ListNodes(ctx, filter)
	if err != nil {
		SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	SendSuccess(w, nodes)
}

// UpdateNode 更新节点
func (h *Handlers) UpdateNode(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的节点 ID")
		return
	}

	ctx := r.Context()
	existing, err := h.nodeService.GetNode(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	var req manualNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	node, err := h.buildManualNodeFromRequest(req, existing)
	if err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.nodeService.ValidateNode(node); err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.nodeService.UpdateNode(ctx, id, node); err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 获取更新后的节点
	updatedNode, err := h.nodeService.GetNode(ctx, id)
	if err != nil {
		SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	SendSuccess(w, updatedNode)
}

func (h *Handlers) buildManualNodeFromRequest(req manualNodeRequest, existing *database.Node) (*database.Node, error) {
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	if req.Protocol == "wireguard" {
		return &database.Node{
			Name:     strings.TrimSpace(req.Name),
			Protocol: req.Protocol,
			Server:   strings.TrimSpace(req.Server),
			Port:     req.Port,
			Config:   req.Config,
			Tags:     tags,
			Enabled:  req.Enabled,
		}, nil
	}

	if existing != nil && existing.Protocol != "wireguard" && strings.TrimSpace(req.ShareURL) == "" {
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = existing.Name
		}
		return &database.Node{
			Name:     name,
			Protocol: existing.Protocol,
			Server:   existing.Server,
			Port:     existing.Port,
			Config:   existing.Config,
			Tags:     tags,
			Enabled:  req.Enabled,
		}, nil
	}

	if strings.TrimSpace(req.ShareURL) == "" {
		return nil, fmt.Errorf("普通节点请使用分享链接导入")
	}

	proxyNode, err := app.ParseNodeURL(strings.TrimSpace(req.ShareURL))
	if err != nil {
		return nil, fmt.Errorf("分享链接解析失败: %w", err)
	}
	if proxyNode.Protocol == "wireguard" {
		return nil, fmt.Errorf("wireguard 请使用专用表单添加")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		if existing != nil {
			name = existing.Name
		} else {
			name = proxyNode.Name
		}
	}

	return &database.Node{
		Name:     name,
		Protocol: proxyNode.Protocol,
		Server:   proxyNode.Server,
		Port:     proxyNode.Port,
		Config:   proxyNode.Options,
		Tags:     tags,
		Enabled:  req.Enabled,
	}, nil
}

// GetNodeShareURL 获取节点的分享链接（用于编辑时预填）
func (h *Handlers) GetNodeShareURL(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的节点 ID")
		return
	}

	ctx := r.Context()
	node, err := h.nodeService.GetNode(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	shareURL, err := app.SerializeNodeURL(node.Name, node.Protocol, node.Server, node.Port, node.Config)
	if err != nil {
		SendError(w, http.StatusBadRequest, "该协议不支持生成分享链接: "+err.Error())
		return
	}

	SendSuccess(w, map[string]string{"share_url": shareURL})
}

// DeleteNode 删除节点
func (h *Handlers) DeleteNode(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的节点 ID")
		return
	}

	ctx := r.Context()
	if err := h.nodeService.DeleteNode(ctx, id); err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	SendSuccess(w, map[string]string{"message": "节点已删除"})
}

// BatchNodeOperation 批量节点操作
func (h *Handlers) BatchNodeOperation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs     []int64 `json:"ids"`
		Enabled *bool   `json:"enabled"`
		Action  string  `json:"action"` // enable, disable
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if len(req.IDs) == 0 {
		SendError(w, http.StatusBadRequest, "节点 ID 列表不能为空")
		return
	}

	ctx := r.Context()

	// 根据操作类型执行
	switch req.Action {
	case "enable":
		enabled := true
		count, err := h.nodeService.BatchEnable(ctx, req.IDs, enabled)
		if err != nil {
			SendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		SendSuccess(w, map[string]interface{}{"message": "节点已启用", "count": count})

	case "disable":
		enabled := false
		count, err := h.nodeService.BatchEnable(ctx, req.IDs, enabled)
		if err != nil {
			SendError(w, http.StatusInternalServerError, err.Error())
			return
		}
		SendSuccess(w, map[string]interface{}{"message": "节点已禁用", "count": count})

	default:
		SendError(w, http.StatusBadRequest, "不支持的操作: "+req.Action)
	}
}

// ==================== 订阅同步 API ====================

// SyncSubscription 同步订阅节点
func (h *Handlers) SyncSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的订阅 ID")
		return
	}

	ctx := r.Context()
	sub, err := h.subscriptionService.GetSubscriptionByID(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	count, err := h.subscriptionSyncService.SyncSubscription(ctx, sub.ID)
	if err != nil {
		SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	SendSuccess(w, map[string]interface{}{
		"message":    "订阅同步成功",
		"node_count": count,
	})
}

// SyncAllSubscriptions 同步全部可用订阅节点
func (h *Handlers) SyncAllSubscriptions(w http.ResponseWriter, r *http.Request) {
	result, err := h.subscriptionSyncService.SyncAllSubscriptions(r.Context())
	if err != nil {
		SendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	SendSuccess(w, result)
}

// GetSubscriptionSyncStatus 获取订阅同步状态
func (h *Handlers) GetSubscriptionSyncStatus(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		SendError(w, http.StatusBadRequest, "无效的订阅 ID")
		return
	}

	ctx := r.Context()
	sub, err := h.subscriptionService.GetSubscriptionByID(ctx, id)
	if err != nil {
		SendError(w, http.StatusNotFound, err.Error())
		return
	}

	status, err := h.subscriptionSyncService.GetSyncStatus(ctx, sub.ID)
	if err != nil {
		SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	SendSuccess(w, status)
}

// GetNodeStats 获取节点统计信息
func (h *Handlers) GetNodeStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := h.nodeService.GetNodeStats(ctx)
	if err != nil {
		SendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	SendSuccess(w, stats)
}

// ConvertSubscription 在线转换订阅配置
// 路由: GET /convert?url=https://example.com/sub&template=154025419991939
func (h *Handlers) ConvertSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logToken := "convert"
	params, err := parseConvertRequestParams(r)
	if err != nil {
		h.recordStandaloneConfigAccess(r, logToken, http.StatusBadRequest, false, nil, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	logToken = buildConvertLogToken(params)
	if params.subURL == "" {
		h.recordStandaloneConfigAccess(r, logToken, http.StatusBadRequest, false, nil, "缺少 url 参数")
		http.Error(w, "缺少 url 参数", http.StatusBadRequest)
		return
	}

	if h.templateService == nil {
		h.recordStandaloneConfigAccess(r, logToken, http.StatusInternalServerError, false, nil, "模板服务不可用")
		http.Error(w, "模板服务不可用", http.StatusInternalServerError)
		return
	}

	content, _, err := app.FetchSubscriptionContent(ctx, params.subURL)
	if err != nil {
		h.recordStandaloneConfigAccess(r, logToken, http.StatusBadGateway, false, nil, "获取订阅失败: "+err.Error())
		http.Error(w, "获取订阅失败: "+err.Error(), http.StatusBadGateway)
		return
	}

	proxyNodes, err := app.ParseSubscription(content)
	if err != nil {
		h.recordStandaloneConfigAccess(r, logToken, http.StatusBadRequest, false, nil, "解析订阅失败: "+err.Error())
		http.Error(w, "解析订阅失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(proxyNodes) == 0 {
		h.recordStandaloneConfigAccess(r, logToken, http.StatusServiceUnavailable, false, nil, "订阅中没有可用节点")
		http.Error(w, "订阅中没有可用节点", http.StatusServiceUnavailable)
		return
	}

	templateContent := ""
	targetFallback := "stash"
	if params.templateID > 0 {
		var tpl *database.Template
		tpl, err = h.templateService.GetTemplateByID(ctx, params.templateID)
		if err != nil {
			h.recordStandaloneConfigAccess(r, logToken, http.StatusNotFound, false, nil, err.Error())
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		templateContent = tpl.Content
		targetFallback = tpl.Target
	}

	target, err := resolveConfigTarget(params.target, targetFallback)
	if err != nil {
		h.recordStandaloneConfigAccess(r, logToken, http.StatusBadRequest, false, nil, err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	configContent, err := buildConfigContent(r, proxyNodes, templateContent, target)
	if err != nil {
		h.recordStandaloneConfigAccess(r, logToken, http.StatusInternalServerError, false, nil, "生成配置失败: "+err.Error())
		http.Error(w, "生成配置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	nodeCount := len(proxyNodes)
	h.recordStandaloneConfigAccess(r, logToken, http.StatusOK, true, &nodeCount, "")
	writeConfigResponse(w, r, target, configContent, len(proxyNodes))
}

// GenerateConfig 根据配置策略 token 生成 YAML 配置（带 Redis 缓存）
// 路由: GET /subscribe?token=xxx
func (h *Handlers) GenerateConfig(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.recordConfigAccess(r, nil, token, http.StatusBadRequest, false, false, nil, "缺少 token 参数")
		http.Error(w, "缺少 token 参数", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// 根据 token 查找策略
	policy, err := h.configPolicyService.GetByToken(ctx, token)
	if err != nil {
		h.recordConfigAccess(r, nil, token, http.StatusNotFound, false, false, nil, "无效的 token")
		http.Error(w, "无效的 token", http.StatusNotFound)
		return
	}

	if !policy.Enabled {
		h.recordConfigAccess(r, policy, token, http.StatusForbidden, false, false, nil, "该配置策略已禁用")
		http.Error(w, "该配置策略已禁用", http.StatusForbidden)
		return
	}

	// 优先从 Redis 缓存读取
	if h.policyCache != nil {
		if cached, err := h.policyCache.GetPolicyConfig(ctx, token); err == nil && cached != "" {
			switch policy.Target {
			case "surge":
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				cached = finalizeConfigContent(r, "surge", cached)
			case "sing-box":
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
			default:
				w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
			}
			w.Header().Set("X-Cache", "HIT")
			if userInfo := h.configPolicyService.GetUserInfoForPolicy(ctx, policy); userInfo != nil {
				w.Header().Set("Subscription-Userinfo", formatUserInfo(userInfo))
			}
			h.recordConfigAccess(r, policy, token, http.StatusOK, true, true, nil, "")
			fmt.Fprint(w, cached)
			return
		}
	}

	// 获取节点
	dbNodes, err := h.configPolicyService.GetNodesForPolicy(ctx, policy)
	if err != nil {
		h.recordConfigAccess(r, policy, token, http.StatusInternalServerError, false, false, nil, "获取节点失败: "+err.Error())
		http.Error(w, "获取节点失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if len(dbNodes) == 0 {
		h.recordConfigAccess(r, policy, token, http.StatusServiceUnavailable, false, false, nil, "该配置策略下没有可用节点，请先同步订阅源")
		http.Error(w, "该配置策略下没有可用节点，请先同步订阅源", http.StatusServiceUnavailable)
		return
	}

	// 获取模板内容
	var templateContent string
	if policy.TemplateName != "" {
		if tpl, err := h.templateService.GetTemplateByName(ctx, policy.TemplateName); err == nil {
			templateContent = tpl.Content
		}
	}

	// 将数据库节点转换为 ProxyNode
	proxyNodes := make([]*app.ProxyNode, 0, len(dbNodes))
	for _, n := range dbNodes {
		dialerProxy := ""
		if v, ok := n.Config["dialer-proxy"].(string); ok {
			dialerProxy = v
		} else if v, ok := n.Config["underlying-proxy"].(string); ok {
			dialerProxy = v
		}
		proxyNodes = append(proxyNodes, &app.ProxyNode{
			Protocol:    n.Protocol,
			Name:        n.Name,
			Server:      n.Server,
			Port:        n.Port,
			Options:     n.Config,
			DialerProxy: dialerProxy,
		})
	}

	// 生成配置，规范化 target（兼容数据库中旧的 clash-meta 等值）
	target, err := resolveConfigTarget(policy.Target, "clash-mihomo")
	if err != nil {
		h.recordConfigAccess(r, policy, token, http.StatusInternalServerError, false, false, nil, "配置策略目标类型无效: "+err.Error())
		http.Error(w, "配置策略目标类型无效: "+err.Error(), http.StatusInternalServerError)
		return
	}
	configContent, err := buildConfigContent(r, proxyNodes, templateContent, target)
	if err != nil {
		h.recordConfigAccess(r, policy, token, http.StatusInternalServerError, false, false, nil, "生成配置失败: "+err.Error())
		http.Error(w, "生成配置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 写入 Redis 缓存
	if h.policyCache != nil {
		_ = h.policyCache.SetPolicyConfig(ctx, token, configContent)
	}

	applyConfigResponseHeaders(w, target, len(proxyNodes))
	w.Header().Set("X-Cache", "MISS")
	if userInfo := h.configPolicyService.GetUserInfoForPolicy(ctx, policy); userInfo != nil {
		w.Header().Set("Subscription-Userinfo", formatUserInfo(userInfo))
	}
	nodeCount := len(proxyNodes)
	h.recordConfigAccess(r, policy, token, http.StatusOK, true, false, &nodeCount, "")
	fmt.Fprint(w, finalizeConfigContent(r, target, configContent))
}

func (h *Handlers) recordConfigAccess(
	r *http.Request,
	policy *database.ConfigPolicy,
	token string,
	statusCode int,
	success bool,
	cacheHit bool,
	nodeCount *int,
	errorMessage string,
) {
	if h.configPolicyService == nil {
		return
	}

	var policyID *int64
	if policy != nil {
		policyID = &policy.ID
	}

	log := &database.ConfigAccessLog{
		PolicyID:     policyID,
		Token:        token,
		ClientIP:     extractClientIP(r),
		UserAgent:    strings.TrimSpace(r.UserAgent()),
		StatusCode:   statusCode,
		Success:      success,
		CacheHit:     cacheHit,
		NodeCount:    nodeCount,
		ErrorMessage: strings.TrimSpace(errorMessage),
	}
	_ = h.configPolicyService.RecordAccess(r.Context(), log)
}

func (h *Handlers) recordStandaloneConfigAccess(
	r *http.Request,
	token string,
	statusCode int,
	success bool,
	nodeCount *int,
	errorMessage string,
) {
	h.recordConfigAccess(r, nil, token, statusCode, success, false, nodeCount, errorMessage)
}

func extractClientIP(r *http.Request) *string {
	candidates := []string{
		r.Header.Get("CF-Connecting-IP"),
		r.Header.Get("X-Real-IP"),
	}
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		candidates = append(candidates, strings.Split(forwardedFor, ",")[0])
	}
	candidates = append(candidates, r.RemoteAddr)

	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if host, _, err := net.SplitHostPort(candidate); err == nil {
			candidate = host
		}
		if addr, err := netip.ParseAddr(candidate); err == nil {
			value := addr.String()
			return &value
		}
	}

	return nil
}

func replaceTemplateRuntimePlaceholders(r *http.Request, content string) string {
	if content == "" {
		return content
	}
	baseURL := requestBaseURLString(r)
	if baseURL == "" {
		return content
	}
	ruleSetPathPattern := regexp.MustCompile(`/rulesets/[^\s",]+`)
	return ruleSetPathPattern.ReplaceAllStringFunc(content, func(path string) string {
		return baseURL + path
	})
}

func buildConfigContent(r *http.Request, proxyNodes []*app.ProxyNode, templateContent, target string) (string, error) {
	templateContent = strings.TrimSpace(templateContent)

	switch target {
	case "surge":
		if templateContent != "" {
			return app.BuildSurgeFromTemplateContent(proxyNodes, replaceTemplateRuntimePlaceholders(r, templateContent))
		}
		configContent, err := app.BuildSurgeFromDefaultTemplate(proxyNodes)
		if err != nil {
			return "", err
		}
		return replaceTemplateRuntimePlaceholders(r, configContent), nil
	case "sing-box":
		if templateContent != "" {
			return app.BuildSingBoxFromTemplateContent(proxyNodes, replaceTemplateRuntimePlaceholders(r, templateContent))
		}
		configContent, err := app.BuildSingBoxFromDefaultTemplate(proxyNodes)
		if err != nil {
			return "", err
		}
		return replaceTemplateRuntimePlaceholders(r, configContent), nil
	default:
		if templateContent != "" {
			return app.BuildYAMLFromTemplateContent(proxyNodes, replaceTemplateRuntimePlaceholders(r, templateContent), target)
		}
		configContent, err := app.BuildYAMLFromDefaultTemplate(proxyNodes, target)
		if err != nil {
			return "", err
		}
		return replaceTemplateRuntimePlaceholders(r, configContent), nil
	}
}

func writeConfigResponse(w http.ResponseWriter, r *http.Request, target, configContent string, nodeCount int) {
	applyConfigResponseHeaders(w, target, nodeCount)
	fmt.Fprint(w, finalizeConfigContent(r, target, configContent))
}

func applyConfigResponseHeaders(w http.ResponseWriter, target string, nodeCount int) {
	meta := configResponseMetaForTarget(target)
	w.Header().Set("Content-Type", meta.contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, meta.filename))
	w.Header().Set("X-Node-Count", fmt.Sprintf("%d", nodeCount))
}

func formatUserInfo(info *database.UserInfo) string {
	s := fmt.Sprintf("upload=%d; download=%d; total=%d", info.Upload, info.Download, info.Total)
	if info.Expire != nil {
		s += fmt.Sprintf("; expire=%d", *info.Expire)
	}
	return s
}

func configResponseMetaForTarget(target string) configResponseMeta {
	switch target {
	case "stash":
		return configResponseMeta{filename: "stash_config.yaml", contentType: "text/yaml; charset=utf-8"}
	case "surge":
		return configResponseMeta{filename: "surge_config.conf", contentType: "text/plain; charset=utf-8"}
	case "sing-box":
		return configResponseMeta{filename: "sing_box_config.json", contentType: "application/json; charset=utf-8"}
	case "clash-mihomo":
		return configResponseMeta{filename: "clash-mihomo_config.yaml", contentType: "text/yaml; charset=utf-8"}
	default:
		return configResponseMeta{filename: "clash-mihomo_config.yaml", contentType: "text/yaml; charset=utf-8"}
	}
}

func resolveConfigTarget(rawTarget, fallback string) (string, error) {
	candidate := strings.TrimSpace(rawTarget)
	if candidate == "" {
		candidate = fallback
	}
	if candidate == "" {
		return "clash-mihomo", nil
	}

	normalized := strings.ToLower(strings.TrimSpace(candidate))
	normalized = strings.ReplaceAll(normalized, "_", "-")

	switch normalized {
	case "clash", "clash-meta", "clash-mihomo":
		return "clash-mihomo", nil
	case "stash":
		return "stash", nil
	case "surge":
		return "surge", nil
	case "sing-box", "singbox":
		return "sing-box", nil
	default:
		return "", fmt.Errorf("不支持的 target: %s", candidate)
	}
}

func templateValidationNodes() []*app.ProxyNode {
	return []*app.ProxyNode{
		{
			Protocol: "trojan",
			Name:     "示例节点",
			Server:   "example.com",
			Port:     443,
			Options: map[string]interface{}{
				"password":         "ruleflow-check",
				"sni":              "example.com",
				"skip-cert-verify": true,
			},
		},
	}
}

func parseConvertRequestParams(r *http.Request) (convertRequestParams, error) {
	if r == nil || r.URL == nil {
		return convertRequestParams{}, nil
	}

	if err := validateConvertTemplateParams(r); err != nil {
		return convertRequestParams{}, err
	}

	rawQuery := strings.TrimSpace(r.URL.RawQuery)
	params := convertRequestParams{}
	segments := strings.Split(rawQuery, "&")
	for i := 0; i < len(segments); i++ {
		key, value, ok := strings.Cut(segments[i], "=")
		if !ok {
			continue
		}

		switch key {
		case "url", "subscription", "subscription_url":
			if isLikelyAbsoluteURL(value) {
				collected := value
				j := i + 1
				for ; j < len(segments); j++ {
					nextKey, _, nextOK := strings.Cut(segments[j], "=")
					if nextOK && isConvertTemplateKey(nextKey) {
						break
					}
					collected += "&" + segments[j]
				}
				params.subURL = strings.TrimSpace(collected)
				i = j - 1
				continue
			}
			params.subURL = decodeQueryValue(value)
		case "template":
			id, err := strconv.ParseInt(strings.TrimSpace(decodeQueryValue(value)), 10, 64)
			if err != nil || id <= 0 {
				return convertRequestParams{}, fmt.Errorf("template 必须是模板 ID")
			}
			params.templateID = id
		case "target":
			params.target = strings.TrimSpace(decodeQueryValue(value))
		}
	}

	if params.subURL == "" {
		params.subURL = strings.TrimSpace(firstNonEmpty(
			r.URL.Query().Get("url"),
			r.URL.Query().Get("subscription"),
			r.URL.Query().Get("subscription_url"),
		))
	}
	if params.target == "" {
		params.target = strings.TrimSpace(r.URL.Query().Get("target"))
	}

	return params, nil
}

func validateConvertTemplateParams(r *http.Request) error {
	if r == nil || r.URL == nil {
		return nil
	}

	query := r.URL.Query()
	if strings.TrimSpace(query.Get("template_id")) != "" {
		return fmt.Errorf("不支持 template_id 参数，请使用 template=模板ID")
	}
	if strings.TrimSpace(query.Get("template_name")) != "" {
		return fmt.Errorf("不支持 template_name 参数，请使用 template=模板ID")
	}
	if raw := strings.TrimSpace(query.Get("template")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return fmt.Errorf("template 必须是模板 ID")
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func isConvertTemplateKey(key string) bool {
	switch key {
	case "template", "template_name", "template_id":
		return true
	default:
		return false
	}
}

func isLikelyAbsoluteURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func decodeQueryValue(value string) string {
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

func buildConvertLogToken(params convertRequestParams) string {
	rawURL := strings.TrimSpace(params.subURL)
	if rawURL == "" {
		switch {
		case params.templateID > 0:
			return fmt.Sprintf("convert:template_id=%d", params.templateID)
		default:
			return "convert"
		}
	}

	if len(rawURL) <= 64 {
		return rawURL
	}
	return rawURL[:61] + "..."
}

// ==================== SQL 执行 ====================

// ExecSQL 执行任意 SQL 语句（仅管理员可用）
func (h *Handlers) ExecSQL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SQL string `json:"sql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	sql := strings.TrimSpace(req.SQL)
	if sql == "" {
		SendError(w, http.StatusBadRequest, "SQL 不能为空")
		return
	}

	ctx := r.Context()

	// SELECT 查询返回结果集
	upper := strings.ToUpper(sql)
	if strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "SHOW") || strings.HasPrefix(upper, "EXPLAIN") {
		rows, err := h.db.Pool.Query(ctx, sql)
		if err != nil {
			SendError(w, http.StatusBadRequest, err.Error())
			return
		}
		defer rows.Close()

		fieldDescs := rows.FieldDescriptions()
		columns := make([]string, len(fieldDescs))
		for i, fd := range fieldDescs {
			columns[i] = string(fd.Name)
		}

		var result []map[string]any
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				SendError(w, http.StatusInternalServerError, err.Error())
				return
			}
			row := make(map[string]any, len(columns))
			for i, col := range columns {
				row[col] = vals[i]
			}
			result = append(result, row)
			if len(result) >= 500 {
				break
			}
		}
		if result == nil {
			result = []map[string]any{}
		}

		SendSuccess(w, map[string]any{
			"type":    "select",
			"columns": columns,
			"rows":    result,
		})
		return
	}

	// DDL / DML
	tag, err := h.db.Pool.Exec(ctx, sql)
	if err != nil {
		SendError(w, http.StatusBadRequest, err.Error())
		return
	}

	SendSuccess(w, map[string]any{
		"type":          "exec",
		"rows_affected": tag.RowsAffected(),
	})
}

// ==================== 数据导入/导出 API ====================

// ExportPayload 导出数据结构
type ExportPayload struct {
	Version        string                   `json:"version"`
	ExportedAt     time.Time                `json:"exported_at"`
	Subscriptions  []database.Subscription  `json:"subscriptions"`
	ManualNodes    []database.Node          `json:"manual_nodes"`
	Templates      []database.Template      `json:"templates"`
	ConfigPolicies []*database.ConfigPolicy `json:"config_policies"`
	RuleSources    []database.RuleSource    `json:"rule_sources"`
}


// ExportData 导出所有数据为 JSON 文件
func (h *Handlers) ExportData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 并发获取所有数据
	type result[T any] struct {
		data T
		err  error
	}

	subsCh := make(chan result[[]database.Subscription], 1)
	nodesCh := make(chan result[[]database.Node], 1)
	tplsCh := make(chan result[[]database.Template], 1)
	policiesCh := make(chan result[[]*database.ConfigPolicy], 1)
	rulesCh := make(chan result[[]database.RuleSource], 1)

	go func() {
		data, err := h.subscriptionService.ListSubscriptions(ctx)
		subsCh <- result[[]database.Subscription]{data, err}
	}()
	go func() {
		data, err := h.nodeService.ListNodes(ctx, database.NodeFilter{ManualOnly: true})
		nodesCh <- result[[]database.Node]{data, err}
	}()
	go func() {
		data, err := h.templateService.ListTemplates(ctx)
		tplsCh <- result[[]database.Template]{data, err}
	}()
	go func() {
		data, err := h.configPolicyService.List(ctx)
		policiesCh <- result[[]*database.ConfigPolicy]{data, err}
	}()
	go func() {
		data, err := h.ruleSourceService.List(ctx)
		rulesCh <- result[[]database.RuleSource]{data, err}
	}()

	subsRes := <-subsCh
	if subsRes.err != nil {
		SendError(w, http.StatusInternalServerError, "获取订阅源失败: "+subsRes.err.Error())
		return
	}
	nodesRes := <-nodesCh
	if nodesRes.err != nil {
		SendError(w, http.StatusInternalServerError, "获取手动节点失败: "+nodesRes.err.Error())
		return
	}
	tplsRes := <-tplsCh
	if tplsRes.err != nil {
		SendError(w, http.StatusInternalServerError, "获取模板失败: "+tplsRes.err.Error())
		return
	}
	policiesRes := <-policiesCh
	if policiesRes.err != nil {
		SendError(w, http.StatusInternalServerError, "获取配置策略失败: "+policiesRes.err.Error())
		return
	}
	rulesRes := <-rulesCh
	if rulesRes.err != nil {
		SendError(w, http.StatusInternalServerError, "获取规则源失败: "+rulesRes.err.Error())
		return
	}

	payload := ExportPayload{
		Version:        "1",
		ExportedAt:     time.Now().UTC(),
		Subscriptions:  subsRes.data,
		ManualNodes:    nodesRes.data,
		Templates:      tplsRes.data,
		ConfigPolicies: policiesRes.data,
		RuleSources:    rulesRes.data,
	}

	// 空切片序列化为 [] 而非 null
	if payload.Subscriptions == nil {
		payload.Subscriptions = []database.Subscription{}
	}
	if payload.ManualNodes == nil {
		payload.ManualNodes = []database.Node{}
	}
	if payload.Templates == nil {
		payload.Templates = []database.Template{}
	}
	if payload.ConfigPolicies == nil {
		payload.ConfigPolicies = []*database.ConfigPolicy{}
	}
	if payload.RuleSources == nil {
		payload.RuleSources = []database.RuleSource{}
	}

	filename := fmt.Sprintf("ruleflow-export-%s.json", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(payload)
}

// ImportData 从 JSON 文件导入数据（支持 multipart/form-data 或直接 JSON body）
