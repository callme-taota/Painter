package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"
)

// This probe checks database/driver capabilities only, not application migrations.
func main() {
	dsn := os.Getenv("PAINTER_TEST_DATABASE_URL")
	if dsn == "" {
		panic("set PAINTER_TEST_DATABASE_URL to an isolated PostgreSQL test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		panic(err)
	}
	defer db.Close(ctx)
	var version, encoding, ctype string
	err = db.QueryRow(ctx, "SELECT version(), current_setting('server_encoding'), datctype FROM pg_database WHERE datname=current_database()").Scan(&version, &encoding, &ctype)
	if err != nil {
		panic(err)
	}
	var trigrams int
	if err = db.QueryRow(ctx, "SELECT cardinality(show_trgm('中文测试'))").Scan(&trigrams); err != nil {
		panic(err)
	}
	if encoding != "UTF8" || trigrams == 0 {
		panic("Unicode database/trigram capability probe failed")
	}
	var payload map[string]any
	var bytes []byte
	var enabled bool
	var milliseconds int64
	err = db.QueryRow(ctx, "SELECT $1::jsonb,$2::bytea,$3::boolean,$4::bigint", json.RawMessage(`{"value":42}`), []byte{1, 2, 3}, true, int64(1792429200000)).Scan(&payload, &bytes, &enabled, &milliseconds)
	if err != nil {
		panic(err)
	}
	if payload["value"] != float64(42) || len(bytes) != 3 || !enabled || milliseconds != 1792429200000 {
		panic("native parameter roundtrip failed")
	}
	out := map[string]any{"driver": "github.com/jackc/pgx/v5 v5.11.0", "go": runtime.Version(), "server": version, "encoding": encoding, "lc_ctype": ctype, "chinese_trigrams": trigrams, "native_jsonb_bytea_boolean_bigint": "passed"}
	raw, _ := json.Marshal(out)
	fmt.Println(string(raw))
}
