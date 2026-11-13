# 手机端 Liquid Glass：实现与验证

2026-11-13 · V47

## 原来的问题

Painter 原先只在 Chromium 上启用 SVG `backdrop-filter: url(...)`。iPhone 上的 Safari、Chrome 等 WebKit 浏览器只得到普通 blur；同时手机导航底色接近不透明，玻璃边缘和背景层次都被盖住。搜索材质的通用选择器还会覆盖底部导航自己的透明度配置。

本次保留手机页面组织：顶部 logo / 搜索、底部四个导航、全屏搜索、更多底部面板、原来的返回栈和触摸目标。只改公共导航材质，阅读正文、账号表单、工作台保持原有实底。

## 开源调研与选型

以下是阅读源码及本地原型后的判断，不能把仓库宣传中的 “Safari supported” 当作任意网页实时折射的保证。

| 实现 | 与本项目相关的方式 | 本次选择 |
| --- | --- | --- |
| [gentpan/liquidglass](https://github.com/gentpan/liquidglass/tree/d5db099ef00828496aa2733e4f35542c05a0b1f8) · MIT | 在玻璃后镜像指定图片 / 背景，再对这些像素使用普通 SVG CSS filter；不依赖 Safari 不支持的 SVG backdrop filter | 采用其图片镜像思路，结合 Painter 原有法线贴图实现，无运行时依赖 |
| [simple-liquid-glass](https://github.com/lucaperullo/simple-liquid-glass/tree/ea84dc99124d87295553040e1b55eaa9fceab2c7) 6.0.0 · MIT | DOM 栅格化后经 WebGL 输出折射，独立 `/webgl` 入口可不带 React | 已做 Chromium / Linux WebKit 原型，最终未引入；整页捕获、外部封面读取及约 42 KB gzip 额外引擎不适合这次改造 |
| [rdev/liquid-glass-react](https://github.com/rdev/liquid-glass-react/tree/ac48eab) · MIT | SVG 材质、边缘高光、交互形变；SVG backdrop 位移受浏览器能力限制 | 参考双层边缘高光的材质组织，不为此引入 React |
| [kwlck/liquid-glass-web](https://github.com/kwlck/liquid-glass-web/tree/a778667) · MIT | 无框架法线图、边缘光与 SVG backdrop filter | 参考薄底色与边缘对比，不能单独解决 WebKit 的背景位移 |
| [naughtyduk/liquidGL](https://github.com/naughtyduk/liquidGL/tree/88f681ab7035fd55b04f63edff1841e32c4199e9) | 自带页面渲染和 GPU 管线，脚本 / 包目录 MIT | 本次不引入整套渲染引擎或演示素材 |

没有复制这些项目的演示图片、视频、字体或 agent skill；新增图片镜像代码为本项目实现，原有 SVG 法线与滤镜仍由 Painter 自己生成。

## 实际渲染分层

由后向前：

1. 浏览器原生背景模糊，负责文字与非图片区域。
2. 与原图位置、尺寸、`object-fit`、`object-position` 对齐的装饰性图片副本；普通 `filter: url(#...)` 根据圆角镜片法线折射边缘。已存在的游戏渐变暗罩可以一起复现。
3. 独立浅 / 深色 tint；底部导航和顶部栏分别使用 54% 白色、62% 深色底色，保证文字在极端底图上仍有对比。
4. 两层明暗边缘与柔和反光，表达厚度。
5. 原来的链接、图标、输入框和按钮；它们不被变形或复制。

手机原生 blur 由控件表面自己承担，避免依赖负 z-index 子层采样。搜索的 `dialog::backdrop` 另有 12px 原生模糊；部分 WebKit 合成器只接受该属性却不实际模糊，因此搜索打开时也对原来的页面背景兄弟层应用普通 CSS blur。输入和结果不滤镜，关闭自动复原；减少透明度时关闭这些增强。

图片滤镜使用独立的 `objectBoundingBox` 图。法线 R / G 轴按镜片长宽归一化，位移量按短边换算；不把桌面原有 user-space backdrop 图改成另一套参数。先解码本地法线 PNG，再发布新的 filter ID，并做两次有界的缓存刷新。这些细节经过滤镜开启 / 关闭截图对比确认；只有 `CSS.supports`、存在图片副本或状态 active 都不足以证明 Safari 真正发生位移。

图片副本输出裁切到原照片真实交集，祖先滚动 / 裁切按 X / Y 轴分别处理。每个镜片最多六张图片。新增层 `aria-hidden`、不接收指针事件，没有新的可聚焦控件，也不改变原表单、搜索请求或历史记录。

全屏搜索保留更强的底色与图片模糊，避免封面影响结果阅读。底部当前项增加薄亮面和按压反馈；减少动态效果时关闭过渡。

## 性能、安全与降级

- 没有新增 npm 依赖，没有整页 DOM 截图、Canvas 像素读取、WebGL 上下文或后台截图服务。
- 不通过 fetch / XHR 抓取封面，不放宽现有 CSP `connect-src 'self'`。镜像的 `img` 仍可能产生正常图片请求，浏览器可复用缓存；不能宣称完全零额外网络请求。
- 仅在 ≤720px 的公共顶部栏、底部导航与搜索启用。登录、账户、设置、预览、后台及含密码、上传、OTP、评论草稿 / 编辑器的主区域不做图片增强。
- 滚动、尺寸、主题、图片加载或相关 DOM 变化时合并为一帧更新；静止时不持续运行 RAF。上滚唤出的顶部栏最多跟随 300ms 过渡，避免它在滚动帧之后才移动而漏掉图片对齐。先批量读取布局，再写装饰层。离开手机尺寸、减少透明度或销毁时释放副本和监听。
- 手机无图区域继续使用原生 blur，桌面保留原有 Chromium SVG backdrop 路径。图片未加载、被旋转或 3D 变换、浏览器缺少所需裁切能力时保留可操作的回退材质。
- 减少透明度、高对比和强制颜色模式使用实底；不为视觉效果改变认证或表单安全。

## 明确边界

这是网页材质增强，不等于 iOS 原生 Liquid Glass。WebKit 的跨浏览器真实位移针对已有图片及有限的渐变暗罩；图片上方的独立文字、复杂遮挡、CSS 背景、视频及任意 DOM 不会被完整重建和折射。纯白背景本来没有可见纹理可供折射，不能通过添加虚构背景来证明效果。

Linux WPE WebKit 在非图片区域还可能不实际绘制原生 backdrop blur，即使 computed style 显示支持；这些区域仍有 tint 与边缘材质，但不宣称已完整模糊。搜索另有已实际验证的普通 CSS blur 回退。图片折射与该平台限制分别验证，不能互相替代。

Linux WebKit 与 Chromium 的手机视口验证，不代表物理 iPhone、iOS Safari 和真实电池 / GPU 性能验收。Mac 节点已检查，Safari Remote Automation 未开启；本轮没有更改用户的该项设置。

## 维护入口

- [liquidGlass.ts](../web/frontend/src/liquidGlass.ts)：现有法线、SVG 滤镜、材质状态与生命周期
- [liquidGlassMedia.ts](../web/frontend/src/liquidGlassMedia.ts)：仅图片的可见交集与镜像同步
- [liquidGlass.css](../web/frontend/src/liquidGlass.css)：光学分层与降级
- [mobileChrome.css](../web/frontend/src/mobileChrome.css)、[searchGlass.css](../web/frontend/src/searchGlass.css)：手机导航与搜索材质参数
- [浏览器回归](../tests/browser/v47-glass-optics.cjs)：限定隔离候选，含 Chromium / Linux WebKit、跨域图片、滤镜截图 A/B、静止写入计数及交互检查

具体检查数量、截图和上线结果见对应发布记录；研究用 WebGL 实验产物不作为最终实现的证据。

本轮最终隔离候选完成 **143 项公共手机 / 桌面回归 + 9 项 WebKit 顶栏检查**，34 张截图，无 CSP / JS / 服务端错误。四种手机引擎 / 主题组合的图片滤镜截图 A/B 均产生显著像素差；跨域测试封面由浏览器路由返回仓库 CC0 图片，验证正常 image 请求路径，不代表外部服务永远可用。真实提供商连通性应随发布另查。静止 1.2 秒无光学样式写入；导航文字、搜索结果和历史返回保持可用。发布入口 gzip 增量约 5.1 KB，没有新增 npm 运行时依赖。
