package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestLoginHandler(t *testing.T) {
	if os.Getenv("IDENTITY_MYSQL_DSN") == "" {
		t.Skip("skip: IDENTITY_MYSQL_DSN not set")
	}
	if os.Getenv("IDENTITY_REDIS_ADDR") == "" {
		t.Skip("skip: IDENTITY_REDIS_ADDR not set")
	}
	s := NewServer()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/identity/auth/login", bytes.NewBufferString(`{"username":"admin","password":"admin"}`))
	req.Header.Set("Content-Type", "application/json")
	s.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
