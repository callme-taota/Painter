package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	"painter-next/internal/app"
)

// Recovery is deliberately available only to the local deployment operator.
// Password reset alone does not remove an account's second factor.
func runDisableMFACommand(ctx context.Context, a *app.App, args []string) error {
	flags := flag.NewFlagSet("disable-local-mfa", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	username := flags.String("username", "", "exact account username")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *username == "" {
		return errors.New("usage: painter disable-local-mfa --username USERNAME; requires local database operator access")
	}
	report, err := a.DisableLocalMFA(ctx, *username)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
