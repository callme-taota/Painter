# 首版验收记录

2026-10-24。使用最终 Go 源码、真实 PostgreSQL 18.6、本机 Chromium 和真实 HTTP 接口验收。测试集群及浏览器身份是独立一次性夹具，未连接旧站 MySQL 或线上站点。首版验收时预览监听 `127.0.0.1:18080`；随后按用户要求切换为 `192.168.2.209:18080`，见文末局域网补充验收。

## 后端与数据库

```bash
PAINTER_TEST_DATABASE_URL='host=/path/to/test-socket port=65439 user=test_admin dbname=postgres sslmode=disable' \
POSTGRES_BIN_DIR=/path/to/postgresql/18/bin \
go test -race -json ./... -count=1

go vet ./...
make build
```

结果：**35 个测试组通过，0 个失败，race 检查通过**。JSON 原始结果在 [`go-test.jsonl`](../tests/artifacts/go-test.jsonl)。依赖 PG 的组在没有测试 DSN 时会跳过，因此单独运行普通 `go test` 不能当成完整数据库验收。测试账号需要在专用集群创建/删除临时库的权限，不应指向生产集群。

实际覆盖：

- GitHub PKCE、state 一次消费/浏览器绑定、默认关闭注册、数值 ID 站长引导、已导入身份引导；CSRF、资料版本冲突、角色即时变化及会话撤销。
- 草稿与公开修订隔离、发布事务回滚、原命令重放、版本冲突、持久 client_draft_id、撤回/恢复/回收站、空标题工作副本恢复前留存、XSS 净化。
- 中文短词/长词、多词 AND、字面通配符、Unicode 规范化、排序、签名游标和发布代次、超时后连接可复用。
- 图片格式头与尺寸解析、上传/读取授权、staging 中断恢复、引用保护和 GC；站长修改文章时原作者仍能继续使用原有图片；恢复时反向媒体顺序的受控并发。
- 评论/赞/收藏/关注/历史、管理设置及权限；埋点去重、站点/文章独立 UV、较小 sequence 晚提交、聚合原子性、lease fencing 与 outbox 收据。
- 真实 pg_dump / 隔离 pg_restore、媒体 hash、维护门、连接 TLS 参数传递、备份记录；旧数据包 dry-run、导入/回滚、关系保留及序列校准。

## 浏览器与产品

最终4份流程报告合计 **51项检查通过、0个浏览器脚本异常**（这不等于完整设备兼容认证）。桌面 1440 px、手机 390 px，使用本机 Chromium，公开页面同时验证禁用 JavaScript 的阅读和搜索。验收身份只通过测试库夹具创建，不是产品的登录后门；实际部署必须配置 GitHub OAuth。

| 流程 | 结果 / 证据 |
| --- | --- |
| 创作到公开阅读 | 创建、自动保存、表格末尾上传图片、检查点、发布、公开图片读取、评论、点赞、收藏均通过；[`frontend-business-qa.json`](../tests/artifacts/frontend-business-qa.json) |
| 首次同步前的新稿 | 离线新建→关闭标签→按账号发现并恢复完整元数据；首次创建成功但响应丢失→新标签按clientID找回原记录，额外POST为0；postID恢复点持久后清理旧记录、不重复提示，共8项通过；[`frontend-new-draft-qa.json`](../tests/artifacts/frontend-new-draft-qa.json) |
| 并发与生命周期 | 两标签页冲突保留本地内容、显式合并、修订恢复为草稿、撤回后公开404、移入回收站/恢复均通过；同一报告共23项 |
| 保存可靠性 | 离线保留、关闭重开恢复仅摘要、服务器不被擅自覆盖、延迟响应不覆盖新输入、后续串行保存、校验错误纠正、Markdown 导出共8项；[`frontend-save-qa.json`](../tests/artifacts/frontend-save-qa.json) |
| 运营写入 | 资料/真实头像公开读取、站点资料、评论/收藏/关注开关关闭再恢复、分类创建归档、owner全站审核、主题及手机检查共12项通过；[`frontend-admin-qa.json`](../tests/artifacts/frontend-admin-qa.json) |
| 移动阅读与编辑 | 无横向溢出、编辑默认单栏、导航不占满屏、18 px 正文、目录/标题、焦点可见、CM6 有可访问名称；[`independent-final-qa.json`](../tests/artifacts/independent-final-qa.json) |
| 导航与搜索 | 手机菜单→分类→筛选文章→作者→带作者条件中文搜索，真实结果与条件保留；[`independent-public-flow.json`](../tests/artifacts/independent-public-flow.json) |
| 主题 | 浅色/深色/跟随系统可来回切换，跟随系统响应 OS 变化；最终暗色 CM6 两种主要语法色对背景对比度实测7.95:1/9.00:1 |
| 权限与隐私 | 匿名访问个人/管理/私有草稿返回401，个人页转登录；公开用户投影不泄漏邮件、手机号、会话或秘密 |
| 备份观测 | 真实从“未记录”变为“隔离恢复已通过”，创建与恢复时间来自持久记录；[`independent-backup-observation.json`](../tests/artifacts/independent-backup-observation.json) |

可复现脚本及运行前提见 [tests/browser/README.md](../tests/browser/README.md)，不包含 Cookie 或测试登录接口。

独立复核另验证新稿恢复列表的当前用户/当前origin过滤，以及390px无溢出；该项拦截服务器写入以聚焦本地找回，真实首次写入和丢响应重试由上表另一个用例验证。见 [`independent-new-draft-recovery.json`](../tests/artifacts/independent-new-draft-recovery.json)。

浏览器验收修复了图片插入被 Markdown 表格吞并后引用不匹配、暗色编辑器语法低对比、头像显示、系统主题恢复、完整元数据本地恢复和管理请求幂等键问题。局域网初版使用`studio-B3YnwdOB.js`、`public-BdHaKL4P.js`和延迟加载的`MarkdownEditor-Cue5ZDCd.js`；当前二进制/资产hash见 [`build-manifest.json`](../tests/artifacts/build-manifest.json)，后续界面迭代见下文补充验收。

可查看的截图包括 [`desktop-editor.png`](../tests/artifacts/desktop-editor.png)、[`desktop-conflict.png`](../tests/artifacts/desktop-conflict.png)、[`desktop-published-qa.png`](../tests/artifacts/desktop-published-qa.png)、[`review-final-mobile-editor.png`](../tests/artifacts/review-final-mobile-editor.png)、[`review-final-mobile-dark-article.png`](../tests/artifacts/review-final-mobile-dark-article.png)、[`review-final-desktop-dark-editor.png`](../tests/artifacts/review-final-desktop-dark-editor.png)。早期诊断截图属于修复前证据，不代表最终状态。

## 备份、迁移角色与打包

- 本地 CLI 实际生成备份、恢复到随机隔离库并核验，然后成功删除隔离库。最后一轮备份包含浏览器上传的 ready 媒体，见 [`cli-final-backup.json`](../tests/artifacts/cli-final-backup.json)、[`cli-final-restore-check.json`](../tests/artifacts/cli-final-restore-check.json)。没有替换原库。
- 在一次性 PG 集群运行交付的 `deploy/init-roles.sh`：迁移账号应用全部迁移，运行账号建示例数据/启动 HTTP；验证业务 DML 和序列权限有效，但无 superuser/建库/建角色/schema CREATE/迁移记录 DML。见 [`deployment-roles.json`](../tests/artifacts/deployment-roles.json)。
- 配置脚本实际生成生产 URL、两个独立32字节密钥、三个不同数据库密码；`.env`0600、导出目录0700，拒绝覆盖既有配置；真实插值后的 Compose 配置合法。报告为 [`configure-check.json`](../tests/artifacts/configure-check.json)，不含密钥。
- `vue-tsc --noEmit`、Vite production build、CGO禁用的 Go 二进制构建、`go vet`、`bash -n`、Compose config 均通过。提交的前端产物支持纯 Go 构建。
- 镜像版本和多架构 manifest 摘要已固定于 [`images.lock.json`](../deploy/images.lock.json)。独立部署复核修复了强制 amd64 覆盖目标平台、PG 初始化临时 socket server 被健康检查提前判定就绪两项问题。

## 实际验证范围

当前环境没有运行 Docker 守护进程，**未执行容器构建/启动**。Compose 静态检查和真实 PG 角色检查不能替代完整容器演练；本次交付可以直接运行的本机二进制，并提供容器部署配置。

未声称完成以下外部条件：真实 GitHub OAuth App 的公网回调、Windows/macOS/iOS/Android 四种物理设备中文输入法、目标1 vCPU/1 GiB机器的联合负载测试、真实旧 MySQL 全量导出切换、异机存储同步及灾备演练。Chromium 移动尺寸模拟不等同物理手机验收。

预览使用专门的临时 PostgreSQL 集群、媒体目录和本地验收身份。长期使用需按 [README](../README.md) 创建自己的持久部署、保管密钥并绑定本人 GitHub 数值 ID；没有默认管理员或演示密码。

## 局域网补充验收

同日按用户要求开放局域网HTTP，改为只监听`192.168.2.209:18080`并同步公开origin。修复`crypto.randomUUID`仅安全上下文可用导致局域网HTTP模块加载失败的问题，统一使用密码学随机UUID helper；健康检查也支持指定监听IP。

在真实LAN IP HTTP页面验证`isSecureContext=false`、原生`crypto.randomUUID`不可用；公开页与工作台加载正常。既有business23项与save8项在该origin重新执行全通过，pageerror为空，公开QA文章已撤回。证据见 [`http-context.json`](../tests/artifacts/lan/http-context.json)、[`frontend-business-qa.json`](../tests/artifacts/lan/frontend-business-qa.json)、[`frontend-save-qa.json`](../tests/artifacts/lan/frontend-save-qa.json)。本次由同一主机浏览器使用局域网地址验证，未冒充另一台物理设备实测。

应用和测试PG由用户级临时systemd服务管理，当前处于active；PG没有开放网络端口。未修改系统防火墙配置，当前UFW配置ENABLED=no。启动/停止方式及Compose局域网配置见 [LAN_ACCESS.md](LAN_ACCESS.md)。

## 2026-10-25 安装与运维脚本补充验收

`PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests/deploy -v` 的45项检查通过，根代理独立复跑仍全部通过（13.957秒）。覆盖三种访问模式、秘密与已有配置保留、残留卷保护、备份/导出失败、构建/拉取失败、停旧应用后再迁移、迁移失败不激活、健康超时、镜像配置成功后才提交、安装包白名单和解包后按指定镜像安装。测试使用临时目录及严格 Docker mock，不连接当前预览、不执行主机安装或外网发布。

另用真实 Docker Compose CLI 做只读配置解析：local/lan/https 三种模式、指定 IPv4 与端口、自定义项目镜像，以及含特殊字符的 secret 字面值往返均通过。所有 Bash 脚本语法检查、`go test ./...` 和 `go vet ./...` 通过；本轮普通 Go 测试未提供测试 DSN，不能算成重新执行了完整数据库集成测试。

具体命令与范围见 [部署测试说明](../tests/deploy/README.md) 和 [报告](../tests/deploy/TEST_REPORT.md)。本次仍未启动 Docker daemon，未执行容器构建/运行、主机 Docker 安装、公开 DNS/ACME 或 NAS 实测；没有发布公共应用镜像或下载地址。

## 2026-10-25 界面补充验收

沿用纸面、墨色、蓝色和克制玻璃的既有体系，实际参考 [Maggie Appleton](https://maggieappleton.com/) 的个人刊物组织、[Josh Comeau](https://www.joshwcomeau.com/) 的文章层级和分类入口，以及 [Apple Materials](https://developer.apple.com/design/human-interface-guidelines/materials) 的功能层材质原则。来源与截图见 [design-references](../tests/artifacts/design-references/sources.json)。CSS 实现是 Web 上的材质近似，不是原生 Liquid Glass API。

首页、阅读页、工作台和编辑器在 1440×1000 / 390×844、浅色/深色下共16张最终截图，HTTP全部200，无横向溢出和页面脚本错误。对比与证据见 [before](../tests/artifacts/redesign-before/home-desktop-light.png)、[after](../tests/artifacts/redesign-final/home-desktop-light.png) 和 [audit.json](../tests/artifacts/redesign-final/audit.json)。截图复制到 `web/.impeccable/review/`，没有额外增加设计轮次。

独立新上下文代理逐张看过最终矩阵，按 Impeccable 备用 finish-reviewer 契约返回 **disposition: ship、material_fixes: none**，见 [完整复核](../tests/artifacts/redesign-final/finish-review.md)。机械 detector 仅运行一次；唯一 flat-type-hierarchy 提示与实际计算字号和渲染不符，已由独立复核判为误报，未重跑检测器。真实字体加载、滚动玻璃/减少透明、键盘信息展开、手机文章编辑/回收/恢复见 [第一轮交互记录](../tests/artifacts/redesign-round1/details.json)。第二轮修正了手机退出入口和编辑器预览标题排版。

同一批最终资产上的真实浏览器回归共39项通过：创作、发布、互动、冲突和生命周期23项，保存8项，新稿恢复8项；各报告见 [summary.json](../tests/artifacts/redesign-final/summary.json)。写用例顺序执行，最终验收文章均为非公开草稿；不向生产添加登录后门。

重新嵌入并部署到 `http://192.168.2.209:18080` 后，根代理另验证8项：新资产实际送达、自托管 Newsreader 已加载、阅读/工作台可用、手机编辑器信息可键盘开合、退出入口可达、无横向溢出/脚本错误，见 [promoted-lan-smoke.json](../tests/artifacts/redesign-final/promoted-lan-smoke.json)。当前资产为 `studio-DjgSl3vJ.js`、`public-CafVZ9Zl.js`、`public-BElquSvo.css`、延迟加载 `MarkdownEditor-XhMgRyOP.js`；[构建清单](../tests/artifacts/build-manifest.json) 已更新。18080应用和PG服务均active；18081临时候选已停止。保存/新稿16项在正式18080地址执行，业务23项在切换前的同资产18081候选执行。

前端类型检查/构建与最终CGO禁用构建通过。设计规范和 sidecar 只记录本次授权的系统变化；没有开展额外配置修复，历史 `buildPath` 未设置提示仍保留。浏览器仍是本机 Chromium 的两种视口，未冒充物理手机、真实GitHub授权或服务器负载验收。

## 本地服务器偏好补充

用户明确选择“个人气质突出 + 本地服务器”后，只将交互安装的访问模式默认值改为 `lan`，并保留 IP 确认步骤、显式模式选择和非交互默认仅本机访问。新增标准库伪终端回归验证实际提示及确认顺序；部署测试共48项通过（13.906秒），Bash语法检查通过，见 [本轮报告](../tests/deploy/TEST_REPORT.md#interactive-local-server-default-follow-up)。未修改已有 `.env`、运行服务或防火墙，未进行真实Docker部署；当前局域网预览的 `/readyz` 再检查为ready。

## 2026-10-28 标签、搜索、封面与本地账号验收

本节是当前版本记录；上文GitHub-only、分类入口与旧资产名属于首版历史。当前使用本地用户名/密码，GitHub可选；产品只保留标签，RSS默认关闭。实现与数据时序见 [EXPERIENCE_V3.md](EXPERIENCE_V3.md)。

最终后端在真实PostgreSQL 18.6执行 `go test -race -json ./... -count=1`，**58项通过、0失败**，没有依赖数据库的跳过用例；`go vet ./...`通过。[原始JSON](../tests/artifacts/v3/go-test.jsonl) 与 [汇总](../tests/artifacts/v3/go-test-summary.json) 可复查。新增覆盖本地登录/注册/限速/改密与离线重置、会话撤销并发、统一标签迁移和历史导入、自动/上传封面权限与修订、真实备份隔离恢复、公开建议隐私、RSS与浮控配置CAS。

独立安全复核发现“等待用户锁后沿用旧session快照”的实质问题，修复后两条受控PostgreSQL回归与独立复跑通过，最终结论SHIP。改密取得排他policy门，普通写在用户锁后重新查询session/CSRF。完整分析及验证边界见 [security-review.md](../tests/artifacts/v3/security-review.md)。

本轮真实浏览器合计 **58项通过、0 pageerror**，使用正常本地账号会话及独立候选数据库，写流程依次运行：

| 流程 | 检查数与证据 |
| --- | --- |
| 新版综合业务 | 25：精简header/footer、真实建议/键盘、过滤位置、编辑器快捷键保护、标签+自动封面、未选上传图暂存、上传/保存/发布、双向点赞、检查点恢复不改公开、撤回、实际手机登录注册改密、RSS及场景设置并恢复；[报告](../tests/artifacts/v3/browser-final/report.json) |
| 保存与纠错 | 8：摘要与封面离线关闭/重开恢复、服务器未经确认不变、慢响应不覆盖新输入、后续保存包含新封面、校验错误纠正与Markdown导出；[报告](../tests/artifacts/v3/recovery/frontend-save-qa.json) |
| 新稿与丢响应 | 9：首次创建前的IndexedDB标签/封面完整性、用户前缀隔离、显式恢复、已提交丢响应找回原clientID、不重复POST、持久化postID后清理旧key；[报告](../tests/artifacts/v3/recovery/frontend-new-draft-qa.json) |
| 公开只读 | 16：斜杠唤起、首次ArrowUp选择末项、建议回车真实跳转、真实滚动视差/减少动效静态、右侧主题与OS跟随、文章回顶/到底、手机轨道与正文不重叠、无JS正文/搜索、RSS关闭；[报告](../tests/artifacts/v3/public-smoke.json) |

综合业务中的公开QA文章已撤回，RSS和场景设置恢复原值；恢复用例留下非公开QA草稿。四个交付账号由离线CLI创建，全部经真实登录验证角色；[无密码验证结果](../tests/artifacts/v3/local-account-verification.json)。另一个随机注册reader仅用于改密验收，不作为交付账号。密码和session只在仓库外0600私有文件，未进入文档、截图、镜像或报告。

安装向导、首次owner创建说明及白名单安装包已同步本地账号；48项隔离Docker mock检查和Bash语法通过，见 [部署报告](../tests/deploy/TEST_REPORT.md#local-account-deployment-guidance-follow-up)。这不代表真实容器构建运行：当前机器没有可用Docker daemon，未安装主机依赖、开放数据库网络端口、修改防火墙或发布公共镜像。

新版从旧预览的真实备份恢复到独立数据库与媒体目录，原库/媒体/二进制/私有配置保留。备份包含14个ready媒体对象，[备份元数据](../tests/artifacts/v3/pre-migration-backup.json) 无密钥。切换前只读检查旧库：备份后用户、文章、修订、站点配置、评论、媒体、标签与互动无新增/更新，唯一新审计为本次 `system.backup`；[核对记录](../tests/artifacts/v3/source-freshness.json)。示例文章旧GitHub-only说明使用正常CAS保存和发布创建新修订，没有改写不可变历史。

最终视觉矩阵共28张：1440×1000和390×1000，浅/深色，首页、文章、搜索浮层、结果、登录、注册与编辑器。全部捕获来自最终 `public-9sRJ_vGi.js`，[audit.json](../tests/artifacts/v3-final/audit.json) 无横向溢出与脚本异常。独立reviewer逐张检查全部28图，明确无实质修改项；线程限额导致复用未参与本轮实现的review角色，未声称全新空上下文。[最终审阅](../tests/artifacts/v3-final/finish-review.md)。文章全页初始捕获的离屏互动占位来自400px预加载边界的懒加载；正常滚近后的互动已由25项综合用例实际验证。

唯一一次结束detector由根执行，输出为 [空数组](../tests/artifacts/v3-final/detect.json)，exit0；没有增加第三轮有效截图或第二次扫描。设计文档代理已独立完成只读准备，重新唤醒被工具以 `agent thread limit reached` 拒绝，根按同一专门角色完成 `DESIGN.md`/v2 sidecar的授权范围合并。23个浅深语义颜色保持不变，8个标准正文分节、10个组件预览及引用关系校验通过，见 [记录校验](../tests/artifacts/v3-final/design-record-check.json)。没有开展config/buildPath漂移修复。

最终Vue类型检查、Vite构建和CGO禁用二进制构建通过。编辑器仍单独延迟加载，Vite保留CodeMirror主chunk超过500kB的提示；这不是首次公开页面的入口包。当前二进制及全部嵌入资产hash见 [build-manifest.json](../tests/artifacts/build-manifest.json)。

新版已切换回 **<http://192.168.2.209:18080>**，应用/测试PG active，18081候选inactive；仅监听指定LAN IP，数据库无TCP端口。切换前再次核对旧预览无新业务变化，保留旧库/媒体及私有回退材料；[切换记录](../tests/artifacts/v3/promoted-runtime.json)。正式18080又执行15项实际验证：四账号正常登录和角色、手机表单登录、私有编辑器、工作台搜索、后台RSS/场景设置、旧无效页脚字段移除、最终资产与canonical、原logo字节一致、RSS404及无pageerror，全部通过；[LAN报告](../tests/artifacts/v3/promoted-lan-smoke.json)。本轮58项浏览器业务与15项切换验证分开记录，不把同一行为重复计为新能力。

本环境仍是专用临时PG/媒体测试部署，长期使用须按正式文档迁移到持久卷。测试库允许注册，生产默认关闭；没有默认生产账号。未重新声称真实Docker容器、物理手机输入法、外部OAuth、公网发布、目标小服务器负载或异机容灾验收完成。

## 2026-10-28 V4：个人滚动首页与文档编辑器

本轮没有新增迁移或后端业务变化。使用真实 PostgreSQL 18.6 的备份克隆库验收，最终只替换正式服务的 Go 二进制，保留原数据库、媒体和私有配置，候选 QA 数据不会覆盖正式内容。变更及兼容边界见 [EXPERIENCE_V4.md](EXPERIENCE_V4.md)。

| 检查 | 结果 / 证据 |
| --- | --- |
| 文档组件隔离 | 37项通过；全部API为内存mock，不能当作数据库验证：[report.json](../tests/artifacts/v4-editor/report.json) |
| 文档真实链路 | 21项通过；打开/模式切换零PUT、metadata-only正文原串不变、真实自动/手动保存、图片上传、检查点/发布、匿名安全读取、恢复不改公开版本、撤回后匿名正文/媒体404：[report.json](../tests/artifacts/v4/document-integration/report.json) |
| 现有业务回归 | 25项通过；搜索/标签/封面/点赞/登录/注册/改密/RSS与场景配置恢复：[report.json](../tests/artifacts/v4/experience/report.json) |
| 原有保存与恢复 | 原断言复跑8+9项通过，errors为空：[保存](../tests/artifacts/v4/save-recovery-repro/frontend-save-qa.json)、[新稿](../tests/artifacts/v4/new-draft-recovery-repro/frontend-new-draft-qa.json) |
| 公共功能 | 27项通过；真实WebGL滚动状态、无空闲循环、像素预算、主题三选一/键盘/系统/外点、hover/focus快捷键、无横向溢出、减少动效/WebGL降级/无JS阅读与搜索：[report.json](../tests/artifacts/v4/public/report.json) |
| 实际截图 | 公共37张（1440×1000、390×900，浅/深色及滚动章节/主题/搜索状态/减少动效）；编辑器8张（文档/斜杠菜单，桌面/手机/浅深）：[公共](../tests/artifacts/v4/public/)、[编辑器](../tests/artifacts/v4/editor-review/) |
| 构建 | Vue类型检查、Vite生产构建、`go vet ./...`、CGO禁用Go构建通过。编辑器1.15MB/约379KB gzip，包含文档和源码引擎，只有进入编辑器时延迟加载；首页三维模块约6KB，公开入口约10KB。未通过调高警告阈值隐藏Vite大分块提示 |

首次并行验收中的两条恢复脚本发生30秒等待超时：已有稿重开提示，以及丢响应后的失败提示。保留原断言分别完整复跑通过；进一步真实探针观察到本机seq大于ack、源码切换零PUT、恢复提示存在且服务端内容未变。未确定首次超时原因，不能把复跑当成已定位并修复的竞态：[观测记录](../tests/artifacts/v4/recovery-repro-evidence.json)。实际文档测试早期DOM断言错误的失败尝试与清理记录也保留，最终21项真实结果单列。

公共界面由未参与公共实现的编辑器代理交叉复核，发现封面比例、文字压到三维物体、轮廓采样和手机结句断行四项；一次批量修正后按同路径确认。编辑器由主线程打开实际应用截图并核对真实链路。新建专职finish-reviewer/documenter受当前会话代理数量上限阻止，因此这里采用范围分离的交叉复核和主线程文档同步，不声称完成新的全站独立审计。[首轮意见](../tests/artifacts/v4/public/independent-review.md)。

静态检测各自只执行一次且范围不重叠：编辑器组件返回空发现；公共源码/模板检测无法解析Go模板中的动态CSS链接，产生默认黑色与统一16px字号两项，实际嵌入样式/截图不符合该默认推断，保留为静态工具限制：[检测](../tests/artifacts/v4/public/detector.json)、[边界](../tests/artifacts/v4/public/detector-boundary.json)。没有再扫描来追求零报告。

WebGL截图使用本机Chromium SwiftShader；同时实际测试故意禁用WebGL的静态降级。没有声称完成实体手机GPU帧率、浏览器软键盘/动态地址栏、真实OS中文输入法验收。GTA官网本机只捕获到加载标识，未用该截图证明其完整视差；参考均无美术资产复制。[官方来源记录](../tests/artifacts/v4/references/sources.json)。本轮没有重跑历史58项PG race测试或Docker运行，历史结果仍按原轮次保留。

正式18080更新后16项检查通过：正常手机owner登录、新首页/主题/搜索、文章顶部、文档默认/源码模式、许可访问与无溢出；内容写入数为0。[正式检查](../tests/artifacts/v4/promoted-lan-smoke.json)。切换前后22篇文章/22份草稿/32份修订/8次迁移数量一致，私有环境文件未变；候选18081停止，Go/PG服务保持active。[切换证据](../tests/artifacts/v4/promotion.json)。公共四项修正的最终交叉复核为 `ship`，仅覆盖该清单；SwiftShader原尺寸图仍有轻微轮廓/色阶瑕疵，不称为达到Apple参考的渲染精度。[四项结论](../tests/artifacts/v4/public/final-four-item-review.json)。编辑器主线程复核记录为 [review.md](../tests/artifacts/v4/editor-review/review.md)。

## 2026-10-28 V5：旧站介绍、渐变封面与时间展示

读取旧首页源码恢复逃塔、七个身份标签及技术笔记/日常定位；补充文案没有添加新的履历。介绍字段由站长配置，渐变只用于文章封面，不改变全站语义色。数据结构、时间来源、旧导入未知值和发布事务见 [EXPERIENCE_V5.md](EXPERIENCE_V5.md)。

| 检查 | 结果 / 证据 |
| --- | --- |
| 后端与数据库 | 真实PostgreSQL 18.6、`go test -race -json ./... -count=1`：74个测试通过、0失败、0个实际测试跳过；两条包级skip仅为无测试文件。含日期SQL约束/快照/创建时间不可变、旧导入不泄漏较新私有更新时间、站点配置权限/省略字段保留；[最终日志](../tests/artifacts/v5/go-test.jsonl) |
| V5真实业务 | 20项通过：正常配置表单、原介绍、旧客户端兼容、渐变SVG、三个独立日期、公开时间与JSON-LD、私有保存/检查点隔离、全部关闭/单项展示、恢复/重新发布/撤回；[报告](../tests/artifacts/v5/experience.json) |
| 文档与媒体 | 原V4真实链路保留断言复跑21项通过：原Markdown、文档/源码切换、媒体、发布/恢复/撤回；[报告](../tests/artifacts/v5/document/report.json) |
| 保存恢复 | 离线与慢响应8项、新稿/丢响应9项通过，均加入日期选择持久化断言，errors为空；[已有稿](../tests/artifacts/v5/recovery/frontend-save-qa.json)、[新稿](../tests/artifacts/v5/recovery/frontend-new-draft-qa.json) |
| 实际视觉 | 最终28图：1440×1000和390×900，浅/深，首页/介绍/文章/时间/编辑选项/设置/About；无横向溢出或pageerror。[捕获报告](../tests/artifacts/v5/visual/report.json)。另有真实Go SVG经Chromium渲染的[12种封面矩阵](../tests/artifacts/v5/covers/gradient-matrix.png) |
| 独立核对 | 全新空上下文reviewer逐一查看全部28原图与12种封面，结论accept，无阻断项；另一个新代理核对设计文档、实际源码与指定原图，修正文档示例，结论accept。[视觉审阅](../tests/artifacts/v5/visual/independent-review.md)、[文档核对](../tests/artifacts/v5/visual/documentation-review.md) |
| 构建与检查 | Vue类型检查/Vite生产构建、`go vet ./...`、CGO禁用release构建通过；正式三个JS入口响应hash与本地资产一致。编辑器仍延迟加载约1.15MB/379KB gzip，保留Vite大分块提示；[构建清单](../tests/artifacts/build-manifest.json) |
| 正式LAN | 更新原18080后24项通过：真实手机视口正常owner登录、介绍/品牌、旧文章渐变及三项时间、自定义编辑器和设置入口、匿名私有数据401、无脚本异常；文章写入0。[报告](../tests/artifacts/v5/promoted-lan-smoke.json) |

后端封面检测与UI检测各执行一次、范围互不重叠。封面检测返回空发现；UI模板检测无法解析Go动态CSS链接，沿用默认黑色与16px推断，实际样式和原图不支持这些推断。[原始检测](../tests/artifacts/v5/ui-detect.json)保留，没有复扫来消除报告。根线程先批量查看，再一次修正中文词组断行、文章导航覆盖封面、时间字号和选项标题，最终按相同28路径重捕；之后仅同步文档，无第三轮自发精修。

正式升级前再次生成真实完整备份 `/tmp/painter-next-before-v5-final-20261028/`，另存旧程序与0600私有配置。停旧写入后在原 `painter_next_v3_20261028` 库执行0009，再激活新二进制；瞬态服务停止后被systemd移除，已按同一私有配置重新创建并确认ready。22篇文章、22份草稿、32份修订、5个公开指针、16个媒体对象数量不变；旧文章、草稿和修订原字段hash相同，私有环境文件不变，迁移数8→9。候选QA库没有覆盖原库，候选18081已停止。[切换记录](../tests/artifacts/v5/promotion.json)。原介绍通过正常站长权限、CSRF/Origin和If-Match接口写入；[配置结果](../tests/artifacts/v5/promoted-intro.json)。

本轮独立视觉及文档角色实际可用，V4的代理额度限制不适用于V5结论。仍仅覆盖本机Linux Chromium和指定视口，没有新增实体设备/真实输入法、外部OAuth、真实Docker容器、目标小服务器负载或异机容灾证明。预览仍使用私有临时目录，持久部署边界见 [LAN_ACCESS.md](LAN_ACCESS.md)。汇总见 [summary.json](../tests/artifacts/v5/summary.json)。

## 2026-10-28 V6：标签选择与相关推荐

无数据库迁移、运行依赖或新服务。编辑标签使用独立可搜索多选组件，移除选择继续走原草稿合同；推荐是一次只读查询和SSR开放列表。完整行为及公开快照边界见 [EXPERIENCE_V6.md](EXPERIENCE_V6.md)。

| 检查 | 结果 / 证据 |
| --- | --- |
| 推荐数据 | 2个新增真实PG集成测试、race通过：共享规范标签、ID排序、最近补齐、草稿/撤回/回收过滤、源文章撤回、标签别名、私有标题/标签隔离、重新发布和SSR；[日志](../tests/artifacts/v6/go-related.jsonl) |
| 实际业务 | 23项通过：标签查询零写入、全角搜索、键盘/点选/移除/重载、metadata-only原Markdown不变、归档选择显式保留、真实21标签夹具的20上限/移除后添加、公开推荐三篇与时间选项、禁JS链接导航。空标签呈现使用隔离响应并明确标记；[报告](../tests/artifacts/v6/experience.json) |
| 保存恢复 | 原断言保留，已有稿8项、新稿9项通过，均无pageerror；新稿选择与恢复断言已适配新控件；[保存](../tests/artifacts/v6/recovery/frontend-save-qa.json)、[新稿](../tests/artifacts/v6/recovery/frontend-new-draft-qa.json) |
| 视觉 | 1440×1000、900×1000、390×900，浅/深，标签已选/展开及推荐区，共18图，零横向溢出、零脚本异常；[报告](../tests/artifacts/v6/visual/report.json)。根线程核对标签原图及六组联图；未参与推荐实现的标签代理复核全部六张推荐原图，结论accept；[独立范围复核](../tests/artifacts/v6/visual/related-review.md)、[主线程核对](../tests/artifacts/v6/visual/review.md) |
| 构建 | Vue类型检查、Vite生产构建、`go vet ./...`和CGO禁用release构建通过。编辑器继续延迟加载，约1.154MB/382KB gzip，保留分块警告；[清单](../tests/artifacts/build-manifest.json) |
| 正式LAN | 16项通过：现存文章三篇推荐、链接可达/无JS、封面解码、手机正常owner登录、标签搜索/关闭且不创建文章、旧时间开关仍在、无溢出及脚本异常。文章写入0；[报告](../tests/artifacts/v6/promoted-lan-smoke.json) |

两条早期失败属于验收脚本前提：Playwright默认等待aria-disabled选项可用，改用真实鼠标坐标验证其忽略点击；新稿恢复后折叠的metadata使标签可见性断言为false，改为先展开再检查。原始数据、完整服务器快照及其余断言保留，修正后完整复跑通过；[禁用选项记录](../tests/artifacts/v6/experience-before-disabled-click-fix.json)、[折叠记录](../tests/artifacts/v6/recovery/frontend-new-draft-before-disclosure-fix.json)。候选首次启动因OAuth回调仍指向18080被配置校验拒绝，已只对候选同步18081回调，正式配置不变。

UI静态检测仅执行一次，范围为新标签组件、其集成、文章样式与模板。Go模板动态CSS链接仍产生默认黑色/16px两条推断，原图及实际样式不支持该推断；[原始结果](../tests/artifacts/v6/ui-detect.json)保留，无复扫。本轮首批18图即最终图，没有再进行自发精修。设计文档只同步本轮组件行为和扩展说明，未改色板/字体。

升级前完整备份保存在仓库外 `/tmp/painter-next-before-v6-20261028/`。原 `painter_next_v3_20261028` 的22篇文章、22份草稿、32份修订、5个公开指针、16个媒体对象及其内容hash保持一致，私有配置未变，schema仍为9；只原子替换Go程序并重启。候选QA撤回且18081停止，Go/PG active；[切换](../tests/artifacts/v6/promotion.json)、[候选清理](../tests/artifacts/v6/qa-cleanup.json)。没有重跑V5全部74项测试，历史结果保持原轮次；外部设备、真实Docker、目标机负载与异机容灾边界不变。

## 2026-10-28 V7：网页安装、后台 GitHub 与鉴权评估

新增0010安装封闭和0011 provider配置迁移。空库、旧站克隆库与既有正式LAN库分别验收；部署运行结构仍为Go与PostgreSQL18，无SQLite。数据时序与API见 [安装](INSTALLATION_V7.md)、[GitHub配置](GITHUB_LOGIN_V7.md)，当前鉴权缺口及后续OIDC MFA合同见 [评估](AUTH_REVIEW_V7.md)。

| 检查 | 结果 / 证据 |
| --- | --- |
| 全部后端测试 | 真实PG18.6、`go test -race -json ./... -count=1`，94个通过的测试条目（含子测试，覆盖79个当前顶层app测试），0失败、0实际跳过；含错误安装码/CSRF/Origin、密码与严格解码、原子提交、审计失败回滚、并发唯一完成、已有身份封闭、GitHub CAS/secret/配置竞态/会话替换。[原始记录](../tests/artifacts/v7/go-test.jsonl) |
| 聚焦独立安全复查 | 认证代理先检查安装源码，再补充迁移后local reader、OAuth reader与停用owner夹具；16顶层+5子场景race通过。该代理实现GitHub部分，因此不将其自己的实现称为独立完整审计。[聚焦记录](../tests/artifacts/v7/github-auth-race.txt)、[范围](AUTH_REVIEW_V7.md) |
| 部署脚本 | 50项隔离mock检查通过，含新码独立随机、仅显式读取、不自动启动/失败传播、已存配置保留、发行包精确白名单及更新保护。已同步三份V7文档的包内交付。[最终日志](../tests/artifacts/v7/deploy-tests-final.log)；没有运行真实Docker容器 |
| 真实浏览器业务 | 27项通过：三步安装真实创建owner/站点、错误码拒绝、确认不一致、无自动登录、关闭后重载与正常登录、默认关闭注册；旧站封闭、GitHub表单保存/读取脱敏/启停保留密钥/官方PKCE跳转/CAS冲突/503重试。[报告](../tests/artifacts/v7/experience.json)。使用隔离假provider，未跟随外部GitHub |
| 安装码CLI | 空库阶段显式读取与私有配置一致；安装成功后退出码非零且不含安装码。[封闭检查](../tests/artifacts/v7/closed-token-cli.json)。实际码、口令与状态文件均在仓库外0600保存 |
| 视觉与可用性 | 三宽1440/900/390×浅深×环境/站点/管理员/GitHub共24图，另完成态1图，零横向溢出、零pageerror。[矩阵](../tests/artifacts/v7/visual/report.json)。认证代理独立核对其未实现的19张向导原图，accept；GitHub由主线程核对，范围分别记录：[安装审阅](../tests/artifacts/v7/visual/install-review.md)、[主线程](../tests/artifacts/v7/visual/review.md) |
| 构建 | Vue类型检查、Vite生产构建、`go vet ./...`、CGO禁用release构建通过；setup独立入口，四个正式JS（含延迟编辑器）响应hash与本地一致。[清单](../tests/artifacts/build-manifest.json)。编辑器仍约1.154MB/382KB gzip并延迟加载，保留Vite大块提示 |
| 正式LAN回归 | 原业务16项+本轮15项通过：手机正常owner登录、现存推荐/封面/标签编辑、安装永久封闭、匿名配置401、secret空/掩码、原环境源与准确18080回调。文章/业务写入均0。[业务](../tests/artifacts/v7/promoted-lan-smoke.json)、[认证](../tests/artifacts/v7/promoted-auth-smoke.json) |

首轮浏览器定位“一句介绍”因label包含提示而超时；随后批量将说明移出label并显式关联、补步骤标题焦点，保留原业务断言完整重跑27项通过。同路径重捕24图并增加完成态，之后未追加UI精修。[首轮诊断](../tests/artifacts/v7/experience-before-label-fix.json)。部署首轮四个失败为旧输出文案断言，指导文案与新setup路径同步后50项通过；历史日志保留，不将其列为容器运行故障。UI检测仅一次，Go动态CSS仍产生默认黑色与16px两条工具推断；实际样式/截图不支持这些推断，保留[原报告](../tests/artifacts/v7/ui-detect.json)，未复扫。

切换前真实数据库/媒体备份及旧程序/0600配置保存在仓库外 `/tmp/painter-next-before-v7-final-20261028/`。停止旧写入后在原 `painter_next_v3_20261028` 执行0010/0011，再原子替换程序并按原环境重建瞬态用户服务。迁移数9→11；22文章、22草稿、32修订、5公开指针、16媒体，以及用户、本地/OAuth凭据、站点设置、标签、公开投影和既存session在切换前后的数量/hash一致。正式环境文件未变，`github_auth_settings`仍0行，没有导入克隆库假凭据；安装状态已封闭。[备份证据](../tests/artifacts/v7/pre-upgrade-backup.json)、[切换](../tests/artifacts/v7/promotion.json)。正常登录后的session更新属于真实登录结果。候选18081/18082随后停止，原18080和PG active；[清理](../tests/artifacts/v7/qa-cleanup.json)。

MFA、WebAuthn/TOTP、恢复码、近期强认证、密码blocklist和统一IdP均未实现。GitHub不等于本次已完成2FA，没有在线账号绑定/解绑。本次不声明真实GitHub App、实体手机/屏幕阅读器、真实Docker、1GiB目标机负载、旧MySQL切换或异机灾备已验证。局域网仍为开发HTTP与私有临时目录，持久部署见 [LAN_ACCESS.md](LAN_ACCESS.md)。汇总与秘密边界见 [summary.json](../tests/artifacts/v7/summary.json)。

## 2026-10-29 V8：独立工作台与片语

新0012表与审计词汇迁移，原迁移不变。后台使用独立侧栏/任务画布，自定义选择与确认控件；公开片语直接读取同一事务表，无异步发布投影。完整API/时序与边界见 [EXPERIENCE_V8.md](EXPERIENCE_V8.md)。

| 检查 | 结果 / 证据 |
| --- | --- |
| 后端全量 | 真实PG18.6，`go test -race -json ./... -count=1`，101条测试（含子测试）通过，0失败/0实际跳过；`go vet ./...`通过。[记录](../tests/artifacts/v8/go-test.jsonl) |
| 片语定向 | 7组真实PG race通过：owner/CSRF/Origin、严格JSON/Unicode/size、公开隔离、创建幂等与并发CAS、数值游标/大ID、SSR撤回删除/XSS转义、后台页面角色。[记录](../tests/artifacts/v8/phrases-race.txt)。数值排序曾发现id::text别名排序隐患，已显式ORDER BY phrases.id并由分页测试验证 |
| 浏览器新流程 | 39项通过：独立后台、设置键盘tabs/deeplink、自定义选择键盘/外点、封面持久化不改原文、片语草稿/发布/撤回/删除、冲突保留/503重载失败保留、未保存离开保护、手机modal内主题、无JS公开读取、复制反馈/实际下一条；0pageerror/0横向溢出。[报告](../tests/artifacts/v8/experience.json) |
| 原编辑器 | 原V4数据断言复跑21项通过，仅把历史恢复确认适配为项目dialog：原Markdown、文档/源码、保存/图片/发布/修订恢复/撤回等。[报告](../tests/artifacts/v8/document/report.json)、[适配脚本](../tests/browser/v8-document-integration.cjs) |
| 实际视觉 | 1440/900/390宽、浅深，文章/编辑选择/片语管理/公开各6图，加设置/成员/词语/空态各2及手机抽屉主题2，共34张viewport图；编辑器与词语交互态可滚动。[矩阵](../tests/artifacts/v8/visual/report.json)。空态为真实SSR cursor=1无结果，词句明确标记候选验收样例，未伪装个人内容 |
| 构建 | Vue类型检查/Vite/CGO禁用release通过；三入口与延迟编辑器的正式响应hash匹配。[当前构建](../tests/artifacts/build-manifest.json)。编辑器仍约1.154MB/382KB gzip并延迟加载，保留大块提示，没有新增运行依赖 |
| 正式LAN | 原业务16项+V8工作台19项通过：普通手机owner登录、推荐/标签/时间/原文档可达、独立桌面/28px品牌、正确GitHub深链接和回调/无密钥回显、片语空库/匿名API拒绝、手机抽屉、首页入口。文章与业务写入0。[业务](../tests/artifacts/v8/promoted-lan-smoke.json)、[工作台](../tests/artifacts/v8/promoted-workbench-smoke.json) |

首个流程从尚未flush的编辑器直接跳转，被既有beforeunload正确拦截；脚本改为明确保存后跳转，保留[首次记录](../tests/artifacts/v8/experience-before-editor-flush.json)。第二次发现真实表单dirty计算缓存问题，initial改为ref后修复：[原记录](../tests/artifacts/v8/experience-before-dirty-cache-fix.json)。安全代理再指出未保存导航与失败reload覆盖输入，已补beforeunload和成功加载后才fill；39项包含这些断言，先前38项记录单独保留。[修复前记录](../tests/artifacts/v8/experience-before-list-retry-fix.json)。

独立空上下文视觉reviewer逐张看完34张有效图，首轮disposition fix只要求桌面品牌字级和设计文档同步；一次批量明确品牌选择器恢复1.75rem，原同名34图重捕。只做两轮截图，无额外自发精修。初审与最终评分见 [独立审阅](../tests/artifacts/v8/visual/independent-review.md)、[最终评分](../tests/artifacts/v8/visual/final-verdict.md)；独立documenter同步工作台布局与组件说明，保留字体/色板，见 [文档核对](../tests/artifacts/v8/visual/documentation-review.md)。静态检测只执行一次，三条均为Go模板动态CSS/独立片段缺全局样式导致的默认黑与16px推断，保留[原报告](../tests/artifacts/v8/ui-detect.json)，未复扫。

升级前再次完成真实PG/媒体备份，并存旧程序与0600配置：`/tmp/painter-next-before-v8-final-20261029/`。停旧写入、原库0012、原子激活新程序，schema11→12；23文章、23草稿、32修订、5公开指针、16媒体及用户/本地OAuth凭据/站点配置/安装封闭状态/既存session，在切换前后hash一致；原环境文件未变，正式phrases初始0条，未导入候选QA。[切换](../tests/artifacts/v8/promotion.json)。正式普通登录产生的新session属于预期登录行为。候选片语样例随后通过正常API撤回，18081停止，原18080/PG active：[撤回](../tests/artifacts/v8/qa-withdrawal.json)、[清理](../tests/artifacts/v8/qa-cleanup.json)。

本轮没有改部署脚本或重跑V7的50项mock部署测试，没有真实Docker、实体手机/屏幕阅读器/200%缩放、外部OAuth或1GiB目标机负载证明。减少动态效果以源码降级与浏览器媒体偏好检查为据，不声称全平台动画性能已验收。当前HTTP局域网/私有临时目录边界仍见 [LAN_ACCESS.md](LAN_ACCESS.md)。汇总与已知秘密扫描见 [summary.json](../tests/artifacts/v8/summary.json)。

## 2026-10-29 V9：两层导航与栏目归属

本轮仅改工作台前端信息架构，没有迁移、后端鉴权或业务写入变化。

| 检查 | 结果 / 证据 |
| --- | --- |
| 导航业务 | 三个真实owner/author/reader账号正常登录，38项通过：四分区/空组过滤、键盘展开、账号偏好、个人资源、作者分析范围、文章编辑及01/+1父项、粉丝别名、设置深链接和前后退、本地未保存表单、抽屉焦点与当前项定位。0业务写入/0pageerror。[报告](../tests/artifacts/v9/navigation.json) |
| 视觉 | 1440×1000、900×1000、390×900，浅深主题、文章/设置/抽屉/角色，共20张最终viewport图，零横向溢出/零脚本异常。[矩阵](../tests/artifacts/v9/visual/report.json) |
| 独立完成与文档审查 | 未参与实现的reviewer查看初批全部20图及最终代表图，核对源码/38项交互证据，结论ship；独立合并PRODUCT/DESIGN/sidecar导航规则，保留视觉体系。[完成审阅](../tests/artifacts/v9/finish-review.md)、[文档核对](../tests/artifacts/v9/documentation-review.md) |
| 构建 | Vue类型检查、Vite生产与CGO禁用Go release通过；正式三入口及延迟编辑器响应hash均与本地一致。[构建日志](../tests/artifacts/v9/build.log)、[当前清单](../tests/artifacts/build-manifest.json)。编辑器仍约1.154MB/382KB gzip，保留原大块提示 |
| 正式LAN | 原业务16项、导航15项通过：普通手机登录、既有文章/推荐/标签/时间、四分区/当前项/编辑父项、设置hash/回退、18080回调、全展开定位/手机抽屉/焦点；内容/配置写入0。[业务](../tests/artifacts/v9/promoted-lan-smoke.json)、[导航](../tests/artifacts/v9/promoted-navigation-smoke.json) |

初批验收发现全部分区展开后，个人会话页当前项会落在侧栏可见区域外；新增仅滚动侧栏的自动定位与抽屉打开后定位，并补桌面/手机断言。修正后同路径第二批20图为最终证据，没有再追加自主精修。[修正前几何记录](../tests/artifacts/v9/active-leaf-before-fix.json)。静态检测仅一次、范围为改动UI文件、无发现；没有在定位修正后追加检测，也不以检测替代实际查看。[原报告](../tests/artifacts/v9/ui-detect.json)。

更新前完成完整备份，位于仓库外 `/tmp/painter-next-before-v9-20261029/`。原库保留schema12，18个内容/身份/配置/既存会话集合在切换前后count/hash一致，环境未变；只原子替换程序并重启，未导入候选QA。[切换](../tests/artifacts/v9/promotion.json)。正式登录产生新session属于正常行为，候选随后停止、18081关闭、正式Go/PG active：[清理](../tests/artifacts/v9/qa-cleanup.json)。

V8的101项PG race、V7部署mock为历史记录，本轮未重跑。没有真实Docker、外部GitHub授权、实体手机/屏幕阅读器或目标机负载证明；本机Chromium尺寸检查不等同完整兼容认证。完整范围及已知秘密扫描见[汇总](../tests/artifacts/v9/summary.json)。

## 2026-10-29 V10：编辑页比例与布局

本轮为已有视觉体系内的编辑页布局优化，没有后端迁移或运行依赖变化。

| 检查 | 结果 / 证据 |
| --- | --- |
| 布局与交互 | 1920/1440/900/390宽、173项通过：标题/正文起点差0px、1360px属性栏、单列披露、源码编辑/预览/分屏、模式/断点切换、键盘与44×44px命令、属性状态和服务器数据指纹不变。0真实/尝试业务写入、0pageerror。[报告](../tests/artifacts/v10/layout.json) |
| 文档实际业务 | 原21项断言通过：原Markdown不改写、文档自动/手动保存、真实图片、检查点、发布、恢复和撤回，0HTTP/脚本异常。QA已撤回。[报告](../tests/artifacts/v10/document/report.json) |
| 离线与恢复 | 已有稿8项、新稿9项通过：离线摘要/标签/封面/时间恢复、服务器原样、慢响应串行、校验失败修正、导出、丢首个响应找回同一client ID、不重复创建。[保存](../tests/artifacts/v10/recovery/frontend-save-qa.json)、[新稿](../tests/artifacts/v10/recovery/frontend-new-draft-qa.json) |
| 视觉与独立审阅 | 四宽浅深×文档/属性或菜单/源码，共24张最终viewport图，零横向溢出/脚本异常。独立reviewer逐张查看全部前后图，结论ship，并独立同步设计文档。[矩阵](../tests/artifacts/v10/visual/report.json)、[完成审阅](../tests/artifacts/v10/finish-review.md)、[文档核对](../tests/artifacts/v10/documentation-review.md) |
| 构建 | Vue类型检查/Vite与CGO禁用Go release通过，正式三入口和延迟编辑器响应hash匹配。[日志](../tests/artifacts/v10/build.log)、[清单](../tests/artifacts/build-manifest.json) |

第一轮布局检查发现11个命令尺寸未达到44px，源于组件scoped CSS优先级；修正尺寸和选择器后173项全通过，同路径第二批24图为最终证据，没有再做自主精修。[修正前报告](../tests/artifacts/v10/layout-before-target-fix.json)。静态检测一次、无发现，不追加复扫：[原报告](../tests/artifacts/v10/ui-detect.json)。

恢复脚本首次执行暴露历史原生select调用与当前combobox不匹配；V10另存适配副本，未改产品来迎合测试。第二个适配问题是折叠属性的按钮innerText不可作为原生inputValue替代，改为读取实际所选label节点textContent，保留原精确值与持久化断言。[适配前记录](../tests/artifacts/v10/recovery/frontend-save-before-adapter-value-fix.json)。

正式更新前完整备份在仓库外 `/tmp/painter-next-before-v10-20261029/`。只原子替换程序、重启，原库18个集合count/hash与环境保持一致，schema12。[切换](../tests/artifacts/v10/promotion.json)。候选29/30号QA均无公开指针，保留私有证据，18081已停止；原Go/PG active。[清理](../tests/artifacts/v10/qa-cleanup.json)。

后端101项race与部署mock仍为历史记录，本轮未重跑；Chromium尺寸模拟不代表实体手机/屏幕阅读器，未执行真实Docker、外部OAuth或目标机负载。当前临时部署、通用后台表单恢复和账号生命周期仍是已有独立待办，本轮没有声称解决。正式实例验证与完整汇总见[summary.json](../tests/artifacts/v10/summary.json)。

V10正式实例另通过既有文章/手机登录/标签/时间16项与同一只读布局173项，合计189项，内容/配置写入0；正式稿件的5次内容/元数据/版本指纹比较不变。[既有业务](../tests/artifacts/v10/promoted-lan-smoke.json)、[正式布局](../tests/artifacts/v10/formal/layout.json)。

## 2026-11-01 V11：编辑保护与资源状态

七项修复及数据时序见 [实现说明](EXPERIENCE_V11.md)。无迁移、依赖或部署拓扑变化，schema 保持 12。

| 验证 | 实际结果 / 证据 |
| --- | --- |
| 编辑器异常路径 | 43 项通过：标签 / 草稿加载失败锁定、重试、异常 200 响应、双版本写入、合并阶段所有自动触发暂停、显式保存、刷新恢复及内容相同恢复点、服务器读取失败、两种正文模式历史恢复锁与重试。0 真实业务写入、0脚本异常。[报告](../tests/artifacts/v11/editor-safety/report.json) |
| 搜索与资源状态 | 47 项通过：原生右键 / 中键 / 修饰键、键盘 ARIA、触摸滑动，桌面 / 手机媒体和评论状态，超过首批 200 条的精确片语重载、503 / 404 / 格式异常保留输入及最新 CAS。[报告](../tests/artifacts/v11/state-paths/report.json) |
| 布局与实际业务 | V10 只读布局 173 项及真实文档生命周期 21 项通过。后者仅在克隆库执行，包含保存 / 上传 / 检查点 / 发布 / 恢复 / 撤回；QA 已撤回。[布局](../tests/artifacts/v11/layout/layout.json)、[文档](../tests/artifacts/v11/document-integration/report.json) |
| 恢复兼容性 | 已有稿 8 项、新稿 9 项通过，覆盖离线元数据、慢响应期间继续编辑、校验纠正、导出、丢首次创建响应与同一 client ID 找回。[已有稿](../tests/artifacts/v11/recovery/frontend-save-qa.json)、[新稿](../tests/artifacts/v11/recovery/frontend-new-draft-qa.json) |
| 后端与构建 | `PAINTER_TEST_DATABASE_URL` 指向可创建临时库的 PostgreSQL18 管理连接，`go test ./... -count=1` 通过；每项数据库集成测试创建、迁移并清理独立临时库。Vue 类型检查 / Vite、CGO 禁用 Go release 通过。[Go](../tests/artifacts/v11/go-test.log)、[构建](../tests/artifacts/v11/build.log) |
| 视觉与独立审阅 | 一批 15 张异常状态图，涵盖桌面 / 手机与浅深主题；另检查 2 张真实 QA 生命周期图。既有图形和布局保留，无新产品位图。独立结论 ship，窄范围同步 PRODUCT / DESIGN；静态检测一次无发现。[图源](../tests/artifacts/v11/visual/provenance.json)、[审阅](../tests/artifacts/v11/finish-review.md)、[文档核对](../tests/artifacts/v11/documentation-review.md) |

第一轮专项回归发现冲突对照为 Vue 响应式对象、不能直接写入 IndexedDB，修为普通 JSON 快照后，手动合并与刷新找回均通过；失败记录保存在 `diagnostics/before-recovery-serialization-fix.json`。独立源代码审阅发现格式异常的成功响应仍可能解锁草稿，以及正文相同的手动合并恢复点会被过滤，均补充修复和专项断言。截图对应 41 / 46 项捕获批次，最终无视觉变化的补充用例使报告为 43 / 47，不混淆两者。已有稿恢复第一次并发执行未找到恢复提示，随后诊断执行与未修改原脚本的最终单独执行均 8 项通过；失败不计入通过结果。

更新前完整数据库、媒体、旧程序及私有配置位于仓库外 `/tmp/painter-next-before-v11-20261101/`。更新仅原子替换程序并重启；18 个内容 / 身份 / 配置 / 会话集合的 count / hash 以及私有环境保持一致，不复制候选 QA 库。[切换记录](../tests/artifacts/v11/promotion.json)。正式公开、安装、工作台入口与延迟编辑器四个资源响应 hash 均与构建产物一致。[构建清单](../tests/artifacts/build-manifest.json)。

正式实例再次验证原业务 16 项（包含正常手机登录）、编辑保护 43 项、资源状态 47 项，均无内容 / 配置写入；后两者仍使用模拟异常响应。[原业务](../tests/artifacts/v11/formal/promoted-lan-smoke.json)、[编辑保护](../tests/artifacts/v11/formal/editor-safety/report.json)、[资源状态](../tests/artifacts/v11/formal/state-paths/report.json)。克隆 QA 31 / 32 号保留为私有草稿，候选服务已停止；原应用和 PostgreSQL 服务继续 active。

证据使用 Linux Chromium 尺寸和触摸模拟，不声称真实手机 / 屏幕阅读器 / 全浏览器兼容认证；未执行真实 Docker、GitHub 外部回调或服务器负载测试。临时部署持久化、通用后台表单与账号生命周期仍为独立待办。完整结果见 [summary.json](../tests/artifacts/v11/summary.json)。

## 2026-11-02 V12：编辑页焦点与留白

本轮只调整编辑页 CSS，具体原因与尺寸见 [实现说明](EXPERIENCE_V12.md)。移除正文全高蓝色焦点框，以工具栏分隔线提示正文焦点；标题与模式用局部下划线，命令键盘焦点仍保留。桌面属性栏采用 size containment，修正跨行尺寸分配导致的标题后约54～132px额外空隙；宽屏最终多余间距为0。未修改保存、冲突、恢复、API或数据库逻辑，schema仍12。

| 检查 | 结果 / 证据 |
| --- | --- |
| 焦点、选区与间距 | 四种尺寸、浅深主题，91项通过：空稿聚焦、文字选区、标题下划线、命令键盘焦点、强制颜色模式、标题/正文起点、无多余网格间距、44px目标及无横向溢出。[报告](../tests/artifacts/v12/visual/report.json) |
| 原布局交互 | 173项通过，包括属性披露、断点与模式切换、源码三视图、自定义控件、数据指纹不变；0真实业务写入。[报告](../tests/artifacts/v12/layout/layout.json) |
| 编辑安全 | V11的43项初始化、手动合并/浏览器恢复、恢复锁与异常读取断言通过；0真实业务写入。[报告](../tests/artifacts/v12/editor-safety/report.json) |
| 视觉与独立审阅 | 3张实际V11基线、16张最终空稿/选区矩阵，独立审阅逐张查看后结论ship；PRODUCT、DESIGN与editorLayout sidecar窄范围同步。[图源](../tests/artifacts/v12/visual/provenance.json)、[审阅](../tests/artifacts/v12/finish-review.md)、[文档](../tests/artifacts/v12/documentation-review.md) |
| 构建 / 检测 | Vue类型检查/Vite、CGO禁用Go release通过；针对两个修改文件的一次静态检测无发现。[构建](../tests/artifacts/v12/build.log)、[检测](../tests/artifacts/v12/ui-detect.json) |

几何检查首先发现只固定首行 intrinsic sizing 仍会受跨行属性尺寸影响，最终通过弹性第二行与属性 size containment 一起消除首尾空白。首批捕获生成1张图后，标题焦点断言采到了已有180ms边框过渡的中间颜色；脚本改为等待实际最终颜色，再用第二批覆盖生成16张最终矩阵。该调整没有修改产品来匹配测试，也没有增加视觉精修轮次。

新项目整体未被父仓库跟踪，缺少完整V11源码基线，不能声称git源码差异独立证明了CSS范围；独立审阅记录该限制，使用限定编辑操作、当前源码与回归证据判断。后端未改，本轮未重跑数据库业务测试。尺寸、触摸与强制颜色验证来自Chromium，不代表实体设备、屏幕阅读器或所有浏览器认证。

正式切换前完整数据库、媒体、V11程序及私有配置位于仓库外 `/tmp/painter-next-before-v12-20261102/`。只原子替换程序并重启，原库18个内容/身份/配置/会话集合的count/hash和私有环境完全一致，未复制候选库。[切换](../tests/artifacts/v12/promotion.json)。三入口与延迟编辑器四项资源hash验证一致。[清单](../tests/artifacts/build-manifest.json)。

正式实例再次通过91项焦点/间距检查及16项既有业务/正常手机登录检查，内容与配置写入为0；未再捕获截图。[焦点与间距](../tests/artifacts/v12/formal/visual/report.json)、[既有业务](../tests/artifacts/v12/formal/promoted-lan-smoke.json)。候选服务已停止，原应用与PostgreSQL继续active，完整记录见[汇总](../tests/artifacts/v12/summary.json)。

## 2026-11-02 V13：阅读页与滚动导航

文章改为标题、成组摘要 / 标签、与正文同宽封面及正文；作者独立右置，三种时间收进原生信息控件，继续遵循已保存的每篇日期开关。讨论区边界、正文短分隔符、浅底相关推荐与无横线条目分别表达不同关系。非首页顶部栏下滚隐藏、上滚出现，并处理焦点 / 菜单 / 弹窗、减少动画、回弹及手机编辑操作避让。[实现说明](EXPERIENCE_V13.md)

| 验证 | 结果 / 证据 |
| --- | --- |
| 阅读页 | 175项通过：1440/900/390/320px浅深主题、封面与正文同边、作者位置、信息展开及模态Escape优先级、无JS、真实推荐 / 作者链接。1024px可选内容缺省与Markdown短线采用明确标识的DOM呈现样本。[报告](../tests/artifacts/v13/reading/report.json) |
| 滚动导航 | 104项通过：公共页面（含安装 / 注册）、手机工作台、抖动 / 回弹、焦点、菜单 / 搜索 / 抽屉、恢复及首页例外。[报告](../tests/artifacts/v13/header/report.json) |
| 编辑页回归 | 91项既有焦点、选区、间距、目标尺寸及无溢出检查通过。[报告](../tests/artifacts/v13/editor-regression/report.json) |
| 后端 | 真实PG18.6一次性测试库，90个顶层测试、14个子测试全部通过，0失败 / 0跳过；新增覆盖8种日期展示组合的SSR。此次没有开启race。[汇总](../tests/artifacts/v13/go-test-summary.json)、[原始记录](../tests/artifacts/v13/go-test.jsonl) |
| 构建 / 静态 | Vue类型检查、Vite、CGO禁用Go release、go vet及两份新浏览器脚本语法检查通过。一次detector扫描对原始Go模板报告默认黑色 / 16px层级，按计算值和实际主题渲染判定为误报，无新忽略项。[构建](../tests/artifacts/v13/build.log)、[检测](../tests/artifacts/v13/detector.json) |
| 视觉与独立审阅 | 一批12张最终图逐张审阅，结论ship；窄同步PRODUCT、DESIGN及sidecar。用户参考为真实post2，最终矩阵为真实post1，不宣称同内容像素对比。[图源](../tests/artifacts/v13/provenance.json)、[审阅](../tests/artifacts/v13/finish-review.md)、[文档](../tests/artifacts/v13/documentation-review.md) |

V13保存了真实源码前副本并生成有限范围差异；新项目仍未被父仓库跟踪，不伪造VCS基线。[范围](../tests/artifacts/v13/source-scope.json)、[差异](../tests/artifacts/v13/source.diff)。无API、业务写入语义、依赖或schema变化；浏览器检查没有真实内容 / 配置写入。测试中的只读预览POST不属于持久化修改。

首次通过模拟HTTP响应移除可选信息时，Chromium的局域网地址空间策略阻止该模拟响应引用的CSS；改为真实导航后移除可选DOM，未放宽产品安全策略。独立审阅发现的宽度与搜索Escape冲突已在最终证据前修复。

完整数据库、媒体、V12程序及私有环境已备份至仓库外 `/tmp/painter-next-before-v13-20261102/`。仅原子替换Go程序并重启，schema仍12，原库18个内容 / 身份 / 配置 / 会话集合的count/hash及私有环境完全一致，未复制候选库。[切换](../tests/artifacts/v13/promotion.json)。实际送达的公共、工作台、安装与编辑器四项入口资源hash匹配。[构建清单](../tests/artifacts/build-manifest.json)

正式18080再次通过175项阅读、104项导航、16项既有内容与正常手机登录检查；没有再捕获截图，没有内容 / 配置写入。[阅读](../tests/artifacts/v13/formal/reading/report.json)、[导航](../tests/artifacts/v13/formal/header/report.json)、[原业务](../tests/artifacts/v13/formal/promoted-lan-smoke.json)。18081候选已停止，正式应用与PG继续active。[汇总](../tests/artifacts/v13/summary.json)。使用本机Chromium尺寸模拟，不声称实体设备 / 屏幕阅读器认证或新增发布泳道 / AB能力。

## 2026-11-02 V14：友链与页面设置

新增公开友链名录与站长管理，沿用现有阅读与工作台体系。公开入口在首页末尾和关于页；顶部栏 / 页脚结构保持，页面关闭后隐藏公开页、入口及sitemap记录。设置草稿与已保存的公开状态分离，头像默认首字、成功加载后显示图片，禁用JS仍可读。[实现和数据链路](EXPERIENCE_V14.md)

| 验证 | 结果 / 证据 |
| --- | --- |
| 后端 | PostgreSQL 18.6，`go test -race -json ./... -count=1` 共113个通过结果：99个顶层测试、14个子测试，0失败 / 0跳过。新增6组API / 事务 / 校验与3组SSR / 权限 / 公开入口测试。[汇总](../tests/artifacts/v14/go-test-summary.json)、[原始记录](../tests/artifacts/v14/go-test.jsonl) |
| 真实业务 | 独立候选库正常站长登录，71项通过：CRUD / 排序 / 显隐、搜索 / 状态筛选、确认取消、真实CAS冲突及输入保留、设置开关 / 持久化、原生新标签与无JS。恢复原设置，仅在候选留4条明确验收记录供截图。[报告](../tests/artifacts/v14/business/report.json) |
| 视觉 / 交互 | 四种宽度（1440/900/390/320）、浅深主题，201项通过；22张全页图，包含两个明确DOM空态样本，真实空态另有PG验证。无持久化业务写入。[报告](../tests/artifacts/v14/visual/report.json)、[图源](../tests/artifacts/v14/provenance.json) |
| 既有导航 | V13的104项滚动导航回归在候选通过。[报告](../tests/artifacts/v14/header-regression/report.json) |
| 构建 / 检测 | Vue类型、Vite、Go release、go vet及脚本语法通过；一次针对友链与接入客户端文件的detector无primary发现，原始Go模板默认黑色advisory按实际主题渲染界定，无新忽略项。[构建](../tests/artifacts/v14/build.log)、[检测](../tests/artifacts/v14/detector.json) |
| 独立审阅 | 初轮完整22张图唯一material fix为后台网址点击区域30→44px；第二批相同22张图重捕获，独立复看8张links图，评分resolved、disposition ship。PRODUCT窄记录，DESIGN / sidecar保持。[审阅](../tests/artifacts/v14/finish-review.md)、[文档](../tests/artifacts/v14/documentation-review.md) |

真实业务验证修正了桌面设置分区误继承隐藏规则，独立源审阅修复了未保存开关提前改变公开状态、无JS坏图遮盖首字的问题。测试脚本修正了关闭搜索框的隐藏listbox选择器、确认对话框异步完成时机和新标签跳转的断言边界；未通过修改产品来满足模拟外站正文。外站响应是验收模拟，只证明原生跳转 / opener隔离，不声称外站可用性。

`0013`新增两表、索引、默认页面设置与审计词汇，未修改已有文章数据。完整备份保留在仓库外 `/tmp/painter-next-before-v14-20261102/`。正式迁移期间旧临时服务停止后单元被回收，已用原环境与程序路径重建同名服务并通过ready；没有恢复或替换正式数据库。原始V13程序不识别schema13，已另准备保留V13界面且兼容schema13的私有回退程序，并在隔离库验证。[回退验证](../tests/artifacts/v14/rollback-compatibility.json)

现场在备份之后、迁移之前已有文章回收 / 恢复及评论操作审计；正式现态为23篇记录、11篇在回收站、0篇公开。当前状态完整保留，没有从候选恢复示例文章。迁移助手中断后没有持久化其内存中的即时切换前快照，因此**不宣称18个集合的切换前后hash完全一致**；改为独立恢复备份作基线，记录已有文章状态 / 公开指针 / 搜索索引的现场差异及迁移前审计，草稿 / 修订 / 身份 / 配置等哈希匹配、备份媒体8个文件hash匹配、原私有环境完全一致。新友链表0行、设置为迁移默认值，证明未复制候选夹具。[完整切换证据](../tests/artifacts/v14/promotion.json)

正式原地址通过28项友链 / 后台 / 入口 / sitemap / 无JS / 布局只读检查和正常手机站长登录，内容 / 配置写入0。原公共文章已在切换前移入回收站，未运行依赖post1仍公开的旧V6脚本，也没有改动现场状态来满足旧夹具。[正式检查](../tests/artifacts/v14/formal/friends-smoke.json)、[登录](../tests/artifacts/v14/formal/login.json)。四项实际送达的入口资源hash与当前构建一致。[清单](../tests/artifacts/build-manifest.json)。候选服务收尾与保密检查见[汇总](../tests/artifacts/v14/summary.json)。


## 2026-11-02 V16：连续编辑、管理表格与作者主页

- 全套Go race124个顶层/19个子例通过；无失败或跳过测试，两个包无测试文件。`go vet ./...`及Vue/TypeScript/Vite构建通过。
- 编辑器177项布局/交互与43项原安全路径通过；包含10组宽度/主题的实际首字命中检测、长标题增长/回缩，以及同一元信息节点保持位置。首轮153项未发现行号遮字，由视觉复核暴露并修复，已补足断言。
- 隔离真实业务25项通过：文章搜索/回收/恢复、标签单行编辑/归档、作者背景上传/冲突/删除保护、关注、同页作者搜索、评论发布/点赞/删除。作者背景后端另覆盖权限、CSRF/CAS、幂等、并发、GC和真实备份恢复。
- 64项只读视觉检查通过，36张最终图；两个完整捕获批次，手机首页两张因懒加载尚未完成而单独补采证据。首页包含桌面滚动推进、手机原生横向、减少动效及无JS回退。图片仅为隔离夹具，不进入正式站点。
- schema14兼容的旧UI回退程序17项实际验证通过，包含新背景媒体引用保护；原schema13程序不能直接回退。
- 单次布局扫描无发现；动态Go模板CSS链接不能解析，颜色/自定义属性分析不完整。Chromium视口模拟不等于实体手机或读屏认证。
- 正式切换、19类原业务集合/环境/媒体保留、正式只读检查和资源指纹见 `tests/artifacts/v16/`。实现和使用入口见 [EXPERIENCE_V16.md](EXPERIENCE_V16.md)。


## 2026-11-02 V17：紧凑选择控件与统一圆角

只读尺寸基线、候选2527项完整矩阵和935项截图批次检查通过，覆盖5宽度/两主题及宽屏粗指针；小屏配色与源码工具栏在最终79项聚焦复验中通过。12个最终局部图人工复核无阻断；实际15次截图操作包括两处目标替换及最终source技术重采。键盘、Teleport定位、复合输入内外高度及320px页面边界均保留证据。

Vue/TypeScript/Vite与Go嵌入构建通过，单次布局扫描无发现；未改后端业务或数据库，不冒用上轮Go结果。正式仅只读确认，程序和6个入口资源与构建相符，20类原业务集合、媒体、配置hash保持一致。证据见 `tests/artifacts/v17/`，实现见 [EXPERIENCE_V17.md](EXPERIENCE_V17.md)。
