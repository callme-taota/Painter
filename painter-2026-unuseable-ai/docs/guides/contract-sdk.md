# 契约与 SDK

## 单一事实源

- `packages/openapi/openapi.yaml` 是接口契约唯一事实源。

## SDK 生成

```bash
npm run sdk:generate
```

## 一致性约束

- 所有前端请求必须走 `@painter/sdk-ts`
- 合并前必须通过 `contract:check`
