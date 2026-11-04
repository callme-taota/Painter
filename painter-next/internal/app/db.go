package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"painter-next/migrations"
)

func openDB(ctx context.Context, c Config) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(c.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("database configuration: %w", err)
	}
	pc.MaxConns = c.MaxConnections
	pc.MinConns = 0
	pc.MaxConnLifetime = 30 * time.Minute
	pc.MaxConnIdleTime = 5 * time.Minute
	pc.ConnConfig.ConnectTimeout = 5 * time.Second
	pc.ConnConfig.RuntimeParams["application_name"] = "painter-next"
	pc.ConnConfig.RuntimeParams["statement_timeout"] = "10000"
	pc.ConnConfig.RuntimeParams["lock_timeout"] = "3000"
	pc.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "15000"
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, err
	}
	var version int
	var encoding string
	if err = pool.QueryRow(ctx, "SELECT current_setting('server_version_num')::int,current_setting('server_encoding')").Scan(&version, &encoding); err != nil {
		pool.Close()
		return nil, err
	}
	if version < 180000 || version >= 190000 || encoding != "UTF8" {
		pool.Close()
		return nil, fmt.Errorf("PostgreSQL 18 UTF8 required (version %d encoding %s)", version, encoding)
	}
	return pool, nil
}

// Migrate serializes migrators on a dedicated connection. DDL and its checksum
// ledger entry commit together. No startup process silently rewrites a migration.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(7284923151123)"); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock(7284923151123)")
	files, err := fs.Glob(migrations.Files, "*.sql")
	if err != nil {
		return err
	}
	sort.Slice(files, func(i, j int) bool {
		a, _ := strconv.ParseInt(strings.SplitN(files[i], "_", 2)[0], 10, 64)
		b, _ := strconv.ParseInt(strings.SplitN(files[j], "_", 2)[0], 10, 64)
		return a < b
	})
	for _, name := range files {
		prefix := strings.SplitN(name, "_", 2)[0]
		version, e := strconv.ParseInt(prefix, 10, 64)
		if e != nil {
			return e
		}
		body, e := migrations.Files.ReadFile(name)
		if e != nil {
			return e
		}
		h := sha256.Sum256(body)
		sum := hex.EncodeToString(h[:])
		var exists bool
		if e = conn.QueryRow(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&exists); e != nil {
			return e
		}
		if exists {
			var old string
			e = conn.QueryRow(ctx, "SELECT checksum FROM schema_migrations WHERE version=$1", version).Scan(&old)
			if e == nil {
				if old != sum {
					return fmt.Errorf("migration %s checksum mismatch", name)
				}
				continue
			}
			if e != pgx.ErrNoRows {
				return e
			}
		}
		tx, e := conn.Begin(ctx)
		if e != nil {
			return e
		}
		// Baseline is also executable with psql; strip only its standalone outer markers.
		lines := strings.Split(string(body), "\n")
		for i, l := range lines {
			if strings.TrimSpace(l) == "BEGIN;" {
				lines[i] = ""
				break
			}
		}
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) == "COMMIT;" {
				lines[i] = ""
				break
			}
		}
		if _, e = tx.Exec(ctx, strings.Join(lines, "\n")); e == nil {
			_, e = tx.Exec(ctx, "INSERT INTO schema_migrations(version,name,checksum,applied_at_ms) VALUES($1,$2,$3,$4)", version, name, sum, NowMillis())
		}
		if e != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", name, e)
		}
		if e = tx.Commit(ctx); e != nil {
			return e
		}
	}
	var count int
	if err = conn.QueryRow(ctx, "SELECT cardinality(show_trgm('中文测试'))").Scan(&count); err != nil || count == 0 {
		return fmt.Errorf("Unicode pg_trgm capability probe failed: %w", err)
	}
	return nil
}
