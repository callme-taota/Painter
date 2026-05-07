package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUnknownDomain(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown/test", nil)
	w := httptest.NewRecorder()
	s.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
}

func TestProxyToIdentity(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","traceId":"x","data":{"userId":"u1"}}`))
	}))
	defer upstream.Close()
	t.Setenv("IDENTITY_BASE_URL", upstream.URL)

	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/identity/test", nil)
	w := httptest.NewRecorder()
	s.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestLegacyLoginCompatRoute(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity/auth/login" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","traceId":"x","data":{"userId":"u1"}}`))
	}))
	defer upstream.Close()
	t.Setenv("IDENTITY_BASE_URL", upstream.URL)

	s := NewServer()
	req := httptest.NewRequest(http.MethodPost, "/api/user/login/email", nil)
	w := httptest.NewRecorder()
	s.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestLegacyCompatStubRoute(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","traceId":"x","data":{"siteName":"Painter"}}`))
	}))
	defer upstream.Close()
	t.Setenv("SYSTEM_BASE_URL", upstream.URL)

	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/api/common/entry", nil)
	w := httptest.NewRecorder()
	s.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
