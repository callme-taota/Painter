# API 契约规范（standardize-api-contract）

## 版本与路径

- 统一前缀：`/api/v1`
- 领域分组：`/identity`、`/content`、`/system`、`/analytics`
- 破坏性变更通过 `v2` 路径升级，不在 `v1` 内破坏兼容

## 响应规范

```json
{
  "code": 0,
  "message": "ok",
  "traceId": "req-xxx",
  "data": {}
}
```

- `code=0` 表示成功
- 错误码按领域分段：
  - `1xxx`: 身份域
  - `2xxx`: 内容域
  - `3xxx`: 系统域
  - `4xxx`: 分析域
  - `9xxx`: 网关与通用错误

## SDK 生成链路

1. 修改契约：`packages/openapi/openapi.yaml`
2. 执行生成：`npm run sdk:generate`
3. 应用更新依赖并重编译

## 契约治理

- 所有新接口先提 OpenAPI 评审再开发
- CI 校验 OpenAPI 与 SDK 是否同步
- 前端禁止手写 URL 常量，统一走 SDK 客户端
