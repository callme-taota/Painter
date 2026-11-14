package app

// Offline operator commands. They are deliberately not registered as HTTP routes.
// Maintenance uses the same PostgreSQL policy row as every writer, across processes.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type BackupObject struct {
	ID         string `json:"id"`
	StorageKey string `json:"storage_key"`
	SHA256     string `json:"sha256"`
	Bytes      int64  `json:"bytes"`
}
type BackupManifest struct {
	Version            int               `json:"version"`
	CreatedAt          string            `json:"created_at"`
	PostgresVersion    string            `json:"postgres_version"`
	DatabaseSHA256     string            `json:"database_sha256"`
	MigrationChecksums map[string]string `json:"migration_checksums"`
	Media              []BackupObject    `json:"media"`
}
type BackupReport struct {
	ID                      string `json:"id"`
	Directory               string `json:"directory"`
	CreatedAt               string `json:"created_at"`
	DatabaseSHA256          string `json:"database_sha256"`
	ManifestSHA256          string `json:"manifest_sha256"`
	MediaObjects            int    `json:"media_objects"`
	MediaBytes              int64  `json:"media_bytes"`
	MaintenanceMilliseconds int64  `json:"maintenance_ms"`
}
type RestoreReport struct {
	BackupID        string   `json:"backup_id"`
	Verified        bool     `json:"verified"`
	Database        string   `json:"temporary_database"`
	DatabaseDropped bool     `json:"temporary_database_dropped"`
	MediaObjects    int      `json:"media_objects"`
	PublicPosts     int64    `json:"public_posts"`
	PostgresVersion string   `json:"postgres_version"`
	Notes           []string `json:"notes"`
}

func pgBinary(name string) (string, error) {
	dir := os.Getenv("POSTGRES_BIN_DIR")
	if dir == "" {
		dir = os.Getenv("PG_BIN")
	}
	if dir != "" {
		p := filepath.Join(dir, name)
		s, e := os.Stat(p)
		if e != nil || s.IsDir() {
			return "", fmt.Errorf("PG_BIN lacks %s", name)
		}
		return p, nil
	}
	p, e := exec.LookPath(name)
	if e != nil {
		return "", fmt.Errorf("%s required (PostgreSQL 18 client tools)", name)
	}
	return p, nil
}
func pgRun(ctx context.Context, name, dsn string, args ...string) error {
	p, e := pgBinary(name)
	if e != nil {
		return e
	}
	probe := exec.CommandContext(ctx, p, "--version")
	out, e := probe.Output()
	if e != nil || !strings.Contains(string(out), "PostgreSQL) 18.") {
		return fmt.Errorf("%s must be PostgreSQL 18", name)
	}
	cmd := exec.CommandContext(ctx, p, args...)
	cc, parseErr := pgx.ParseConfig(dsn)
	if parseErr != nil {
		return parseErr
	}
	env := append(os.Environ(), "PGDATABASE="+cc.Database, "PGHOST="+cc.Host, "PGPORT="+strconv.Itoa(int(cc.Port)), "PGUSER="+cc.User, "PGPASSWORD="+cc.Password)
	sslmode := os.Getenv("PGSSLMODE")
	if sslmode == "" {
		sslmode = "prefer"
	}
	if cc.TLSConfig == nil {
		sslmode = "disable"
	}
	if u, err := url.Parse(dsn); err == nil && (u.Scheme == "postgres" || u.Scheme == "postgresql") {
		for key, envkey := range map[string]string{"sslmode": "PGSSLMODE", "sslrootcert": "PGSSLROOTCERT", "sslcert": "PGSSLCERT", "sslkey": "PGSSLKEY"} {
			if v := u.Query().Get(key); v != "" {
				env = append(env, envkey+"="+v)
				if key == "sslmode" {
					sslmode = v
				}
			}
		}
	} else {
		params := libpqKeywords(dsn)
		for key, envkey := range map[string]string{"sslmode": "PGSSLMODE", "sslrootcert": "PGSSLROOTCERT", "sslcert": "PGSSLCERT", "sslkey": "PGSSLKEY"} {
			if v := params[key]; v != "" {
				env = append(env, envkey+"="+v)
				if key == "sslmode" {
					sslmode = v
				}
			}
		}
	}
	if name == "pg_restore" {
		args = append([]string{"--dbname=" + cc.Database}, args...)
		cmd = exec.CommandContext(ctx, p, args...)
	}
	cmd.Env = append(env, "PGSSLMODE="+sslmode)
	if _, e = cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("%s failed: %w (provider output withheld because it may contain private data)", name, e)
	}
	return nil
}
func databaseDSN(dsn, name string) (string, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, e := url.Parse(dsn)
		if e != nil {
			return "", e
		}
		u.Path = "/" + name
		return u.String(), nil
	}
	if _, e := pgx.ParseConfig(dsn); e != nil {
		return "", e
	}
	return dsn + " dbname=" + name, nil
}
func (a *App) maintenance(ctx context.Context) (pgx.Tx, error) {
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	if _, e = tx.Exec(ctx, "SET LOCAL statement_timeout=0; SET LOCAL idle_in_transaction_session_timeout=0; SET LOCAL lock_timeout='30s'"); e == nil {
		_, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(7284923151123)")
		if e == nil {
			_, e = tx.Exec(ctx, "SELECT id FROM policy_barrier WHERE id=1 FOR UPDATE")
		}
	}
	if e != nil {
		tx.Rollback(context.Background())
		return nil, e
	}
	return tx, nil
}
func safeStorage(s string) bool {
	return s != "" && filepath.Base(s) == s && s != "." && s != ".." && !strings.ContainsAny(s, "/\\\x00")
}
func fileHash(path string) (string, int64, error) {
	info, e := os.Lstat(path)
	if e != nil {
		return "", 0, e
	}
	if !info.Mode().IsRegular() {
		return "", 0, fmt.Errorf("expected regular file without symlink")
	}
	f, e := os.Open(path)
	if e != nil {
		return "", 0, e
	}
	defer f.Close()
	s, e := f.Stat()
	if e != nil || !s.Mode().IsRegular() {
		return "", 0, fmt.Errorf("expected regular file")
	}
	h := sha256.New()
	n, e := io.Copy(h, f)
	if e != nil {
		return "", 0, e
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
func copyVerified(src, dst, expected string, size int64) error {
	info, e := os.Lstat(src)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source must be a regular file, not symlink")
	}
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	if e = os.MkdirAll(filepath.Dir(dst), 0700); e != nil {
		return e
	}
	out, e := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(out, h), in)
	if e == nil && (n != size || hex.EncodeToString(h.Sum(nil)) != expected) {
		e = fmt.Errorf("media checksum or byte size mismatch")
	}
	if e == nil {
		e = out.Sync()
	}
	closeErr := out.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		_ = os.Remove(dst)
	}
	return e
}
func writePrivateJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(append(b, '\n'))
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	return e
}
func syncDir(path string) error {
	d, e := os.Open(path)
	if e != nil {
		return e
	}
	defer d.Close()
	return d.Sync()
}

// Backup holds the maintenance gate for the entire dump+copy window. Public reads
// remain available; new writes/GC wait or return WRITE_BUSY. This is an explicit
// maintenance command, not a zero-pause backup claim. App keys are stored separately.
func (a *App) Backup(ctx context.Context, dir string) (BackupReport, error) {
	report := BackupReport{}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	dir, e := filepath.Abs(dir)
	if e != nil {
		return report, e
	}
	entries, e := os.ReadDir(dir)
	if e != nil && !errors.Is(e, fs.ErrNotExist) {
		return report, e
	}
	if len(entries) > 0 {
		return report, fmt.Errorf("backup directory must be new or empty")
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return report, e
	}
	if e = os.Chmod(dir, 0700); e != nil {
		return report, e
	}
	started := NowMillis()
	tx, e := a.maintenance(ctx)
	if e != nil {
		return report, e
	}
	defer tx.Rollback(context.Background())
	var snapshot, version string
	if e = tx.QueryRow(ctx, "SELECT pg_export_snapshot(),current_setting('server_version')").Scan(&snapshot, &version); e != nil {
		return report, e
	}
	manifest := BackupManifest{Version: 1, CreatedAt: TimeString(started), PostgresVersion: version, MigrationChecksums: map[string]string{}, Media: []BackupObject{}}
	rows, e := tx.Query(ctx, "SELECT version::text,checksum FROM schema_migrations ORDER BY version")
	if e != nil {
		return report, e
	}
	for rows.Next() {
		var v, sum string
		if e = rows.Scan(&v, &sum); e != nil {
			rows.Close()
			return report, e
		}
		manifest.MigrationChecksums[v] = sum
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return report, e
	}
	rows, e = tx.Query(ctx, "SELECT id,storage_key,sha256,byte_size FROM media_assets WHERE state='ready' ORDER BY id")
	if e != nil {
		return report, e
	}
	for rows.Next() {
		var object BackupObject
		if e = rows.Scan(&object.ID, &object.StorageKey, &object.SHA256, &object.Bytes); e != nil {
			rows.Close()
			return report, e
		}
		if !safeStorage(object.StorageKey) {
			rows.Close()
			return report, fmt.Errorf("unsafe media storage key")
		}
		manifest.Media = append(manifest.Media, object)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return report, e
	}
	dump := filepath.Join(dir, "database.dump")
	if e = pgRun(ctx, "pg_dump", a.Config.DatabaseURL, "--format=custom", "--no-owner", "--no-acl", "--snapshot="+snapshot, "--file="+dump); e != nil {
		return report, e
	}
	if e = os.Chmod(dump, 0600); e != nil {
		return report, e
	}
	sum, _, e := fileHash(dump)
	if e != nil {
		return report, e
	}
	manifest.DatabaseSHA256 = sum
	for _, o := range manifest.Media {
		if e = copyVerified(filepath.Join(a.Config.MediaDir, "objects", o.StorageKey), filepath.Join(dir, "media", "objects", o.StorageKey), o.SHA256, o.Bytes); e != nil {
			return report, fmt.Errorf("media %s: %w", o.ID, e)
		}
		report.MediaBytes += o.Bytes
	}
	if len(manifest.Media) > 0 {
		if e = syncDir(filepath.Join(dir, "media", "objects")); e != nil {
			return report, e
		}
		if e = syncDir(filepath.Join(dir, "media")); e != nil {
			return report, e
		}
	}

	if e = writePrivateJSON(filepath.Join(dir, "manifest.json"), manifest); e != nil {
		return report, e
	}
	if e = syncDir(dir); e != nil {
		return report, e
	}
	manifestSum, _, e := fileHash(filepath.Join(dir, "manifest.json"))
	if e != nil {
		return report, e
	}
	recordID := RandID()
	// Files and all directory entries are durable before recording success. A
	// commit failure can leave a valid artifact without a row, never a false success.
	if e = syncDir(filepath.Dir(dir)); e != nil {
		return report, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO backup_records(id,directory,database_sha256,manifest_sha256,media_objects,media_bytes,created_at_ms,origin) VALUES($1,$2,$3,$4,$5,$6,$7,'local')", recordID, dir, sum, manifestSum, len(manifest.Media), report.MediaBytes, started); e != nil {
		return report, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO audit_logs(action,target_type,target_id,request_id,reason,created_at_ms) VALUES('system.backup','system',$1,$2,'local artifact and manifest fsynced; isolated restore not yet verified',$3)", recordID, RandID(), NowMillis()); e != nil {
		return report, e
	}
	if e = tx.Commit(ctx); e != nil {
		return report, e
	}
	report.ID = recordID
	report.Directory = dir
	report.CreatedAt = manifest.CreatedAt
	report.DatabaseSHA256 = sum
	report.ManifestSHA256 = manifestSum
	report.MediaObjects = len(manifest.Media)
	report.MaintenanceMilliseconds = NowMillis() - started
	return report, nil
}
func readBackupManifest(dir string) (BackupManifest, error) {
	var m BackupManifest
	f, e := os.Open(filepath.Join(dir, "manifest.json"))
	if e != nil {
		return m, e
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 64<<20))
	d.DisallowUnknownFields()
	if e = d.Decode(&m); e != nil {
		return m, e
	}
	if e = d.Decode(new(any)); e != io.EOF {
		return m, fmt.Errorf("backup manifest must contain one JSON object")
	}
	created, createdErr := time.Parse(time.RFC3339Nano, m.CreatedAt)
	if createdErr != nil || created.UnixMilli() <= 0 {
		return m, fmt.Errorf("invalid backup creation timestamp")
	}
	if m.Version != 1 || len(m.DatabaseSHA256) != 64 || len(m.MigrationChecksums) == 0 {
		return m, fmt.Errorf("unsupported or incomplete backup manifest")
	}
	seen := map[string]bool{}
	keys := map[string]bool{}
	for _, o := range m.Media {
		if o.ID == "" || seen[o.ID] || !safeStorage(o.StorageKey) || keys[o.StorageKey] || len(o.SHA256) != 64 || o.Bytes < 0 {
			return m, fmt.Errorf("invalid media manifest entry")
		}
		seen[o.ID] = true
		keys[o.StorageKey] = true
	}
	return m, nil
}

// RestoreCheck actually restores into a random isolated database, validates the
// resulting constraints/projection/media mapping, and drops it. It never replaces
// a running application DB or silently installs application encryption keys.
func (a *App) RestoreCheck(ctx context.Context, dir string) (report RestoreReport, err error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	dir, e := filepath.Abs(dir)
	if e != nil {
		return report, e
	}
	m, e := readBackupManifest(dir)
	if e != nil {
		return report, e
	}
	manifestSHA, _, e := fileHash(filepath.Join(dir, "manifest.json"))
	if e != nil {
		return report, e
	}
	var mediaBytes int64
	for _, o := range m.Media {
		if o.Bytes > 0 && mediaBytes > (1<<63-1)-o.Bytes {
			return report, fmt.Errorf("media manifest byte total overflow")
		}
		mediaBytes += o.Bytes
	}
	validationPassed := false
	sum, _, e := fileHash(filepath.Join(dir, "database.dump"))
	if e != nil || sum != m.DatabaseSHA256 {
		return report, fmt.Errorf("database dump checksum mismatch")
	}
	for _, o := range m.Media {
		sum, n, e := fileHash(filepath.Join(dir, "media", "objects", o.StorageKey))
		if e != nil || sum != o.SHA256 || n != o.Bytes {
			return report, fmt.Errorf("media %s checksum mismatch", o.ID)
		}
	}
	adminDSN := a.Config.DatabaseURL
	if v := os.Getenv("MAINTENANCE_DATABASE_URL"); v != "" {
		adminDSN = v
	}
	admin, e := pgx.Connect(ctx, adminDSN)
	if e != nil {
		return report, e
	}
	defer admin.Close(context.Background())
	name := "painter_restore_" + RandID()[:20]
	report.Database = name
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+" TEMPLATE template0 ENCODING 'UTF8' LC_COLLATE 'C.UTF-8' LC_CTYPE 'C.UTF-8'"); e != nil {
		return report, fmt.Errorf("restore-check requires a maintenance role with CREATEDB: %w", e)
	}
	defer func() {
		clean, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, dropErr := admin.Exec(clean, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		report.DatabaseDropped = dropErr == nil
		if dropErr != nil {
			if err == nil {
				err = fmt.Errorf("verified temporary database could not be dropped: %w", dropErr)
			}
			return
		}
		if err != nil || !validationPassed {
			return
		}
		recordID, recordErr := a.recordRestoreObservation(clean, dir, manifestSHA, m, mediaBytes)
		if recordErr != nil {
			err = fmt.Errorf("restore verified but observation could not be committed: %w", recordErr)
			return
		}
		report.BackupID = recordID
		report.Verified = true
	}()
	target, e := databaseDSN(adminDSN, name)
	if e != nil {
		return report, e
	}
	if e = pgRun(ctx, "pg_restore", target, "--no-owner", "--no-acl", "--exit-on-error", filepath.Join(dir, "database.dump")); e != nil {
		return report, e
	}
	restored, e := pgx.Connect(ctx, target)
	if e != nil {
		return report, e
	}
	defer restored.Close(context.Background())
	var version int
	if e = restored.QueryRow(ctx, "SELECT current_setting('server_version_num')::int,current_setting('server_version')").Scan(&version, &report.PostgresVersion); e != nil {
		return report, e
	}
	if version < 180000 || version >= 190000 {
		return report, fmt.Errorf("restored server must be PostgreSQL 18")
	}
	var invalid int
	if e = restored.QueryRow(ctx, "SELECT count(*) FROM pg_constraint WHERE connamespace='public'::regnamespace AND NOT convalidated").Scan(&invalid); e != nil || invalid != 0 {
		return report, fmt.Errorf("unvalidated restored constraints")
	}
	for v, sum := range m.MigrationChecksums {
		var actual string
		if e = restored.QueryRow(ctx, "SELECT checksum FROM schema_migrations WHERE version=$1", v).Scan(&actual); e != nil || actual != sum {
			return report, fmt.Errorf("restored migration checksum mismatch")
		}
	}
	var mismatch int
	e = restored.QueryRow(ctx, `SELECT (SELECT count(*) FROM search_documents s LEFT JOIN public_posts p ON p.post_id=s.post_id WHERE p.post_id IS NULL OR p.revision_id<>s.revision_id)+(SELECT count(*) FROM public_posts p WHERE NOT EXISTS(SELECT 1 FROM search_documents s WHERE s.post_id=p.post_id))`).Scan(&mismatch)
	if e != nil || mismatch != 0 {
		return report, fmt.Errorf("restored public search projection inconsistent")
	}
	if e = restored.QueryRow(ctx, "SELECT count(*) FROM public_posts").Scan(&report.PublicPosts); e != nil {
		return report, e
	}
	byID := map[string]BackupObject{}
	for _, o := range m.Media {
		byID[o.ID] = o
	}
	rows, e := restored.Query(ctx, "SELECT id,storage_key,sha256,byte_size FROM media_assets WHERE state='ready' ORDER BY id")
	if e != nil {
		return report, e
	}
	for rows.Next() {
		var o BackupObject
		if e = rows.Scan(&o.ID, &o.StorageKey, &o.SHA256, &o.Bytes); e != nil {
			rows.Close()
			return report, e
		}
		if got, ok := byID[o.ID]; !ok || got != o {
			rows.Close()
			return report, fmt.Errorf("ready media missing from restore manifest")
		}
		delete(byID, o.ID)
		report.MediaObjects++
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return report, e
	}
	if len(byID) > 0 {
		return report, fmt.Errorf("manifest contains extra media outside restored ready set")
	}
	validationPassed = true
	report.Notes = []string{"Isolated pg_restore and ready-media checksums verified; no production database was replaced.", "Keep APP_ENCRYPTION_KEY and ANALYTICS_HMAC_KEY in a separately protected recovery channel.", "Staging/deleting objects are intentionally not copied; reconciliation completes their interrupted lifecycle after an operator restore."}
	return report, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Parse only already driver-validated libpq keyword syntax, preserving quoted TLS
// file paths for the command-line client's environment without putting passwords
// or a secret-bearing DSN in process arguments.
func libpqKeywords(s string) map[string]string {
	out := map[string]string{}
	i := 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
			i++
		}
		start := i
		for i < len(s) && s[i] != '=' && s[i] != ' ' {
			i++
		}
		key := s[start:i]
		for i < len(s) && s[i] == ' ' {
			i++
		}
		if i >= len(s) || s[i] != '=' {
			break
		}
		i++
		for i < len(s) && s[i] == ' ' {
			i++
		}
		var b strings.Builder
		quoted := i < len(s) && s[i] == '\''
		if quoted {
			i++
		}
		for i < len(s) {
			c := s[i]
			i++
			if c == '\\' && i < len(s) {
				b.WriteByte(s[i])
				i++
				continue
			}
			if quoted && c == '\'' {
				break
			}
			if !quoted && (c == ' ' || c == '\t' || c == '\n') {
				break
			}
			b.WriteByte(c)
		}
		out[key] = b.String()
	}
	return out
}

// A restore observation is committed only after both validation and temporary DB
// deletion succeed. Existing directory/origin/creation history remains immutable.
func (a *App) recordRestoreObservation(ctx context.Context, dir, manifestSHA string, m BackupManifest, mediaBytes int64) (string, error) {
	created, e := time.Parse(time.RFC3339Nano, m.CreatedAt)
	if e != nil {
		return "", e
	}
	id := RandID()
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, "SELECT id FROM policy_barrier WHERE id=1 FOR SHARE"); e != nil {
		return "", e
	}
	e = tx.QueryRow(ctx, `INSERT INTO backup_records(id,directory,database_sha256,manifest_sha256,media_objects,media_bytes,created_at_ms,restore_verified_at_ms,origin) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'imported')
 ON CONFLICT(manifest_sha256) DO UPDATE SET restore_verified_at_ms=GREATEST(backup_records.restore_verified_at_ms,EXCLUDED.restore_verified_at_ms)
 WHERE backup_records.database_sha256=EXCLUDED.database_sha256 AND backup_records.media_objects=EXCLUDED.media_objects AND backup_records.media_bytes=EXCLUDED.media_bytes AND backup_records.created_at_ms=EXCLUDED.created_at_ms
 RETURNING id`, id, dir, m.DatabaseSHA256, manifestSHA, len(m.Media), mediaBytes, created.UnixMilli(), NowMillis()).Scan(&id)
	if e == pgx.ErrNoRows {
		return "", fmt.Errorf("existing backup record metadata differs from manifest")
	}
	if e != nil {
		return "", e
	}
	return id, tx.Commit(ctx)
}
