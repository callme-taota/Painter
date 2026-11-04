# Painter 接口清单

个人博客的后续架构与实施路线见 [个人博客架构分析与改造计划](PERSONAL_BLOG_ARCHITECTURE_PLAN.md)。

当前工作区包含 **86 个已注册业务接口，分属 11 组**；其中公开 32 个、登录 42 个、管理员 12 个。健康检查、条件注册的调试测试入口和静态资源单列，不计入业务接口数。

业务接口的路径、方法和权限分组以 [Router.go](../painter-backend/server/Router.go) 与 [Server.go](../painter-backend/server/Server.go) 为准。下表覆盖当前未注释的全部业务路由；接口存在不代表产品流程已完成，具体限制列在各组表后。产品结构与优先事项见 [产品功能与抽象](PRODUCT_MAP.md)。

## 调用约定

- 基础前缀为 `/api`。前端使用 `VITE_BASE_API_URL + /api`，带 `withCredentials`。
- 登录以 `painter-session` cookie 和 Redis 会话为准；登录路由组不使用 Authorization token。管理员另检查用户组是否包含管理规则。
- debug 模式的全局调试 cookie 检查也会作用于公开路由和健康检查；“公开”表示无需业务登录。
- 常规响应为 `{msg, ok, data}`。下面的返回列只描述 `data`；业务失败也可能返回 HTTP 200，调用方须检查 `ok`。健康检查返回独立的 `{status}`。
- GET 参数保留代码实际读取的大小写，例如 `Author`、`title`、`Content` 并不统一；POST 记录请求模型的 JSON 字段名。POST 以 JSON 为主要契约，文件上传使用 multipart。
- 常规分页为 `Limit=20, Offset=0`，收藏文章聚合接口改用小写；`tag/list` 默认 `Limit=1, Offset=20`。分页没有统一上限。公共必填检查无法正确判断 int 零值，参数列不代表验证已完善。
- 本清单基于当前代码静态核对，未连接运行中的服务验证请求结果。

### 常见返回对象

| 对象 | 内容 |
| --- | --- |
| ArticleInfo | 文章元数据、标签、文章赞数、评论数、收藏数 |
| FullArticle | ArticleInfo 加正文、作者摘要、分类名和本人点赞收藏状态 |
| ArticleTable | 标题、作者、摘要、分类、阅读数、状态、创建更新时间等元数据 |
| HistoryTable | 用户与文章的访问记录及 HistoryTime，不含文章卡片 |
| CollectionTable | 用户与文章的收藏记录，不是收藏夹 |

## 业务接口总览

| 接口组 | 数量 | 产品职责 |
| --- | ---: | --- |
| `user` | 20 | 账号与身份 |
| `article` | 27 | 文章与创作 |
| `comment` | 6 | 评论与反馈 |
| `collection` | 3 | 文章收藏 |
| `follow` | 4 | 关注与粉丝 |
| `history` | 1 | 阅读历史 |
| `category` | 7 | 分类 |
| `tag` | 7 | 标签 |
| `file` | 1 | 文件上传 |
| `common` | 6 | 站点公共信息 |
| `setting` | 4 | 站点管理 |
| **合计** | **86** | **11 组** |

## 账号与身份

20 个接口。路由入口：[注册位置](../painter-backend/server/Router.go#L16)。

| 方法与路径 | 用途 | 输入 | 返回 data | 访问条件 |
| --- | --- | --- | --- | --- |
| `POST /api/user/create` | 创建账户 [实现](../painter-backend/server/api/User.go#L44) | JSON: username、email、nickname、phonenum(int)、password；headerfield 可选 | userid | 公开 |
| `POST /api/user/login/email` | 邮箱密码登录 [实现](../painter-backend/server/api/User.go#L86) | JSON: email、password | Session；新建 session 时写 HttpOnly painter-session cookie，30 天 | 公开 |
| `POST /api/user/login/email/pass` | 只校验邮箱和密码 [实现](../painter-backend/server/api/User.go#L128) | JSON: email、password | Correct:boolean | 公开 |
| `POST /api/user/login/uname` | 用户名密码登录 [实现](../painter-backend/server/api/User.go#L201) | JSON: username、password | Session；新建 session 时写 30 天 HttpOnly cookie | 公开 |
| `POST /api/user/login/uname/pass` | 只校验用户名和密码 [实现](../painter-backend/server/api/User.go#L243) | JSON: username、password | Correct:boolean | 公开 |
| `POST /api/user/login/check` | 校验请求体中的 session [实现](../painter-backend/server/api/User.go#L18) | JSON: session | userid（字符串） | 公开 |
| `POST /api/user/login/exist` | 检查用户名或邮箱是否已存在 [实现](../painter-backend/server/api/User.go#L520) | 绑定体: Key | Has:boolean | 公开 |
| `POST /api/user/login/send` | 发送邮箱登录验证码 [实现](../painter-backend/server/api/User.go#L540) | 绑定体: Email（也可传用户名） | 空 data 的 success；异步发送，不返回投递结果 | 公开 |
| `POST /api/user/login/mailcheck` | 用验证码免密登录已有账户 [实现](../painter-backend/server/api/User.go#L567) | JSON: Email（邮箱或用户名）、Code(int) | Session；写 30 天 HttpOnly painter-session cookie | 公开 |
| `GET /api/user/self` | 读取当前账户资料 [实现](../painter-backend/server/api/User.go#L460) | 仅登录 cookie | UserTable（ID、UserName、Email、AdminFlag、UserGroup、LastLogin、NickName、PhoneNum、HeaderField、时间字段） | 登录 |
| `GET /api/user/self/full` | 读取个人资料与创作社交概览 [实现](../painter-backend/server/api/User.go#L475) | 仅登录 cookie | UserInfo、ArticleList、ArticleNumber、CollectionNumber、FollowingNumber、FollowerNumber、TotalCount | 登录 |
| `POST /api/user/logout` | 注销当前 session [实现](../painter-backend/server/api/User.go#L274) | 绑定体: session，必须与登录 cookie 相同 | 空 data success，并清除 cookie | 登录 |
| `POST /api/user/update/name` | 更新自己的用户名 [实现](../painter-backend/server/api/User.go#L305) | 绑定体: name | name 回显请求值 | 登录 |
| `POST /api/user/update/email` | 更新自己的邮箱 [实现](../painter-backend/server/api/User.go#L330) | 绑定体: email | email 回显请求值 | 登录 |
| `POST /api/user/update/nickname` | 更新自己的昵称 [实现](../painter-backend/server/api/User.go#L355) | 绑定体: nickname | nickname 回显请求值 | 登录 |
| `POST /api/user/update/phone` | 更新自己的手机号 [实现](../painter-backend/server/api/User.go#L380) | 绑定体: phonenum(int) | phonenum 回显请求值 | 登录 |
| `POST /api/user/update/headerfield` | 更新自己的头像/头图字段 [实现](../painter-backend/server/api/User.go#L405) | 绑定体: headerfield | headerField 回显请求值 | 登录 |
| `POST /api/user/update/passwd` | 尝试通过旧密码修改密码 [实现](../painter-backend/server/api/User.go#L65) | JSON: ID、OldPassword、NewPassword | 空 data success 或通用错误 | 登录 |
| `POST /api/user/update` | 合并更新本人基本资料 [实现](../painter-backend/server/api/User.go#L430) | 绑定体: name、email、nickname、phonenum(int) | name/email/nickname/phonenum 回显 | 登录 |
| `GET /api/user/info` | 读取用户公开主页 [实现](../painter-backend/server/api/User.go#L490) | Query: ID(int) | UserInfo（含 Email、NickName、HeaderField、CreatedAt、LastLogin）、ArticleList、ArticleNumber、CollectionNumber、FollowingNumber、FollowerNumber、Following、Self | 公开 |

### 当前行为与限制

- `/api/user/create`：创建 user 与独立密码记录，密码 bcrypt 哈希；不自动登录；不检查 CanRegister 开关、不校验邮箱验证码、不分配普通用户组；UserGroup 默认为 0；字符串做非空检查，int 的 0 值不能被公共校验发现；用户名/邮箱唯一约束由数据库承担 [依据](../painter-backend/database/User.go#L19) [依据](../painter-backend/database/repository/User.go#L11)
- `/api/user/login/email`：先按邮箱找用户，再 bcrypt 校验；复用 Redis 现有 session 的分支直接返回，不重新 SetCookie；session 映射没有设置 TTL [依据](../painter-backend/cache/User.go#L20)
- `/api/user/login/email/pass`：不创建 session、不设置 cookie
- `/api/user/login/uname`：复用已有 session 分支不重新 SetCookie
- `/api/user/login/uname/pass`：不创建 session、不设置 cookie
- `/api/user/login/check`：校验的是 body session，不是 cookie；不会建立登录态；Redis 查询出错分支返回 ok=true、msg=user session invalid，语义矛盾
- `/api/user/login/exist`：SQL 精确匹配 user_name 或 email；不检查手机号 [依据](../painter-backend/database/User.go#L61)
- `/api/user/login/send`：非邮箱格式则按用户名查邮箱；邮箱格式直接向该地址发码，不先要求账户存在；邮件功能需站点 MailActive 开启；4 位验证码在 Redis 存活 5 分钟；发送失败或停用也可能已向调用方返回成功；未见限流 [依据](../painter-backend/server/mail/Sender.go#L19) [依据](../painter-backend/cache/User.go#L99)
- `/api/user/login/mailcheck`：此接口不只是验证码检查：匹配已有账户并建立登录态；不会创建新用户；复用和新建 session 两条分支都会设置 cookie；验证码校验成功后未删除，可在 5 分钟内复用 [依据](../painter-backend/cache/User.go#L111)
- `/api/user/self`：密码在独立表中，本接口不返回密码 [依据](../painter-backend/database/User.go#L226) [依据](../painter-backend/models/Mysql.go#L8)
- `/api/user/self/full`：文章查询不筛 status，因此包含本人草稿等状态；TotalCount 为文章 ReadCount 总和；收藏数量实现对单个 CollectionTable 结构体 Find 并用 RowsAffected，统计可信度需运行验证 [依据](../painter-backend/database/User.go#L250) [依据](../painter-backend/models/APIs/Response/User.go#L9)
- `/api/user/logout`：从 Redis 删除正反向 session 映射；未带 body session 不能只靠 cookie 注销 [依据](../painter-backend/cache/User.go#L67)
- `/api/user/update/name`：存在实现缺陷：UpdateUserUnix 将值写到仓库模板 tb 而非待保存 row；单字段 UpdateUser 还开启事务但没有提交，不能把成功回包等同于已持久化 [依据](../painter-backend/database/User.go#L121) [依据](../painter-backend/database/User.go#L128) [依据](../painter-backend/database/repository/Query.go#L59)
- `/api/user/update/email`：存在实现缺陷：UpdateUserUnix 将值写到仓库模板 tb 而非待保存 row；单字段 UpdateUser 还开启事务但没有提交，不能把成功回包等同于已持久化 [依据](../painter-backend/database/User.go#L121) [依据](../painter-backend/database/User.go#L128) [依据](../painter-backend/database/repository/Query.go#L59)
- `/api/user/update/nickname`：存在实现缺陷：UpdateUserUnix 将值写到仓库模板 tb 而非待保存 row；单字段 UpdateUser 还开启事务但没有提交，不能把成功回包等同于已持久化 [依据](../painter-backend/database/User.go#L121) [依据](../painter-backend/database/User.go#L128) [依据](../painter-backend/database/repository/Query.go#L59)
- `/api/user/update/phone`：存在实现缺陷：UpdateUserUnix 将值写到仓库模板 tb 而非待保存 row；单字段 UpdateUser 还开启事务但没有提交，不能把成功回包等同于已持久化 [依据](../painter-backend/database/User.go#L121) [依据](../painter-backend/database/User.go#L128) [依据](../painter-backend/database/repository/Query.go#L59)
- `/api/user/update/headerfield`：存在实现缺陷：UpdateUserUnix 将值写到仓库模板 tb 而非待保存 row；单字段 UpdateUser 还开启事务但没有提交，不能把成功回包等同于已持久化 [依据](../painter-backend/database/User.go#L121) [依据](../painter-backend/database/User.go#L128) [依据](../painter-backend/database/repository/Query.go#L59)
- `/api/user/update/passwd`：handler 使用 body ID，未核对等于当前登录用户；ResetPasswordV2 对旧密码重新生成 bcrypt 后直接比哈希，因随机盐通常无法相等；更新分支直接写入 NewPassword 原文而未哈希 [依据](../painter-backend/database/UserPassword.go#L44) [依据](../painter-backend/utils/EncryptionPassword.go#L7)
- `/api/user/update`：不包括 headerfield；该部分逻辑已注释；UpdateUserProfile 虽提交事务，但 UpdateUserUnix 将修改应用于仓库模板 tb，待保存的 row 未被修改 [依据](../painter-backend/database/User.go#L95) [依据](../painter-backend/database/User.go#L128)
- `/api/user/info`：已登录时补充是否关注和是否本人；公开接口返回邮箱与上次登录时间；文章列表筛 status=1，但 ArticleNumber 计入全部状态，数字可能与列表不一致 [依据](../painter-backend/database/User.go#L286) [依据](../painter-backend/database/Article.go#L225) [依据](../painter-backend/database/Article.go#L246)

## 文章与创作

27 个接口。路由入口：[注册位置](../painter-backend/server/Router.go#L110)。

| 方法与路径 | 用途 | 输入 | 返回 data | 访问条件 |
| --- | --- | --- | --- | --- |
| `GET /api/article/get/author` | 按作者浏览已发布文章 [实现](../painter-backend/server/api/Article.go#L322) | query: Author, Limit/Offset（默认20/0） | {ArticleCount, ArticleList: ArticleInfo[]} | 公开 |
| `GET /api/article/get/title` | 按标题检索文章 [实现](../painter-backend/server/api/Article.go#L373) | query: title, Limit/Offset（默认20/0） | ArticleInfo[] | 公开 |
| `GET /api/article/get/content` | 按正文检索文章 [实现](../painter-backend/server/api/Article.go#L398) | query: Content, Limit/Offset（默认20/0） | ArticleInfo[] | 公开 |
| `GET /api/article/get/collection` | 按用户收藏获取文章列表 [实现](../painter-backend/server/api/Article.go#L453) | query: userID, limit/offset（小写，默认20/0） | {ArticleCount, ArticleList: ArticleInfo[]} | 公开 |
| `GET /api/article/get/category` | 按分类浏览已发布文章 [实现](../painter-backend/server/api/Article.go#L423) | query: CategoryID, Limit/Offset（默认20/0） | {ArticleCount, ArticleList: ArticleInfo[]} | 公开 |
| `GET /api/article/get/tag` | 按标签浏览文章 [实现](../painter-backend/server/api/Article.go#L483) | query: TagID, Limit/Offset（默认20/0） | {ArticleCount, ArticleList: ArticleInfo[]} | 公开 |
| `GET /api/article/get/time` | 获取已发布文章流 [实现](../painter-backend/server/api/Article.go#L513) | query: Limit/Offset（默认20/0） | {ArticleCount, ArticleList: ArticleInfo[]} | 公开 |
| `GET /api/article/get/count` | 统计已发布文章数量 [实现](../painter-backend/server/api/Article.go#L537) | 无 | {ArticleCount} | 公开 |
| `GET /api/article/get/self` | 获取当前用户文章列表 [实现](../painter-backend/server/api/Article.go#L348) | query: Limit/Offset（默认20/0） | {ArticleCount, ArticleList: ArticleInfo[]} | 登录 |
| `GET /api/article/get` | 读取文章全文及互动状态 [实现](../painter-backend/server/api/Article.go#L547) | query: ArticleID | FullArticle：文章元数据/正文/标签/作者 MiniUserInfo/分类名/三类互动计数/Liked/Collected | 公开 |
| `POST /api/article/create` | 创建草稿 [实现](../painter-backend/server/api/Article.go#L15) | body JSON: title/content 必填字符串，summary/categoryID/tags[] 可选 | {ID} | 登录 + 发文权限 rule_id=2 |
| `POST /api/article/update/content` | 修改文章正文 [实现](../painter-backend/server/api/Article.go#L54) | body JSON: articleID, content | {} | 登录 + 发文权限 rule_id=2 + 文章作者本人 |
| `POST /api/article/update/summary` | 修改文章摘要 [实现](../painter-backend/server/api/Article.go#L146) | body JSON: articleID, summary | {} | 登录 + 发文权限 rule_id=2 + 文章作者本人 |
| `POST /api/article/update/category` | 修改文章分类 [实现](../painter-backend/server/api/Article.go#L100) | body JSON: articleID, categoryID | {} | 登录 + 发文权限 rule_id=2 + 文章作者本人 |
| `POST /api/article/update/title` | 修改文章标题 [实现](../painter-backend/server/api/Article.go#L232) | body JSON: articleID, title | {} | 登录 + 发文权限 rule_id=2 + 文章作者本人 |
| `POST /api/article/update/status/dart` | 将文章设为草稿（撤回发布） [实现](../painter-backend/server/api/Article.go#L738) | body JSON: articleID | {} | 登录 + 发文权限 rule_id=2 + 文章作者本人 |
| `POST /api/article/update/status/public` | 发布文章 [实现](../painter-backend/server/api/Article.go#L773) | body JSON: articleID | {} | 登录 + 发文权限 rule_id=2 + 文章作者本人 |
| `POST /api/article/update` | 统一保存文章内容和组织信息 [实现](../painter-backend/server/api/Article.go#L838) | body JSON: articleID/content/title/categoryID，summary/tagList[] | {} | 登录 + 发文权限 rule_id=2 + 文章作者本人 |
| `POST /api/article/delete` | 删除自己的文章 [实现](../painter-backend/server/api/Article.go#L297) | body JSON: articleID | {} | 登录；数据库条件限制 author=当前用户 |
| `POST /api/article/like/create` | 点赞文章 [实现](../painter-backend/server/api/Article.go#L573) | body JSON: articleID | {} | 登录 |
| `POST /api/article/like/delete` | 取消文章点赞 [实现](../painter-backend/server/api/Article.go#L598) | body JSON: articleID | {} | 登录 |
| `POST /api/article/like` | 切换文章点赞 [实现](../painter-backend/server/api/Article.go#L623) | body JSON: articleID | {} | 登录 |
| `POST /api/article/like/check` | 查询本人文章点赞状态 [实现](../painter-backend/server/api/Article.go#L648) | body JSON: articleID | {IsLiked} | 登录 |
| `POST /api/article/collection` | 切换文章收藏 [实现](../painter-backend/server/api/Collection.go#L62) | body JSON: articleID | {} | 登录 |
| `POST /api/article/tag/create` | 为自己的文章添加标签 [实现](../painter-backend/server/api/Article.go#L673) | body JSON: articleID, tagID | {} | 登录 + 文章作者本人 |
| `POST /api/article/tag/update` | 替换自己的文章标签集合 [实现](../painter-backend/server/api/Article.go#L703) | body JSON: articleID, tagList[] | {} | 登录 + 发文权限 rule_id=2 + 文章作者本人 |
| `POST /api/article/tag/delete` | 移除自己文章的标签关联 [实现](../painter-backend/server/api/Article.go#L808) | body JSON: articleID, tagID | {} | 登录 + 文章作者本人 |

### 当前行为与限制

- `/api/article/get/author`：列表只含 status=1，ArticleCount 却统计作者全部未软删除文章，可能不一致。 [依据](../painter-backend/database/Article.go#L225)
- `/api/article/get/title`：已注册但 SQL 为 title like and status = 1，缺少 LIKE 占位符；不能视为可用搜索。 [依据](../painter-backend/database/Article.go#L257)
- `/api/article/get/content`：已注册但 SQL 为 content like and status = 1，缺少 LIKE 占位符；ArticleContentTable 亦无 status/updated_at。 [依据](../painter-backend/database/Article.go#L275)
- `/api/article/get/collection`：userID 被错误赋给 json.Title，实际 json.UserID=0；数据库查询 collection.status，模型无此字段。 [依据](../painter-backend/database/Article.go#L322)
- `/api/article/get/category`：列表与总数均按 status=1，更新时间倒序。 [依据](../painter-backend/database/Article.go#L293)
- `/api/article/get/tag`：文章 ID 查询未筛发布状态；计数查询 article_tag.status，但关联表无此字段，处理器会返回失败。 [依据](../painter-backend/database/Article.go#L350)
- `/api/article/get/time`：按 updated_at DESC，属于最近更新流，没有推荐算法。 [依据](../painter-backend/database/Article.go#L367)
- `/api/article/get/count`：统计 status=1 且未软删除。 [依据](../painter-backend/database/Article.go#L384)
- `/api/article/get/self`：复用 GetArticlesByAuthor，只返回 status=1；总数包含草稿。该接口漏草稿，但/user/self/full个人中心接口仍返回本人全部文章，可找回草稿。 [依据](../painter-backend/database/Article.go#L225)
- `/api/article/get`：数据库只按 ID 查找且未校验 status/作者，草稿可被公开读取；每次详情调用尝试阅读量+1。登录时尝试 AutoHistory，首次历史创建有缺陷且错误被忽略。 [依据](../painter-backend/database/Article.go#L406)
- `/api/article/create`：事务写入文章、正文、标签；固定 Status=0，不直接发布。categoryID 未校验有效性。 [依据](../painter-backend/database/Article.go#L14)
- `/api/article/update/content`：修改正文后尝试更新文章更新时间。 [依据](../painter-backend/database/Article.go#L63)
- `/api/article/update/title`：实现只赋值局部 article.Title，随后另行读取并更新时间，未保存标题；会返回成功但标题不变。 [依据](../painter-backend/database/Article.go#L159)
- `/api/article/update/status/dart`：路径保留代码原拼写 dart；写入 status=0。 [依据](../painter-backend/database/Article.go#L176)
- `/api/article/update/status/public`：写入 status=1；没有审核、排期、可见范围审批流程。 [依据](../painter-backend/database/Article.go#L176)
- `/api/article/update`：事务保存 title/summary/category_id/content，并差量同步标签；不处理请求模型中的 status/readCount/isTop；提交错误未检查。 [依据](../painter-backend/database/Article.go#L117)
- `/api/article/delete`：ArticleTable 使用 GORM DeletedAt，实际软删除；未删除正文/标签/点赞/评论/收藏/历史关联。未命中本人文章仍可返回成功。 [依据](../painter-backend/database/Article.go#L215)
- `/api/article/like/create`：点赞以(article_id,user_id)唯一索引约束；没有校验文章状态/存在性。 [依据](../painter-backend/database/Article.go#L476)
- `/api/article/like/delete`：点赞以(article_id,user_id)唯一索引约束；没有校验文章状态/存在性。 [依据](../painter-backend/database/Article.go#L491)
- `/api/article/like`：点赞以(article_id,user_id)唯一索引约束；没有校验文章状态/存在性。 [依据](../painter-backend/database/Article.go#L501)
- `/api/article/like/check`：点赞以(article_id,user_id)唯一索引约束；没有校验文章状态/存在性。查询出错也返回 ok=true。 [依据](../painter-backend/database/Article.go#L521)
- `/api/article/collection`：只有收藏文章，无收藏夹实体。没有校验文章状态/存在性。 [依据](../painter-backend/database/Collection.go#L58)
- `/api/article/tag/create`：create/delete 不要求发文权限；update 要求；关联以(article_id,tag_id)唯一索引约束。 [依据](../painter-backend/database/Article.go#L581)
- `/api/article/tag/update`：create/delete 不要求发文权限；update 要求；关联以(article_id,tag_id)唯一索引约束。 [依据](../painter-backend/database/Article.go#L669)
- `/api/article/tag/delete`：create/delete 不要求发文权限；update 要求；关联以(article_id,tag_id)唯一索引约束。 [依据](../painter-backend/database/Article.go#L595)

## 评论与反馈

6 个接口。路由入口：[注册位置](../painter-backend/server/Router.go#L75)。

| 方法与路径 | 用途 | 输入 | 返回 data | 访问条件 |
| --- | --- | --- | --- | --- |
| `GET /api/comment/list` | 读取文章评论 [实现](../painter-backend/server/api/Comment.go#L125) | query: ArticleID, Limit/Offset（默认20/0） | {Comments, CommentCount}；评论正文/作者昵称头像/LikeCount，登录时另有 Liked/IsSelf | 公开 |
| `GET /api/comment/list/l` | 读取评论并附本人点赞/所有权状态 [实现](../painter-backend/server/api/Comment.go#L164) | query: ArticleID, Limit/Offset（默认20/0） | {Comments}（不含 CommentCount） | 登录 |
| `POST /api/comment/create` | 发表文章评论 [实现](../painter-backend/server/api/Comment.go#L16) | body JSON: articleID, content | {UserID, CommentID, ArticleID} | 登录 + 评论权限 rule_id=3 |
| `POST /api/comment/delete` | 删除自己的评论 [实现](../painter-backend/server/api/Comment.go#L50) | body JSON: commentID | {UserID, CommentID} 或失败 | 登录；数据库检查评论作者本人 |
| `POST /api/comment/like` | 点赞评论 [实现](../painter-backend/server/api/Comment.go#L75) | body JSON: commentID | {UserID, CommentID} | 登录 |
| `POST /api/comment/dislike` | 取消评论点赞 [实现](../painter-backend/server/api/Comment.go#L100) | body JSON: commentID | {UserID, CommentID} | 登录 |

### 当前行为与限制

- `/api/comment/list`：平铺评论，无父评论/回复字段。登录版 JOIN 按点赞用户分组，可能一条评论多行、点赞计数与 Liked 不准确。 [依据](../painter-backend/database/Comment.go#L70)
- `/api/comment/list/l`：和公开接口登录分支复用；JOIN 按点赞用户分组，可能重复评论/计数错误。 [依据](../painter-backend/database/Comment.go#L108)
- `/api/comment/create`：len(content) 限制100字节，汉字通常约33个；不支持回复/编辑；未显式校验文章是否存在/已发布。 [依据](../painter-backend/database/Comment.go#L11)
- `/api/comment/delete`：处理器 if err!=nil \|\| flag==true 判断反向：成功删除反而返回失败，非本人未删反而返回成功。 [依据](../painter-backend/database/Comment.go#L27)
- `/api/comment/like`：(comment_id,user_id)唯一索引，重复点赞可能失败；未显式校验评论存在。 [依据](../painter-backend/database/Comment.go#L46)
- `/api/comment/dislike`：dislike 是撤销点赞，未实现负向点踩。 [依据](../painter-backend/database/Comment.go#L60)

## 文章收藏

3 个接口。路由入口：[注册位置](../painter-backend/server/Router.go#L86)。

| 方法与路径 | 用途 | 输入 | 返回 data | 访问条件 |
| --- | --- | --- | --- | --- |
| `POST /api/collection/delete` | 取消文章收藏 [实现](../painter-backend/server/api/Collection.go#L37) | body JSON: articleID | {UserID, ArticleID} | 登录 |
| `POST /api/collection/list` | 分页读取我的收藏记录 [实现](../painter-backend/server/api/Collection.go#L112) | body JSON: limit, offset | {Collections: CollectionTable[], Total} | 登录 |
| `POST /api/collection/check` | 检查自己是否收藏文章 [实现](../painter-backend/server/api/Collection.go#L87) | body JSON: articleID | {IsCollected} | 登录 |

### 当前行为与限制

- `/api/collection/delete`：仅操作当前用户收藏；数据库未命中也可成功。 [依据](../painter-backend/database/Collection.go#L28)
- `/api/collection/list`：只返回收藏记录（CollectionID/UserID/ArticleID/CollectionTime），不附文章详情；未明确排序。 [依据](../painter-backend/database/Collection.go#L38)
- `/api/collection/check`：HasCollection 忽略查询错误，按 RowsAffected 判断。 [依据](../painter-backend/database/Collection.go#L74)

## 关注与粉丝

4 个接口。路由入口：[注册位置](../painter-backend/server/Router.go#L64)。

| 方法与路径 | 用途 | 输入 | 返回 data | 访问条件 |
| --- | --- | --- | --- | --- |
| `GET /api/follow/followers` | 查看我的粉丝 [实现](../painter-backend/server/api/Follow.go#L64) | query: Limit/Offset（默认20/0） | {Total, FollowList: FollowUserInfo[]} | 登录 |
| `GET /api/follow/followings` | 查看我关注的人 [实现](../painter-backend/server/api/Follow.go#L93) | query: Limit/Offset（默认20/0） | {Total, FollowList: FollowUserInfo[]} | 登录 |
| `POST /api/follow/follow` | 关注用户 [实现](../painter-backend/server/api/Follow.go#L14) | body JSON: followingID | {UserID, FollowingID} | 登录 |
| `POST /api/follow/unfollow` | 取消关注用户 [实现](../painter-backend/server/api/Follow.go#L39) | body JSON: followingID | {UserID, UnFollowingID} | 登录 |

### 当前行为与限制

- `/api/follow/followers`：粉丝条目的 Following 通过反向查询，用于表示当前用户是否回关；列表无显式排序。 [依据](../painter-backend/database/Follow.go#L40)
- `/api/follow/followings`：Following 实际查询被关注者是否反向关注当前用户，因此不能直当作我是否已关注；列表无显式排序。 [依据](../painter-backend/database/Follow.go#L69)
- `/api/follow/follow`：唯一索引防重复关系；未检查自关注或目标用户存在；没有关注动态/通知流。 [依据](../painter-backend/database/Follow.go#L13)
- `/api/follow/unfollow`：数据库未命中仍可返回成功。 [依据](../painter-backend/database/Follow.go#L29)

## 阅读历史

1 个接口。路由入口：[注册位置](../painter-backend/server/Router.go#L57)。

| 方法与路径 | 用途 | 输入 | 返回 data | 访问条件 |
| --- | --- | --- | --- | --- |
| `GET /api/history/list` | 读取自己的阅读历史记录 [实现](../painter-backend/server/api/History.go#L39) | query: Limit/Offset（默认20/0） | HistoryTable[]（HistoryID/UserID/ArticleID/HistoryTime） | 登录 |

### 当前行为与限制

- `/api/history/list`：无文章详情/总数/显式最近时间排序/清空删除接口；自动记录首次创建路径不可达：First 查不到返回错误，AutoHistory 直接退出。 [依据](../painter-backend/database/History.go#L28)

## 分类

7 个接口。路由入口：[注册位置](../painter-backend/server/Router.go#L96)。

| 方法与路径 | 用途 | 输入 | 返回 data | 访问条件 |
| --- | --- | --- | --- | --- |
| `GET /api/category/list` | 分页分类列表 [实现](../painter-backend/server/api/Category.go#L94) | Query: Limit 默认 20、Offset 默认 0 | categoriesNumber、data.categories（CategoryID、CategoryName、Description） | 公开 |
| `GET /api/category/get` | 读取单个分类 [实现](../painter-backend/server/api/Category.go#L109) | Query: ID(int) | CategoryTable | 公开 |
| `GET /api/category/get/fulllist` | 全部分类及每类文章数 [实现](../painter-backend/server/api/Category.go#L121) | 无参数 | categoriesNumber、data.categories（CategoryTable + ArticleCount） | 公开 |
| `POST /api/category/create` | 创建分类 [实现](../painter-backend/server/api/Category.go#L14) | 绑定体: name、description | CategoryID | 管理员 |
| `POST /api/category/update/name` | 按旧名称修改分类名称 [实现](../painter-backend/server/api/Category.go#L34) | 绑定体: oldName、newName | OldName、NewName | 管理员 |
| `POST /api/category/update/desc` | 按分类名称修改描述 [实现](../painter-backend/server/api/Category.go#L54) | 绑定体: name、description | Name、Description | 管理员 |
| `POST /api/category/update/` | 按 ID 同时更新分类名与描述 [实现](../painter-backend/server/api/Category.go#L74) | 绑定体: categoryID(int)、name、description | 空 data success | 管理员 |

### 当前行为与限制

- `/api/category/list`：无参数边界限制、无指定排序
- `/api/category/get`：ID 转换错误被忽略，公共整数校验也不能排除 0
- `/api/category/get/fulllist`：ArticleCount 不按文章 status 过滤 [依据](../painter-backend/database/Category.go#L150)
- `/api/category/create`：创建前按名称查重；没有分类删除路由 [依据](../painter-backend/database/Category.go#L13)
- `/api/category/update/name`：用名称定位分类，不是 categoryID；会先查 ID [依据](../painter-backend/database/Category.go#L49)
- `/api/category/update/desc`：用名称定位分类，不是 categoryID [依据](../painter-backend/database/Category.go#L80)
- `/api/category/update/`：实际路由有尾斜杠 [依据](../painter-backend/database/Category.go#L94)

## 标签

7 个接口。路由入口：[注册位置](../painter-backend/server/Router.go#L45)。

| 方法与路径 | 用途 | 输入 | 返回 data | 访问条件 |
| --- | --- | --- | --- | --- |
| `GET /api/tag/suggest` | 取建议标签 [实现](../painter-backend/server/api/Tag.go#L15) | 无参数 | TagTable[]（TagID、TagName、Description） | 公开 |
| `GET /api/tag/list` | 分页标签列表 [实现](../painter-backend/server/api/Tag.go#L25) | Query: Limit 默认 1，Offset 默认 20 | Tags、data.TagNumber | 公开 |
| `GET /api/tag/list/full` | 全部标签及每个标签文章关联数 [实现](../painter-backend/server/api/Tag.go#L53) | 无参数 | Tags（TagTable + ArticleCount）及 data.TagNumber | 公开 |
| `POST /api/tag/create` | 创建标签 [实现](../painter-backend/server/api/Tag.go#L64) | 绑定体: name、description | Name、Description、ID | 管理员 |
| `POST /api/tag/update/name` | 更新标签名称 [实现](../painter-backend/server/api/Tag.go#L84) | 绑定体: tagID(int)、name | PreName、CurName、ID | 管理员 |
| `POST /api/tag/update/desc` | 更新标签描述 [实现](../painter-backend/server/api/Tag.go#L109) | 绑定体: tagID(int)、description | PreDescription、CurDescription、ID | 管理员 |
| `POST /api/tag/update/` | 同时更新标签名称和描述 [实现](../painter-backend/server/api/Tag.go#L134) | 绑定体: tagID(int)、name、description | 空 data success | 管理员 |

### 当前行为与限制

- `/api/tag/suggest`：实际只是未指定排序的前 20 条标签，不是关键词联想或个性化推荐 [依据](../painter-backend/database/Tag.go#L106)
- `/api/tag/list`：默认值与其他分页接口不同；不传参数会从第 21 条起取 1 条；未限制范围、未指定排序
- `/api/tag/list/full`：Router 绑定 FullTagsListWithCount；不是同文件中未注册的 FullTagsList；ArticleCount 直接计 article_tag 关联记录，不按文章发布状态过滤 [依据](../painter-backend/database/Tag.go#L116)
- `/api/tag/create`：没有标签删除接口；名称受数据库 unique 约束 [依据](../painter-backend/database/Tag.go#L10)
- `/api/tag/update/`：实际路由保留尾斜杠；通过 storage.Updater + repository.Tag 更新，和两个单字段接口实现不同 [依据](../painter-backend/database/storage/Tag.go#L16) [依据](../painter-backend/database/storage/Common.go#L126)

## 文件上传

1 个接口。路由入口：[注册位置](../painter-backend/server/Router.go#L150)。

| 方法与路径 | 用途 | 输入 | 返回 data | 访问条件 |
| --- | --- | --- | --- | --- |
| `POST /api/file/upload` | 上传本地文件 [实现](../painter-backend/server/api/File.go#L16) | multipart/form-data: file；大小至多 10 MiB | filePath=/fs/、data.fileName=时间戳_原文件名 | 登录 |

### 当前行为与限制

- `/api/file/upload`：无额外角色、扩展名、MIME 类型校验，也没有所有者字段或文件管理/删除接口；记录先写数据库，再保存磁盘；保存失败无回滚；数据库记录 FilePath 是项目根目录直接拼文件名，与实际 /server/static/upload/ 目录不同；release 模式从 /fs 提供静态文件；debug 只有 /f，但上传响应始终返回 /fs/；云存储函数为空且未注册；FileGetter GET 路由被注释，不能列为可用下载 API [依据](../painter-backend/database/FileStorage.go#L11) [依据](../painter-backend/server/Server.go#L94) [依据](../painter-backend/server/Router.go#L146)

## 站点公共信息

6 个接口。路由入口：[注册位置](../painter-backend/server/Router.go#L156)。

| 方法与路径 | 用途 | 输入 | 返回 data | 访问条件 |
| --- | --- | --- | --- | --- |
| `GET /api/common/starttime` | 读取站点初始化时间 [实现](../painter-backend/server/api/Common.go#L15) | 无参数 | TimeStamp（配置字符串）、JSTimeStamp（乘 1000 的整数） | 公开 |
| `GET /api/common/vis/preday` | 名义上的昨日访客数 [实现](../painter-backend/server/api/Common.go#L23) | 无参数 | Count | 公开 |
| `GET /api/common/vis/currmonth` | 名义上的本月访客数 [实现](../painter-backend/server/api/Common.go#L33) | 无参数 | Count | 公开 |
| `GET /api/common/info` | 前台启动时读取站点展示配置 [实现](../painter-backend/server/api/Common.go#L50) | 无参数 | Github、ICPCode、CanRegister、SiteName、CanSendMail | 公开 |
| `GET /api/common/isadmin` | 查询当前 cookie 对应账户是否管理员 [实现](../painter-backend/server/api/Common.go#L82) | 可选 painter-session cookie | isAdmin:boolean；未登录/无效 session 为 ok=false | 公开 |
| `GET /api/common/entry` | 首页聚合统计 [实现](../painter-backend/server/api/Common.go#L115) | 无参数 | TimeStamp、JSTimeStamp、YesterdayCount、CurrentMonthCount、ArticleCount | 公开 |

### 当前行为与限制

- `/api/common/starttime`：名称 GetServerRunningTime 容易误导：返回配置 FirstInit，不是本次进程启动时刻或已运行时长
- `/api/common/vis/preday`：统计实现使用本月 1 日为 start，再减 1 月/1 日得到更早的 end，却查询 date>=start AND date<end；区间方向反了，不能据此认定指标已正确实现 [依据](../painter-backend/database/Visitor.go#L32)
- `/api/common/vis/currmonth`：统计实现使用本月 1 日为 start，再减 1 月/1 日得到更早的 end，却查询 date>=start AND date<end；区间方向反了，不能据此认定指标已正确实现 [依据](../painter-backend/database/Visitor.go#L32)
- `/api/common/info`：只返回邮件是否启用；不返回邮件连接详情；GetMailSetting 的 err 未处理
- `/api/common/isadmin`：不是管理接口的授权入口；真正授权由 CheckAdminMid 完成；正常普通用户返回 ok=true、isAdmin=false
- `/api/common/entry`：复用上述访客统计，存在同样日期区间错误；ArticleCount 来自全站文章计数 [依据](../painter-backend/database/Visitor.go#L32)

## 站点管理

4 个接口。路由入口：[注册位置](../painter-backend/server/Router.go#L167)。

| 方法与路径 | 用途 | 输入 | 返回 data | 访问条件 |
| --- | --- | --- | --- | --- |
| `GET /api/setting/` | 读取站点基本与邮件设置 [实现](../painter-backend/server/api/Setting.go#L14) | 仅管理员 cookie | Github、ICPCode、CanRegister、SiteName、Mail 配置对象 | 管理员 |
| `POST /api/setting/` | 保存站点基本设置与邮件设置 [实现](../painter-backend/server/api/Setting.go#L50) | 绑定体: SiteName、ICPCode、Github、CanRegister(bool)、MailActive(bool)、MailFrom、MailPassword、MailSMTPHost、MailSMTPPort(int) | 空 data success | 管理员 |
| `GET /api/setting/userlist` | 分页管理用户列表 [实现](../painter-backend/server/api/Setting.go#L70) | Query: Limit 默认 20、Offset 默认 0 | Users、UserNumber；Users 仅选择 ID、UserName、NickName、HeaderField、UserGroup，其余 UserTable 字段为零值 | 管理员 |
| `POST /api/setting/user/permission` | 给用户指定一个权限组 [实现](../painter-backend/server/api/Setting.go#L88) | 绑定体: userID(int)、groupID(int) | 空 data success | 管理员 |

### 当前行为与限制

- `/api/setting/`：当前要求所有字符串非空，不能按需仅更新单项或清空字符串；按 9 个键逐项 Save，缺少整体事务；注册开关没有在注册入口执行 [依据](../painter-backend/database/Setting.go#L123)
- `/api/setting/userlist`：未提供用户搜索、封禁/删除的独立接口 [依据](../painter-backend/database/Setting.go#L147)
- `/api/setting/user/permission`：直接保存 UserGroup；未验证 groupID 是否存在、未阻止修改自己或最后一个管理员；这不是细粒度任意规则编辑；没有 group/rule 列表与维护路由 [依据](../painter-backend/database/Rule.go#L10)

## 前端调用对照

前端有 **87 个 API 封装**，其中 **51 个在页面、布局、组件或 store 中存在调用点，36 个没有调用点**。调用点不等同于可达页面：手机号登录仅剩 store 分支。头像上传和编辑器贴图直接调用 Axios 上传接口，不经过 `api_*.ts`。

### 方法或路径不匹配

| 前端封装 | 当前请求 | 后端实际情况 |
| --- | --- | --- |
| [CreateHistory](../painter-frontend-nuxt/apis/api_history.ts#L4) | `POST /api/history/create` | 没有活动路由 |
| [LoginUserWithPhone](../painter-frontend-nuxt/apis/api_user.ts#L10) | `POST /api/user/login/phone` | 没有活动路由 |
| [CreateCollection](../painter-frontend-nuxt/apis/api_collection.ts#L4) | `POST /api/collection/create` | 没有活动路由；收藏通过 `POST /api/article/collection` 切换 |
| [GetArticleByTitle](../painter-frontend-nuxt/apis/api_article.ts#L5) | `POST /api/article/get/title` | 仅注册 GET |
| [GetArticleByContent](../painter-frontend-nuxt/apis/api_article.ts#L9) | `POST /api/article/get/content` | 仅注册 GET |

其余 82 个封装与已注册的方法和路径匹配，参数、返回值和业务语义仍需按上表核对。例如收藏文章列表前端发送 `UserID/Limit/Offset`，后端读取 `userID/limit/offset`，另有赋值和查询错误。

### 有接口但缺少用户操作

- 文章删除：路由和封装都有，无删除按钮或调用。
- 阅读历史：路由和封装都有，无历史页面或调用，自动创建首条历史的实现另有缺陷。
- 修改密码：路由存在，无前端封装或页面；“忘记密码”菜单为空，不能当作找回密码功能。
- 站点管理：页面与后台接口存在，但未接入站内导航；关注粉丝页面已写，现有入口却跳到 `/follow`，页面文件为 `followList.vue`。

## 基础与环境入口

下列入口不计入 86 个业务接口。

| 方法与路径 | 启用条件 | 行为 |
| --- | --- | --- |
| `GET /healthz` | 所有模式 | 检查 DB、Cache、Conf、Server 内存状态标志，正常 200，异常 400；受全局中间件影响 |
| `GET /ready` | 所有模式 | 返回 ready 或 not ready，二者均 HTTP 200 |
| `GET /api/{random-debug-key}` | debug | 设置调试 cookie；免检路径比较漏了 `/api` 前缀，初次访问可能被拒绝。占位符不是真实字面路径 |
| `POST /test` | test | 测试字段绑定并记日志，无正式业务响应 |
| `GET/HEAD /fs/*filepath` | release | 上传目录的静态读取 |
| `GET/HEAD /f/*filepath` | debug | 上传目录的静态读取；与上传响应固定 `/fs/` 不一致 |
| `GET/HEAD /assets/*filepath` | release | 打包前端资源 |
| `GET/HEAD /favicon.ico` | release | 站点图标 |
| `GET/HEAD /` | release | 嵌入的前端首页；未匹配路由另回退前端 index.html |

基础入口依据：[Router.go](../painter-backend/server/Router.go#L173)、[Server.go](../painter-backend/server/Server.go#L94)、[DebugModeMid.go](../painter-backend/server/mid/DebugModeMid.go#L13)。

## 仅定义或注释的入口

以下函数存在，当前没有绑定为活动 API；不能从函数名或旧文档把它们计入可用接口。

| Handler | 状态 |
| --- | --- | --- |
| [FileGetter](../painter-backend/server/api/File.go#L73) | 有定义但未注册 |
| [PhoneLogin](../painter-backend/server/api/User.go#L159) | 有定义但未注册 |
| [FullTagsList](../painter-backend/server/api/Tag.go#L42) | `/api/tag/list/full` 实际绑定 FullTagsListWithCount |
| [CreateCollection](../painter-backend/server/api/Collection.go#L12) | 有定义但未注册 |
| [UpdateArticleReadCount](../painter-backend/server/api/Article.go#L192) | 有定义但未注册 |
| [UpdateArticleStatus](../painter-backend/server/api/Article.go#L267) | 有定义但未注册 |
| [CreateHistory](../painter-backend/server/api/History.go#L14) | 有定义但未注册 |

置顶路由 `/api/article/update/istop` 也被注释，未找到对应 handler。云存储上传下载函数仅为空占位。标签删除、分类删除、收藏夹、评论回复、通知、推荐、审核、文件管理和细粒度权限管理均没有活动业务接口。

## 统计定义

`/common/vis/*` 和首页入口统计采集的是 **UA 加 IP 的每日去重访客**，不能直接称为页面浏览量 PV。日期查询区间方向错误，且访客入库事务没有提交，当前数字不能作为运营判断依据。文章详情读取及已有文章编辑页的加载都会尝试递增阅读数；本地 Markdown 预览和文章列表聚合本身不增加阅读数。文章阅读计数也不等同于独立读者数。

依据：[访客去重](../painter-backend/cache/Visitor.go#L17)、[统计入库与查询](../painter-backend/database/Visitor.go#L21)、[文章详情计数](../painter-backend/database/Article.go#L406)。
