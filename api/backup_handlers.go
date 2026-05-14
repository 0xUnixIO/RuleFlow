package api

import (
	"encoding/json"
	"net/http"

	"github.com/ablate-ai/RuleFlow/database"
	"github.com/ablate-ai/RuleFlow/services"
)


// BackupHandlers 备份相关接口
type BackupHandlers struct {
	svc *services.BackupService
}

func NewBackupHandlers(svc *services.BackupService) *BackupHandlers {
	return &BackupHandlers{svc: svc}
}

// GetSettings GET /api/backup/settings
func (h *BackupHandlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.GetSettings(r.Context())
	if err != nil {
		SendError(w, http.StatusInternalServerError, "读取备份配置失败: "+err.Error())
		return
	}
	// 脱敏：隐藏 secret key
	resp := map[string]interface{}{
		"enabled":             settings.Enabled,
		"r2_account_id":       settings.R2AccountID,
		"r2_access_key_id":    settings.R2AccessKeyID,
		"r2_secret_access_key": maskSecret(settings.R2SecretAccessKey),
		"r2_bucket_name":      settings.R2BucketName,
		"updated_at":          settings.UpdatedAt,
	}
	SendSuccess(w, resp)
}

// SaveSettings PUT /api/backup/settings
func (h *BackupHandlers) SaveSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled           bool   `json:"enabled"`
		R2AccountID       string `json:"r2_account_id"`
		R2AccessKeyID     string `json:"r2_access_key_id"`
		R2SecretAccessKey string `json:"r2_secret_access_key"`
		R2BucketName      string `json:"r2_bucket_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	// 如果 secret 传的是脱敏占位符，保留数据库中原值
	settings := &database.BackupSettings{
		Enabled:      req.Enabled,
		R2AccountID:  req.R2AccountID,
		R2AccessKeyID: req.R2AccessKeyID,
		R2BucketName: req.R2BucketName,
	}
	if req.R2SecretAccessKey != "" && req.R2SecretAccessKey != "••••••••" {
		settings.R2SecretAccessKey = req.R2SecretAccessKey
	} else {
		// 保留原值
		existing, err := h.svc.GetSettings(r.Context())
		if err == nil {
			settings.R2SecretAccessKey = existing.R2SecretAccessKey
		}
	}

	if err := h.svc.SaveSettings(r.Context(), settings); err != nil {
		SendError(w, http.StatusInternalServerError, "保存备份配置失败: "+err.Error())
		return
	}
	SendSuccess(w, map[string]bool{"saved": true})
}

// TestConnection POST /api/backup/test
// body 与 SaveSettings 相同结构，直接用表单中的值测试（不必先保存）
func (h *BackupHandlers) TestConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		R2AccountID       string `json:"r2_account_id"`
		R2AccessKeyID     string `json:"r2_access_key_id"`
		R2SecretAccessKey string `json:"r2_secret_access_key"`
		R2BucketName      string `json:"r2_bucket_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	secretKey := req.R2SecretAccessKey
	if secretKey == "" || secretKey == "••••••••" {
		existing, err := h.svc.GetSettings(r.Context())
		if err == nil {
			secretKey = existing.R2SecretAccessKey
		}
	}

	settings := &database.BackupSettings{
		R2AccountID:       req.R2AccountID,
		R2AccessKeyID:     req.R2AccessKeyID,
		R2SecretAccessKey: secretKey,
		R2BucketName:      req.R2BucketName,
	}
	if err := h.svc.TestConnection(r.Context(), settings); err != nil {
		SendError(w, http.StatusBadGateway, "连接 R2 失败: "+err.Error())
		return
	}
	SendSuccess(w, map[string]bool{"ok": true})
}

// TriggerBackup POST /api/backup/trigger
func (h *BackupHandlers) TriggerBackup(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.RunBackup(r.Context()); err != nil {
		SendError(w, http.StatusInternalServerError, "备份失败: "+err.Error())
		return
	}
	SendSuccess(w, map[string]bool{"triggered": true})
}

// ListR2Objects GET /api/backup/r2-objects
func (h *BackupHandlers) ListR2Objects(w http.ResponseWriter, r *http.Request) {
	objects, err := h.svc.ListR2Objects(r.Context())
	if err != nil {
		SendError(w, http.StatusBadGateway, "列出 R2 文件失败: "+err.Error())
		return
	}
	if objects == nil {
		objects = []*services.R2Object{}
	}
	SendSuccess(w, objects)
}

// Restore POST /api/backup/restore  body: {"file_key":"backup-xxx.tar.gz"}
func (h *BackupHandlers) Restore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FileKey string `json:"file_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FileKey == "" {
		SendError(w, http.StatusBadRequest, "缺少 file_key")
		return
	}
	if err := h.svc.RestoreFromKey(r.Context(), req.FileKey); err != nil {
		SendError(w, http.StatusInternalServerError, "恢复失败: "+err.Error())
		return
	}
	SendSuccess(w, map[string]bool{"restored": true})
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	return "••••••••"
}
