package app

import (
	"bytes"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func transportConfig(t *testing.T) Config {
	t.Helper()
	c := Config{DatabaseURL: "host=/nonexistent dbname=transport_test", Environment: "test", PublicBaseURL: "https://blog.example.test", EnforcePublicHTTPS: true, TrustedProxyCIDRs: "192.0.2.3/32,2001:db8::3/128", EncryptionKey: bytes.Repeat([]byte{1}, 32), AnalyticsHMACKey: bytes.Repeat([]byte{2}, 32)}
	if err := c.defaults(); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPublicHTTPTransportBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, host, peer string
		proto            []string
		tls, disabled    bool
		redirect, hsts   bool
	}{
		{name: "public HTTP", host: "blog.example.test", peer: "192.0.2.3:3210", proto: []string{"http"}, redirect: true},
		{name: "trusted HTTPS proxy", host: "blog.example.test", peer: "192.0.2.3:3210", proto: []string{"https"}, hsts: true},
		{name: "IPv6 trusted HTTPS proxy", host: "blog.example.test", peer: "[2001:db8::3]:3210", proto: []string{"https"}, hsts: true},
		{name: "IPv4-mapped trusted peer", host: "blog.example.test", peer: "[::ffff:192.0.2.3]:3210", proto: []string{"https"}, hsts: true},
		{name: "spoofed scheme untrusted peer", host: "blog.example.test", peer: "192.0.2.4:3210", proto: []string{"https"}, redirect: true},
		{name: "multiple header values", host: "blog.example.test", peer: "192.0.2.3:3210", proto: []string{"https", "https"}, redirect: true},
		{name: "ambiguous comma scheme", host: "blog.example.test", peer: "192.0.2.3:3210", proto: []string{"https,http"}, redirect: true},
		{name: "invalid peer", host: "blog.example.test", peer: "192.0.2.3", proto: []string{"https"}, redirect: true},
		{name: "direct TLS", host: "blog.example.test", peer: "192.0.2.4:3210", tls: true, hsts: true},
		{name: "public case and HTTP port", host: "BLOG.EXAMPLE.TEST:80", peer: "192.0.2.4:3210", redirect: true},
		{name: "public trailing dot", host: "blog.example.test.", peer: "192.0.2.4:3210", redirect: true},
		{name: "LAN access retained", host: "192.168.2.209:18080", peer: "192.0.2.3:3210"},
		{name: "other application", host: "other.example.test", peer: "192.0.2.3:3210", tls: true},
		{name: "disabled legacy behavior", host: "blog.example.test", peer: "192.0.2.3:3210", disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := transportConfig(t)
			c.EnforcePublicHTTPS = !tc.disabled
			a := App{Config: c}
			r := httptest.NewRequest("GET", "http://"+tc.host+"/posts/a%2Fb?q=x%26y&tag=%E4%B8%AD%E6%96%87", nil)
			r.RemoteAddr = tc.peer
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			for _, p := range tc.proto {
				r.Header.Add("X-Forwarded-Proto", p)
			}
			r.Header.Set("X-Forwarded-Host", "evil.example")
			w := httptest.NewRecorder()
			if got := a.redirectPublicHTTP(w, r); got != tc.redirect {
				t.Fatalf("redirect = %v, want %v", got, tc.redirect)
			}
			if tc.redirect {
				if w.Code != 308 || w.Header().Get("Location") != "https://blog.example.test/posts/a%2Fb?q=x%26y&tag=%E4%B8%AD%E6%96%87" || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
					t.Fatalf("unsafe or lossy redirect: status %d headers %v", w.Code, w.Header())
				}
			} else if w.Header().Get("Location") != "" {
				t.Fatal("unexpected redirect location")
			}
			if got := w.Header().Get("Strict-Transport-Security"); (got == "max-age=86400") != tc.hsts {
				t.Fatalf("unexpected HSTS: %q", got)
			}
		})
	}
}

func TestPublicHTTPSConfigValidation(t *testing.T) {
	for _, value := range []string{"192.0.2.3", "0.0.0.0/33", "192.0.2.3/32,", "secret-invalid-config"} {
		c := transportConfig(t)
		c.TrustedProxyCIDRs = value
		if err := c.defaults(); err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") || strings.Contains(err.Error(), value) {
			t.Fatalf("invalid proxy configuration must reject without echo: %v", err)
		}
	}
	c := transportConfig(t)
	c.PublicBaseURL, c.GitHubCallbackURL = "http://blog.example.test", ""
	if err := c.defaults(); err == nil || !strings.Contains(err.Error(), "ENFORCE_PUBLIC_HTTPS") {
		t.Fatalf("enforcement on HTTP origin accepted: %v", err)
	}
	c = transportConfig(t)
	c.TrustedProxyCIDRs = ""
	if err := c.defaults(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "http://blog.example.test/", nil)
	r.RemoteAddr = "192.0.2.3:3210"
	r.Header.Set("X-Forwarded-Proto", "https")
	if c.secureRequest(r) {
		t.Fatal("empty proxy list must trust no forwarded scheme")
	}
}

func TestPublicHTTPSConfigFromEnv(t *testing.T) {
	backgroundConfigEnv(t)
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.3/32")
	t.Setenv("ENFORCE_PUBLIC_HTTPS", "true")
	c, err := ConfigFromEnv()
	if err != nil || !c.EnforcePublicHTTPS || len(c.trustedProxyPrefixes) != 1 {
		t.Fatalf("valid proxy environment rejected: %v", err)
	}
	t.Setenv("ENFORCE_PUBLIC_HTTPS", "invalid-private-value")
	_, err = ConfigFromEnv()
	if err == nil || !strings.Contains(err.Error(), "ENFORCE_PUBLIC_HTTPS") || strings.Contains(err.Error(), "invalid-private-value") {
		t.Fatalf("invalid boolean accepted or echoed: %v", err)
	}
}

func TestPublicHTTPSHandlerDispatch(t *testing.T) {
	a := &App{Config: transportConfig(t), Mux: http.NewServeMux()}
	called := false
	a.Mux.HandleFunc("GET /probe", func(w http.ResponseWriter, r *http.Request) {
		called = true
		if requestScriptNonce(r.Context()) == "" || RequestID(r.Context()) == "" {
			t.Error("dispatch lost security context")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r := httptest.NewRequest(http.MethodGet, "http://blog.example.test/probe", nil)
	r.RemoteAddr = "192.0.2.3:1234"
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if called || w.Code != http.StatusPermanentRedirect || w.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("HTTP reached the handler or lost security headers")
	}
	r = httptest.NewRequest(http.MethodGet, "http://blog.example.test/probe", nil)
	r.RemoteAddr = "192.0.2.3:1234"
	r.Header.Set("X-Forwarded-Proto", "https")
	w = httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if !called || w.Code != http.StatusNoContent || w.Header().Get("Strict-Transport-Security") != "max-age=86400" {
		t.Fatal("trusted HTTPS request did not reach the handler")
	}
}
