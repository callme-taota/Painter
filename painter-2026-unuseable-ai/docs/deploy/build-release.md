# 构建与发布

## 本地构建

```bash
npm run build
npm run docs:build
```

后端编译检查：

```bash
go list ./...
go test ./...
```

## 镜像构建

建议为每个服务独立构建镜像：

- `api-gateway`
- `identity-service`
- `content-service`
- `system-service`
- `analytics-service`

可用 `deploy/docker-compose.yml` 做本地验证。
