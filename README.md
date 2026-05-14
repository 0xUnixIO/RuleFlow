<div align="center">

# RuleFlow

**自托管订阅转换服务** — 将代理订阅转换为多客户端配置文件

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)

</div>

## 简介

RuleFlow 是一个功能强大的自托管订阅转换服务，可将代理订阅源转换为多种主流代理客户端的配置文件。无论是个人使用还是团队部署，RuleFlow 都提供了灵活的订阅管理、节点过滤和规则模板功能。

### 核心能力

- **多协议支持** — Trojan、VMess、VLESS、Shadowsocks、Hysteria2、TUIC v5、WireGuard
- **多客户端输出** — Clash Meta、Stash、Surge、Sing-Box
- **智能节点管理** — 正则过滤、分组管理、链式代理
- **规则模板引擎** — 支持 YAML/INI 模板，灵活配置分流规则
- **可视化控制台** — React SPA，Bun 构建，shadcn/ui + Tailwind CSS
- **高性能架构** — Redis 缓存、并发同步、定时任务

## ✨ 功能特性

### 客户端支持
- **Clash Meta** — 完整 YAML 配置，支持 proxy-groups、rules 等高级特性
- **Stash** — iOS/macOS 平台优化配置
- **Surge** — INI 格式，支持 `#!MANAGED-CONFIG` 远程更新
- **Sing-Box** — 通用代理配置格式

### 协议解析
- Trojan、VMess、VLESS
- Shadowsocks（含 AEAD 加密）
- Hysteria2、TUIC v5
- WireGuard

### 高级特性
- **规则模板** — 自定义 YAML/INI 模板，自动注入节点
- **节点过滤** — 正则表达式精准匹配节点
- **链式代理** — 一键配置中转落地链路
- **规则源管理** — 内置规则集同步与缓存
- **访问日志** — 配置访问记录与统计分析

### 运维能力
- **Web 管理控制台** — 可视化管理订阅、节点、模板、配置策略
- **REST API** — 完整 CRUD 接口，支持程序化集成
- **Redis 缓存** — 配置缓存加速，减轻订阅源压力
- **定时同步** — 自动拉取和刷新订阅源与规则集
- **鉴权保护** — Session 鉴权保护管理接口

---

## 🚀 快速开始

### 前置要求

- **Go** 1.24 或更高版本
- **Bun** — 前端构建和包管理
- **PostgreSQL** 数据库（必需）
- **Redis**（可选，用于缓存）
- **psql** 客户端（用于数据库初始化）

### 方式一：本地运行

```bash
# 1. 复制环境变量配置
cp .env.example .env
# 编辑 .env，填写数据库、Redis 连接信息和管理密码

# 2. 初始化数据库
make migrate

# 3. 启动服务
make run
```

内置模板位于 `rules/clash.yaml` 和 `rules/surge.conf`；也可以在 Web 控制台中上传自定义模板。

### 方式二：一键安装（Linux 服务器）

> 依赖：`docker`、`docker compose`，需以 **root** 身份运行。

```bash
curl -fsSL https://raw.githubusercontent.com/ablate-ai/RuleFlow/main/install.sh | sh
```

安装脚本会自动完成：

1. 检测 `PORT`（默认 8080）、`5432`、`6379` 端口占用，冲突时交互询问
2. 下载 `docker-compose.yaml`、`migrations/init.sql`、`uninstall.sh` 到 `$HOME/ruleflow/`
3. 初始化 `$HOME/ruleflow/.env`（含数据库、Redis 连接信息）
4. 下载适合当前架构（amd64 / arm64）的预编译二进制
5. 用 Docker Compose 启动 PostgreSQL + Redis 基础设施
6. 注册并启动 systemd 服务 `ruleflow`

安装完成后打印真实访问地址，例如：

```
访问地址: http://1.2.3.4:8080
查看日志: journalctl -u ruleflow -f
```

**一键卸载：**

```bash
curl -fsSL https://raw.githubusercontent.com/ablate-ai/RuleFlow/main/uninstall.sh | sh
```

卸载脚本会停止并移除：systemd 服务、Docker 容器/网络/数据卷、二进制文件、安装目录。**PostgreSQL 数据将被清除。**

**自定义安装目录：**

```bash
curl -fsSL https://raw.githubusercontent.com/ablate-ai/RuleFlow/main/install.sh | sudo RULEFLOW_DIR=/opt/ruleflow sh
curl -fsSL https://raw.githubusercontent.com/ablate-ai/RuleFlow/main/uninstall.sh | sudo RULEFLOW_DIR=/opt/ruleflow sh
```

---

## 🛠️ 技术栈

| 层 | 技术 |
|----|------|
| **后端** | Go 1.24+, PostgreSQL, Redis |
| **前端运行时/构建** | Bun (原生 bundler) |
| **UI 框架** | React 19, TypeScript |
| **组件库** | shadcn/ui (Radix UI) |
| **样式** | Tailwind CSS v4 |
| **路由** | React Router v7 |
| **状态管理** | TanStack Query |
| **代码编辑器** | CodeMirror 6 |
| **图标** | Lucide React |

---

## 📖 使用流程

### 基础用法

1. **添加订阅源**：在控制台「订阅源」页面填入订阅 URL
2. **同步节点**：点击同步按钮拉取节点列表
3. **配置规则源**（可选）：添加规则集订阅源，自动同步分流规则
4. **选择或上传模板**：使用内置模板或在「规则模板」页面上传自定义模板
5. **创建配置策略**：绑定订阅源 + 模板 + 目标客户端，生成专属订阅链接
6. **在客户端中使用**：将生成的 `/subscribe?token=xxx` 链接填入客户端

### 高级用法

- **链式代理**：在模板中使用 `dialer-proxy` 配置中转落地
- **节点过滤**：使用正则表达式精准筛选节点
- **访问监控**：查看配置访问日志，分析使用情况

### 订阅链接格式

```
http://your-server:8080/subscribe?token=<token>
```

Surge 客户端会自动识别响应中的 `#!MANAGED-CONFIG` 头，支持远程更新。

---

## 🔧 规则模板

### Clash Meta / Stash 模板（YAML）

在 `proxy-groups` 中支持两个扩展字段，生成配置时自动处理并从输出中删除：

`url` 和 `benchmark-url` 可混写；生成时会按目标客户端自动规范化：Clash Meta 输出 `url`，Stash 输出 `benchmark-url`。

#### `filter` — 节点过滤

```yaml
proxy-groups:
  - name: 🇸🇬 新加坡
    type: url-test
    filter: "SG|新加坡|Singapore"    # 正则，仅匹配的节点进入该组
    proxies: ["__NODES__"]
    url: http://cp.cloudflare.com/generate_204
    interval: 300
```

#### `exclude-filter` — 排除节点

```yaml
  - name: 🇸🇬 新加坡（直连）
    type: url-test
    filter: "SG|新加坡"
    exclude-filter: "IPLC|BGP|中转"  # 在 filter 结果中再排除
    proxies: ["__NODES__"]
```

#### `dialer-proxy` — 链式代理（中转落地）

```yaml
  - name: 🇺🇸 美国 via 新加坡
    type: select
    filter: "US|美国"
    dialer-proxy: "SG|新加坡"        # 正则匹配第一个新加坡节点作为中转
    proxies: ["__NODES__"]
```

生成效果：

```yaml
proxies:
  - name: 🇸🇬 SG-01
    type: trojan
    ...                              # 中转节点，无 dialer-proxy
  - name: 🇺🇸 US-01
    type: vmess
    dialer-proxy: 🇸🇬 SG-01         # 自动注入
    ...
```

### Surge 模板（INI）

```ini
[Proxy]
__NODES__

[Proxy Group]
🇸🇬 SG = url-test, __NODES__, policy-regex-filter=SG|新加坡, url=http://cp.cloudflare.com/generate_204, interval=300
🤖 AI = select, __NODES__, policy-regex-filter=US|美国, exclude-filter=IPLC|BGP, dialer-proxy=🇸🇬 SG

[Rule]
RULE-SET,https://ruleset.skk.moe/Clash/non_ip/ai.txt,🤖 AI
FINAL,🇸🇬 SG
```

Surge 模板使用 `policy-regex-filter=` 按正则筛选节点。
生成后 `policy-regex-filter=`、`exclude-filter=`、`dialer-proxy=` 均不保留；`dialer-proxy` 会被翻译为节点行的 `underlying-proxy=` 参数。

> **注意**：`dialer-proxy` 只会作用到该组最终展开出的实际节点。
> 中转目标会优先按组名匹配，找不到时再按节点名匹配；如果整组只引用其他组、不直接包含节点，则不会给任何节点注入 `underlying-proxy=`。

---

## 📡 API 参考

设置 `ADMIN_PASSWORD` 后，`/api/*` 接口需登录后方可访问（Cookie session）。

| 资源 | 方法 | 路径 | 说明 |
|------|------|------|------|
| **订阅源** |
| 订阅源 | CRUD | `/api/subscriptions` | 管理订阅源 |
| 订阅同步 | POST | `/api/subscriptions/{id}/sync` | 手动同步节点 |
| **节点** |
| 节点 | CRUD + 批量导入 | `/api/nodes` | 管理节点 |
| **规则模板** |
| 规则模板 | CRUD | `/api/templates` | 管理配置模板 |
| 模板检测 | POST | `/api/templates/lint` | 检测模板语法 |
| **配置策略** |
| 配置策略 | CRUD | `/api/config-policies` | 管理输出策略 |
| 清除缓存 | DELETE | `/api/config-policies/{id}/cache` | 清除策略缓存 |
| **规则源** |
| 规则源 | CRUD | `/api/rule-sources` | 管理规则集源 |
| 规则同步 | POST | `/api/rule-sources/{id}/sync` | 同步规则集 |
| **访问日志** |
| 访问日志 | 查询 | `/api/config-access-logs` | 查询访问记录 |
| **订阅分发** |
| 生成配置 | GET | `/subscribe?token=xxx` | 获取客户端配置 |
| 健康检查 | GET | `/health` | 服务健康状态 |

---

## ⚙️ 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| **服务配置** |
| `PORT` | HTTP 服务端口 | `8080` |
| `ADMIN_PASSWORD` | 控制台登录密码；为空则不启用鉴权 | 空 |
| `CORS_ALLOWED_ORIGINS` | CORS 允许来源（逗号分隔） | `*` |
| **数据库** |
| `DATABASE_URL` | PostgreSQL 连接串（必需） | - |
| **缓存** |
| `REDIS_ADDR` | Redis 地址；为空时禁用缓存 | `localhost:6379` |
| `REDIS_PASSWORD` | Redis 密码 | 空 |
| `REDIS_DB` | Redis 数据库编号 | `0` |
| `CACHE_TTL_SECONDS` | 配置缓存有效期（秒） | `3600` |
| **外部访问** |
| `PUBLIC_BASE_URL` | 对外访问基地址，用于补全相对路径 | 空 |
| `SURGE_MANAGED_CONFIG_BASE_URL` | Surge `#!MANAGED-CONFIG` 更新地址 | 空 |
| **日志清理** |
| `LOG_KEEP_DAYS` | 访问日志保留天数 | `30` |
| `LOG_MAX_RECORDS` | 访问日志最大记录数 | `10000` |
| `LOG_CHECK_INTERVAL` | 日志清理检查间隔（小时） | `1` |

---

## 🗄️ 数据库初始化

```bash
# 推荐（自动读取 .env）
make migrate

# 手动执行
psql "$DATABASE_URL" -f migrations/init.sql
```

数据库初始化使用 `migrations/init.sql`；如需升级已有库，请按发布内容执行对应的增量迁移脚本，例如 `migrations/20260315_drop_nodes_source.sql`。

---

## 📦 项目结构

```
RuleFlow/
├── main.go                          # 入口，路由注册
├── internal/app/                    # 核心逻辑
│   ├── parser.go                    # 多协议节点 URL 解析
│   ├── config_builder.go            # Clash Meta / Stash 配置生成
│   ├── surge_builder.go             # Surge INI 配置生成
│   ├── singbox_builder.go           # Sing-Box 配置生成
│   ├── models.go                    # 数据模型
│   ├── subscription.go              # 订阅拉取
│   ├── rule_set.go                  # 规则集管理
│   ├── country_emoji.go             # 节点名称地区 emoji
│   └── wireguard.go                 # WireGuard 配置处理
├── api/                             # HTTP 处理层
│   ├── handlers.go                  # REST API 处理器
│   ├── middleware.go                # 鉴权、CORS
│   ├── template_lint.go             # 模板语法检测
│   └── surge_managed_config.go      # Surge #!MANAGED-CONFIG 支持
├── services/                        # 业务逻辑层
│   ├── subscription_service.go      # 订阅服务
│   ├── subscription_sync_service.go # 订阅同步
│   ├── subscription_scheduler.go    # 订阅定时任务
│   ├── node_service.go              # 节点服务
│   ├── template_service.go          # 模板服务
│   ├── config_policy_service.go     # 配置策略服务
│   ├── rule_source_service.go       # 规则源服务
│   ├── rule_source_sync_service.go  # 规则源同步
│   ├── rule_source_scheduler.go     # 规则源定时任务
│   ├── maintenance_service.go       # 维护服务
│   ├── log_cleanup_scheduler.go     # 日志清理任务
│   └── scheduler_loop.go            # 定时任务调度器
├── database/                        # 数据访问层
│   ├── database.go                  # 数据库连接
│   ├── subscription_repo.go         # 订阅仓储
│   ├── node_repo.go                 # 节点仓储
│   ├── template_repo.go             # 模板仓储
│   ├── config_policy_repo.go        # 策略仓储
│   ├── rule_source_repo.go          # 规则源仓储
│   ├── config_access_log_repo.go    # 访问日志仓储
│   └── snowflake_migration.go       # ID 迁移脚本
├── cache/                           # 缓存层
│   ├── redis.go                     # Redis 客户端
│   └── subscription_cache.go        # 订阅缓存
├── config/                          # 配置加载
├── web-ui/                          # React SPA 前端
│   ├── build.ts                     # Bun 构建脚本
│   ├── dev.ts                       # 开发服务器
│   ├── package.json
│   ├── src/
│   │   ├── main.tsx                 # 入口
│   │   ├── App.tsx                  # 路由 + 布局
│   │   ├── pages/                   # 页面组件
│   │   ├── components/              # UI 组件 (shadcn/ui)
│   │   ├── hooks/                   # React hooks
│   │   ├── lib/                     # API 客户端等工具
│   │   └── types/                   # TypeScript 类型
│   └── dist/                        # 构建产物 → Go embed
├── rules/                           # 内置规则模板
│   ├── clash.yaml
│   ├── surge.conf
│   └── sing-box.json.template
├── migrations/                      # 数据库迁移脚本
├── deploy/                          # 部署配置
│   └── docker-compose.yaml
├── Dockerfile
├── Makefile
├── go.mod
└── .env.example
```

---

## 🧪 开发与测试

### 运行测试

```bash
make test
# 或
GOCACHE=$(pwd)/.cache/go-build go test ./...
```

### 开发命令

```bash
make help        # 查看所有可用命令
make migrate     # 初始化数据库
make build       # 编译程序（含前端构建）
make run         # 启动服务（读取 .env）

# 前端开发
cd web-ui
bun install      # 安装依赖
bun run dev      # 启动开发服务器（代理 API 到后端）
bun run build    # 生产构建 → dist/
```

---

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

### 开发建议

1. 遵循现有代码风格
2. 添加测试覆盖新功能
3. 更新相关文档

---

## 📄 许可证

本项目采用 [MIT License](LICENSE) 开源协议。

Copyright (c) 2026 [ablate-ai](https://github.com/ablate-ai)

---

## 📮 联系方式

- **问题反馈**: [GitHub Issues](https://github.com/ablate-ai/RuleFlow/issues)
- **功能建议**: [GitHub Discussions](https://github.com/ablate-ai/RuleFlow/discussions)
