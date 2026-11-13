# V45：修复登录入口与表单校验

2026-11-13 18:14:46（Asia/Shanghai）已上线，源码提交 `666bd5c`。

## 确认的问题与修复

- 正式账号仍为 `Taota`，角色 owner、未停用、有本地密码、未启用 MFA。关闭外部登录的策略明确豁免 owner；本轮没有更改账号、密码或开关。
- 局域网 HTTP 的登录页原先直接显示表单，但生产认证只接受 HTTPS 公网 Origin，且 Secure Cookie 无法在该 HTTP 地址保存，必现 `403 ORIGIN_REJECTED`。现在登录 / 注册页在收集密码前转到配置中的 **https://blog.taota.cc**，保留站内返回地址；公开阅读仍可使用局域网地址。
- 登录原先错误套用了注册密码的 15–1024 字符限制，导致短密码不发请求。现在登录交给服务器验证已有凭据，仅保留 4096 UTF-8 字节资源上限；新密码强度规则没有降低。
- 用户名 HTML pattern 的连字符已转义，消除现代 Chrome Unicode v 模式的无效正则。

跳转目标只来自配置的 HTTPS Origin，Host / 转发 Host / return_to 不会替换目标域名；API POST 不跟随该跳转，原有 Origin、CSRF、Secure / HttpOnly / SameSite Cookie 均保留。完全独立于公网域名的内网认证需要另行设计内网 HTTPS 与可信来源；本次没有开放 HTTP 密码会话或新增可信 Origin。

## 验证

- 本地 PostgreSQL 18 一次性测试库上的登录、外部账号策略、传输安全、Web 和 OIDC 相关 Go race 测试通过。
- 前端类型检查与发布构建通过；隔离浏览器客户端校验 **33 项通过**，4 张手机 / 桌面截图，无页面或用户名 pattern 错误。此组认证 POST 使用拦截响应，仅验证客户端逻辑。
- 另用一次性本机 TLS 反代、生产 Cookie 配置及同一隔离库执行 **13 项真实登录检查**：手机与桌面均从 HTTP 入口转 HTTPS，注册 / 外部登录关闭时 owner 密码登录仍成功，导航后会话有效，Nonce 与 Session 的 Secure / HttpOnly / Lax 均保留。没有使用正式 Taota 密码，也没有修改系统证书或正式 Tunnel。
- 正式公网浏览器验证局域网登录跳转、返回参数、安全 nonce 和表单提交；仅用一个明确不存在的合成用户名获得预期 `401 INVALID_CREDENTIALS`。没有猜测或测试 Taota 的密码，不能用该结果断言用户持有的密码一定正确。

证据：`tests/artifacts/v45/`；可复用浏览器脚本：`tests/browser/v45-login-regression.cjs`、`tests/browser/v45-login-https.cjs`。本轮为 Chrome 与手机模拟，不代表其他浏览器或实机全面验收。

## 发布保护

正式内外网健康检查、首页新静态资源、登录页和 OIDC discovery 正常，运行配置逐字节保持不变，数据库仍为 schema 23，正式业务计数与账号访问状态保持不变。

发布前备份 `auto-20261113T101244Z-e039c3436c7afc51`，发布后备份 `auto-20261113T101638Z-8ef6e6aa0c244f0e`，均完成真实隔离恢复校验。原 V44 二进制保存在本机私有发布目录，备份定时器继续启用。

登录方式与安全边界见 [账号访问与两步验证](ACCOUNT_SECURITY.md)。
