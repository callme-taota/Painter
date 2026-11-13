package app

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is trusted process configuration, never populated from HTTP parameters.
type Config struct {
	DatabaseURL          string
	ListenAddr           string
	PublicBaseURL        string
	EnforcePublicHTTPS   bool
	TrustedProxyCIDRs    string
	trustedProxyPrefixes []netip.Prefix
	publicHTTPSOrigin    *url.URL
	MediaDir             string
	Environment          string
	RunBackgroundWorkers *bool
	Demo                 bool
	EncryptionKey        []byte
	AnalyticsHMACKey     []byte
	MaxConnections       int32
	UploadLimit          int64
	GitHubClientID       string
	GitHubClientSecret   string
	GitHubCallbackURL    string
	BootstrapGitHubID    string
	SetupToken           string
	OAuthURL             string
	TokenURL             string
	ProfileURL           string
	HTTPClient           *http.Client
}

func ConfigFromEnv() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), ListenAddr: os.Getenv("LISTEN_ADDR"), PublicBaseURL: os.Getenv("PUBLIC_BASE_URL"), MediaDir: os.Getenv("MEDIA_DIR"), Environment: os.Getenv("ENVIRONMENT"), GitHubClientID: os.Getenv("GITHUB_CLIENT_ID"), GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"), GitHubCallbackURL: os.Getenv("GITHUB_CALLBACK_URL"), BootstrapGitHubID: os.Getenv("BOOTSTRAP_GITHUB_ID")}
	c.TrustedProxyCIDRs = os.Getenv("TRUSTED_PROXY_CIDRS")
	if raw, present := os.LookupEnv("ENFORCE_PUBLIC_HTTPS"); present {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return c, errors.New("ENFORCE_PUBLIC_HTTPS must be a valid boolean (true or false)")
		}
		c.EnforcePublicHTTPS = enabled
	}
	if raw, present := os.LookupEnv("RUN_BACKGROUND_WORKERS"); present {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return c, errors.New("RUN_BACKGROUND_WORKERS must be a valid boolean (true or false)")
		}
		c.RunBackgroundWorkers = &enabled
	}
	var err error
	c.SetupToken = os.Getenv("SETUP_TOKEN")
	key := os.Getenv("APP_ENCRYPTION_KEY")
	if key == "" {
		key = os.Getenv("ENCRYPTION_KEY")
	}
	c.EncryptionKey, err = decodeKey(key)
	if err != nil {
		return c, errors.New("ENCRYPTION_KEY must be base64 encoded 32 bytes")
	}
	c.AnalyticsHMACKey, err = decodeKey(os.Getenv("ANALYTICS_HMAC_KEY"))
	if err != nil {
		return c, errors.New("ANALYTICS_HMAC_KEY must be base64 encoded 32 bytes")
	}
	return c, c.defaults()
}

// BackgroundWorkersEnabled preserves the established worker behavior when the
// optional setting is absent, including callers that construct Config directly.
func (c Config) BackgroundWorkersEnabled() bool {
	return c.RunBackgroundWorkers == nil || *c.RunBackgroundWorkers
}
func decodeKey(s string) ([]byte, error) {
	b, e := base64.StdEncoding.DecodeString(s)
	if e != nil || len(b) != 32 {
		return nil, errors.New("invalid key")
	}
	return b, nil
}
func (c *Config) defaults() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if c.Environment == "" {
		c.Environment = "production"
	}
	if c.Environment != "production" && c.Environment != "development" && c.Environment != "test" {
		return errors.New("ENVIRONMENT must be production, development or test")
	}
	if c.SetupToken != "" && (len(c.SetupToken) < 32 || len(c.SetupToken) > 256 || strings.TrimSpace(c.SetupToken) != c.SetupToken || strings.ContainsAny(c.SetupToken, "\r\n\x00")) {
		return errors.New("SETUP_TOKEN must be a private random value of 32..256 bytes")
	}
	if c.ListenAddr == "" {
		c.ListenAddr = ":8080"
	}
	if c.MediaDir == "" {
		c.MediaDir = "./data/media"
	}
	if c.PublicBaseURL == "" {
		c.PublicBaseURL = "http://localhost:8080"
	}
	u, e := url.Parse(c.PublicBaseURL)
	if e != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
		return errors.New("PUBLIC_BASE_URL must be an absolute HTTP(S) origin")
	}
	c.PublicBaseURL = strings.TrimRight(c.PublicBaseURL, "/")
	if c.Environment == "production" && u.Scheme != "https" {
		return errors.New("production requires HTTPS PUBLIC_BASE_URL")
	}
	if err := c.configureTransportSecurity(u); err != nil {
		return err
	}
	if len(c.EncryptionKey) != 32 || len(c.AnalyticsHMACKey) != 32 {
		return errors.New("independent 32-byte EncryptionKey and AnalyticsHMACKey required")
	}
	if string(c.EncryptionKey) == string(c.AnalyticsHMACKey) {
		return errors.New("encryption and analytics keys must differ")
	}
	if c.MaxConnections == 0 {
		c.MaxConnections = 4
	}
	if c.MaxConnections < 2 || c.MaxConnections > 64 {
		return errors.New("MaxConnections must be 2..64")
	}
	if c.UploadLimit == 0 {
		c.UploadLimit = 10 << 20
	}
	if c.UploadLimit < 1 || c.UploadLimit > 10<<20 {
		return errors.New("UploadLimit must be 1..10485760")
	}
	if c.GitHubCallbackURL == "" {
		c.GitHubCallbackURL = c.PublicBaseURL + "/auth/github/callback"
	}
	callback, e := url.Parse(c.GitHubCallbackURL)
	if e != nil || callback.Scheme != u.Scheme || callback.Host != u.Host || callback.Path != "/auth/github/callback" || callback.RawQuery != "" {
		return errors.New("GitHub callback must equal PUBLIC_BASE_URL/auth/github/callback")
	}
	if c.OAuthURL == "" {
		c.OAuthURL = "https://github.com/login/oauth/authorize"
	}
	if c.TokenURL == "" {
		c.TokenURL = "https://github.com/login/oauth/access_token"
	}
	if c.ProfileURL == "" {
		c.ProfileURL = "https://api.github.com/user"
	}
	if c.Environment != "test" && (c.OAuthURL != "https://github.com/login/oauth/authorize" || c.TokenURL != "https://github.com/login/oauth/access_token" || c.ProfileURL != "https://api.github.com/user") {
		return errors.New("identity endpoints must be GitHub official endpoints outside tests")
	}
	if c.BootstrapGitHubID != "" {
		id, e := strconv.ParseInt(c.BootstrapGitHubID, 10, 64)
		if e != nil || id <= 0 {
			return errors.New("BOOTSTRAP_GITHUB_ID must be a positive numeric GitHub ID")
		}
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	}
	return nil
}
