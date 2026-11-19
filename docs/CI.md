# 持续集成

[CI 工作流](../.github/workflows/ci.yml)在推送 `next`、向 `next` 提交或更新 PR 时自动运行，也支持 GitHub Actions 页面的 **Run workflow**。工作流只验证代码，不部署应用，不使用正式数据库、站点会话或仓库 secrets。

## 检查范围

| 检查 | 内容 |
| --- | --- |
| Frontend and browser regressions | Node 24、锁定 npm 依赖、Vue/TypeScript 类型检查、生产构建、离线资源完整性、4 项混淆构建回归、Markdown/卡片协议及请求协调回归、独立 Chromium 通知回归 |
| Go and PostgreSQL 18 | 按 `go.mod` 安装 Go，校验模块、`go vet`、全量 `go test -race -count=1`、覆盖率、独立服务器构建 |
| Deployment and backup contracts | Python 部署与备份工具契约，使用临时目录、模拟 Docker 和服务边界 |

Go 检查使用该次 CI 前端任务新生成的资源，包含隐藏的 `.vite/manifest.json`、图片、字体和授权文件。仓库已有离线资源和本次构建资源都检查入口、依赖及实际文件是否齐全。现有打包存在非业务变量名造成的哈希差异，因此不以构建目录的逐字节 Git diff 作为失败条件；本地改前端仍须按协作约定运行 `make frontend` 并提交必要资源。

数据库是每个任务临时启动的 PostgreSQL 18 service，采用仅供可丢弃 runner 使用的 trust 认证，不保存应用凭据。连接地址通过该 service 的动态端口生成，只注入 `PAINTER_TEST_DATABASE_URL`。预检要求 PostgreSQL 18 及同版本 `pg_dump` / `pg_restore`；缺失数据库直接失败，避免集成测试被全部跳过后误报成功。每个 Go 集成测试继续新建、删除自己的临时数据库；测试不会回退到 `DATABASE_URL`。客户端来自 PostgreSQL 官方 Ubuntu 仓库。

真实供应商网络检查默认关闭。Python 备份测试中需要运行中用户 systemd manager 的执行检查，在 runner 没有该能力时按原用例跳过，不能把它描述为实际生产服务验收。

工作流使用只读仓库权限；PR 不取得部署权限。Actions 固定到完整提交 SHA，npm 依赖分别由 `web/frontend/package-lock.json` 和 `tests/ci/package-lock.json` 锁定。新提交会取消同一分支旧的检查。合成浏览器报告及 Go 覆盖率保留 7 天，前端资源仅供同一次运行的 Go 任务使用，保留 1 天；不上传会话、环境文件或数据库备份。

## 本地运行

```bash
npm ci --prefix web/frontend
npm ci --prefix tests/ci
node tests/ci/check-assets.mjs
npm run build --prefix web/frontend
node tests/ci/check-assets.mjs
npm run test:obfuscation --prefix web/frontend
node tests/ci/frontend.mjs unit
./tests/ci/node_modules/.bin/playwright install chromium
node tests/ci/frontend.mjs browser

# Linux 环境缺少 Chromium 系统库时，用 install --with-deps chromium。
# 也可显式设置 CHROMIUM_EXECUTABLE 使用本机已安装的兼容浏览器。

# 必须先设置指向可丢弃 PostgreSQL 18 的 PAINTER_TEST_DATABASE_URL，
# 并设置 POSTGRES_BIN_DIR 指向 PostgreSQL 18 客户端目录。
go mod verify
go vet ./...
go test -race -count=1 -timeout=20m -coverprofile=coverage.out ./...
make build
python3 -m unittest discover -s tests/deploy -v
python3 -m unittest discover -s tests/backup -v
```

`tests/ci/frontend.mjs` 使用明确的用例清单，不自动扫描执行历史浏览器脚本。部分历史脚本需要固定端口、真实候选内容、角色账号或供应商网络，甚至会修改数据；它们继续按[浏览器测试说明](../tests/browser/README.md)单独准备和运行。新增适合 CI 的纯前端用例时，同步扩充 `tests/ci/frontend.mjs` 的显式清单。CI 中的 Chromium 用例使用独立 DOM，不连接运行中的 Painter，也不代替真实业务端到端或物理手机验收。

仓库管理员如需阻止失败合并，可在 `next` 的 branch protection / ruleset 中将上述三个检查设为 required；工作流本身不修改仓库规则。

参考：[GitHub PostgreSQL services](https://docs.github.com/en/actions/tutorials/use-containerized-services/create-postgresql-service-containers)、[PostgreSQL Ubuntu 客户端](https://www.postgresql.org/download/linux/ubuntu/)、[Playwright CI](https://playwright.dev/docs/ci)。

## 统一颜色检查

前端构建先运行 `npm run check:colors`：验证 `web/colors.json` 的生成物未过期，再检查 CSS / Vue / TS / HTML 与明确列出的 Go 界面入口，拒绝裸色值、未知颜色 key 和组件覆盖全局颜色。CI 额外执行 `npm run test:colors` 的正反例。新增颜色和派生槽位的维护方法见 [全端色彩契约](COLOR_SYSTEM.md)。
