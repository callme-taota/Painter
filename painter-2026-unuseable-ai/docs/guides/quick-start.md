# 快速开始

## 1. 环境要求

- Go >= 1.23
- Node.js >= 20
- npm >= 10

## 2. 准备配置

```bash
cp .env.example .env
```

## 3. 启动后端服务

```bash
go run ./services/identity-service/cmd/identity
go run ./services/content-service/cmd/content
go run ./services/system-service/cmd/system
go run ./services/analytics-service/cmd/analytics
go run ./services/api-gateway/cmd/gateway
```

## 4. 启动前端

```bash
npm run dev:web
npm run dev:admin
```
