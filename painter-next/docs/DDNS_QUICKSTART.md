# Painter 的 IPv6 DDNS 接入

DDNS 负责在家庭公网 IP 变化时，自动更新域名的 DNS 记录。它不负责内网穿透，也不会自动打开路由器防火墙。[DNS 动态更新说明](https://developers.cloudflare.com/dns/manage-dns-records/how-to/managing-dynamic-ip-addresses/)

如果希望博客流量直接到家里，可使用：

```text
ddns-go → 自动把域名 AAAA 更新为本机当前公网 IPv6
访客 → 域名解析 → 家庭 IPv6 → Caddy HTTPS → Painter
```

## 本机检查结果

2026-11-11 只读检查：

- 网卡为 `ens2`，有 `2409:…:65f7` 形式的动态全球单播 IPv6，当前不是 temporary 地址；另有内网 ULA 和链路本地地址，不能用它们做公网 AAAA。
- IPv6 直接请求 Cloudflare 成功返回 200；另外两个 IPv6 目标超时，不能据此承诺所有外网链路可达。**外网到本机的入站访问尚未验证。**
- Painter 正常运行，仍只监听 `192.168.2.209:18080`，没有 IPv6 HTTP 监听。
- 本机已安装 Caddy，80/443 暂无监听。`ddns-go` 尚未安装；本轮未修改 DNS、路由器、应用配置或启动公网服务。
- 公网 IPv4 是否可入站，需要将路由器 WAN 地址与出口地址核对；当前只确认本机使用私网 IPv4，暂不配置 A 记录。

下面以 `blog.example.com` 为示例，域名需要换成自己的。DNS 服务商与路由器型号尚待确认。

## 1 先确定是否需要代理

| 模式 | 流量路径 | 要知道的区别 |
| --- | --- | --- |
| 纯 DNS 直连 | 访客 → 家庭服务器 | 不经过 Cloudflare HTTP 代理；当前如果只有 IPv6 源站，纯 IPv4 访客无法访问 |
| Cloudflare 橙云 | 访客 → Cloudflare → 家庭服务器 | 仍使用 Cloudflare 代理/CDN，需要回源能入站，继续适用对应使用条款 |

如果 DNS 放在 Cloudflare，选 **DNS only（灰云）**就是第一种；使用 Cloudflare 做 DNS 不等于使用其网页代理。阿里云、DNSPod 等也可以只做 DNS，不必为 DDNS 迁移域名。[Cloudflare 代理状态区别](https://developers.cloudflare.com/dns/proxy-status/)

## 2 安装 ddns-go 并选择本机 IPv6

从 [ddns-go 官方发布页](https://github.com/jeessy2/ddns-go/releases)下载 Linux x86_64 版本并解压，在解压目录执行以下服务安装命令。此步骤需要在服务器终端输入 sudo 密码：

```bash
sudo ./ddns-go -s install -l 192.168.2.209:9876
```

然后在局域网浏览器打开 `http://192.168.2.209:9876`，完成管理员初始化。保持“禁止从公网访问”，路由器不要放行 9876。配置文件属于私有凭据，不放进项目仓库。[官方安装与 IPv6 说明](https://github.com/jeessy2/ddns-go)

填写这些项目：

| 配置项 | 当前应选择 |
| --- | --- |
| DNS 服务商 | 域名当前使用的 DNS 服务商 |
| IPv4 | 暂时关闭 |
| IPv6 | 开启 |
| 获取方式 | 网卡 |
| 网卡 | `ens2` |
| 地址 | 当前 `2409:…` 的公网 IPv6，不选 `fd…` 或 `fe80…` |
| 域名 | `blog.example.com` |
| DNS 记录 | AAAA |
| 同步间隔 | 默认 5 分钟可先保留 |

不要硬编码当前 `2409:…` 的完整动态前缀；前缀变化后要自动读取新地址。如果以后出现多个公网 IPv6，选择非 temporary、非 deprecated 的服务器地址。

如果使用 Cloudflare，在 [API Tokens 页面](https://dash.cloudflare.com/profile/api-tokens)通过 **Edit zone DNS** 模板创建令牌，范围限定为自己的这个域名，再填入 ddns-go 的本地配置页，不使用 Global API Key。纯直连模式保持该 AAAA 为灰云，并确认 ddns-go 没有把代理重新打开。[官方令牌创建说明](https://developers.cloudflare.com/fundamentals/api/get-started/create-token/)

保存后，检查日志更新成功，DNS 中的 AAAA 与当前 `ens2` 的公网地址相符；同名旧 A 或 Tunnel CNAME 应先核对用途，不能混指其他实例。

## 3 配置路由器和 Caddy 的 HTTPS 入口

在路由器的 IPv6 防火墙中，仅对这台服务器放行 TCP 80/443；主机防火墙也需允许相应入站。这里通常是 IPv6 入站放行，不是 IPv4 NAT 端口转发。若服务器是虚拟机，还要核对宿主机和虚拟网络的入站路径。具体菜单需要看路由器型号。

Caddy 配置如下，仍代理到目前真实可达的 IPv4 地址：

```caddyfile
blog.example.com {
    reverse_proxy 192.168.2.209:18080
}
```

这个例子用于当前宿主机部署，不替换项目 Compose 里的 `app:8080` 代理。Caddy 要能对外监听 IPv6 80/443，并通过系统服务长期运行；当前普通用户绑定这两个端口需要相应系统权限。

域名解析正确且入站可达后，Caddy 可以自动申请、续期公众信任证书。默认 HTTP-01 / TLS-ALPN-01 分别需要外部能到达 80 / 443；不要使用仅被 Cloudflare 信任的 Origin CA 证书给浏览器直连。[Caddy HTTPS 要求](https://caddyserver.com/docs/automatic-https)

若这两个端口不能入站，DDNS 本身无法解决。其他 HTTPS 端口或 DNS-01 需要单独设计，DNS-01 取得证书也不会让被阻断的服务端口变得可达。

## 4 验证可达后再切换 Painter 网址

使用另一条具备 IPv6 的网络访问新域名。手机流量未必有 IPv6，先确认其网络能力；不以局域网内访问成功作为公网验收。

```bash
curl -6 --fail --silent --show-error --max-time 15 https://blog.example.com/readyz
```

应返回包含 `"status":"ready"` 的 JSON，证书校验正常。完成后再编辑本机 `~/.config/painter/runtime-v33.env` 的三项：

```dotenv
PUBLIC_BASE_URL=https://blog.example.com
GITHUB_CALLBACK_URL=https://blog.example.com/auth/github/callback
ENVIRONMENT=production
```

保留原数据库、密钥、媒体目录和 `LISTEN_ADDR=192.168.2.209:18080`，然后：

```bash
systemctl --user restart painter.service
```

启用 GitHub 登录时，同步修改 GitHub OAuth App 的回调。此后登录和管理统一走新 HTTPS 域名；原局域网 HTTP 地址仍可阅读和诊断，不能同时当另一个完整登录 origin。若希望内网绕开公网但保持完整登录，使用同一正式域名的内网 DNS 解析和可信 HTTPS。

最后还要观察一次公网 IPv6 前缀变化，确认 ddns-go 更新 AAAA、路由器规则仍匹配该主机、Caddy 的监听和 HTTPS 恢复正常。当前配置在外部认证/写操作集中时仍有共享代理来源限流边界，见 [完整网络说明](NETWORK_ACCESS.md)。
