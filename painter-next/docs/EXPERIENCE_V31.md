# V31 · 液态玻璃导航与命令搜索

2026-11-04。按站长选定的范围实施，保留现有首页视差、内容结构、字体、正文、封面、后台与鉴权。

## 可见行为

- 顶部沿用原有条形布局，外形改为完整胶囊。左侧原 Painter 标识回首页，右侧搜索有原生 `/search` 目标及快捷键提示；文章、动态和片语移到右侧。
- 右侧外层只定位，不再绘制一整条背景。三个内容入口与既有配置工具各自拥有一颗独立玻璃镜片，可见标签在悬停或键盘焦点时出现。高度不超过 600px 时，三个内容入口收为一个带文字链接的浏览菜单。触点至少 44px，浮层向内打开并避让视口边界。
- 不增设 index 页面。文章 `/posts`、动态 `/moments`、片语 `/phrases` 均为现有真实路由；友链继续按设置留在页脚。未添加不存在的唱片、图片或项目独立页面。
- 浮控 `home/article/other` 设置、工具顺序、点赞/文章底部场景和创作者权限继续生效。空工具数组只清空工具，不移除新的公共浏览导航。创作仍须配置启用且服务端 `Creator` 授权，未擅自开启。
- 文字链接与导航选中状态增加单一蓝色：浅色正文 `#0066cc`，深色正文 `#2997ff`；玻璃上的选中图标使用更稳妥的深蓝/浅蓝。主按钮、内容卡片、标题和其余中性界面不整体换色。
- 搜索增强为 Raycast 风格命令面板。保留真实文章/标签建议，按类型分组；空输入附带文章、动态和片语入口，非空查询只展示匹配的真实建议。筛选仍只在完整搜索页。

## 搜索交互

复用 `/api/v1/search/suggestions`，180ms 防抖、AbortController 和响应序号阻止旧请求覆盖新查询。输入法合成时不抢提交或 Escape；合成结束继续正常查询。Ctrl/Command-K、非编辑区的 `/`、上下键、Home/End、Enter、Escape、点击与原生链接的新标签语义均保留。

关闭取消请求并恢复实际打开按钮焦点。正常成功的重复状态仅供读屏读取；加载、空结果和失败信息仍可见。面板不加入标签筛选器，不改变搜索接口的公开性边界。

## 玻璃实现与限制

参考 [Apple HIG 材质](https://developer.apple.com/design/human-interface-guidelines/materials)、[rdev 原始实现](https://github.com/rdev/liquid-glass-react)、[rizzytoday 原生 TypeScript 实现](https://github.com/rizzytoday/liquid-glass) 与 [MDN backdrop-filter](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/backdrop-filter)。搜索参照 [Raycast 列表界面](https://developers.raycast.com/api-reference/user-interface/list) 和 [官方键盘快捷键](https://manual.raycast.com/keyboard-shortcuts)。

这是导航/命令功能层，不覆盖文章内容。Painter 的原生模块自行生成圆角距离场边缘法线图，通过 SVG 三通道微差位移折射真实背景，叠加克制的边缘高光。文字和图标不参与滤镜。Dock 中心保留可读性遮色，边缘逐渐更透；头部文字使用较厚材质。

不引入 React 或新的第三方运行依赖；不截图页面，不克隆 DOM，不附加可见 canvas，不读取私密页面像素。仅尺寸与辅助设置变化时更新小图，单图最多 18 万像素，缓存上限 16，无空闲或滚动绘制循环。

实际 Chromium 使用经像素差验证的 SVG 折射。Safari、Firefox 和 iOS 浏览器采用 CSS 磨砂/高光降级；本轮未运行实际 WebKit，UA 路由检查不冒充 Safari 渲染验证。减少透明度、提高对比度、强制色彩及不支持背景滤镜时为实色。深浅色和设置变更实时响应。

新 Dock 出现时跳过 V25 整轨材质初始化，避免两层玻璃重叠；V28 的共享元素、点阵、唱片和游戏时序不改。首页保留原有绝对定位头部例外，其他页面继续向下隐藏、向上出现，并保留焦点/弹窗保护。

## 实现与验证

核心文件：`web/templates/page.html`、`web/frontend/src/public.ts`、`public-chrome.css`、`dock.ts`、`liquidGlass.ts/.css`、`search.ts` 与 `raycast-search.css`。局部视觉约定记录于 `web/.impeccable/surfaces/navigation-v31.md`，DESIGN.md 只更新对应导航、链接与材质条款。

- 前端类型检查与 Vite 构建通过；已有 Markdown 编辑器大 chunk 提示仍在，本轮未扩大编辑器范围。
- `go vet ./internal/app ./web ./cmd/painter` 通过。
- 无数据库的 5 项 V31 模板合同与既有浮控校验通过：真实 SSR 入口、空工具、Creator 与配置双门控、文章场景和页脚友链。
- 独立搜索检查 66 项与成功/加载/空/错误状态检查 4 项通过。
- 最终候选限定功能检查 120 项通过：导航、滚动头部、320/390/横屏触控、44px 触点、短屏键盘菜单、主题和辅助降级、无 JavaScript 导航、现有管理员 SSR。所有非 GET/HEAD 请求在测试页面本地拦截。
- 桌面/手机、浅深色的真实首页、搜索、阅读及媒体画面已检查。首页相关样式/脚本与 V27 查看器对 HEAD 的源哈希一致；没有把旧整轨材质的外观断言继续套到新设计上。
- 设计 detector 只运行一次；两条结果来自原始 Go 模板无法解析动态资产而误判的默认黑色和 16px 标题，真实页面已按实际构建检查，未为消除误报改变正文。

本轮无 schema、迁移、业务内容、媒体、账号、站点配置或 provider 快照变更。数据库集成测试未重新运行：既有维护库损坏不在本次范围，未修复或替换它，也未使用 SQLite。候选与正式使用各自既有环境；发布仅提升经验证的二进制，不复制候选数据库或媒体。

局部证据保存在忽略的 `tests/artifacts/v31/`：构建日志、模板/功能/搜索报告、一次 detector 原始结果、真实页面截图、独立与实景折射对照及发布记录。发布完成状态以该目录 `promotion.json` 和实际正式服务检查为准，本说明不把候选数据当成站长内容。
