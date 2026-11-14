# 自动备份与恢复

Painter 使用 PostgreSQL 18。定时任务复用现有一致性数据库快照、媒体校验和真实隔离恢复，不另起数据库或常驻备份进程。自动备份与 Cloudflare Tunnel 分开运行，内网地址、公网域名和 Tunnel 连接不因备份改变。

## 备份包含什么

每个自动备份目录包含：

- `database.dump`：PostgreSQL custom 归档，包含文章、动态、句子、用户、密码哈希、MFA 绑定、客户端注册及全部业务数据。
- `manifest.json`、`media/objects/`：同一维护窗口内的 ready 媒体和 SHA256/大小校验信息。staging/deleting 对象由恢复后的生命周期协调处理。
- `recovery.env`：指定 `--config-snapshot` 时复制原运行配置，保留解密数据库中 OAuth、MFA、OIDC 等加密数据所需的原 `APP_ENCRYPTION_KEY`，以及 `ANALYTICS_HMAC_KEY`。不会在日志、状态输出或 Git 中展示这些值。
- `automatic.json`：数据库和媒体校验、真实 `pg_restore`、临时库删除全部成功后才写入的完成标记，并记录私有配置文件的 SHA256；恢复会先核对该文件。

目录权限为 0700，文件为 0600。`recovery.env` 是私有恢复材料，备份本身不做密码加密；复制到独立设备或云存储前应使用自己的加密存储通道。只有本机磁盘上的副本无法覆盖整机或磁盘丢失。工具提供本机自动备份、保留策略、实际恢复和回滚，不预设任何第三方付费存储。

数据库一致性沿用维护闸门：备份期间公开读取保持可用，写入短暂等待或返回忙碌。恢复验证发生在随机隔离数据库，结束后删除临时库，不覆盖正在服务的数据库。

## 原生部署：开启每日备份

先部署当前版本的 Painter 二进制。以下路径为示例，应替换为自己的二进制、私有配置、PostgreSQL 18 客户端与备份目录：

```bash
python3 scripts/backup-schedule.py install \
  --mode native \
  --binary "$HOME/.local/share/painter/bin/painter" \
  --env-file "$HOME/.config/painter/runtime.env" \
  --postgres-bin "/path/to/postgresql/18/bin" \
  --backup-root "$HOME/.local/share/painter/backups/automatic" \
  --keep 14
```

默认本机时区每天 03:30 开始，增加最多 10 分钟随机延迟，保留最近 14 份**完成真实恢复验证**的自动备份。`Persistent=true` 会在机器恢复运行后补做错过的任务。若需要在用户未登录时运行，应由部署方配置用户 systemd 的 linger。

立即做一次并查看结果：

```bash
systemctl --user start painter-backup.service
systemctl --user status painter-backup.service --no-pager
journalctl --user -u painter-backup.service -n 30 --no-pager
python3 scripts/backup-schedule.py status
"$HOME/.local/share/painter/bin/painter" backup-status \
  --root "$HOME/.local/share/painter/backups/automatic"
```

`backup-status` 不需要连接数据库或读取应用密钥，可以在站点无法启动时查看最后成功时间、最后目录、失败阶段和快照清单。定时器本身只代表已经排期，**应以 `last_success_at` 和最近快照中的 `verified_at` 判断备份是否完成**。没有成功记录、机器离线或连续失败时，应查看 journal 并检查空间、PG18 客户端和维护角色的 CREATEDB 权限。

修改时间可重新执行安装，追加 `--calendar '*-*-* 04:00:00'`；只生成单位不启用可加 `--write-only`。停用定时器：

```bash
python3 scripts/backup-schedule.py disable
```

只停用以后执行的排期，不删除数据或已保存的备份。

## 自动备份的失败与保留规则

- 使用操作系统文件锁防止同一自动备份目录下任务重入，进程退出后锁自动释放。
- 任一步失败均记录有限的错误码；不会把数据库连接串或供应商输出写入状态文件。
- 只有带有本工具完成标记、且真实恢复验证成功的 `auto-*` 目录才参与保留清理。手工备份、符号链接、没有完成标记的失败目录不会被自动删除。
- 失败时保留已有成功备份，**不执行保留清理**。没有完成标记的目录可在确认故障原因后由管理员手工清理，避免持续故障积累磁盘占用。
- 最近 `keep` 份是按成功备份计数，并非“保留多少天”。失败或停机不会淘汰可用的旧副本。

再次验证已有快照（需要与备份相容的程序、PG18 客户端和维护数据库权限）：

```bash
# 由相同 EnvironmentFile 启动的维护进程执行
painter backup-check --root /private/backups/automatic --snapshot auto-实际目录名
```

原有 `painter backup <新目录>`、`painter restore-check <目录>` 仍然可用，适合手工发布前快照。

## 原生部署恢复：先准备，再切换

恢复采用**新数据库 + 新媒体目录 + 私有候选配置**。原库、原媒体和原配置一直保留，便于回滚。源备份必须带 `recovery.env`；旧手工备份如果没有该文件，应先从独立保管的原配置中补齐原密钥，不能临时生成新密钥代替。

第一步，准备隔离恢复目标。脚本自动探测本机已经安装的 PG18 客户端；其他安装位置可加 `--postgres-bin /自己的/pg18/bin`。`--workspace` 必须是尚不存在的新目录：

```bash
python3 scripts/recover-backup.py stage \
  --backup "$HOME/.local/share/painter/backups/automatic/auto-实际目录名" \
  --workspace "$HOME/.local/share/painter/recoveries/本次唯一名称" \
  --runtime-env "$HOME/.config/painter/runtime.env" \
  --binary "$HOME/.local/share/painter/bin/painter"
```

它会执行以下操作：

1. 检查完整备份，真实恢复到临时库并校验，临时库清理后才继续。
2. 新建 `painter_recovery_*` 数据库，拒绝覆盖任何已存在的数据库；媒体也必须写入新的专用目录。
3. 恢复归档，在新数据库中以迁移角色校验迁移 checksum 并执行当前版本缺少的迁移，再恢复媒体和当前部署的运行角色权限，保留迁移账本/历史表原有的权限限制。原角色必须已存在，维护角色需要 CREATEDB 及所需的授权能力。
4. 保留账号与 MFA 长期绑定，同时使恢复出来的旧登录会话、未完成登录和 OAuth 授权尝试失效，避免恢复备份重新激活旧登录。
5. 生成 `previous.env`、`candidate.env` 和 `recovery.json`，均为私有文件。候选校验原密钥格式（独立的 32 字节 base64）并使用原备份的加密根密钥，但数据库连接的主机/账号仍取当前机器，只有目标库名和媒体目录改变。
6. 在回环地址启动候选实例，关闭后台同步/任务，检查 `/readyz` 后退出。此时对外服务仍使用原数据库。

如果备份的 schema 比当前程序旧，恢复准备会先在新库使用维护连接 `SET ROLE` 到迁移角色完成升级，并报告 `migrations_verified` 与 `schema_version`；运行角色无需拥有 DDL 权限。需要当前维护连接具备切换至该迁移角色的权限；可用 `--migration-role`、`--runtime-role` 指定已经存在的 PostgreSQL 角色。跨机器恢复应先准备相容的 PG18、角色与当前机器运行配置，不能直接复用旧机器的数据库口令、监听地址或 Tunnel token。

第二步，确认恢复时间点后实际切换。此操作会放弃当前页面所见的备份之后新增内容，所以需要显式的停写参数：

```bash
python3 scripts/recover-backup.py activate \
  --workspace "$HOME/.local/share/painter/recoveries/本次唯一名称" \
  --confirm-stop-writes
```

切换时暂停定时器，若已有备份正在运行则退出并要求稍后重试，不中断该备份。随后短暂停止 `painter.service`，原子替换运行配置，启动并检查内网 `/readyz`。健康失败会恢复原配置并尝试重启原服务。原数据库和两套媒体都不会删除。

切换成功后应打开内网和公网分别检查：

```text
http://当前内网地址:端口/readyz
https://自己的公网域名/readyz
```

然后抽查文章、图片、管理登录和 MFA。内网健康检查不等于外网 Tunnel 链路验收；外网也通过后再向外开放使用。旧库可暂留到本次恢复完成后再按数据保留要求人工清理。

需要回滚到切换前数据库：

```bash
python3 scripts/recover-backup.py rollback \
  --workspace "$HOME/.local/share/painter/recoveries/本次唯一名称" \
  --confirm-stop-writes
```

若运行配置在准备后又被修改，工具会拒绝直接覆盖，应重新准备。回滚也不会把两库之间后续新增内容进行合并。

## Tunnel 与内外网地址

恢复工具保留当前机器的 `PUBLIC_BASE_URL`、`LISTEN_ADDR`、`GITHUB_CALLBACK_URL`、`ENFORCE_PUBLIC_HTTPS`、`TRUSTED_PROXY_CIDRS` 等配置，只从备份取数据解密所需的原根密钥。它不会读取、备份覆盖或更换 cloudflared token，不改变 Cloudflare 的公开主机名，也不停止/重装 `painter-tunnel.service`。

因此 `blog.example.com → 当前 Tunnel → 当前内网 Painter` 的入口保持不变，内网仍可通过原地址访问。更换服务器属于独立的网络切换：先在新机器准备内网地址和 Tunnel，再进行数据恢复与入口验收。

## Docker Compose 部署

同一个 Go 命令可在 `ops` 维护容器运行。部署镜像应包含当前备份与恢复命令：

```bash
python3 scripts/backup-schedule.py install \
  --mode compose \
  --project-root /path/to/painter \
  --backup-root /var/backups/painter/automatic \
  --keep 14
```

调度器使用 `docker compose run --rm --no-deps -T ops backup-run`，不会隐式启动、更新或重建应用/数据库。配置通过只读挂载写入私有恢复快照；维护容器以 root 运行以读取宿主 0600 配置，数据库仍使用原有的维护角色。自动副本保存在现有 `backup-data` 卷，需要另行导出并加密复制到独立设备。

隔离检查或准备全新恢复数据库可使用：

```bash
docker compose run --rm --no-deps -T --user 0:0 ops backup-status --root /var/backups/painter/automatic

docker compose run --rm --no-deps -T --user 0:0 \
  --volume /自己的私有恢复目录:/var/lib/painter/recovery \
  ops restore-stage \
  --source /var/backups/painter/automatic/auto-实际目录名 \
  --database painter_recovery_example \
  --media /var/lib/painter/recovery/media \
  --runtime-role painter_app --migration-role painter_migrator
```

`restore-stage` 的目标媒体目录需先挂载到独立持久卷或宿主私有目录，再按新库和新媒体配置进行发布检查。原 Compose 业务库名及媒体挂载由 `compose.yaml` 固定管理，**`recover-backup.py activate` 仅适用于 native 用户 systemd 部署，不能直接切换 Compose**。Compose 的受控切换需要明确的 override 指向新数据库/新媒体，在隔离候选健康检查后停止旧应用并切换；旧卷和旧库保留。Compose 切换步骤由部署方按自己的卷与网络配置执行。

## 验证

```bash
go test -race ./internal/backupops
python3 -m unittest discover -s tests/backup -v
# PAINTER_TEST_DATABASE_URL 必须是一次性独立 PG18 测试集群
# 同时设置 POSTGRES_BIN_DIR 指向 PG18 客户端
go test ./internal/app -run 'TestRecoveryStage|TestRecoveryTargetValidation|TestRecoveryReplays' -count=1
```

测试覆盖失败不删除可用副本、防重入、状态中断识别、配置权限、符号链接拒绝、候选配置保持内外网设置、实际隔离恢复、旧会话失效、激活失败回退和再次回滚。
