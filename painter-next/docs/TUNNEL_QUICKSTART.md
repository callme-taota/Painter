# Painter 接入 Cloudflare Tunnel

按这五步，把当前本地服务器的 Painter 接到自己的域名。无需 DDNS、公网 IP 或路由器端口转发；服务器需要保持开机、联网。**当前已接入并启用正式网址 https://blog.taota.cc。** 用户级服务 `painter-tunnel.service` 已启用自动启动；下面保留接入步骤供维护参考，不需要重复创建隧道或重装服务。

以下使用当前实际域名 `blog.taota.cc`。本机连接器使用用户级服务，日常重启不需要 sudo。

```text
外网 → https://blog.taota.cc → Cloudflare → 本机 cloudflared → http://192.168.2.209:18080
```

## 1 在 Cloudflare 创建 Tunnel

打开 [Cloudflare 控制台](https://dash.cloudflare.com/)，进入 **Networking → Tunnels → Create Tunnel**，名称填 `painter`。安装环境选择 **Linux / Debian 或 Ubuntu / amd64**，使用 `cloudflared` 连接器。

保留这个页面，下一步在运行 Painter 的 Ubuntu 服务器上操作。[官方创建流程](https://developers.cloudflare.com/tunnel/get-started/)

当前本机已连接到你创建的隧道，无需再新建。若仍停在安装连接器页面，刷新确认连接器上线后点击 Continue。

## 2 在服务器安装并启动连接器

当前 Ubuntu / x86_64 服务器已安装官方独立程序 **2026.10.7**，并与官方发布资产的 SHA-256 核对一致。路径是 `/home/callumchan/.local/bin/cloudflared`。本机采用现有用户的 systemd 服务管理连接器，凭据保存在权限为 0600 的 `~/.config/painter/cloudflared-token`，不写入命令参数或仓库。无需再执行 apt 安装或 `sudo ... service install`，避免重复启动连接器。

```bash
/home/callumchan/.local/bin/cloudflared --version
systemctl --user is-active painter-tunnel.service
curl --fail --silent --show-error --max-time 5 http://127.0.0.1:20241/ready
```

程序来自 [Cloudflare 官方发布](https://github.com/cloudflare/cloudflared/releases/tag/2026.10.7)。当前服务为 active/running，连接器 `/ready` 返回 200，实际有四条 QUIC 连接。用户已启用 Linger，用户服务不依赖当前终端保持打开。日常管理命令：

```bash
systemctl --user restart painter-tunnel.service
systemctl --user stop painter-tunnel.service
```

在控制台应能看到已连接的连接器。连接成功只证明本机到 Cloudflare 的隧道已经建立，还需要下面的域名路由，才能访问 Painter。[官方连接器流程](https://developers.cloudflare.com/tunnel/get-started/)

这条控制台管理流程只需要登录 Cloudflare 网页，不需要另执行 `cloudflared tunnel login`。

## 3 绑定博客域名

在刚才的 Tunnel 中进入 **Routes → Add route → Published application**，填写：

| 字段 | 填写内容 |
| --- | --- |
| Subdomain | `blog` |
| Domain | `taota.cc` |
| Path | 留空，转发整个站点 |
| Service type | **HTTP** |
| Service URL / 地址 | `192.168.2.209:18080` |

如果控制台只提供一个完整 URL 输入框，就填 **`http://192.168.2.209:18080`**。当前服务未监听 `127.0.0.1:18080`，不要填 `localhost`；也不要把这里填成公网域名或 HTTPS。

保存路由后，控制台会创建对应 DNS 记录；若提示同名记录冲突，先核对旧记录用途，或改用未占用的子域名。[官方路由说明](https://developers.cloudflare.com/tunnel/concepts/routing/)

## 4 把 Painter 的正式网址改成新域名

先保存编辑器里尚未保存的内容。在服务器编辑现有配置文件：

```bash
nano ~/.config/painter/runtime-v33.env
```

找到并修改下面三项，不要覆盖整个文件，也不要改数据库、密钥或媒体路径：

```dotenv
PUBLIC_BASE_URL=https://blog.taota.cc
GITHUB_CALLBACK_URL=https://blog.taota.cc/auth/github/callback
ENVIRONMENT=production
```

保留原来的 `LISTEN_ADDR=192.168.2.209:18080`。在 nano 中按 `Ctrl+O`、回车保存，再按 `Ctrl+X` 退出。重启 Painter：

```bash
systemctl --user restart painter.service
systemctl --user is-active painter.service
```

即使暂时不用 GitHub 登录，也要同步更新配置文件里已有的 `GITHUB_CALLBACK_URL`，否则新旧地址不一致会使应用启动校验失败。如果已启用 GitHub 登录，再到 GitHub 的 **Settings → Developer settings → OAuth Apps → 对应应用**，把 Authorization callback URL 改为 `https://blog.taota.cc/auth/github/callback`。

## 5 用手机流量验收

先在服务器检查本地回源：

```bash
curl --fail --silent --show-error --max-time 5 http://192.168.2.209:18080/readyz
```

应看到 `"status":"ready"`。随后让手机关闭 Wi-Fi、使用移动网络，打开 `https://blog.taota.cc`，再打开 `/login` 验证账号登录。

**接入后，登录、发文和后台设置统一使用新的 HTTPS 域名。** 原内网 `http://192.168.2.209:18080` 仍可用于公开阅读和健康检查，但不能继续作为独立的登录后台：当前应用只接受一个正式 Origin，且新会话 Cookie 只通过 HTTPS 发送。内网与外网的登录态也不会自动迁移。

内网若也要完整登录并且不经过 Tunnel，后续需要为同一个正式域名配置内网 DNS 与可信 HTTPS 回源；这不属于本次最简接入步骤。

## 遇到问题

| 现象 | 先检查 |
| --- | --- |
| Tunnel 未 Healthy | `journalctl --user -u painter-tunnel.service -n 50 --no-pager`；日志仅在本机查看，网络需允许连接 Cloudflare 的 7844 端口，UDP 或 TCP。[连接要求](https://developers.cloudflare.com/tunnel/configuration/) |
| Healthy 但网页 502 | 本地 `/readyz` 是否正常，Service 是否为 HTTP、地址是否为 `192.168.2.209:18080` |
| Painter 重启失败 | `journalctl --user -u painter.service -n 30 --no-pager`，先核对两处域名和 callback 是否一致 |
| 页面正常但登录或保存 403 | 浏览器地址必须与 `PUBLIC_BASE_URL` 完全一致，包括 HTTPS；不要从旧 IP 登录 |

Cloudflare 缓存先保持默认，不新增“全站强制缓存”规则，保留 Painter 自己的动态页面与登录响应缓存头。更多网络方案与代理限流边界见 [完整网络说明](NETWORK_ACCESS.md)。

核对日期：2026-11-11。域名已路由到 `http://192.168.2.209:18080`，正式配置的 `PUBLIC_BASE_URL`、GitHub callback 和 production 环境均已更新。IPv4/IPv6 公网 HTTPS、就绪接口、登录页面、匿名会话、canonical、robots 与 sitemap 已验证；没有执行真实账号登录或写入测试内容。原私有运行配置已备份在本机。

公网 HTTP 现已由 Painter 针对 `blog.taota.cc` 执行 308 跳转到 HTTPS，公网 HTTPS 响应启用一天有效的 HSTS。现有私有配置已设置 `ENFORCE_PUBLIC_HTTPS=true`、`TRUSTED_PROXY_CIDRS=192.168.2.209/32`；该精确来源是同机 cloudflared 的回源地址，不能扩大为整个局域网。内网 IP 的 HTTP 阅读与健康检查仍可用。详细规则、CSP 与静态资源保护见 [公网安全说明](PUBLIC_SECURITY.md)。无需为此启用影响其他子域名的 Cloudflare 全域 Always Use HTTPS。
