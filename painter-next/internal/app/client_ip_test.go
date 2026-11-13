package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudflareClientIPTrustAndSyntax(t *testing.T) {
	for _, tc := range []struct {
		name, peer string
		values     []string
		enabled    bool
		want       string
	}{
		{"explicit trusted connector", "192.0.2.3:1234", []string{"203.0.113.1"}, true, "203.0.113.1"},
		{"trusted IPv6 connector", "[2001:db8::3]:1234", []string{"2001:db8:abcd::1"}, true, "2001:db8:abcd::1"},
		{"canonical IPv6", "192.0.2.3:1234", []string{"2001:db8:0:0::1"}, true, "2001:db8::1"},
		{"mapped visitor normalized", "192.0.2.3:1234", []string{"::ffff:203.0.113.1"}, true, "203.0.113.1"},
		{"mapped trusted connector", "[::ffff:192.0.2.3]:1234", []string{"203.0.113.1"}, true, "203.0.113.1"},
		{"opt in required", "192.0.2.3:1234", []string{"203.0.113.1"}, false, "192.0.2.3"},
		{"spoofed untrusted direct client", "192.0.2.4:1234", []string{"203.0.113.1"}, true, "192.0.2.4"},
		{"missing peer port", "192.0.2.3", []string{"203.0.113.1"}, true, "192.0.2.3"},
		{"missing header", "192.0.2.3:1234", nil, true, "192.0.2.3"},
		{"empty header", "192.0.2.3:1234", []string{""}, true, "192.0.2.3"},
		{"two header fields", "192.0.2.3:1234", []string{"203.0.113.1", "203.0.113.2"}, true, "192.0.2.3"},
		{"comma chain", "192.0.2.3:1234", []string{"203.0.113.1,203.0.113.2"}, true, "192.0.2.3"},
		{"IPv4 with port", "192.0.2.3:1234", []string{"203.0.113.1:443"}, true, "192.0.2.3"},
		{"IPv6 with port", "192.0.2.3:1234", []string{"[2001:db8::1]:443"}, true, "192.0.2.3"},
		{"IPv6 brackets", "192.0.2.3:1234", []string{"[2001:db8::1]"}, true, "192.0.2.3"},
		{"zone", "192.0.2.3:1234", []string{"fe80::1%eth0"}, true, "192.0.2.3"},
		{"hostname", "192.0.2.3:1234", []string{"visitor.example.test"}, true, "192.0.2.3"},
		{"leading zeros", "192.0.2.3:1234", []string{"203.0.113.001"}, true, "192.0.2.3"},
		{"CIDR", "192.0.2.3:1234", []string{"203.0.113.1/32"}, true, "192.0.2.3"},
		{"surrounding whitespace", "192.0.2.3:1234", []string{" 203.0.113.1 "}, true, "192.0.2.3"},
		{"control sequence", "192.0.2.3:1234", []string{"203.0.113.1\r\nX: y"}, true, "192.0.2.3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := App{Config: transportConfig(t)}
			a.Config.TrustCloudflareConnectingIP = tc.enabled
			r := httptest.NewRequest("POST", "http://blog.example.test/api/v1/auth/login", nil)
			r.RemoteAddr = tc.peer
			for _, v := range tc.values {
				r.Header.Add("CF-Connecting-IP", v)
			}
			r.Header.Set("X-Forwarded-For", "198.51.100.91")
			r.Header.Set("X-Real-IP", "198.51.100.92")
			r.Header.Set("True-Client-IP", "198.51.100.93")
			if got := a.ClientIP(r); got != tc.want {
				t.Fatalf("ClientIP=%q want=%q", got, tc.want)
			}
		})
	}
}
func TestCloudflareClientIPConfigRequiresTrust(t *testing.T) {
	c := transportConfig(t)
	c.TrustCloudflareConnectingIP = true
	c.TrustedProxyCIDRs = ""
	if err := c.defaults(); err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXY_CIDRS") {
		t.Fatal("trusted forwarding without a peer allowlist", err)
	}
	for _, broad := range []string{"0.0.0.0/0", "::/0"} {
		c.TrustedProxyCIDRs = broad
		if err := c.defaults(); err == nil {
			t.Fatal("Cloudflare header cannot trust every direct peer")
		}
	}
	c.TrustedProxyCIDRs = "192.0.2.3/32"
	if err := c.defaults(); err != nil {
		t.Fatal(err)
	}
	backgroundConfigEnv(t)
	t.Setenv("TRUSTED_PROXY_CIDRS", "192.0.2.3/32")
	t.Setenv("TRUST_CLOUDFLARE_CONNECTING_IP", "true")
	parsed, err := ConfigFromEnv()
	if err != nil || !parsed.TrustCloudflareConnectingIP {
		t.Fatal("valid opt in rejected", err)
	}
	t.Setenv("TRUST_CLOUDFLARE_CONNECTING_IP", "private-invalid-value")
	if _, err = ConfigFromEnv(); err == nil || !strings.Contains(err.Error(), "TRUST_CLOUDFLARE_CONNECTING_IP") || strings.Contains(err.Error(), "private-invalid-value") {
		t.Fatal("invalid boolean rejected unsafely", err)
	}
}
func TestCloudflareClientIPSeparatesTunnelRateBuckets(t *testing.T) {
	a := App{Config: transportConfig(t)}
	a.Config.TrustCloudflareConnectingIP = true
	r := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	r.RemoteAddr = "192.0.2.3:1234"
	r.Header.Set("CF-Connecting-IP", "203.0.113.1")
	for i := 0; i < 60; i++ {
		if !a.allowLocalAuth(r, "account-"+RandID(), false) {
			t.Fatal("first visitor throttled early", i)
		}
	}
	if a.allowLocalAuth(r, RandID(), false) {
		t.Fatal("missing per visitor limit")
	}
	r.Header.Set("CF-Connecting-IP", "203.0.113.2")
	if !a.allowLocalAuth(r, RandID(), false) {
		t.Fatal("second tunnel visitor shared first visitor bucket")
	}
	r.Header.Set("CF-Connecting-IP", "203.0.113.1")
	r.Header.Set("X-Forwarded-For", "198.51.100.7")
	if a.allowLocalAuth(r, RandID(), false) {
		t.Fatal("XFF bypassed verified client IP bucket")
	}
	// Core write/setup/OAuth limiters use the same verified source.
	r.URL.Path = "/api/v1/content"
	for i := 0; i < 180; i++ {
		if !a.allowRequest(r) {
			t.Fatal("core write limiter early", i)
		}
	}
	if a.allowRequest(r) {
		t.Fatal("core write limit missing")
	}
	r.Header.Set("CF-Connecting-IP", "203.0.113.2")
	if !a.allowRequest(r) {
		t.Fatal("second tunnel visitor shared write limit")
	}
	// A direct caller cannot opt itself into trusted header processing.
	b := App{Config: a.Config}
	r.RemoteAddr = "192.0.2.4:1234"
	for i := 0; i < 60; i++ {
		r.Header.Set("CF-Connecting-IP", "203.0.113."+RandID()[:2])
		if !b.allowLocalAuth(r, RandID(), false) {
			t.Fatal("direct limiter early", i)
		}
	}
	r.Header.Set("CF-Connecting-IP", "203.0.113.99")
	if b.allowLocalAuth(r, RandID(), false) {
		t.Fatal("spoofed CF header bypassed direct limit")
	}
}
