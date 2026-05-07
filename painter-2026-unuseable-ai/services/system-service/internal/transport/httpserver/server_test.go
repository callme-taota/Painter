package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestListConfigsHandler(t *testing.T) {
	if os.Getenv("SYSTEM_MYSQL_DSN") == "" {
		t.Skip("skip: SYSTEM_MYSQL_DSN not set")
	}
	if os.Getenv("SYSTEM_REDIS_ADDR") == "" {
		t.Skip("skip: SYSTEM_REDIS_ADDR not set")
	}
	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/system/configs?namespace=public", nil)
	w := httptest.NewRecorder()
	s.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
