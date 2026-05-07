# 开发指南

## 代码组织

- 前端：`apps/web`、`apps/admin`
- 后端：`services/*`
- 契约：`packages/openapi`
- SDK：`packages/sdk-ts`

## 开发流程

1. 先定义或更新 OpenAPI
2. 更新服务实现
3. 生成/更新 SDK
4. 接入前端页面
5. 补齐测试
6. 更新文档

## 常用命令

```bash
npm run sdk:generate
npm run contract:check
npm run verify
```
