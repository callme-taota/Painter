package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type psnConnectorFunc func(context.Context, string, string, bool) (map[string]any, error)

func (fn psnConnectorFunc) ConnectHomePSNLocal(ctx context.Context, credential, onlineID string, auto bool) (map[string]any, error) {
	return fn(ctx, credential, onlineID, auto)
}

func psnPrivateCredential(t *testing.T, content string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "psn-credential")
	if err := os.WriteFile(name, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestPSNConnectOptionsRejectSecretArgumentsWithoutEcho(t *testing.T) {
	secret := "argv-secret-must-not-appear"
	diagnostics, err := os.CreateTemp(t.TempDir(), "psn-flag-stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer diagnostics.Close()
	previous := os.Stderr
	os.Stderr = diagnostics
	defer func() { os.Stderr = previous }()
	for _, args := range [][]string{
		{"--npsso", secret}, {"--npsso=" + secret}, {secret},
		{"--npsso-stdin", secret}, {"--npsso-stdin=" + secret},
		{"--npsso-stdin", "--auto-sync=" + secret}, {"--" + secret},
		{}, {"--npsso-stdin", "--npsso-file", secret},
		{"--npsso-stdin=false"}, {"--npsso-file", ""},
		{"--npsso-stdin=false", "--npsso-file", secret},
	} {
		err := runPSNConnectCommand(context.Background(), nil, args)
		if err == nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "PSN") {
			t.Fatal("unsafe PSN argument rejection", err)
		}
	}
	options, err := parsePSNConnectOptions([]string{"--npsso-stdin"})
	if err != nil || !options.stdin || options.file != "" || options.onlineID != "" || options.autoSync {
		t.Fatal("default PSN connection unexpectedly enabled sync or required an account hint", options, err)
	}
	options, err = parsePSNConnectOptions([]string{"--npsso-file", "/private/credential", "--online-id", "Fixture_Player", "--auto-sync"})
	if err != nil || options.stdin || options.file != "/private/credential" || options.onlineID != "Fixture_Player" || !options.autoSync {
		t.Fatal("valid PSN flags lost their values", options, err)
	}
	if _, err := diagnostics.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(diagnostics)
	if err != nil || len(raw) != 0 {
		t.Fatal("flag parsing wrote unchecked diagnostics to stderr", err)
	}
}

func TestPSNConnectPrivateFileRestrictionsAndSafeReadErrors(t *testing.T) {
	secret := strings.Repeat("X", 64)
	name := psnPrivateCredential(t, secret+"\r\n")
	for _, mode := range []os.FileMode{0600, 0400} {
		if err := os.Chmod(name, mode); err != nil {
			t.Fatal(err)
		}
		got, err := readPSNConnectCredential(psnConnectOptions{file: name})
		if err != nil || got != secret {
			t.Fatal("private credential file or transport newline was mishandled", err)
		}
	}
	for _, mode := range []os.FileMode{0640, 0644} {
		if err := os.Chmod(name, mode); err != nil {
			t.Fatal(err)
		}
		if _, err := readPSNConnectCredential(psnConnectOptions{file: name}); err == nil || strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), secret) {
			t.Fatal("shared-readable PSN file or unsafe read error accepted", err)
		}
	}
	if err := os.Chmod(name, 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(name), "linked-credential")
	if err := os.Symlink(name, link); err != nil {
		t.Fatal(err)
	}
	oversize := psnPrivateCredential(t, strings.Repeat("Z", 4100))
	missing := filepath.Join(t.TempDir(), secret)
	for _, unsafe := range []string{link, filepath.Dir(name), oversize, missing} {
		if value, err := readPSNConnectCredential(psnConnectOptions{file: unsafe}); err == nil || value != "" || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), unsafe) || strings.Contains(err.Error(), "password") {
			t.Fatal("non-private/nonregular/oversized PSN input was accepted or leaked its path", err)
		}
	}
}

func TestPSNConnectStdinKeepsCredentialBytesAndStripsOneLineEnding(t *testing.T) {
	input, err := os.CreateTemp(t.TempDir(), "psn-stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	secret := "  " + strings.Repeat("X", 64) + "  "
	if _, err = input.WriteString(secret + "\r\n"); err != nil {
		t.Fatal(err)
	}
	if _, err = input.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	previous := os.Stdin
	os.Stdin = input
	defer func() { os.Stdin = previous }()
	value, err := readPSNConnectCredential(psnConnectOptions{stdin: true})
	if err != nil || value != secret {
		t.Fatal("stdin silently trimmed credential bytes beyond the transport newline", err)
	}
}

func TestPSNConnectForwardsOptionsAndPrintsOnlyStatusDTO(t *testing.T) {
	secret := strings.Repeat("X", 64)
	name := psnPrivateCredential(t, secret+"\n")
	options, err := parsePSNConnectOptions([]string{"--npsso-file", name, "--online-id", "Fixture_Player", "--auto-sync"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	connector := psnConnectorFunc(func(got context.Context, credential, online string, auto bool) (map[string]any, error) {
		calls++
		if got != ctx || credential != secret || online != "Fixture_Player" || !auto {
			t.Fatal("local PSN connector received altered input/options")
		}
		return map[string]any{
			"version": int64(2), "connected": true, "online_id": "Fixture_Player", "account_id": "1234",
			"auto_sync": true, "status": "queued", "last_sync_at": nil, "last_error": "", "next_sync_at": nil, "game_count": 0,
			"npsso": secret, "access_token": "access-must-not-appear", "refresh_token": "refresh-must-not-appear", "token_ciphertext": []byte("cipher-must-not-appear"),
		}, nil
	})
	var output bytes.Buffer
	if err := connectPSNWithOptions(ctx, connector, options, &output); err != nil {
		t.Fatal(err)
	}
	var dto map[string]any
	if err := json.Unmarshal(output.Bytes(), &dto); err != nil {
		t.Fatal("stdout is not one JSON status object", err)
	}
	if calls != 1 || len(dto) != 10 || dto["connected"] != true || dto["auto_sync"] != true || dto["status"] != "queued" || !strings.HasSuffix(output.String(), "\n") {
		t.Fatal("PSN status output or connector call contract changed")
	}
	for _, sensitive := range []string{secret, "npsso", "access_token", "refresh_token", "token_ciphertext", "must-not-appear"} {
		if strings.Contains(output.String(), sensitive) {
			t.Fatal("private fields leaked to stdout")
		}
	}
}

func TestPSNConnectFailuresDoNotEchoCredentialsOrProducePartialJSON(t *testing.T) {
	secret := strings.Repeat("X", 64)
	options := psnConnectOptions{file: psnPrivateCredential(t, secret)}
	for _, connector := range []psnConnectorFunc{
		func(context.Context, string, string, bool) (map[string]any, error) {
			return nil, errors.New("upstream accidentally includes " + secret)
		},
		func(context.Context, string, string, bool) (map[string]any, error) {
			return map[string]any{"connected": true, "last_error": secret}, nil
		},
	} {
		var output bytes.Buffer
		err := connectPSNWithOptions(context.Background(), connector, options, &output)
		if err == nil || strings.Contains(err.Error(), secret) || output.Len() != 0 {
			t.Fatal("failed PSN connect leaked credentials or wrote a partial DTO", err)
		}
	}
}
