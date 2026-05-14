package main

import (
	"context"
	"encoding/json"
	"fmt"
	iofs "io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/ablate-ai/RuleFlow/api"
	"github.com/ablate-ai/RuleFlow/cache"
	"github.com/ablate-ai/RuleFlow/config"
	"github.com/ablate-ai/RuleFlow/database"
	"github.com/ablate-ai/RuleFlow/services"
)

// version 由构建时 ldflags 注入：-X main.version=vX.Y.Z
var version = "dev"

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化数据库和 Redis（数据库必需，Redis 可选）
	var redisClient *cache.Client
	var subscriptionService *services.SubscriptionService

	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		log.Fatal("❌ DATABASE_URL 未设置，RuleFlow 需要 PostgreSQL 才能启动")
	}

	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("❌ 数据库连接失败: %v\n", err)
	}
	log.Printf("✅ 数据库连接成功\n")
	defer db.Close()

	if err := database.RunMigrations(context.Background(), db.Pool, migrationsFS); err != nil {
		log.Fatalf("❌ 数据库迁移失败: %v\n", err)
	}

	// 尝试连接 Redis
	if os.Getenv("REDIS_ADDR") != "" {
		var err error
		redisClient, err = cache.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
		if err != nil {
			log.Printf("⚠️ Redis 连接失败（将使用无缓存模式）: %v\n", err)
			redisClient = nil // 确保 redisClient 为 nil，避免后续空指针引用
		} else {
			log.Printf("✅ Redis 连接成功\n")
			defer redisClient.Close()
		}
	}

	// 创建订阅服务（数据库可用即可；Redis 仅用于策略配置缓存）
	var templateService *services.TemplateService
	var configPolicyService *services.ConfigPolicyService
	var ruleSourceService *services.RuleSourceService
	var nodeService *services.NodeService
	var maintenanceService *services.MaintenanceService
	var subscriptionSyncService *services.SubscriptionSyncService
	var ruleSourceSyncService *services.RuleSourceSyncService
	var subscriptionCache *cache.SubscriptionCache
	if redisClient != nil {
		subscriptionCache = cache.NewSubscriptionCache(redisClient, time.Duration(cfg.CacheTTLSeconds)*time.Second)
	}
	subscriptionRepo := database.NewSubscriptionRepo(db)
	nodeRepo := database.NewNodeRepo(db)
	subscriptionService = services.NewSubscriptionService(subscriptionRepo, subscriptionCache, nodeRepo)

	// 创建模板服务
	templateRepo := database.NewTemplateRepo(db)
	templateService = services.NewTemplateService(templateRepo)

	// 创建配置策略服务
	configPolicyRepo := database.NewConfigPolicyRepo(db)
	configAccessLogRepo := database.NewConfigAccessLogRepo(db)
	configPolicyService = services.NewConfigPolicyService(configPolicyRepo, configAccessLogRepo, subscriptionRepo, nodeRepo)

	// 创建规则源服务
	ruleSourceRepo := database.NewRuleSourceRepo(db)
	ruleSourceService = services.NewRuleSourceService(ruleSourceRepo)
	ruleSourceSyncService = services.NewRuleSourceSyncService(ruleSourceRepo)

	// 创建节点服务
	nodeService = services.NewNodeService(nodeRepo)
	maintenanceService = services.NewMaintenanceService(db)

	// 创建订阅同步服务
	subscriptionSyncService = services.NewSubscriptionSyncService(subscriptionRepo, nodeRepo, configPolicyRepo, subscriptionCache)

	// 启动自动刷新调度器
	schedulerCtx, cancelScheduler := context.WithCancel(context.Background())
	defer cancelScheduler()
	if subscriptionSyncService != nil {
		scheduler := services.NewSubscriptionScheduler(database.NewSubscriptionRepo(db), subscriptionSyncService)
		scheduler.Start(schedulerCtx)
	}
	if ruleSourceSyncService != nil {
		ruleSourceScheduler := services.NewRuleSourceScheduler(ruleSourceSyncService)
		ruleSourceScheduler.Start(schedulerCtx)
	}
	// 启动日志清理调度器
	logCleanupScheduler := services.NewLogCleanupScheduler(configAccessLogRepo,
		services.WithLogCleanupKeepDays(cfg.LogKeepDays),
		services.WithLogCleanupMaxRecords(cfg.LogMaxRecords),
		services.WithLogCleanupCheckInterval(time.Duration(cfg.LogCheckInterval)*time.Hour),
	)
	logCleanupScheduler.Start(schedulerCtx)

	// 创建备份服务并启动调度器
	backupRepo := database.NewBackupRepo(db)
	backupService := services.NewBackupService(backupRepo, db.Pool)
	backupScheduler := services.NewBackupScheduler(backupService)
	backupScheduler.Start(schedulerCtx)
	backupHandlers := api.NewBackupHandlers(backupService)

	adminUserRepo := database.NewAdminUserRepo(db)

	sessionSecret, err := database.LoadOrCreateSessionSecret(context.Background(), db)
	if err != nil {
		log.Fatalf("❌ 加载 session secret 失败: %v\n", err)
	}

	// 创建 API 处理器
	apiHandlers := api.NewHandlers(subscriptionService, templateService, configPolicyService, ruleSourceService, nodeService, maintenanceService, subscriptionSyncService, ruleSourceSyncService, subscriptionCache, redisClient, db)

	// 启动服务器（backupHandlers 通过 setupRoutes 注册）
	port := cfg.Port
	log.Printf("🚀 RuleFlow %s 启动: http://localhost:%s\n", version, port)
	log.Printf("💡 管理界面: http://localhost:%s/dashboard\n", port)
	log.Printf("💡 管理接口: http://localhost:%s/api/subscriptions\n", port)
	log.Printf("💡 模板接口: http://localhost:%s/api/templates\n", port)
	log.Printf("💡 规则源接口: http://localhost:%s/api/rule-sources\n", port)
	log.Printf("💡 健康检查: http://localhost:%s/health\n", port)

	// 优雅关闭
	r := setupRoutes(cfg, sessionSecret, apiHandlers, backupHandlers, adminUserRepo)
	server := &http.Server{
		Addr:    ":" + port,
		Handler: api.LoggingMiddleware(api.CORSMiddleware(cfg.CORSAllowedOrigins)(api.RecoveryMiddleware(r))),
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("❌ 服务器错误: %v\n", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 收到第二次信号时强制退出
	signal.Reset(syscall.SIGINT, syscall.SIGTERM)

	log.Println("🛑 正在关闭服务器...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("⚠️ 优雅关闭超时，强制关闭: %v\n", err)
		_ = server.Close()
	}

	log.Println("✅ 服务器已关闭")
	os.Exit(0)
}

func setupRoutes(cfg *config.Config, sessionSecret string, apiHandlers *api.Handlers, backupHandlers *api.BackupHandlers, adminUserRepo *database.AdminUserRepo) chi.Router {
	log.Printf("🔒 Web 控制台鉴权已启用\n")

	r := chi.NewRouter()
	webAuth := api.WebAuthMiddleware(sessionSecret)

	// ── SPA 静态资源 ──────────────────────────────────────
	distFS, _ := iofs.Sub(webFS, "web-ui/dist")
	indexHTML, _ := iofs.ReadFile(distFS, "index.html")

	serveSPA := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	fileServer := http.FileServer(http.FS(distFS))
	r.Handle("/assets/*", fileServer)
	r.Handle("/favicon.svg", fileServer)
	r.Handle("/icons.svg", fileServer)

	// ── 初始化设置（首次使用创建管理员）─────────────────
	r.Get("/setup", serveSPA)
	r.Get("/api/setup/status", func(w http.ResponseWriter, r *http.Request) {
		count, err := adminUserRepo.Count(r.Context())
		if err != nil {
			api.SendError(w, http.StatusInternalServerError, "查询失败")
			return
		}
		api.SendSuccess(w, map[string]bool{"setup_required": count == 0})
	})
	r.Post("/api/setup", func(w http.ResponseWriter, req *http.Request) {
		count, err := adminUserRepo.Count(req.Context())
		if err != nil {
			api.SendError(w, http.StatusInternalServerError, "查询失败")
			return
		}
		if count > 0 {
			api.SendError(w, http.StatusConflict, "管理员已存在")
			return
		}
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.Username == "" || body.Password == "" {
			api.SendError(w, http.StatusBadRequest, "用户名和密码不能为空")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			api.SendError(w, http.StatusInternalServerError, "密码加密失败")
			return
		}
		if err := adminUserRepo.Create(req.Context(), body.Username, string(hash)); err != nil {
			api.SendError(w, http.StatusInternalServerError, "创建用户失败")
			return
		}
		api.SendSuccess(w, nil)
	})

	// ── 登录 / 退出 ──────────────────────────────────────
	r.Get("/login", serveSPA)
	r.Post("/login", func(w http.ResponseWriter, req *http.Request) {
		username := req.FormValue("username")
		password := req.FormValue("password")
		next := req.FormValue("next")
		if next == "" {
			next = "/dashboard"
		}
		user, err := adminUserRepo.GetByUsername(req.Context(), username)
		if err != nil || user == nil {
			http.Redirect(w, req, "/login?error=1&next="+next, http.StatusFound)
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
			http.Redirect(w, req, "/login?error=1&next="+next, http.StatusFound)
			return
		}
		api.SetSessionCookie(w, sessionSecret)
		http.Redirect(w, req, next, http.StatusFound)
	})
	r.Get("/logout", func(w http.ResponseWriter, r *http.Request) {
		api.ClearSessionCookie(w)
		http.Redirect(w, r, "/login", http.StatusFound)
	})

	// ── 公开 SPA 路由 ────────────────────────────────────
	r.Get("/converter", serveSPA)

	// ── 受保护 SPA 路由 ──────────────────────────────────
	r.With(webAuth).Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})
	r.With(webAuth).Get("/dashboard", serveSPA)
	r.With(webAuth).Get("/subscriptions", serveSPA)
	r.With(webAuth).Get("/nodes", serveSPA)
	r.With(webAuth).Get("/rule-sources", serveSPA)
	r.With(webAuth).Get("/templates", serveSPA)
	r.With(webAuth).Get("/configs", serveSPA)
	r.With(webAuth).Get("/config-access-logs", serveSPA)
	r.With(webAuth).Get("/data-migration", serveSPA)
	r.With(webAuth).Get("/backup", serveSPA)

	// ── 公开接口（无需鉴权）──────────────────────────────
	r.Get("/subscribe", apiHandlers.GenerateConfig)
	r.Get("/convert", apiHandlers.ConvertSubscription)
	r.Get("/api/templates/public", apiHandlers.ListPublicTemplates)
	r.Get("/api/templates/public/{id}", apiHandlers.GetPublicTemplate)
	r.Get("/health", apiHandlers.Health)
	r.Get("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"version":%q}`, version)
	})
	r.Get("/rulesets/{name}", apiHandlers.ExportRuleSource)

	// ── 鉴权检查接口 ──────────────────────────────────────
	r.Get("/api/auth/check", func(w http.ResponseWriter, r *http.Request) {
		valid := api.ValidateSession(r, sessionSecret)
		api.SendSuccess(w, map[string]bool{"authenticated": valid})
	})

	// ── API 路由（整体加鉴权）────────────────────────────
	r.With(api.APIAuthMiddleware(sessionSecret)).Route("/api", func(r chi.Router) {
		r.Post("/cache/policies/clear", apiHandlers.ClearAllPolicyCache)
		r.Post("/admin/migrate-snowflake-ids", apiHandlers.MigrateSnowflakeIDs)
		r.Post("/admin/exec-sql", apiHandlers.ExecSQL)

		// 订阅管理
		r.Get("/subscriptions", apiHandlers.ListSubscriptions)
		r.Post("/subscriptions", apiHandlers.CreateSubscription)
		r.Post("/subscriptions/sync", apiHandlers.SyncAllSubscriptions)
		r.Route("/subscriptions/{id}", func(r chi.Router) {
			r.Get("/", apiHandlers.GetSubscription)
			r.Put("/", apiHandlers.UpdateSubscription)
			r.Patch("/", apiHandlers.UpdateSubscription)
			r.Delete("/", apiHandlers.DeleteSubscription)
			r.Post("/sync", apiHandlers.SyncSubscription)
			r.Get("/sync-status", apiHandlers.GetSubscriptionSyncStatus)
		})

		// 模板管理
		r.Get("/templates", apiHandlers.ListTemplates)
		r.Post("/templates", apiHandlers.CreateTemplate)
		r.Post("/templates/validate", apiHandlers.ValidateTemplate)
		r.Route("/templates/{id}", func(r chi.Router) {
			r.Get("/", apiHandlers.GetTemplate)
			r.Put("/", apiHandlers.UpdateTemplate)
			r.Patch("/", apiHandlers.UpdateTemplate)
			r.Delete("/", apiHandlers.DeleteTemplate)
		})

		// 规则源管理
		r.Get("/rule-sources", apiHandlers.ListRuleSources)
		r.Post("/rule-sources", apiHandlers.CreateRuleSource)
		r.Route("/rule-sources/{id}", func(r chi.Router) {
			r.Get("/", apiHandlers.GetRuleSource)
			r.Put("/", apiHandlers.UpdateRuleSource)
			r.Patch("/", apiHandlers.UpdateRuleSource)
			r.Delete("/", apiHandlers.DeleteRuleSource)
			r.Post("/sync", apiHandlers.SyncRuleSource)
		})

		// 配置策略管理
		r.Get("/config-policies", apiHandlers.ListConfigPolicies)
		r.Get("/config-access-logs", apiHandlers.ListAllConfigAccessLogs)
		r.Post("/config-policies", apiHandlers.CreateConfigPolicy)
		r.Route("/config-policies/{id}", func(r chi.Router) {
			r.Get("/", apiHandlers.GetConfigPolicy)
			r.Put("/", apiHandlers.UpdateConfigPolicy)
			r.Patch("/", apiHandlers.UpdateConfigPolicy)
			r.Delete("/", apiHandlers.DeleteConfigPolicy)
			r.Delete("/cache", apiHandlers.ClearPolicyConfigCache)
			r.Get("/access-logs", apiHandlers.ListConfigPolicyAccessLogs)
		})

		// 数据导入/导出
		r.Get("/export", apiHandlers.ExportData)

		// 数据库备份
		r.Get("/backup/settings", backupHandlers.GetSettings)
		r.Put("/backup/settings", backupHandlers.SaveSettings)
		r.Post("/backup/test", backupHandlers.TestConnection)
		r.Post("/backup/trigger", backupHandlers.TriggerBackup)
r.Get("/backup/r2-objects", backupHandlers.ListR2Objects)
		r.Post("/backup/restore", backupHandlers.Restore)

		// 节点管理
		r.Get("/nodes", apiHandlers.ListNodes)
		r.Post("/nodes", apiHandlers.CreateNode)
		r.Get("/nodes/stats", apiHandlers.GetNodeStats)
		r.Post("/nodes/import", apiHandlers.ImportNodes)
		r.Post("/nodes/batch", apiHandlers.BatchNodeOperation)
		r.Patch("/nodes/batch", apiHandlers.BatchNodeOperation)
		r.Route("/nodes/{id}", func(r chi.Router) {
			r.Get("/", apiHandlers.GetNode)
			r.Get("/share", apiHandlers.GetNodeShareURL)
			r.Put("/", apiHandlers.UpdateNode)
			r.Patch("/", apiHandlers.UpdateNode)
			r.Delete("/", apiHandlers.DeleteNode)
		})
	})

	return r
}
