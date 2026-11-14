# V7：后台配置 GitHub 登录

站长在“站点设置 → GitHub 登录”中登记应用信息、保存 Client ID 和 Client Secret，并启用入口。界面提供本站准确的 callback 地址以及 GitHub Developer Settings 链接；创建 GitHub OAuth App 和真实站点连通验收由站长使用自己的 GitHub 账号完成。

## 配置与保密

没有数据库配置行时，沿用服务器的 `GITHUB_CLIENT_ID`、`GITHUB_CLIENT_SECRET`。首次保存会把有效凭据写入数据库；之后数据库配置具有优先级，包括明确停用时也不会再次启用环境变量中的凭据。停用只关闭新的 GitHub 登录，现有 Painter 会话遵守原来的过期和撤销规则。

Client Secret 使用现有应用加密密钥与 AES-GCM 加密保存。读取接口只显示是否已配置；加载配置和成功保存后输入框为空，留空保持有效密钥，输入新密钥则替换。首次保存时省略密钥会保留环境配置中的有效密钥，不保存占位字符串。密钥不存入浏览器 localStorage、sessionStorage 或草稿恢复库。备份可能包含数据库中的加密密文，恢复时必须保留正确的应用加密密钥。

只有 owner 可以读取或修改此项配置。写入要求当前会话、精确 Origin 和会话 CSRF，并在 policy barrier 下重新检查角色和会话；版本冲突返回 409，必须重新加载后保存。密钥只在配置写入请求中提交，服务端加密保存；读取响应、请求日志和审计不记录密钥值。

## API

`GET /api/v1/admin/auth/github` 和成功的 PUT 返回：

```json
{
  "version": 0,
  "enabled": false,
  "client_id": "",
  "client_secret_configured": false,
  "source": "environment",
  "callback_url": "https://your-site.example/auth/github/callback"
}
```

上例仅说明字段；本站实际 callback 来自 `PUBLIC_BASE_URL`。未保存时版本为 0，保存后递增；`source` 为 `environment` 或 `database`。读取接口绝不返回 `client_secret` 或密文。

`PUT /api/v1/admin/auth/github` 接受 `{version,enabled,client_id,client_secret?}`。省略或 `null` 保留当前有效密钥；显式空字符串可以清除，但清除时必须同时停用。启用需要非空 Client ID，以及已保存或本次提交的非空密钥。不存在 provider URL、token URL 或 profile URL 请求字段；严格 JSON 解码会拒绝这些未知字段。

`GET /api/v1/auth/options` 的 `github_login_enabled` 使用数据库/环境配置后的实际开关；保存的可选 GitHub 密钥无法解密时关闭该入口，本地登录仍可使用。callback 固定为 `PUBLIC_BASE_URL/auth/github/callback`；除 `test` 外的所有环境（包括 development）只允许 GitHub 官方授权、token 和用户信息端点，只有隔离测试使用 mock provider。保存配置和显示“已启用”表示本地开关及必需字段已就绪，不证明应用凭据或真实 callback 已经验通。

## 身份和会话边界

GitHub 登录按稳定的数字 subject 匹配原用户，不按邮箱或 GitHub 登录名合并。入口启用且本站账号未停用时，已有 GitHub 身份可在关闭注册后继续登录；普通新 GitHub 身份仅在开放注册时得到 reader。兼容的 owner 引导仍按显式配置的 `BOOTSTRAP_GITHUB_ID` 数字身份运行，独立于普通读者注册。站长本地账号不会因用户名或邮箱相同自动绑定 GitHub，本轮没有新增绑定/解绑功能。

OAuth state 绑定创建时的有效配置。保存、替换或停用配置会使在途登录失效；callback 在网络交换之前和最终创建会话的事务中都重新检查。升级前遗留的在途登录同样需要重新开始。成功登录在创建新会话的同一个事务中撤销该浏览器请求携带的旧 Painter token；失败的交换或配置变更不会撤销旧会话。

GitHub 是第一步身份验证来源；绑定 TOTP 的账号仍须完成动态码或恢复码验证后才建立完整会话。敏感设置要求近期身份验证。完整操作见 [账号安全](ACCOUNT_SECURITY.md)，跨项目身份接入见 [OIDC 身份服务](IDENTITY_PROVIDER.md)。在线账号绑定/解绑、Passkey 和设备码尚未提供。

## 验证范围

新增集成用例位于 [github_settings_test.go](../internal/app/github_settings_test.go)：权限/Origin/CSRF、CAS、密钥密文与响应脱敏、环境密钥保留、停用覆盖环境、配置校验与显式清除、数据库有效凭据、成功登录的会话替换、callback 重放，以及交换前/交换中的配置变更拒绝。测试使用 disposable PostgreSQL 和 mock GitHub，不证明真实 GitHub 凭据或公网 callback 已验通。
