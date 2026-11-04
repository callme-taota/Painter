package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivatePasswordInputAndArgvRejection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("  meaningful spaces password  \n"), 0600); err != nil {
		t.Fatal(err)
	}
	value, err := readLocalPassword(false, path)
	if err != nil || value != "  meaningful spaces password  " {
		t.Fatal("password altered", err)
	}
	if err = os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = readLocalPassword(false, path); err == nil {
		t.Fatal("world-readable password file accepted")
	}
	os.Chmod(path, 0600)
	link := filepath.Join(filepath.Dir(path), "linked")
	if err = os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err = readLocalPassword(false, link); err == nil {
		t.Fatal("symlink accepted")
	}
	for _, args := range [][]string{{"--username", "account", "--password", "argv-secret-must-not-appear"}, {"--username", "account", "argv-secret-must-not-appear"}, {"--username", "account", "--role", "owner", "--password-stdin"}} {
		err = runLocalAccountCommand(context.Background(), nil, "reset-local-password", args)
		if err == nil || strings.Contains(err.Error(), "argv-secret-must-not-appear") {
			t.Fatal("unsafe argv behavior", err)
		}
	}
}
