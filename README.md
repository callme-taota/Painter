# Painter

Painter 是个人博客与发布系统，采用 Go 模块化单体、PostgreSQL 18 和服务端渲染。Vue 工作台提供 Tiptap 文档编辑与 CodeMirror 源码模式；前端资源嵌入 Go 二进制，生产运行不需要 Node、Redis 或独立搜索服务。

## 当前功能

- 文章、图文动态、片语、连载专栏、统一标签、中文搜索、作者主页与友链；公开内容支持无 JavaScript 阅读。
- 文档与 Markdown 双模式编辑、自动保存、本地恢复、版本冲突处理、检查点和历史修订。保存与发布分离，恢复历史不会自动上线。
- 可配置的个人首页、发布日历、GitHub 项目、Steam / PlayStation 游戏、Apple Music 唱片及照片展示。
- 个人资料、收藏、阅读历史、关注、评论与点赞；短链接、专属分享页和外站嵌入卡片。
- 本地账号、可选 GitHub 登录、TOTP 两步验证与恢复码，以及 OIDC Authorization Code + PKCE 应用接入。
- 角色与站点管理、访问分析、媒体与审计、管理员 CLI / MCP、一致性备份、定时备份及隔离恢复验证。

## 安装与运维

在仓库根目录运行安装向导，需要 Docker Engine 与 Compose：

```bash
./install.sh
```

向导支持指定局域网 IPv4、仅本机访问与域名 HTTPS，准备配置、迁移和健康检查。首次源码构建需要网络；也可使用自行发布且固定版本的应用镜像。新站在服务器执行 `./scripts/painter setup-token` 读取安装码，再访问安装输出地址的 `/setup` 创建首位站长。没有默认管理员密码，公开注册默认关闭。

```bash
./maintain.sh
# 或直接使用维护命令
./scripts/painter status
./scripts/painter backup
./scripts/painter update --build
```

安装参数、数据库角色、原生二进制部署与 HTTPS 代理见[部署指南](docs/DEPLOYMENT.md)；维护命令见[部署维护工具](docs/deployment-tool.md)。迁移或切换前先备份，恢复先在隔离环境验证，保留原密钥、监听地址及 Tunnel 配置。

## 构建与开发

当前目录就是应用根目录。Go 版本以 [go.mod](go.mod) 为准；前端构建使用 Node 24 和锁定的 npm 依赖。数据库仅支持 PostgreSQL 18，需要 UTF-8、Unicode locale 和 `pg_trgm` 扩展。

```bash
# 使用仓库内已构建的前端资源，生成 bin/painter
make build

# 修改前端后，重新构建资源并嵌入二进制
make frontend
make build
```

`web/assets/` 是随源码交付的前端资源，包含 Vite manifest、字体、图片和授权文件。直接 Go 构建不会运行 npm；Go 模块依赖已缓存时可离线构建。配置数据库连接、公开地址和应用密钥后可运行 `./bin/painter serve`，首次迁移须使用独立迁移角色，日常服务使用运行角色，具体配置见部署指南和 [deploy/example.env](deploy/example.env)。

## 测试

GitHub Actions 自动检查 `next` 的推送和 PR，覆盖 Go／PostgreSQL 18、前端与独立浏览器回归、部署与备份工具。运行范围及本地复现命令见[持续集成说明](docs/CI.md)。

```bash
# 纯逻辑检查；未提供测试数据库时跳过数据库集成用例
go test ./...
go vet ./...

# 部署与备份工具契约测试，不连接正式服务
python3 -m unittest discover -s tests/deploy -v
python3 -m unittest discover -s tests/backup -v

# 前端类型检查与生产构建
make frontend
```

数据库集成测试只读取显式的 `PAINTER_TEST_DATABASE_URL`，不会回退到 `DATABASE_URL`。指向可创建并删除临时数据库的可丢弃 PostgreSQL 18 测试集群；备份用例还需要匹配的 PostgreSQL 18 客户端：

```bash
PAINTER_TEST_DATABASE_URL='host=/path/to/test-socket port=5432 user=test_admin dbname=postgres sslmode=disable' \
POSTGRES_BIN_DIR=/path/to/postgresql/18/bin \
go test -race ./... -count=1
```

浏览器用例、隔离环境要求和参数见[浏览器测试说明](tests/browser/README.md)。截图、报告、浏览器会话与测试凭据不提交 Git。

## 目录

| 路径 | 用途 |
| --- | --- |
| `cmd/painter/` | 服务、迁移、备份与管理命令入口 |
| `internal/` | 应用、身份、自动化和备份模块 |
| `migrations/` | 版本化 PostgreSQL 迁移及嵌入入口 |
| `web/templates/` | 服务端页面模板 |
| `web/frontend/` | Vue / TypeScript 源码和前端构建工具 |
| `web/assets/` | 必须随源码保留的已构建嵌入资源 |
| `deploy/`、`scripts/` | 部署示例、安装与运维工具 |
| `tests/` | 浏览器、部署和备份工具测试及必要 fixtures |
| `docs/` | 当前架构、功能合同和操作指南 |

## 文档

- [实现架构](docs/ARCHITECTURE.md)、[界面规范](web/DESIGN.md)、[全端体验规范](docs/CROSS_PLATFORM_EXPERIENCE.md)
- [安装与部署](docs/DEPLOYMENT.md)、[首次网页安装](docs/INSTALLATION_V7.md)、[部署维护](docs/deployment-tool.md)
- [本地账号与离线恢复](docs/local-accounts.md)、[账号安全与两步验证](docs/ACCOUNT_SECURITY.md)、[GitHub 登录](docs/GITHUB_LOGIN_V7.md)、[OIDC 接入](docs/IDENTITY_PROVIDER.md)
- [专栏与分享](docs/SERIALS_AND_SHARING.md)、[文章内容卡片](docs/CONTENT_CARDS_V24.md)、[AI 文章格式与导入](docs/ARTICLE_IMPORT.md)、[Steam / PlayStation 游戏墙](docs/PSN_GAME_WALL.md)
- [自动备份与恢复](docs/AUTOMATIC_BACKUP_RECOVERY.md)、[管理员 CLI / MCP](docs/admin-cli.md)、[旧站离线数据包导入](docs/LEGACY_PACKAGE.md)

## 分支与源码边界

`next` 是独立新架构分支，应用直接位于仓库根目录。`unstable` 保留此前混合目录及历史，旧架构不进入当前源码树。模块名 `painter-next` 和现有数据库迁移保持不变，避免目录整理影响运行合同。

只跟踪源代码、必要的构建资源、测试夹具和通用文档。真实环境、Token、数据库、媒体、恢复材料、浏览器会话及本机验收产物保存在仓库外或忽略目录中。
