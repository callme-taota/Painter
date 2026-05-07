# 部署总览

当前提供本地 compose 方案与服务独立启动方案。

## 端口规划

- gateway: 18080
- identity: 18081
- content: 18082
- system: 18083
- analytics: 18084

## 依赖

- Go runtime
- Node.js + npm（前端与文档）

## 健康检查

每个服务都提供 `/healthz`。
