package app

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

// Forwarded scheme is accepted only from explicitly configured proxy peers.
// An HTTPS public origin alone never makes arbitrary forwarded headers trusted.
func (c *Config) configureTransportSecurity(origin *url.URL) error {
	c.publicHTTPSOrigin = nil
	c.trustedProxyPrefixes = nil
	if c.EnforcePublicHTTPS && origin.Scheme != "https" {
		return errors.New("ENFORCE_PUBLIC_HTTPS requires HTTPS PUBLIC_BASE_URL")
	}
	if origin.Scheme == "https" {
		copy := *origin
		copy.Path, copy.RawPath = "", ""
		c.publicHTTPSOrigin = &copy
	}
	if strings.TrimSpace(c.TrustedProxyCIDRs) == "" {
		if c.TrustCloudflareConnectingIP {
			return errors.New("TRUST_CLOUDFLARE_CONNECTING_IP requires explicit TRUSTED_PROXY_CIDRS")
		}
		return nil
	}
	for _, raw := range strings.Split(c.TrustedProxyCIDRs, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return errors.New("TRUSTED_PROXY_CIDRS must contain comma-separated IP CIDRs")
		}
		if c.TrustCloudflareConnectingIP && prefix.Bits() == 0 {
			return errors.New("TRUST_CLOUDFLARE_CONNECTING_IP requires a restricted TRUSTED_PROXY_CIDRS allowlist")
		}
		c.trustedProxyPrefixes = append(c.trustedProxyPrefixes, prefix.Masked())
	}
	return nil
}

func (c Config) secureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	proto := r.Header.Values("X-Forwarded-Proto")
	if len(proto) != 1 || !strings.EqualFold(strings.TrimSpace(proto[0]), "https") {
		return false
	}
	return c.trustedProxyPeer(r)
}

// Proxy trust is checked against the direct TCP peer, never X-Forwarded-For,
// CF-Connecting-IP or a forwarded host. Keep tunnel connector CIDRs narrow.
func (c Config) trustedProxyPeer(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	peer, err := netip.ParseAddr(host)
	if err != nil || peer.Zone() != "" {
		return false
	}
	peer = peer.Unmap()
	for _, prefix := range c.trustedProxyPrefixes {
		if prefix.Contains(peer) {
			return true
		}
	}
	return false
}

// redirectPublicHTTP leaves LAN IP access intact. The redirect authority comes
// only from process configuration, never Host or a forwarded host header.
func (a *App) redirectPublicHTTP(w http.ResponseWriter, r *http.Request) bool {
	origin := a.Config.publicHTTPSOrigin
	if origin == nil {
		return false
	}
	host, err := url.Parse("//" + r.Host)
	if err != nil || host.User != nil || !strings.EqualFold(strings.TrimSuffix(host.Hostname(), "."), strings.TrimSuffix(origin.Hostname(), ".")) {
		return false
	}
	if a.Config.secureRequest(r) {
		// Short initial lifetime; no preload or includeSubDomains affects other apps.
		w.Header().Set("Strict-Transport-Security", "max-age=86400")
		return false
	}
	if !a.Config.EnforcePublicHTTPS {
		return false
	}
	target := *origin
	target.Path, target.RawPath = r.URL.Path, r.URL.RawPath
	target.RawQuery, target.ForceQuery = r.URL.RawQuery, r.URL.ForceQuery
	w.Header().Set("Cache-Control", "private, no-store")
	http.Redirect(w, r, target.String(), http.StatusPermanentRedirect)
	return true
}

// Authentication uses one configured HTTPS origin. A LAN HTTP page cannot keep
// production Secure cookies; send its login form to that origin before asking
// for credentials. Public reading and all API CSRF checks remain unchanged.
func (a *App) redirectAuthenticationOrigin(w http.ResponseWriter, r *http.Request) bool {
	origin := a.Config.publicHTTPSOrigin
	if origin == nil || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return false
	}
	if strings.EqualFold(r.Host, origin.Host) && a.Config.secureRequest(r) {
		return false
	}
	target := *origin
	target.Path, target.RawPath = r.URL.Path, r.URL.RawPath
	target.RawQuery, target.ForceQuery = r.URL.RawQuery, r.URL.ForceQuery
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, target.String(), http.StatusTemporaryRedirect)
	return true
}

// ClientIP accepts Cloudflare's single client address only with explicit opt-in
// and a trusted direct connector. It deliberately ignores all forwarding chains.
// Official header semantics: https://developers.cloudflare.com/fundamentals/reference/http-headers/#cf-connecting-ip
func (a *App) ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	fallback := host
	if err == nil && peer.Zone() == "" {
		fallback = peer.Unmap().String()
	}
	if !a.Config.TrustCloudflareConnectingIP || !a.Config.trustedProxyPeer(r) {
		return fallback
	}
	values := r.Header.Values("CF-Connecting-IP")
	if len(values) != 1 || values[0] == "" || strings.TrimSpace(values[0]) != values[0] {
		return fallback
	}
	client, err := netip.ParseAddr(values[0])
	if err != nil || client.Zone() != "" {
		return fallback
	}
	return client.Unmap().String()
}
