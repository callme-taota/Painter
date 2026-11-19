# Painter 全端设计与色彩契约

更新：2026-11-19

适用：Go SSR 公开页面、Vue 工作台与两种编辑视图、手机 / 平板 Web、PWA、独立分享 / 内嵌页、账号授权页面。当前没有独立原生客户端；未来客户端沿用本契约的 key 和语义，不复制一套自选调色板。

## 统一规则与实现入口

| 规范 | 唯一维护位置 | 使用方式 |
| --- | --- | --- |
| 颜色、主题、数据色阶与作品预设 | [`web/colors.json`](../web/colors.json) | 所有端按注册的语义 key 取色 |
| CSS / TypeScript / Go 颜色输出 | `color-tokens.css`、`color-tokens.ts`、`colors_generated.go` | 自动生成，禁止手改 |
| 字体、密度、间距、圆角、组件角色 | [`web/DESIGN.md`](../web/DESIGN.md)、`style.css`、`shadcn-theme.css` | 使用共享布局变量，颜色引用上述配置 |
| 手机、平板、桌面交互 | [全端体验与交互规范](CROSS_PLATFORM_EXPERIENCE.md) | 同一资源 URL、权限与内容，各端可有不同操作路径 |
| 新增颜色的检查 | [`color-policy.mjs`](../web/frontend/build/color-policy.mjs) | 本地构建与 CI 强制执行 |

公开页的视觉原则保持黑白灰、苹果蓝阅读链接、受控的 Liquid Glass。工作台、表单和正文承载区使用清楚的实色表面。手机仍按单手操作组织界面，不把桌面侧栏、悬停与多列布局直接缩小。

## 颜色只能按角色取用

`web/colors.json` 是唯一允许定义固定色值的配置。每个条目必须有 `light`、`dark`、中文用途说明和类别。key 描述用途，不描述某个页面或某个十六进制值。

| 角色 | key | 使用约束 |
| --- | --- | --- |
| 页面与表面 | `bg`、`paper`、`canvas`、`sidebar`、`subtle` | 页面底色、内容层、独立页外层、侧栏、嵌入层分别取用 |
| 文字 | `ink`、`muted`、`muted-strong` | 主文字、次文字、透明表面上的较强次文字 |
| 操作 | `accent`、`accent-soft`、`primary-ink` | 黑白主操作、选中底色、按钮文字；不可把 accent 当成所有链接的颜色 |
| 导航与链接 | `link`、`link-hover`、`link-soft`、`link-ink` | 阅读动作 / 链接、悬停、轻量背景、蓝色实心按钮文字 |
| 边界与焦点 | `line`、`line-strong`、`focus-ring` | 装饰性分隔、需辨识的控件边界、键盘焦点；焦点不能只靠轻微颜色变化 |
| 状态 | `success`、`warning`、`danger` | 配合图标 / 文案表达成功、警告、错误；禁止仅以红绿区分状态 |
| 首页名片 | `hero-sheet`、`hero-sheet-ink`、`hero-sheet-muted` | 保持名片与阅读表面的反差，不借用普通正文色模拟 |
| 玻璃 | `glass-tint`、`glass-header`、`glass-mobile`、`glass-dense`、`glass-fallback` | 导航、顶部、手机、密集背景、降级材质；手机与桌面共享材质语义 |
| 搜索玻璃 | `search-surface`、`search-border`、`search-fill` | 与导航同一体系，避免搜索弹窗拥有另一套调色板 |
| 媒体 | `media-ink`、`media-muted`、`media-canvas`、`media-line` | 图片 / 游戏封面上的文字和遮罩，不随浅深主题反转 |
| 叠层 | `shadow-color`、`glass-highlight`、`scrim` | 阴影基色、高光基色、默认模态遮罩 |
| 代码 | `syntax-keyword`、`syntax-string`、`syntax-comment`、`syntax-name`、`syntax-number` | 服务端代码块与源码编辑器共享，不分别指定一套高亮色 |
| 数据 | `activity-0` … `activity-4` | 日历与首页变形中的发布频率使用同一色阶，数值和日期仍可阅读 |
| 安全码 | `qr-paper`、`qr-ink` | 固定明暗二维码，不跟随页面深色反转 |
| 作品 | `cover-*`、`cover-title`、`author-*-start/end` | 文章封面、作者背景预设；两种主题保持作品原色 |

`share-*`、`game-ink/game-canvas`、`content-panel-link` 是配置中明确列出的语义别名，不是新色板。新代码优先直接使用目标角色。

链接无需全站染蓝。正文超链接、明确阅读动作、选中导航可以用 `link`；作者元信息、页脚、结构性卡片标题优先用 `ink` / `muted`，悬停及焦点给予可辨识反馈。

## 各端怎样使用

CSS / Vue 样式只引用 key。透明度和阴影形状可以由注册颜色派生，禁止为了微小透明度创建一个新的颜色名。

```css
.example-panel {
  color: var(--ink);
  background: var(--paper);
  border: 1px solid var(--line);
  box-shadow: 0 4px 14px color-mix(in srgb, var(--shadow-color) 9%, transparent);
}
.example-panel a { color: var(--link); }
.example-panel a:hover { color: var(--link-hover); }
```

需要颜色字符串的画布、二维码、浏览器主题栏等使用生成的 TypeScript API：

```ts
import {colorValue, colorVar, type ColorKey} from './color-tokens'
const role: ColorKey = 'qr-ink'
const fixedQRInk = colorValue(role)
const cssForeground = colorVar('ink')
```

`colorValue` 返回配置表达式，不负责解析 CSS。二维码、manifest 等只使用具体颜色值的角色；需要解析 `color-mix` 的画布应先在浏览器中解析已注册角色。不可把未经验证的用户文本作为颜色传入。

Go 的 manifest、生成封面等使用同一配置生成的强类型 key：

```go
web.Color(web.ColorBg, web.Light)
web.Color(web.ColorCoverInkDeep, web.Light)
```

授权页面直接嵌入生成的颜色 CSS，独立分享页也显式导入同一 CSS，不依赖先访问首页。PWA manifest 的启动色使用 `bg` 的浅色值；已打开页面的浏览器主题栏跟随实际主题更新。PWA 安装壳受浏览器能力约束，不代表已有原生客户端。

## 主题、无障碍与材质

- 浅色、深色共用 key，组件不自己判断主题来指定文字色。配置生成 `data-theme` 主题及无 JavaScript 时的系统主题回退；明确选择优先于系统偏好。
- 跟随系统由统一 `theme.ts` 处理；工作台选择器和公共菜单都通过 `setTheme` 更新，浏览器主题栏同步，跨标签页的存储变化也同步。
- 高对比度及打印色集中在配置的 `modes` 中，生成后拥有足够优先级，不能被后加载的工作台主题覆盖。打印保持浅底深字；高对比度提高边界和次文字辨识度。
- 系统强制颜色模式保留 `Canvas`、`CanvasText`、`LinkText` 等系统色，不为追求作品效果禁用系统颜色。
- 玻璃只用于导航和明确的临时命令层。正文、长表单及管理表格保持稳定表面。减少透明 / 高对比模式降为实色；保留原有减少动画逻辑，不新增逐帧模糊或折射计算。
- 一般文字需有足够对比度；透明背景应在实际内容上复核，不能仅凭 token 对比度宣称整页合规。状态同时用文字，选中同时用标记，焦点有轮廓。

本轮统一颜色来源，不宣称对全站所有页面完成 WCAG 认证。尺寸、命中区、键盘逻辑继续遵循全端交互规范：手机或粗指针主要操作至少 44×44 CSS px；桌面允许紧凑控件；内容断点可按任务单独调整。

## 内容颜色与组件派生变量的边界

照片、专辑封面、游戏图、第三方官方 logo 是内容资产，不强行改成站点调色板。封面和作者背景的固定预设则在配置 `artwork` 类别注册，保持原作品配色，不因主题变化改写历史封面。

动态图片底部的色彩扩散是唯一图片采样入口：`momentCover.ts` 将受控小画布的 RGB 通道写入 `--moment-color-left/right`；`momentsPublic.css` 只用这两个槽位合成。它不是让组件随意定义颜色的通道，不接收任意 CSS 字符串。

`--liquid-glass-tint`、搜索内部槽位、`--home-surface`、`--work-day-color` 和阴影复合值属于受检查的组件派生变量：只允许在检查器逐项列出的文件中，从已注册 key、合法数字和 `color-mix` 派生。新组件不能自行发明颜色变量，也不能以改名绕开配置。

`transparent`、`currentColor`、继承及系统强制色是合法 CSS 基础值。第三方依赖内部默认样式、官方图像资产和生成的生产 bundle 不在源码字面量检查范围；这些例外不适用于我们自己的 Vue / CSS / TS / HTML。

## 修改流程与强制检查

```sh
cd web/frontend
npm run colors:generate
npm run check:colors
npm run test:colors
cd ../..
make frontend
```

改颜色先检查是否已有合适角色。确需增加 key 时，只修改 `web/colors.json`，说明用途、浅深色和类别，然后重新生成、构建并检查桌面 / 手机的浅深主题。新增特殊图片取色或派生槽位必须同时增加精确的检查规则和正反测试，不能整目录豁免。

`check:colors` 先验证生成物与配置一致，再解析源码：拒绝裸十六进制、RGB/HSL 等独立色值、CSS 命名色、未注册颜色变量、组件内重写全局颜色、动态拼接任意颜色；允许注册颜色的透明度与渐变组合。CSS / Vue 样式、Vue 绑定样式、TS 样式赋值、模板样式和当前 Go 界面生成入口均受检查。Go 当前界面入口为显式清单，新增生成界面的 Go 文件时也必须扩展此清单。

`npm run build` 自动执行检查，CI 同时运行正反例测试。提交前必须包含 JSON、生成的 CSS / TS / Go、前端离线资源及 manifest；禁止只改生成物。后续新增 Swift / Android / 桌面客户端时由同一 JSON 生成对应平台映射，再接入各平台的编译与无障碍验证，不能仅复制本页的表格视为完成。
