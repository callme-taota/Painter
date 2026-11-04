# PostgreSQL 设计验证复现

此目录验证设计 DDL 与必要并发语义，不实现业务服务器。当前结果分别记录在 [postgres-result.json](postgres-result.json) 和 [pgx-probe-result.json](pgx-probe-result.json)。测试的 schema SHA256 为 `d4e08b685182e0c453db5c92729091d50e348b297c22f8028cf50d593a410ff3`。

## 环境

- PostgreSQL **18.6** server/client binaries，包含官方 `pg_trgm` 扩展。
- UTF8，Unicode `LC_CTYPE=C.UTF-8`。只有 UTF8 编码而 `LC_CTYPE=C` 不足以保证中文 trigram：测试已复现其中文 trigram 为空。
- Python 3 与 psycopg2；此次使用 Ubuntu 包 python3-psycopg2 2.9.11，仅解包到 `/tmp`，未安装系统包。
- pgx probe：Go **1.26.7**，`github.com/jackc/pgx/v5 v5.11.0`，依赖由自己的 go.mod/go.sum 锁定，`CGO_ENABLED=0`。

## 完整 SQL 验证

从仓库根目录运行：

```bash
python3 docs/next/validation/verify_postgres.py \
  --pg-bin /path/to/postgresql/18/bin \
  --result /tmp/painter-postgres-validation.json
```

脚本只接受 PostgreSQL binary 路径，**不接受业务数据库 DSN**。它自行在 `/tmp/painter-pg-check-*` 创建集群，禁用 TCP，仅通过私有 Unix socket 连接；创建 `checks` 和 `restored` 两个测试数据库，完成后通过 `atexit` 停止该临时实例。它不自动删除数据目录，以便检查日志及 dump。运行账号不能是 root，且系统须提供 `C.UTF-8` locale。

本次实际命令（`PYTHONPATH` 是解包的测试依赖，不是应用运行配置）：

```bash
PYTHONPATH=/tmp/painter-pg18-validation/root/usr/lib/python3/dist-packages \
python3 docs/next/validation/verify_postgres.py \
  --pg-bin /tmp/painter-pg18-validation/root/usr/lib/postgresql/18/bin \
  --result /tmp/painter-postgres-validation.json
```

脚本覆盖 52 项顺序与并发检查：草稿/公开快照隔离、分类标签封存、幂等与版本、站点/最后 owner 屏障、媒体与 GC 竞争、两连接 CAS、analytics 提交乱序、待聚合锁和回滚、中文 substring/GIN eligibility、原生类型与约束，以及 `pg_dump -Fc` 恢复到另一隔离库后逐表行数、公开查询、搜索和修订触发器复核。

中文 `EXPLAIN` 使用 `enable_seqscan=off` 只验证索引可用性，不能据此宣称自然计划必用 GIN 或达到任何 QPS。分析聚合测试只验证提交乱序与原子性等关键规则，不替代应用事件验证器的完整验收。

## pgx 纯 Go 编译与连接

先为独立测试库创建 `pg_trgm` 扩展，再在 `driver-probe` 目录运行：

```bash
CGO_ENABLED=0 go build -o /tmp/painter-pgx-probe .
PAINTER_PG_PROBE_DSN='host=/path/to/test-socket port=65439 dbname=test_db sslmode=disable' \
  /tmp/painter-pgx-probe
```

probe 不打印 DSN。它实际连接服务器，检查 UTF8 和中文 `show_trgm`，并经 pgx 原生参数/Scan 往返 JSONB、BYTEA、BOOLEAN 与 BIGINT。它只执行只读参数查询，不创建或写入业务表。

此次实际连接的临时测试库位于 `/tmp/painter-pg18-validation/socket`；运行后实例已关闭，不应把记录中的路径当作持续在线服务。

## 尚未验证

没有启动或实现新博客 HTTP 服务；未证明文件 fsync/断电恢复、真正旧站导入、OAuth 网络回调、整站媒体与密钥灾备、生产角色权限配置、目标机器性能或资源占用。临时集群使用本地 trust 仅为了隔离设计测试，不是生产认证配置。
