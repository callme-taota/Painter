# 测试指南

## 测试分层

- 后端 domain 单测
- 后端 application 单测
- 后端 transport handler 单测
- 网关代理集成测试
- 前端 store/composable/component 单测
- 契约检查（OpenAPI required paths）

## 执行方式

后端：

```bash
go test ./...
```

前端：

```bash
npm run test -w @painter/web
npm run test -w @painter/admin
```

契约：

```bash
npm run contract:check
```
