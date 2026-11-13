package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestRecoveryStagePreservesOriginalAndRevokesRestoredSessions(t *testing.T) {
	if _, err := pgBinary("pg_dump"); err != nil {
		local := "/tmp/painter-next-pg18/root/usr/lib/postgresql/18/bin"
		if _, err = os.Stat(filepath.Join(local, "pg_dump")); err != nil {
			t.Skip("PostgreSQL 18 client tools required")
		}
		t.Setenv("POSTGRES_BIN_DIR", local)
	}
	a := testApp(t)
	ctx := context.Background()
	path, _ := legacyFixture(t)
	if _, err := a.ImportLegacy(ctx, path, false); err != nil {
		t.Fatal(err)
	}
	_, _, _ = testIdentity(t, a, "owner")
	backup, err := a.Backup(ctx, filepath.Join(t.TempDir(), "backup"))
	if err != nil {
		t.Fatal(err)
	}
	target := "painter_recovery_" + RandID()[:20]
	media := filepath.Join(t.TempDir(), "recovered-media")
	report, err := a.RestoreStage(ctx, RecoveryOptions{Source: backup.Directory, Database: target, MediaDir: media})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := a.Pool.Exec(ctx, "DROP DATABASE "+pgx.Identifier{target}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	})
	if !report.Verified || !report.MigrationsVerified || report.SchemaVersion < 1 || report.Activated || !report.OriginalDatabasePreserved || report.MediaObjects != 1 || report.SessionsRevoked != 1 {
		t.Fatal(report)
	}
	targetDSN, err := databaseDSN(a.Config.DatabaseURL, target)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := pgx.Connect(ctx, targetDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close(ctx)
	var count int
	if err = restored.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE revoked_at_ms IS NULL").Scan(&count); err != nil || count != 0 {
		t.Fatal("restored bearer session revived", count, err)
	}
	if err = a.Pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE revoked_at_ms IS NULL").Scan(&count); err != nil || count != 1 {
		t.Fatal("original sessions changed", count, err)
	}
	if err = restored.QueryRow(ctx, "SELECT count(*) FROM public_posts").Scan(&count); err != nil || count != 1 {
		t.Fatal("public content not restored", count, err)
	}
	manifest, err := readBackupManifest(backup.Directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, object := range manifest.Media {
		sum, size, err := fileHash(filepath.Join(media, "objects", object.StorageKey))
		if err != nil || sum != object.SHA256 || size != object.Bytes {
			t.Fatal("restored media wrong", err)
		}
	}
	if _, err = a.RestoreStage(ctx, RecoveryOptions{Source: backup.Directory, Database: target, MediaDir: filepath.Join(t.TempDir(), "another")}); err == nil {
		t.Fatal("existing recovery database overwritten")
	}
	if _, err = a.RestoreStage(ctx, RecoveryOptions{Source: backup.Directory, Database: "painter_recovery_" + RandID()[:20], MediaDir: media}); err == nil {
		t.Fatal("existing recovery media overwritten")
	}
}
func TestRecoveryTargetValidation(t *testing.T) {
	for _, name := range []string{"painter", "painter_recovery_", "painter_recovery_../bad", "painter_recovery_NAME", "painter_recovery_bad;drop"} {
		if recoveryName(name) {
			t.Fatal("unsafe recovery name accepted", name)
		}
	}
	if !recoveryName("painter_recovery_a0123") {
		t.Fatal("valid recovery name rejected")
	}
}

func TestRecoveryReplaysRuntimeRestrictionsAfterDefaultGrants(t *testing.T) {
	if _, err := pgBinary("pg_dump"); err != nil {
		t.Skip("PostgreSQL 18 client tools required")
	}
	a := testApp(t)
	ctx := context.Background()
	suffix := RandID()[:12]
	runtimeRole := "painter_recover_app_" + suffix
	migrationRole := "painter_recover_migr_" + suffix
	for _, role := range []string{runtimeRole, migrationRole} {
		if _, err := a.Pool.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" NOLOGIN"); err != nil {
			t.Fatal(err)
		}
	}
	target := "painter_recovery_" + RandID()[:20]
	t.Cleanup(func() {
		_, _ = a.Pool.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{target}.Sanitize()+" WITH (FORCE)")
		for _, role := range []string{runtimeRole, migrationRole} {
			_, _ = a.Pool.Exec(ctx, "DROP OWNED BY "+pgx.Identifier{role}.Sanitize()+" CASCADE")
			if _, err := a.Pool.Exec(ctx, "DROP ROLE "+pgx.Identifier{role}.Sanitize()); err != nil {
				t.Error(err)
			}
		}
	})
	if _, err := a.Pool.Exec(ctx, "GRANT SELECT ON ALL TABLES IN SCHEMA public TO "+pgx.Identifier{runtimeRole}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Pool.Exec(ctx, "ALTER DEFAULT PRIVILEGES FOR ROLE "+pgx.Identifier{migrationRole}.Sanitize()+" IN SCHEMA public GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO "+pgx.Identifier{runtimeRole}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	backup, err := a.Backup(ctx, filepath.Join(t.TempDir(), "backup"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.RestoreStage(ctx, RecoveryOptions{Source: backup.Directory, Database: target, MediaDir: filepath.Join(t.TempDir(), "media"), RuntimeRole: runtimeRole, MigrationRole: migrationRole}); err != nil {
		t.Fatal(err)
	}
	dsn, err := databaseDSN(a.Config.DatabaseURL, target)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close(ctx)
	var selectAllowed, insertAllowed, deleteAllowed bool
	if err = restored.QueryRow(ctx, "SELECT has_table_privilege($1,'schema_migrations','SELECT'),has_table_privilege($1,'schema_migrations','INSERT'),has_table_privilege($1,'backup_records','DELETE')", runtimeRole).Scan(&selectAllowed, &insertAllowed, &deleteAllowed); err != nil {
		t.Fatal(err)
	}
	if !selectAllowed || insertAllowed || deleteAllowed {
		t.Fatal("restore broadened runtime role privileges", selectAllowed, insertAllowed, deleteAllowed)
	}
}
