package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"painter-next/internal/app"
	"painter-next/internal/automation"
)

func main() {
	if err := run(); err != nil {
		var reported *automation.ReportedError
		if errors.As(err, &reported) {
			os.Exit(1)
		}
		slog.Error("painter stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command == "admin" || command == "mcp" {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		if command == "admin" {
			return automation.RunAdmin(ctx, os.Args[2:], os.Stdin, os.Stdout, os.Stderr)
		}
		return automation.RunMCP(ctx, os.Args[2:], os.Stdin, os.Stdout)
	}
	if command == "healthcheck" {
		addr := os.Getenv("LISTEN_ADDR")
		if addr == "" {
			addr = ":8080"
		}
		host, port, e := net.SplitHostPort(addr)
		if e != nil {
			return e
		}
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		client := http.Client{Timeout: 2 * time.Second}
		res, e := client.Get("http://" + net.JoinHostPort(host, port) + "/readyz")
		if e != nil {
			return e
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return fmt.Errorf("not ready: %d", res.StatusCode)
		}
		return nil
	}
	switch command {
	case "serve", "migrate", "setup", "setup-token", "seed-demo", "backup", "restore-check", "import-legacy", "create-local-user", "reset-local-password", "psn-connect":
	default:
		return fmt.Errorf("usage: painter [serve|migrate|seed-demo|backup <directory>|restore-check <directory>|import-legacy <file> [--dry-run]|create-local-user|reset-local-password|psn-connect]")
	}
	config, err := app.ConfigFromEnv()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	a, err := app.New(ctx, config)
	if err != nil {
		return err
	}
	defer a.Close()
	if command == "setup-token" {
		token, e := a.InstallationToken(ctx)
		if e != nil {
			return e
		}
		fmt.Fprintln(os.Stdout, token)
		return nil
	}
	if command == "create-local-user" || command == "reset-local-password" {
		return runLocalAccountCommand(ctx, a, command, os.Args[2:])
	}
	if command == "psn-connect" {
		return runPSNConnectCommand(ctx, a, os.Args[2:])
	}
	if command == "migrate" || command == "setup" {
		slog.Info("database migrations verified")
		return nil
	}
	if command == "seed-demo" {
		report, e := a.SeedDemo(ctx)
		if e == nil {
			e = json.NewEncoder(os.Stdout).Encode(report)
		}
		return e
	}
	if command == "backup" || command == "restore-check" || command == "import-legacy" {
		if len(os.Args) < 3 || strings.HasPrefix(os.Args[2], "--") {
			return fmt.Errorf("%s requires a path", command)
		}
		var report any
		switch command {
		case "backup":
			report, err = a.Backup(ctx, os.Args[2])
		case "restore-check":
			report, err = a.RestoreCheck(ctx, os.Args[2])
		case "import-legacy":
			dryRun := false
			for _, arg := range os.Args[3:] {
				if arg != "--dry-run" {
					return fmt.Errorf("unknown import option: %s", arg)
				}
				dryRun = true
			}
			report, err = a.ImportLegacy(ctx, os.Args[2], dryRun)
		}
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(report)
	}
	a.RegisterContent()
	a.RegisterSearch()
	a.RegisterMedia()
	a.RegisterInteractions()
	a.RegisterAdmin()
	a.RegisterAnalytics()
	a.RegisterWeb()
	if config.BackgroundWorkersEnabled() {
		go a.RunWorkers(ctx)
		go a.RunHomePSN(ctx)
	}
	server := &http.Server{Addr: config.ListenAddr, Handler: a.OperationsHandler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	errors := make(chan error, 1)
	go func() { errors <- server.ListenAndServe() }()
	slog.Info("painter serving", "address", config.ListenAddr, "background_workers", config.BackgroundWorkersEnabled())
	select {
	case err = <-errors:
		if err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
	return nil
}
