# 历史 schema 的隔离验证工具

此目录只验证 [原始 schema.sql](../schema.sql) 的 DDL 与部分数据库不变量，以及 pgx 的基础类型能力。**不会应用或验证仓库根 [migrations](../../../migrations)，不代表当前表数、HTTP 行为、认证能力、部署状态或性能结果。** 历史执行输出不纳入本设计目录；运行结果写到仓库外的私有临时位置。

## 环境要求

- PostgreSQL 18 的服务端与客户端程序，提供 `pg_trgm` 扩展。
- UTF8 数据库与 Unicode 字符分类；SQL 脚本使用 `C.UTF-8`，运行环境需提供该 locale。
- Python 3 与 psycopg2。脚本不安装依赖，也不复用应用数据库。
- Go 驱动探针的 Go 与 pgx 版本由自己的 [go.mod](driver-probe/go.mod) 和 go.sum 约束。

## 验证原始 SQL

从仓库根运行，替换 PostgreSQL 程序目录与输出路径：

```bash
python3 docs/design-baseline/validation/verify_postgres.py \
  --pg-bin /path/to/postgresql/18/bin \
  --result /tmp/painter-baseline-validation.json
```

脚本从自身位置读取 `docs/design-baseline/schema.sql`，不接受业务数据库 DSN，不读取 `DATABASE_URL`。它自行创建临时 PostgreSQL 集群，禁用 TCP，使用私有 Unix socket，并创建独立验证库和恢复库。运行账号不能是 root；本地 trust 仅限这个隔离实例，不是生产认证配置。

检查素材包括草稿/公开快照隔离、修订封存、CAS、站点和账号屏障、媒体竞争、聚合提交乱序、中文检索，以及 dump/restore 后的不变量。`enable_seqscan=off` 仅用于验证 GIN 可用路径，不能证明真实规模下的自然查询计划或性能。

脚本通过退出清理钩子停止自己启动的实例，但保留临时数据、日志和 dump 以便诊断；完成检查后由运行者核对进程已停止并清理临时目录。输出包含隔离实例位置，不提交 Git，不复用为当前版本验收报告。

## 检查 pgx 基础能力

Go 探针只执行只读能力与参数查询，不创建业务表，也不验证原始或当前 schema 的完整性。它仅接受显式 `PAINTER_TEST_DATABASE_URL`，不回退到应用 `DATABASE_URL`。先为可丢弃的 PostgreSQL 18 测试库准备 `pg_trgm`，再在 `driver-probe` 目录构建：

```bash
CGO_ENABLED=0 go build -o /tmp/painter-baseline-pgx-probe .
```

通过受信任环境为该进程提供测试连接后执行 `/tmp/painter-baseline-pgx-probe`。探针检查 UTF8、中文 trigram 和 JSONB / BYTEA / BOOLEAN / BIGINT 的参数与 Scan 往返；基础能力成功不代表 Painter 应用测试通过。不要把真实 DSN、凭据或探针输出提交仓库。

当前应用验证应使用根模块的测试流程与可丢弃 PostgreSQL 库；实际 schema 以根迁移为准。
