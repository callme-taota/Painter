package app

// Recovery is deliberately offline and additive. This stage never changes the
// application's configured DSN, public URL, reverse proxy, or Tunnel credentials.
import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RecoveryOptions struct {
	Source        string
	Database      string
	MediaDir      string
	RuntimeRole   string
	MigrationRole string
}
type RecoveryReport struct {
	Database                  string `json:"database"`
	MediaDir                  string `json:"media_directory"`
	MediaObjects              int    `json:"media_objects"`
	SessionsRevoked           int64  `json:"sessions_revoked"`
	Verified                  bool   `json:"verified"`
	Activated                 bool   `json:"activated"`
	RuntimeRole               string `json:"runtime_role"`
	OriginalDatabasePreserved bool   `json:"original_database_preserved"`
	SchemaVersion             int64  `json:"schema_version"`
	MigrationsVerified        bool   `json:"migrations_verified"`
}

func recoveryName(name string) bool {
	if !strings.HasPrefix(name, "painter_recovery_") || len(name) > 63 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return len(name) > len("painter_recovery_")
}
func ensureRecoveryDirectory(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	walk := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(filepath.Dir(absolute), string(filepath.Separator)), string(filepath.Separator)) {
		walk = filepath.Join(walk, part)
		info, e := os.Lstat(walk)
		if errors.Is(e, os.ErrNotExist) {
			if e = os.Mkdir(walk, 0700); e != nil {
				return e
			}
			continue
		}
		if e != nil {
			return e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("recovery media ancestors must be real directories")
		}
	}
	return os.Mkdir(absolute, 0700) // Exclusive: an existing/empty/symlink target is refused.
}

type recoveryGrant struct {
	Table, Privilege string
	Sequence         bool
}

// Capture the current deployment's effective runtime permissions, including its
// explicit restrictions. A recovered dump deliberately contains no original ACL.
func (a *App) recoveryGrants(ctx context.Context, role string) ([]recoveryGrant, error) {
	rows, err := a.Pool.Query(ctx, `SELECT c.relname,p.privilege,false FROM pg_class c CROSS JOIN (VALUES ('SELECT'),('INSERT'),('UPDATE'),('DELETE'),('TRUNCATE'),('REFERENCES'),('TRIGGER')) p(privilege) WHERE c.relnamespace='public'::regnamespace AND c.relkind IN ('r','p','v','m','f') AND has_table_privilege($1,c.oid,p.privilege)
 UNION ALL SELECT c.relname,p.privilege,true FROM pg_class c CROSS JOIN (VALUES ('USAGE'),('SELECT'),('UPDATE')) p(privilege) WHERE c.relnamespace='public'::regnamespace AND c.relkind='S' AND has_sequence_privilege($1,c.oid,p.privilege)`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := []recoveryGrant{}
	for rows.Next() {
		var g recoveryGrant
		if err = rows.Scan(&g.Table, &g.Privilege, &g.Sequence); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}
func (a *App) RestoreStage(ctx context.Context, options RecoveryOptions) (report RecoveryReport, err error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if options.Source == "" || options.MediaDir == "" || !recoveryName(options.Database) {
		return report, errors.New("source, new media directory, and a new painter_recovery_ database name required")
	}
	options.Source, err = filepath.Abs(options.Source)
	if err != nil {
		return report, err
	}
	options.MediaDir, err = filepath.Abs(options.MediaDir)
	if err != nil {
		return report, err
	}
	liveMedia, liveErr := filepath.Abs(a.Config.MediaDir)
	if liveErr != nil {
		return report, liveErr
	}
	if options.MediaDir == liveMedia || strings.HasPrefix(options.MediaDir, liveMedia+string(filepath.Separator)) || strings.HasPrefix(liveMedia, options.MediaDir+string(filepath.Separator)) || options.MediaDir == options.Source || strings.HasPrefix(options.MediaDir, options.Source+string(filepath.Separator)) {
		return report, errors.New("recovery media must be separate from live media and the backup")
	}
	current, err := pgx.ParseConfig(a.Config.DatabaseURL)
	if err != nil {
		return report, errors.New("invalid runtime database configuration")
	}
	if current.Database == options.Database {
		return report, errors.New("recovery must not target the live database")
	}
	if options.RuntimeRole == "" {
		options.RuntimeRole = current.User
	}
	adminDSN := a.Config.DatabaseURL
	if value := os.Getenv("MAINTENANCE_DATABASE_URL"); value != "" {
		adminDSN = value
	}
	maintenance, err := pgx.ParseConfig(adminDSN)
	if err != nil {
		return report, errors.New("invalid maintenance database configuration")
	}
	if maintenance.Host != current.Host || maintenance.Port != current.Port {
		return report, errors.New("maintenance and runtime database must use the same PostgreSQL cluster")
	}
	if options.MigrationRole == "" {
		options.MigrationRole = maintenance.User
	}
	// The same well-tested real restoration primitive verifies hashes, constraints,
	// schema checksums, projections, and ready-media references before staging.
	checked, err := a.RestoreCheck(ctx, options.Source)
	if err != nil {
		return report, err
	}
	if !checked.Verified || !checked.DatabaseDropped {
		return report, errors.New("isolated restore check did not finish")
	}
	manifest, err := readBackupManifest(options.Source)
	if err != nil {
		return report, err
	}
	grants, err := a.recoveryGrants(ctx, options.RuntimeRole)
	if err != nil {
		return report, fmt.Errorf("cannot capture runtime role permissions: %w", err)
	}
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		return report, err
	}
	defer admin.Close(context.Background())
	var roles int
	if err = admin.QueryRow(ctx, "SELECT count(*) FROM pg_roles WHERE rolname=ANY($1::text[])", []string{options.RuntimeRole, options.MigrationRole}).Scan(&roles); err != nil {
		return report, err
	}
	expected := 2
	if options.RuntimeRole == options.MigrationRole {
		expected = 1
	}
	if roles != expected {
		return report, errors.New("runtime and migration roles must already exist on the recovery cluster")
	}
	if err = ensureRecoveryDirectory(options.MediaDir); err != nil {
		return report, fmt.Errorf("new recovery media directory required: %w", err)
	}
	createdDatabase := false
	success := false
	defer func() {
		if success {
			return
		}
		clean, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if createdDatabase {
			_, dropErr := admin.Exec(clean, "DROP DATABASE "+pgx.Identifier{options.Database}.Sanitize()+" WITH (FORCE)")
			if dropErr != nil {
				err = errors.Join(err, errors.New("failed recovery database needs explicit operator cleanup"))
			}
		}
		if removeErr := os.RemoveAll(options.MediaDir); removeErr != nil {
			err = errors.Join(err, errors.New("failed recovery media needs explicit operator cleanup"))
		}
	}()
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{options.Database}.Sanitize()+" OWNER "+pgx.Identifier{options.MigrationRole}.Sanitize()+" TEMPLATE template0 ENCODING 'UTF8' LC_COLLATE 'C.UTF-8' LC_CTYPE 'C.UTF-8'")
	if err != nil {
		return report, fmt.Errorf("cannot create new recovery database; no existing database replaced: %w", err)
	}
	createdDatabase = true
	target, err := databaseDSN(adminDSN, options.Database)
	if err != nil {
		return report, err
	}
	if err = pgRun(ctx, "pg_restore", target, "--no-owner", "--no-acl", "--exit-on-error", "--role="+options.MigrationRole, filepath.Join(options.Source, "database.dump")); err != nil {
		return report, err
	}
	// A restored archive can predate this binary. Run migrations as the object
	// owner on the NEW database before runtime readiness; the runtime role need
	// not have DDL privileges. A separate one-connection pool pins SET ROLE.
	migrationConfig, parseErr := pgxpool.ParseConfig(target)
	if parseErr != nil {
		return report, parseErr
	}
	migrationConfig.MaxConns = 1
	migrationConfig.AfterConnect = func(ctx context.Context, connection *pgx.Conn) error {
		_, setErr := connection.Exec(ctx, "SET ROLE "+pgx.Identifier{options.MigrationRole}.Sanitize())
		return setErr
	}
	migrationPool, poolErr := pgxpool.NewWithConfig(ctx, migrationConfig)
	if poolErr != nil {
		return report, poolErr
	}
	migrationErr := Migrate(ctx, migrationPool)
	if migrationErr == nil {
		migrationErr = migrationPool.QueryRow(ctx, "SELECT max(version) FROM schema_migrations").Scan(&report.SchemaVersion)
	}
	migrationPool.Close()
	if migrationErr != nil {
		return report, fmt.Errorf("new recovery database migrations failed; active database unchanged: %w", migrationErr)
	}
	report.MigrationsVerified = true
	restored, err := pgx.Connect(ctx, target)
	if err != nil {
		return report, err
	}
	defer restored.Close(context.Background())
	tx, err := restored.Begin(ctx)
	if err != nil {
		return report, err
	}
	defer tx.Rollback(context.Background())
	// Passwords, TOTP bindings, content and settings survive; bearer sessions and
	// in-flight OAuth attempts do not become valid again after rollback/recovery.
	tag, err := tx.Exec(ctx, "UPDATE sessions SET revoked_at_ms=$1 WHERE revoked_at_ms IS NULL", NowMillis())
	if err != nil {
		return report, err
	}
	report.SessionsRevoked = tag.RowsAffected()
	if _, err = tx.Exec(ctx, "DELETE FROM oauth_states"); err != nil {
		return report, err
	}
	// Older backups may predate MFA/OIDC. Preserve long-lived bindings and
	// signing keys, but never resurrect temporary challenges or old grants.
	for _, item := range []struct{ table, statement string }{
		{"mfa_login_challenges", "DELETE FROM mfa_login_challenges"},
		{"identity_grants", "UPDATE identity_grants SET revoked_at_ms=$1 WHERE revoked_at_ms IS NULL"},
		{"identity_auth_requests", "DELETE FROM identity_auth_requests"},
	} {
		var relation *string
		if err = tx.QueryRow(ctx, "SELECT to_regclass($1)::text", "public."+item.table).Scan(&relation); err != nil {
			return report, err
		}
		if relation == nil {
			continue
		}
		if strings.Contains(item.statement, "$1") {
			_, err = tx.Exec(ctx, item.statement, NowMillis())
		} else {
			_, err = tx.Exec(ctx, item.statement)
		}
		if err != nil {
			return report, err
		}
	}
	if _, err = tx.Exec(ctx, "GRANT CONNECT ON DATABASE "+pgx.Identifier{options.Database}.Sanitize()+" TO "+pgx.Identifier{options.RuntimeRole}.Sanitize()); err != nil {
		return report, err
	}
	if _, err = tx.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+pgx.Identifier{options.RuntimeRole}.Sanitize()); err != nil {
		return report, err
	}
	// Existing cluster-wide default privileges can grant broad DML while the
	// archive is restored. Reset direct runtime grants before replaying the
	// source deployment's effective restrictions (e.g. no ledger writes).
	if options.RuntimeRole != options.MigrationRole {
		for _, kind := range []string{"TABLES", "SEQUENCES"} {
			if _, err = tx.Exec(ctx, "REVOKE ALL ON ALL "+kind+" IN SCHEMA public FROM "+pgx.Identifier{options.RuntimeRole}.Sanitize()); err != nil {
				return report, err
			}
		}
	}
	for _, g := range grants {
		var relation *string
		if err = tx.QueryRow(ctx, "SELECT to_regclass($1)::text", pgx.Identifier{"public", g.Table}.Sanitize()).Scan(&relation); err != nil {
			return report, err
		}
		if relation == nil {
			continue
		} // A backup may predate the current schema.
		kind := "TABLE"
		if g.Sequence {
			kind = "SEQUENCE"
		}
		// Catalog names are quoted; privilege strings come only from the SQL VALUES.
		if _, err = tx.Exec(ctx, "GRANT "+g.Privilege+" ON "+kind+" "+pgx.Identifier{"public", g.Table}.Sanitize()+" TO "+pgx.Identifier{options.RuntimeRole}.Sanitize()); err != nil {
			return report, err
		}
	}
	if _, err = tx.Exec(ctx, "ALTER DEFAULT PRIVILEGES FOR ROLE "+pgx.Identifier{options.MigrationRole}.Sanitize()+" IN SCHEMA public GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO "+pgx.Identifier{options.RuntimeRole}.Sanitize()); err != nil {
		return report, err
	}
	if _, err = tx.Exec(ctx, "ALTER DEFAULT PRIVILEGES FOR ROLE "+pgx.Identifier{options.MigrationRole}.Sanitize()+" IN SCHEMA public GRANT USAGE,SELECT ON SEQUENCES TO "+pgx.Identifier{options.RuntimeRole}.Sanitize()); err != nil {
		return report, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO audit_logs(action,target_type,target_id,request_id,reason,created_at_ms) VALUES('system.backup','system',$1,$2,'restored to a new database and media directory; previous bearer sessions revoked; activation pending',$3)", options.Database, RandID(), NowMillis()); err != nil {
		return report, err
	}
	if err = tx.Commit(ctx); err != nil {
		return report, err
	}
	for _, object := range manifest.Media {
		if err = copyVerified(filepath.Join(options.Source, "media", "objects", object.StorageKey), filepath.Join(options.MediaDir, "objects", object.StorageKey), object.SHA256, object.Bytes); err != nil {
			return report, err
		}
		report.MediaObjects++
	}
	if err = os.MkdirAll(filepath.Join(options.MediaDir, "objects"), 0700); err != nil {
		return report, err
	}
	if err = syncDir(filepath.Join(options.MediaDir, "objects")); err != nil {
		return report, err
	}
	if err = syncDir(options.MediaDir); err != nil {
		return report, err
	}
	if err = syncDir(filepath.Dir(options.MediaDir)); err != nil {
		return report, err
	}
	report.Database = options.Database
	report.MediaDir = options.MediaDir
	report.RuntimeRole = options.RuntimeRole
	report.OriginalDatabasePreserved = true
	report.Verified = true
	report.Activated = false
	success = true
	return report, nil
}
