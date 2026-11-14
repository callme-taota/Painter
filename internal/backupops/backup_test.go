package backupops

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func fakeHooks() Hooks {
	return Hooks{
		Backup: func(_ context.Context, path string) (json.RawMessage, error) {
			if err := os.Mkdir(path, 0700); err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(path, "database.dump"), []byte("test archive"), 0600); err != nil {
				return nil, err
			}
			return json.RawMessage(`{"id":"backup","database_sha256":"checksum"}`), nil
		},
		Check: func(_ context.Context, path string) (json.RawMessage, error) {
			if _, err := os.Stat(filepath.Join(path, "database.dump")); err != nil {
				return nil, err
			}
			return json.RawMessage(`{"verified":true,"temporary_database_dropped":true}`), nil
		},
	}
}
func TestVerifiedRetentionPreservesForeignAndFailedDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "automatic")
	hooks := fakeHooks()
	options := Options{Root: root, Keep: 2}
	first, err := Run(context.Background(), options, hooks)
	if err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(root, "manual-backup")
	if err = os.Mkdir(foreign, 0700); err != nil {
		t.Fatal(err)
	}
	incomplete := filepath.Join(root, "auto-incomplete")
	if err = os.Mkdir(incomplete, 0700); err != nil {
		t.Fatal(err)
	}
	second, err := Run(context.Background(), options, hooks)
	if err != nil {
		t.Fatal(err)
	}
	hooks.Check = func(context.Context, string) (json.RawMessage, error) {
		return nil, errors.New("injected failure with secret-bearing provider details")
	}
	failed, err := Run(context.Background(), options, hooks)
	if err == nil {
		t.Fatal("failed restore check reported success")
	}
	state, err := Status(root)
	if err != nil {
		t.Fatal(err)
	}
	if state.Phase != "failed" || state.ErrorCode != "RESTORE_CHECK_FAILED" || state.LastSnapshot != second.Name || len(state.Snapshots) != 2 {
		t.Fatal(state)
	}
	statusRaw, _ := os.ReadFile(filepath.Join(root, "status.json"))
	if strings.Contains(string(statusRaw), "secret-bearing") {
		t.Fatal("provider error leaked into status")
	}
	for _, name := range []string{first.Name, second.Name, failed.Name, "manual-backup", "auto-incomplete"} {
		if _, err = os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("failure pruned %s", name)
		}
	}
	third, err := Run(context.Background(), options, fakeHooks())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, first.Name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("old successful backup not pruned")
	}
	for _, name := range []string{second.Name, third.Name, failed.Name, "manual-backup", "auto-incomplete"} {
		if _, err = os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("retention removed unowned/newer %s", name)
		}
	}
}
func TestBackupLockAndInterruptedStatus(t *testing.T) {
	root := filepath.Join(t.TempDir(), "automatic")
	entered := make(chan struct{})
	release := make(chan struct{})
	hooks := fakeHooks()
	old := hooks.Backup
	hooks.Backup = func(ctx context.Context, path string) (json.RawMessage, error) {
		close(entered)
		<-release
		return old(ctx, path)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	var runErr error
	go func() { defer wg.Done(); _, runErr = Run(context.Background(), Options{Root: root, Keep: 1}, hooks) }()
	<-entered
	if _, err := Run(context.Background(), Options{Root: root, Keep: 1}, fakeHooks()); !errors.Is(err, ErrBusy) {
		t.Fatal("overlap allowed", err)
	}
	state, err := Status(root)
	if err != nil || !state.Running || state.Phase != "backup" {
		t.Fatal(state, err)
	}
	close(release)
	wg.Wait()
	if runErr != nil {
		t.Fatal(runErr)
	}
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = atomicJSON(filepath.Join(root, "status.json"), State{Format: format, Phase: "restore_check", StartedAt: time.Now(), Running: true}); err != nil {
		t.Fatal(err)
	}
	store.Close()
	state, err = Status(root)
	if err != nil || state.Running || state.Phase != "interrupted" || state.ErrorCode != "INTERRUPTED" {
		t.Fatal(state, err)
	}
}
func TestPrivateConfigurationAndNoSymlinkPruning(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "automatic")
	source := filepath.Join(parent, "runtime.env")
	if err := os.WriteFile(source, []byte("APP_ENCRYPTION_KEY='private test value'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), Options{Root: root, Keep: 1, ConfigSnapshot: source}, fakeHooks())
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(filepath.Join(root, result.Name, "recovery.env"))
	if err != nil || !strings.Contains(string(saved), "private test value") || !result.ConfigIncluded || len(result.ConfigurationSHA256) != 64 {
		t.Fatal("configuration not preserved", err)
	}
	info, _ := os.Stat(filepath.Join(root, result.Name, "recovery.env"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("config not private")
	}
	outside := filepath.Join(parent, "outside")
	if err = os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(outside, filepath.Join(root, "auto-symlink")); err != nil {
		t.Fatal(err)
	}
	if _, err = Run(context.Background(), Options{Root: root, Keep: 1}, fakeHooks()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(outside); err != nil {
		t.Fatal("symlink target deleted")
	}
	if err = os.Chmod(source, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = Run(context.Background(), Options{Root: root, Keep: 1, ConfigSnapshot: source}, fakeHooks()); err == nil {
		t.Fatal("public configuration accepted")
	}
}
func TestRefuseUnownedRootAndInvalidVerification(t *testing.T) {
	parent := t.TempDir()
	unowned := filepath.Join(parent, "unowned")
	if err := os.Mkdir(unowned, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unowned, "other"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(unowned); err == nil {
		t.Fatal("took ownership of nonempty directory")
	}
	linked := filepath.Join(parent, "linked")
	if err := os.Symlink(unowned, linked); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(linked); err == nil {
		t.Fatal("accepted symlink root")
	}
	root := filepath.Join(parent, "new")
	hooks := fakeHooks()
	hooks.Check = func(context.Context, string) (json.RawMessage, error) {
		return json.RawMessage(`{"verified":true,"temporary_database_dropped":false}`), nil
	}
	if _, err := Run(context.Background(), Options{Root: root, Keep: 1}, hooks); err == nil {
		t.Fatal("temporary DB leak accepted as success")
	}
	state, err := Status(root)
	if err != nil || state.ErrorCode != "RESTORE_CHECK_INCOMPLETE" || len(state.Snapshots) != 0 {
		t.Fatal(state, err)
	}
	validAlias := filepath.Join(parent, "alias-of-store")
	if err := os.Symlink(root, validAlias); err != nil {
		t.Fatal(err)
	}
	if _, err := Status(validAlias); err == nil {
		t.Fatal("status followed a symlink root")
	}
	for _, bad := range []string{"../escape", "auto-../../escape", "manual", "auto-x/y"} {
		if _, err := Check(context.Background(), root, bad, fakeHooks()); err == nil {
			t.Fatal("invalid snapshot accepted", bad)
		}
	}
}
