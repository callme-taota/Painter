# Steam + PlayStation 游戏墙

Steam 与 PlayStation 使用统一游戏墙，公开页面读取本地快照。实际 PSN 账号由站长通过 HTTPS 后台或服务器 CLI 连接。

## 展示规则

- Steam 和 PlayStation 共同使用首页游戏墙与 `/games`；首页总共最多5条，完整页面显示全部可见记录。
- 可按全部、Steam、PlayStation筛选；位置、时长与标题排序和分页保持当前平台条件。
- PlayStation条目展示平台名称和平台实际提供的 PS4 / PS5 标识；同名游戏在不同平台分别保留，不合并或相加时长。
- 封面、大图、透明标题标识、商店链接均使用实际返回的数据。缺少素材时回退到文字；无时长显示“时长未提供”，真实0分钟保留为0。
- 支持手动添加 PlayStation条目或上传自选封面；同步保留站长修改的展示信息、隐藏状态和排序。

## 连接自己的账号

后台入口：**站点管理 → 首页内容 → 游戏账号 → PlayStation账号**。当前支持一个 PSN 账号。

1. 在自己的浏览器登录 [Sony账号管理](https://www.playstation.com/acct/management/)。
2. 打开 [Sony会话凭据页面](https://ca.account.sony.com/api/v1/ssocookie)，取得自己的 `npsso`。
3. 在 HTTPS Painter后台填入 NPSSO，可选填公开 Online ID 核对身份，连接后点击“同步 PlayStation游戏”。也可开启每6小时自动同步。

NPSSO 是私密会话凭据，不是公开昵称。不要发送到聊天、文章、URL或命令行参数。Painter不会要求Sony密码，也不保存NPSSO；交换后的访问/刷新令牌使用现有服务端密钥加密存储。输入框保存成功后清空，状态接口不回传凭据。

普通局域网 HTTP 后台禁用 NPSSO 输入。服务器本机可使用 CLI，在已经加载 Painter 服务配置的环境下运行：

```bash
# 私有文件仅限当前用户读取，不接受符号链接
chmod 600 /安全路径/psn-npsso
./bin/painter psn-connect --npsso-file /安全路径/psn-npsso --online-id 你的公开ID --auto-sync
```

或通过受保护的标准输入使用 `--npsso-stdin`；默认不开启自动同步。不要把真实凭据写进 shell 命令、shell历史或环境变量。CLI与后台共用同一套加密连接、账号核对和同步机制。连接完成后可移除临时凭据文件。

断开连接会停止同步、清除服务端令牌，并保留游戏快照。相同账号可以重连、修改公开昵称。更换成其他账号前，须清理已有 `psn_sync` 记录，以免混合不同人的游戏数据；没有旧同步记录时允许换绑。

## 数据链路与并发

```text
HTTPS后台 / 本机CLI
  → Sony NPSSO换授权码 → access/refresh令牌
  → /accounts/me核对accountId → 查询公开Online ID
  → AES-GCM加密保存连接 → 排队同步（202）
  → 独立worker领取数据库租约
  → 刷新令牌（需要时立即CAS保存）
  → 分页读取PS4/PS5游戏记录与素材
  → 完整校验、去重、容量检查
  → 事务upsert本地快照 + 更新时间 + 审计
  → 首页/游戏页读取PostgreSQL快照，不实时请求Sony
```

- 数据库：PostgreSQL，迁移21；无SQLite、无新的运行时或独立微服务。
- `homepage_items` 保留历史 `kind='steam'` 集合键，新增 `platform` 和 `platforms`；已有Steam数据只回填平台，不改身份、排序、图片和时长。
- PSN同步键是平台返回的titleId，按 `kind,source,provider_id` 唯一，`source='psn_sync'`。不把PSN字符串编号转换为数字，不生成Steam图片URL。
- `homepage_game_connections` 单行保存账号、加密令牌、绝对过期时间、同步计划、连接版本、凭据版本、租约和脱敏错误。
- 锁序为首页settings → connection → items按id → media按id；全局5000项容量与Steam/手动操作使用相同settings锁。
- 管理API仅owner使用，写入要求CSRF、If-Match与幂等键。同步请求立即排队，独立worker每15秒检查，任务超时120秒、租约180秒；跨进程由数据库租约与CAS保护。
- 外部HTTP不占数据库事务锁；刷新令牌立即保存，之后分页失败也不会丢失已轮换凭据。
- 请求限速、超时、429退避；分页完整成功后才upsert，失败/私密/无数据均保留已有快照，不因本次缺席删除旧游戏。
- 图片只接受已确认Sony HTTPS图片域与受控本地上传，禁止任意来源/重定向抓取。所有公开页面从本地数据库读取。

API：

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/v1/admin/home/psn` | 脱敏连接/同步状态 |
| PUT | `/api/v1/admin/home/psn` | 连接、自动同步配置、断开 |
| POST | `/api/v1/admin/home/psn/sync` | 异步排队同步，202 |
| GET | `/api/v1/games?platform=all\|steam\|psn` | 统一游戏墙 |

## 接口边界

Sony没有在此提供面向博客的一键OAuth产品。当前实现参考社区维护的 [psn-api](https://github.com/achievements-app/psn-api) 与 [PSNAWP](https://github.com/isFakeAccount/psnawp) 的Sony客户端协议，使用原生Go实现。游戏范围为接口返回的PS4/PS5游玩记录，不能承诺完整购买库、PS3/Vita、奖杯或未公开记录。协议测试使用隔离 TLS 模拟服务和独立 PostgreSQL；模拟测试不等于真实账号授权或上游连通性验证。

凭据过期时提示重新连接；上游协议或隐私策略变化可能使自动同步暂不可用，手动管理与已保存游戏墙继续工作。

账号的连接状态、同步时间和错误以后台返回为准；公开入口为 `/games`，管理入口为 `/studio/home`。不要为了检查展示效果向正式站点添加测试游戏。
