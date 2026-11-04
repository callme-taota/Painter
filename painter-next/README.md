# Painter Next

个人博客完整首版：Go模块化单体 + PostgreSQL 18，公开页面服务端渲染，Vue工作台、Tiptap文档编辑器与CodeMirror源码模式在构建时嵌入。数据库禁止SQLite；生产应用不需要Node、Redis或独立搜索服务。

项目方向已确认：界面优先突出个人气质，通过站名、简介、文章与主题组织表达；主要部署到本地服务器，优先支持局域网访问。

最新迭代 [V27 无框图片查看器](docs/EXPERIENCE_V27.md)已更新局域网：深色全屏遮罩直接显示完整图片，正常浏览只保留关闭与前后图标。隔离60项、实际候选11项和正式交付14项检查通过；正式检查页无公开图片，图片交互与视觉证据来自候选及隔离环境。58表、媒体、配置与既有首页顺序保留。

[V26 首页连续叙事](docs/EXPERIENCE_V26.md)已更新局域网并启用首页顺序：介绍纸页连向真实项目与发布日历，唱片格展开为游戏，收束后以最近文章结束。候选129项功能检查、25项收尾复核及10张替换视觉证据通过，获候选限定 SHIP；正式51项只读核验通过。112款游戏、291张专辑及既有内容保留，仅按请求更新首页顺序与收束阅读邀请。

[V25 首页浮动工具的液态玻璃](docs/EXPERIENCE_V25.md)已更新局域网：右侧工具随游戏、音乐影像的实际覆盖渐变为玻璃，离开后恢复原有实色。候选75项、正式33项只读检查通过，10张候选视觉证据获独立 SHIP；数据库、媒体与配置保持一致。

[V24 游戏库、发布日历与文章内容卡片](docs/EXPERIENCE_V24.md)已发布到局域网：更大的完整游戏墙、真实发布日历与 GitHub 项目牌组，文章支持行内 Emoji 网页参考、GitHub / Steam / Apple Music 卡片和共享图片查看器。正式59项只读检查与10项空稿回归通过，112款游戏 /291张专辑保留；实际日历2次发布 /1天，三个公开仓库已刷新启用。

[V22 首页内容与展示墙](docs/EXPERIENCE_V22.md)：六区块启停、排序与文案配置，Steam 游戏、Apple Music 唱片和自己的照片墙，以及 owner 首页管理。

[V21 动态图片与原图浏览](docs/EXPERIENCE_V21.md)：动态支持最多9张有序图片、逐图说明、失败重试和原图查看；可只发图片，保存与发布仍分离。

[V20 独立动态与创作入口](docs/EXPERIENCE_V20.md)：新增500字符纯文字动态、草稿与公开分离、动态专用布局和后台管理，以及按场景配置的可选右侧创作入口。

[V18 中性界面与纸页首页](docs/EXPERIENCE_V18.md)：后台采用 shadcn 风格的实色内嵌工作区，前台同步简化；首页替换为轻量 DOM 层叠纸页。

[V17 紧凑控件与统一圆角](docs/EXPERIENCE_V17.md)：收紧单选、下拉与标签栏，统一字号、间距和圆角层级。

[V16 连续写作、管理表格与作者主页](docs/EXPERIENCE_V16.md)：弱化源码入口、统一元信息与对齐，新增作者背景/同页搜索，完善评论层次和大封面展架。

[V15 搜索与管理工具](docs/EXPERIENCE_V15.md)：自定义多标签筛选、统一选择控件、导航调整，以及一键部署、维护工具和管理员 CLI / MCP。

[V14 友链与页面设置](docs/EXPERIENCE_V14.md)：公开友链名录、头像回退、后台增删改 / 显隐 / 排序，以及独立页面标题、介绍与开关。

[V13 阅读页与滚动导航](docs/EXPERIENCE_V13.md)：时间收进信息按钮，摘要与标签成组，封面与正文对齐、作者右置；区分相关推荐与正文分隔，非首页顶部栏下滚隐藏、上滚出现。

[V12 编辑页焦点与留白](docs/EXPERIENCE_V12.md)：移除正文大蓝框、收紧工具间距，并修正属性栏撑高标题的空隙。

[V11 编辑保护与资源状态](docs/EXPERIENCE_V11.md)：修复加载失败误保存、冲突自动覆盖、恢复时输入丢失，以及搜索、媒体、评论和片语状态问题。

[V10 编辑页布局](docs/EXPERIENCE_V10.md)：连续写作页、桌面文章信息栏、统一标题/正文起点与窄屏布局。

[V9 导航层级](docs/EXPERIENCE_V9.md)：职责分区与页面入口两层，设置深链接、角色范围与当前位置统一。

[V8 工作台与片语](docs/EXPERIENCE_V8.md)：独立侧栏后台、统一选择控件与设置分区，以及站长维护、前台大字滚动展示的句子/词语页面。

[V7 网页安装引导](docs/INSTALLATION_V7.md)、[GitHub 登录配置](docs/GITHUB_LOGIN_V7.md)与[鉴权检查 / 共享 MFA 方案](docs/AUTH_REVIEW_V7.md)。V7 支持三步安装与一次性首位管理员创建；MFA 仍是后续方案。上一轮 [V6 实现说明](docs/EXPERIENCE_V6.md)：编辑器使用可搜索、可移除的标签多选框，文章末尾显示最多三篇相关推荐。[V5](docs/EXPERIENCE_V5.md)恢复旧站“逃塔”的介绍、渐变封面及三种时间独立展示；[V4](docs/EXPERIENCE_V4.md)实现个人滚动首页与直接文档编辑。已有 Markdown 与保存链路保留。

## 本地服务器安装

新项目独立位于本目录，未修改旧站业务代码。实现边界与数据时序见 [实现架构](docs/ARCHITECTURE.md)，完成范围见 [开发记录](IMPLEMENTATION_STATUS.md)。

```bash
./install.sh
# 可选：创建5篇真实、可删除的示例文章，不创建登录凭据或管理员。
docker compose run --rm app seed-demo
```

交互向导默认选择局域网模式，检测并提示确认服务器的私网 IPv4，完成后输出 `http://服务器IP:端口`。仅绑定确认的一个 IP；也可选择仅本机访问或域名 HTTPS。非交互调用省略模式时仍使用仅本机模式，需要 LAN 时明确传入 IP。需要可用的 Docker Engine 与 Compose，首次源码构建需要网络与构建内存。向导自动生成配置、运行迁移并等待应用健康；`db`、一次性`migrate`和`app`由Compose编排，数据库不暴露宿主机端口。示例作者没有登录凭据。运行 `./scripts/painter setup-token` 在服务器读取安装码，再访问 `/setup` 完成站点与首位管理员设置；安装成功后通过 `/login` 正常登录。

已经有PostgreSQL 18时，可以直接运行Go程序：

```bash
make build
export DATABASE_URL='postgres://painter_app:YOUR_URL_ENCODED_PASSWORD@127.0.0.1:5432/painter?sslmode=disable'
export ENVIRONMENT=development
export PUBLIC_BASE_URL=http://localhost:8080
export APP_ENCRYPTION_KEY="$(openssl rand -base64 32)"
export ANALYTICS_HMAC_KEY="$(openssl rand -base64 32)"
export SETUP_TOKEN="$(openssl rand -hex 32)"
# 首次迁移使用独立迁移角色，随后服务使用上面的运行角色。
DATABASE_URL='postgres://painter_migrator:YOUR_URL_ENCODED_PASSWORD@127.0.0.1:5432/painter?sslmode=disable' ./bin/painter migrate
./bin/painter seed-demo
./bin/painter serve
```

数据库必须UTF8、Unicode `LC_CTYPE`（例如C.UTF-8），并允许迁移账号安装官方`pg_trgm`扩展。连接池初始4，启动核对迁移checksum和中文trigram。已提交的迁移文件不能编辑，升级使用新迁移。直接Go构建使用已交付的`web/assets`，不会运行npm。

## 局域网访问

当前预览：<http://192.168.2.209:18080>。同一局域网设备可使用该地址，当前机器需要保持运行。监听/公开地址、用户服务管理和Compose局域网配置见 [LAN_ACCESS.md](docs/LAN_ACCESS.md)。

## 本地账号与第一位站长

默认使用用户名与密码登录，入口 `/login`，注册入口 `/register`。生产没有默认密码，默认关闭注册。新站优先使用服务器安装码保护的 `/setup` 引导，已有身份的站点自动封闭安装。也可由有服务器权限的操作者使用离线命令创建。准备一个仅自己可读的单行密码文件（0600，密码至少15个字符），然后：

```bash
docker compose exec -T app painter create-local-user \
  --username admin --display-name 站长 --role owner --password-stdin < /secure/admin-password
# 直接二进制部署使用相同配置，读取宿主机私有文件。
./bin/painter create-local-user --username admin --display-name 站长 \
  --role owner --password-file /secure/admin-password
```

以上是两种部署方式的选择，不要重复创建同一用户名。密码不能放命令参数、Git或`.env`。登录后可在站点设置开放注册，新注册账号固定为reader，写作权限由站长授予；用户可验证当前密码后改密，忘记密码通过服务器侧离线重置。完整步骤、隐藏输入示例和API见 [本地账号文档](docs/local-accounts.md) 与 [部署文档](docs/DEPLOYMENT.md)。

密码使用随机盐 Argon2id；登录和注册有匿名CSRF、精确Origin检查、账号/IP限速与有限并发哈希。会话为哈希保存的opaque token、HttpOnly Cookie。改密/重置撤销会话，排队写请求在拿到用户锁后还会复核会话；封禁与权限即时生效。

GitHub 是可选登录，owner 可在后台设置填写 Client ID/Secret 与开关；页面提供准确回调，secret 加密保存且读取不回显。需要自行登记真实 OAuth App，尚未用真实凭据完成外部回调。未保存后台配置时兼容以下进程配置：

```dotenv
GITHUB_CLIENT_ID=YOUR_CLIENT_ID
GITHUB_CLIENT_SECRET=YOUR_CLIENT_SECRET
BOOTSTRAP_GITHUB_ID=YOUR_IMMUTABLE_NUMERIC_USER_ID
```

`BOOTSTRAP_GITHUB_ID`是实际账号的数值ID，仅在没有有效owner时用于旧GitHub引导；已有本地站长不会被任意登录取代。OAuth保留PKCE S256、一次性state和加密verifier，配置变化会中止在途流程；成功登录撤销当前请求携带的旧会话。不按相同邮件或用户名自动合并身份，本轮没有在线身份绑定/解绑。

## 页面与功能

| 区域 | 可用能力 |
| --- | --- |
| 阅读 | 视差首页、游戏墙与完整游戏库、文章流/详情、独立图文动态、统一标签、作者、页脚About、中文搜索/浮层建议、sitemap、旧链接跳转；禁JavaScript仍可阅读和搜索；RSS默认关闭、后台保留开关 |
| 创作 | 创建/自动保存、草稿找回、Tiptap文档/CodeMirror 6源码、工具栏/查找/快捷键、服务端规范预览、图片上传/粘贴、3种渐变封面/4种配色/自选图片、三项时间独立显示、IME组合保护、IndexedDB恢复、冲突比较/导出、检查点和历史恢复 |
| 发布 | 草稿/不可变修订/发布指针分离；版本CAS、幂等、原子搜索投影；撤回、回收站及恢复；恢复旧修订不会自动上线 |
| 个人空间 | 资料/头像、会话管理、文章赞、评论及评论赞、收藏、关注/粉丝、最近阅读历史与清空 |
| 站点运营 | 首页六区块配置与游戏/唱片/照片墙、公开歌单预览与确认导入、片语句子/词语管理与草稿/公开、大字滚动展示、品牌/简介、注册与功能开关、统一标签、分场景右侧悬浮控件、RSS开关、成员角色/封禁、评论隐藏、访问分析、媒体/任务/审计、真实备份状态 |
| 数据链路 | 持久埋点ACK、语义去重、站点/文章各自日UV、有效阅读、覆盖率；待处理行锁聚合；业务outbox、lease、消费收据和重试/dead |
| 交付 | 版本化迁移、非root容器、最小数据库角色、备份/隔离恢复校验、离线旧数据包导入、健康与受保护metrics |

界面保留旧 Painter 图标，统一为中性浅深色、实色表面与黑白主按钮。后台参考官方 shadcn dashboard-01/sidebar-07，使用两层侧栏、面包屑和有细边框的内嵌工作区；现有 Vue 组件继续承担交互与权限，没有迁移到 React/Radix。窄屏使用菜单抽屉，后台不显示公开顶栏、页脚或右浮控。V17 的 36px/44px 控件和 4px/6px/8px/12px 圆角层级继续沿用。

首页默认用承载实际站名、简介与 Painter 标识的 DOM 层叠纸页替换旧 WebGL 圆环和 homeLens，由原生滚动驱动轻量位移，没有模型下载或空闲渲染循环；减少动态效果和禁 JavaScript 时仍按文档流阅读。标题及界面改用本地 Geist Latin（29,400 字节）与系统中文无衬线字体，授权随站点提供于 `/assets/licenses/Geist-OFL.txt`，运行时不请求字体 CDN。

公开顶部为文章/动态/片语/搜索，页脚居中 logo/About/友链/2026（友链随开关显示）；标签通过搜索多选筛选与文章上下文进入，主题和文章快捷操作位于右侧悬浮控件。`Ctrl/Cmd+K` 打开全局建议搜索，筛选只在搜索结果页。主题支持浅色、深色、跟随系统，保留减少透明效果偏好；文章页边目录与移动端文章列表继续可用。V20 的独立动态与可选创作工具见 [迭代说明](docs/EXPERIENCE_V20.md)。当前设计规则见 [界面规范](web/DESIGN.md)，中性界面与首页的实现范围见 [V18 说明](docs/EXPERIENCE_V18.md)；业务数据链路见 [V3 说明](docs/EXPERIENCE_V3.md)。

## 一键部署与运维

在完整源码目录或已制作的安装包里运行 `./install.sh`。日常使用 `./maintain.sh`，支持诊断、备份、隔离恢复校验、重启与先备份再升级；参见[部署维护工具](docs/deployment-tool.md)。

交互向导默认面向本地服务器的局域网部署，并可填写真实 GitHub OAuth；重复执行保留原配置、密钥和持久卷。无人值守示例：

```bash
# 本地服务器：改为服务器实际的私网 IPv4。
./install.sh --non-interactive --mode lan --lan-ip 192.168.2.209 --build
# 仅本机体验。
./install.sh --non-interactive --local --build
# 可选公网：DNS 先指向服务器，确保 80/443 可达。
./install.sh --non-interactive --domain blog.example.com --build

./scripts/painter status
./scripts/painter logs --tail 100
./scripts/painter backup
./scripts/painter update --build
./scripts/painter stop
```

以上安装命令是首次安装的不同选择；已有站点变更访问方式需保留原秘密并明确编辑 `.env`，然后执行更新。更新依次执行备份与导出、构建/拉取候选、停止旧应用、迁移、启动和健康检查。失败不自动降级 schema；停止命令保留全部数据卷。

低资源服务器建议使用 `--image 你的镜像仓库/painter:明确版本` 替代 `--build`，避免在服务器编译。当前没有发布公共应用镜像；发布者可用 `scripts/package-deploy.sh` 制作带镜像版本的安装包。Ubuntu/Debian 支持显式 `--install-docker` 安装官方 Docker，需授权的 root/sudo；NAS 使用已安装的引擎。完整参数、安装包、失败处理和验证边界见 [DEPLOYMENT.md](docs/DEPLOYMENT.md)。

若已有HTTPS代理，可通过原有 `scripts/configure.sh --domain` 配置后仅启动默认服务，把代理转发到`127.0.0.1:8080`。生产要求HTTPS的`PUBLIC_BASE_URL`；不信任客户端任意代理头决定OAuth回调。数据库跨机请用正确证书和`sslmode=verify-full`。

默认预算：Go容器256MiB，数据库512MiB，连接20、shared_buffers64MiB；Go默认`GOMEMLIMIT=192MiB`（软内存目标）、`GOMAXPROCS=1`，可通过环境覆盖。1vCPU/1GiB是联合验收目标，尚未在该硬件或容器限制下压测。PostgreSQL是独立服务且自身多进程。运行镜像共享PostgreSQL基础层以携带匹配的备份客户端，应用容器只启动Go进程。

- 主数据：`pg-data`与`app-media`两个持久卷。
- 备份工作区：单独`backup-data`卷；完成后导出并按自己的异机存储策略同步。
- `GET /livez`为进程探针，`GET /readyz`校验DB/迁移/中文能力/媒体目录。
- `GET /metrics`仅已登录owner可访问，输出低基数请求/进程/连接池指标。
- `scripts/configure.sh`生成独立随机秘密并写0600的`.env`；不要在已有部署重新生成密钥。

基础镜像已固定具体版本与manifest摘要，记录见`deploy/images.lock.json`；依赖由`go.sum`及`package-lock.json`锁定。Docker构建和运行命令已提供，当前机器没有运行Docker守护进程，因此本次以本机二进制+真实PG验证，并检查Compose配置；没有把它称为已经验证的容器部署。

## 备份与恢复

备份会取得全站写入维护门，等待在途事务完成；公开读取继续可用，写命令会等待或返回可重试错误。锁同时阻止新媒体GC、后台聚合与迁移。数据库dump、媒体hash、manifest全部持久化后才写成功记录。

```bash
docker compose exec app painter backup /var/backups/painter/2026-10-24
# 恢复验证要创建并删除临时数据库，使用独立ops服务的维护账号。
docker compose run --rm ops restore-check /var/backups/painter/2026-10-24
# 导出完整目录，再加密传到异机存储。
docker compose cp app:/var/backups/painter/2026-10-24 ./backups/
```

直接二进制相同命令为`painter backup <新目录>`、`painter restore-check <目录>`。需要PostgreSQL 18客户端，可用`POSTGRES_BIN_DIR`指定。`restore-check`创建随机隔离库，实际pg_restore、核对约束/发布投影/媒体manifest、删除临时库；成功才更新持久验证时间，不替换生产库。

运维页区分“本地包已创建”和“已通过隔离恢复”，不会根据本地成功推断异机同步完成。保留期与异机同步由部署方安排；密钥独立备份。每日备份的初始目标RPO≤24小时/RTO≤1小时，未承诺任意秒PITR。

**实际恢复生产**：停写并保全当前数据→新建同locale的PG18库→pg_restore到新库→恢复`media/objects`和原密钥→重新设置迁移/运行角色授权→核对迁移checksum/引用/私有内容/登录→只读抽查→切连接并开放写。归档刻意不携带原库角色/ACL，因此恢复后必须重设业务DML与序列USAGE，并撤销运行角色对`schema_migrations`的DML；仅运行`migrate`不会重放已记录的授权迁移。保留原库以便失败回退。

## 旧站数据

提供可信离线JSON迁移包导入器（不是任意MySQL dump解析器），详见 [迁移包规范](docs/LEGACY_PACKAGE.md)。保留原作者/文章ID、分类标签/媒体映射/评论与全部互动关系；公开来源与后续工作副本分别导入，旧阅读数独立标注legacy。显式导入ID后校准identity序列。

```bash
painter import-legacy /path/to/package.json --dry-run
painter import-legacy /path/to/package.json
```

先对新空业务库执行迁移；dry-run只检查包和空库条件，不导入业务行/文件。失败保留journal，已复制而未提交的对象清理；模糊提交状态按journal核对，不能盲目重复。尚未读取或迁移用户真实旧MySQL，实际切流前还需只读导出、异常报告与全量演练。

## 开发与验证

```bash
# 修改前端后重新嵌入构建
make frontend
make build
# 纯函数与未依赖DB的检查
go test ./...
go vet ./...
# 真实PG集成：显式指向可创建临时库的测试集群；每项新建随机库并清理。
PAINTER_TEST_DATABASE_URL='host=/path/to/test-socket port=65439 user=test_admin dbname=postgres sslmode=disable' \
POSTGRES_BIN_DIR=/path/to/postgresql/18/bin go test -race ./... -count=1
```

实际验证记录见 [VALIDATION.md](docs/VALIDATION.md)；完整设计基线见父目录 [技术设计](../docs/next/TECHNICAL_DESIGN.md)。浏览器证据位于`tests/artifacts`。真实GitHub凭据、四种物理设备中文输入法、目标1GiB机器负载与容器运行、旧库正式切换、异机灾备不冒充本机测试已完成。

## 管理员 CLI 与 AI 工具

同一 Go 二进制提供 `bin/painter admin` 和 `bin/painter mcp`。管理员可登录后查询全站文章、状态和完整草稿、列出标签，创建/保存/发布/撤回文章与创建标签。AI 客户端通过 stdio MCP 调用，默认只读；需要写入时显式加 `--allow-write`。会话保存在私有文件，服务器校验站长权限、CSRF、内容版本和幂等键，不绕过发布链路。首次登录和客户端配置见 [CLI / MCP 使用说明](docs/admin-cli.md)。
