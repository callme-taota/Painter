# Chrome 独立应用

Painter 可作为联网 Web 应用安装。公开页面、登录、GitHub 授权、2FA 和工作台继续使用现有服务器与权限；安装不会创建新的账户或授予额外业务权限。

## 使用

1. 用桌面 Chrome 打开配置的公共 HTTPS 站点。
2. 从右侧「更多操作」选择「安装应用」。该入口仅在浏览器支持安装时出现。
3. 若未出现，使用 Chrome 菜单「投放、保存和分享 → 将网页安装为应用」，或点击「安装说明」。浏览器最终决定是否允许安装。
4. 完成后从系统应用列表或 Dock／任务栏打开。独立窗口内，存在可证明同源的上一页时显示返回按钮。

普通局域网 HTTP IP 与公共 HTTPS 域名是不同来源，不共用安装身份、Cookie 或草稿存储。正式安装使用公共 HTTPS 域名；开发时可以使用浏览器信任的 localhost。

## 范围

- `/manifest.webmanifest` 使用稳定 `id`、`scope`、`start_url` 与 `display: standalone`，名称跟随站点配置。
- `/pwa/icon-192.png` 与 `/pwa/icon-512.png` 沿用原 Painter 标识；清单和图标需要重新验证，不走 Vite 一年不可变缓存。
- 安装入口按能力检测：支持的 Web Install API，或浏览器发出的 `beforeinstallprompt`。不依赖用户代理字符串，不静默安装，不把用户同意误报为安装完成。
- 首版不注册 Service Worker，不新增 Cache Storage、离线文章缓存或后台请求重放。服务器 HTML、会话和 API 的 `private, no-store` 边界保持不变。
- 编辑器原有按来源及用户隔离的 IndexedDB 草稿恢复继续生效；这不承诺断网后可重新打开完整工作台。
- 关闭应用、重新打开和普通网页访问使用同一来源。OAuth 可能暂时打开浏览器界面；系统链接是否自动进入应用依赖 Chrome／操作系统，需分别验证。
- 不强制重新加载正在编辑的窗口。独立窗口并不消除跨部署旧按需资源失效的问题，版本静态资源保留策略应独立迭代。

## 验证

Go 测试检查清单、图标与缓存头；浏览器测试检查安装能力、取消／失败／完成反馈和安全返回。安装资格可通过 Chrome DevTools 的 Manifest／CDP 检查。模拟安装事件不等于已完成 Windows／macOS／Linux 实际系统安装，真机安装由用户主动确认。

参考：[Chrome 安装说明](https://support.google.com/chrome/answer/9658361?hl=zh-Hans)、[安装条件](https://web.dev/articles/install-criteria)。
