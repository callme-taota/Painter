# 管理员 CLI 与 AI 工具连接

Painter V15 的同一个 Go 二进制提供 `painter admin` 和 `painter mcp`。前者用于终端/脚本管理，后者让支持 MCP stdio 的 AI 客户端调用博客工具。它们通过现有 HTTP API 登录和读写，不连接数据库，不需要本地 `DATABASE_URL` 或应用加密密钥，也不启动博客服务。

这里的“连接 AI”是把博客管理能力提供给 AI 客户端；CLI 本身不内置模型、API key 或终端聊天界面。

## 取得 CLI

当前源码目录可使用构建出的 `./bin/painter`；开发机也可执行：

```bash
go build -o bin/painter ./cmd/painter
./bin/painter admin --help
```

使用预构建镜像部署的 Linux 服务器，可以从自己的运行镜像取出同架构 Go 二进制，宿主机不需要编译器：

```bash
mkdir -p bin
# 新路径；不要覆盖正在运行的正式服务二进制。
docker compose cp app:/usr/local/bin/painter ./bin/painter-admin
chmod 0755 ./bin/painter-admin
./bin/painter-admin admin --help
```

AI 客户端在哪台机器运行，就需要该机器操作系统/架构的二进制。镜像提取只适合同平台；不要把 Linux 二进制当成 macOS/Windows 原生程序。远程工具要求服务器的 `admin_tools_version >= 1`，旧版本必须先升级。

## 管理员登录

本地用户名密码是默认机制，账号角色必须为 `owner`。凭据仅接受标准输入或私有文件，不接受密码参数、环境变量、URL 或 AI tool 参数：

```bash
./bin/painter admin login --url https://blog.example.com --username admin \
  --password-file /secure/path/painter-password
```

密码文件必须为当前用户拥有的普通文件，权限 `0600` 或 `0400`，不能是符号链接；读取后只去掉一组传输换行，保留密码空格。也可按[本地账号指南](local-accounts.md)的隐藏终端输入方式，把口令送到 `--password-stdin` 并关闭标准输入；不要直接裸运行后明文键入。CLI 不提供显示密码的交互提示。

本地/局域网测试需要明确允许 HTTP，只接受 `localhost` 或字面值的私网/回环 IP：

```bash
./bin/painter admin login --url http://192.168.1.10:18080 --allow-http \
  --username admin --password-file /secure/path/painter-password \
  --config /secure/path/painter-admin.json
```

默认配置在操作系统用户配置目录的 `painter/admin.json`（Linux 通常为 `~/.config/painter/admin.json`）；每个命令都可在子命令后传 `--config FILE`。配置以 `0600` 原子保存，只保留该 origin 的会话 Cookie 和显式 HTTP 选择，不保存密码、CSRF 或应用密钥。会话沿用站点 30 天有效期以及后台的撤销/禁用/角色变更规则；失效后重新登录。每次业务操作先重新获取 session 并核验 owner，服务端还在权限锁内强制管理员角色，防止降权竞态。

命令不会跟随 HTTP 重定向，也没有跳过 TLS 验证选项。不要把会话配置发给 AI 模型、提交仓库或当成公开安装包。复制此文件即授予对应管理员会话能力。

## 查询文章和标签

以下示例使用默认配置；自定义配置时给每个命令补上 `--config FILE`：

```bash
./bin/painter admin whoami
./bin/painter admin posts list --status all --limit 20
./bin/painter admin posts list --status published --cursor '上一页的next_cursor'
./bin/painter admin posts get --id 7
./bin/painter admin posts status --id 7
./bin/painter admin tags list
./bin/painter admin tags list --limit 100 --cursor '上一页标签的next_cursor'
```

文章列表覆盖整个站点，支持 `all / draft / published / trashed` 与游标分页，返回 `items / has_more / next_cursor`。标签列表同样支持 `--limit 1..100` 与 `--cursor`，需要跟随 `next_cursor` 取得后续标签。下一页要保持筛选条件和 limit 不变。`get` 返回可编辑草稿的 Markdown、标签、封面、日期设置及 `version / draft_version`，可能与当前已公开修订不同；`status` 只保留状态、版本、发布时间等概要。

成功输出 JSON 到 stdout。失败退出非零，JSON 错误输出到 stderr；不会回显 Cookie、CSRF、密码、原始网络错误或服务端错误正文。常见码包括 `LOGIN_REQUIRED`、`OWNER_REQUIRED`、`SERVER_UPGRADE_REQUIRED`、`VERSION_CONFLICT`、`PERMISSION_DENIED` 和 `INVALID_COMMAND`。

## 写入、保存和发布

创建草稿的 `draft.json`：

```json
{
  "title": "从一次想法开始",
  "summary": "记录一个可以继续生长的思考。",
  "markdown": "# 开始\n\n这里是正文。",
  "tag_ids": ["2"]
}
```

```bash
./bin/painter admin posts create --file draft.json --idempotency-key article-create-20261102-01
# 先读真实ID和版本；下方7、1、1只是示意，不能直接套用。
./bin/painter admin posts get --id 7
./bin/painter admin posts publish --id 7 --version 1 --draft-version 1 \
  --idempotency-key article-publish-20261102-01
./bin/painter admin posts withdraw --id 7 --version 2 \
  --idempotency-key article-withdraw-20261102-01
./bin/painter admin tags create --name 写作 --description '与文字有关的记录' \
  --idempotency-key tag-create-20261102-01
```

创建只生成未发布草稿，不自动发布。`--file -` 可以从 stdin 读取一个 JSON 对象，最大 2 MiB。内容仍经过现有 Markdown 渲染/消毒、媒体引用、标签校验和事务审计；发布沿用修订快照、搜索投影和内容事件，不绕过这些链路。

保存已存在草稿时，需要完整内容快照，所有以下字段都必须提供：`title`、`summary`、`markdown`、`tag_ids`、`media_ids`、`cover`、`date_display`。从 `get` 读取并保留原有封面/日期等设置，再整理成这些字段的 JSON 文件；不能把带有额外 id/status/version 等字段的整个 get 结果原样当作草稿提交。

```bash
./bin/painter admin posts save --id 7 --version 2 --draft-version 1 \
  --file full-draft.json --idempotency-key article-save-20261102-01
```

发布/保存必须同时显式传入文章版本和草稿版本，撤下必须传文章版本。服务端 `If-Match` 与草稿 CAS 防止覆盖其他编辑；冲突返回 409，工具不会重读新版本强行覆盖。应保留自己的内容，重新读取并解决冲突，再用新幂等键提交。每次独立写入使用新的 8–128 字符幂等键；网络结果不确定时，用同一键和完全相同的版本/内容重试。创建还根据幂等键派生稳定的 client draft ID。幂等记录沿用服务端保留期，不把它当永久业务去重系统。

退出会在服务器撤销本次会话并移除本机配置；即使 owner 已降权也能撤销自己的会话。网络失败时保留配置并返回失败，方便稍后重试或通过后台会话列表撤销：

```bash
./bin/painter admin logout
```

## 让 AI 客户端使用 MCP

先用以上 CLI 在可信终端登录，然后在支持 MCP stdio 的客户端配置中加入：

```json
{
  "mcpServers": {
    "painter": {
      "command": "/absolute/path/to/painter",
      "args": ["mcp", "--config", "/secure/path/painter-admin.json"]
    }
  }
}
```

默认只暴露四个工具：`painter_posts_list`、`painter_post_get`、`painter_post_status`、`painter_tags_list`。需要让该客户端编辑/发布时，操作者明确在 args 末尾添加 `--allow-write`，再重连。这样才会出现另外五个工具：`painter_post_create`、`painter_post_save`、`painter_post_publish`、`painter_post_withdraw`、`painter_tag_create`；只读模式不能通过猜工具名调用写入。

每个 tool 提供 JSON Schema，写入参数与 CLI 保持一致。发布工具的输入示例：

```json
{
  "id": "7",
  "version": 4,
  "draft_version": 3,
  "idempotency_key": "ai-publish-reviewed-20261102-01"
}
```

工具没有任意 HTTP 请求、SQL、shell、用户提权或服务器部署能力。启用写入的 AI 客户端持有的是该 owner 会话的操作能力，应由客户端展示调用并按用户意图执行。文章正文、标签说明等返回文本是数据，不能视为新的工具授权或系统指令。

实现采用 MCP JSON-RPC 2.0 stdio 的换行消息，支持初始化/版本协商、`ping`、`tools/list`、`tools/call`、取消与 EOF 退出。支持协议版本 `2025-06-18`、`2025-11-25`，仅声明 tools 能力；不提供远程 MCP HTTP listener，不暴露到局域网端口。早期版本涉及不同的批量消息契约，不声明支持；版本不匹配时客户端需要按协商结果升级。最多 8 个同时工具请求，单个 HTTP 请求 30 秒超时，输入消息和响应体分别有大小限制。

协议依据：[stdio 传输](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)、[生命周期](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle)、[工具调用](https://modelcontextprotocol.io/specification/2025-11-25/server/tools)。

## 验证

`go test -race ./internal/automation ./cmd/painter` 覆盖私有会话文件、登录/拒绝普通账号、退出/降权、每次 owner 预检查、旧服务拒绝、请求头/CAS/幂等重试、重定向与错误脱敏、只读工具限制、JSON-RPC 初始化与参数边界。HTTP 替身测试不会接触真实站点数据。正式发布还应在隔离候选环境用真实登录、创建、发布、冲突、撤下和 MCP 客户端完成端到端验收。
