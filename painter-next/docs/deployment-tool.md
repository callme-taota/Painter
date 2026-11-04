# 一键部署与维护工具

Painter 的安装入口是 `./install.sh`，日常维护入口是 `./maintain.sh`。两者复用 `scripts/painter`，不会要求服务器安装 Node 或 Go 来运行已构建的发行镜像。运行形态是单个 Go 服务 + PostgreSQL 18；数据库、媒体和备份分别保存在固定 Compose 项目的持久卷中。

## 第一次部署

在完整源码目录或已制作的部署包目录执行：

```bash
./install.sh
```

向导默认推荐局域网模式，确认本机私网 IPv4、端口和可选 GitHub 配置后，生成私有 `.env`，构建源码或拉取明确版本的镜像，完成迁移并等待应用健康。然后执行 `./scripts/painter setup-token`，用服务器显示的一次性安装码进入输出地址的 `/setup`，创建站点与首位管理员。没有默认用户名和密码；默认使用本地账号，GitHub 登录是可选项。

无人值守本地服务器示例（替换为服务器实际的私网 IPv4）：

```bash
./install.sh --non-interactive --mode lan --lan-ip 192.168.2.209 --port 18080 --build
```

低资源目标机推荐在 CI/开发机事先构建并发布镜像，再使用 `--image` 替代 `--build`。项目没有默认公共镜像地址，必须填自己实际可拉取的版本 tag 或 digest，不能使用 `latest`：

```bash
./install.sh --non-interactive --mode lan --lan-ip 192.168.2.209 \
  --image "$PAINTER_RELEASE_IMAGE"
```

`.env` 权限固定为 `0600`，原文件存在时复用其项目名和全部秘密，不轮换密钥。若容器或卷仍在而 `.env` 丢失，安装会停止，要求找回原配置。部署目录应放在服务器持久磁盘，如 `/srv/painter`；不要把正式配置和业务数据放在 `/tmp`。Compose 项目名不能随目录调整而变更。

域名 HTTPS、Docker 显式安装和安装包制作参见[完整部署说明](DEPLOYMENT.md)。安装脚本不自动修改防火墙，应用健康也不等于其他设备的网络链路已验证。

## 日常维护

有终端时运行 `./maintain.sh`，可以选择检查健康、查看状态/日志、备份、恢复验证、启动、重启、升级或停止。菜单一次执行一个操作并透传退出码。无人值守任务直接使用以下命令，也可以把 `./scripts/painter` 换成 `./maintain.sh`：

| 命令 | 行为 |
| --- | --- |
| `./scripts/painter doctor` | 检查配置、Compose、数据库/应用健康与一次新的 HTTP readiness；展示宿主文件系统容量和备份目录。不重启服务、不泄露配置 |
| `./scripts/painter status` | 查看固定项目的容器状态 |
| `./scripts/painter logs --tail 100` | 查看近期日志；`--follow` 持续跟随 |
| `./scripts/painter backup` | 备份数据库和媒体并导出到 `backups/<目录名>`，追加原始私有配置与 Compose 快照 |
| `./scripts/painter backups` | 列出本机导出的备份目录；即使 Docker 不可用也能执行 |
| `./scripts/painter verify-backup --backup 目录名` | 用维护角色在独立临时数据库中真实恢复、校验并删除临时库；不替换运行数据库 |
| `./scripts/painter start` | 仅启动已存在的数据库/应用容器，先等数据库健康；不构建、拉取或迁移 |
| `./scripts/painter restart` | 仅重启应用容器并等待健康；不重启数据库、不更新版本 |
| `./scripts/painter update --image "$PAINTER_RELEASE_IMAGE"` | 完整备份成功后更新到指定镜像，按迁移、激活、健康等待顺序执行 |
| `./scripts/painter update --build` | 完整备份成功后构建当前源码并升级；不会自动 `git pull` |
| `./scripts/painter stop` | 停止服务，保留数据库、媒体、备份和全部卷 |

`--root /srv/painter` 可从其他工作目录指定部署目录；`--timeout 600` 调整健康等待秒数（默认 180，最大 3600）。安装、备份、恢复验证、启动/重启、升级和停止使用同一目录锁，防止维护操作相互穿插。诊断、状态、日志和备份目录查询不占用写锁。

升级时先用旧应用备份并完整导出，再拉取/构建新镜像。准备镜像失败时保留运行中的旧应用；停止后迁移失败时应用可能已经停止，工具返回失败并提示检查日志。不会在数据库可能升级后自动放回旧二进制；应修复后向前升级，或把备份恢复到独立数据库并明确切换。备份验证失败同样返回非零，不能把“导出了文件”等同于“已经验证可恢复”。

## 备份和恢复边界

每次备份有新的时间/随机目录，包含 Go 备份生成的 `manifest.json`、`database.dump`、`media/`，以及 `deployment.env`、`compose.yaml`、`deployment.txt`。其中 `deployment.env` 保存原始数据库密码、应用加密密钥及可选 OAuth 秘密，文件权限 `0600`，目录 `0700`。它只保留在私有备份里，不能提交仓库、贴进聊天或混入公开发行包。

```bash
./maintain.sh backup
./maintain.sh backups
./maintain.sh verify-backup --backup 20261102T010203Z-abc12345
```

验证从 Docker 备份卷读取同名目录，通过 `ops` 一次性容器运行现有 `restore-check`。该命令检查归档/媒体哈希，在随机临时 PostgreSQL 数据库实际恢复，并核对数据结构、投影和媒体引用，临时库清理完成后才报告成功与登记验证时间。它不会隐式启动/重建生产服务；数据库必须已运行，备份目录必须在该部署备份卷里。

仅把异机备份拷回宿主 `backups/` 不会自动导入 Docker 卷。异机恢复需要操作者明确把完整备份放到目标备份卷，使用与备份 schema 兼容的程序运行校验，恢复到新的业务数据库，补齐运行角色授权，保留原应用密钥，再检查后切换。工具不提供覆盖当前数据库的“一键还原”，避免把线上新增内容覆盖掉。原机持久卷不是异机备份，应定期把整个导出目录加密同步到独立设备。

## 当前验证范围

`python3 -m unittest discover -s tests/deploy -v` 用隔离临时目录和严格 Docker 替身验证命令顺序、权限、配置保密、失败退出码、菜单转发和独立恢复调用。测试不会连接真实 Docker、正式数据库或局域网服务。

当前开发环境没有可用 Docker daemon，因此这里不能声称完成了真实容器部署、镜像拉取、容器网络或真实容器内恢复验收。首次在目标服务器使用时，应部署实际镜像、完成 `/setup`，从另一台局域网设备检查访问，并至少执行一次真实的 `backup` 与 `verify-backup`。
