ENV_FILE ?= .env
MIGRATION_FILE := migrations/init.sql
GOFLAGS ?= -buildvcs=false
GOCACHE_DIR ?= $(CURDIR)/.cache/go-build

.PHONY: help env-check migrate build run test release dev web-dev web-build

help:
	@echo "可用命令:"
	@echo "  make dev               # 一键启动开发环境（后端 + 前端热重建）"
	@echo "  make run               # 读取 .env 后启动后端服务"
	@echo "  make web-dev           # 启动前端开发服务器（需后端运行）"
	@echo "  make web-build         # 构建前端（minified 生产版本）"
	@echo "  make build             # 编译 Go 二进制（含前端）"
	@echo "  make migrate           # 读取 .env 并初始化数据库"
	@echo "  make test              # 运行测试"
	@echo "  make release V=x.y.z   # 打 tag 并推送，触发 GitHub Actions 发布"

env-check:
	@if [ ! -f "$(ENV_FILE)" ]; then \
		echo "缺少 $(ENV_FILE)"; \
		exit 1; \
	fi

migrate: env-check
	@bash -lc 'set -a; source "$(ENV_FILE)"; set +a; test -n "$$DATABASE_URL" || { echo "DATABASE_URL 未设置"; exit 1; }; psql "$$DATABASE_URL" -f "$(MIGRATION_FILE)"'

build: web-build
	GOCACHE=$(GOCACHE_DIR) GOFLAGS="$(GOFLAGS)" go build -o ruleflow .

run: env-check
	@bash -lc 'set -a; source "$(ENV_FILE)"; set +a; GOCACHE="$(GOCACHE_DIR)" GOFLAGS="$(GOFLAGS)" go run .'

test:
	GOCACHE=$(GOCACHE_DIR) GOFLAGS="$(GOFLAGS)" go test ./...

release:
	@CURRENT=$$(git tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$' | head -1); \
	CURRENT=$${CURRENT:-v0.0.0}; \
	MAJOR=$$(echo $$CURRENT | sed 's/v\([0-9]*\)\..*/\1/'); \
	MINOR=$$(echo $$CURRENT | sed 's/v[0-9]*\.\([0-9]*\)\..*/\1/'); \
	PATCH=$$(echo $$CURRENT | sed 's/v[0-9]*\.[0-9]*\.\([0-9]*\)/\1/'); \
	echo "当前版本: $$CURRENT"; \
	echo "  1) patch  -> v$$MAJOR.$$MINOR.$$((PATCH+1))"; \
	echo "  2) minor  -> v$$MAJOR.$$((MINOR+1)).0"; \
	echo "  3) major  -> v$$((MAJOR+1)).0.0"; \
	printf "选择发布类型 [1/2/3]: "; \
	read CHOICE; \
	case $$CHOICE in \
		1) NEW="v$$MAJOR.$$MINOR.$$((PATCH+1))";; \
		2) NEW="v$$MAJOR.$$((MINOR+1)).0";; \
		3) NEW="v$$((MAJOR+1)).0.0";; \
		*) echo "无效选项"; exit 1;; \
	esac; \
	echo "发布版本 $$NEW ..."; \
	git tag $$NEW; \
	git push origin $$NEW; \
	echo "已推送 tag $$NEW，GitHub Actions 开始构建"

web-dev:
	cd web-ui && bun run dev

web-build:
	cd web-ui && bun run build

dev: env-check
	@echo "🚀 启动开发环境：后端 :8080 + 前端 :3000"
	@bash -lc 'set -a; source "$(ENV_FILE)"; set +a; \
		GOCACHE="$(GOCACHE_DIR)" GOFLAGS="$(GOFLAGS)" go run . & GO_PID=$$!; \
		cd web-ui && PORT=3000 bun run dev & WEB_PID=$$!; \
		trap "kill $$GO_PID $$WEB_PID 2>/dev/null; exit" INT TERM; \
		wait'
