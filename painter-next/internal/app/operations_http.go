package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"
)

type observedWriter struct {
	http.ResponseWriter
	status int
}

func (w *observedWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
		w.ResponseWriter.WriteHeader(code)
	}
}
func (w *observedWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *observedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// OperationsHandler assembles process/readiness probes and low-cardinality
// process/request metrics. It is installed once, by the CLI composition root.
func (a *App) OperationsHandler() http.Handler {
	var requests, failures, elapsed atomic.Int64
	a.Mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) { JSON(w, 200, map[string]any{"status": "alive"}) })
	a.Mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		var ready bool
		e := a.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations) AND cardinality(show_trgm('中文搜索'))>0").Scan(&ready)
		stat, se := os.Stat(a.Config.MediaDir)
		if e != nil || se != nil || !stat.IsDir() || !ready {
			JSON(w, 503, map[string]any{"status": "not_ready"})
			return
		}
		JSON(w, 200, map[string]any{"status": "ready", "database": "postgresql18"})
	})
	a.Handle("GET /metrics", func(w http.ResponseWriter, r *http.Request) error {
		if _, e := a.Require(r, "owner"); e != nil {
			return e
		}
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		p := a.Pool.Stat()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		_, e := fmt.Fprintf(w, "# TYPE painter_http_requests_total counter\npainter_http_requests_total %d\n# TYPE painter_http_failures_total counter\npainter_http_failures_total %d\n# TYPE painter_http_request_duration_seconds_sum counter\npainter_http_request_duration_seconds_sum %.6f\n# TYPE painter_go_heap_bytes gauge\npainter_go_heap_bytes %d\n# TYPE painter_db_acquired_connections gauge\npainter_db_acquired_connections %d\n# TYPE painter_db_idle_connections gauge\npainter_db_idle_connections %d\n", requests.Load(), failures.Load(), float64(elapsed.Load())/float64(time.Second), m.HeapAlloc, p.AcquiredConns(), p.IdleConns())
		return e
	})
	next := a.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		capture := &observedWriter{ResponseWriter: w}
		next.ServeHTTP(capture, r)
		requests.Add(1)
		if capture.status >= 500 {
			failures.Add(1)
		}
		elapsed.Add(int64(time.Since(start)))
	})
}

// Observe only durable completed operations. A local artifact and an isolated
// restore check are distinct from off-site replication, which is not inferred.
func (a *App) observedBackup(ctx context.Context) (map[string]any, error) {
	var id, dir, dbHash, manifestHash, origin string
	var objects, size, created int64
	var verified *int64
	e := a.Pool.QueryRow(ctx, `SELECT id,directory,database_sha256,manifest_sha256,media_objects,media_bytes,created_at_ms,restore_verified_at_ms,origin FROM backup_records ORDER BY created_at_ms DESC,id DESC LIMIT 1`).Scan(&id, &dir, &dbHash, &manifestHash, &objects, &size, &created, &verified, &origin)
	if errors.Is(e, pgx.ErrNoRows) {
		return map[string]any{"method": "local_cli", "status": "not_observed", "message": "尚无本地备份完成记录"}, nil
	}
	if e != nil {
		return nil, e
	}
	status, message := "artifact_created", "本地备份包已生成；尚未记录隔离恢复校验。"
	var at any
	if verified != nil {
		status = "restore_verified"
		message = "备份包已通过本地隔离恢复校验；异机同步由部署方另行确认。"
		at = TimeString(*verified)
	}
	return map[string]any{"method": "local_cli", "status": status, "message": message, "latest": map[string]any{"id": id, "name": filepath.Base(dir), "origin": origin, "created_at": TimeString(created), "database_sha256": dbHash, "manifest_sha256": manifestHash, "media_objects": objects, "media_bytes": size, "restore_verified_at": at}}, nil
}
