# 老项目到 Painter-2026 接口迁移矩阵

本文基于老项目路由文件 `painter-backend/server/Router.go`，给出 1:1 复刻映射。

## 状态说明

- `mapped`: 已映射到新接口并可转发
- `compat`: 兼容层保留，返回统一协议（待业务细化）
- `native`: 新项目已有同名或同语义实现

## User

- `POST /api/user/create` -> `mapped` `/api/v1/identity/auth/register`
- `POST /api/user/login/email` -> `mapped` `/api/v1/identity/auth/login`
- `POST /api/user/login/email/pass` -> `mapped` `/api/v1/identity/auth/login`
- `POST /api/user/login/uname` -> `mapped` `/api/v1/identity/auth/login`
- `POST /api/user/login/uname/pass` -> `mapped` `/api/v1/identity/auth/login`
- `POST /api/user/login/check` -> `mapped` `/api/v1/identity/auth/check`
- `POST /api/user/login/exist` -> `mapped` `/api/v1/identity/auth/exist`
- `POST /api/user/login/send` -> `mapped` `/api/v1/identity/auth/send-code`
- `POST /api/user/login/mailcheck` -> `mapped` `/api/v1/identity/auth/check-code`
- `GET /api/user/self` -> `mapped` `/api/v1/identity/users/self`
- `GET /api/user/self/full` -> `mapped` `/api/v1/identity/users/self/full`
- `POST /api/user/logout` -> `mapped` `/api/v1/identity/users/logout`
- `POST /api/user/update/*` -> `mapped` `/api/v1/identity/users/update*`
- `GET /api/user/info` -> `mapped` `/api/v1/identity/users/info`

## Tag / Category

- `GET /api/tag/suggest` -> `mapped` `/api/v1/content/tags/suggest`
- `GET /api/tag/list` -> `mapped` `/api/v1/content/tags/list`
- `GET /api/tag/list/full` -> `mapped` `/api/v1/content/tags/list/full`
- `POST /api/tag/create` -> `mapped` `/api/v1/content/tags/create`
- `POST /api/tag/update/*` -> `mapped` `/api/v1/content/tags/update`
- `GET /api/category/list` -> `mapped` `/api/v1/content/categories/list`
- `GET /api/category/get` -> `mapped` `/api/v1/content/categories/get`
- `GET /api/category/get/fulllist` -> `mapped` `/api/v1/content/categories/full`
- `POST /api/category/create` -> `mapped` `/api/v1/content/categories/create`
- `POST /api/category/update/*` -> `mapped` `/api/v1/content/categories/update*`

## Follow / History / Collection

- `GET /api/history/list` -> `mapped` `/api/v1/analytics/history/list`
- `GET /api/follow/followers` -> `mapped` `/api/v1/identity/followers`
- `GET /api/follow/followings` -> `mapped` `/api/v1/identity/follows`
- `POST /api/follow/follow` -> `mapped` `/api/v1/identity/follows`
- `POST /api/follow/unfollow` -> `mapped` `/api/v1/identity/follows` (兼容语义)
- `POST /api/collection/delete` -> `mapped` `/api/v1/content/collection/delete`
- `POST /api/collection/list` -> `mapped` `/api/v1/content/collection/list`
- `POST /api/collection/check` -> `mapped` `/api/v1/content/collection/check`

## Comment

- `GET /api/comment/list` -> `mapped` `/api/v1/content/comments`
- `GET /api/comment/list/l` -> `mapped` `/api/v1/content/comments/liked`
- `POST /api/comment/create` -> `mapped` `/api/v1/content/comments`
- `POST /api/comment/delete` -> `mapped` `/api/v1/content/comment/delete`
- `POST /api/comment/like` -> `mapped` `/api/v1/content/comment/like`
- `POST /api/comment/dislike` -> `mapped` `/api/v1/content/comment/dislike`

## Article

- `GET /api/article/get/author` -> `mapped` `/api/v1/content/articles/by-author`
- `GET /api/article/get/title` -> `mapped` `/api/v1/content/articles/by-title`
- `GET /api/article/get/content` -> `mapped` `/api/v1/content/articles/by-content`
- `GET /api/article/get/collection` -> `mapped` `/api/v1/content/articles/by-collection`
- `GET /api/article/get/category` -> `mapped` `/api/v1/content/articles/by-category`
- `GET /api/article/get/tag` -> `mapped` `/api/v1/content/articles/by-tag`
- `GET /api/article/get/time` -> `mapped` `/api/v1/content/articles`
- `GET /api/article/get/count` -> `mapped` `/api/v1/content/articles/count`
- `GET /api/article/get/self` -> `mapped` `/api/v1/content/articles/self`
- `GET /api/article/get` -> `mapped` `/api/v1/content/articles/get`
- `POST /api/article/create` -> `mapped` `/api/v1/content/articles`
- `POST /api/article/update/*` -> `mapped` `/api/v1/content/articles/update*`
- `POST /api/article/delete` -> `mapped` `/api/v1/content/articles/delete`
- `POST /api/article/like/create` -> `mapped` `/api/v1/content/interactions/like/create`
- `POST /api/article/like/delete` -> `mapped` `/api/v1/content/interactions/like/delete`
- `POST /api/article/like` -> `mapped` `/api/v1/content/interactions/like`
- `POST /api/article/like/check` -> `mapped` `/api/v1/content/interactions/like/check`
- `POST /api/article/collection` -> `mapped` `/api/v1/content/interactions/collect`
- `POST /api/article/tag/create` -> `mapped` `/api/v1/content/tags/create`
- `POST /api/article/tag/update` -> `mapped` `/api/v1/content/tags/update`
- `POST /api/article/tag/delete` -> `mapped` `/api/v1/content/tags/delete`

## File / Common / Setting

- `POST /api/file/upload` -> `mapped` `/api/v1/system/file/upload`
- `GET /api/common/starttime` -> `mapped` `/api/v1/system/common/starttime`
- `GET /api/common/vis/preday` -> `mapped` `/api/v1/system/common/vis/preday`
- `GET /api/common/vis/currmonth` -> `mapped` `/api/v1/system/common/vis/currmonth`
- `GET /api/common/info` -> `mapped` `/api/v1/system/common/info`
- `GET /api/common/isadmin` -> `mapped` `/api/v1/system/common/isadmin`
- `GET /api/common/entry` -> `mapped` `/api/v1/system/common/entry`
- `GET /api/setting/` -> `mapped` `/api/v1/system/admin/settings`
- `POST /api/setting/` -> `mapped` `/api/v1/system/admin/settings` (method normalize to PATCH)
- `GET /api/setting/userlist` -> `mapped` `/api/v1/identity/admin/userlist`
- `POST /api/setting/user/permission` -> `mapped` `/api/v1/identity/admin/user/permission`

## 修复优先说明

- 老接口中的方法不规范（如应 PATCH 却使用 POST）在新接口层已规范化。
- 兼容层会保留旧路径并保证响应协议一致，逐步替换为新契约调用。
