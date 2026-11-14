# 安装与运维

Painter 提供可执行安装向导，不需要在页面里手工创建数据库表。它复用固定的 Compose 项目、独立迁移账号、运行账号、持久卷和健康检查。当前从源码 checkout 或自行制作的发行包运行；项目没有宣称已发布可下载的公共镜像或 `curl | sh` 地址。

已确认以本地服务器为主要部署场景，优先使用局域网模式。交互向导默认推荐 LAN；仅本机体验和公网域名 HTTPS 继续作为可选模式。

## 三种访问方式

| 模式 | 应用绑定 | 访问入口 | 适用情况 |
| --- | --- | --- | --- |
| `local` | `127.0.0.1:8080` | `http://localhost:8080` | 仅本机体验；非交互省略模式时保留此默认值 |
| `lan` | 指定的一个 RFC1918 IPv4 | `http://指定IP:8080` | 本地服务器、家庭/办公局域网；交互向导默认选择，不会绑定 `0.0.0.0` 或 IPv6 |
| `https` | 应用仍在 `127.0.0.1:8080` | Caddy 的域名 HTTPS，80/443 | Linux 服务器公网部署 |

运行入口需要 Bash、Docker Engine、Compose 插件（v2.20 起或更新主版本）、OpenSSL。NAS 使用预装的 Docker/Compose 与 Bash；脚本不会替换 NAS 的容器引擎。运行镜像包含 PG18 备份工具，宿主机不必另装 PostgreSQL/Node/Go。源代码构建仍需要足够的构建内存和网络。

## 安装向导

在 `painter-next` 目录运行：

```bash
./install.sh
```

交互向导依次选择访问方式（默认 `lan`）、端口，以及是否加配可选的 GitHub OAuth。本地用户名与密码是默认登录方式，不依赖 GitHub；安装不会创建默认账号或密码，健康后执行 `./scripts/painter setup-token`，在输出地址的 `/setup` 完成站点与首位管理员设置，再通过普通 `/login` 登录。局域网模式会提示检测到的私网 IPv4，必须确认或输入正确地址；HTTPS 模式需要已解析到该服务器的域名，以及可达的 80/443。选择配置 GitHub 时，client secret 隐藏输入。非交互部署 LAN 必须明确给出 `--mode lan --lan-ip`，省略模式的已有自动化仍保持仅本机访问。

脚本先核对 Docker、Compose 与 daemon 权限。随后生成独立数据库密码、应用加密密钥、分析 HMAC 密钥和首次安装码，写入权限 `0600` 的 `.env`。它会构建或拉取镜像，停止旧应用后明确重建迁移容器并等待成功，最后启动应用并等 `/readyz` 对应的容器健康检查。HTTP 模式成功后输出访问 URL；HTTPS 只确认应用容器健康和 Caddy 进程运行，DNS、ACME 证书及外网链路仍须实际验证。

已有 `.env` 会原样复用，不改访问方式、OAuth 信息或秘密；过宽的文件权限会收紧到 `0600`。已有应用的安装重跑也会先备份再激活。缺失 `.env` 而相同项目已存在容器或数据卷时拒绝生成新秘密：必须恢复原配置，不能用新数据库密码/密钥配旧数据。

默认项目名为 `painter-next`。首次安装可用 `--project blog-home`，之后固定复用它；现有站点不能用 `--project` 偷换卷身份。源码镜像也按项目名隔离为 `<project>:local`。迁移、安装、更新、备份和停止通过项目目录下的原子目录锁串行执行；崩溃残留锁不会被强抢，需先核对其 `pid` 和运行进程，再人工移除该锁目录。

## 网页安装与创建首位管理员

新站访问 `/setup`，依次检查环境、填写站点信息、提交安装码并创建管理员。安装码仅通过服务器上的 `./scripts/painter setup-token` 显式读取；不要写进 URL、聊天或仓库。三步提交原子创建 owner 并永久封闭安装，不自动登录，公开注册保持关闭。已有 owner 或凭据的部署只显示已安装，安装码命令也拒绝输出。完整请求、数据表与并发时序见 [网页安装技术说明](INSTALLATION_V7.md)。

后台可由 owner 配置 GitHub OAuth App；未保存后台配置时继续兼容环境变量，保存后后台开关优先。需自己的真实 App 凭据；[配置说明](GITHUB_LOGIN_V7.md)列明密钥保存与独立身份边界。

### 离线创建与恢复入口

安装成功且 `app` 健康后，在受信任的服务器 Bash 终端、包含该部署 `compose.yaml` 与原 `.env` 的目录内执行。Compose 会读取该部署配置并复用其项目名；不要 `source .env`，也不要改用另一个项目或数据库。以下密码由隐藏终端输入，经宿主 Bash 内建 `printf` 送入容器标准输入，不写入命令参数、历史或 `.env`：

```bash
(
  set +x
  set +a
  unset painter_owner_password
  IFS= read -r -s -p '首位管理员密码（至少 15 个字符）：' painter_owner_password </dev/tty || exit 1
  printf '\n' >/dev/tty
  printf '%s' "$painter_owner_password" | docker compose exec -T app \
    painter create-local-user --username admin --display-name '站长' --role owner --password-stdin
)
```

括号让密码变量在命令结束后消失，关闭命令跟踪与自动导出，`IFS=` 保留口令空格；管道结束即提供 EOF。CLI 自身不会隐藏键入内容，也不会仅按回车就结束读取，因此不要直接在裸 `--password-stdin` 命令后明文键入密码。

也可使用已安全生成、放在可信私有目录的宿主密码文件。它必须由当前操作者持有、权限为 `0600`，是仅该操作者可读写的普通文件，不能是符号链接；将真实路径替换到下面的输入重定向中。这里仍使用 `--password-stdin`，宿主文件不需要复制进容器；stdin 分支不会替操作者检查宿主文件权限：

```bash
docker compose exec -T app painter create-local-user \
  --username admin --display-name '站长' --role owner --password-stdin \
  < /secure/path/painter-owner-password
```

使用独立强口令：15–1024 个 Unicode 字符、最多 4096 字节，不能包含 NUL；空格保留，输入末尾的一组换行符仅作为传输结束符移除。不要把密码 `export` 为环境变量、写成命令字面量、保存到仓库或聊天中；由操作者安全保管并按需清理临时密码文件。CLI 也支持 `--password-file`，但该路径必须在命令运行的容器内，不能直接填宿主路径。

成功时命令只输出账号 ID、用户名与 `owner` 角色，不回显密码；它不覆盖已有用户名或更改已有账号角色。随后通过安装输出的同一 origin 的 `/login`，用 `admin` 和刚设置的密码登录。这个离线入口由具备服务器与数据库访问权限的操作者使用；网页入口另受一次性安装码保护，两者均没有默认密码。

生产环境默认关闭公开注册。owner 登录后台后，可在站点设置中开启 `registration_enabled`；浏览器 API 对应状态名为 `registration_open`，公开注册创建的账号固定为 reader，不能指定管理员角色。账号 API、密码文件约束、离线重置与会话撤销详见[本地账号与离线恢复](local-accounts.md)。本地账号不依赖邮件找回或 GitHub 登录；已有 GitHub 身份仍可按下节选择配置兼容登录。

## 非交互与低资源镜像模式

```bash
# 明确从当前 checkout 构建；构建内存不属于 1 GiB 运行目标。
./install.sh --non-interactive --local --build

# LAN 必须明确私网 IPv4；例如本机当前地址，不能填通配或公网地址。
./install.sh --non-interactive --mode lan --lan-ip 192.168.1.10 --port 18080 --build

# 将下面变量替换为你已发布、能实际拉取的镜像；没有默认公共 registry 地址。
export PAINTER_RELEASE_IMAGE='YOUR_EXISTING_REGISTRY/PAINTER:EXPLICIT_VERSION'
./install.sh --non-interactive --domain blog.example.com --image "$PAINTER_RELEASE_IMAGE"
```

`--image` 接受明确版本 tag 或 `@sha256:<digest>`，拒绝 `latest`、无版本镜像和 shell 字符。配置镜像时使用 `pull` 和 `up --no-build`，服务器不编译 Node/Go；私有 registry 需事先使用自己的授权方式登录。只有 `--build` 或未配置镜像的完整源码 checkout 才构建。当前没有公共镜像可替用户填写，1 GiB 运行预算也不表示 1 GiB 可以完成编译。

可选的无人值守 GitHub 配置（本地账号无需此配置）：

```bash
./install.sh --non-interactive --domain blog.example.com --image "$PAINTER_RELEASE_IMAGE" \
  --github-client-id YOUR_REAL_CLIENT_ID \
  --github-client-secret-file /secure/path/github-client-secret \
  --owner-github-id YOUR_VERIFIED_NUMERIC_GITHUB_ID
```

secret 文件最多 4096 字节且为单行。回调必须在 GitHub OAuth App 中实际登记为安装结果的 `PUBLIC_BASE_URL/auth/github/callback`。若选择通过该 GitHub 身份授予站长，必须指定本人核实的不可变数字 ID，并完成真实 GitHub 登录；这不是本地账号登录的前置条件。脚本不根据用户名或邮件猜测或合并身份。秘密使用 dotenv 字面值转义；读取配置不执行 `source .env`，父 shell 的同名环境变量不会覆盖已保存配置。不要把 secret 放在命令参数、聊天记录或 Git 中。

旧入口继续有效：

```bash
./scripts/configure.sh --local
./scripts/configure.sh --domain blog.example.com
./scripts/configure.sh --non-interactive --mode lan --lan-ip 192.168.1.10
```

`configure.sh` 仅生成/检查配置，不启动 Docker 服务。它也不会覆盖已有配置；已有站点要改访问方式或 OAuth，请保留原秘密并明确编辑 `.env`，再执行受备份保护的更新。用输出的同一个 origin 访问；端口、协议或 IP 改变意味着浏览器 Cookie/IndexedDB 是另一个站点。

## 运维命令

```bash
./scripts/painter setup-token  # 仅新站且由服务器操作者读取安装码
./scripts/painter status
./scripts/painter logs --tail 100
./scripts/painter logs --follow
./scripts/painter backup
./scripts/painter update --image "$PAINTER_RELEASE_IMAGE"
./scripts/painter update --build
./scripts/painter stop
```

`backup` 用旧运行容器执行数据库与媒体的完整备份，写入独立备份卷，再导出到本目录 `backups/<UTC时间>-<随机后缀>/`。每次使用新目录；备份失败、导出失败或导出包缺 manifest 都返回失败。`update` 必须先完成这一流程，失败不会构建、拉取、停止应用或启动迁移。

更新顺序为：旧应用备份与导出 → 构建/拉取候选 → 停止旧应用 → 强制创建新迁移容器并等待退出码 0 → 启动新应用 → 健康等待。镜像变更仅在新应用健康后原子写入 `.env`；失败保留旧配置及备份。构建/拉取失败时旧应用继续运行；停止之后的迁移/启动失败时旧应用可能已停止，脚本不会谎报仍在服务，也不会自动把旧二进制放回可能已更新的 schema。查看错误阶段和日志，选择修复后继续前进，或按独立恢复流程恢复到新数据库并人工切换。

脚本不执行 `git pull/reset/checkout`，不覆盖本地源码，不做自动 schema 回滚。源码更新由你明确获取并审阅版本后执行 `update --build`。不要修改已应用迁移；退出码非零时不应继续批处理为成功。默认健康等待 180 秒，`--timeout 600` 可调整到最多 3600 秒。

`stop` 仅执行 Compose stop，保留数据库、媒体、备份与所有卷；没有任何默认 `down -v`、清库、删除镜像或轮换密钥的路径。状态和日志不需要项目写锁。完整卷身份包括固定 Compose 项目名和其 `.env`，请一起保管。

恢复验证仍使用独立维护账号：

```bash
docker compose run --rm ops restore-check /var/backups/painter/PASTE_THE_BACKUP_DIRECTORY_NAME
```

它创建临时数据库、实际恢复并核对，再删除临时库；成功后才登记验证时间。这不自动恢复生产、不证明异机同步。备份卷与原机一起损坏仍会丢失，应把已导出的完整目录加密同步到另一台机器，并独立保管原应用密钥。生产恢复需重新设置运行角色授权；归档不携带源库 ACL，不能用重跑已记录的授权迁移代替重新授权。

## 可选安装 Docker

Ubuntu 22.04/24.04/26.04/26.10 与 Debian 12/13、systemd 主机可以显式使用：

```bash
./install.sh --install-docker
# 已经授权的无人值守 root/sudo 环境：
./install.sh --non-interactive --local --build --install-docker
```

该选项依据官方 apt 仓库流程安装 `docker-ce`、CLI、containerd、Buildx、Compose，并启用 Docker 服务。它只在显式选项下申请 root/sudo，不自动添加有 root 等价权限的 docker 用户组成员，不卸载冲突包，不替换 NAS 已有引擎。无非交互 sudo 权限时会直接失败，不能假装安装成功。Docker 已可用时不安装/升级任何包；已有冲突包或不同 apt source 则保留并要求人工处理。

参考官方来源：[Ubuntu](https://docs.docker.com/engine/install/ubuntu/)、[Debian](https://docs.docker.com/engine/install/debian/)、[Compose 插件](https://docs.docker.com/compose/install/linux/)。脚本不修改防火墙；Docker 发布端口需要按实际网络管理，不应只凭 ufw 状态推断不可访问。

## CI 的安装包

在你实际构建并发布应用镜像之后，可生成不带编译器、源码、密钥或业务数据的安装包：

```bash
./scripts/package-deploy.sh --image "$PAINTER_RELEASE_IMAGE" --output /tmp/painter-next-deploy.tar.gz
sha256sum -c /tmp/painter-next-deploy.tar.gz.sha256
```

归档采用文件白名单，带 `install.sh`、Compose、部署脚本、网页安装/GitHub 配置/鉴权方案及本地账号运维文档，以及 `deploy/release-image.txt`。解包后向导默认使用该镜像，因此目标主机无需编译；已存在 `.env` 的站点仍固定使用自己保存的镜像，更新需显式 `--image`。此工具不推送镜像、不上传安装包、不创建下载地址；未来发布系统可以把归档和校验文件作为 CI 产物分发。

## 验证边界

测试用临时项目与 mock Docker 覆盖配置、保密、残留卷、更新先备份、停止/迁移/激活顺序、失败保护和健康超时；真实 Compose CLI 仅做只读解析和特殊字符 secret 的字面值 roundtrip。测试命令和报告见 [`tests/deploy`](../tests/deploy/README.md)。本次机器 Docker CLI/Compose 可用，但 daemon 未启动且没有免密 sudo；没有执行主机依赖安装、真实容器部署、公开 ACME 或远端 NAS 验证，也没有触碰当前本机运行的 Go/PG/LAN 服务。
