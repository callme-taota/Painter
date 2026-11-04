package automation

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testSession = "private-session-fixture-0123456789abcdef"
const testCSRF = "private-csrf-fixture-never-print"
const testPassword = "PRIVATE_PASSWORD_FIXTURE_NEVER_PRINT"

type captured struct {
	Method, Path, CSRF, Version, Key, Marker string
	Body                                     json.RawMessage
}
type fakeSite struct {
	mu            sync.Mutex
	server        *httptest.Server
	role          string
	authenticated bool
	version       int
	requests      []captured
	revoked       bool
}

func fixture(t *testing.T) (*fakeSite, *Client, string) {
	t.Helper()
	f := &fakeSite{role: "owner", authenticated: true, version: 1}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	client := newClient(f.server.URL, testSession)
	t.Cleanup(client.http.CloseIdleConnections)
	path := filepath.Join(t.TempDir(), "admin.json")
	if err := saveCredentials(path, credentials{Version: 1, Origin: f.server.URL, Session: testSession, AllowHTTP: true}); err != nil {
		t.Fatal(err)
	}
	return f, client, path
}
func (f *fakeSite) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(io.LimitReader(r.Body, 3<<20))
	f.requests = append(f.requests, captured{r.Method, r.URL.RequestURI(), r.Header.Get("X-CSRF-Token"), r.Header.Get("If-Match"), r.Header.Get("Idempotency-Key"), r.Header.Get("X-Painter-Admin-Tool"), body})
	w.Header().Set("Content-Type", "application/json")
	write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	if r.URL.Path == "/api/v1/auth/options" {
		http.SetCookie(w, &http.Cookie{Name: "painter_local_nonce", Value: "nonce-fixture", Path: "/"})
		write(map[string]string{"csrf_token": testCSRF})
		return
	}
	if r.URL.Path == "/api/v1/auth/login" {
		if r.Header.Get("Origin") != f.server.URL || r.Header.Get("X-CSRF-Token") != testCSRF {
			w.WriteHeader(403)
			write(map[string]any{"error": map[string]string{"code": "CSRF_FAILED"}})
			return
		}
		var p map[string]string
		_ = json.Unmarshal(body, &p)
		if p["password"] != testPassword {
			w.WriteHeader(401)
			write(map[string]any{"error": map[string]string{"code": "INVALID_CREDENTIALS", "message": p["password"]}})
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "painter_session", Value: testSession, Path: "/", HttpOnly: true})
		write(map[string]bool{"authenticated": true})
		return
	}
	if r.URL.Path == "/api/v1/session" && r.Method == "GET" {
		if !f.authenticated {
			write(map[string]bool{"authenticated": false})
			return
		}
		write(map[string]any{"authenticated": true, "csrf_token": testCSRF, "admin_tools_version": f.version, "expires_at": "2026-12-01T00:00:00Z", "user": map[string]string{"id": "1", "username": "admin", "display_name": "Owner", "role": f.role}})
		return
	}
	if r.URL.Path == "/api/v1/session" && r.Method == "DELETE" {
		f.revoked = true
		w.WriteHeader(204)
		return
	}
	if r.Header.Get("X-Painter-Admin-Tool") != "1" {
		t := map[string]any{"error": map[string]string{"code": "PERMISSION_DENIED"}}
		w.WriteHeader(403)
		write(t)
		return
	}
	if r.Method != "GET" && (r.Header.Get("Origin") != f.server.URL || r.Header.Get("X-CSRF-Token") != testCSRF) {
		w.WriteHeader(403)
		write(map[string]any{"error": map[string]string{"code": "CSRF_FAILED"}})
		return
	}
	if strings.Contains(r.URL.Path, "/studio/posts/") && r.Method == "GET" {
		write(map[string]any{"id": "7", "title": "Article", "markdown": "# Source", "status": "draft", "version": 4, "draft_version": 3, "cover": map[string]string{"mode": "generated"}, "date_display": map[string]bool{}, "tag_ids": []string{"2"}, "media_ids": []string{}})
		return
	}
	if r.Method == "GET" {
		write(map[string]any{"items": []any{}, "has_more": false, "next_cursor": ""})
		return
	}
	write(map[string]any{"id": "7", "version": 5, "draft_version": 3, "ack_mutation_id": r.Header.Get("Idempotency-Key")})
}
func noPrivate(t *testing.T, text string) {
	t.Helper()
	for _, secret := range []string{testSession, testCSRF, testPassword} {
		if strings.Contains(text, secret) {
			t.Fatalf("private value exposed in output")
		}
	}
}

func TestLoginStoresPrivateSessionAndOnlyReportsSafeIdentity(t *testing.T) {
	f, _, path := fixture(t)
	_ = os.Remove(path)
	var out, errs bytes.Buffer
	err := RunAdmin(context.Background(), []string{"login", "--url", f.server.URL, "--username", "admin", "--password-stdin", "--allow-http", "--config", path}, strings.NewReader(testPassword+"\n"), &out, &errs)
	if err != nil {
		t.Fatalf("login: %v; %s", err, errs.String())
	}
	noPrivate(t, out.String()+errs.String())
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("credential permissions")
	}
	c, e := loadCredentials(path)
	if e != nil || c.Session != testSession {
		t.Fatal("session was not persisted")
	}
}

func TestLoginRejectsNonOwnerAndRevokesAccidentalSession(t *testing.T) {
	f, _, path := fixture(t)
	_ = os.Remove(path)
	f.role = "author"
	var out, errs bytes.Buffer
	err := RunAdmin(context.Background(), []string{"login", "--url", f.server.URL, "--username", "author", "--password-stdin", "--allow-http", "--config", path}, strings.NewReader(testPassword), &out, &errs)
	if err == nil || !strings.Contains(errs.String(), "OWNER_REQUIRED") {
		t.Fatal("non-owner login accepted")
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("non-owner credential persisted")
	}
	if !f.revoked {
		t.Fatal("rejected login session not revoked")
	}
	noPrivate(t, out.String()+errs.String())
}

func TestPreflightRejectsExpiredDemotedOrOldServer(t *testing.T) {
	for _, state := range []string{"expired", "reader", "old"} {
		t.Run(state, func(t *testing.T) {
			f, c, _ := fixture(t)
			switch state {
			case "expired":
				f.authenticated = false
			case "reader":
				f.role = "reader"
			case "old":
				f.version = 0
			}
			_, err := c.execute(context.Background(), "tags_list", operationArgs{}, false)
			if err == nil {
				t.Fatal("unsafe preflight accepted")
			}
			if len(f.requests) != 1 || f.requests[0].Path != "/api/v1/session" {
				t.Fatal("business request sent after rejected preflight")
			}
		})
	}
}

func TestEachOperationRefreshesSessionAndPreservesCASAndIdempotency(t *testing.T) {
	f, c, _ := fixture(t)
	args := operationArgs{ID: "7", Version: 4, DraftVersion: 3, Key: "publish-test-001"}
	for range 2 {
		if _, err := c.execute(context.Background(), "post_publish", args, true); err != nil {
			t.Fatal(err)
		}
	}
	if len(f.requests) != 4 {
		t.Fatalf("wanted 2 preflights and 2 writes, got %d", len(f.requests))
	}
	for i, req := range f.requests {
		if req.Marker != "1" {
			t.Fatal("owner marker missing")
		}
		if i%2 == 0 {
			if req.Path != "/api/v1/session" {
				t.Fatal("fresh session missing")
			}
			continue
		}
		if req.Method != "POST" || req.Path != "/api/v1/studio/posts/7/publish" || req.Version != `"4"` || req.Key != args.Key || req.CSRF != testCSRF || string(req.Body) != `{"expected_draft_version":3}` {
			t.Fatalf("wrong publication contract: %#v", req)
		}
	}
}

func TestCreateStableClientIDAndFullSaveRetryBodies(t *testing.T) {
	f, c, _ := fixture(t)
	draft := map[string]json.RawMessage{"title": json.RawMessage(`"Hello"`), "markdown": json.RawMessage(`"# Body"`)}
	create := operationArgs{Key: "create-stable-001", Draft: draft}
	for range 2 {
		if _, e := c.execute(context.Background(), "post_create", create, true); e != nil {
			t.Fatal(e)
		}
	}
	if !bytes.Equal(f.requests[1].Body, f.requests[3].Body) || !bytes.Contains(f.requests[1].Body, []byte(`"client_draft_id":"cli-`)) {
		t.Fatal("unstable create retry")
	}
	full := map[string]json.RawMessage{"title": json.RawMessage(`"Saved"`), "summary": json.RawMessage(`"subtitle"`), "markdown": json.RawMessage(`"# Body"`), "tag_ids": json.RawMessage(`["2"]`), "media_ids": json.RawMessage(`[]`), "cover": json.RawMessage(`{"mode":"generated"}`), "date_display": json.RawMessage(`{}`)}
	save := operationArgs{ID: "7", Version: 4, DraftVersion: 3, Key: "save-stable-0001", Draft: full}
	for range 2 {
		if _, e := c.execute(context.Background(), "post_save", save, true); e != nil {
			t.Fatal(e)
		}
	}
	if !bytes.Equal(f.requests[5].Body, f.requests[7].Body) {
		t.Fatal("save retry changed its payload")
	}
	for _, r := range f.requests {
		if strings.HasSuffix(r.Path, "/7") && r.Method == "GET" {
			t.Fatal("save unexpectedly refetched mutable state")
		}
	}
}

func TestMutationValidationAndReadOnlyPolicyNeverSendHTTP(t *testing.T) {
	f, c, _ := fixture(t)
	cases := []struct {
		name  string
		args  operationArgs
		write bool
	}{
		{"post_publish", operationArgs{ID: "7", Version: 4, DraftVersion: 3, Key: "valid-key"}, false},
		{"post_publish", operationArgs{ID: "7", Key: "valid-key"}, true},
		{"post_publish", operationArgs{ID: "7", Version: 4, Key: "valid-key"}, true},
		{"post_publish", operationArgs{ID: "../admin", Version: 4, DraftVersion: 3, Key: "valid-key"}, true},
		{"post_create", operationArgs{Key: "short"}, true},
		{"post_save", operationArgs{ID: "7", Version: 4, DraftVersion: 3, Key: "valid-key", Draft: map[string]json.RawMessage{"title": json.RawMessage(`"partial"`)}}, true},
		{"post_create", operationArgs{Key: "valid-key", Draft: map[string]json.RawMessage{"role": json.RawMessage(`"owner"`)}}, true},
	}
	for _, tc := range cases {
		if _, e := c.execute(context.Background(), tc.name, tc.args, tc.write); e == nil {
			t.Fatalf("invalid mutation accepted: %s", tc.name)
		}
	}
	if len(f.requests) > 0 {
		t.Fatal("invalid mutation sent an HTTP request")
	}
}

func TestCLIFiltersOwnSiteAndStatusAndLogoutWorksAfterDemotion(t *testing.T) {
	f, _, path := fixture(t)
	var out, errs bytes.Buffer
	if err := RunAdmin(context.Background(), []string{"posts", "list", "--status", "published", "--limit", "8", "--config", path}, nil, &out, &errs); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.requests[1].Path, "scope=site") || !strings.Contains(f.requests[1].Path, "status=published") {
		t.Fatal("site/status filtering missing")
	}
	f.role = "reader"
	out.Reset()
	errs.Reset()
	if err := RunAdmin(context.Background(), []string{"logout", "--config", path}, nil, &out, &errs); err != nil {
		t.Fatalf("logout failed: %s", errs.String())
	}
	if !f.revoked {
		t.Fatal("session not revoked")
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("credential not removed")
	}
	noPrivate(t, out.String()+errs.String())
}

func TestOriginAndPrivateFileBoundary(t *testing.T) {
	for _, origin := range []string{"http://127.0.0.1:18080", "https://u:password@example.com", "https://example.com/path", "https://example.com/?secret=yes", "https://example.com/#secret", "https://example.com:0", "file:///tmp/site"} {
		if _, err := validOrigin(origin, false); err == nil {
			t.Fatalf("origin accepted: %s", origin)
		}
	}
	if _, e := validOrigin("http://192.168.2.209:18080", true); e != nil {
		t.Fatal(e)
	}
	if _, e := validOrigin("http://203.0.113.1:8080", true); e == nil {
		t.Fatal("public plaintext origin accepted")
	}
	_, _, path := fixture(t)
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(path, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e = loadCredentials(path); e == nil {
		t.Fatal("public credentials accepted")
	}
	_ = os.Chmod(path, 0600)
	link := filepath.Join(filepath.Dir(path), "link")
	if e = os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if _, e = loadCredentials(link); e == nil {
		t.Fatal("symlink credential accepted")
	}
	if e = saveCredentials(link, credentials{}); e == nil {
		t.Fatal("symlink credential overwritten")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(raw, after) {
		t.Fatal("symlink target modified")
	}
}

func TestRedirectsAndReflectedErrorsDoNotLeakCredentials(t *testing.T) {
	called := false
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(200) }))
	defer evil.Close()
	for _, mode := range []string{"redirect", "reflected"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "redirect" {
					http.Redirect(w, r, evil.URL, 307)
					return
				}
				w.WriteHeader(401)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": testPassword, "message": testSession + testCSRF}})
			}))
			defer srv.Close()
			c := newClient(srv.URL, testSession)
			defer c.http.CloseIdleConnections()
			_, err := c.session(context.Background(), true)
			if err == nil {
				t.Fatal("unexpected success")
			}
			noPrivate(t, err.Error())
		})
	}
	if called {
		t.Fatal("followed redirect with administrator credentials")
	}
}

type mcpHarness struct {
	in     *io.PipeWriter
	out    *bufio.Reader
	done   chan error
	reader *io.PipeReader
}

func startMCP(t *testing.T, c *Client, write bool) *mcpHarness {
	t.Helper()
	reader, input := io.Pipe()
	output, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := serveMCP(context.Background(), c, write, reader, writer)
		_ = writer.Close()
		done <- err
	}()
	h := &mcpHarness{input, bufio.NewReader(output), done, output}
	t.Cleanup(func() {
		_ = h.in.Close()
		_ = h.reader.Close()
		select {
		case <-h.done:
		case <-time.After(2 * time.Second):
			t.Error("MCP did not exit on EOF")
		}
	})
	return h
}
func (h *mcpHarness) send(t *testing.T, line string) {
	t.Helper()
	if _, e := io.WriteString(h.in, line+"\n"); e != nil {
		t.Fatal(e)
	}
}
func (h *mcpHarness) read(t *testing.T) map[string]any {
	t.Helper()
	ch := make(chan []byte, 1)
	go func() { line, _ := h.out.ReadBytes('\n'); ch <- line }()
	select {
	case line := <-ch:
		var response map[string]any
		if json.Unmarshal(line, &response) != nil {
			t.Fatalf("invalid MCP response: %s", line)
		}
		noPrivate(t, string(line))
		return response
	case <-time.After(2 * time.Second):
		t.Fatal("MCP response timeout")
	}
	return nil
}
func (h *mcpHarness) init(t *testing.T) {
	h.send(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	r := h.read(t)
	if r["error"] != nil {
		t.Fatal("initialize rejected")
	}
	h.send(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
}

func TestMCPReadOnlyLifecycleAndStrictArgumentSurface(t *testing.T) {
	_, c, _ := fixture(t)
	h := startMCP(t, c, false)
	h.send(t, `{"jsonrpc":"2.0","id":0,"method":"tools/list"}`)
	if h.read(t)["error"] == nil {
		t.Fatal("request before initialize accepted")
	}
	h.init(t)
	h.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	result := h.read(t)["result"].(map[string]any)
	if len(result["tools"].([]any)) != 4 {
		t.Fatal("read-only tool list includes writes")
	}
	for i, params := range []string{`{"name":"painter_post_publish","arguments":{"id":"7","version":4,"draft_version":3,"idempotency_key":"valid-key"}}`, `{"name":"painter_tags_list","arguments":{"draft":{"title":"bad"}}}`} {
		h.send(t, fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":%s}`, 3+i, params))
		if h.read(t)["error"] == nil {
			t.Fatal("hidden write or unknown argument accepted")
		}
	}
	h.send(t, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"painter_post_status","arguments":{"id":"7"}}}`)
	r := h.read(t)["result"].(map[string]any)
	if r["isError"] != false {
		t.Fatal("read-only status failed")
	}
	text := r["content"].([]any)[0].(map[string]any)["text"].(string)
	if strings.Contains(text, "markdown") {
		t.Fatal("status includes article body")
	}
}

func TestMCPWriteToolRechecksOwnerAndRequiresVersions(t *testing.T) {
	f, c, _ := fixture(t)
	h := startMCP(t, c, true)
	h.init(t)
	h.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if len(h.read(t)["result"].(map[string]any)["tools"].([]any)) != 9 {
		t.Fatal("write tools missing")
	}
	h.send(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"painter_post_publish","arguments":{"id":"7","idempotency_key":"valid-key"}}}`)
	if h.read(t)["error"] == nil {
		t.Fatal("required versions accepted as missing")
	}
	h.send(t, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"painter_post_publish","arguments":{"id":"7","version":4,"draft_version":3,"idempotency_key":"valid-key"}}}`)
	if h.read(t)["result"].(map[string]any)["isError"] != false {
		t.Fatal("publish failed")
	}
	f.mu.Lock()
	f.role = "reader"
	f.mu.Unlock()
	h.send(t, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"painter_tags_list","arguments":{}}}`)
	if h.read(t)["result"].(map[string]any)["isError"] != true {
		t.Fatal("role downgrade ignored")
	}
}

func TestMCPParseErrorsNullIDsAndUnsupportedMethods(t *testing.T) {
	_, c, _ := fixture(t)
	h := startMCP(t, c, false)
	for _, input := range []string{`not json`, `[]`, `{"jsonrpc":"2.0","id":null,"method":"ping"}`} {
		h.send(t, input)
		if h.read(t)["error"] == nil {
			t.Fatal("invalid request accepted")
		}
	}
	h.init(t)
	h.send(t, `{"jsonrpc":"2.0","id":9,"method":"resources/list"}`)
	if h.read(t)["error"] == nil {
		t.Fatal("unsupported capability accepted")
	}
}

func TestMCPCancellationInterruptsInFlightHTTPAndContextStopsReader(t *testing.T) {
	started := make(chan struct{})
	finished := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(finished)
	}))
	defer srv.Close()
	c := newClient(srv.URL, testSession)
	defer c.http.CloseIdleConnections()
	h := startMCP(t, c, false)
	h.init(t)
	h.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"painter_tags_list","arguments":{}}}`)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("HTTP request did not start")
	}
	h.send(t, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":2,"reason":"test"}}`)
	if h.read(t)["result"].(map[string]any)["isError"] != true {
		t.Fatal("cancelled request reported success")
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("HTTP request did not cancel")
	}

	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	defer writer.Close()
	done := make(chan error, 1)
	go func() { done <- serveMCP(ctx, c, false, reader, io.Discard) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("context cancellation did not stop stdio reader")
	}
}

func TestMCPAndHTTPSizeLimitsDoNotEchoInput(t *testing.T) {
	_, c, _ := fixture(t)
	var out bytes.Buffer
	if err := serveMCP(context.Background(), c, false, strings.NewReader(strings.Repeat("x", (3<<20)+1)), &out); err == nil {
		t.Fatal("oversized MCP input accepted")
	}
	if out.Len() > 512 {
		t.Fatal("MCP error echoed oversized input")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", maxResponseBytes+1))
	}))
	defer srv.Close()
	large := newClient(srv.URL, testSession)
	defer large.http.CloseIdleConnections()
	if _, err := large.request(context.Background(), "GET", "/session", "", "", "", nil); err == nil {
		t.Fatal("oversized HTTP response accepted")
	}
}

func TestCLIInvalidPasswordArgumentsNeverEchoSecretsOrSendHTTP(t *testing.T) {
	f, _, path := fixture(t)
	var out, errs bytes.Buffer
	err := RunAdmin(context.Background(), []string{"login", "--url", f.server.URL, "--username", "admin", "--password", testPassword, "--allow-http", "--config", path}, nil, &out, &errs)
	if err == nil {
		t.Fatal("command-line password accepted")
	}
	if len(f.requests) != 0 {
		t.Fatal("invalid password argument sent HTTP")
	}
	noPrivate(t, out.String()+errs.String())
}

func TestTagListPaginationCLIAndMCPContract(t *testing.T) {
	f, c, path := fixture(t)
	var out, errs bytes.Buffer
	if err := RunAdmin(context.Background(), []string{"tags", "list", "--limit", "100", "--cursor", "opaque cursor/+", "--config", path}, nil, &out, &errs); err != nil {
		t.Fatal(err)
	}
	if got := f.requests[1].Path; got != "/api/v1/admin/tags?cursor=opaque+cursor%2F%2B&limit=100" {
		t.Fatalf("tag cursor/limit lost: %s", got)
	}
	before := len(f.requests)
	if _, err := c.execute(context.Background(), "tags_list", operationArgs{Limit: 101}, false); err == nil {
		t.Fatal("oversized tag page accepted")
	}
	if _, err := c.execute(context.Background(), "tags_list", operationArgs{Cursor: strings.Repeat("x", 4097)}, false); err == nil {
		t.Fatal("oversized cursor accepted")
	}
	if len(f.requests) != before {
		t.Fatal("invalid pagination sent HTTP")
	}
	if err := RunAdmin(context.Background(), []string{"tags", "list", "--limit", "0", "--config", path}, nil, &out, &errs); err == nil {
		t.Fatal("explicit zero CLI page accepted")
	}
	h := startMCP(t, c, false)
	h.init(t)
	h.send(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"painter_tags_list","arguments":{"limit":100,"cursor":"next-page"}}}`)
	if h.read(t)["result"].(map[string]any)["isError"] != false {
		t.Fatal("paginated tag tool failed")
	}
	f.mu.Lock()
	last := f.requests[len(f.requests)-1]
	f.mu.Unlock()
	if last.Path != "/api/v1/admin/tags?cursor=next-page&limit=100" {
		t.Fatal("MCP tag pagination lost")
	}
	h.send(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"painter_tags_list","arguments":{"limit":0}}}`)
	if h.read(t)["error"] == nil {
		t.Fatal("explicit zero MCP page accepted")
	}
}
