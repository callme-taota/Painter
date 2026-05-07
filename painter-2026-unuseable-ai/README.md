# Painter-2026

Painter-2026 是面向未来三年的全新重构版本，采用 **Go 微服务 + Vue 前端** 架构。

## 基础设施基线

- 默认生产栈：`MySQL + Redis`
- 按场景扩展：`PostgreSQL`（特定关系能力）、`MQ`（异步解耦链路）
- 非经明确批准，不使用 SQLite 作为业务持久化链路

## Monorepo 结构

- `apps/web`: 门户前端（Vue3 + Vite）
- `apps/admin`: 管理后台（Vue3 + Vite）
- `services/api-gateway`: 网关/BFF
- `services/identity-service`: 认证鉴权与用户域
- `services/content-service`: 文章内容域
- `services/system-service`: 配置与系统域
- `services/analytics-service`: 统计与分析域
- `packages/openapi`: 契约优先定义
- `packages/sdk-ts`: 前端 SDK 生成产物
- `packages/ui`: 共享设计令牌
- `deploy`: 本地部署与环境样例
- `docs`: VitePress 完整说明书

## 快速开始

1. 进入目录：`cd painter-2026`
2. 阅读架构文档：`docs/architecture/current-audit.md` 与 `docs/architecture/target-architecture.md`
3. 启动微服务：
   - `go run ./services/identity-service/cmd/identity`
   - `go run ./services/content-service/cmd/content`
   - `go run ./services/system-service/cmd/system`
   - `go run ./services/analytics-service/cmd/analytics`
4. 启动网关：`go run ./services/api-gateway/cmd/gateway`
5. 启动前端（npm workspace）：
   - Web: `npm run dev:web`
   - Admin: `npm run dev:admin`

### 一键启动（推荐）

- 启动全套（MySQL、Redis、五个后端服务、web/admin 前端）：
  - `bash ./scripts/dev-up.sh`
- 停止应用进程：
  - `bash ./scripts/dev-down.sh`
- 运行日志目录：
  - `.runtime/logs`

## 测试与校验

- 后端测试：`go test ./...`（在 `services/*` 下执行）
- 契约校验：`npm run contract:check`
- 全量验证：`npm run verify`

## 文档站

- 本地开发：`npm run docs:dev`
- 构建：`npm run docs:build`

## 部署与构建

- 本地 compose：`deploy/docker-compose.yml`
- 服务 Dockerfile：
  - `deploy/docker/api-gateway.Dockerfile`
  - `deploy/docker/identity-service.Dockerfile`
  - `deploy/docker/content-service.Dockerfile`
  - `deploy/docker/system-service.Dockerfile`
  - `deploy/docker/analytics-service.Dockerfile`
- 详细手册请看：
  - `docs/deploy/overview.md`
  - `docs/deploy/build-release.md`
  - `docs/deploy/rollback.md`

## 设计原则

- 契约优先：OpenAPI 是前后端唯一事实来源
- 领域边界清晰：服务按 identity/content/system/analytics 拆分
- 配置驱动：功能开关、字典、主题与安全策略后台可配置
- 可观测优先：日志、指标、链路追踪内建
- 渐进迁移：保留旧系统并行，灰度切流与可回滚
