# 浏览器与前端回归

本目录保留编解码、DOM 交互、编辑恢复、认证、移动端和真实候选业务的回归源，以及必要的图片 fixtures。命令从仓库根目录运行。

这些脚本不是统一的测试运行器，也不保证每个文件无需准备即可运行。部分用例仍含固定端口、临时路径、指定角色、既存内容或界面状态前提；运行前必须阅读文件头、配置和请求拦截规则。文件名中的版本号用于定位用例来源，不表示当前已完成整套验收。不要批量执行所有脚本。

## 测试分类

| 类型 | 代表入口 | 环境和准备 |
| --- | --- | --- |
| Node 编解码与请求协调 | `v24-cards-unit.mjs`、`v39-cards-unit.mjs`、`v49-request-coordination.cjs` | Node 24 与已安装的前端锁定依赖；不连接应用、数据库或供应商 |
| 独立浏览器运行时 | `v24-card-runtime.cjs`、`v27-image-viewer.cjs`、`v33-guest-interactions.cjs`、`v33-home-snap.cjs`、`v35-home-snap.cjs`、`v39-cards-runtime.cjs`、`v39-game-selection.cjs` | Playwright 与对应浏览器；使用合成 DOM / 拦截响应，部分用例会在临时目录转译前端源码 |
| 独立组件或模板 | `v39-cards-fixture-build.mjs` → `v39-cards-editor.cjs`、`v43-identity-consent.cjs` | 先生成临时组件构建或 Go 模板夹具，再运行浏览器；准备流程见文件头及对应 Go 测试 |
| 带模拟响应的页面回归 | `v11-editor-safety.cjs`、`v11-state-paths.cjs`、`v12-editor-polish.cjs`、`v16-editor.cjs`、`v24-editor-cards.cjs`、`v45-login-regression.cjs` | 通常需要可访问的候选页面；业务响应由用例模拟，但导航、登录态或纯计算预览仍可能依赖候选服务 |
| 真实候选业务 | `business.cjs`、`admin.cjs`、保存/新稿恢复脚本、`v14-friends.cjs`、`v15-search.cjs`、`v20-moments.cjs`、`v21-moment-images.cjs`、`v49-*-real.cjs` | 独立数据库、媒体、测试账号及正常登录状态；会写测试业务数据，需逐一核对前置条件和清理行为 |
| 移动端、安全与集成 | V40–V45、`v48-glass-scroll.cjs`、`v49-article-requests.cjs`、`v49-logout-recovery.cjs` | 各脚本使用的候选、模拟响应、浏览器内核和凭据不同，不能套用同一条准备命令 |

Node 用例示例（先准备前端依赖）：

```sh
node tests/browser/v39-cards-unit.mjs
node tests/browser/v49-request-coordination.cjs
```

浏览器脚本需要开发机已准备的 Playwright 及对应浏览器。Playwright 不属于应用生产运行依赖；部分脚本支持通过环境变量选择模块和浏览器，另一些仍使用固定开发路径，须先适配测试环境。不要为了复用这些路径去读取其他部署的临时账号或会话。

## 环境与私有输入

下列是常见参数，**不是所有脚本都支持**；真实名称和默认值以文件头为准：

| 参数 | 用途 |
| --- | --- |
| `BASE_URL` 或脚本专用候选 URL 参数 | 已准备的隔离应用地址 |
| `PLAYWRIGHT_MODULE` | Playwright 模块名或开发机安装位置 |
| `CHROMIUM_EXECUTABLE` | 对应 Chromium 可执行文件 |
| `ACCOUNT_FILE` | 私有测试账号文件；所需字段与角色以脚本读取逻辑为准 |
| `OWNER_STATE_FILE`、`BROWSER_STATE_FILE` 或专用状态参数 | 正常登录后导出的浏览器 storage state |
| `POST_ID`、`FIXTURE_FILE` 或专用元数据参数 | 当前候选实例中的专用测试记录与夹具映射 |
| `ARTIFACT_DIR`、`ARTIFACT_FILE` 或专用结果参数 | 截图、JSON 报告等输出位置 |
| `CAPTURE_SCREENSHOTS` 或专用截图开关 | 仅在该用例支持时控制截图 |

账号、密码、安装码、Cookie、CSRF、MFA 密钥与恢复码始终保存在仓库外的私有文件中。账号和 storage state 应通过该候选实例的正常创建与登录流程准备，不能复制生产登录态。私有文件限制为操作者可读写，不把真实值放入文档、命令参数、截图或报告。

本地测试账号的创建入口见[本地账号指南](../../docs/local-accounts.md)。有些认证用例要求空库、专用注册策略或 MFA 状态；不能借用已安装的普通候选替代其前提。`v43-identity-consent.cjs` 的合成模板由 `internal/app/oidc_test.go` 的预览夹具入口生成，路径通过 `PAINTER_IDENTITY_PREVIEW_DIR` 传入。

## 隔离与夹具准备

真实业务用例只能针对可丢弃的 PostgreSQL 18 数据库、独立媒体目录和独立应用配置。即使用例名含 smoke 或 read-only，也应先确认实际允许的请求；正常登录、预览或埋点可能产生不同于内容保存的请求。端口限制只是防误操作措施，不能代替检查数据库、媒体和 origin 的归属。

Go 集成用例仅使用显式 `PAINTER_TEST_DATABASE_URL`，不回退到 `DATABASE_URL`。浏览器则访问已启动的候选服务；不能因为 Go 测试使用隔离库，就推断浏览器的 `BASE_URL` 也安全。

保留的候选准备链包括：

- `v16-fixtures.cjs` 为编辑、管理和视觉用例准备明确标记的文章、评论与角色状态；需先准备 owner / author / reader。
- `v20-moments.cjs`、`v21-moment-images.cjs` 会创建和修改隔离动态及图片，不能作为只读检查执行。
- `v22-fixtures.cjs` 为 `v22-home.cjs` / `v22-visual.cjs` 准备展示条目和照片；会读取公开供应商元数据，不能当成无网络单测。
- `v44-mobile-fixtures.cjs` 使用正常测试 owner 和合成收藏准备移动端内容。关联用例还可能需要标签及单独导出的浏览器状态；生成内容不等于已准备这些输入。

这些链仍有各自的固定路径、候选约束和前序状态要求。逐条核对，不把已有映射文件当成当前测试库的证明。重新创建候选库时，应同步清理或重新生成其私有映射与会话。保存恢复用例不要并发操作同一测试文章。

用例可能保留草稿、发布内容、媒体、互动、检查点或临时设置。完成后按该用例的清理约定处理，再销毁整个候选数据库和媒体目录；不要为了旧断言向正式站点添加示例内容或修改配置。

## Fixtures、结果与验证范围

`fixtures/avatar.png` 是小尺寸上传图片；`fixtures/moments/lake-cc0.jpg` 的来源和授权见[图片夹具说明](fixtures/moments/README.md)。这些文件是测试输入，保留在 Git 中；不要将测试内容发布为真实用户内容。

生成的截图和 JSON 报告通常写入已忽略的 `tests/artifacts/`，部分用例默认输出到临时目录。应为每次候选运行设置独立结果目录，先确认报告里的 origin、夹具与当前实例一致，再复用结果。报告不保存 Cookie、口令或真实私有正文。

脚本通过只表示其断言覆盖的范围。模拟响应不证明真实后端持久化；只读候选检查不证明生产迁移；截图数量不代表视觉质量。Chromium / WebKit 的视口与触摸模拟也不替代真实手机、操作系统输入法、帧率或外部账号授权验证。完整数据库、构建和部署检查见[仓库测试说明](../../README.md#测试)。

## 统一操作通知

`toast-core.cjs` 使用独立 DOM 验证通知的去重、数量限制、计时暂停、文本安全、主题、手机位置、原生 dialog 与 popover 回退，不连接应用或数据库。需要前端锁定依赖中的 TypeScript、Playwright 与 Chromium。

`editor-toast.cjs` 访问明确指定的回环候选地址，使用候选站长的私有 `OWNER_STATE_FILE` 进入真实编辑器，拦截业务写入以验证保存 ACK 时序、幂等重试、自动保存安静状态、失败与恢复、发布、冲突、导入、导出和卡片插入。候选 SSR 路由仍需正常登录；模拟写入不证明后端持久化。另以独立候选的真实保存检查完成持久化闭环。

```bash
PLAYWRIGHT_MODULE=/private/node_modules/playwright \
CHROMIUM_EXECUTABLE=/path/to/chromium \
node tests/browser/toast-core.cjs

PLAYWRIGHT_MODULE=/private/node_modules/playwright \
CHROMIUM_EXECUTABLE=/path/to/chromium \
PAINTER_QA_URL=http://127.0.0.1:18515 \
OWNER_STATE_FILE=/private/isolated-owner-state.json \
node tests/browser/editor-toast.cjs
```

## V50 连续列表与搜索

`v50-continuous-reading.cjs` 和 `v50-infinite-recovery.cjs` 固定只访问可丢弃实例 `http://127.0.0.1:18094`。设置 `OWNER_STATE_FILE` 为该实例已有的站长会话，`PLAYWRIGHT_MODULE` 为 Playwright 模块位置，可选 `CHROMIUM_EXECUTABLE` 与 `ARTIFACT_DIR`。会话和账号文件必须放在 Git 外。前者会创建明确标注 UX50 的未公开草稿并保存历史版本；后者只读，不可复用正式实例或正式账号。

夹具前提：超过 20 篇 UX50 公开文章、动态和专栏，超过 30 条公开片语，45 个标题为 `UX50 Game 0` 至 `UX50 Game 44` 的公开游戏；最新公开文章有 45 条评论，测试站长有 30 个收藏。夹具仅用于独立 PostgreSQL 18 测试库。公共脚本用 `limit=3` 验证追加，后台使用实际批量；内容选择器检查自身滚动容器。脚本覆盖两种视口、离开保护、保存与历史版本、失败不循环请求、重试不丢记录、末尾停止、评论和收藏等。

```bash
PLAYWRIGHT_MODULE=/private/node_modules/playwright \
OWNER_STATE_FILE=/private/isolated-owner-state.json \
ARTIFACT_DIR=tests/artifacts/v50 \
node tests/browser/v50-continuous-reading.cjs
# 保持同一环境变量，再运行 v50-infinite-recovery.cjs。
```
