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
		return nil
	}
	for _, raw := range strings.Split(c.TrustedProxyCIDRs, ",") {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return errors.New("TRUSTED_PROXY_CIDRS must contain comma-separated IP CIDRs")
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
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
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
