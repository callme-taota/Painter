# 局域网访问

Painter 可以直接绑定局域网地址供同一网络的设备访问，也可以只监听本机回环地址，再通过反向代理或 Cloudflare Tunnel 提供入口。以下以服务器 `192.168.1.10`、端口 `8080` 为示例，请替换成自己的地址。

## 选择访问方式

| 用途 | 应用监听 | 浏览器入口 |
| --- | --- | --- |
| 独立的局域网 HTTP 预览 | `192.168.1.10:8080` | `http://192.168.1.10:8080` |
| 同机代理或 Tunnel 回源 | `127.0.0.1:8080` | `https://blog.example.com` |
| 同时保留内网阅读和公网入口 | `192.168.1.10:8080` | 内网 HTTP 阅读；正式域名 HTTPS 登录 |

`127.0.0.1` 只能由同一网络命名空间访问。手机不能用自己的 `localhost` 访问服务器；容器中的回环地址也不等于宿主机。回源地址选择见 [网络配置](NETWORK_ACCESS.md)。

## 配置独立 HTTP 预览

先按 [部署说明](DEPLOYMENT.md) 准备 PostgreSQL 18、密钥与媒体存储。原生部署在自己的私有运行配置中修改以下非秘密项，例如 `~/.config/painter/runtime.env`：

```dotenv
ENVIRONMENT=development
LISTEN_ADDR=192.168.1.10:8080
PUBLIC_BASE_URL=http://192.168.1.10:8080
ENFORCE_PUBLIC_HTTPS=false
```

运行配置由已有服务管理方式加载，二进制不会自动读取这个文件。若使用自己的 `painter.service`，修改其实际 `EnvironmentFile` 后重启应用；不要启动第二个指向同一数据库和媒体目录的临时实例。

Compose 部署在该部署原有 `.env` 中设置：

```dotenv
ENVIRONMENT=development
APP_BIND_ADDRESS=192.168.1.10
APP_PORT=8080
PUBLIC_BASE_URL=http://192.168.1.10:8080
ENFORCE_PUBLIC_HTTPS=false
```

环境配置变更需要重新创建应用容器才会生效，单纯 `docker compose restart` 不会载入新的环境变量。使用既有部署流程应用变更；不要重新生成数据库密码或加密密钥。PostgreSQL 保留在本机 Unix socket 或 Compose 私有网络，不为网页访问开放数据库端口。

该 HTTP 配置适用于独立预览。HTTP 局域网流量是明文，不能作为生产密码认证的传输保护。生产部署使用 HTTPS 的 `PUBLIC_BASE_URL`，配置方法见 [Tunnel 接入](TUNNEL_QUICKSTART.md) 或 [IPv6 DDNS 接入](DDNS_QUICKSTART.md)。

## 同时使用内网阅读和公网 HTTPS

当正式配置为 `PUBLIC_BASE_URL=https://blog.example.com` 时，可按需继续监听 `192.168.1.10:8080` 供内网公开阅读和健康检查。登录、注册、管理与写入统一通过正式 HTTPS 域名；应用只接受一个正式 Origin，Secure Cookie 不会发送到 HTTP 内网地址。

内网设备若也要绕过公网并完整登录，需要为同一个正式域名配置内网 DNS 和可信 HTTPS 入口。仅修改 hosts 或 DNS，不能让原来的 HTTP 端口自动支持 TLS。相关边界见 [账号安全](ACCOUNT_SECURITY.md)。

## 连通性与浏览器状态

先从服务器检查实际绑定地址，再从同一局域网的另一设备访问：

```bash
curl --fail --silent --show-error --max-time 5 http://192.168.1.10:8080/readyz
```

如果无法连接，依次核对应用监听地址与端口、设备所在子网、Wi-Fi 客户端隔离、虚拟机网络、路由器及主机防火墙。需要放行时，只允许可信子网访问应用端口，规则由管理员按实际网络配置；不要把预览端口映射到公网。

IP、端口、协议或域名变化都会改变 Origin。Cookie、浏览器草稿和本地恢复记录不会自动迁移；切换前保存或导出尚未保存的内容，切换后重新登录。GitHub 登录若已启用，也要同步核对新 `PUBLIC_BASE_URL` 对应的回调地址，见 [GitHub 登录](GITHUB_LOGIN_V7.md)。

局域网 HTTP 属于非安全上下文，部分浏览器能力会受限。前端的 UUID 生成使用密码学随机数兼容路径；这不改变 HTTP 的明文传输性质。[浏览器 randomUUID](https://developer.mozilla.org/en-US/docs/Web/API/Crypto/randomUUID)、[getRandomValues](https://developer.mozilla.org/en-US/docs/Web/API/Crypto/getRandomValues)
