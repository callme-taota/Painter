# 数据流

## 请求链路

1. 前端调用 SDK（`@painter/sdk-ts`）
2. SDK 请求网关 `/api/v1/*`
3. 网关按域转发到对应服务
4. 服务返回统一 envelope
5. 前端页面解析并渲染

## 统一响应

```json
{
  "code": 0,
  "message": "ok",
  "traceId": "gw-xxx",
  "data": {}
}
```
