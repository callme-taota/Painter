# 个人博客体验与本地账号迭代

2026-10-28。本轮仍是 Go 模块化单体 + PostgreSQL 18，不引入生产 Node、独立搜索服务或 SQLite。旧站代码与原始数据不在修改范围。

## 阅读与导航

产品只使用“标签”这一种分类概念。顶部为旧 Painter logo、文章、标签和搜索入口；关于移到页脚，页脚只保留居中的 logo、About 与 2026。右侧浮动控件按 home/article/other 场景配置，可显示主题、回顶、到底、点赞与账号入口；点赞使用真实互动接口，游客通过登录继续。

首页的主文章与封面构成分层视差，借鉴 [Scroll-driven Animations 的 cover-to-header](https://scroll-driven-animations.style/demos/cover-to-fixed-header/css/) 等原生滚动演示。默认内容始终可见，减少动态效果时采用静态版面，文章正文不依赖动画才能阅读。

全局搜索以 `Ctrl/Cmd+K` 唤起浮层，支持建议词/文章、键盘选择、关闭后恢复焦点、中文组合输入及取消旧请求。浮层不包含筛选，筛选只在 `/search` 结果页。没有JavaScript仍可访问独立搜索页。

## 统一标签的数据兼容

`0006_unified_tags.sql` 创建 `category_tag_map`、`tag_aliases` 和两个 effective 视图：

- `effective_draft_tags(post_id,tag_id)`：草稿标签与历史分类的统一投影。
- `effective_revision_tags(revision_id,tag_id,tag_name)`：已发布/历史修订的统一标签投影。

已有分类迁移为标签，按规范化名字合并重名，保留旧ID映射。旧修订的正文、hash和分类快照不被改写。新草稿不再写分类；旧字段只用于可信旧包和历史恢复兼容。旧分类写API返回410，`/categories`重定向到`/tags`，旧筛选ID映射到对应标签。公开列表、搜索、标签计数与建议统一读取effective关系，迁移后旧包导入也会同步映射。

## 本地账号

`0005_local_accounts.sql` 增加凭据表，以用户ID关联既有用户，密码只存带独立随机盐的Argon2id哈希。实现依据 [OWASP Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html) 的参数建议，当前19MiB、2次迭代、并行度1，全进程最多两个哈希任务。

流程：获取匿名nonce及CSRF → 精确Origin与CSRF校验 → 限速及输入校验 → 哈希验证/创建 → policy维护门内复核凭据、用户状态与注册开关 → 轮换opaque session及nonce。注册只能创建reader，不能提交角色。密码15–1024个Unicode字符且不超过4096字节，不截断；用户名按明确ASCII规则规范化并使用唯一约束。

改密校验当前密码，取得排他policy门后更新哈希并撤销会话。普通写在取得用户锁后再用独立语句复查会话，避免等待锁期间沿用已撤销会话的旧快照。离线重置通过同一维护/策略锁协调；没有公开默认管理员或重置后门。

生产默认关闭注册；首位站长由服务器操作员通过离线CLI建立。GitHub保留可选兼容，界面默认本地账号。无SMTP确认消费者时不虚构“邮件已发送”；忘记密码的实际恢复步骤见 [local-accounts.md](local-accounts.md)。本地测试密码通过会话交付，不进入仓库、镜像或测试报告。

## 搜索建议

`GET /api/v1/search/suggestions?q=` 返回 `{items:[{kind,label,url}],query}`，固定最多6篇文章与4个标签。空查询返回真实最新公开内容与有公开文章的标签；非空查询规范化并按字面匹配，支持中文短词与全角字符。没有任意HTML片段。

建议只读取当前 `public_posts`、对应 `search_documents` 与effective公开标签关系。草稿独有标题、只被草稿使用的标签和撤回内容不进入建议。请求使用有界并发、350ms数据库超时与只读一致性事务，和正常搜索共享资源预算。

## 文章封面的全链路

`0007_article_covers.sql` 在草稿与不可变修订增加cover配置：

```json
{"mode":"auto","template":"editorial","color":"ink"}
```

mode为auto/upload/none；模板editorial/geometric/minimal，颜色ink/moss/clay/plum。上传模式额外保存 `media_id`。默认auto；没有外部AI服务依赖。

编辑器将cover与标题、摘要、正文和标签一并进入本地恢复、待保存快照、CAS及冲突比较。服务端验证封面枚举与媒体权限；封面图片和正文图片分别输入，但合并进入draft/revision媒体引用集合，受相同的GC、删除及发布保护。旧包导入与历史恢复携带cover，只有封面不同也构成新的工作副本差异。

生成预览：`POST /api/v1/editor/cover-preview` 接收当前标题与配置，经作者权限/CSRF校验后产生1200×675安全SVG。标题进行XML合法字符过滤与转义，样式只选服务端枚举，不允许脚本或外部资源。

发布：校验当前草稿版本 → 将标题和cover复制到不可变修订 → 写媒体引用 → 原子切换公开指针与搜索投影。公开 `GET /api/v1/posts/{id}/cover?revision=...` 只允许当前公开修订，撤回/更换发布后旧公开封面不能继续借旧URL获得权限。工作室封面读取独立鉴权；上传封面也经过同一公开指针验证。

备份的数据库dump保存cover和映射表，媒体manifest覆盖所有ready对象。实际隔离恢复测试已比较公开auto与私有upload配置、媒体hash及引用；不把“生成封面预览”当成“已发布封面”。

## 站点开关

`0008_site_experience.sql` 增加 `rss_enabled`（默认false）与 `floating_controls`。RSS关闭时 `/rss.xml` 返回404；后台开关可启用已有feed实现，公开界面暂不宣传RSS入口。

悬浮控件是每场景的有序动作数组；空数组可隐藏，遗漏场景继承默认。只接受已知场景/动作，拒绝重复、脚本、任意选择器/URL与不适用的文章动作。站长使用现有版本CAS及幂等设置接口修改，旧客户端遗漏新字段会保留原值。

## 验证与环境

后端使用专用PostgreSQL 18临时库，覆盖原有业务与新凭据、并发撤销、标签迁移、封面权限、备份恢复、建议隐私和RSS/浮控设置。独立安全评审及回归见 [security-review.md](../tests/artifacts/v3/security-review.md)，综合结果与浏览器证据见 [验收记录](VALIDATION.md)。

当前本地新版从旧预览的真实备份恢复到独立测试库和媒体目录，旧库保留；测试账号通过真实离线CLI创建，再通过正常登录获取会话。最终切换仍使用指定LAN IP，未开放数据库网络端口、未改防火墙、未发布公共镜像或公网服务。
