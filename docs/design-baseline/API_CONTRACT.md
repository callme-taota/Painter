# Painter Next：接口与事件契约

版本：原始设计稿 2.0，2026-10-21。与 [技术设计](TECHNICAL_DESIGN.md) 和 [schema.sql](schema.sql) 配套。以下保留初始方案的 API 与事件语义，不是当前应用的完整路由清单。文中的首期、待实现、身份方式与表名均属于成文时的方案；当前行为以 [实现架构](../ARCHITECTURE.md)、[根迁移](../../migrations)、实际代码和相应功能合同为准。

## 1. 统一约定

- 业务 API 根为 `/api/v1`，OAuth 为 `/auth/*`，公开 HTML 路由与 API 分开。旧 86 个路由按用户结果迁移，不维持其所有命名和错误返回方式。
- 唯一数据库为 PostgreSQL 18，使用 UTF8 与支持中文字符分类的数据库 LC_CTYPE（部署基线 C.UTF-8）；Go应用与PostgreSQL是两个运行服务（PG内部多进程），不增加外置搜索服务。
- JSON 使用 snake_case，资源 ID 是字符串，时间字段为 RFC 3339 UTC 字符串；数据库 BIGINT UTC 毫秒由边界层转换，统计日 DATE 转为 YYYY-MM-DD。所有列表均明确自己的可见性。
- 成功使用真实 `200/201/204`；错误使用真实 HTTP 状态，格式为 `{"error":{"code":"VERSION_CONFLICT","message":"…","request_id":"…","details":{}}}`。
- 列表 `{items,next_cursor,has_more}`，默认 limit=20，最大50；普通列表按 `(created_at,id)` 或明示排序键做 keyset，搜索见单独规则。cursor 为签名的不透明值，绑定筛选和排序。
- Cookie 会话写请求必须通过 CSRF + 允许的 Origin 检查；公开 GET 不写业务状态。客户端不提交可信 user_id / role / server_timestamp。
- 标注“版本”的命令使用 `If-Match: "<version>"`；首次 create 除外。缺少条件返回 `428 PRECONDITION_REQUIRED`，并发冲突 `409 VERSION_CONFLICT`。
- 标注“幂等”的创建 / 业务命令使用 `Idempotency-Key`，值为稳定随机 UUID，最大128字符。作用域为当前主体+路由，保留7天；相同key不同请求hash返回409。关系 PUT/DELETE 自身幂等。
- 输入 JSON 默认最多1 MiB；头像 / 文章图10 MiB；analytics更小，另列。标题1～200字符，摘要最多500字符，正文最多1 MiB UTF-8，最多20标签；超限由服务端拒绝。
- 私有 API / 工作台 / 预览 `Cache-Control: private, no-store`；公开内容首期不共享缓存，静态构建资产长期 immutable 缓存。
- 私有资源不存在或调用者无权查看时统一404；未登录401；已知操作权限不足403。所有写接口有实际后端权限，不靠隐藏按钮。

角色：`visitor` 未登录；`reader` 可互动；`author` 可管理本人文章并互动；`owner` 有站点管理和明确的全站审核权限。disabled 用户不允许写入。作者不是独立身份系统。

## 2. 身份和会话

| 方法 / 路由 | 权限 | 请求 / 输出与副作用 |
| --- | --- | --- |
| GET `/auth/github/start` | 访客 | return_to站内路径；生成一次性state与PKCE，302到GitHub；改变的是临时认证状态，不是业务资源 |
| GET `/auth/github/callback` | OAuth流程 | code/state+绑定cookie；消费state→外部兑换→用户身份和session事务→303；失败不设置正式会话 |
| GET `/session` | 任意 | 未登录返回 `{authenticated:false}`；登录返回安全用户DTO、能力、CSRF token、session到期；no-store |
| DELETE `/session` | 登录+CSRF | 撤销当前会话、清cookie，204；已退出重复请求保持退出效果 |
| GET `/me/sessions` | 登录 | 当前用户会话清单，不返回token/hash |
| DELETE `/me/sessions/{id}` | 本人 | 按独立management_id撤销指定会话；不暴露token/hash |
| GET `/users/{id}` | 任意 | 昵称、头像、公开简介、文章 / 关注计数；不返回email、phone、OAuth细节 |
| GET `/me` | 登录 | 私有资料及角色、版本 |
| PATCH `/me` | 本人，版本 | username、display_name、bio、email、phone、avatar_media_id；If-Match匹配users.version，只更新本人资料并增加version |

首期没有密码、邮件验证码、短信登录和找回密码 API。资料 email 不代表已验证，更不能作为账号合并依据。提供方后续绑定需 recent-auth、CSRF 和 `intent=link` 的单独流程；不开放接受任意 provider subject 的普通 HTTP 绑定端点。旧用户认领采用有审计的本机导入映射 / 单次凭据。

GitHub端点、issuer命名空间、回调URL来自受信配置；不接收客户端任意URL作为OAuth服务器。API适配器固定GitHub官方支持的API版本，并将其纳入升级测试。

## 3. 公开内容与检索

| 方法 / 路由 | 请求 | 返回 / 规则 |
| --- | --- | --- |
| GET `/site` | 无 | 站名、简介、公开社交链接、ICP、可见功能开关；无SMTP与秘密 |
| GET `/posts` | author_id/category_id/tag_id/cursor/limit | 仅当前发布文章；排序默认首次发布时间DESC、id DESC；不读草稿 |
| GET `/posts/{id}` | 无 | 发布修订DTO，title/summary/sanitized_html/toc/author/taxonomy/revision_id/read_receipt；无自动read++ |
| GET `/posts/{id}/metrics` | 无 | 当前可见评论和赞数、有效阅读数与legacy阅读分列 |
| GET `/categories`、`/tags` | cursor/limit | 未归档主题及公开文章数 |
| GET `/categories/{id}`、`/tags/{id}` | 无 | 名称、说明、公开文章数；文章通过/posts过滤 |
| GET `/search` | q/author_id/category_id/tag_id/sort/cursor/limit | 全文搜索结果和转义高亮片段、effective_sort、query_hash、generation、search_instance_id；无私有数据 |
| GET `/posts/{id}/comments` | cursor/limit | 仅visible平铺评论，按created_at/id ASC；首屏可直接SSR |
| GET `/users/{id}/posts` | cursor/limit | 同/posts?author_id，仅公开 |

搜索规范：索引与查询共用同一Go函数，固定顺序为`NFKC→Unicode case fold→再次NFKC→Unicode空白压缩`，锁定`golang.org/x/text`版本；norm版本纳入renderer_version与搜索重建版本。q规范化后为1～64个Unicode字符，再按Unicode空白拆为最多8个不重复词；中文连续输入不作词典分词。`search_documents.title/summary/body`保存同一函数的规范化结果，列使用确定性`pg_c_utf8`，各建`GIN (... gin_trgm_ops)`。数据库LC_CTYPE也必须支持中文，启动检查中文`show_trgm`非空；仅给列加collation不能修复数据库字符分类错误。

每词分别构造匹配条件，词间AND，同一词在title/summary/body任一列出现即可：

- 词长≥3：参数化`column LIKE $pattern ESCAPE '!'`；服务端将`!/%/_`依次转义后加首尾`%`，用户符号按字面处理，不开放正则或数据库查询语法。
- 词长1/2：`strpos(column,$term)>0`，有意使用短词扫描；不能声称此条件使用GIN。查询不对索引列临时再包一层不匹配索引定义的`lower()`。
- pg_trgm也支持ILIKE，但其大小写规则依locale；本契约选择预规范化+LIKE以保持长短词一致。中文是Unicode子串匹配，不是语言学分词。长度≥3但没有可提取字母数字trigram的标点词仍可能扫描；小表或高命中查询也可能由优化器选择顺序扫描。索引是否有效以真实规模下`EXPLAIN (ANALYZE,BUFFERS)`验证，不用强制关闭seqscan来证明性能。[pg_trgm](https://www.postgresql.org/docs/18/pgtrgm.html)、[模式匹配](https://www.postgresql.org/docs/18/functions-matching.html)、[strpos](https://www.postgresql.org/docs/18/functions-string.html)

`sort=relevance`统一按`title_hit_terms DESC,summary_hit_terms DESC,body_hit_terms DESC,published_at DESC,post_id DESC`排列，返回`effective_sort=field_hits_v1`。每项是该字段命中的不同查询词数量，每词每字段只计一次；这是字段优先的词典序，不能改成5/2/1加权总分或相似度距离。`sort=recent`按published_at/id DESC，返回`effective_sort=published_at_desc_v1`。

游标绑定q/filter/sort/query_hash、normalization/ranker版本、全局generation和offset，最多前1000结果，返回分页上限提示。publication改变时在同一事务更新公开搜索投影与generation，GIN由PostgreSQL维护。每次搜索使用显式只读REPEATABLE READ事务，generation核对、当前公开修订过滤、结果和空结果元信息使用同一请求快照；READ COMMITTED下先查generation再另发结果查询不能提供这个保证。游标过期返回409 SEARCH_CHANGED并重开，结束请求即关闭事务，不跨页面保留数据库快照。

只检索当前publication对应的修订并JOIN当前公开视图，草稿不进入projection，旧索引/片段不能绕过可见性。高亮在应用层对当前公开纯文本生成并转义；不把数据库片段当可信HTML。搜索不默认返回精确总数，最多50结果/页；Go deadline与只读事务内`SET LOCAL statement_timeout='500ms'`共同限制执行，必须验证取消后连接可复用。超时503 SEARCH_TIMEOUT，不返回部分结果；空串400 EMPTY_QUERY。公共HTML `/search` 是原生GET表单，禁JS仍可用。

## 4. 创作与发布

| 方法 / 路由 | 权限 | 请求 / 事务效果 |
| --- | --- | --- |
| GET `/me/posts` | author/owner | status=draft/published/deleted/all；本人可找回全部草稿；分页 |
| GET `/me/posts/by-client-id/{client_draft_id}` | author/owner | 查询本人持久创建映射；处理创建响应丢失、幂等窗过期 |
| POST `/posts` | author/owner，幂等 | client_draft_id必填，初始title/summary/markdown/category_id/tag_ids；创建posts+draft，201返回id/version/draft_version |
| GET `/studio/posts/{id}` | 作者或明确授权owner | 工作副本、当前聚合version/draft_version、发布revision、上传引用 |
| PUT `/studio/posts/{id}/draft` | 作者/owner，版本+幂等 | 完整草稿快照、expected_draft_version；CAS保存及++version，内容变化时++draft_version |
| POST `/studio/posts/{id}/checkpoints` | 作者/owner，版本+幂等 | expected_draft_version；规范渲染并创建不可变修订，返回revision_id和新version |
| GET `/studio/posts/{id}/revisions` | 作者/owner | 修订列表、创建原因/时间/作者/渲染版本；不直接返回全部全文 |
| GET `/studio/posts/{id}/revisions/{rid}` | 作者/owner | 指定历史快照，校验rid属于本文章 |
| POST `/studio/posts/{id}/restore` | 作者/owner，版本+幂等 | revision_id；先为当前草稿留检查点，复制旧修订到新工作副本；不发布 |
| POST `/studio/posts/{id}/publish` | 作者/owner，版本+幂等 | expected_draft_version；渲染→事务复核→快照/公开指针/搜索投影/generation/outbox一次commit |
| DELETE `/studio/posts/{id}/publication` | 作者或owner审核，版本+幂等 | 可带reason；删除公开指针和搜索投影并++version；草稿和历史不删 |
| DELETE `/studio/posts/{id}` | 作者或owner审核，版本+幂等 | 回收/软删除，同时撤回；不物理删除修订和媒体 |
| POST `/studio/posts/{id}/recover` | 作者/owner，版本+幂等 | 恢复为草稿，++version；不自动重新公开 |
| POST `/editor/preview` | author/owner | markdown+seq；无持久副作用，返回安全HTML/TOC/content_hash/seq/renderer_version |

草稿保存示意（目标合同）：

```http
PUT /api/v1/studio/posts/42/draft
If-Match: "7"
Idempotency-Key: 09bd8ba4-73d0-4a81-a113-85a39fa24f58
X-CSRF-Token: <from-private-session-endpoint>
Content-Type: application/json

{"expected_draft_version":4,"title":"文章标题","summary":"摘要","markdown":"# 正文","category_id":"2","tag_ids":["3"],"media_ids":[]}
```

响应 `{"post_id":"42","version":8,"draft_version":5,"ack_mutation_id":"…","saved_at":"2026-10-20T12:00:00Z"}`。没有内容变化可保持draft_version但创作命令version仍增加。发布响应额外返回revision_id、published_url；失败不清空编辑器。

ack_mutation_id原样回显该请求的Idempotency-Key。客户端在发送前保存key→本地seq映射，以此确认保存的是哪一份快照；它不代表服务器已接收后续输入。users.version只用于资料编辑CAS；last_login/last_seen类时间更新不改变资料version，角色与封禁由独立授权命令检查并撤销会话。

媒体引用由服务端解析正文并与media_ids核对，不允许传入与实际内容不一致的引用来绕过GC或授权。创建client_draft_id在本地会话中持久化，数据库长期唯一；作者变更所有权不属于首期普通编辑命令。

### 4.1 PostgreSQL事务与锁顺序

默认写事务为READ COMMITTED。进入事务前可预渲染/解析，但事务内必须重新验证当前授权、If-Match与draft_version；一条普通SELECT的结果不能代替锁或CAS。更新用`UPDATE ... WHERE version=$expected RETURNING ...`判定成功，等待其他写者后仍需按最新行判断。

凡涉及相同资源的命令采用一致锁顺序：幂等事务锁→policy_barrier(id=1)策略行屏障→涉及的users行按ID排序→posts及draft/publication按文章ID排序→评论、分类/标签按ID排序→media按ID排序→从属关系/outbox。授权users行取FOR SHARE，资料/角色等修改取FOR UPDATE；主题快照读取取FOR SHARE。首次取得最强必要锁，不在持有后序锁时升级前序锁。资料/禁用/身份绑定同时复核角色与disabled状态；普通写命令对policy_barrier取FOR SHARE，角色/禁用/owner/站点策略等管理命令取FOR UPDATE并递增version；必须在users之前用独立SQL语句取得该行锁，不能等触发器执行时在已持有users锁后补拿。创建不存在的外部身份还必须依赖唯一约束与ON CONFLICT，必要的身份协调事务锁在域行锁之前取得，而不是只锁一个不存在的查询结果。事务重试保持同一业务幂等key。[READ COMMITTED](https://www.postgresql.org/docs/18/transaction-iso.html)、[显式锁](https://www.postgresql.org/docs/18/explicit-locking.html)

保存、发布、撤回、删除及评论/点赞/收藏/阅读历史写入，都先锁对应posts行，再用新语句复核publication及删除状态；媒体引用修改先锁父文章/草稿，再按media ID锁定并验证ready。GC只锁媒体、再用新语句核对所有引用，不倒序锁文章；标deleting后提交，再做文件I/O。遵循这些协议决定“撤回先提交则后续互动被拒绝”的实际时序；FK本身不能证明文章仍公开或媒体仍ready。

事务不等待GitHub、文件或SMTP I/O。死锁/可重试隔离错误仅有界重试整个事务，不在失败事务里继续SQL；客户端版本冲突保持409。数据库锁等待/连接池超时返回503 WRITE_BUSY，不能以过载为由接受未提交的保存。

## 5. 互动与个人空间

| 方法 / 路由 | 权限 | 规则 |
| --- | --- | --- |
| GET `/me/post-states?ids=…` | 登录 | 有上限批量读取本人like/bookmark状态，no-store |
| PUT / DELETE `/posts/{id}/like` | reader+ | 唯一关系存在/不存在，重复不加计数，不用toggle |
| PUT / DELETE `/posts/{id}/bookmark` | reader+ | 同上；收藏列表仅本人 |
| GET `/me/bookmarks` | 本人 | 返回当前公开收藏文章；撤回项不暴露其正文或题目 |
| POST `/posts/{id}/comments` | reader+，幂等 | body纯文本1～5000字符；事务校验当前publication及站点开关 |
| DELETE `/comments/{id}` | 本人/owner | 软删除，重复删除维持删除效果；管理动作审计 |
| PUT / DELETE `/comments/{id}/like` | reader+ | visible评论且文章公开；取消赞不是踩 |
| PUT / DELETE `/users/{id}/follow` | reader+ | 目标可用、非本人；幂等 |
| GET `/me/following`、`/me/followers` | 本人 | 身份取session，不按任意user_id查私有列表 |
| GET `/me/history` | 本人 | 按last_visited_at/post_id DESC；只显示当前可见内容 |
| DELETE `/me/history` | 本人 | 清空本人记录，幂等 |

评论首期不提供回复树、编辑、举报、通知；owner可在管理端隐藏评论并记录原因。评论新增/删除/隐藏、当前赞数均由事实表查询或可对账投影得出，与浏览器遥测分离。

## 6. 媒体

| 方法 / 路由 | 权限 | 规则 |
| --- | --- | --- |
| POST `/media` | 登录，幂等 | multipart；author可文章图，reader可本人头像；10MiB；返回media_id/state/private_preview_url |
| GET `/me/media` | 本人 | 已上传对象状态/占用/引用，不向普通用户暴露磁盘路径 |
| GET `/media/{id}/status` | 所有者/owner | 上传重试和故障恢复查询 |
| DELETE `/media/{id}` | 所有者/owner | 仅完全无引用媒体可标记删除，引用中409 IN_USE |
| GET `/media/{id}`（站点根路径） | 动态校验 | 当前公开引用可读，私有引用需编辑权；流式字节，no-store；不暴露objects目录 |

上传staging→验证→fsync/rename→ready；ready变更与引用写入按媒体行锁协议执行。失败通过状态与巡检补偿。接口不在DB事务内等待文件I/O；ready前无法发布；清理检查工作副本、所有修订及头像引用。备份CLI将PostgreSQL逻辑快照与媒体manifest关联，并在备份窗口禁止相关GC，不能把数据库目录复制当作可恢复备份。

## 7. 管理与运行接口

| 方法 / 路由 | 权限 | 规则 |
| --- | --- | --- |
| GET / PUT `/admin/site-profile` | owner，写带版本 | 站点品牌/介绍/社交链接/ICP；公开DTO独立 |
| GET / PUT `/admin/settings` | owner，写带版本 | 注册/评论/互动/analytics开关及私有通知配置；secret只写，回包仅configured状态 |
| POST `/admin/categories`、`/admin/tags` | owner，幂等 | name/description；唯一性校验 |
| PATCH `/admin/categories/{id}`、`/admin/tags/{id}` | owner | 名称/说明/归档；引用中不硬删除 |
| GET `/admin/users` | owner | 分页、当前角色/disabled；不返回session/token |
| PATCH `/admin/users/{id}` | owner，幂等 | role/disabled；不允许去掉最后有效owner；撤销必要会话 |
| GET `/admin/comments` | owner | 管理可见性/文章过滤，不对公开接口放宽权限 |
| PUT `/admin/comments/{id}/visibility` | owner，幂等 | visible/hidden及reason；审计 |
| GET `/admin/analytics` | owner | day_from/day_to/resource/metric；data_as_of/coverage/legacy边界 |
| GET `/me/analytics` | author | 仅本人文章的聚合，不含个人访客身份明细 |
| GET `/admin/storage`、`/admin/jobs` | owner | 媒体/数据库占用、备份、未聚合最老事件时间、worker lag与dead任务，不暴露密钥；seq仅诊断，不是已处理高水位 |
| GET `/livez`、`/readyz`（根路径） | 运维 | readiness只在初始化完毕且存储可用时200；不能以固定sleep替代 |

性能profile、详细metric与运行错误只从受限运维入口读取。初始导入、账号映射、备份和restore-check为本机CLI，不提供无需本机权限的HTTP恢复后门。

## 8. 埋点合同

端点 `POST /api/v1/analytics/events`，同源、允许的Origin、单请求最多50条且64KiB；事件级属性白名单。浏览器来源不可视为认证业务事实。批次提交后才ACK；响应 `{accepted:3,duplicate:1}`。任一事件格式不合规则整批400并返回索引/错误码，已重复项不造成400。

```json
{
  "events": [{
    "event_id": "9dcfe2a0-5a28-4d3e-998a-02c1416c9451",
    "schema_version": 1,
    "event_name": "page_view.v1",
    "page_instance_id": "cdf18eca-b5bf-4089-8b35-abf0ebf25bcd",
    "post_id": "42",
    "revision_id": "revision-opaque-id",
    "client_at_ms": 1792500000000,
    "read_receipt": "server-signed-post-revision-expiry",
    "props": {}
  }]
}
```

| 事件 | props白名单 | 语义去重 / 验证 |
| --- | --- | --- |
| page_view.v1 | 无 | page_instance_id仅一次；首次可见；文章事件带服务端read_receipt |
| post_read.v1 | duration_ms（10000～600000） | 每page_instance一次；同上验证post/revision；列映射duration_ms |
| search_submit.v1 | query_rune_count、result_count、duration_ms | search_instance_id必填；result_count是当前响应展示条数0～50，不是全文精确总数 |
| search_result_open.v1 | result_rank | search_instance_id必填；post当前公开且revision合法；rank 1～1000 |
| editor_open.v1 | 无 | 作者登录，服务端决定traffic_class；可关联本人文章 |
| editor_error.v1 | editor_operation、error_code | operation_id必填；operation枚举load/save/publish/unpublish/upload/preview/delete；error_code白名单 |

search_instance_id、operation_id为事件顶层可选字段，按上表条件必填；props逐列映射schema中的同名白名单列，未知字段拒绝，不存任意JSON。normalized_path由服务端按受限page路径 / post ID归一，不保存query/fragment/referrer。client_at超出24小时补传窗或明显未来返回400 STALE_EVENT，统计日始终由服务端接收时间按Asia/Shanghai计算。用户禁用分析时不设置追踪标识；无标识可计PV但不虚构UV，报表提供标识覆盖率。

文章SSR/API输出签名read_receipt（post_id、revision_id、issued_at、expires_at，最长24小时）。上报核验后丢弃receipt，只存已校验的ID；阅读期间发布了新版本时，旧修订receipt仍可证明当时确实公开。若文章已撤回/删除则不再接受其公开事件或写历史；checkpoint修订不能靠猜ID上报。客户端行为仍可被伪造或丢失，签名不将其变成业务事实。

服务端从受签名随机第一方标识派生 `visitor_day_key`，客户端不能指定hash或user_id。事件具有持久ingest序号，但PostgreSQL sequence分配顺序不是提交顺序，不用`seq>last_seq`作为消费条件。UV对site和post各自去重。首期不做跨日cohort或月UV；daily UV不可相加。原始事件30天、去重集合35天、日表400天。

### 8.1 浏览事件聚合

采集端批量写raw并COMMIT后才ACK，`event_id`与语义唯一键负责重试去重。aggregation worker在短事务内对`aggregated_at_ms IS NULL`的有界批次执行`FOR UPDATE SKIP LOCKED`；同事务完成日指标delta、site/post独立visitor集合和逐行aggregated标记，随后COMMIT。崩溃会一同回滚，不需要把仅在内存的认领当成功。多worker按`day_key,resource_key,visitor_day_key`确定顺序更新聚合键，避免交叉锁死；UV只在去重INSERT确实新增时加1。

不使用seq高水位判定完成：某事务先取得小seq后迟提交，必须仍被`aggregated_at_ms IS NULL`扫描到；last_seq只能是诊断数字。清理raw须满足保留期且aggregated非空，不能按`seq<=last_seq`删除。报表新鲜度来自最老未处理接收时间/待处理数量，不能由最大已处理seq推断。[SKIP LOCKED适用边界](https://www.postgresql.org/docs/18/sql-select.html#SQL-FOR-UPDATE-SHARE)

### 8.2 业务事实与outbox lease

业务outbox最小字段：`event_id/schema_version/event_type/aggregate_type/aggregate_id/aggregate_version/actor_id/occurred_at_ms/payload`；payload每事件固定schema，不含Markdown全文或凭据。业务变更与outbox插入同一事务。每个投递目标有稳定dedupe_key，内部指标与外部通知不混用一个无独立幂等边界的完成标记。

1. 短事务用`FOR UPDATE SKIP LOCKED`选择到期pending任务或过期processing任务，写入新的随机lease_token、数据库时钟计算的lease_until与attempts，COMMIT后才执行任务。
2. 纯内部指标consumer在另一个短事务锁定并验证当前token/未过期lease，先向`outbox_consumptions(consumer,event_id)`插入幂等收据，仅首次插入才原子增加`business_daily`；收据、指标与任务完成状态一起提交。失败回滚，不能先done再写报表。
3. SMTP/文件/HTTP等I/O在事务外执行，向支持幂等的目标发送稳定event/投递ID。结束时仅以`id+state=processing+lease_token+未过期lease`条件更新done；续租、失败退避也必须校验token。过期旧worker不能确认新worker已重领的任务。
4. 超时/崩溃由lease到期恢复；达到上限进入dead并留可诊断错误。进程在外部目标成功后、done提交前崩溃，可能重复投递，所以外部语义为at-least-once；不声称SMTP exactly-once。

SKIP LOCKED只用于队列分工，不能用于公开分页、版本检查或其他需要完整结果的查询。

## 9. 关键错误码与恢复动作

| HTTP / code | 客户端动作 |
| --- | --- |
| 400 VALIDATION_FAILED / EMPTY_QUERY / STALE_EVENT | 展示字段错误，不盲目重试；过期遥测丢弃 |
| 401 SESSION_EXPIRED | 保留编辑恢复点，重新登录后比较版本 |
| 403 PERMISSION_DENIED / CSRF_FAILED / REGISTRATION_CLOSED | 更新会话能力 / 状态；不改用其他user_id绕过 |
| 404 NOT_FOUND | 对无权及不存在资源统一处理 |
| 409 VERSION_CONFLICT | 暂停自动保存，获取服务器版本，人工合并 |
| 409 IDEMPOTENCY_KEY_REUSED | 请求内容有变，明确新的用户动作后生成新key |
| 409 IN_USE / MEDIA_NOT_READY / SEARCH_CHANGED | 等待上传 / 解除引用 / 重开搜索 |
| 413 PAYLOAD_TOO_LARGE | 减少正文、文件或埋点批次 |
| 428 PRECONDITION_REQUIRED | 先读取版本，勿直接补一个猜测值 |
| 429 RATE_LIMITED | 按Retry-After退避，不阻塞阅读正文 |
| 503 WRITE_BUSY / SEARCH_TIMEOUT / STORAGE_UNAVAILABLE | 连接池/锁等待/查询期限或存储故障；按Retry-After有界退避，写命令保持原key，不把失败显示成成功 |

## 10. 兼容与演进

HTML旧链接做301映射，旧移动端 / 第三方API若存在独立调用方，需要迁移盘点后单独提供限期适配层；不能因为旧前端曾导出87个函数就承诺所有未用接口永久兼容。新API可增加可选字段；删除字段、改变可见性或事件含义使用新版本。事件消费者必须拒绝未知schema或进入dead待处理，不能静默误算。
