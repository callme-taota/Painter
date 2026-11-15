---
name: Painter Next
description: A personal publication with neutral reading surfaces and a compact inset workbench.
colors:
  accent: "#18181b"
  accent-soft: "#f4f4f5"
  background: "#fff"
  paper: "#fff"
  sidebar: "#fafafa"
  ink: "#18181b"
  muted: "#66666e"
  line: "#e4e4e7"
  subtle: "#f4f4f5"
  primary-ink: "#fafafa"
  focus-ring: "#52525b"
  link: "#0066cc"
  link-hover: "#004d99"
  danger: "#b42318"
  success: "#146b4d"
  dark-accent: "#fafafa"
  dark-accent-soft: "#27272a"
  dark-background: "#09090b"
  dark-paper: "#18181b"
  dark-sidebar: "#111113"
  dark-ink: "#fafafa"
  dark-muted: "#a1a1aa"
  dark-line: "#3f3f46"
  dark-subtle: "#27272a"
  dark-primary-ink: "#18181b"
  dark-focus-ring: "#a1a1aa"
  dark-link: "#2997ff"
  dark-link-hover: "#72bbff"
  dark-danger: "#ffb4ab"
  dark-success: "#8ed9b5"
typography:
  display:
    fontFamily: 'Geist, "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", Arial, sans-serif'
    fontSize: "clamp(3.5rem, 6.6vw, 6rem)"
    fontWeight: 600
    lineHeight: 1.05
    letterSpacing: "-0.04em"
  headline:
    fontFamily: 'Geist, "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", Arial, sans-serif'
    fontSize: "clamp(2rem, 3.4vw, 3.25rem)"
    fontWeight: 600
    lineHeight: 1.3
    letterSpacing: "-0.04em"
  workbench-title:
    fontFamily: 'Geist, "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", Arial, sans-serif'
    fontSize: "24px"
    fontWeight: 600
    lineHeight: "32px"
    letterSpacing: "-0.025em"
  body:
    fontFamily: 'Geist, "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", Arial, sans-serif'
    fontSize: "16px"
    fontWeight: 400
    lineHeight: 1.6
  reading-body:
    fontFamily: 'Geist, "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", Arial, sans-serif'
    fontSize: "18px"
    fontWeight: 400
    lineHeight: 1.85
  label:
    fontFamily: 'Geist, "Noto Sans CJK SC", "PingFang SC", "Microsoft YaHei", Arial, sans-serif'
    fontSize: "13px"
    fontWeight: 500
    lineHeight: "20px"
  source:
    fontFamily: '"SFMono-Regular", Consolas, "Noto Sans CJK SC", monospace'
    fontSize: "15px"
    fontWeight: 400
    lineHeight: 1.85
rounded:
  item: "4px"
  control: "6px"
  menu: "8px"
  surface: "12px"
  navigation: "999px"
  navigation-mobile: "999px"
spacing:
  xs: "4px"
  sm: "8px"
  inset: "12px"
  md: "16px"
  content: "20px"
  lg: "24px"
  studio: "28px"
  xl: "32px"
  section: "40px"
  chapter: "48px"
components:
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.primary-ink}"
    rounded: "{rounded.control}"
    padding: "7px 12px"
    height: "36px"
  button-primary-dark:
    backgroundColor: "{colors.dark-accent}"
    textColor: "{colors.dark-primary-ink}"
    rounded: "{rounded.control}"
    padding: "7px 12px"
    height: "36px"
  button-secondary:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "7px 12px"
    height: "36px"
  button-quiet:
    backgroundColor: "transparent"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "7px 12px"
    height: "36px"
  input:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "7px 12px"
    height: "36px"
  chip:
    backgroundColor: "{colors.subtle}"
    textColor: "{colors.ink}"
    rounded: "{rounded.item}"
    height: "28px"
  panel:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.surface}"
    padding: "24px"
  navigation:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.navigation}"
    padding: "0 26px"
  work-navigation-active:
    backgroundColor: "{colors.subtle}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
  menu:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.menu}"
    padding: "4px"
  search-dialog:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.surface}"
    padding: "28px"
  toast:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.surface}"
    padding: "16px"
---

# Painter 界面规范

## 视觉方向

保留 Painter 原标识和个人出版物气质。身份、简介、封面和作品来自实际配置；界面以黑白灰、中性浅深色、细边界和克制的 Apple 蓝文字链接组织内容。工作台采用紧凑的内嵌任务区、分组侧栏和面包屑；基于现有 Vue 组件，不引入 React/Radix 运行时。

上方 frontmatter 记录颜色、字体、间距和控件角色；实现位于 `frontend/src/shadcn-theme.css` 等共享样式。产品行为见 [PRODUCT.md](PRODUCT.md)，跨端约束见 [全端交互规范](../docs/CROSS_PLATFORM_EXPERIENCE.md)。这些规则不代表自动完成视觉或真机验收。

## 色彩与材质

- 黑白主按钮用于保存、发布、搜索等主要任务。错误 / 成功色只表达真实业务状态，并配合文字。
- 蓝色用于正文链接、明确的阅读动作和当前导航。页脚、作者元信息、结构导航、卡片正文和次要去向优先使用 ink / muted，悬停或聚焦提供明确反馈。
- 公开顶栏为圆头导航，只放品牌与搜索。桌面 Dock 图标各自独立；相关选项成组。明确需要的导航、Dock和搜索使用 Liquid Glass，其余阅读和管理表面保持实色。
- 手机材质使用浏览器原生 backdrop 模糊、底色与边缘高光，不生成全页图像、滚动跟随镜像或每帧位移贴图。搜索不对整个主页面施加 CSS filter。减少透明度、高对比或不支持时回退实色；减少动态时停止非必要运动。

## 字体与密度

Geist Latin WOFF2 与系统中文无衬线回退，使用 `font-display: swap`，不运行时请求字体 CDN。来源见 [Geist-SOURCE](frontend/src/assets/Geist-SOURCE.md)，SIL OFL 授权随 `frontend/public/licenses/` 与离线资源一同交付。源码使用系统等宽字体。

阅读正文和可编辑文档采用独立的 18px / 1.85 比例；常规桌面控件保持 36px 高、主要手机触控目标至少 44px，手机输入不因小字触发放大。圆角遵守 item / control / menu / surface 层级，圆头只用于导航和需要的胶囊控件。数字、选择、光标和焦点均使用共享角色。

正文、标题、封面和工具栏共享明确的对齐边线。公开内容 wrapper 与阅读列分开，作者 / 目录在宽屏右侧，窄屏回到正常内容流。后台表格突出名称、状态和直接操作，避免大块说明占据任务空间。

## 导航与跨端任务

桌面公开入口位于 Dock，手机使用底部导航和更多面板。文章目录、日期、分享、点赞是当前页任务，不冒充全局导航。非首页顶栏随向下滚动隐藏、回滚出现。链接保持原生 URL、长按和新标签语义。

工作台按内容、数据运维、站点、个人空间分组，遵循真实角色过滤。手机任务独立呈现，编辑器采用正文 / 文章信息 / 发布任务，不把桌面双栏简单缩小。dialog 保留焦点、Escape和返回行为；地址和筛选能够恢复。

搜索为键盘可操作的命令面板，建议来自实际公开内容；完整筛选在结果页。单选、多选、排序与确认统一使用项目组件和可访问状态，文件与系统授权使用平台实际能力。

## 核心表面

- 编辑器：一个标题、一套元信息、一条保存 / 发布链路。文档为默认，源码和导出位于辅助入口。焦点在标题和工具分隔处明确呈现，不用覆盖整篇正文的大蓝框。
- 个人中心：我的 / 资料 / 公开主页 / 安全按任务组织。首次按需加载后保持实例，避免丢失未保存设置或一次性密钥；公开读者页不呈现创作者空态。
- 专栏：桌面列表与编排器清楚分区，手机进入一项后专注编排并可返回；目录编号、可见性和顺序可读，删除引用不删除正文。
- 文章：标签与摘要成组，三种时间收进信息入口，作者单独呈现；正文分隔、相关文章分区和推荐卡片有不同层级。
- 动态与评论：图文数据来自真实存储，清晰区分作者、正文、图片与互动。错误和分页反馈放在附近，失败不伪装成空列表。
- 图片：直接在暗色全屏蒙层展示完整图片，不加外框、标题栏或装饰卡片；单图仅关闭，多图保留前后切换。
- 首页：桌面连续收藏墙围绕实际格子身份和矩形对齐实现变换；中央完整网格同步放大，不随机抽点。手机和减少动态保留自然滚动，不用磁吸干扰内部滚动、交互焦点或过渡。

## 状态与验证

每个任务具备加载、空、失败、成功、禁用、冲突与离开处理；错误说明原因和下一步，保留用户输入。颜色不独自承担状态，交互保持标签、键盘与焦点语义。非必要动画不妨碍滚动与输入。

全站操作反馈共用 `frontend/src/toast.ts`：保存、发布、删除、复制等显式操作在服务器确认或平台操作成功后提示，普通操作失败同样使用 toast。界面采用实色中性表面、统一圆角、状态图标、可关闭按钮及可访问通知；桌面右下角，手机位于可视区域顶部，避让底部导航和软键盘。原生弹窗中的通知仍可读、可关闭，不夺走当前输入焦点。最多同时三条，相同消息去重，同一任务通过稳定 id 更新；悬停、聚焦和页面隐藏时暂停关闭计时。

自动保存保留编辑器紧凑状态，失败与恢复仅在状态变化时提醒；草稿保存成功不代表已经发布。版本冲突、本地恢复、字段校验、带重试的加载失败、一次性密钥与恢复码继续在对应任务中持久展示。禁止通用请求拦截器为所有响应弹成功提示，也不把原始响应、密码或凭据传入通知。

批量检查实际界面的小屏 / 大屏、深浅主题、长文本、慢网、失败、减少动态 / 透明度、键盘和放大。浏览器仿真不能代替真实 iOS / Android 软键盘、辅助技术或帧率测试。过程截图与报告放在忽略目录，不作为源代码提交。
