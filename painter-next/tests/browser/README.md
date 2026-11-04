# 浏览器验收

## V24 游戏库、发布日历与内容参考（2026-11-03）

本轮延续已有视觉系统。候选为18081独立数据库 / 媒体，正式为18080；不要把候选文章33、六款游戏样本或项目行随二进制复制到正式库。当前功能、截图、正式发布和实际配置边界见 [V24记录](../../docs/EXPERIENCE_V24.md)，内容语法见[文章卡片合同](../../docs/CONTENT_CARDS_V24.md)。

- `v24-cards-unit.mjs` 用 Node24直接检查最终7项 Markdown / Tiptap编解码，不启动浏览器、服务，不读供应商或写业务数据。初次6项之后新增空文档默认段落回归；依赖工作区已有前端包。
- `v24-card-runtime.cjs` 在拦截的 `card-runtime.test` 页面运行34项共享运行时检查；TypeScript在内存转译，不构建应用、不连接候选或数据库。覆盖 hover / focus / touch、原生目标、重复初始化、单一图片 dialog、失败和溢出，输出2张隔离运行时图。
- `v24-editor-cards.cjs` 拒绝18080，需仓库外私有 `OWNER_STATE_FILE`；支持 `BASE_URL`、`PLAYWRIGHT_MODULE`、`CHROMIUM_EXECUTABLE` 和 `ARTIFACT_DIR`。最终34项覆盖1440浅 /390深的文档/源码卡片、插入编辑、Emoji、未来格式与失败保留，包含初次26项后增加的8项空稿检查。文章读写和供应商读取使用明确夹具 / 模拟；规范预览POST才调用真实候选后端。报告实际业务写入为0，不能说成真实文章保存验收。
- `v24-home-admin.cjs` 固定18081和本机会话 / 浏览器路径，是真实候选写验收：配置预览/保存、CAS、项目CRUD、幂等与真实GitHub刷新，最终86项。临时手动项目删除，三个实际公开仓库和启用配置留在候选；保留已有墙条目。作者 / reader权限与320/390两主题布局同批验证，输出8张后台图。不能指向正式实例或作为只读smoke运行。
- `v24-article-fixture.cjs` 仅准备隔离文章33：读取四类真实供应商信息，复用候选已有owner图片，创建并发布明确标注的专用文章。已有映射时保留。它会写候选内容，不得在正式站点运行或复制其文章 / 映射作为真实内容。
- `v24-public.cjs` 固定18081，从候选映射只读验证真实公共页，拦截所有业务写入并本地应答analytics。最终104项功能检查加同一协调批次30项截图断言，输出14张公共原生视口图。观察真实外部href后以本地文档应答，不实际验收第三方应用；故意失败的Steam封面请求用于回退检查。脚本含本机Chrome / Playwright路径，移植前须核对文件头。

Node /隔离运行时可以独立运行；真实候选写验收须顺序执行，公共截图只在数据稳定后的协调批次采集。脚本中的本机会话和账号文件始终放在仓库外，不能随证据提交。不要为了旧的固定前提向正式库增加演示数据。

最终视觉要求为14公共 +2实际编辑器 +8后台，共24张当前图，另有2张隔离运行时图。独立[收尾裁决](../artifacts/v24/finish-review.md)已打开24图，并在唯一剩余P2日历发现性问题修正后给出SHIP。手机Steam布局修正仅复验14项、替换2公共图；日历近期定位修正仅复验公共22项 /后台38项、替换2公共及4后台图。`*.before-steam-fix.png`、`*.before-calendar-fix.png` 与 `home-admin/before-calendar-fix/` 都是已被替换的历史证据，不能混入当前矩阵计数。初次日历定向报告的键盘检查受触摸模拟后的鼠标hover影响；最终报告明确隔离两种输入后通过，未借此扩大截图范围。

正式发布、owner配置激活与正式只读smoke分别记录。备份dump全解码不等于已恢复到数据库；迁移不重渲染旧发布HTML。`v24-activate-user-home.cjs` 是本轮已授权的实际正式配置操作：使用普通owner接口新建 /刷新三个初选真实仓库、启用发布日历并保存顺序，不能作为日常回归执行；它拒绝覆盖已有activation报告。

`v24-empty-editor-smoke.cjs` 可在候选或正式正常会话下只读运行，需私有 `OWNER_STATE_FILE`，接受 `BASE_URL`、`PLAYWRIGHT_MODULE`、`CHROMIUM_EXECUTABLE` 与 `ARTIFACT_FILE`。最终候选和正式各10项验证空白新稿保持可编辑段落、卡片工具可打开 /取消且不产生内容；所有内容 /配置写入拦截，只允许纯计算的规范 /封面预览POST。它关闭了卡片atom优先级1100高于paragraph1000导致空稿兼容降级的真实回归；最小优先级修正为900，未放宽HTML或兼容保护。原24图的视觉裁决继续限定于当时截图，不能替代此回归。

`v24-lan-smoke.cjs` 固定正式18080和本机正常owner会话 /Chrome /Playwright路径；最终[59项](../artifacts/v24/lan-smoke.json)通过，验证实际112款游戏、3个已刷新仓库、2次发布 /1天、响应式布局 /卡片工具 /规范预览与7个服务资产指纹，输出4张正式当前图。它拦截内容 /配置写入并本地应答analytics，允许正文规范预览及封面预览POST；报告单列直接规范预览1次，不能把0业务写入说成0POST。另行的正式10项空稿回归未计入59项。首次页签定位笔误与随后发现的真实空稿问题分别保留在 `lan-initial-harness-tab.json`、`lan-empty-editor-discovery.json`；最终以通过报告为准。

[独立程序修正](../artifacts/v24/empty-editor-fix.json)只替换程序，58表数据 /媒体 /环境均不变、schema20不变，不覆盖先前迁移和激活证据。正式4图与候选24图分别计数，候选SHIP不替代正式smoke；完整状态见V24记录。

## V17 控件与圆角（2026-11-02）

`v17-controls.cjs`使用仓库外的`OWNER_STATE_FILE`，只读检查搜索、主题、作者配色、编辑标签/封面/段落/更多/源码分段及管理选择器。设置`PHASE=final`、候选`BASE_URL`、`PLAYWRIGHT_MODULE`执行，默认5宽×2主题和宽屏coarse矩阵。`PHASE=baseline`只记录尺寸差异，不强制新规范，不能用来声称最终尺寸通过。`FIXTURE_FILE`可指定隔离文章映射，默认沿用V16候选夹具。所有内容/配置写入拦截，analytics本地应答，预览允许只读请求。

截图必须协调批次后显式设置`CAPTURE_SCREENSHOTS=1`，只采1440/390深浅主题12张局部图，单独记录capture.json，不覆盖final.json。`v17-controls-finish.cjs`仅验证本轮最后两个窄屏布局修正并替换两张对应图。`v17-lan-smoke.cjs`在原LAN用有效私有站长会话只读确认实际发布构建，无内容或配置写入。V16及更早固定尺寸测试记录历史契约，不以旧44px桌面断言替代本轮36px要求。

## V16 当前编辑与管理（2026-11-02）

历史V4–V15脚本记录对应版本的界面与既有内容前提，不能把其中的旧模式标签定位直接用于当前UI。当前编辑安全工作负载由`v16-editor.cjs`在内存中适配V11定位，保留原有断言。

候选服务必须使用可清理的独立schema14数据库/媒体并绑定本机LAN18081。备好私有0600的`ACCOUNT_FILE`，含正常owner/author/reader账号，先设置`PLAYWRIGHT_MODULE`后执行：

```sh
V16_DISPOSABLE_DATABASE=1 node tests/browser/v16-fixtures.cjs
OWNER_STATE_FILE=/tmp/painter-next-v16-owner-state.json node tests/browser/v16-editor.cjs
node tests/browser/v16-business.cjs
node tests/browser/v16-visual.cjs
# 截图只在协调的视觉验收批次开启，不要随每个修改重复。
CAPTURE_SCREENSHOTS=1 node tests/browser/v16-visual.cjs
```

夹具生成器只固定访问18081，创建六篇明确标注的验收文章、三条评论和作者背景/简介；映射和会话保存在仓库外0600文件中。映射已存在时不重复生成；换隔离库时应先移走旧映射和状态。业务脚本会修改该候选库，正式数据库不可运行它们。视觉脚本拦截业务写入，等待可见懒加载封面后截图。`v16-home-evidence.cjs`仅用于记录此次两张未加载封面的技术补采，不是新的设计迭代。

正式已有测试站的`v16-lan-smoke.cjs`使用`ACCOUNT_FILE`正常登录，仅允许会话登录和只读预览，检查55项且无内容/设置写入；它包含当前站点数据与地址前提，不是新安装的通用测试。

## V10 编辑页布局（2026-10-29）

`v10-editor-layout.cjs`是只读现有草稿检查，使用仓库外0600的`OWNER_STATE_FILE`、`BASE_URL`、可选`POST_ID`，验证四种宽度的布局/源码视图/键盘/属性/命令尺寸及数据指纹；意外业务写入会被拦截并判失败。`v10-capture.cjs`使用私有`BROWSER_STATE_FILE`，读取候选库既有1号文章，生成1920/1440/900/390×浅深×文档/属性或封面菜单/源码共24张viewport图。两者沿用下文Playwright/Chromium参数。

真实保存链路复用`v8-document-integration.cjs`，`ARTIFACT_DIR`指向V10独立目录；该脚本仅可用于可清理的克隆库，生成并最终撤回自己的QA文章。随后以报告中的`created_post_id`设置`POST_ID`，顺序运行`v10-save-recovery.cjs`和`v10-new-draft-recovery.cjs`。这两份保留原恢复断言，仅适配V8之后的自定义封面combobox，旧脚本保留作历史记录。恢复用例会写候选库，不能指向正式站。


## V9 导航层级（2026-10-29）

`v9-navigation.cjs`在已安装的隔离克隆库使用正常owner/author/reader账号登录，读取各角色页面并测试层级、键盘、当前项、设置hash/前后退、表单本地保留和手机抽屉。设置`BASE_URL`、仓库外0600的`ACCOUNT_FILE`，格式沿用下文accounts数组并包含三个role。可用`OUTPUT_STATE_PREFIX`导出三个私有状态文件，后缀`-owner-state.json`、`-author-state.json`、`-reader-state.json`，仅用于后续截图，不可提交。

```sh
node tests/browser/v9-navigation.cjs
# STATE_PREFIX指向上述私有文件的公共前缀，不含角色后缀。
node tests/browser/v9-capture.cjs
```

截图脚本需`STATE_PREFIX`；三宽浅深共20张，保存展开状态和滚动指标。测试只修改未提交的本地表单，不保存内容或站点配置；正常登录仍会创建session。`v9-lan-smoke.cjs`用于正式既有测试站的只读导航检查，使用私有`BROWSER_STATE_FILE`；先用`v6-lan-smoke.cjs`正常登录并可导出`OUTPUT_STATE_FILE`。两者按顺序运行。Playwright/Chromium变量沿用下文；本轮没有重跑历史后端101项，不以历史结果冒充V9执行。

## V7 网页安装与 GitHub 配置（2026-10-28）

`v7-install-auth.cjs` 只对可清理的空库与已安装站点克隆库运行。它会完成一次性安装并修改克隆库的 GitHub 配置，不能指向真实站点。`BASE_URL` 为旧站克隆，`FRESH_URL` 为尚未安装的独立空库；两者须已迁移到 schema 11，并有各自正确的公开地址/回调。

设置私有 `BROWSER_STATE_FILE`（克隆库 owner 正常登录状态）与 `INSTALL_FIXTURE_FILE`（0600 JSON：`setup_token`、`username`、`display_name`、`password`、`github_secret`）。安装码必须与空库服务配置一致，测试 secret 必须为假的。另可设置 `OUTPUT_STATE_FILE` 导出新 owner 状态到仓库外。沿用下文的 Playwright/Chromium 环境变量执行：

```sh
node tests/browser/v7-install-auth.cjs
```

脚本覆盖三步实际安装、拒绝错误安装码、普通 owner 登录、永久关闭、旧站封闭、后台 GitHub 启停/密钥不回显/版本冲突/失败重试。它只检查官方授权跳转，不跟随外部 GitHub，也不证明真实 OAuth App 回调可用。最终克隆库 provider 停用，测试凭据仍只在克隆库。三尺寸浅深主题共25图，初始空库完成安装后不可重复运行；新一轮需重建隔离空库。`v7-lan-smoke.cjs` 是正式库只读检查；原 `v6-lan-smoke.cjs` 可继续回归现有文章/编辑器，文章写入应为0。

## V6 标签选择与相关推荐（2026-10-28）

在独立候选库（已有Go/写作标签）按下文配置私有owner状态与浏览器变量，然后运行 `v6-tags-related.cjs`。23项检查包含真实标签保存、Markdown原串保留、移除/重载、规范化检索、键盘/外点、真实20标签上限与归档选择，以及无JS推荐链接/公开隔离；空taxonomy呈现使用隔离响应并单独标记。脚本会创建21个限额标签和多篇文章，成功时保留公开QA供截图，失败时尝试撤回；只能用于可清理的候选库。

`v6-capture.cjs`从固定 `tests/artifacts/v6/experience.json` 读取QA文章，捕获1440/900/390px、浅深主题的标签关闭/展开和推荐区，共18图；仅截图不写内容。用同一QA ID执行 `save-recovery.cjs`，随后执行 `new-draft-recovery.cjs`，保留完整标签/封面/时间恢复断言。新稿脚本已改用combobox与移除按钮检查，不再把时间checkbox误当标签。最后按报告中的 `qa_post_ids` 撤回本轮QA公开文章，保留私有证据；不得把候选库覆盖正式库。

`v6-lan-smoke.cjs`验证更新后的既有LAN测试站，需要仓库外0600的 `ACCOUNT_FILE`；可用 `OUTPUT_STATE_FILE` 导出正常登录状态到仓库外。它读取本地构建manifest核对正式入口，检查现存1号文章的推荐/无JS阅读、手机正常登录及新编辑器，文章写入为0。其固定测试内容前提不适用于空白新安装。

## V5 旧站介绍、渐变与时间选项（2026-10-28）

使用已构建最终前端并嵌入Go的独立候选库，执行全部9次迁移。按下文配置 `BASE_URL`、私有 `BROWSER_STATE_FILE`、`PLAYWRIGHT_MODULE`、`CHROMIUM_EXECUTABLE` 和独立 `ARTIFACT_DIR` 后，顺序运行：

```sh
node tests/browser/v5-experience.cjs
node tests/browser/v4-document-integration.cjs
# POST_ID选择v5-experience报告中已撤回、仍可编辑的专用QA草稿。
node tests/browser/save-recovery.cjs
node tests/browser/new-draft-recovery.cjs
node tests/browser/v5-capture.cjs
```

`v5-experience.cjs`会修改候选站点介绍并创建/发布/撤回专用文章，验证独立时间开关、公开快照隔离、恢复和JSON-LD；不得指向正式站点。恢复脚本已增加三个时间选项的断言。截图脚本只读，从固定的 `tests/artifacts/v5/experience.json` 读取候选草稿ID，输出桌面/手机、浅深主题的28图；因此该报告须属于当前候选库。完整视觉判定另由人工/独立代理查看原图，不能仅凭脚本通过判断美观。

`v5-lan-smoke.cjs`用于更新后的现有LAN测试站，要求仓库外0600的 `ACCOUNT_FILE`，可用 `OUTPUT_STATE_FILE` 把正常登录状态写到仓库外私有文件。它不写文章，验证已配置的逃塔介绍、现存1号文章的渐变与时间、手机登录、编辑器默认值和设置入口。它包含本轮固定资产名及现有测试内容断言，不能当作任意新安装的通用健康检查。

## V4 文档编辑与个人首页（2026-10-28）

候选环境需要先构建前端并嵌入Go程序，保持独立测试数据库。沿用下文环境变量和真实owner状态，新增：

```sh
node tests/browser/v4-document-integration.cjs
node tests/browser/v3-experience.cjs
# 设置上一步已撤回的专用QA草稿ID，再运行两个恢复脚本。
node tests/browser/save-recovery.cjs
node tests/browser/new-draft-recovery.cjs
node tests/browser/v4-public.cjs
```

真实写入流程顺序执行。`v4-document-integration.cjs`验证原文保留、直接文档编辑、真实媒体到发布/恢复/撤回；`v4-public.cjs`只读，验证自定义主题与新滚动首页并批量截图，CI使用SwiftShader，另测无WebGL降级。原恢复脚本已适配先明确进入源码模式，保留原数据断言。

`v4-editor.cjs`是独立Vite组件测试，所有API均mock，须按文件头单独启动开发服务器。它验证文档节点、菜单、输入法、格式/粘贴与原文保护，不能代替真实HTTP/PG链路。当前正式发布依据见 [最新验收](../../docs/VALIDATION.md)。

## V3 历史版本（2026-10-28）

先在可清理的独立数据库创建测试owner，并通过正常本地登录导出私有 `storageState`。新版推荐顺序如下；沿用下文的环境变量，额外设置仓库外的 `ACCOUNT_FILE`：

```sh
export ACCOUNT_FILE=/private/path/local-test-accounts.json
node tests/browser/v3-experience.cjs
# 从上一步 report.json 的 created_post_id 选择已撤回的专用QA草稿。
export POST_ID=YOUR_QA_DRAFT_ID
node tests/browser/save-recovery.cjs
node tests/browser/new-draft-recovery.cjs
node tests/browser/v3-public-smoke.cjs
```

`ACCOUNT_FILE` 格式为 `{"accounts":[{"username":"...","role":"owner","password":"..."}]}`，须为0600文件，不能提交、复制到报告或镜像。新版综合脚本使用它实际登录，并随机注册一位reader、修改密码后重新登录。它验证精简导航、建议搜索、标签、自动/上传封面、检查点恢复、公开封面撤回、浮动与正文点赞同步、RSS及场景设置；成功或捕获错误时尝试撤回自己的公开QA文章并恢复设置，仍须检查最终报告。注册用例需要测试库已允许注册，生产默认无需为测试放开。

两个恢复脚本验证封面与标签同正文/摘要一起进入离线、慢响应与丢响应恢复流程。旧 `v3-public-smoke.cjs` 针对已替换的原生主题select和封面堆叠，保留作历史证据，不用于V4验收。旧公开smoke脚本只读，验证快捷键跳转、真实滚动与减少动效、浮控滚动、主题、无JS阅读/搜索和RSS默认关闭。全部脚本按顺序执行，避免同一草稿的并发写入。

## 首版历史用例

这些脚本对应 `tests/artifacts/frontend-{business,save,new-draft,admin}-qa.json` 中已实际执行的验收流程。它们通过页面与真实 HTTP 接口创建和修改数据，需使用可清理的独立测试数据库，应用和 PostgreSQL 均须运行。前端资产应先构建并重新编译进 Go 程序。

需要 Node.js、Playwright 和 Chromium。浏览器会话来自正常登录且已成为 owner 的账号；通过 Playwright 导出 `storageState` 到仓库外的私有文件。仓库不包含 Cookie、会话状态或测试登录入口。

```sh
# 在 painter-next 目录执行；Playwright 也可安装在外部测试工具目录。
export BASE_URL=http://127.0.0.1:18080
export BROWSER_STATE_FILE=/private/path/owner-state.json
export PLAYWRIGHT_MODULE=playwright
export ARTIFACT_DIR=tests/artifacts
# 若使用 Playwright 自带 Chromium，省略此项。
export CHROMIUM_EXECUTABLE=/path/to/chromium

node tests/browser/business.cjs
node tests/browser/save-recovery.cjs
node tests/browser/new-draft-recovery.cjs
node tests/browser/admin.cjs
```

按上述顺序执行。`business.cjs` 创建文章、上传图片、发布、互动、双标签冲突合并、修订恢复、撤回、回收并恢复；它输出新文章 ID。`save-recovery.cjs` 默认读取该 ID，也可用 `POST_ID` 指定测试账号可编辑的草稿，验证离线摘要恢复、响应延迟期间继续输入、校验错误纠正和导出。`new-draft-recovery.cjs` 验证首次创建前的新稿断网写作、关闭标签后显式找回、完整元数据、创建成功但响应丢失后的原 ID 映射与清理，以及其他用户前缀记录不可见（同账号浏览器内构造 foreign-user 前缀，不声称真实账号切换）。`admin.cjs` 验证头像、站点资料、功能开关、主题归档、全站审核及浅深/系统主题。

可用 `UPLOAD_FIXTURE_FILE` 替换随仓库提供的 32 × 32 PNG 测试图。`PLAYWRIGHT_MODULE` 接受模块名或外部 Playwright 安装目录。截图和结构化报告写入 `ARTIFACT_DIR`。

业务脚本和新稿恢复脚本最终将测试文章保留为草稿；点赞、评论、收藏、媒体和检查点会保留在测试库。管理脚本恢复原有简介、头像与评论/收藏/关注开关，保留归档的测试分类；站点品牌只原值保存。若使用原始用户名不满足当前校验的历史测试账号，会将其改为 `browser_qa`。清理测试库应在所有验收和备份验证完成后进行。

本验收覆盖 Linux Chromium 的中文文本输入与组合输入保护逻辑，不替代不同操作系统真实输入法的设备验收，也不访问真实 GitHub OAuth 授权弹窗。


## V11 异常路径

`v11-editor-safety.cjs` 与 `v11-state-paths.cjs` 接受 `BASE_URL`、私有 `OWNER_STATE_FILE`、`PLAYWRIGHT_MODULE`、`CHROMIUM_EXECUTABLE` 和 `ARTIFACT_DIR`。所有业务修改请求均拦截或模拟；脚本报告不保存 Cookie、登录凭据或真实正文。默认不截图，`CAPTURE_SCREENSHOTS=1` 开启同批状态截图。前者验证初始化 / 重试、手动冲突与浏览器恢复、文档 / 源码恢复锁及异常读取；后者验证原生搜索链接 / 触摸、媒体 / 评论状态、超过首批分页的精确片语重载及失败保留。

实际业务回归继续使用 `v8-document-integration.cjs`、适配自定义选择器的 `v10-save-recovery.cjs` 和 `v10-new-draft-recovery.cjs`；这些脚本必须在独立测试库执行。保存恢复的 `POST_ID` 应指向专用未发布 QA 稿件，不并发操作同一文章。V10 布局脚本仍可只读执行。


## V12 编辑页焦点与留白

`v12-editor-polish.cjs` 使用私有 `OWNER_STATE_FILE`，支持 `BASE_URL`、`PLAYWRIGHT_MODULE`、`CHROMIUM_EXECUTABLE`、`ARTIFACT_DIR`。默认输出截图，`CAPTURE_SCREENSHOTS=0` 关闭截图；文章读取使用视觉夹具，真实业务修改全部拦截。验证宽屏 / 高屏 / 平板 / 手机的浅深主题、空稿焦点、已有正文选区、标题及命令键盘焦点、强制颜色、44px目标、网格多余留白和横向溢出。浏览器状态和凭据必须保存在仓库之外。

## V13 阅读页与滚动导航

`v13-reading.cjs` 使用已有公开文章 `/posts/1` 及其三篇相关推荐、私有 `OWNER_STATE_FILE`。支持相同的环境路径参数；默认不截图，`CAPTURE_SCREENSHOTS=1` 输出1440/390px浅深主题的首屏、时间层和相关推荐12张图。另验证900/320px、无JS原生信息披露、真实作者 / 推荐链接、手机编辑操作避让；无封面 / 摘要 / 标签 / 时间仅通过DOM呈现模拟，未修改服务器数据。Markdown短线的DOM样本不代表持久化测试，全部8种日期开关由 `TestReadingInfoRespectsEveryPublishedDateVisibilityCombination` 在一次性PG库验证。允许只渲染的预览POST，拦截其他业务写入及模拟埋点收据。

`v13-header.cjs` 验证真实SSR公共页面与手机工作台顶部栏：下滚隐藏 / 上滚恢复、布局空间、键盘焦点、搜索和抽屉、回弹、微小抖动、页面恢复、响应式重挂载、减少动画、无JS及首页例外。工作台会话响应 / 列表使用有标识夹具，公共页面使用真实内容；为短页追加的滚动空间和原生菜单为DOM测试样本。所有业务写入拦截，默认不截图。两脚本均需 `BASE_URL`、`PLAYWRIGHT_MODULE`、`CHROMIUM_EXECUTABLE`，结果目录以 `ARTIFACT_DIR` 指定；Cookie与账号文件始终放在仓库之外。

## V14 友链

`v14-friends.cjs` 是真实业务写验收，**始终拒绝正式18080端口**。默认只允许18081一次性候选；其他测试端口必须显式设置 `ALLOW_CANDIDATE_WRITES=I_UNDERSTAND_DISPOSABLE_CANDIDATE`。使用私有 `ACCOUNT_FILE` 正常站长登录，验证CRUD、显示 / 隐藏 / 排序 / 搜索、CAS冲突与确认、页面设置及公开入口。默认删除仅本次创建的记录并恢复原设置，验证原记录指纹不变；`KEEP_FIXTURES=1` 留下4条明确带“V14验收”标签的记录用于视觉复核，不能复制到正式库。`OUTPUT_STATE_FILE` 只允许/tmp私有0600路径；报告去除账号和口令。外链目的地使用example.invalid的模拟响应，只验证原生新标签及opener隔离，不宣称验证外部网站内容。默认无截图。

`v14-visual.cjs` 需已有3条公开 / 1条隐藏的候选记录与私有 `OWNER_STATE_FILE`，验证1440/900/390/320px浅深主题、SSR与无JS、头像回退、列表 / 设置布局、导航 / 自定义选择器 / 键盘分区、失败读取重试、未保存开关及访问统计退出。全部持久化业务写入拦截，埋点只记录事件名并模拟收据。默认无截图；`CAPTURE_SCREENSHOTS=1` 输出22张全页图，包含两张明确标注的DOM空态样本。此脚本针对有验收记录的隔离候选；正式空实例使用只读smoke，不能为了通过断言向正式库塞入记录。两脚本沿用 `BASE_URL`、`PLAYWRIGHT_MODULE`、`CHROMIUM_EXECUTABLE`、`ARTIFACT_DIR`。

### V15 搜索和管理员工具

- `v15-search.cjs`：仅18081候选服务，正常站长登录；创建四篇明确命名的隔离文章，验证真实搜索筛选。`CAPTURE_SCREENSHOTS=1` 才输出16张图，默认只检查。读取 `ACCOUNT_FILE`，Playwright模块用 `PLAYWRIGHT_MODULE` 指定。
- `v15-lan-smoke.cjs`：正式环境只读检查，唯一允许的写请求为正常登录。访问统计被拦截，任何内容/配置写入导致失败。不依赖正式环境存在示例文章。
- `../cli/v15-lifecycle.py`：只允许18081候选origin；指定 `ACCOUNT_FILE`、`PAINTER_BINARY`，可用 `BASE_URL`。测试创建独立标签和文章并走发布/撤回，不能对正式实例运行。凭据和草稿留在私有临时文件，报告不记录口令或会话。

现有正式LAN内容由用户维护；不要为了旧测试恢复已被删除或撤下的演示文章。

## V22 首页内容与展示墙

- `v22-fixtures.cjs` 只向18081隔离候选导入三款真实Steam商店元数据、三张真实Apple Music专辑元数据及三张带说明的CC0照片；私有映射保存在仓库外。不会连接站长个人账号，也不能向正式库复制这些内容。
- `v22-home.cjs` 同样固定18081，验证首页配置、数量、排序、CAS、上游导入预览、照片上传丢回执重试、可见性/引用和原图查看器。恢复首页配置，删除本次保存的临时照片条目；未引用的上传仅留在候选库。
- `v22-visual.cjs` 在显式设置 `V22_DATA_STABLE=1` 后进行只读17张截图。`V22_CHECKED_REVIEW=1` 只复验受复选框修复影响的三个后台画面，保留初版证据。拦截业务写入和埋点。
- `v22-lan-smoke.cjs` 固定正式18080、读取已有私有会话，只读验证新管理入口、默认首页、无候选数据、响应式布局及嵌入资产指纹。所有业务写入拦截；不会为了检查三面墙而向正式环境塞入内容。

这组脚本记录本地验收环境的Chrome、Playwright和私有会话文件路径，移植时须换成独立测试环境的正常登录状态。凭据、Steam密钥和Cookie不得放入仓库或报告。`TestHome*` Go用例通过 `PAINTER_TEST_DATABASE_URL` 指定测试集群，并为各用例创建/销毁独立数据库；不使用应用的 `DATABASE_URL`。

## V23 沉浸游戏与公开歌单

`v23-home-admin.cjs` 固定18081隔离候选，使用仓库外正常 owner / author 登录状态和本机 Chrome / Playwright。候选新版本与迁移就绪后，在项目根执行：

```bash
node tests/browser/v23-home-admin.cjs
```

脚本会真实读取用户指定的公开歌单、显示300曲 /291专辑 /24张预览与部分读取提示，确认导入本地唱片库。它保留既有条目，新增歌单条目按实际合同可见；**这是候选业务写测试，不能指向正式实例，也不能把导入结果当作只读测试或随程序发布。** 再次执行按专辑身份去重，可出现新增0 /保留291。测试仅删除自己创建的隐藏 Steam 临时项，不删除原有游戏、唱片、照片或 fixture。

46项最终检查包括：502预览失败保留旧内容、条目/配置未保存时禁止导入、受控版本冲突及状态刷新保留输入、第一次真实导入成功后人为丢失回执并用相同 key / version / preview ID 重试、列表无重复、Steam真实POST/PUT不提交只读分钟、空Hero/Logo保留自动语义、固定 /games 地址、320/390两主题无横溢和作者403。手机几何检查复用已经验证的预览响应，不再重复请求上游；不截图，埋点由本地响应替代。

结果写入 `tests/artifacts/v23/admin-browser.json`。首轮 `admin-browser-first-harness-error.json` 保留真实新增291及成功重试证据；其后临时游戏夹具漏了必填平台封面，补齐测试数据后最终46项通过，产品源码未因该夹具问题变更。人为丢失回执产生的预期网络错误单独记录，运行时 `pageerror` 为0。

`v23-fixtures.cjs` 只准备隔离候选：从正式库只读取得六款原本已公开的游戏，重新读取官方元数据并仅写候选；不复制Steam身份、密钥或账户凭据。候选小时取可确认的旧正分钟，不能把这六款样本说成真实账户同步。它还会更改候选首页编排并暂时隐藏候选已有游戏，须使用可清理的隔离库。

公共验收按同一协调批次执行，需先有实际歌单导入及六款游戏样本：

```bash
node tests/browser/v23-fixtures.cjs
node tests/browser/v23-showcase.cjs
node tests/browser/v23-admin-visual.cjs
```

`v23-showcase.cjs` 拦截业务写入并模拟埋点收据，最终109项通过、0运行时错误，捕获18张原生视口图；覆盖入口 / 游戏 / 出口 / 唱片、五项上限、完整库排序分页、字标失败回退、减少动态和无JS。`v23-admin-visual.cjs` 只读取实际候选歌单预览、捕获2张后台图，不确认导入。两者合计20张原始图，不表示所有状态都在每个尺寸与主题拍摄。

独立评审发现唯一P2手机顶部小字对比度问题后，`v23-contrast-fix.cjs` 仅复验受影响的390px浅深与320px浅色范围：五款游戏15张顶部裁切及3张完整补图，等待真实字标解码后截图；没有另开完整矩阵。最终[视觉裁决](../artifacts/v23/visual-review.md)保留初次FIX、确认P2解决并在20+15+3证据范围给出SHIP。[detector记录](../artifacts/v23/detector-review.md)为0主要问题 /2项独立模板颜色提示，未扩展全局palette或添加忽略规则。

这些V23脚本固定当前LAN origin、本机Chrome、`/tmp/painter-browser/node_modules/playwright`及仓库外私有会话路径；它们不采用旧脚本的通用环境变量。移植时先核对文件头及正常登录状态，保持18081候选边界，并重新准备独立数据库 / 媒体。不要把Cookie、账号、Steam密钥或会话文件加入仓库或报告。截图仅在协调验收批次运行；修正批次只限真实材料问题。

正式发布已完成schema17→19，54张旧表的旧字段在主动配置前均保持计数 / 哈希一致，媒体与环境文件未变，候选行未晋升。[`v23-activate-user-walls.cjs`](v23-activate-user-walls.cjs)是**实际owner配置操作，不是只读测试或日常回归**：本次在用户授权下用普通接口另行导入291张专辑、同步并保留112款真实游戏、保存首页5款游戏 /24张封面的相邻编排。重放它会触发正常写入，不能因为测试文档列出它而自动执行。

配置完成后执行的 `node tests/browser/v23-lan-smoke.cjs` 固定正式18080并使用已有私有owner会话，拦截所有业务写入、模拟埋点。最终[104项只读检查](../artifacts/v23/lan-smoke.json)通过，核对7个实际服务资产指纹、0业务写入、0JS错误，另捕获4张当前正式游戏 /音乐图。4张运行截图与独立评审38张范围分别记录。正式及PostgreSQL服务保持运行，隔离候选已停止；完整状态、日志、迁移与激活边界见[V23记录](../../docs/EXPERIENCE_V23.md)及[最终汇总](../artifacts/v23/summary.json)。
