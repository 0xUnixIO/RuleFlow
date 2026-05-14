# 开发指南

## 前置要求

- **Go** 1.24+
- **Bun** — 前端构建和包管理
- **PostgreSQL** 数据库
- **Redis**（可选，禁用时无缓存）
- **psql** 客户端（用于初始化数据库）

## 本地环境搭建

```bash
# 1. 复制并编辑环境变量
cp .env.example .env
# 填写 DATABASE_URL、REDIS_ADDR、ADMIN_PASSWORD 等

# 2. 初始化数据库
make migrate

# 3. 启动服务
make run
```

内置规则模板位于 `template/clash.yaml`、`template/surge.conf`、`template/sing-box.json`。

## 后端开发

```bash
make help        # 查看所有可用命令
make build       # 编译（含前端构建）
make run         # 启动服务（读取 .env）
make test        # 运行测试
```

手动运行测试：

```bash
GOCACHE=$(pwd)/.cache/go-build go test ./...
```

## 前端开发

```bash
cd web-ui
bun install       # 安装依赖
bun run dev       # 启动开发服务器（自动代理 API 到后端）
bun run build     # 生产构建 → dist/（由 Go embed 打包）
```

## 技术栈

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

## 项目结构

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
├── database/                        # 数据访问层
├── cache/                           # 缓存层
├── config/                          # 配置加载
├── web-ui/                          # React SPA 前端
│   ├── src/
│   └── dist/                        # 构建产物 → Go embed
├── template/                           # 内置规则模板
├── migrations/                      # 数据库迁移脚本
├── deploy/                          # 部署配置
│   ├── docker-compose.yaml
│   └── Caddyfile
├── Dockerfile
├── Makefile
└── .env.example
```

## 贡献

参见 [CONTRIBUTING.md](CONTRIBUTING.md)。
