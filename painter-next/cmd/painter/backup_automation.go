package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	"painter-next/internal/app"
	"painter-next/internal/backupops"
)

// This command has no HTTP route. Only the local deployment owner/maintenance
// role can operate snapshots or request a new recovery database.
func runBackupCommand(ctx context.Context, a *app.App, command string, args []string) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", "", "private automatic backup root")
	keep := flags.Int("keep", 14, "number of verified automatic backups to retain")
	config := flags.String("config-snapshot", "", "private deployment environment file to include")
	snapshot := flags.String("snapshot", "", "automatic backup directory name")
	if err := flags.Parse(args); err != nil {
		return errors.New("invalid backup options; use --root DIR [--keep 14 --config-snapshot FILE] or --snapshot NAME")
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional backup argument")
	}
	if command == "backup-status" {
		state, err := backupops.Status(*root)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(state)
	}
	if a == nil {
		return errors.New("backup command requires a database connection")
	}
	hooks := backupops.Hooks{
		Backup: func(ctx context.Context, path string) (json.RawMessage, error) {
			report, err := a.Backup(ctx, path)
			if err != nil {
				return nil, err
			}
			return json.Marshal(report)
		},
		Check: func(ctx context.Context, path string) (json.RawMessage, error) {
			report, err := a.RestoreCheck(ctx, path)
			if err != nil {
				return nil, err
			}
			return json.Marshal(report)
		},
	}
	var report any
	var err error
	switch command {
	case "backup-run":
		report, err = backupops.Run(ctx, backupops.Options{Root: *root, Keep: *keep, ConfigSnapshot: *config}, hooks)
	case "backup-check":
		report, err = backupops.Check(ctx, *root, *snapshot, hooks)
	default:
		return errors.New("unknown backup automation command")
	}
	if errors.Is(err, backupops.ErrBusy) {
		return errors.New("backup skipped: another backup/recovery operation is running")
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}

func runRecoveryStageCommand(ctx context.Context, a *app.App, args []string) error {
	flags := flag.NewFlagSet("restore-stage", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	source := flags.String("source", "", "existing complete backup directory")
	database := flags.String("database", "", "new database name (painter_recovery_ prefix)")
	media := flags.String("media", "", "new private media directory")
	runtimeRole := flags.String("runtime-role", "", "runtime PostgreSQL role (defaults to current configured role)")
	migrationRole := flags.String("migration-role", "", "database object owner (defaults to maintenance role)")
	if err := flags.Parse(args); err != nil {
		return errors.New("invalid restore-stage options: --source DIR --database painter_recovery_NAME --media NEW_DIR [--runtime-role ROLE --migration-role ROLE]")
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional recovery argument")
	}
	report, err := a.RestoreStage(ctx, app.RecoveryOptions{Source: *source, Database: *database, MediaDir: *media, RuntimeRole: *runtimeRole, MigrationRole: *migrationRole})
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
