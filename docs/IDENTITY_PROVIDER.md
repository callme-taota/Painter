# Painter 本地认证中台

Painter 以嵌入模块提供 OpenID Connect 登录，和博客共用 Go 单体服务、PostgreSQL、账号、会话、注册与登录开关、TOTP 双重验证。其他项目通过标准浏览器跳转接入，不读取 Painter 数据库，不共享 Cookie，也不能用 ID Token 调用博客管理 API。

协议实现使用 [ZITADEL oidc/v3](https://github.com/zitadel/oidc/tree/v3.51.16)，本版本固定支持 **Authorization Code + PKCE S256**。这是明确收敛的实现范围，并未对 Painter 自身做 OpenID 认证：不支持 implicit、密码直传、设备码、客户端凭据、动态客户端注册、私钥客户端认证或 Token Exchange。Web 与原生客户端均能采用系统浏览器登录；没有浏览器的纯 CLI/电视设备当前不提供 Device Authorization Grant。

## 入口与客户端

issuer 为部署配置中的 `PUBLIC_BASE_URL` 加 `/oidc`，例如 `https://blog.example.com/oidc`；绝不从外部 Host/Forwarded 头推断 issuer。

| 用途 | 地址 |
| --- | --- |
| 自动发现 | `/oidc/.well-known/openid-configuration` |
| 浏览器授权 | `/oidc/authorize` |
| 用户明确授权页 | `/oidc/consent` |
| 兑换及刷新 | `/oidc/oauth/token` |
| 公钥 | `/oidc/keys` |
| 用户资料 | `/oidc/userinfo` |
| Token 撤销 | `/oidc/revoke` |
| Token 有效性查询 | `/oidc/oauth/introspect` |
| 注销应用授权 | `/oidc/end_session` |

以 discovery 返回值为准，不在客户端硬编码这些相对路径。站长在后台登录安全中登记应用：

- **Web**：有可信服务端，生成仅显示一次的随机 `client_secret`，Token 端使用 HTTP Basic 客户端认证，并且仍要求 PKCE S256。密钥只放服务端。
- **SPA**：纯浏览器应用，无客户端密钥，HTTPS 回调精确匹配。仅对站长登记的 SPA 回调 origin 开放 Token/UserInfo CORS，不开放跨域 Cookie。
- **Native**：桌面和移动客户端，无客户端密钥，使用外部系统浏览器和 PKCE。回调可以是 HTTPS App Link、反向域名私有 scheme（例如 `com.example.painter://callback/login`）或 IP 回环 HTTP。

所有回调禁止通配符、片段、用户名/密码和控制字符；Web/SPA 必须 HTTPS 且字符串精确一致。Native 仅允许已登记 IP 回环地址改变端口：`http://127.0.0.1/callback` 可以使用运行时空闲端口，主机、路径、查询参数必须保持一致；`localhost`、局域网 HTTP、另一个回环 IP 不做隐式替换。客户端还需用自己的 state 防登录 CSRF，并校验 nonce、issuer、audience、签名与到期时间。

## 数据与完整时序

```text
其他应用 -> Painter /oidc/authorize（client_id、精确 redirect_uri、scope、state、nonce、S256 challenge）
  -> 库校验协议 + 本地策略校验 -> identity_auth_requests（10 分钟）
  -> 没有有效 Painter 会话：统一账号密码 / GitHub + 已启用的 MFA
  -> 展示应用名称、返回地址和权限 -> 用户允许 / 拒绝
  -> 同源 CSRF 事务绑定 user_id、Painter session_id 与实际认证方式
  -> 一次性授权码（哈希入库，1 分钟）返回应用
其他应用 -> Token 端（code、redirect_uri、code_verifier；Web 另有 Basic）
  -> PKCE/客户端/回调检查 -> 事务重新检查用户、登录政策、MFA、会话与应用
  -> 原子消费 code -> identity_grants + identity_tokens -> access_token、id_token、可选 refresh_token
其他应用 -> UserInfo（Bearer access_token）
  -> Token、授权、客户端、Painter 会话、登录政策/MFA 仍有效 -> 最小公开资料
其他应用 -> refresh_token
  -> 原子消费旧 refresh -> 同 family 新 refresh + access/id_token
  -> 若重放旧 refresh：撤销该 family 全部 Token
```

应用每次授权都展示确认页，不因 Painter 已登录就自动批准第三方；`prompt=none` 返回需要授权错误。当前支持 `prompt=consent/login` 与 `max_age`：过期身份须用户主动重新验证，再回授权页确认。

| 表 | 内容与生命周期 |
| --- | --- |
| `identity_clients` | 站长登记的应用、回调、类型、客户端密钥哈希、停用状态 |
| `identity_signing_keys` | RSA 2048 / RS256 私钥密文，使用部署 APP_ENCRYPTION_KEY 加密；JWKS 仅公开公钥 |
| `identity_auth_requests` | 10 分钟授权上下文、用户与会话绑定、授权码哈希；兑换时原子消费 |
| `identity_grants` | 应用、账号、Painter 会话、最长 30 天绝对到期和撤销状态 |
| `identity_tokens` | 5 分钟 Access Token 的内部 ID、Refresh Token 的 SHA-256 哈希、scope、使用/撤销时间 |

Access Token 是库生成的认证加密不透明 Token，数据库中的随机内部 ID 不能直接当 Bearer 凭据。ID Token 为 5 分钟 RS256 JWT，audience 仅当前应用；scope 仅 `openid`、`profile`、`offline_access`，没有默认后台权限、邮箱或手机号。仅请求 `offline_access` 并完成明确授权时签发 Refresh Token。Refresh 旋转不会延长原始 30 天授权期限，旧哈希保留至 family 到期以检测重放。

禁用用户、关闭外部用户登录、撤销对应 Painter 会话、启用或更新 MFA、停用客户端，都会阻止后续换取 Token、刷新和 UserInfo。注销应用授权只撤销该应用授权，不自动注销浏览器中其他应用。轮换客户端密钥同时撤销该应用现有授权。JWT 的离线签名校验无法立即获知服务端撤销；消费方需要实时失效时必须调用 UserInfo/Introspection，不应只根据尚未过期的 ID Token 判定当前登录状态。

## 管理 API

全部接口仅站长 Cookie 会话可用；写操作要求标准 Origin 与 CSRF。OAuth Bearer Token 不授予这些接口权限。

```text
GET    /api/v1/admin/identity/clients
POST   /api/v1/admin/identity/clients
DELETE /api/v1/admin/identity/clients/{id}
POST   /api/v1/admin/identity/clients/{id}/rotate-secret
```

创建体 `{ "name": "我的应用", "kind": "web", "redirect_uris": ["https://app.example/auth/callback"] }`；响应 `{ "client": {...}, "client_secret": "仅此次显示" }`。SPA/Native 不返回密钥。读取不包含密钥或其哈希；删除是停用并撤销授权，保留审计证据。密钥不进入幂等响应缓存、URL、日志或 Git。

## Tunnel、备份与恢复

Cloudflare Tunnel 继续代理同一个单体。HTTPS 终止于 Tunnel，发行者与回调仍使用外部 HTTPS 域名，内部服务不需要另开身份端口，也不把内网 HTTP 注册成外部 Web 应用回调。应用注册自己的业务域名或原生回环回调即可。

数据库备份包含账号、MFA 密文、恢复码哈希、客户端、签名私钥密文及授权状态。恢复还必须保留原部署 `APP_ENCRYPTION_KEY`，否则 MFA 与身份签名私钥无法解密，程序明确失败；不要生成一个新密钥冒充恢复成功。域名改变意味着 issuer 改变，需要更新消费方配置并重新授权，不能把旧 issuer Token 自动当新站点身份。

恢复旧快照可能让快照时尚未消费的 refresh 再次出现，也可能恢复已撤销的会话。生产恢复流程必须在恢复库启动对外服务前撤销 `sessions`、`identity_grants`、清理 `identity_auth_requests` / `oauth_states` 与 MFA challenge，再由用户重新登录；本模块不假设“恢复数据库”和“继续信任旧凭据”是同一操作。签名私钥当前持久保存，没有自动密钥轮换，备份加密和部署密钥离线保管属于必要运维。不要把备份放到静态资源目录或经 Tunnel 公开下载。

## 接入验证

客户端优先使用其平台维护的 OIDC/RP SDK。原生客户端采用系统浏览器（[RFC 8252](https://www.rfc-editor.org/rfc/rfc8252)），所有授权码都采用 [PKCE S256](https://www.rfc-editor.org/rfc/rfc7636)。实现边界依据 [OpenID Connect Core](https://openid.net/specs/openid-connect-core-1_0.html) 与 [OAuth 2.0 安全最佳实践 RFC 9700](https://www.rfc-editor.org/rfc/rfc9700)，无需其他项目重写密码、TOTP、恢复码或登录 Cookie。
