# 公网传输与前端资源保护

浏览器要执行 JavaScript、显示文章，就必须能读取收到的内容。开发者工具能看到自己的 JS 和 JSON 是正常机制；把解密密钥一起发给浏览器，不能让访问者看不到内容。私钥、数据库凭据和第三方 API 密钥应留在服务端，私有接口由服务端鉴权。生产 JS 使用压缩与自有代码混淆构建，不发布 source map。

## V41：生产代码自动混淆

`npm run build` 默认在本机使用固定版本 `javascript-obfuscator`，对 `src` 下编译为 JS 的 79 个自有 TS/Vue 模块进行变量处理、字符串数组提取和 base64 编码。Vue、Tiptap、CodeMirror 等第三方库保持常规压缩，避免对其重复混淆。Go 单体仍只嵌入构建产物，生产服务器不增加 Node 或混淆器进程。`npm run dev` 保留可调试代码。

插件在 Vue/TS 编译后、Rollup 打包与资源哈希生成前运行，保留静态/动态 import 和 re-export 的字面路径，使异步 JS、CSS 与 preload 继续生成正确的资源地址。固定 seed 基于相对模块路径，不包含机器路径或时间；同样的源码和锁文件会生成相同资源。开启 source map 或未处理任何自有模块时构建失败。

使用 `browser-no-eval`，保留对象属性和接口字段；关闭控制流平坦化、死代码注入、防调试、自保护与字符串数组旋转，不放宽 CSP，也不拦截用户打开开发者工具。全部混淆在本地进行，不调用混淆器的付费云端 API。[混淆器官方选项](https://github.com/javascript-obfuscator/javascript-obfuscator)、[Vite 7 插件顺序](https://v7.vite.dev/guide/api-plugin#plugin-ordering)

本轮全量 JS gzip 从 546,765 B 增为 608,836 B（约 11.4%，包含惰性加载的后台资源）；首页入口从 9,790 B 增为 15,392 B（约增加 5.5 KiB）。这是代码可读性与浏览器负担之间的取舍，不是加密保密保证。锁文件固定构建依赖，并将新依赖链中的 Zod 固定为修复版本 4.6.7；本轮 `npm ci --ignore-scripts` 后审计为零已知漏洞。

```bash
cd web/frontend
npm ci
npm run test:obfuscation
npm run build
```

插件及专项测试位于 [build/obfuscation.ts](../web/frontend/build/obfuscation.ts) 和 [build/obfuscation.test.ts](../web/frontend/build/obfuscation.test.ts)。Docker/一键部署原有 `npm run build` 路径自动包含该步骤；只用 Go 构建时使用仓库内已混淆的 assets。旧浏览器已下载的 JS 无法撤回，刷新页面后会加载新的哈希产物。

## 本次实现

- 静态资源只提供真实普通文件，不再列出目录；拒绝点路径、构建 manifest、source map 和源码扩展。拒绝和缺失资源返回 404、`no-store`；正常 JS/CSS/字体保留 GET、HEAD、Range 与缓存行为。
- 页面设置 CSP：同源脚本与每响应随机 nonce；禁止任意内联脚本、eval、iframe、插件与外部 API 连接。主题初始化和 JSON-LD 带对应 nonce。编辑器动态样式、HTTPS 图片/音乐和 blob 图片预览继续允许。
- 增加防 iframe 嵌入和浏览器敏感权限限制。生成的 SVG 封面保留自身更严格的 CSP。
- 可配置公网 HTTP → HTTPS 的 308 跳转，保留路径与查询参数。跳转地址仅来自 `PUBLIC_BASE_URL`，不使用客户端提供的转发 Host。
- 公网 HTTPS 响应增加一天有效的 HSTS，不包含 `includeSubDomains` 或 preload，不影响同域名下其他站点。

## 当前 Tunnel 配置

在现有私有运行配置里保留数据库、媒体路径和密钥，仅配置以下非秘密项：

```dotenv
PUBLIC_BASE_URL=https://blog.taota.cc
ENVIRONMENT=production
ENFORCE_PUBLIC_HTTPS=true
TRUSTED_PROXY_CIDRS=192.168.2.209/32
```

`TRUSTED_PROXY_CIDRS` 只允许精确的代理连接来源。当前 cloudflared 与 Painter 在同一主机，连接 `192.168.2.209:18080`，其源地址为 `192.168.2.209`。应用只接受可信来源的单一 `X-Forwarded-Proto: https`；其他来源、多值或逗号值都不能把 HTTP 请求伪装成 HTTPS。不要扩大为整个局域网，更不能设为 `0.0.0.0/0`。同机进程也在这个信任边界内。

更换代理、改绑回环地址或改用容器时，应根据真实连接来源更新白名单。代理必须保留公网 Host，并正确覆盖 `X-Forwarded-Proto`，否则可能不跳转或循环跳转。未配置时 `ENFORCE_PUBLIC_HTTPS` 默认为 false，可信代理列表为空。

内网 IP 的 HTTP 阅读和健康检查保持可用，这段访问仍是明文；登录和管理统一使用正式 HTTPS 域名。公网浏览器到 Cloudflare 使用 TLS，Cloudflare 到 cloudflared 使用加密隧道，当前最后一跳是同机 HTTP 回源。

本次没有改动访客 IP 解析或限流身份：限流仍使用直接连接来源，Tunnel 下的共享限额边界见 [网络说明](NETWORK_ACCESS.md)。

## 维护与检查

```bash
systemctl --user is-active painter.service painter-tunnel.service
curl --noproxy '*' -sS -I http://blog.taota.cc/
curl --noproxy '*' -sS -I https://blog.taota.cc/
curl --noproxy '*' -sS -I https://blog.taota.cc/assets/
```

预期分别为 308（Location 为 HTTPS）、200（CSP/HSTS）、404（禁止目录列表）。不要在 Cloudflare 强制缓存全部 HTML/API；含 nonce 的页面和认证响应使用 `private, no-store`。不要把凭据、备份、任意配置 JSON 或开发源文件复制到 assets；资源路径拦截不是秘密扫描器。

自动化检查包括无数据库的资源/代理/CSP 测试、隔离 PostgreSQL 的完整 Go 测试，以及独立浏览器上下文中的页面和编辑器回归。公网匿名探针只能证明被检查的接口与响应，不能替代完整安全审计。

源码入口：[代理与 HTTPS](../internal/app/transport_security.go)、[CSP](../internal/app/security_headers.go)、[静态文件](../internal/app/assets_http.go)。

## 本机验收记录

2026-11-12（北京时间）：正式服务已更新。完整 `go test -race ./...` 在隔离 PostgreSQL 上通过，`go vet ./...` 与前端生产构建通过；Chrome 隔离回归 30 项通过，正常 CSP 违例、JS 异常、资源错误均为零，两次故意注入脚本均被拦截。公网匿名验收 19 项通过，包括 HTTP 跳转、IPv6 HTTPS、代理协议头覆盖、nonce 更新、后台/草稿 401、资源 404 与 LAN 就绪。未进行正式账号写入测试；隔离测试库、临时凭据已清理。

机器可读记录保存在本机 `tests/artifacts/v40/` 的 `verification.json`、`browser-security.json`、`public-security.json` 和 `release.json`。旧二进制与私有配置已备份，路径记录在同目录 `backup-path.txt`；不应上传私有配置或测试会话。

V41 混淆发布验收（2026-11-12）：4 项构建专项、36 项隔离编辑器/页面回归、37 项匿名首页桌面/移动视差回归通过；安全头/资源/HTTPS 的 Go 竞态专项通过。公开域名的混淆入口文件与本地候选 SHA-256 一致，公网安全探针 19 项通过。配置、服务端业务和正式内容未变；临时测试账号和数据库已清理。V41 本机记录位于 `tests/artifacts/v41/`。
