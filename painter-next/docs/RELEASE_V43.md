# V43：自动备份、账号安全与统一登录

记录日期：2026-11-13。本文是使用与验证索引，详细实现见 [自动备份与恢复](AUTOMATIC_BACKUP_RECOVERY.md)、[账号访问与两步验证](ACCOUNT_SECURITY.md)、[本地认证中台](IDENTITY_PROVIDER.md)。

## 部署记录

2026-11-13 16:41:55（Asia/Shanghai）已完成正式部署验证：

- 认证源码提交 `934090a`；备份功能提交 `812d57c`，调度路径修复 `221fd07`。此前工作已单独保存在 `9dcbcb2`，均为本地提交。
- 正式数据库由 schema 21 迁移到 **23**。仍只有一个 `Taota` 站长账号，密码未改，文章、动态、评论与媒体数量保持不变；未替用户启用 MFA。
- 注册和外部账号登录均为 **关闭**；站长仍可登录。可信 Cloudflare 访客 IP 识别已开启，代理来源严格限定为当前本机地址。
- `http://192.168.2.209:18080/readyz` 与 `https://blog.taota.cc/readyz` 均返回 200；首页、认证选项、OIDC discovery 与公开 JWKS 验证通过。Issuer 固定为 `https://blog.taota.cc/oidc`，只公布 Code、S256 与刷新授权，JWKS 不含私钥。
- 发布前成功备份 `auto-20261113T083903Z-c01dd1a287f4dd35`；新版本发布后成功备份 `auto-20261113T084242Z-e1bbfec9f9f656f0`，后者在 **16:42:44** 完成真实隔离恢复与临时库清理，包含私有配置快照。
- 应用、PostgreSQL、Tunnel 与备份定时器均为 active；备份当前已有 3 份成功快照。原程序与原配置保留为本机私有发布回退材料，没有修改 cloudflared 配置。

部署二进制 SHA256：`2bd532a3f88365faf9e1b49e4ca5658b011a56d5c1d3bc9427f36953f06f0ab8`。机器可读记录保存在 `tests/artifacts/v43/release.json`，不含配置值、密码或令牌。

发布顺序为先备份，再迁移与切换，检查内网和 Tunnel 公网入口，最后使用新版本再完成一次备份验证。恢复切换保留原库、原媒体与原配置，避免失去回退点。

## 自动备份与恢复

- 本机用户 systemd 定时器 `painter-backup.timer` 每天本地时间 **03:30** 起执行，随机延迟最多 10 分钟；停机错过后恢复运行可补做。
- 默认保留最近 **14 份成功备份**。成功意味着数据库、媒体及配置校验通过，实际执行隔离 `pg_restore`，并成功清理临时库；并非只有归档文件生成成功。
- 默认目录为 `~/.local/share/painter/backups/automatic`，包括数据库、ready 媒体和私有配置快照。目录 0700、文件 0600；失败不会淘汰已有成功副本，手工备份不参与自动清理。
- 当前范围是**本机副本**，没有配置异机副本，也没有额外的备份密码加密。磁盘或整机丢失仍需要独立设备上的加密副本。
- 恢复先创建新数据库、新媒体目录与候选配置，迁移并隔离检查，再显式停写切换；失败回退原配置，原库保留。恢复出的旧会话、MFA 登录请求与 OIDC 授权会失效，需要重新登录。
- 当前机器的域名、内网监听与 Tunnel 配置保持不变；恢复工具不替换 cloudflared token，不停止或重装 Tunnel。完整自动切换针对当前 native systemd 部署，Compose 的隔离恢复准备与实际切换边界见技术文档。

查看备份是否真正完成：

```bash
python3 scripts/backup-schedule.py status
"$HOME/.local/share/painter/bin/painter" backup-status \
  --root "$HOME/.local/share/painter/backups/automatic"
```

以 `last_success_at` 和快照 `verified_at` 判断完成状态。安装、调度调整、`stage` / `activate` / `rollback` 操作见 [自动备份与恢复](AUTOMATIC_BACKUP_RECOVERY.md)，不要直接覆盖正在运行的数据库。

## 账号访问与两步验证

站长在 [站点管理 → 登录安全](https://blog.taota.cc/studio/settings#security) 控制两个独立开关：

| 设置 | 实际效果 |
| --- | --- |
| 允许外部账号登录 | 控制站长以外的已有账号；关闭时撤销其会话并阻止后续本地、GitHub 与 OIDC 访问。站长 `owner` 豁免，公开阅读继续可用 |
| 允许新账号注册 | 控制本地注册与 GitHub 首次建号；外部登录关闭时，注册也实际暂停，保存的注册偏好仍保留 |

服务端执行这些限制，界面隐藏不是鉴权依据。本次发布后的实际开关值见上方部署记录。

两步验证由用户在 [个人资料 → 两步验证](https://blog.taota.cc/me#account-security-title) **自行绑定**，部署不会代替用户绑定站长验证器。输入当前密码，扫描浏览器本地生成的二维码，确认 6 位动态码，再保存一次性显示的 10 组恢复码。只有 GitHub 身份的账号使用最近的 GitHub 身份验证管理绑定。

已绑定账号完成密码或 GitHub 验证后仍需通过 TOTP / 单次恢复码，才能建立完整会话。实现包含动态码重放保护、持久失败锁定、恢复码原子消费、敏感操作重新验证和会话撤销。修改密码保留 MFA；验证器与恢复码都丢失时，仅有本机运维入口 `disable-local-mfa`，操作方法见 [账号安全文档](ACCOUNT_SECURITY.md)。

Tunnel 下只有受限可信代理直接来源才能启用 `CF-Connecting-IP` 限流识别，不信任任意访客的转发头。原部署的 `APP_ENCRYPTION_KEY` 必须随私有备份安全保存，恢复时不能临时生成新密钥替代。

## 其他项目如何共用登录

认证作为 Go 单体内的 OIDC 模块运行，与 Painter 共用账号、MFA、策略与 PostgreSQL，不要求其他项目共享数据库或 Cookie。

- Issuer：`https://blog.taota.cc/oidc`。
- Discovery：`https://blog.taota.cc/oidc/.well-known/openid-configuration`。
- 站长在 **登录安全 → 接入其他应用** 登记应用及精确回调；客户端以 discovery 返回地址为准接入。

| 客户端 | 接入方式 |
| --- | --- |
| Web 服务端 | Authorization Code + PKCE S256；客户端密钥仅显示一次，保存在服务端，Token 端使用 HTTP Basic |
| SPA | 公共客户端，无客户端密钥；HTTPS 精确回调，仅登记的 origin 可使用相应 CORS |
| 桌面 / 移动 Native | 系统浏览器 + Code + S256；支持 HTTPS、反向域名私有 scheme、已登记 IP 回环 HTTP 回调，回环仅允许端口变化 |

每次授权都需要用户明确同意。Access / ID Token 为 5 分钟，Refresh Token 原子旋转，授权最多 30 天；重放旧 Refresh 会撤销该授权族。关闭外部登录、会话撤销、MFA 变化或客户端停用都会限制后续换取、刷新与 UserInfo。ID Token 不授予博客管理 API 权限；仅离线校验 JWT 不能立即获知撤销。

这是可供其他项目接入的登录核心，并未完成完整企业身份平台。本轮不支持 Passkey、邮件验证 / 自助找回、设备码、密码直传、客户端凭据、动态客户端注册、组织权限、完整跨应用单点注销或自动签名密钥轮换。无浏览器 CLI / 电视设备不能使用未实现的设备码流程。协议端点、表结构、事务时序、撤销与恢复语义见 [认证中台文档](IDENTITY_PROVIDER.md)。

## 验证依据与范围

- 使用 **Go 1.26.11 linux/amd64** 完成 `go test -race ./... -count=1`，退出码 0；集成测试显式使用隔离 PostgreSQL 18，未使用正式业务库。
- `govulncheck` 补丁后扫描退出码 0，可达代码路径未发现已知漏洞；不等同于整个依赖图或业务逻辑没有风险。
- 真实 Chrome 浏览器完成 **39 项检查、12 张截图**：密码与二步登录、本地二维码、一次性恢复码、两个访问开关、客户端创建 / 轮换 / 停用、真实授权回调与 PKCE 兑换、会话撤销，以及未保存个人资料的保留。没有记录到 JavaScript 错误、CSP 违规或服务端 500。
- 视觉矩阵为 Chrome **1440 × 1000 桌面**与 **390 × 844 模拟手机**，各含浅色 / 深色；不代表实体手机、Safari、Firefox 或全部辅助技术已验收。截图已遮蔽二维码、验证器密钥、恢复码和客户端密钥。
- 备份验证包括实际 PG18 隔离恢复、旧版本迁移、运行角色权限保持、调度单位验证以及失败回退；当前环境没有 Docker daemon，因此不声称完成 Docker 镜像构建或 Compose 切换实测。

本机可复核证据位于以下被 Git 忽略的测试产物；测试脚本保留在源码中：

```text
tests/artifacts/v43/go-test-patched-toolchain.json
tests/artifacts/v43/go-test-patched-toolchain.log
tests/artifacts/v43/govulncheck-fixed.json
tests/artifacts/v43/govulncheck-fixed.log
tests/artifacts/v43/auth-ui/verification.json
tests/artifacts/v43/legacy-restore.json
tests/artifacts/v43/release.json
tests/browser/v43-auth-security.cjs
tests/browser/v43-identity-consent.cjs
tests/backup/test_tools.py
```

## 后续协作约定

项目根目录 [AGENTS.md](../../AGENTS.md) 已记录：每轮完成、验证通过的修改创建本地 commit，可独立交付的改动分开提交；未获推送指示时只做本地提交。密码、令牌、环境文件、数据库备份、恢复密钥和测试凭据不得进入 Git。继续使用 PostgreSQL 18，禁止 SQLite；测试不得回退正式 `DATABASE_URL`。
