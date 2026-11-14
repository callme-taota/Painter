# 网络入口与公网访问

Painter 保持 Go 单体、PostgreSQL 18 和媒体存储的架构。网络入口可以使用同机代理、Cloudflare Tunnel，或可入站的公网 IPv6 与 DDNS。本文使用 `blog.example.com`、`192.168.1.10` 和 `127.0.0.1:8080` 作为示例，不代表任何已部署实例。

## 两条公网链路

```text
Cloudflare Tunnel
浏览器 → HTTPS → Cloudflare ← 加密出站连接 ← cloudflared → HTTP/HTTPS → Painter

IPv6 DDNS + HTTPS 代理
浏览器 → HTTPS → 公网 IPv6 上的 Caddy → HTTP → Painter
                    ↑ AAAA 由 DDNS 更新

IPv6 DDNS + Cloudflare 橙云
浏览器 → HTTPS → Cloudflare → HTTPS → 公网 IPv6 上的 Caddy → HTTP → Painter
```

| 条件 | Cloudflare Tunnel | IPv6 DDNS |
| --- | --- | --- |
| 公网地址 | 不要求本机有公网 IP | 服务器需要可路由、可入站的公网 IPv6 |
| 入站端口 | 无需为博客新增公网入站端口 | 需要允许 HTTPS 入口入站 |
| 地址变化 | 连接器重连 | 更新 AAAA，并等待 DNS 缓存过期 |
| 长期运行组件 | `cloudflared` | TLS 代理与 DDNS 更新任务 |
| 回源保护 | 加密隧道；最后一跳按实际网络配置 | 直连浏览器或 Cloudflare 回源都需要有效 HTTPS |
| 操作说明 | [Tunnel 最简步骤](TUNNEL_QUICKSTART.md) | [IPv6 DDNS 条件与配置](DDNS_QUICKSTART.md) |

Tunnel 需要服务器出站连接 Cloudflare，连接端口使用 7844，QUIC 走 UDP、HTTP/2 走 TCP；DNS 及安装维护所需访问也要可用。[连接要求](https://developers.cloudflare.com/tunnel/configuration/)

## 回源地址必须真实可达

| 连接器与 Painter 的位置 | Service URL 示例 |
| --- | --- |
| 同一主机和网络命名空间，Painter 绑定回环 | `http://127.0.0.1:8080` |
| Painter 绑定局域网地址 | `http://192.168.1.10:8080` |
| 同一 Compose 私有网络 | `http://app:8080` |

`0.0.0.0` 是监听地址，不能直接作为 Service URL。容器中的 `127.0.0.1` 指向该容器，应从连接器所在网络检查回源。跨不可信网络的最后一跳使用经验证的 HTTPS，保持证书校验；同机回环 HTTP 只适用于该受限链路。

## 统一公开 Origin

生产部署只配置一个完整 HTTPS Origin，不带 `/blog` 等路径前缀：

```dotenv
ENVIRONMENT=production
PUBLIC_BASE_URL=https://blog.example.com
LISTEN_ADDR=127.0.0.1:8080
```

原生部署修改其真实私有运行配置，例如 `~/.config/painter/runtime.env`；Compose 修改原 `.env`。保留数据库、密钥和媒体路径。变更前按 [备份恢复流程](AUTOMATIC_BACKUP_RECOVERY.md) 保存配置和数据，再使用既有部署流程加载新配置。

`PUBLIC_BASE_URL` 决定规范链接、站点地图、OAuth 回调和认证边界。浏览器登录、注册与写请求必须来自这个 Origin，并通过 Cookie 和 CSRF 校验；代理保留浏览器 Origin，不增加另一个 API 域名，也不靠放宽 CORS 适配旧地址。启用 GitHub 时，把提供方登记的回调和已有显式 `GITHUB_CALLBACK_URL` 同步到 `https://blog.example.com/auth/github/callback`。

需要保留内网公开阅读时，可继续将应用绑定到 `192.168.1.10:8080`；连接器也改用该可达地址。内网 HTTP 和正式 HTTPS 的登录态、浏览器草稿互相独立。生产 Secure Cookie 只通过 HTTPS 发送，登录与管理统一使用正式域名。内网完整认证需要同一域名的内网解析与可信 HTTPS，见 [局域网访问](LAN_ACCESS.md)。

## 代理协议与访客 IP

可按需要启用公开 HTTP 到 HTTPS 的跳转。以下示例仅适用于代理从本机 IPv4 回环连接应用的情况：

```dotenv
ENFORCE_PUBLIC_HTTPS=true
TRUSTED_PROXY_CIDRS=127.0.0.1/32
```

应用只接受命中白名单的直接 TCP 来源提供的单一 `X-Forwarded-Proto`。代理应保留公开 Host，并覆盖协议头。改绑局域网或改用容器时，按真实连接来源调整窄白名单，不照搬示例、不信任整个局域网或全网。同机进程也属于这个信任边界。

默认限流使用直接连接来源。确实经 Cloudflare 回源、且可信代理能控制请求头时，可显式设置 `TRUST_CLOUDFLARE_CONNECTING_IP=true`，配合受限 `TRUSTED_PROXY_CIDRS` 读取单一合法 `CF-Connecting-IP`。未知来源或异常值回退直接来源；应用不使用任意 `X-Forwarded-For`、`X-Real-IP` 作为访客身份。代理白名单仅启用协议识别时，不会自动改变限流身份。实现见 [代理与来源处理](../internal/app/transport_security.go)。

## 缓存和传输

保留源站响应头：`/assets/` 哈希静态资源可公开缓存，动态页面、API 和认证响应不强制缓存。不要设置全站 Cache Everything，也不要对 `/media/` 按图片目录统一公开缓存；媒体可能受可见性和登录权限控制。[Cloudflare 缓存说明](https://developers.cloudflare.com/cache/get-started/)

Tunnel 的最后一跳由 Service URL 决定；Cloudflare 的 SSL 模式不会把 HTTP 监听变成 HTTPS。IPv6 橙云回源使用 Full (strict)，回源证书要有效、主机名匹配且受信任。不要用 Flexible 或关闭证书验证掩盖协议配置问题。[Full (strict)](https://developers.cloudflare.com/ssl/origin-configuration/ssl-modes/full-strict/)

## 验收与排障

先从实际连接器或代理所在网络验证回源，再从另一条外网验证域名：

```bash
curl --fail --silent --show-error --max-time 5 http://127.0.0.1:8080/readyz
curl --fail --silent --show-error --max-time 15 https://blog.example.com/readyz
```

| 现象 | 优先核对 |
| --- | --- |
| Tunnel Healthy 但网页 502 | 回源地址、实际监听、容器网络、HTTP/HTTPS 是否匹配 |
| IPv6 访问超时 | 公网地址、AAAA、前缀变化、路由器与主机入站策略 |
| 证书错误或循环跳转 | 证书域名与信任链、Service URL 协议、代理协议头和跳转规则 |
| 可阅读但登录或保存失败 | 浏览器 Origin 与 `PUBLIC_BASE_URL`、Cookie、CSRF 和错误缓存 |
| 正常访问触发 429 | 共享代理来源、可信访客 IP 配置、实际流量与失败窗口 |

验收同时核对规范链接、登录与退出、草稿权限、上传、缓存和重启后的连接恢复；写入验收先在独立候选环境执行。PostgreSQL、备份目录与运维接口保持私有。公网连通性和访问质量应按实际客户端与网络测量，不从连接器上线推断整站已可用。
