# 实时发现与收口日志

用于记录执行过程中发现的“半实现 / 伪逻辑 / 链路不完整”问题，并在后续统一收口修复。

## 使用规则

- 发现即登记，不延后。
- 每条记录必须可定位到具体代码路径。
- 收口阶段按优先级逐条关闭，关闭时补充修复说明与验证结果。

## 状态定义

- `open`: 已发现，待修复
- `in_progress`: 正在修复
- `closed`: 已修复并验证

## 记录模板

| ID | 发现时间 | 范围 | 问题描述 | 证据路径 | 影响 | 状态 | 收口说明 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| F-003 | 2026-04-28 23:30 | identity/auth | 登录仅做字段校验，未校验 MySQL 账号密码；用户上下文多处仍硬编码 `u-admin` | `services/identity-service/internal/transport/httpserver/server.go` | 认证正确性与越权风险 | closed | 登录改为 MySQL 凭证校验；self/update/follows/followers 改为基于 Redis 会话用户；`go test ./...` 通过 |
| F-004 | 2026-04-28 23:30 | identity/follow | `/api/follow/follow` 与 `/api/follow/unfollow` 映射到同一路由，当前下游未区分取消关注语义 | `services/api-gateway/internal/transport/httpserver/server.go` | 业务语义错误 | closed | 网关向下游透传 `X-Legacy-Path`；identity 按 legacy path 分支 follow/unfollow；网关与 identity 测试通过 |
| F-005 | 2026-04-28 23:30 | content/interactions | 交互接口忽略 DB 错误并恒返回 success（假成功） | `services/content-service/internal/transport/httpserver/server.go` | 数据一致性风险 | closed | interactions 与 tag delete 改为错误透出，不再吞错；`go test ./...` 通过 |
| F-006 | 2026-04-28 23:30 | system/common | `/system/common/*` 多接口固定返回（starttime/vis/isadmin/entry） | `services/system-service/internal/transport/httpserver/server.go` | 伪逻辑 | closed | starttime 使用服务启动时刻，vis 基于 MySQL 计数，entry 读 DB 设置，isadmin 非固定值；`go test ./...` 通过 |
| F-007 | 2026-04-28 23:30 | analytics | `/analytics/overview` 与 `/analytics/history/list` 为固定/空返回 | `services/analytics-service/internal/*` | 统计链路不真实 | closed | 新增 analytics MySQL 仓储与聚合查询，overview/history 均由 DB 返回；`go test ./...` 通过 |
| F-001 | 2026-04-27 16:20 | system-service/configs | `/system/configs` 仍先走内存服务再用 DB 覆盖，存在双通路与伪兜底，不是单一真实链路 | `services/system-service/internal/transport/httpserver/server.go` | 数据一致性、可维护性 | closed | 已改为仅通过 MySQL repo 返回配置，移除内存兜底路径，`go test ./...` 通过 |
| F-002 | 2026-04-27 16:38 | identity/content/system | 当前仅完成 SQLite->MySQL，Redis 仍未接入会话/验证码/热点缓存链路，未完全满足 MySQL+Redis 基线 | `services/*/internal/transport/httpserver/*.go` | 性能与一致性风险 | closed | identity 已接入 Redis 会话/验证码，system/content 已接入 Redis 读缓存与写后失效；三服务 `go test ./...` 通过 |
| F-008 | 2026-04-28 23:55 | content/user-context | 多个 content 交互接口仍硬编码 `u-admin`，非真实请求上下文 | `services/content-service/internal/transport/httpserver/server.go` | 数据隔离与权限风险 | closed | 二次复核补齐 `commentLike/commentDislike` 的用户解析，content 全量测试通过 |
| F-009 | 2026-04-28 23:55 | system/file-upload | 文件上传未落盘且忽略 DB 保存错误，属于伪链路 | `services/system-service/internal/transport/httpserver/server.go` | 功能正确性风险 | closed | 增加本地落盘（`uploads/`）与 DB 错误显式处理；`go test ./...` 通过 |
| F-010 | 2026-04-28 23:55 | identity/code | 登录验证码生成策略过弱（时间可预测） | `services/identity-service/internal/transport/httpserver/server.go` | 安全风险 | closed | 改为 `crypto/rand` 随机 6 位验证码，保留 Redis TTL 校验；`go test ./...` 通过 |
| F-011 | 2026-04-29 01:58 | web/detail-follow-editor | 文章详情/编辑器/关注页缺少统一会话头与错误处理，联调稳定性不足 | `apps/web/src/views/*.vue` | 前后端联调不稳定 | closed | 已补 token+userId 透传、异常提示与状态反馈，页面可直接联调 |
| F-012 | 2026-04-29 02:14 | web-admin/ui-state | 文章列表/管理总览/站点配置缺少统一加载态与错误态，交互一致性不足 | `apps/web/src/views/ArticleListPage.vue`, `apps/admin/src/views/*.vue` | 体验一致性风险 | closed | 已补齐 loading/empty/error/hint 状态与按钮禁用逻辑，联调反馈闭环完整 |
| F-013 | 2026-04-29 02:32 | web-admin/app-shell | web/admin 导航壳缺少当前路由高亮与快捷入口，主流程切换成本高 | `apps/web/src/App.vue`, `apps/admin/src/App.vue` | 导航效率与一致性风险 | closed | 已补路由高亮、登录态入口切换与快捷入口，壳层交互一致 |
| F-014 | 2026-04-29 02:48 | web-admin/visual-copy | ArticleDetail/EditArticle/Register/UserPermission 页面按钮层级、提示语、间距样式不统一 | `apps/web/src/views/*.vue`, `apps/admin/src/views/UserPermissionPage.vue` | 复刻一致性风险 | closed | 已统一 panel 布局、按钮样式、hint/error 文案与交互反馈 |
| F-015 | 2026-04-29 03:10 | web-admin/remaining-pages | TagList/CategoryList/Dashboard/AdminRedirect/TagManage/CategoryManage 缺少统一状态闭环或交互细节 | `apps/web/src/views/*.vue`, `apps/admin/src/views/*.vue` | 复刻完成度风险 | closed | 六个页面均已补 loading/empty/error/hint、确认交互与统一样式，剩余清零 |

## Findings

| ID | 发现时间 | 范围 | 问题描述 | 证据路径 | 影响 | 状态 | 收口说明 |
| --- | --- | --- | --- | --- | --- | --- | --- |
