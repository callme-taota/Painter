# Painter 接入 Cloudflare Tunnel

使用 Cloudflare 控制台把 Painter 发布到 `https://blog.example.com`。服务器需要保持运行并能出站连接 Cloudflare，不要求公网 IP、DDNS 或路由器入站转发。域名和地址均为示例；已有部署应复用自己的 Tunnel、连接器和私有运行配置。

```text
公网浏览器 → https://blog.example.com → Cloudflare
                                  ↕ 加密隧道
                              cloudflared → http://127.0.0.1:8080 → Painter

内网设备 → http://192.168.1.10:8080 → Painter（仅在应用保留 LAN 监听时）
```

## 1. 确认回源

在实际运行 `cloudflared` 的网络中，确认能访问 Painter：

```bash
curl --fail --silent --show-error --max-time 5 http://127.0.0.1:8080/readyz
```

回环示例要求连接器与应用位于同一网络命名空间，且应用确实绑定 `127.0.0.1:8080`。若应用绑定局域网，则改用 `http://192.168.1.10:8080`；同一 Compose 网络可用 `http://app:8080`。`0.0.0.0` 不是回源地址。

## 2. 在控制台创建连接器

登录 [Cloudflare 控制台](https://dash.cloudflare.com/)，进入 **Networking → Tunnels → Create Tunnel**，填写名称，例如 `painter`。选择服务器实际操作系统和 CPU 架构，按页面的 **Install and Run** 说明安装并启动 `cloudflared`，连接后选择 Continue。[官方创建流程](https://developers.cloudflare.com/tunnel/get-started/)

页面提供的连接器令牌属于私有凭据，仅在受信任服务器上使用，不放进 Git、聊天、共享脚本或截图。将连接器按所选平台配置为持久服务；已有连接器无需重复安装或再建一套服务。控制台流程无需额外运行 `cloudflared tunnel login`。

## 3. 发布博客域名

选择该 Tunnel，进入 **Routes → Add route → Published application**：

| 字段 | 示例 |
| --- | --- |
| Subdomain | `blog` |
| Domain | `example.com` |
| Path | 留空，转发整个站点 |
| Service type | `HTTP` |
| Service URL | `127.0.0.1:8080`；完整输入框则填 `http://127.0.0.1:8080` |

地址必须使用第 1 步验证过的实际回源。不要填公开域名，也不要把 HTTP 应用端口写成 HTTPS。保存路由时核对同名 DNS 记录，已有记录冲突时先确认用途。[路由说明](https://developers.cloudflare.com/tunnel/concepts/routing/)

## 4. 配置 Painter 的公开地址

先备份部署配置和数据，保存或导出编辑器尚未保存的内容。原生部署编辑既有私有配置，例如 `~/.config/painter/runtime.env`；Compose 编辑原 `.env`，保留数据库、媒体与密钥，仅调整访问相关项：

```dotenv
PUBLIC_BASE_URL=https://blog.example.com
ENVIRONMENT=production
```

保留已验证的应用监听和 Service URL。若配置中已有 `GITHUB_CALLBACK_URL`，同步改为 `https://blog.example.com/auth/github/callback`；启用 GitHub 登录时，也更新 OAuth App 登记的回调。[GitHub 登录](GITHUB_LOGIN_V7.md)

用既有部署方式加载变更。原生用户服务若名为 `painter.service`，可执行 `systemctl --user restart painter.service`；Compose 的环境变量变更需要重新创建应用容器，不能仅重启。公开 HTTP 跳转与可信代理来源按 [网络配置](NETWORK_ACCESS.md) 设置，不为连接器扩大应用监听或代理白名单。

## 5. 分别验收内网和公网

从连接器网络重新检查回源，再让手机关闭 Wi-Fi，用移动网络访问：

```bash
curl --fail --silent --show-error --max-time 15 https://blog.example.com/readyz
```

确认就绪、公开页面和证书正常，再验证正式域名下的登录与退出。Tunnel Healthy 只表示连接器已连到 Cloudflare，不能代替回源健康和整站验收。

登录、注册、发文和管理统一使用 `https://blog.example.com`。内网 HTTP 可用于公开阅读和健康检查，前提是保留 LAN 监听；生产 Secure Cookie 不会发送到内网 HTTP 地址，应用也只接受一个正式 Origin。完整内网认证需要同一域名的内网 DNS 与可信 HTTPS。[局域网访问](LAN_ACCESS.md)、[账号安全](ACCOUNT_SECURITY.md)

公网浏览器到 Cloudflare 使用 HTTPS，Cloudflare 到连接器使用加密隧道。同机最后一跳可用受限 HTTP；跨不可信网络应使用经验证的 HTTPS，保持证书校验。[HTTPS 回源说明](https://developers.cloudflare.com/tunnel/troubleshooting/https-origins/)

## 排障

| 现象 | 先检查 |
| --- | --- |
| Tunnel 未 Healthy | 连接器进程与日志、DNS、7844 端口的 UDP/TCP 出站连通性。[连接要求](https://developers.cloudflare.com/tunnel/configuration/) |
| Healthy 但网页 502 | 实际回源地址与端口、监听范围、容器网络和 Service URL 协议 |
| 应用无法加载新配置 | 正式域名、已有 callback、配置文件实际加载位置和进程日志 |
| 页面正常但登录或保存失败 | HTTPS、浏览器 Origin、Cookie、CSRF、缓存和代理协议头 |

保留 Painter 的缓存头，不新增全站强制缓存规则，不对外发布数据库、备份或运维端口。其他网络方案与来源限流说明见 [网络入口](NETWORK_ACCESS.md)。
