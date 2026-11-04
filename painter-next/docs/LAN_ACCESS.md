# 局域网预览

2026-11-02 新版已切换到当前地址：<http://192.168.2.209:18080>。同一局域网的电脑/手机可以使用该地址；承载服务的机器需要保持运行。当前接口为 `ens2`，IPv4子网为 `192.168.2.0/24`。

## 当前运行配置

- `LISTEN_ADDR=192.168.2.209:18080`，只绑定该局域网IPv4地址。
- `PUBLIC_BASE_URL=http://192.168.2.209:18080`，规范链接、RSS、站点地图、Origin检查及OAuth回调保持同一地址。
- `ENVIRONMENT=development`，供局域网HTTP预览。未放宽CSRF、身份或私有内容权限。
- PostgreSQL仍只使用本机Unix socket，没有对局域网开放数据库端口。当前专用测试库为 `painter_next_v3_20261028`，媒体目录为 `/tmp/painter-next-v3-media`。
- 默认入口为 `/login` 本地用户名/密码，本次测试库已开放 `/register`（新用户固定reader），站长可在后台关闭。四个专用测试账号的密码通过会话单独交付，不进入仓库；没有生产默认密码。
- V3从旧预览备份克隆并迁移；V7在原库执行0010/0011。V8执行0012，schema为12，继续使用原库、媒体与私有配置。23篇文章、23份草稿、32份修订、5个公开指针及16个媒体对象，连同用户/凭据/站点配置和安装状态的升级前后hash一致。没有复制候选QA库或预置片语；后台“内容管理→片语”管理，公开页 `/phrases`，首页末尾有入口。候选18081已停止，原18080和PG active；[切换](../tests/artifacts/v8/promotion.json)、[原业务16项](../tests/artifacts/v8/promoted-lan-smoke.json)、[工作台19项](../tests/artifacts/v8/promoted-workbench-smoke.json)。
- 当前V9只更新程序与导航：四个职责分区、二级入口、设置深链接和当前位置。未执行迁移，schema仍12；原库18个内容/身份/配置/会话集合在切换前后count/hash一致，私有环境未变，候选库未复制。正式登录后的31项只读检查通过，内容/配置写入0；候选18081已停止。[切换](../tests/artifacts/v9/promotion.json)、[原业务16项](../tests/artifacts/v9/promoted-lan-smoke.json)、[导航15项](../tests/artifacts/v9/promoted-navigation-smoke.json)。
- 当前V10仅更新编辑页布局：连续写作页、宽屏属性栏、窄屏折叠与源码整页宽度；schema仍12，原库、媒体及环境保留，18个集合在切换前后count/hash一致。候选QA只留在克隆库，18081已停止。[切换](../tests/artifacts/v10/promotion.json)、[验收](VALIDATION.md)。
- 当前 V11 修复编辑器加载 / 冲突 / 历史恢复保护，以及搜索、媒体、评论和片语状态；schema 仍为12。程序已更新，原库18个集合及私有环境在切换前后完全一致；未复制候选 QA 数据。[实现](EXPERIENCE_V11.md)、[切换](../tests/artifacts/v11/promotion.json)、[验收](VALIDATION.md)。
- 当前 V12 更新编辑页焦点与间距，修正属性栏撑高标题；仅 CSS 调整，schema 仍12，原数据及私有环境在切换前后保持一致。[实现](EXPERIENCE_V12.md)、[切换](../tests/artifacts/v12/promotion.json)、[验收](VALIDATION.md)。
- 当前 V13 更新文章首屏、时间信息披露、右侧作者、相关推荐边界及非首页滚动导航。schema仍12；原库18个集合及环境在切换前后完全一致。原地址再次通过175项阅读、104项导航和16项原内容 / 登录检查；未复制候选库，18081候选服务已停止。[实现](EXPERIENCE_V13.md)、[切换](../tests/artifacts/v13/promotion.json)、[验收](VALIDATION.md)。
- 当前 V14 新增 `/friends` 与“内容管理 → 友链”，包含CRUD、显隐 / 排序、页面标题 / 介绍 / 开关。原库迁移至schema13，未替换数据库、媒体或私有环境；当前已有的文章回收站状态保留，候选验收记录未复制。正式28项只读检查及正常站长登录通过。[实现](EXPERIENCE_V14.md)、[切换](../tests/artifacts/v14/promotion.json)、[验收](VALIDATION.md)。
- 当前 V16 更新连续编辑、文章/标签表格、作者主页背景与同页搜索、评论层次及大封面写作展架。原库增量迁移至schema14；原文章、账号、设置和媒体保留，不复制候选测试内容。[实现](EXPERIENCE_V16.md)、[切换](../tests/artifacts/v16/promotion.json)、[验收](VALIDATION.md)。
- 当前V17统一选择控件密度与4/6/8/12px圆角层级，保留导航大圆角；仅更新前端与嵌入程序，无迁移，schema仍14。原20类业务集合、媒体及私有环境在切换前后保持一致。[说明](EXPERIENCE_V17.md)、[切换](../tests/artifacts/v17/promotion.json)。
- 用户级临时服务：`painter-next-lan.service`、`painter-next-preview-db.service`。由systemd管理，离开启动终端不会停止；这不是开机持久部署，配置、数据库及媒体仍使用本次预览的私有临时目录。

```bash
systemctl --user status painter-next-lan painter-next-preview-db
# 修改当前私有配置后重启应用
systemctl --user restart painter-next-lan
# 停止本次预览
systemctl --user stop painter-next-lan painter-next-preview-db
```

私有进程配置在 `/tmp/painter-next-dev.env`，权限0600；不要将其中的密钥提交仓库或发给他人。IP变更时需要同时更新监听地址和公开地址，并重启服务。正式持久部署请使用 [README](../README.md) 的Compose与持久卷。

## 自己启动局域网实例

在既有数据库、独立密钥和媒体目录配置就绪后，设置自己的局域网IP：

```bash
ENVIRONMENT=development \
LISTEN_ADDR=192.168.2.209:8080 \
PUBLIC_BASE_URL=http://192.168.2.209:8080 \
./bin/painter serve
```

使用Compose时，在私有`.env`中同时设置：

```dotenv
ENVIRONMENT=development
APP_BIND_ADDRESS=192.168.2.209
APP_PORT=8080
PUBLIC_BASE_URL=http://192.168.2.209:8080
```

然后执行`docker compose up --build -d`。`APP_BIND_ADDRESS`默认仍是127.0.0.1；公网HTTPS部署继续走Caddy或已有反向代理。

## 浏览器兼容与访问边界

局域网HTTP属于非安全上下文，原生`crypto.randomUUID()`可能不可用。前端统一使用支持`crypto.getRandomValues()`的UUID v4实现，保留密码学随机强度，没有使用Math.random生成请求身份。[randomUUID说明](https://developer.mozilla.org/en-US/docs/Web/API/Crypto/randomUUID)、[getRandomValues说明](https://developer.mozilla.org/en-US/docs/Web/API/Crypto/getRandomValues)。

本地账号不依赖GitHub配置；可选GitHub登录仍需要真实OAuth App配置，回调须改成当前`PUBLIC_BASE_URL/auth/github/callback`。新站安装步骤及服务器安装码命令见 [INSTALLATION_V7.md](INSTALLATION_V7.md)。当前站点已有管理员，访问 `/setup` 只显示安装完成，不重新注册站长。GitHub 配置在后台设置中，真实外部回调尚未验证。

本机127.0.0.1的Cookie及本地草稿属于另一个origin，不会自动出现在局域网地址下；原origin的草稿可先导出，再在新地址继续。

如果另一设备无法连接，检查是否同一局域网、是否启用了Wi-Fi客户端隔离，以及主机防火墙规则。本次未更改系统防火墙；`/etc/ufw/ufw.conf`显示`ENABLED=no`。如以后启用UFW，可由管理员仅放行本子网到该端口：

```bash
sudo ufw allow from 192.168.2.0/24 to 192.168.2.209 port 18080 proto tcp
```

V3切换前备份在仓库外 `/tmp/painter-next-before-v3-20261028/`；V4更新前备份在 `/tmp/painter-next-before-v4-20261028/`。V5候选开始前和最终升级前均已备份，V5完整回退材料在 `/tmp/painter-next-before-v5-final-20261028/`；V6升级前备份在 `/tmp/painter-next-before-v6-20261028/`。V7完整数据库/媒体及V6二进制/私有配置在 `/tmp/painter-next-before-v7-final-20261028/`，候选前备份另保留于 `/tmp/painter-next-before-v7-20261028/`。当前V8完整数据库/媒体与V7程序/私有配置在 `/tmp/painter-next-before-v8-final-20261029/`，克隆验收前备份另在 `/tmp/painter-next-before-v8-20261029/`。0012提交后不得直接换回只认识schema11的V7程序；应向前修复或把完整备份恢复到隔离库后受控切换。上述目录为私有测试回退材料，不应共享、提交或作为长期备份存储。测试目录位于 `/tmp`，重启/清理前应按正式部署文档迁移到持久卷。

V9更新前完整数据库、媒体、V8程序与私有配置位于 `/tmp/painter-next-before-v9-20261029/`；本轮无schema变化，回退程序仍须沿用原数据库与配置，不用候选QA库替换。该目录同样是临时私有回退材料。

V10更新前完整数据库、媒体、V9程序与私有配置在 `/tmp/painter-next-before-v10-20261029/`。本轮不改变schema；仍应使用原库/环境受控回退，不能复制候选QA库。

V11 更新前完整数据库、媒体、V10 程序与私有配置在 `/tmp/painter-next-before-v11-20261101/`。本轮无 schema 变化，原库与配置继续使用；该目录为临时私有回退材料。

V12 更新前完整数据库、媒体、V11 程序和私有配置在 `/tmp/painter-next-before-v12-20261102/`，为临时私有回退材料；本轮无迁移，继续使用原库和配置。

V13 更新前完整数据库、媒体、V12 程序和私有配置在 `/tmp/painter-next-before-v13-20261102/`，同为临时私有回退材料；没有数据库迁移，仍沿用原库与私有环境。

V14 更新前完整备份、原V13程序和私有环境在 `/tmp/painter-next-before-v14-20261102/`。schema13加入两张友链表及审计词汇；原V13程序只认识schema12，不能直接换回。私有 `runtime/painter-v13-schema13` 保留V13界面且识别schema13，已在隔离库验证ready和既有文章，可供受控程序回退；不要用备份覆盖更新期间产生的现场数据。停止临时systemd单元可能使其卸载；本轮已重建同名LAN单元，仍使用原私有环境、二进制路径与IP。

V16更新前完整数据库、媒体、原程序及私有配置在 `/tmp/painter-next-before-v16-final-20261102/`。schema14只新增作者主页表；应使用其中 `runtime/painter-v15-schema14` 做受控程序回退，它保留旧界面及新背景引用保护，已经隔离实际验证。不要直接运行schema13原程序或覆盖更新后的现场数据库。这仍是临时私有回退材料，重启清理前应迁入持久备份。

V17更新前完整备份与原V16程序在 `/tmp/painter-next-before-v17-20261102`。本次schema不变，可保留现场数据库/媒体/配置只回退程序，不使用候选数据覆盖正式数据。该备份仍为私有临时材料。
