# 本地 Painter 的公网访问方案

直接接入可以看 [Cloudflare Tunnel 最简操作步骤](TUNNEL_QUICKSTART.md)；希望公网 IPv6 直连、由 DDNS 跟随地址变化时，看 [IPv6 DDNS 操作步骤](DDNS_QUICKSTART.md)。

方案核对日期：2026-11-07；接入更新：2026-11-11。当前已采用 Cloudflare Tunnel，正式入口为 **https://blog.taota.cc**，回源为 `http://192.168.2.209:18080`；连接器通过用户级 `painter-tunnel.service` 自动运行，应用为 production 环境，HTTPS 访问与规范链接已验证。下文保留两种方案的原理与边界；其中 `blog.example.com`、Tunnel UUID、证书路径和 `2001:db8::1234` 仍是占位示例。DDNS 与公网 IPv6 入站方案尚未实施。

本地个人博客优先选 **Cloudflare Tunnel**：无需先获得可入站的公网 IPv6，也不需要 DDNS、开放路由器入站端口或在博客主机上另开公网 TLS 服务。若已有可靠的公网 IPv6、入站 443 和证书维护能力，**IPv6 DDNS + Cloudflare 橙云**也可行。两者都保留现有 Go 单体、PostgreSQL和媒体存储，不需要改写业务系统。

## 两条链路的区别

```text
IPv6 DDNS + 橙云
浏览器 ──HTTPS:443──> Cloudflare ──HTTPS:443 / 公网IPv6──> Caddy ──HTTP──> Painter
                                  AAAA由DDNS维护

Cloudflare Tunnel
浏览器 ──HTTPS:443──> Cloudflare <──加密出站连接:7844── cloudflared ──HTTP──> Painter
```

| 比较项 | IPv6 DDNS + 橙云 | Cloudflare Tunnel |
| --- | --- | --- |
| 本地公网地址 | 需要真正可路由、可入站的公网 IPv6 | 不要求公网 IP |
| 家庭入站规则 | 放行服务器 IPv6 的 TCP 443 | 不需要为博客新增公网入站端口 |
| 地址变化 | DDNS 更新 AAAA，期间可能短暂不可达 | connector 重新连接，不依赖家庭 IP 的 DNS 更新 |
| 新增组件 | TLS 反向代理、DDNS 更新任务 | `cloudflared` 常驻进程 |
| TLS 维护 | 公网回源证书与续期/更换 | 公网 HTTPS 由边缘承接；同机最后一跳可用 HTTP |
| 当前 18080 | 放在 443 TLS 代理后面 | 可作为 Tunnel 的本地 Service URL |
| 主要故障面 | IPv6 前缀、入站策略、DDNS、证书 | 出站连通性、connector、Service URL |
| 适合当前项目 | 条件具备时的备选 | 首选试运行方案 |

Tunnel 使用主动出站连接；传输端口为 7844，QUIC 使用 UDP，HTTP/2 使用 TCP。主机仍需能正常解析域名并访问安装、管理等所需服务，不能把“只放行 7844”当作完整出站配置。[Cloudflare Tunnel 配置与防火墙要求](https://developers.cloudflare.com/tunnel/configuration/)

## 当前项目已确认的边界

下列限制依据源码和网络检查；本次接入仅更新正式网址、GitHub 回调与运行环境，保留其余私有运行配置，没有输出其中的秘密。

本轮已完成独立的数据清理与持久部署，当前实际应用单元为 `painter.service`，数据库单元为 `painter-postgres.service`；应用配置位于 `~/.config/painter/runtime-v33.env`（0600）。后续配置公网 origin 时修改该文件，并运行 `systemctl --user restart painter.service`。原 `/tmp/painter-next-dev.env` 已指向这份持久配置；不要从旧备份另起实例。

- 当前正式应用监听 `192.168.2.209:18080`。同机访问 `http://192.168.2.209:18080/readyz` 返回 200；`http://127.0.0.1:18080/readyz` 无法连接。因此当前 Tunnel Service URL 应填 **`http://192.168.2.209:18080`**。
- 将来若把应用改绑 `127.0.0.1:18080`，并确认 connector 与应用在同一网络命名空间，才可以填回环地址。`0.0.0.0` 是监听地址，不是 Service URL；也不应为 Tunnel 无故扩大应用的监听范围。
- 若 `cloudflared` 在容器里，容器内的 `127.0.0.1` 指向该容器。应使用实际可达的宿主地址，或将其加入应用私有网络后使用服务名，例如 `http://app:8080`；不能照搬宿主机的回环示例。[现有 Compose](../compose.yaml)
- Go 服务使用 HTTP `ListenAndServe`，当前没有直接提供 TLS；公网 443 TLS 需要由反向代理或 Tunnel 链路承接。[服务入口](../cmd/painter/main.go)
- `PUBLIC_BASE_URL` 决定 canonical、站点地图、GitHub callback 和 Cookie 的 Secure 标志；生产配置要求 HTTPS。它必须是一个完整 origin，不能带 `/blog` 等路径前缀。[配置校验](../internal/app/config.go)
- 登录、注册与会话写请求严格校验 `Origin == PUBLIC_BASE_URL`，并校验 CSRF。前端 API 使用同源 Cookie，不应拆成另一个 API 域名，也不应由代理伪造或重写浏览器的 `Origin`。[同源校验](../internal/app/core.go)、[本地登录校验](../internal/app/local_auth.go)、[前端 API](../web/frontend/src/api.ts)
- 当前 SSR 页面、API 和私有资源按 `private, no-store` 返回；`/assets/` 静态资源按 `public, max-age=31536000, immutable` 返回。不能仅因文章公开就覆盖响应头缓存整站 HTML。[页面与静态资源响应](../internal/app/web.go)、[API 响应](../internal/app/core.go)

### 需要明确保留的反代限制

当前全局限流和登录限流使用直接连接的 `r.RemoteAddr`，刻意忽略未经信任的转发头。本轮已加入仅用于识别 HTTPS 协议的可信代理白名单；尚未加入真实访客 IP 的解析机制，白名单不会改变限流身份。[公网安全说明](PUBLIC_SECURITY.md)、[全局限流](../internal/app/core.go)、[登录限流](../internal/app/local_auth.go)

因此，Tunnel 后的访问可能都计入 connector 的同一个来源地址；Caddy 转发后也会有相同问题。低流量个人站可以先验证，但登录、注册或写操作较集中时可能互相触发 429。Cloudflare 边缘规则可以增加按访客的保护，**不能解除应用已有的共享限额**。公开前应实测正常访问量，监控 429；后续若需要真实 IP 限流，应单独实现“仅接受指定代理来源、解析受信任头”的策略，不能简单相信任意 `X-Forwarded-For` 或关闭全部限流。

## 两套方案共同需要的应用配置

先确定一个长期使用的域名，例如 `blog.example.com`。保留现有数据库、媒体、应用加密密钥和分析密钥，只修改访问相关配置；不要通过重新安装来生成一套新秘密。

以下仅展示需要调整的非秘密配置；实施时应修改该部署真正使用的私有配置，而不是另外启动一个指向旧数据的实例：

```dotenv
ENVIRONMENT=production
PUBLIC_BASE_URL=https://blog.example.com
# 当前宿主机部署可先保持此绑定：
LISTEN_ADDR=192.168.2.209:18080
```

实施检查清单：

1. 备份现有配置、数据库和媒体；正在旧 origin 编辑的文章先保存或导出。本地草稿和会话不会自动搬到新域名。
2. 在维护窗口同步切换公开域名和应用配置。此后读写、登录和管理都通过 `https://blog.example.com`；原 LAN IP 可继续作为受限诊断入口，但不再是另一个完整登录入口。
3. 若启用 GitHub 登录，将 GitHub OAuth App 的 callback 登记为 `https://blog.example.com/auth/github/callback`，并核对后台显示的 callback；已有显式 `GITHUB_CALLBACK_URL` 也必须匹配。[GitHub 登录说明](GITHUB_LOGIN_V7.md)
4. 保留 `HttpOnly; Secure; SameSite=Lax` Cookie、原有 Origin/CSRF/CAS 和账号权限。域名切换后重新登录，不通过放宽 Cookie 或 CORS 修复旧会话。
5. PSN 新凭据只在新 HTTPS 后台输入，不把 NPSSO 放在链接、DNS、Tunnel 配置、截图或聊天中。当前前端和后端都有安全连接检查；公开 origin 改为 HTTPS后，仍应限制应用原始 HTTP 端口的可访问范围。[PSN 后端检查](../internal/app/home_psn.go)
6. PostgreSQL继续保留在本机或私有网络；公网只暴露博客所需入口。Tunnel 不需要暴露 PostgreSQL、备份目录或运维端口。

## 方案 A：公网 IPv6 + DDNS + 橙云

### A1. 先确认是否具备条件

- 服务器自身有可用的全球单播 IPv6，而非只有路由器 WAN 地址、链路本地地址或内网地址。选择适合服务器长期使用的地址，避免把短期隐私地址写入 AAAA。
- 从另一条具备 IPv6 的外网验证，能够到达该服务器的 TCP 443；“本机可以 IPv6 上网”不能证明外部可以连入。
- 路由器 IPv6 防火墙与主机防火墙均允许所需流量。IPv4 的端口转发配置不能替代 IPv6 入站规则；运营商阻断 443 时，本方案按本文默认链路不可用。
- 有域名的 Cloudflare DNS 管理权限，可以创建一条专用 AAAA 记录和受限的 DNS 更新令牌。

这些条件尚未在当前家庭公网链路验证，不能因局域网服务正常就判定方案 A 可用。

### A2. 在 443 提供经过验证的 TLS 回源

普通 Cloudflare 橙云支持固定的一组 HTTP/HTTPS 端口，**18080 不在其中**。本方案采用 `443 → 18080` 的 TLS 反向代理，不把访问地址写成 `https://blog.example.com:18080`。[支持的代理端口](https://developers.cloudflare.com/fundamentals/reference/network-ports/)

可使用 Caddy；以下是宿主机安装、手动提供证书时的示例，不替换现有 Compose 的 `app:8080` 配置：

```caddyfile
blog.example.com {
    tls /etc/caddy/certs/blog-origin.pem /etc/caddy/certs/blog-origin.key
    reverse_proxy 192.168.2.209:18080
}
```

证书必须覆盖真实域名、在有效期内，并配齐所需证书链。可选公众信任 CA 或 Cloudflare Origin CA；私钥只放服务器的受限目录。Cloudflare 设置为 **Full (strict)**，保留源站证书验证。[Full (strict) 要求](https://developers.cloudflare.com/ssl/origin-configuration/ssl-modes/full-strict/)、[Caddy TLS](https://caddyserver.com/docs/caddyfile/directives/tls)、[Caddy 反向代理](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)

Origin CA 证书面向 Cloudflare 回源，不被普通浏览器默认信任。若以后要绕过橙云直接访问，必须考虑公众信任证书，不能把“关橙云”当成无条件可用的回退。[Origin CA 的信任边界](https://developers.cloudflare.com/ssl/origin-configuration/origin-ca/troubleshooting/)

若只允许 Cloudflare 回源，可在防火墙按其官方 IPv6 地址范围收紧 443，并维护更新；不要复制一份多年不更新的地址表。验证需要从 Cloudflare 侧完成，临时直连探测还需相应的受控放行。[Cloudflare IP 范围](https://www.cloudflare.com/ips/)

### A3. 建立并维护 AAAA

创建 `blog.example.com` 的 AAAA，内容为服务器真实公网 IPv6，开启代理。不要给同名同时保留 Tunnel CNAME；从一种方案切换到另一种时，应明确替换记录。

DDNS 更新任务的接口形态如下。这是请求规范，不包含可执行令牌：

```text
PATCH https://api.cloudflare.com/client/v4/zones/ZONE_ID/dns_records/RECORD_ID
Authorization: Bearer 从受限凭据文件读取的DNS令牌
Content-Type: application/json
```

```json
{
  "type": "AAAA",
  "name": "blog.example.com",
  "content": "2001:db8::1234",
  "proxied": true,
  "ttl": 1
}
```

更新令牌只授予目标 zone 所需的 DNS 写权限，若更新器需发现 zone/record 再增加对应只读权限；不要使用 Global API Key。`ttl: 1` 表示自动 TTL，不是 1 秒。DDNS 可以由受控脚本或现有客户端承担，Painter 当前没有内置该更新器。[动态 DNS 方案](https://developers.cloudflare.com/dns/manage-dns-records/how-to/managing-dynamic-ip-addresses/)、[DNS 更新 API](https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/edit/)

建议更新任务满足：

1. 启动时及网络地址变化时检查；定时补查可先设为每 5 分钟，再按实际前缀变化情况调整。
2. 读取服务器正确网卡的有效公网地址，与 Cloudflare API 中该记录的 `content` 比较。橙云开启后，公共 DNS 查询看到的是边缘地址，不能据此判断源站 AAAA 是否已更新。
3. 地址为空、仅有非公网地址、API失败时不写入错误地址、不删除最后一条可用记录；失败重试采用退避，并发任务串行执行。
4. 保持 `proxied: true`，仅更新指定记录；凭据文件限制读取权限，日志不打印授权头。
5. 实测一次路由器重新拨号或 IPv6 前缀变化后的恢复过程。DDNS 不消除变化期间的回源中断。

## 方案 B：Cloudflare Tunnel（推荐）

### B1. 用命名 Tunnel 建立稳定域名

1. 将真实域名接入 Cloudflare，确认有配置权限。正式站点使用命名 Tunnel；临时 `trycloudflare.com` 地址不作为长期 canonical、OAuth callback 或后台入口。
2. 从官方发行渠道安装 `cloudflared`，记录版本。建议在博客主机上运行，减少跨机器的最后一跳。
3. 在 Cloudflare 控制台创建 Tunnel，登记 connector，并新增 Published application route：

| 字段 | 当前部署应填写 |
| --- | --- |
| Public hostname | `blog.example.com`，替换为真实域名 |
| Service URL | `http://192.168.2.209:18080` |
| 路径 | 整个站点，保留原始路径与查询参数 |
| 额外 TLS 参数 | 当前 HTTP origin 不需要；不要把 HTTP 端口写成 HTTPS |

4. 控制台创建路由时会为该 hostname 建立指向 Tunnel 的 DNS 记录。若已有同名 A/AAAA/CNAME，先核对冲突；不把公网 hostname 再填为 Service URL，否则可能形成回环。
5. 用系统服务管理 connector 的启动、异常重启和日志。按实际安装方式配置；不要同时启动多份不明来源的 connector，也不要把 Tunnel token 放入仓库或复制到聊天里。

上述入口与字段依据当前官方流程；控制台布局更新时以实际显示为准。[创建与发布应用](https://developers.cloudflare.com/tunnel/get-started/)、[Tunnel 路由](https://developers.cloudflare.com/tunnel/concepts/routing/)

### B2. 本地管理配置的等价示例

如希望配置随服务器运维管理，可以按官方 CLI 流程先创建命名 Tunnel、取得 UUID 与凭据，再使用此配置。它与控制台管理是两种管理方式，不应混用同一份路由配置。

```yaml
# /etc/cloudflared/painter.yml
tunnel: REPLACE_WITH_TUNNEL_UUID
credentials-file: /etc/cloudflared/REPLACE_WITH_TUNNEL_UUID.json
ingress:
  - hostname: blog.example.com
    service: http://192.168.2.209:18080
  - service: http_status:404
```

凭据 JSON 只授权给实际运行 connector 的账号读取；目录与文件使用受限权限。账户级管理证书与运行所需的 Tunnel 凭据分开保管，备份时也按秘密处理。配置最后保留拒绝未匹配 hostname 的规则。[本地管理创建流程](https://developers.cloudflare.com/tunnel/features/locally-managed-tunnels/create-local-tunnel/)、[配置文件说明](https://developers.cloudflare.com/tunnel/features/locally-managed-tunnels/configuration-file/)

后续实施时，在替换完真实 UUID、域名和路径后校验规则，再前台短测：

```bash
cloudflared tunnel --config /etc/cloudflared/painter.yml ingress validate
cloudflared tunnel --config /etc/cloudflared/painter.yml ingress rule https://blog.example.com
cloudflared tunnel --config /etc/cloudflared/painter.yml run
```

若 QUIC 连不上，先核对 UDP 7844；可用 `--protocol http2` 做受控对照测试，但 HTTP/2 仍需要 TCP 7844，并不是改走普通 HTTPS 443。默认 `auto` 支持 UDP 失败后回退到 HTTP/2；是否稳定必须由当前网络实测。[传输参数](https://developers.cloudflare.com/tunnel/reference/run-parameters/)

### B3. 分清三段协议

- 浏览器到 Cloudflare：使用公开域名 HTTPS。
- Cloudflare 到 `cloudflared`：加密 Tunnel。
- `cloudflared` 到 Painter：当前同机地址是 HTTP；若跨不可信网络，应增加经验证的 HTTPS。

Tunnel 的最后一跳由 Service URL 决定，不能靠 zone 的 Full/Flexible 设置把 HTTP 监听变成 HTTPS。若以后回源到已有 HTTPS 代理，使用匹配的 `https://` 地址、正确的 Origin Server Name 和可信 CA，保持证书验证开启。Tunnel 显示 Healthy 只证明 connector 到 Cloudflare 的连接，并不证明 Painter 可达。[HTTPS origin 排障](https://developers.cloudflare.com/tunnel/troubleshooting/https-origins/)

两套方案都不通过 **Flexible** 修复重定向问题。公开 HTTPS 与回源强制 HTTPS 的配置冲突会造成循环，应修正协议与重定向链路。[重定向循环原因](https://developers.cloudflare.com/ssl/troubleshooting/too-many-redirects/)

## 缓存与安全配置

初次发布采用保守且可验证的规则：**只有 `/assets/` 静态资源沿用服务器的公开缓存头，其余路径绕过边缘缓存**。不设置覆盖源站指令的 Edge TTL，不开启全站 Cache Everything。

尤其不能缓存 `/login`、`/register`、`/setup`、`/studio`、`/me`、`/auth/`、`/api/` 或携带登录态的响应。`/media/` 同时可能承载不同可见性的资源，不能因为是图片就按目录统一强制公开缓存。后续若增加公开 HTML 或媒体边缘缓存，必须另行设计发布/撤回失效、权限变化和会话隔离。[Cloudflare 默认缓存](https://developers.cloudflare.com/cache/get-started/)、[动态页面与登录缓存故障](https://developers.cloudflare.com/cache/troubleshooting/dynamic-content-and-login-issues/)

Cloudflare/Tunnel 不替代 Painter 的账号权限、CSRF 或敏感凭据检查。仍保留站内登录；若另外引入 Cloudflare Access，需单独评估管理页面、API和 GitHub callback 的完整路径，本文不宣称已接入 Access 或 2FA。

## 实施后验收

先从实际 connector 所在的机器或容器检查 Service URL，再从另一条外网检查公开域名。下面的命令不含凭据；目前只执行过当前 LAN 与回环的只读探测，尚未执行域名测试：

```bash
# 当前真实回源地址
curl --fail --silent --show-error --max-time 5 http://192.168.2.209:18080/readyz

# 域名配置完成后，在外网执行
curl --fail --silent --show-error --max-time 15 https://blog.example.com/readyz
curl --silent --show-error --output /dev/null --max-time 20 \
  --write-out 'HTTP=%{http_code} DNS=%{time_namelookup}s TLS=%{time_appconnect}s TTFB=%{time_starttransfer}s TOTAL=%{time_total}s\n' \
  https://blog.example.com/
```

| 验收项 | 通过标准 |
| --- | --- |
| 回源与域名 | 本地 `/readyz` 和公网 `/readyz` 均200；证书有效，无反复重定向 |
| 规范地址 | 文章 canonical、站点地图和分享地址使用同一公开 HTTPS origin |
| 本地登录 | 登录成功、刷新保留会话；Cookie具备 Secure/HttpOnly/SameSite；退出后会话失效 |
| 后台写入 | 管理员按正常CSRF流程保存一项可恢复的设置成功；错误origin仍被拒绝，不为公网访问放宽校验 |
| OAuth | 若已启用GitHub，真实登录能返回新域名；无旧IP callback、旧state或其他账号误绑定 |
| 私有内容与缓存 | 未登录不可读草稿；登录/API响应无边缘HIT；Cookie不丢失，私有媒体不被共享缓存 |
| 编辑与媒体 | 草稿保存、发布、上传、图片预览和撤回按实际业务验收，切域名不丢本地未保存工作 |
| 限流 | 正常登录/注册/创作不会因共享代理来源频繁429；记录实际失败量和触发窗口 |
| 断线恢复 | 服务器重启后服务恢复；Tunnel重连或DDNS地址变化有告警与可观测恢复过程 |

涉及账号、发布、上传的验收应先在候选环境进行；正式环境只用站长明确允许的可恢复内容。测试结果应记录日期、网络、客户端、HTTP状态和耗时，截图和日志隐去 Cookie、token、NPSSO 与私密文章。

## 故障定位与访问质量

| 现象 | 优先排查 |
| --- | --- |
| LAN正常、Tunnel回源失败 | Service URL是否误写127.0.0.1；容器网络命名空间；HTTP端口是否被写成HTTPS |
| connector Healthy，公开页面仍502 | 直接探测本地服务；Healthy不等于origin健康；查看应用和connector对应时间日志 |
| DDNS方案连接超时 | 服务器真实公网IPv6、前缀变化、AAAA内容、路由器/主机443规则、运营商入站策略 |
| 回源证书错误 | 域名/SNI、有效期、CA与证书链；修证书，保持Full(strict)，不长期关闭验证 |
| 页面无限跳转 | Service URL协议、源站跳转、Cloudflare跳转规则是否互相冲突；不要来回切Flexible试运气 |
| 页面可读但登录/保存403 | PUBLIC_BASE_URL与浏览器Origin是否完全相同；旧域名Cookie、CSRF、错误缓存或代理改写 |
| 登录反复掉线 | 是否强制缓存动态HTML或移除了Set-Cookie；是否仍从旧IP/旧协议访问 |
| 多个正常用户同时429 | 当前应用将代理地址作为共享来源；核查本文的真实IP限流限制，而非只重启应用 |

两条方案都经过 Cloudflare，**不承诺国内三网的延迟、稳定性或必然加速**。上线前应分别用移动网络、家庭宽带、IPv4/IPv6客户端，在日间和晚高峰测首页、文章、图片及登录；观察 TTFB、失败率和上传耗时。Tunnel 的连接恢复能力也不能弥补本地断电、断网、上行不足或主机停机。

当前待确认：真实域名及 Cloudflare 权限、家庭公网 IPv6 可入站性、7844出站质量、证书方案和实际流量。未取得这些条件前，保持当前 LAN 服务；下一步优先在独立测试子域配置 Tunnel，完成验收后再切换正式公开 origin。
