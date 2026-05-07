package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestOverviewHandler(t *testing.T) {
	if os.Getenv("ANALYTICS_MYSQL_DSN") == "" {
		t.Skip("skip: ANALYTICS_MYSQL_DSN not set")
	}
	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/analytics/overview", nil)
	w := httptest.NewRecorder()
	s.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
