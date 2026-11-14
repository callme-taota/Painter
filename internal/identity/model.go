// Package identity provides Painter's embedded OpenID Provider boundary. It does
// not know about HTTP cookies, passwords, or Painter content permissions.
package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

const AccessLifetime = 5 * time.Minute
const GrantLifetime = 30 * 24 * time.Hour

func Random() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func Hash(value string) string { s := sha256.Sum256([]byte(value)); return hex.EncodeToString(s[:]) }

type Client struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Redirects  []string `json:"redirect_uris"`
	DisabledAt *int64   `json:"disabled_at_ms,omitempty"`
	CreatedAt  int64    `json:"created_at_ms"`
}

func (c *Client) GetID() string                    { return c.ID }
func (c *Client) RedirectURIs() []string           { return c.Redirects }
func (c *Client) PostLogoutRedirectURIs() []string { return nil }
func (c *Client) ApplicationType() op.ApplicationType {
	if c.Kind == "native" {
		return op.ApplicationTypeNative
	}
	if c.Kind == "spa" {
		return op.ApplicationTypeUserAgent
	}
	return op.ApplicationTypeWeb
}
func (c *Client) AuthMethod() oidc.AuthMethod {
	if c.Kind == "web" {
		return oidc.AuthMethodBasic
	}
	return oidc.AuthMethodNone
}
func (c *Client) ResponseTypes() []oidc.ResponseType {
	return []oidc.ResponseType{oidc.ResponseTypeCode}
}
func (c *Client) GrantTypes() []oidc.GrantType {
	return []oidc.GrantType{oidc.GrantTypeCode, oidc.GrantTypeRefreshToken}
}
func (c *Client) LoginURL(id string) string           { return "/oidc/consent?id=" + url.QueryEscape(id) }
func (c *Client) AccessTokenType() op.AccessTokenType { return op.AccessTokenTypeBearer }
func (c *Client) IDTokenLifetime() time.Duration      { return AccessLifetime }
func (c *Client) DevMode() bool                       { return false }
func (c *Client) RestrictAdditionalIdTokenScopes() func([]string) []string {
	return func(s []string) []string { return s }
}
func (c *Client) RestrictAdditionalAccessTokenScopes() func([]string) []string {
	return func(s []string) []string { return s }
}
func (c *Client) IsScopeAllowed(s string) bool {
	return slices.Contains([]string{"openid", "profile", "offline_access"}, s)
}
func (c *Client) IDTokenUserinfoClaimsAssertion() bool { return false }
func (c *Client) ClockSkew() time.Duration             { return 0 }

// ValidateRedirects deliberately disallows wildcards, fragments and embedded
// credentials. Native HTTP callbacks require an IP literal loopback, never LAN.
func ValidateRedirects(kind string, redirects []string) error {
	if !slices.Contains([]string{"web", "spa", "native"}, kind) || len(redirects) < 1 || len(redirects) > 10 {
		return errors.New("invalid client kind or redirect count")
	}
	seen := map[string]bool{}
	for _, raw := range redirects {
		u, err := url.Parse(raw)
		if err != nil || len(raw) > 2048 || u.Fragment != "" || u.User != nil || u.Opaque != "" || strings.ContainsAny(raw, "\\\r\n\t *") || seen[raw] {
			return errors.New("invalid redirect URI")
		}
		seen[raw] = true
		if u.Scheme == "https" && u.Hostname() != "" {
			continue
		}
		if kind != "native" {
			return errors.New("web and SPA redirect URIs must use HTTPS")
		}
		ip := net.ParseIP(u.Hostname())
		if u.Scheme == "http" && ip != nil && ip.IsLoopback() {
			continue
		}
		// RFC 8252 private schemes need a reverse-domain scheme, e.g. cc.taota.app.
		if u.Scheme != "http" && u.Scheme != "https" && strings.Contains(u.Scheme, ".") && u.Host != "" {
			continue
		}
		return errors.New("native redirect requires HTTPS, IP loopback, or reverse-domain private scheme")
	}
	return nil
}
func (c *Client) RedirectAllowed(raw string) bool {
	for _, registered := range c.Redirects {
		if registered == raw {
			return true
		}
		if c.Kind != "native" {
			continue
		}
		a, e1 := url.Parse(registered)
		b, e2 := url.Parse(raw)
		if e1 != nil || e2 != nil || a.Scheme != "http" || b.Scheme != "http" || a.Hostname() != b.Hostname() || a.Path != b.Path || a.RawPath != b.RawPath || a.RawQuery != b.RawQuery || b.Fragment != "" || b.User != nil {
			continue
		}
		ip := net.ParseIP(a.Hostname())
		if ip != nil && ip.IsLoopback() {
			return true
		}
	}
	return false
}

type Request struct {
	ID        string     `json:"id"`
	ClientID  string     `json:"client_id"`
	Redirect  string     `json:"redirect"`
	State     string     `json:"state"`
	Nonce     string     `json:"nonce"`
	Scopes    []string   `json:"scopes"`
	Challenge string     `json:"challenge"`
	Subject   string     `json:"subject"`
	SessionID string     `json:"session_id"`
	AuthTime  time.Time  `json:"auth_time"`
	MFAAt     *time.Time `json:"mfa_at,omitempty"`
	AMR       []string   `json:"amr"`
	Complete  bool       `json:"complete"`
	CreatedAt time.Time  `json:"created_at"`
	MaxAge    *uint      `json:"max_age,omitempty"`
}

func (r *Request) GetID() string { return r.ID }
func (r *Request) GetACR() string {
	if slices.Contains(r.AMR, "otp") {
		return "urn:painter:acr:mfa"
	}
	return "urn:painter:acr:single-factor"
}
func (r *Request) GetAMR() []string       { return r.AMR }
func (r *Request) GetAudience() []string  { return []string{r.ClientID} }
func (r *Request) GetAuthTime() time.Time { return r.AuthTime }
func (r *Request) GetClientID() string    { return r.ClientID }
func (r *Request) GetCodeChallenge() *oidc.CodeChallenge {
	return &oidc.CodeChallenge{Challenge: r.Challenge, Method: oidc.CodeChallengeMethodS256}
}
func (r *Request) GetNonce() string                   { return r.Nonce }
func (r *Request) GetRedirectURI() string             { return r.Redirect }
func (r *Request) GetResponseType() oidc.ResponseType { return oidc.ResponseTypeCode }
func (r *Request) GetResponseMode() oidc.ResponseMode { return oidc.ResponseModeQuery }
func (r *Request) GetScopes() []string                { return r.Scopes }
func (r *Request) GetState() string                   { return r.State }
func (r *Request) GetSubject() string                 { return r.Subject }
func (r *Request) Done() bool                         { return r.Complete }
func (r *Request) SetCurrentScopes(scopes []string)   { r.Scopes = scopes }

// RefreshRequest intentionally does not implement op.AuthRequest. Otherwise the
// library would mistake a narrowed refresh scope for a fresh authorization code.
type RefreshRequest struct{ Request *Request }

func (r *RefreshRequest) GetAMR() []string                 { return r.Request.AMR }
func (r *RefreshRequest) GetAudience() []string            { return []string{r.Request.ClientID} }
func (r *RefreshRequest) GetAuthTime() time.Time           { return r.Request.AuthTime }
func (r *RefreshRequest) GetClientID() string              { return r.Request.ClientID }
func (r *RefreshRequest) GetScopes() []string              { return r.Request.Scopes }
func (r *RefreshRequest) GetSubject() string               { return r.Request.Subject }
func (r *RefreshRequest) SetCurrentScopes(scopes []string) { r.Request.Scopes = scopes }

var _ op.Client = (*Client)(nil)
var _ op.AuthRequest = (*Request)(nil)
var _ op.RefreshTokenRequest = (*Request)(nil)
var _ op.RefreshTokenRequest = (*RefreshRequest)(nil)
