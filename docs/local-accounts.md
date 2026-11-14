# 本地账号与离线恢复

Painter 使用本地 PostgreSQL 账号数据和 Argon2id 密码散列，没有默认用户名或默认密码。新站优先通过 `/setup` 创建首位站长；安装步骤见 [部署说明](DEPLOYMENT.md) 和 [网页安装](INSTALLATION_V7.md)。本指南中的 `admin` 是可替换的示例用户名，所有密码都由操作者自行输入。

## 账号与权限

用户名为 2–64 位 ASCII 字母、数字、下划线或连字符，以字母或数字开头；创建和离线恢复时会去掉两端空白并转为小写。显示名为 1–100 个 Unicode 字符。新密码须为 15–1024 个 Unicode 字符、最多 4096 个 UTF-8 字节，不能包含 NUL；允许空格，不会裁剪口令中的空格。

| 角色 | 用途 |
| --- | --- |
| `owner` | 站长，管理站点和账号 |
| `author` | 创作者，按服务端权限管理内容 |
| `reader` | 读者，公开注册固定使用此角色 |

站长在站点管理中控制注册和外部账号登录。关闭外部登录后，非站长已有会话与后续登录也受限；公开阅读保持可用。账号访问、两步验证和其他应用的 OIDC 接入见 [账号安全](ACCOUNT_SECURITY.md) 与 [认证中台](IDENTITY_PROVIDER.md)。

## 在部署环境中运行命令

以下示例用于受信任服务器上的 Bash 终端。先进入包含该部署 `compose.yaml` 和原 `.env` 的目录，确认 `app` 健康、数据库迁移已完成，再运行命令。Compose 读取原配置并复用既有项目名；不要把 `.env` 当 shell 脚本执行。

这些命令直接访问部署数据库，不经 HTTP，不要求浏览器会话，只能由具备服务器与数据库操作权限的管理员执行。直接运行 Go 二进制时，也必须由既有安全启动方式提供该部署的数据库、媒体和密钥配置；二进制不会自动读取 `.env`。不要为恢复账号另造数据库或应用加密密钥。

## 隐藏输入并创建账号

在 Bash 中执行以下完整命令块，按提示输入自己的强口令。密码通过管道传入标准输入，不出现在命令参数中：

```bash
(
  set +x
  set +a
  unset painter_account_password
  IFS= read -r -s -p '新账号密码（至少 15 个字符）：' painter_account_password </dev/tty || exit 1
  printf '\n' >/dev/tty
  printf '%s' "$painter_account_password" | docker compose exec -T app \
    painter create-local-user --username admin --display-name '站长' --role owner --password-stdin
)
```

括号限定变量生命周期；`set +x` 关闭命令跟踪，`set +a` 和 `unset` 避免密码变量被导出，`IFS=` 保留口令空格。管道结束提供 EOF。CLI 自身没有隐藏输入提示，不要在单独的 `--password-stdin` 命令后直接明文键入密码。

按需替换用户名、显示名和角色；省略 `--role` 时创建 `reader`，省略显示名时使用用户名。命令创建新账号，不覆盖已有同名账号，也不修改已有角色。创建首位 `owner` 会完成安装并关闭网页安装入口；无需再用 `/setup` 重复创建。成功响应仅含账号 ID、用户名、角色及创建状态。

登录时使用该站配置的 `/login`。生产环境通常要求 HTTPS；局域网 HTTP 阅读地址可能先跳转到 `PUBLIC_BASE_URL` 中的 HTTPS 登录域名。

## 使用私有密码文件

也可使用由操作者安全准备、保存在仓库外可信目录的密码文件。文件应归运行命令的操作者所有、权限为 `0600` 或 `0400`，且为普通文件，不使用符号链接。文件中只存密码，可附一组末尾换行。

宿主机上的文件可以通过标准输入交给容器，替换下例中的私有路径：

```bash
docker compose exec -T app painter create-local-user \
  --username admin --display-name '站长' --role owner --password-stdin \
  < /secure/path/painter-account-password
```

此时 `--password-stdin` 只读取输入，不检查宿主机文件权限，由操作者负责保护文件。若直接使用 `--password-file /secure/path/account-password`，路径必须位于运行该二进制的环境内；容器内路径与宿主机路径不同。CLI 拒绝最终路径为符号链接、非普通文件或向组/其他用户开放权限的文件。`--password-file` 和 `--password-stdin` 必须且只能选择一个。

CLI 仅移除末尾一组 LF 或 CRLF 传输换行，保留密码空格。不要把密码放进参数、命令字面量、导出的环境变量、Git、日志或聊天；私有文件由操作者按自己的保管策略清理。

## 忘记密码时离线重置

用目标账号的准确用户名执行重置。以下同样使用隐藏终端输入：

```bash
(
  set +x
  set +a
  unset painter_reset_password
  IFS= read -r -s -p '重置后的密码（至少 15 个字符）：' painter_reset_password </dev/tty || exit 1
  printf '\n' >/dev/tty
  printf '%s' "$painter_reset_password" | docker compose exec -T app \
    painter reset-local-password --username admin --password-stdin
)
```

该操作替换指定账号的本地密码、撤销其全部已有登录会话并记录审计；不改变显示名或角色，不接受 `--role` 或 `--display-name`。被禁用账号不能通过重置恢复访问。对仅有 GitHub 身份的已有账号执行此操作，会给同一个账号增加本地密码，应先核对目标身份。

密码重置保留已启用的 MFA。若验证器和恢复码均遗失，具备本机运维权限的管理员可另外执行：

```bash
docker compose exec -T app painter disable-local-mfa --username admin
```

移除 MFA 会撤销会话与未完成的两步验证请求并记录审计。用户重新登录后应重新绑定验证器并安全保管新恢复码；具体行为见 [账号安全](ACCOUNT_SECURITY.md)。

## 日常改密与会话管理

已登录用户可在个人空间验证当前密码后修改密码，完成后所有设备退出。会话列表支持撤销单个设备。忘记密码使用上述服务器离线入口，不通过邮件自助找回。

| API | 行为 |
| --- | --- |
| `GET /api/v1/auth/options` | 获取本地认证和注册状态及认证 CSRF 信息 |
| `POST /api/v1/auth/register` | 注册读者，受注册及外部登录策略限制 |
| `POST /api/v1/auth/login` | 验证本地密码；启用 MFA 时先进入第二步 |
| `POST /api/v1/me/password` | 验证当前密码并修改密码，撤销全部会话 |
| `GET /api/v1/me/sessions` | 列出当前账号有效会话 |
| `DELETE /api/v1/me/sessions/{id}` | 撤销指定会话 |

浏览器写请求仍需对应的 Origin、Cookie 与 CSRF 保护。密码、恢复码、Cookie 和部署密钥都不属于可提交的测试证据；备份及恢复方法见 [自动备份与恢复](AUTOMATIC_BACKUP_RECOVERY.md)。
