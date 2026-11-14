package automation

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
)

const adminUsage = `Painter administrator HTTP CLI
  painter admin login --url https://blog.example --username admin --password-stdin [--config FILE]
      Local/LAN HTTP requires --allow-http. Passwords also accept --password-file PRIVATE_FILE.
  painter admin whoami | logout [--config FILE]
  painter admin posts list [--status all|draft|published|trashed --limit 20 --cursor CURSOR]
  painter admin posts get | status --id ID
  painter admin posts create --file draft.json --idempotency-key KEY
  painter admin posts save --id ID --version N --draft-version N --file draft.json --idempotency-key KEY
  painter admin posts publish --id ID --version N --draft-version N --idempotency-key KEY
  painter admin posts withdraw --id ID --version N --idempotency-key KEY
  painter admin tags list [--limit 20 --cursor CURSOR]
  painter admin tags create --name NAME [--description TEXT] --idempotency-key KEY
Every command accepts --config FILE after the subcommand. JSON draft input may use --file -.
No local database/environment is required. See docs/admin-cli.md.
`

// ReportedError prevents main from duplicating the already serialized JSON error.
type ReportedError struct{}

func (*ReportedError) Error() string { return "administrator command failed" }

func writeSafeError(out io.Writer, err error) {
	var api *APIError
	if !errors.As(err, &api) {
		api = &APIError{Code: "INVALID_COMMAND"}
	}
	_ = json.NewEncoder(out).Encode(map[string]any{"error": api})
}

func RunAdmin(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) (returnErr error) {
	defer func() {
		if returnErr != nil {
			writeSafeError(stderr, returnErr)
			returnErr = &ReportedError{}
		}
	}()
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(stdout, adminUsage)
		return err
	}
	verb := args[0]
	args = args[1:]
	if verb == "posts" || verb == "tags" {
		if len(args) == 0 {
			return errors.New("subcommand required")
		}
		if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
			_, err := io.WriteString(stdout, adminUsage)
			return err
		}
		verb += "_" + args[0]
		args = args[1:]
	}
	flags := flag.NewFlagSet("admin", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	config := flags.String("config", "", "private session configuration")
	urlFlag := flags.String("url", "", "server origin")
	username := flags.String("username", "", "local owner username")
	passwordStdin := flags.Bool("password-stdin", false, "read password to EOF")
	passwordFile := flags.String("password-file", "", "owner-only password file")
	allowHTTP := flags.Bool("allow-http", false, "trust local/LAN HTTP")
	id := flags.String("id", "", "post ID")
	status := flags.String("status", "all", "post status")
	limit := flags.Int("limit", 20, "page size")
	cursor := flags.String("cursor", "", "pagination cursor")
	version := flags.Int64("version", 0, "article version")
	draftVersion := flags.Int64("draft-version", 0, "draft version")
	key := flags.String("idempotency-key", "", "stable mutation key")
	file := flags.String("file", "", "JSON draft file or -")
	name := flags.String("name", "", "tag name")
	description := flags.String("description", "", "tag description")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, e := io.WriteString(stdout, adminUsage)
			return e
		}
		return errors.New("invalid command options")
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	allowed := map[string]bool{"config": true}
	switch verb {
	case "login":
		for _, k := range []string{"url", "username", "password-stdin", "password-file", "allow-http"} {
			allowed[k] = true
		}
	case "whoami", "logout":
	case "tags_list":
		allowed["limit"] = true
		allowed["cursor"] = true
	case "posts_list":
		for _, k := range []string{"status", "limit", "cursor"} {
			allowed[k] = true
		}
	case "posts_get", "posts_status":
		allowed["id"] = true
	case "posts_create":
		for _, k := range []string{"file", "idempotency-key"} {
			allowed[k] = true
		}
	case "posts_save":
		for _, k := range []string{"file", "idempotency-key", "id", "version", "draft-version"} {
			allowed[k] = true
		}
	case "posts_publish":
		for _, k := range []string{"idempotency-key", "id", "version", "draft-version"} {
			allowed[k] = true
		}
	case "posts_withdraw":
		for _, k := range []string{"idempotency-key", "id", "version"} {
			allowed[k] = true
		}
	case "tags_create":
		for _, k := range []string{"name", "description", "idempotency-key"} {
			allowed[k] = true
		}
	default:
		return errors.New("unknown administrator command")
	}
	invalid := false
	flags.Visit(func(f *flag.Flag) {
		if !allowed[f.Name] {
			invalid = true
		}
		if f.Name == "limit" && (*limit < 1 || *limit > 100) {
			invalid = true
		}
	})
	if invalid {
		return errors.New("option does not apply to this command")
	}
	path, err := configPath(*config)
	if err != nil {
		return err
	}
	if verb == "login" {
		if *username == "" {
			return errors.New("username is required")
		}
		origin, e := validOrigin(*urlFlag, *allowHTTP)
		if e != nil {
			return e
		}
		password, e := readPassword(stdin, *passwordStdin, *passwordFile)
		if e != nil {
			return e
		}
		previousSession := ""
		if previous, e := loadCredentials(path); e == nil && previous.Origin == origin {
			previousSession = previous.Session
		}
		// Present only a same-origin prior CLI session, allowing the server's
		// normal login rotation to retire it without leaking it to another site.
		client := newClient(origin, previousSession)
		defer client.http.CloseIdleConnections()
		s, token, e := client.login(ctx, *username, password)
		password = ""
		if e != nil {
			return e
		}
		if e = saveCredentials(path, credentials{Version: 1, Origin: origin, Session: token, AllowHTTP: *allowHTTP}); e != nil {
			_, _ = client.request(ctx, "DELETE", "/session", s.CSRF, "", "", nil)
			return e
		}
		return json.NewEncoder(stdout).Encode(s.public())
	}
	c, err := loadCredentials(path)
	if err != nil {
		return err
	}
	client := newClient(c.Origin, c.Session)
	defer client.http.CloseIdleConnections()
	if verb == "logout" {
		s, e := client.session(ctx, false)
		if e != nil {
			var a *APIError
			if !errors.As(e, &a) || a.Code != "LOGIN_REQUIRED" {
				return e
			}
		} else {
			if _, e = client.request(ctx, "DELETE", "/session", s.CSRF, "", "", nil); e != nil {
				return e
			}
		}
		if e = os.Remove(path); e != nil {
			return errors.New("remote session revoked but local configuration could not be removed")
		}
		return json.NewEncoder(stdout).Encode(map[string]bool{"logged_out": true})
	}
	if verb == "whoami" {
		s, e := client.session(ctx, true)
		if e != nil {
			return e
		}
		return json.NewEncoder(stdout).Encode(s.public())
	}
	operation := strings.TrimPrefix(verb, "posts_")
	if strings.HasPrefix(verb, "posts_") {
		if operation == "list" {
			operation = "posts_list"
		} else {
			operation = "post_" + operation
		}
	} else if verb == "tags_list" {
		operation = "tags_list"
	} else if verb == "tags_create" {
		operation = "tag_create"
	}
	input := operationArgs{ID: *id, Status: *status, Limit: *limit, Cursor: *cursor, Version: *version, DraftVersion: *draftVersion, Key: *key, Name: *name, Description: *description}
	if verb == "posts_create" || verb == "posts_save" {
		if *file == "" {
			return errors.New("file is required")
		}
		var reader io.Reader = stdin
		if *file != "-" {
			f, e := os.OpenFile(*file, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
			if e != nil {
				return errors.New("cannot open draft file")
			}
			defer f.Close()
			info, e := f.Stat()
			if e != nil || !info.Mode().IsRegular() {
				return errors.New("draft input must be a regular file")
			}
			reader = f
		}
		raw, e := io.ReadAll(io.LimitReader(reader, (2<<20)+1))
		if e != nil || len(raw) > 2<<20 {
			return errors.New("draft input exceeds size limit or cannot be read")
		}
		if e = decodeStrict(raw, &input.Draft); e != nil {
			return e
		}
	}
	result, err := client.execute(ctx, operation, input, true)
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(result)
}

func errMessage(err error) string {
	var api *APIError
	if errors.As(err, &api) {
		return api.Code
	}
	// Only internal fixed validation messages reach here; never include user
	// arguments, file paths, remote response bodies or network error strings.
	return fmt.Sprint("INVALID_ARGUMENTS")
}
