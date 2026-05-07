package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestCreateArticleHandler(t *testing.T) {
	if os.Getenv("CONTENT_MYSQL_DSN") == "" {
		t.Skip("skip: CONTENT_MYSQL_DSN not set")
	}
	if os.Getenv("CONTENT_REDIS_ADDR") == "" {
		t.Skip("skip: CONTENT_REDIS_ADDR not set")
	}
	s := NewServer()
	body := `{"title":"x","content":"y","categoryId":"z"}`
	req := httptest.NewRequest(http.MethodPost, "/content/articles", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
