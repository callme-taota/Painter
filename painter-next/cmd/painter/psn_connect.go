package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"strings"

	"painter-next/internal/app"
)

type psnConnectOptions struct {
	stdin    bool
	file     string
	onlineID string
	autoSync bool
}

func parsePSNConnectOptions(args []string) (psnConnectOptions, error) {
	var options psnConnectOptions
	flags := flag.NewFlagSet("psn-connect", flag.ContinueOnError)
	// flag errors include the rejected value; neither stderr nor the returned
	// error may echo a credential accidentally supplied on the command line.
	flags.SetOutput(io.Discard)
	flags.BoolVar(&options.stdin, "npsso-stdin", false, "read the PSN credential from stdin")
	flags.StringVar(&options.file, "npsso-file", "", "read the PSN credential from a private regular file")
	flags.StringVar(&options.onlineID, "online-id", "", "expected PSN Online ID (optional)")
	flags.BoolVar(&options.autoSync, "auto-sync", false, "enable automatic synchronization")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return psnConnectOptions{}, errors.New("invalid PSN connection options; credentials are accepted only via --npsso-stdin or --npsso-file")
	}
	sources := 0
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "npsso-stdin" || f.Name == "npsso-file" {
			sources++
		}
	})
	if sources != 1 || options.stdin == (options.file != "") {
		return psnConnectOptions{}, errors.New("choose exactly one of --npsso-stdin or --npsso-file for the PSN credential")
	}
	return options, nil
}

func readPSNConnectCredential(options psnConnectOptions) (string, error) {
	value, err := readLocalPassword(options.stdin, options.file)
	if err != nil {
		return "", errors.New("cannot read PSN credential; use stdin or a private regular file readable only by its owner (0600 or 0400)")
	}
	return value, nil
}

type psnLocalConnector interface {
	ConnectHomePSNLocal(context.Context, string, string, bool) (map[string]any, error)
}

func connectPSNWithOptions(ctx context.Context, connector psnLocalConnector, options psnConnectOptions, output io.Writer) error {
	npsso, err := readPSNConnectCredential(options)
	if err != nil {
		return err
	}
	report, err := connector.ConnectHomePSNLocal(ctx, npsso, options.onlineID, options.autoSync)
	if err != nil {
		return errors.New("PSN connection could not be completed; check the private credential, account, and connection status")
	}
	// Keep stdout a status DTO even if the provider later adds private fields.
	dto := map[string]any{}
	for _, key := range []string{"version", "connected", "online_id", "account_id", "auto_sync", "status", "last_sync_at", "last_error", "next_sync_at", "game_count"} {
		if value, ok := report[key]; ok {
			dto[key] = value
		}
	}
	raw, err := json.Marshal(dto)
	if err != nil || (npsso != "" && strings.Contains(string(raw), npsso)) {
		return errors.New("cannot prepare safe PSN connection status")
	}
	raw = append(raw, '\n')
	n, err := output.Write(raw)
	if err != nil || n != len(raw) {
		return errors.New("cannot write PSN connection status")
	}
	return nil
}

func runPSNConnectCommand(ctx context.Context, a *app.App, args []string) error {
	options, err := parsePSNConnectOptions(args)
	if err != nil {
		return err
	}
	return connectPSNWithOptions(ctx, a, options, os.Stdout)
}
