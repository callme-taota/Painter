# Painter 的 IPv6 DDNS 接入

DDNS 在公网地址变化时更新 DNS 记录。它不提供内网穿透，也不负责路由器入站规则或 HTTPS 证书。以下说明使用公网 IPv6 发布 Painter 的必要条件和配置边界，不包含 DDNS 客户端实现、安装脚本或任何实例的接入结果。[动态 DNS 说明](https://developers.cloudflare.com/dns/manage-dns-records/how-to/managing-dynamic-ip-addresses/)

```text
DDNS 客户端 → 更新 blog.example.com 的 AAAA

纯 DNS：浏览器 → 公网 IPv6 → Caddy HTTPS → http://127.0.0.1:8080 → Painter
橙云：浏览器 → Cloudflare HTTPS → 公网 IPv6 上的 Caddy HTTPS → Painter
```

## 先确认公网 IPv6 条件

服务器自身需要可路由的全球单播 IPv6，以及从外网到该服务器的 HTTPS 入站路径。仅有路由器 WAN 地址、`fe80::` 链路本地地址或 `fc00::/7` 本地地址不够；“可以通过 IPv6 访问外网”也不能证明外网可以连接进来。

用于 AAAA 的地址应适合服务器长期使用，不采用短期临时或已弃用地址。路由器、主机防火墙及虚拟化网络都要允许目标服务器的 HTTPS 流量；这是 IPv6 入站策略，通常不需要 IPv4 NAT 端口转发。端口被运营商或防火墙阻断时，更新 DNS 仍不能恢复入站，可改用 [Cloudflare Tunnel](TUNNEL_QUICKSTART.md)。

## 选择 DNS 直连或 Cloudflare 代理

| 模式 | 流量路径 | 客户端与证书要求 |
| --- | --- | --- |
| DNS only / 灰云 | 访客直接连接服务器 | 仅有 IPv6 源站时，访客也需要 IPv6；源站证书必须受浏览器信任 |
| Proxied / 橙云 | 访客连接 Cloudflare，再回源服务器 | 公网 IPv6 仍须可入站；回源使用 HTTPS 与有效证书 |

使用 Cloudflare 做 DNS 不等于启用了其 HTTP 代理。橙云可承接访客的 IPv4/IPv6 请求，但不会绕过源站入站限制。[代理状态说明](https://developers.cloudflare.com/dns/proxy-status/)

选择支持现有 DNS 服务商的 DDNS 客户端，配置其跟随服务器公网 IPv6 更新 `blog.example.com` 的 AAAA；保留所选代理状态。DNS 更新凭据存放在仓库外私有配置中，权限只覆盖所需域名与 DNS 操作；不要使用覆盖整个账号的凭据。同名旧 A、AAAA 或 Tunnel CNAME 应先核对用途，避免解析到不同实例。本文不指定客户端实现，也不自动修改 DNS。

## HTTPS 入口与回源

Painter 原生服务提供 HTTP，公网 HTTPS 由 Caddy 等代理承接。同机代理可采用：

```caddyfile
blog.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

此例要求 Caddy 与 Painter 同机，且 Painter 监听 `127.0.0.1:8080`。同一 Compose 网络应使用项目已有的 `app:8080` 回源，不把容器回环当成宿主机。代理监听公网 IPv6 的 443；证书申请方式所需端口也必须可达。Caddy 的 HTTP-01 和 TLS-ALPN-01 验证分别依赖 80 和 443，DNS-01 获得证书也不会开放被阻断的 HTTPS 端口。[Caddy 自动 HTTPS](https://caddyserver.com/docs/automatic-https)

纯 DNS 直连使用公众信任证书，不把仅受 Cloudflare 信任的 Origin CA 证书直接交给浏览器。橙云回源选择 Full (strict)，要求证书在有效期内、主机名匹配且由公开 CA 或 Cloudflare Origin CA 签发。不要以 Flexible 或关闭验证替代正确的回源 HTTPS。[Full (strict) 要求](https://developers.cloudflare.com/ssl/origin-configuration/ssl-modes/full-strict/)

## 配置 Painter 的正式地址

切换前保存或导出未完成编辑，并备份数据库、媒体及私有配置。原生部署在实际使用的配置文件（例如 `~/.config/painter/runtime.env`）中修改访问项；Compose 使用原 `.env`：

```dotenv
PUBLIC_BASE_URL=https://blog.example.com
ENVIRONMENT=production
```

保留原数据库、密钥和媒体目录，并维持代理可达的应用监听。若有显式 `GITHUB_CALLBACK_URL`，同步到 `https://blog.example.com/auth/github/callback`；启用 GitHub 时也更新提供方回调。使用既有部署流程加载配置，不重复安装站点。[部署说明](DEPLOYMENT.md)

登录和管理统一使用正式 HTTPS 域名。内网 HTTP 仅用于公开阅读和诊断；若内网需要完整登录，则使用同一正式域名的内网解析与可信 HTTPS。Cookie、浏览器草稿与会话不会自动随 Origin 迁移。[局域网访问](LAN_ACCESS.md)

## 配置后验证

用另一条具备 IPv6 的网络检查纯直连入口，确认 DNS、TCP 443、证书和 `/readyz` 均正常。橙云模式还要分别核对边缘访问与真实源站回源，不能把代理域名响应当成源站 IPv6 已验证。

随后观察一次公网前缀变化后的恢复过程：服务器地址、AAAA、入站规则、证书与代理监听仍应一致；DNS 缓存期间可能短暂使用旧地址。账号和内容操作先在独立候选环境验收，正式切换保留原配置与完整备份。代理来源与限流设置见 [网络入口](NETWORK_ACCESS.md)。
