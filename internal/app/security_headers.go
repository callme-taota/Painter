package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
)

type scriptNonceContextKey struct{}

func requestScriptNonce(ctx context.Context) string {
	nonce, _ := ctx.Value(scriptNonceContextKey{}).(string)
	return nonce
}

// securityHeaders runs before dispatch so handlers for restricted documents
// (such as generated SVG covers) can replace the default policy. Nonces are
// generated per response and are never accepted from request headers or clients.
func (a *App) securityHeaders(w http.ResponseWriter, r *http.Request) *http.Request {
	random := make([]byte, 24)
	if _, err := rand.Read(random); err != nil {
		panic(err)
	}
	nonce := base64.RawStdEncoding.EncodeToString(random)
	policy := []string{
		"default-src 'self'",
		"script-src 'self' 'nonce-" + nonce + "'",
		"script-src-attr 'none'",
		// The editor and scroll transitions set element styles at runtime.
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' https: data: blob:",
		"media-src 'self' https: blob:",
		"font-src 'self' data:",
		"connect-src 'self'",
		"worker-src 'self' blob:",
		"frame-src https://player.bilibili.com/player.html",
		"base-uri 'none'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"form-action 'self'",
	}
	w.Header().Set("Content-Security-Policy", strings.Join(policy, "; "))
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
	return r.WithContext(context.WithValue(r.Context(), scriptNonceContextKey{}, nonce))
}
