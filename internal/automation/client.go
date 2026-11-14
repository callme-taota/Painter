package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 4 << 20

type APIError struct {
	Status int    `json:"status,omitempty"`
	Code   string `json:"code"`
}

func (e *APIError) Error() string { return e.Code }

type Client struct {
	origin string
	http   *http.Client
}

func newClient(origin, session string) *Client {
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(origin)
	proxy := http.ProxyFromEnvironment
	// Plaintext is explicitly limited to trusted local/LAN origins. Do not
	// reroute those session cookies through an ambient external HTTP proxy.
	if u.Scheme == "http" {
		proxy = nil
	}
	if session != "" {
		jar.SetCookies(u, []*http.Cookie{{Name: "painter_session", Value: session, Path: "/", HttpOnly: true, Secure: u.Scheme == "https"}})
	}
	return &Client{origin: origin, http: &http.Client{
		Jar: jar, Timeout: 30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		Transport:     &http.Transport{Proxy: proxy, DialContext: (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 15 * time.Second, MaxIdleConnsPerHost: 4},
	}}
}

func (c *Client) request(ctx context.Context, method, path, csrf, version, key string, body any) (json.RawMessage, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil || len(raw) > 2<<20 {
			return nil, &APIError{Code: "INVALID_REQUEST"}
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.origin+"/api/v1"+path, reader)
	if err != nil {
		return nil, &APIError{Code: "INVALID_REQUEST"}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Painter-Admin-CLI/1")
	// Revoking one's own login is safe even after an owner was demoted. Every
	// content operation retains the server-enforced owner marker.
	if method != http.MethodDelete || path != "/session" {
		req.Header.Set("X-Painter-Admin-Tool", "1")
	}
	if method != http.MethodGet {
		req.Header.Set("Origin", c.origin)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
	}
	if version != "" {
		req.Header.Set("If-Match", `"`+version+`"`)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, &APIError{Code: "NETWORK_REQUEST_FAILED"}
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil || len(raw) > maxResponseBytes {
		return nil, &APIError{Code: "RESPONSE_TOO_LARGE_OR_UNREADABLE"}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		// Never print a server-supplied message/body: a proxy or HTML error could
		// reflect a password, Cookie or CSRF header. Only allow short code tokens.
		var failure struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &failure)
		code := failure.Error.Code
		if !safeCode(code) {
			code = "HTTP_REQUEST_FAILED"
		}
		return nil, &APIError{Status: res.StatusCode, Code: code}
	}
	if res.StatusCode == http.StatusNoContent {
		return json.RawMessage(`{}`), nil
	}
	if !json.Valid(raw) {
		return nil, &APIError{Code: "INVALID_JSON_RESPONSE"}
	}
	return raw, nil
}

func safeCode(s string) bool {
	// An allowlist also prevents an uppercase password reflected as "code"
	// from becoming diagnostic output.
	switch s {
	case "INVALID_CREDENTIALS", "RATE_LIMITED", "LOGIN_REQUIRED", "PERMISSION_DENIED", "CSRF_FAILED", "ORIGIN_REJECTED", "VALIDATION_FAILED", "INVALID_JSON", "NOT_FOUND", "VERSION_CONFLICT", "PRECONDITION_REQUIRED", "IDEMPOTENCY_REQUIRED", "IDEMPOTENCY_KEY_REUSED", "CONFLICT", "WRITE_BUSY", "INTERNAL_ERROR", "PAYLOAD_TOO_LARGE", "MEDIA_NOT_READY", "STORAGE_UNAVAILABLE":
		return true
	}
	return false
}

type session struct {
	Authenticated bool   `json:"authenticated"`
	CSRF          string `json:"csrf_token"`
	User          struct {
		ID          string `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	} `json:"user"`
	ExpiresAt         string `json:"expires_at"`
	AdminToolsVersion int    `json:"admin_tools_version"`
}

func (c *Client) session(ctx context.Context, owner bool) (session, error) {
	var s session
	raw, err := c.request(ctx, http.MethodGet, "/session", "", "", "", nil)
	if err != nil {
		return s, err
	}
	if json.Unmarshal(raw, &s) != nil || !s.Authenticated {
		return s, &APIError{Code: "LOGIN_REQUIRED"}
	}
	if owner && s.User.Role != "owner" {
		return s, &APIError{Code: "OWNER_REQUIRED"}
	}
	if owner && s.AdminToolsVersion < 1 {
		return s, &APIError{Code: "SERVER_UPGRADE_REQUIRED"}
	}
	if s.CSRF == "" {
		return s, &APIError{Code: "INVALID_SESSION_RESPONSE"}
	}
	return s, nil
}

func (s session) public() any {
	return map[string]any{"authenticated": true, "user": s.User, "expires_at": s.ExpiresAt}
}

func (c *Client) ownerRequest(ctx context.Context, method, path, version, key string, body any) (json.RawMessage, error) {
	s, err := c.session(ctx, true)
	if err != nil {
		return nil, err
	}
	return c.request(ctx, method, path, s.CSRF, version, key, body)
}

func (c *Client) login(ctx context.Context, username, password string) (session, string, error) {
	var s session
	raw, err := c.request(ctx, http.MethodGet, "/auth/options", "", "", "", nil)
	if err != nil {
		return s, "", err
	}
	var options struct {
		CSRF string `json:"csrf_token"`
	}
	if json.Unmarshal(raw, &options) != nil || options.CSRF == "" {
		return s, "", errors.New("invalid login options response")
	}
	_, err = c.request(ctx, http.MethodPost, "/auth/login", options.CSRF, "", "", map[string]string{"username": username, "password": password})
	if err != nil {
		return s, "", err
	}
	s, err = c.session(ctx, true)
	if err != nil {
		if cleanup, e := c.session(ctx, false); e == nil {
			_, _ = c.request(ctx, http.MethodDelete, "/session", cleanup.CSRF, "", "", nil)
		}
		return s, "", err
	}
	u, _ := url.Parse(c.origin)
	for _, cookie := range c.http.Jar.Cookies(u) {
		if cookie.Name == "painter_session" && cookie.Value != "" {
			return s, cookie.Value, nil
		}
	}
	return s, "", errors.New("login returned no session cookie")
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid JSON input or unknown fields")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("expected one JSON object")
	}
	return nil
}

func validKey(key string) bool {
	return len(key) >= 8 && len(key) <= 128 && !strings.ContainsAny(key, "\r\n\x00")
}
