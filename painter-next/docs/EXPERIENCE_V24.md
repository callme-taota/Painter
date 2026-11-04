# V24：游戏收藏、发布日历与文章内容卡片

V24 延续 Painter 的中性界面、Geist 字体与既有首页故事：完整游戏墙优先展示更大的竖版封面，首页新增可配置的发布日历和 GitHub 项目牌组，文章增加行内 Emoji 网页参考、GitHub / Steam / Apple Music 卡片及共享图片查看器。没有更换全局视觉身份，也没有引入可执行内容组件运行时。

**当前状态：V24已发布到正式局域网入口，schema20；真实发布日历和三个初选公开仓库已启用。正式59项只读检查及独立10项空稿回归通过，原有112款游戏 /291张专辑保留。候选24张当前要求图获得独立 SHIP；随后发现的空白新稿回归已用独立程序修正和数据不变核验关闭。** [视觉裁决](../tests/artifacts/v24/finish-review.md)仅覆盖已检查的候选画面；下列候选数据与正式数据分别记录。

## 更大的完整游戏墙

`/games` 使用真实 Steam library portrait 素材，封面比例为 2:3；没有素材时保留已保存封面与标题回退。常规桌面三列，1600px 起四列，1050px 及以下两列，720px 及以下一列。桌面标题保留相同的两行高度，时长落在一致基线；手机按内容自然增长。图像和文字共同链接到真实 Steam 目标，悬停 / 聚焦露出入口，手机始终显示。

已有展示顺序、时长、名称排序和签名游标分页不变；未知时长仍显示未知，已确认零分钟仍是零。首页继续最多五款沉浸场景，完整库不受该上限影响。关闭 Steam 区块时公开游戏库关闭；首页数量为零仍保留已启用的完整库。排序、分页与原始链接无需 JavaScript。

## 首页发布日历

owner 在 `/studio/home` →「页面内容」选择发布日历，设置可见性、顺序、标题、说明、统计来源、时间范围与时区。「公开文章与瞬间 / 仅公开文章 / 仅公开瞬间」统计当前仍公开的内容，每项按首次发布时间计一次；编辑、再次发布、GitHub 提交和片语不会增加记录，撤回或删除后不再计入。

支持最近365天或自然年；年份0跟随当前年，明确年份范围2000–2100。日期按保存的 IANA 时区划分，默认 `Asia/Shanghai`。自然年可含366天，未来日期和补齐格子有独立状态。日历显示确切日期范围、来源、时区、发布总数与活跃天数；四档绿色仅表达每天计数，最高档覆盖4次及以上，不替代具体数值。

格子可以悬停、点击或键盘聚焦查看当天次数。增强运行时初始定位到所选区间最后一个非未来日期，窄屏立即可见近期真实记录；「左右滑动查看更多日期」持续说明局部横向浏览。方向键按日 / 周移动，Home / End 跳到首尾。后台真实预览使用同样的日期呈现和近期定位，不为预览保存设置。无 JavaScript 保留 SSR 日历、原生滚动和日期说明。

## GitHub 项目牌组

`/studio/home` 新增「项目」任务页签，最多保存100个手动项目或公开 GitHub 仓库；首页数量0–24，并须同时开启区块和项目本身的可见性。新项目默认隐藏。排序依据保存的 `(position,id)`，没有配置时不伪造项目或统计。

公开牌组以中性纸面承载仓库名、实际简介、语言、头像和已知 stars。悬停、键盘聚焦或触摸选中抬起当前牌，相邻牌向两侧移动；窄屏只在牌组内部原生横向浏览。系统减少动态效果时移除旋转、抬升和过渡，保持可读链接。

保存 GitHub 地址与「刷新仓库信息」是两个动作。刷新只读取无凭据公开 GitHub API，校验请求和响应仓库身份；不使用站点 OAuth secret，也不读取私有仓库。标题、简介、链接、语言可保持自动元数据或保存人工覆盖；刷新保留覆盖、隐藏与排序。缺失统计保持缺失，明确的0保留为零。`repo_updated_at` 是仓库时间，`fetched_at` 是上次成功读取时间，均不等于本地编辑时间。

失败保留上次成功快照并保存通用错误与新版本；此状态以 HTTP200 和 `last_error` 返回，界面明确提示，后续编辑使用新版本。单项目刷新间隔30秒，owner 持久限额每分钟10次；供应商另有自己的限流。公开 SSR 只读已保存数据，不因访客打开页面请求 GitHub。

## 文章中的参考与图片

文档工具栏「卡片」或斜杠菜单可插入四个已实现类型：`github`、`steam`、`apple_music`、`link`。网页参考既可作为区块，也可作为带文字 / Emoji 的普通行内链接；作者可读取公开元数据，或手动填写网页标题与说明。预览和插入均经过服务器规范渲染，保存与发布继续使用原有草稿 / 不可变修订 / 发布指针。

行内引用悬停或聚焦显示来源说明；触摸先查看说明，再次点按打开原目标，Escape 和外部操作可关闭。原生 Enter、新标签、修饰键行为保留。GitHub、Steam 和 Apple Music 区块显示保存快照与真实目标，不嵌入播放器。手机 Steam 卡片将图像放在全宽文字上方；所有形式沿用现有文章阅读宽度。

文章普通上传图片接入动态 / 首页照片已经使用的同一 dialog，支持前后切换、Home / End、原图、加载失败提示、Escape 与关闭后焦点返回。供应商卡片图片仍打开供应商目标；作者已为图片设置的其他导航链接保留。图片查看器不是第五种 typed image card。

卡片使用 `painter-card` fenced JSON 和普通链接 title 中的版本化快照，前后端仅接受编译期白名单。已知无效字段报错，未知类型 / 版本保留普通代码或链接。正文经过同一 Goldmark AST、转义和 Bluemonday 管线；媒体引用继续受 ready / 所有权 / draft / revision 保护。新 renderer version 不批量重渲染旧文章，历史发布 HTML 保持原样。语法、网络约束、作者操作和扩展合同见[文章内容卡片](CONTENT_CARDS_V24.md)。

后续组件池当前仅是建议：callout / aside、附件下载、code tabs、书目。它们尚无已交付语法或界面；新增类型须先明确版本、schema、SSR、编辑器、纯文本、媒体和导出合同，不能放宽现有 v1 校验来接入任意 HTML。

## 接口与追加迁移

| 动作 | 接口（`/api/v1` 前缀） |
| --- | --- |
| 读取真实日历预览 | `GET /admin/home/activity?source=…&timezone=…&period=…&year=…` |
| 读取 / 创建项目 | `GET /admin/home/projects` / `POST /admin/home/projects` |
| 更新 / 删除项目 | `PUT /admin/home/projects/{id}` / `DELETE /admin/home/projects/{id}` |
| 显式刷新公开仓库 | `POST /admin/home/projects/{id}/refresh` |
| 作者解析内容参考 | `POST /editor/cards/resolve` |

首页配置与项目操作继续 owner-only，卡片解析限 author / owner；写接口保留同源、CSRF、幂等、CAS、权限复核和审计。项目更新、删除、刷新使用当前 `If-Match`。普通网页读取仅接收公开 HTTPS HTML，不执行脚本或任意导入 OG 图片；固定提供方主机、DNS/IP 校验、重定向预算和有界响应详见卡片文档及[后端复核](../tests/artifacts/v24/backend-review.md)。

[`0020_home_activity_projects.sql`](../migrations/0020_home_activity_projects.sql)追加首次发布索引、`homepage_projects` 和 `homepage_project_rates`，SHA-256 为 `c13512a072b3e13e665731dbfc87db758dd786abff2817126af603a27bd347dc`。旧六区块配置在读取时规范化为八区块，新日历 / 项目默认关闭；迁移不重写旧配置行、文章或历史发布 HTML。卡片数据直接保存在 Markdown，不新增文章卡片表。

## 已完成的候选验证

| 范围 | 实际结果与边界 |
| --- | --- |
| Go 与类型 | [`go test ./...`](../tests/artifacts/v24/go-test.log)、[`go vet ./...`](../tests/artifacts/v24/vet.log)、[2项 SSR race](../tests/artifacts/v24/ssr-race.log)、[前端类型 / Vite构建](../tests/artifacts/v24/build.log)与[Go构建](../tests/artifacts/v24/go-build.log)通过；[后端复核](../tests/artifacts/v24/backend-review.md)另记录内容卡片、日历 / 项目 race、原首页回归和类型检查。数据库测试使用独立临时 PostgreSQL 库 |
| Markdown 编解码 | [`v24-cards-unit.mjs`](../tests/browser/v24-cards-unit.mjs)最终7项；初次6项覆盖完整 JSON、Emoji / 字面文本、未知格式、源码定位和安全 DOM，新增空文档默认段落回归 |
| 共享运行时 | [34项](../tests/artifacts/v24/card-runtime/report.json)，独立拦截页面，无应用服务 / 业务写入；桌面、触摸、原生目标、动态初始化、单一灯箱、失败和溢出 |
| 作者编辑器 | [最终34项](../tests/artifacts/v24/editor-cards/report.json)，1440浅 /390深；初次26项基础上增加8项空稿检查。供应商与文章写入隔离模拟，规范预览用真实候选后端，实际业务写入0 |
| 真实空稿回归 | [候选10项](../tests/artifacts/v24/empty-editor/candidate.json)和[正式10项](../tests/artifacts/v24/empty-editor/formal.json)，正常会话打开新稿，确认默认文档、可编辑正文与卡片工具；内容 / 配置写入0 |
| 首页后台 | [86项](../tests/artifacts/v24/home-admin/results.json)，正常 owner / author / reader；真实候选配置 / 项目写入、CAS、幂等重放、权限、真实 GitHub 刷新、320/390浅深布局；运行错误 / 失败请求0 |
| 公共功能与截图 | [104项功能](../tests/artifacts/v24/visual/functional-report.json)和同批30项截图断言，最终[134项报告](../tests/artifacts/v24/visual/report.json)通过；公共业务写入0。一次预期封面失败用于验证回退，不是未处理页面错误 |
| 针对性修正 | 手机 Steam 布局[14项](../tests/artifacts/v24/visual/steam-fix-report.json)与2张替换图；日历近期定位公共[22项](../tests/artifacts/v24/visual/calendar-fix-public.json)、后台[38项](../tests/artifacts/v24/home-admin/calendar-fix-admin.json)，共6张日历替换图，未重开完整矩阵 |
| 视觉裁决 | [独立收尾评审](../tests/artifacts/v24/finish-review.md)打开全部24张当前要求图，确认唯一剩余P2日历发现性问题解决，最终 SHIP；更早 Steam 和日历 before 图保留为历史证据 |
| 设计记录 | [文档复核](../tests/artifacts/v24/design-documentation-review.md)对照 source-before，记录新增局部模式、既有 token 和未修复的上下文漂移 |

当前要求图为14张公共图、2张编辑器图、8张后台图；另有2张隔离运行时截图，不计入24张视觉要求。截图并非所有宽度 × 主题 × 状态的笛卡尔积。触摸使用 Chromium 模拟，未覆盖实机或 Safari；外部目标在观察真实 href 后本地应答，不宣称验证第三方应用。无 JavaScript / 减少动态和键盘行为由功能报告支持，截图评审没有重新执行这些测试。

前端构建保留既有大型编辑器 chunk 提示（当前约1.18 MB、gzip约390 kB）。类型与构建通过不等于完成全站性能认证；本轮没有借卡片扩展重构整个编辑器分包。

候选实际内容为6款公开游戏、3个真实公开 `callme-taota` 仓库和专用文章33。后台初次截图时有12次首次发布；文章33随后发布，公共图记录13次 /2天（7篇文章、6条瞬间）。这是候选时间顺序，不能将其替换为正式站点的收藏或发布历史。Apple Music 文章卡片保留实际返回的100×90源图，放大后偏软；没有虚构高清 URL。

## 正式备份、发布与配置边界

[备份核验](../tests/artifacts/v24/backup-validation.json)记录 schema19 的56表完整 dump 解码、8个媒体文件逐项核对、运行程序 / 环境文件一致和私有权限；未将归档解码说成实际数据库恢复。[发布准备](../tests/artifacts/v24/promotion-preparation.md)中的预检与临时夹具检查不等于已执行迁移。

2026-11-03 的[正式发布报告](../tests/artifacts/v24/promotion.json)记录程序替换与 schema19→20 已完成，迁移账本 count / max 均为20，新增后共58表。停止边界前后，56张旧表的全部旧列定义、旧行数和旧字段数据哈希一致；迁移账本对照范围为原有 `version<=19` 记录，新增20号记录单独核验。媒体与环境文件不变，运行程序指纹与最终候选构建一致。迁移后、配置激活前，两张新表均为空。没有复制候选数据、重渲染历史文章或产生正式业务写入。

迁移后另行执行了[实际站长配置](../tests/artifacts/v24/activation.json)。使用已有正常 owner 会话及正式 API，初选该站已配置 GitHub 身份下的 [Painter](https://github.com/callme-taota/Painter)、[Tracker](https://github.com/callme-taota/Tracker)、[tolog](https://github.com/callme-taota/tolog)，分别新建正式项目并真实刷新元数据；三次均成功。初选已在执行前说明，后续可在管理页更改，不把它写成用户逐项指定的仓库清单。

正式首页顺序保存为 `intro, articles, activity, projects, steam, music, photos, closing`，项目上限3；日历启用当前公开文章与瞬间、最近365天、`Asia/Shanghai`。激活时实际范围为2025-11-04至2026-11-03，**2次发布 /1天有记录**，并非候选的13次 /2天。没有发布演示文章、晋升候选行、复制凭据或改写原有内容。V23 原正式收藏仍为112款真实游戏、291张去重专辑。

[`v24-activate-user-home.cjs`](../tests/browser/v24-activate-user-home.cjs)是本轮已授权的实际配置操作，**不是只读测试或日常回归**。报告存在时脚本拒绝重复运行；重放其写逻辑会修改站点，不得因为它列在测试目录而自动执行。

### 空白新稿回归与独立程序修正

正式 smoke 的[第一次中断](../tests/artifacts/v24/lan-initial-harness-tab.json)只是脚本把「项目」页签误写成「开源项目」，修正定位后保留记录。继续执行又发现[真实空稿回归](../tests/artifacts/v24/lan-empty-editor-discovery.json)：新增卡片 atom 的优先级1100高于 paragraph 的1000，ProseMirror为新空文档自动填入无效卡片，兼容保护因此切换为源码模式。它影响空白新文章，已有包含正文 / 卡片的验收文章不能覆盖该状态。

最小修正将卡片优先级改为900，低于 paragraph、仍高于 code block 的100，恢复默认空段落。没有放宽兼容判断、HTML白名单、渲染或媒体策略，也没有改变截图中的卡片几何。新增单位和编辑器空稿回归后，最终为7项单位检查、34项编辑器检查及候选 /正式各10项只读空稿检查，全部通过；初次6 /26项是此前的验证范围，不冒充已经覆盖空稿。

[独立程序修正报告](../tests/artifacts/v24/empty-editor-fix.json)记录只替换前端嵌入程序后的正式运行指纹，状态 `updated`；全部58表数据、媒体和环境文件前后不变，迁移账本仍为 count / max20。没有新增迁移、配置写入、候选数据晋升或历史文章重渲染。原迁移 /激活报告继续保留，不被这次程序替换覆盖。候选24图的视觉裁决范围不因此扩大为正式部署批准。

### 正式只读验收与截图

修正后 [`v24-lan-smoke.cjs`](../tests/browser/v24-lan-smoke.cjs)完成[59项正式检查](../tests/artifacts/v24/lan-smoke.json)，验证112款游戏、三个实际仓库、真实2次发布 /1个活跃日、1440 /390 /320布局、项目聚焦和避让、原始游戏封面、后台入口、文章卡片规范预览及7个实际服务资产指纹。浏览器运行错误0，内容 /配置写入0。允许纯计算的正文规范预览和封面预览POST；报告另记直接规范预览1次，不能写成「所有POST为0」。[正式空稿10项](../tests/artifacts/v24/empty-editor/formal.json)是独立回归，未混入59项。

正式另捕获4张当前运行图：[桌面项目](../tests/artifacts/v24/visual/formal-1440-light-projects.png)、[桌面游戏](../tests/artifacts/v24/visual/formal-1440-light-games.png)、[手机日历](../tests/artifacts/v24/visual/formal-390-dark-activity.png)、[手机游戏](../tests/artifacts/v24/visual/formal-390-dark-games.png)。它们记录实际正式内容，不属于此前24张候选独立视觉评审的范围。正式应用与PostgreSQL保持运行，候选在验收结束后停止。

原[飞书预览文档](https://wcno45oaikgv.feishu.cn/docx/NMuTdRputo2Yzfx7sVYc7jjxnRe#doxcnDkf6dkP1lq8jk4zRIH64mc)已追加8张V24图片：4张正式运行图及4张明确标注的候选文章演示图，保留原V23图片；未为展示卡片向正式库发布演示文章。[预览更新记录](../tests/artifacts/v24/feishu-preview.json)确认新标题定位和图片数量，主执行的文档复读核验为0警告。
