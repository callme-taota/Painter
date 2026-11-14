# 公网传输与前端资源保护

浏览器要执行 JavaScript、显示文章，就必须能读取收到的内容。开发者工具能看到自己的 JS 和 JSON 是正常机制；把解密密钥一起发给浏览器，不能让访问者看不到内容。私钥、数据库凭据和第三方 API 密钥应留在服务端，私有接口由服务端鉴权。生产 JS 使用压缩与自有代码混淆构建，不发布 source map。

## 生产代码自动混淆

`npm run build` 在构建机使用锁定版本的 `javascript-obfuscator`，对 `src` 下编译为 JS 的自有 TS/Vue 模块进行变量处理、字符串数组提取和 base64 编码。Vue、Tiptap、CodeMirror 等第三方库保持常规压缩。Go 单体只嵌入构建产物，生产服务器不增加 Node 或混淆器进程；`npm run dev` 保留可调试代码。

插件在 Vue/TS 编译后、Rollup 打包与资源哈希生成前运行，保留静态/动态 import 和 re-export 的字面路径，使异步 JS、CSS 与 preload 生成正确的资源地址。固定 seed 基于相对模块路径，不包含机器路径或时间；同样的源码和锁文件会生成相同资源。开启 source map 或未处理任何自有模块时构建失败。

使用 `browser-no-eval`，保留对象属性和接口字段；关闭控制流平坦化、死代码注入、防调试、自保护与字符串数组旋转，不放宽 CSP，也不拦截用户打开开发者工具。全部混淆在本地进行，不调用付费云端 API。混淆会增加资源体积和浏览器负担，只是代码可读性的取舍，不提供加密保密保证。构建依赖以锁文件为准；升级依赖后应重新运行专项测试和前端构建。[混淆器官方选项](https://github.com/javascript-obfuscator/javascript-obfuscator)、[Vite 插件顺序](https://v7.vite.dev/guide/api-plugin#plugin-ordering)

```bash
cd web/frontend
npm ci
npm run test:obfuscation
npm run build
```

插件及专项测试位于 [build/obfuscation.ts](../web/frontend/build/obfuscation.ts) 和 [build/obfuscation.test.ts](../web/frontend/build/obfuscation.test.ts)。Docker/一键部署的前端构建自动包含该步骤；直接 Go 构建使用仓库内已混淆的 `web/assets/`。旧浏览器已下载的 JS 无法撤回，刷新页面后会加载新的哈希产物。

## 资源与响应保护

- 静态资源只提供真实普通文件，不列出目录；拒绝点路径、构建 manifest、source map 和源码扩展。拒绝和缺失资源返回 404、`no-store`；正常 JS/CSS/字体保留 GET、HEAD、Range 与缓存行为。
- 页面设置 CSP：同源脚本与每响应随机 nonce；禁止任意内联脚本、eval、iframe、插件与外部 API 连接。主题初始化和 JSON-LD 带对应 nonce。编辑器动态样式、HTTPS 图片/音乐和 blob 图片预览继续允许。
- 普通页面禁止被 iframe 嵌入，并限制浏览器敏感权限；`/embed` 是明确允许外站嵌入的无脚本分享卡片，具体限制见[专栏与分享](SERIALS_AND_SHARING.md)。生成的 SVG 封面保留自身更严格的 CSP。
- 可配置公网 HTTP → HTTPS 的 308 跳转，保留路径与查询参数。跳转地址仅来自 `PUBLIC_BASE_URL`，不使用客户端提供的转发 Host。
- 公网 HTTPS 响应增加一天有效的 HSTS，不包含 `includeSubDomains` 或 preload，不影响同域名下其他站点。

## Tunnel 配置

在部署的私有运行配置里保留数据库、媒体路径和密钥，配置以下非秘密项。域名和代理地址仅为示例，须替换为自己的公开域名及应用实际观察到的代理连接来源：

```dotenv
PUBLIC_BASE_URL=https://blog.example.com
ENVIRONMENT=production
ENFORCE_PUBLIC_HTTPS=true
TRUSTED_PROXY_CIDRS=192.168.1.10/32
```

`TRUSTED_PROXY_CIDRS` 只允许精确的代理连接来源。例如 cloudflared 与 Painter 在同一主机、通过该主机的局域网地址回源时，可以只信任 `192.168.1.10/32`。应用只接受可信来源的单一 `X-Forwarded-Proto: https`；其他来源、多值或逗号值都不能把 HTTP 请求伪装成 HTTPS。不要扩大为整个局域网，更不能设为 `0.0.0.0/0`。同机进程也在这个信任边界内。

更换代理、改绑回环地址或改用容器时，应根据真实连接来源更新白名单。代理必须保留公网 Host，并正确覆盖 `X-Forwarded-Proto`，否则可能不跳转或循环跳转。未配置时 `ENFORCE_PUBLIC_HTTPS` 默认为 false，可信代理列表为空。

内网 IP 的 HTTP 阅读和健康检查可以保持可用，这段访问仍是明文；生产登录和管理使用 `PUBLIC_BASE_URL` 配置的 HTTPS 域名。公网浏览器到 Cloudflare 使用 TLS，Cloudflare 到 cloudflared 使用加密隧道；如果回源使用 HTTP，最后一跳仍受部署网络的信任边界约束。

默认限流使用直接连接来源。Tunnel 下可显式启用 `TRUST_CLOUDFLARE_CONNECTING_IP`，仅对命中可信代理的连接解析合法的 `CF-Connecting-IP`；完整校验与限制见[账号安全](ACCOUNT_SECURITY.md#cloudflare-tunnel-下的访问限制)。

## 维护与检查

使用自己的服务名和域名执行检查：

```bash
systemctl --user is-active painter.service painter-tunnel.service
curl --noproxy '*' -sS -I http://blog.example.com/
curl --noproxy '*' -sS -I https://blog.example.com/
curl --noproxy '*' -sS -I https://blog.example.com/assets/
```

在启用公网 HTTPS 跳转的配置下，预期分别为 308（Location 为 HTTPS）、200（CSP/HSTS）、404（禁止目录列表）。不要在 Cloudflare 强制缓存全部 HTML/API；含 nonce 的页面和认证响应使用 `private, no-store`。不要把凭据、备份、任意配置 JSON 或开发源文件复制到 assets；资源路径拦截不是秘密扫描器。

自动化检查包括无数据库的资源/代理/CSP 测试、显式指定可丢弃 PostgreSQL 的 Go 集成测试，以及独立浏览器上下文中的页面和编辑器回归。公网匿名探针只能证明被检查的接口与响应，不能替代完整安全审计。测试入口和数据库要求见[开发与测试](../README.md#测试)。

源码入口：[代理与 HTTPS](../internal/app/transport_security.go)、[CSP](../internal/app/security_headers.go)、[静态文件](../internal/app/assets_http.go)。
