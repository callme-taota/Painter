# 可配置化能力设计（design-configurability）

## 配置中心模型

配置存储键采用三段式：`namespace.key.environment`

- `feature.comment.enabled`
- `feature.upload.enabled`
- `security.cors.allowedOrigins`
- `theme.web.primaryColor`
- `dict.userGroup.options`

## 配置分类

- 功能开关：评论、注册、上传、审核、实验特性
- 字典中心：用户组、文章状态、审核状态、枚举映射
- 主题令牌：颜色、字号、圆角、暗黑策略
- 安全策略：CORS 白名单、限流阈值、上传大小、风控参数

## 生效机制

- 后台修改配置 -> system-service 持久化 + 发布版本号
- 网关与前端按版本号拉取增量
- 高风险配置走“预发布环境先验 + 手动确认”流程

## 前端接入方式

- 启动阶段拉取 `public config`
- 登录后拉取用户级别配置（权限、实验特性）
- 使用本地缓存并带 `configVersion` 做快速比较

## 审计要求

- 配置变更记录操作人、旧值、新值、变更理由
- 高危配置要求二次确认
