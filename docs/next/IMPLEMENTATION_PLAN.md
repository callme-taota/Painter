# Painter Next：建设、迁移与验收计划

版本：2026-10-21，PostgreSQL方案。以 [技术设计](TECHNICAL_DESIGN.md) 为执行基线。实施时创建独立 `painter-next` 仓库，旧站作为功能与迁移来源。工期是单人专注开发的估算，在骨架验证后重估，不作为交付承诺。

## 1. 阶段及依赖

| 阶段 | 交付物 | 进入下一阶段的门槛 | 估算 |
| --- | --- | --- | --- |
| P0：技术验证 | 新仓库、版本锁、PostgreSQL 18与pgx连接池、迁移runner、Vite→embed、SSR页面、pg_trgm/GIN、两服务联合资源基线 | 无Node运行；PG真实schema/Unicode locale与1/2/3字搜索实测；Go+PG部署可复现 | 3～5天 |
| P1：身份与站点壳 | GitHub/PKCE、会话/CSRF、owner、角色、DTO、Liquid Glass token及响应式壳 | state不可重放；关闭注册不阻断配置owner；键盘/深浅色可用 | 3～5天 |
| P2：内容链路 | 草稿、修订、发布撤回、回收恢复、主题、SSR详情/列表、媒体状态机 | 行锁顺序/CAS/幂等；草稿与媒体不泄露；发布/公开搜索投影/GIN/outbox原子 | 5～7天 |
| P3：编辑与发现 | CM6、串行保存、IME、恢复点、规范预览、上传；中文搜索；SEO/RSS/sitemap | 首次创建不重复；冲突不覆盖；发布flush；禁JS可读可搜 | 4～6天 |
| P4：互动与个人中心 | 文章/评论赞、平铺评论治理、收藏、关注粉丝、足迹、资料、成员管理 | 旧功能逐条通过；关系幂等；撤回竞争/资料越权/私有历史检查 | 4～6天 |
| P5：运营与交付 | 浏览埋点、业务outbox、日报/UV、审计、清理、占用、备份、发布包 | ACK/commit一致；SKIP LOCKED/lease与聚合重放不重计；PG+媒体异机恢复；合计资源目标实测 | 3～5天 |
| P6：迁移与上线 | 导入器、ID/身份/媒体映射、旧URL、演练、切换与回退 | 两轮全量演练；数量/内容/权限核对；开放写入后回退预案 | 3～5天 |

合计约25～39个专注工作日；OAuth配置、历史异常、界面打磨和真实硬件测量可能改变估算。P2媒体与内容实现可并行，但必须先共享同一行锁与引用协议，P3界面细化可在P1后启动，但不能跳过P0直接铺全部页面。

### P0 必须完成的数据库与部署探针

- PostgreSQL 18是唯一数据库。发布包包含Go服务与独立PG服务的启动/健康检查；也可连接受支持外部PG。运行时不需要Node；Go与PG是两个服务，PG内部包含多个进程。数据库和媒体均持久化，PG不直接暴露到公共网络。
- 固定Go/pgx、PG完整版本和镜像摘要；建立UTF8、Unicode LC_CTYPE（基线C.UTF-8）数据库。实际检查`server_version/server_encoding/lc_ctype`、`pg_trgm`扩展及`show_trgm('中文测试')`非空；列collation不能替代数据库locale。扩展由迁移角色安装，运行角色只有所需DML/函数权限。
- 直接在真实PG执行schema与集成测试。迁移runner采用部署锁/版本与checksum，不能在每次HTTP请求或多个实例启动时竞争DDL。PG不可用、迁移未完成、中文索引前置失败时ready非200。
- 联合资源目标为1vCPU/1GiB，初始容器预算Go 256MiB、PG 512MiB；不承诺512MiB整机部署。pgxpool初始max4/min0，PG max_connections20、shared_buffers64MiB、work_mem2MiB、maintenance_work_mem32MiB；这些是待实测起点，work_mem按操作/并发叠加，不能当作整个PG的总额。
- 为Go连接池、后台任务和PG总连接设上限；配置事务局部statement_timeout/lock_timeout与idle-in-transaction超时；搜索请求使用只读REPEATABLE READ，写命令默认READ COMMITTED并按合同锁序复核。压测记录Go RSS、PG进程/共享内存、排序/GIN维护和WAL/磁盘占用，目标机预算按总量核算。
- 固定同一Go归一化函数与x/text版本：NFKC→Unicode case fold→再次NFKC→Unicode空白压缩；norm版本纳入renderer_version和重建版本，索引/查询/高亮测试共用规范。
- 搜索数据至少覆盖公开中文长文、1/2字符、多词AND、英文/全角、ß/SS与组合字符规范化、`%/_/!`字面、纯标点、很多命中和无命中。三列GIN与短词strpos分别验证；执行ANALYZE后采集自然计划及`EXPLAIN (ANALYZE,BUFFERS)`，不强制索引冒充性能验收。
- 以policy_barrier(id=1)行实现策略屏障：普通写FOR SHARE，管理变更FOR UPDATE并递增version，必须在users之前分开语句取得；同时测试角色/禁用/注册策略变更与普通写入的实际排序。
- 写作/媒体/互动用多连接同步屏障做真实竞争测试；同时验证并发SQLSTATE处理、连接取消后的复用和事务完整重试，单连接顺序脚本不能证明行锁正确。

## 2. 旧能力必须有落点

| 旧领域 / 接口数 | 首个替换版本的验收结果 |
| --- | --- |
| article 27 | 唯一草稿、完整元数据、分类多标签、本人全部文章、版本、公开查询、发布撤回回收、赞/藏 |
| user 20 | GitHub替换旧认证；会话、资料头像、公开作者、私有中心、多作者角色、旧资产认领 |
| category 7 + tag 7 | 公开主题/说明/计数；owner创建改名说明归档；引用中不硬删 |
| comment 6 | 平铺分页、新增、本人删除、赞/取消赞；补owner隐藏与审计 |
| follow 4 + collection 3 | 本人关注/粉丝/收藏页面可达；保留文章收藏关系，不擅自变收藏夹 |
| history 1 | 首次可插入、最近访问更新、足迹页面和清空；预览不记历史 |
| file 1 | 头像/文章图、粘贴拖入、验证、ready、引用授权、旧媒体地址兼容 |
| common 6 + setting 4 | 品牌/ICP/社交链接、开关、成员角色、运行指标；旧统计隔离 |
| 本次新增 | 中文全文搜索、埋点、可靠CM6写作、SEO、RSS/sitemap、移动菜单、部署与恢复 |

旧路由数量是盘点证据，不要求新项目仍有86个端点。合并细粒度标题/正文更新为原子草稿保存是明确改进。手机号登录注释路由等不当作已交付旧功能；密码/验证码是本次明确替换项，其余有效能力不能被静默删除。

## 3. 界面验收

先做首页、文章流、长文、搜索、作者与主题，确定纸面阅读和玻璃导航；再做创作、我的空间和运营。检查375/768/1440px、浅深色、减少动效/透明度、键盘与触控。

原型必须包括长中文、代码、表格、无封面、多标签、空结果、上传失败、保存冲突及权限不足。不能只验收静态首页效果图，也不能以“玻璃更明显”为通过标准。

## 4. 必须验证的时序

| 编号 | 场景 | 通过条件 |
| --- | --- | --- |
| T01 | 创建提交后响应丢失，跨幂等窗找回 | 持久client_draft_id仍定位同一篇 |
| T02 | 保存A在途时继续输入B | A不覆盖B，最终保存包含B；ack只覆盖对应seq |
| T03 | 两标签同时保存，分别使用独立PG连接 | READ COMMITTED下锁posts后复核version；一方CAS成功，另一方409并可比较、合并、导出 |
| T04 | 渲染期间草稿又变 | 发布409；旧线上修订不变 |
| T05 | 图片未ready、rename后DB失败、ready后文件丢失 | 不发布未完成图；巡检补偿；错误可见 |
| T06 | 发布事务的搜索投影/GIN维护/outbox失败 | 指针、投影、索引、generation、事件一起回滚 |
| T07 | 撤回与互动竞争 | 两路径都先锁同一posts行再检查publication；撤回先提交则后续互动拒绝，提交后新读取无泄露 |
| T08 | OAuth拒绝、state重放、兑换断网 | 不建session，可重启登录，同外部ID不重复建人 |
| T09 | 相同email、GitHub改名 | 不自动合并，稳定subject仍归原用户 |
| T10 | raw/聚合各自commit前后终止worker/断开连接 | 采集commit决定ACK；聚合内visitor集合/delta/aggregated标记同退同进，重试不翻倍 |
| T11 | 同访客看两篇、缺cookie、作者预览、爬虫 | 站点UV独立，无标识不假计UV，覆盖率明确 |
| T12 | PG连接耗尽、锁等待、磁盘满、长搜索、遥测高峰 | 超时与背压及时；不把每查询work_mem当全局预算；Go与PG合计资源可控，已确认数据不静默丢失 |
| T13 | pg_dump快照与上传/发布/GC竞争 | 同一备份协议保留媒体；恢复DB引用对象均在manifest且可读，非仅dump命令退出0 |
| T14 | 新站开写后需要回退 | 先冻结保全新写入，不直接换旧库 |
| T15 | 事务A取小seq但晚提交，事务B取大seq先聚合 | A提交后仍按aggregated为空被消费；last_seq不充当WHERE或清理高水位 |
| T16 | 两个outbox worker认领、过期重领、旧worker迟到ACK | SKIP LOCKED不双领同一有效lease；新token接管后旧token无法done/续租；外部目标允许幂等或明确重复风险 |
| T17 | 加媒体引用与GC同时执行 | 双方锁同一媒体，GC新语句复核引用；deleting后无法新增引用，无post/media倒序锁 |
| T18 | search generation读取期间并发发布/撤回 | generation与结果来自同一请求的只读REPEATABLE READ快照；发布提交后开始的新请求用旧cursor明确409，不返回新结果配旧generation |
| T19 | UTF8但LC_CTYPE=C的错误数据库、缺扩展或中文GIN不可用 | 中文show_trgm探针/locale检查让ready失败；正确库下取得实际ANALYZE+BUFFERS计划，不能只验证LIKE结果 |
| T20 | users禁用/降权、OAuth同subject并发、重复Idempotency-Key | 幂等锁后以独立语句锁policy_barrier（普通SHARE/管理UPDATE），再锁users/posts；管理先提交则新写入重新鉴权，同外部身份不建重复用户 |

### P5 任务与聚合的完成门槛

浏览事件worker仅认领`aggregated_at_ms IS NULL`行，短事务内`FOR UPDATE SKIP LOCKED`取有界批次，按稳定聚合键顺序更新指标/独立UV集合，再标记本批已聚合并提交；删除仅限已聚合且过保留期的事件。验收必须覆盖PG sequence与commit乱序，不依赖最大seq证明全部处理。

outbox worker分“短事务认领/新lease_token→提交→事务外I/O→携token和期限的短事务ACK”。续租、失败退避、dead与过期重领均在合同内；内部指标幂等收据outbox_consumptions、business_daily增量与done同事务，外部目标使用稳定幂等ID并承认at-least-once。多consumer的投递完成状态不能互相覆盖，关闭进程/丢ACK后仍可恢复。

## 5. 导入与切换

1. **只读盘点**：导出旧MySQL和媒体副本；核对重复内容、孤儿关系、缺文件、角色0、状态2/3/4、统计范围；保留导出hash。
2. **dry-run**：输出各实体数量、ID冲突、隔离项；没有可信身份映射不认领，不公开异常内容。
3. **幂等导入**：文章尽量保留ID；其余映射。显式导入ID后校准PG identity sequence，避免后续创建碰撞。发布文建import修订和公开搜索投影/GIN，草稿保持私有，关系去重出报告。
4. **数据核对**：按作者/状态对数量、抽样全文hash、关系、头像和图片；旧统计单独legacy，不补造page_view。
5. **身份确认**：owner先固定GitHub ID；其他旧账号凭核验映射/一次凭据认领；无法证明时保留未认领作者。
6. **至少两轮演练**：全量导入、旧URL、GitHub测试账号、故障注入、低配压测、异机恢复；记录耗时及停机预算。
7. **切换**：旧站只读→最终导出核对→导入→新站只读抽查→备份→切代理→明确开写时间。

开写前可以切回旧站；开写后回退是保全与回灌作业。迁移错误、归属不能核对、备份不可恢复或可见性泄露时停止开写，保留导出/日志后修复再演练。

## 6. 完成证据

每阶段提交可运行产物、契约、PG迁移文件及与风险相称的验证。测试覆盖真实不变量与竞争，不为数量给getter/DTO生成无意义用例。性能记录硬件、规模、负载、时长、驱动版本、p95/RSS，空库与本机开发服务不代替目标服务器。

数据库验证状态以更新后的 [VALIDATION.md](VALIDATION.md) 为准。只有真实PostgreSQL执行日志、版本/locale、约束与多连接竞争结果才能作为本方案证据；旧方案的执行结果不得沿用为PG通过证明。后续从P0新项目骨架和迁移runner开始，不在旧源码上直接换数据库驱动。

## 7. PostgreSQL备份与发布检查

使用`pg_dump`自定义归档与`pg_restore`做可移植逻辑备份/恢复，保留角色/扩展/版本和迁移元信息，并关联媒体manifest与禁止相关GC的备份窗口。不能直接复制运行中PG数据目录或把数据库容器卷打包称为完整一致快照；备份完成条件是在独立PG实例恢复后核对内容、作者、发布指针、引用媒体与搜索查询。[官方SQL备份说明](https://www.postgresql.org/docs/18/backup-dump.html)

新站开写后回退须先冻结并保全新增PG数据及媒体；新schema上线遵循增量兼容与后续清理，数据库major升级和应用功能发布单独演练。维护autovacuum/ANALYZE与索引，观察长事务、dead tuple、锁等待、连接与磁盘；GIN重建/并发建索引需单独迁移步骤，不把维护DDL混入业务事务。资源不足时先限制查询/worker并发和遥测批量，不能关闭持久提交来达到漂亮数字。
