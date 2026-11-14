package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Each integration test gets a new database on an explicitly supplied test-only
// cluster. No DATABASE_URL or application database is ever used by tests.
func testApp(t *testing.T) *App {
	t.Helper()
	dsn := os.Getenv("PAINTER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PAINTER_TEST_DATABASE_URL to a disposable PostgreSQL 18 cluster")
	}
	ctx := context.Background()
	conn, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := "painter_test_" + RandID()[:20]
	if _, e = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE template0 ENCODING 'UTF8' LC_COLLATE 'C.UTF-8' LC_CTYPE 'C.UTF-8'"); e != nil {
		conn.Close(ctx)
		t.Fatal(e)
	}
	c, e := pgx.ParseConfig(dsn)
	if e != nil {
		t.Fatal(e)
	}
	c.Database = name
	testDSN := dsn + " dbname=" + name
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, e := url.Parse(dsn)
		if e != nil {
			t.Fatal(e)
		}
		u.Path = "/" + name
		testDSN = u.String()
	}
	config := Config{DatabaseURL: testDSN, Environment: "test", PublicBaseURL: "http://painter.test", MediaDir: t.TempDir(), EncryptionKey: bytes.Repeat([]byte{1}, 32), AnalyticsHMACKey: bytes.Repeat([]byte{2}, 32), GitHubClientID: "test-client", GitHubClientSecret: "test-secret"}
	a, e := New(ctx, config)
	if e != nil {
		conn.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		conn.Close(ctx)
		t.Fatal(e)
	}
	t.Cleanup(func() {
		a.Close()
		_, e := conn.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		if e != nil {
			t.Error(e)
		}
		conn.Close(ctx)
	})
	return a
}
func testIdentity(t *testing.T, a *App, role string) (Principal, *http.Cookie, string) {
	t.Helper()
	var id string
	now := NowMillis()
	if e := a.Pool.QueryRow(context.Background(), "INSERT INTO users(username,display_name,role,created_at_ms,updated_at_ms) VALUES($1,'Test User',$2,$3,$3) RETURNING id::text", "test-"+RandID()[:12], role, now).Scan(&id); e != nil {
		t.Fatal(e)
	}
	token := RandID()
	management := RandID()
	csrf := a.csrf(token)
	if _, e := a.Pool.Exec(context.Background(), "INSERT INTO sessions(token_hash,management_id,user_id,csrf_secret_hash,created_at_ms,expires_at_ms,last_seen_at_ms) VALUES($1,$2,$3,$4,$5,$6,$5)", hashString(token), management, id, hashString(csrf), now, now+sessionLifetime.Milliseconds()); e != nil {
		t.Fatal(e)
	}
	return Principal{ID: id, Role: role, SessionID: management}, &http.Cookie{Name: sessionCookie, Value: token}, csrf
}
func testRequest(a *App, method, path, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", a.Config.PublicBaseURL)
	r.Header.Set("X-CSRF-Token", csrf)
	r.Header.Set("Idempotency-Key", RandID())
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}
func jsonMap(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var v map[string]any
	if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	return v
}
func TestNormalizeAndReceipt(t *testing.T) {
	if got := Normalize("  ＨＥＬＬＯ\u3000Straße 中文\t"); got != "hello strasse 中文" {
		t.Fatal(got)
	}
	a := &App{Config: Config{AnalyticsHMACKey: bytes.Repeat([]byte{2}, 32)}}
	receipt := a.ReadReceipt("1", "revision")
	id, revision, e := a.VerifyReadReceipt(receipt)
	if e != nil || id != "1" || revision != "revision" {
		t.Fatal(id, revision, e)
	}
	if _, _, e = a.VerifyReadReceipt(receipt + "x"); e == nil {
		t.Fatal("tampered receipt accepted")
	}
}
func TestMigrationsAndChecksums(t *testing.T) {
	a := testApp(t)
	if e := Migrate(context.Background(), a.Pool); e != nil {
		t.Fatal(e)
	}
	if _, e := a.Pool.Exec(context.Background(), "UPDATE schema_migrations SET checksum='tampered'"); e != nil {
		t.Fatal(e)
	}
	if e := Migrate(context.Background(), a.Pool); e == nil {
		t.Fatal("changed checksum accepted")
	}
}
