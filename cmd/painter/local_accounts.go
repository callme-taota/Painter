package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"syscall"

	"painter-next/internal/app"
)

func readLocalPassword(stdin bool, path string) (string, error) {
	if stdin == (path != "") {
		return "", errors.New("choose exactly one of --password-stdin or --password-file")
	}
	var reader io.Reader = os.Stdin
	if !stdin {
		file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
		if err != nil {
			return "", errors.New("cannot open password file")
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return "", errors.New("password file must be regular, without symlinks, and readable only by its owner (0600 or 0400)")
		}
		reader = file
	}
	raw, err := io.ReadAll(io.LimitReader(reader, 4099))
	if err != nil {
		return "", errors.New("cannot read password input")
	}
	if len(raw) > 4098 {
		return "", errors.New("password input exceeds 4096 bytes")
	}
	// Strip one transport line ending only; preserve all actual password spaces.
	value := strings.TrimSuffix(string(raw), "\n")
	value = strings.TrimSuffix(value, "\r")
	for i := range raw {
		raw[i] = 0
	}
	return value, nil
}
func runLocalAccountCommand(ctx context.Context, a *app.App, command string, args []string) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	username := flags.String("username", "", "account username")
	display := flags.String("display-name", "", "display name (create only)")
	role := flags.String("role", "reader", "owner, author, or reader (create only)")
	stdin := flags.Bool("password-stdin", false, "read password from stdin")
	path := flags.String("password-file", "", "read password from private 0600 file")
	if err := flags.Parse(args); err != nil {
		return errors.New("invalid account command options; passwords are accepted only via --password-stdin or --password-file")
	}
	if flags.NArg() != 0 || *username == "" {
		return errors.New("account command requires --username and password input; positional passwords are forbidden")
	}
	if command == "reset-local-password" {
		invalid := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "display-name" || f.Name == "role" {
				invalid = true
			}
		})
		if invalid {
			return errors.New("reset-local-password cannot change profile or role")
		}
	}
	password, err := readLocalPassword(*stdin, *path)
	if err != nil {
		return err
	}
	var report map[string]any
	if command == "create-local-user" {
		if *display == "" {
			*display = *username
		}
		report, err = a.CreateLocalUser(ctx, *username, *display, password, *role)
	} else {
		report, err = a.ResetLocalPassword(ctx, *username, password)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
