package app

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func uploadTimeoutBody(t *testing.T) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "slow-cover.png")
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(part, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func TestMediaSlowUploadBeyondServerDeadline(t *testing.T) {
	a := testApp(t)
	a.RegisterMedia()
	_, cookie, csrf := testIdentity(t, a, "owner")
	a.Mux.HandleFunc("POST /timeout-control", func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			Error(w, E(408, "TIMEOUT", "timed out"))
			return
		}
		JSON(w, 201, map[string]any{"ok": true})
	})
	server := httptest.NewUnstartedServer(a.OperationsHandler())
	server.Config.ReadTimeout = 200 * time.Millisecond
	server.Config.WriteTimeout = 200 * time.Millisecond
	server.Start()
	defer server.Close()
	body, contentType := uploadTimeoutBody(t)
	for _, path := range []string{"/api/v1/media", "/timeout-control"} {
		t.Run(path, func(t *testing.T) {
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			go func() {
				defer writer.Close()
				if _, err := writer.Write(body[:len(body)/2]); err != nil {
					return
				}
				time.Sleep(350 * time.Millisecond)
				_, _ = writer.Write(body[len(body)/2:])
			}()
			request, err := http.NewRequest("POST", server.URL+path, reader)
			if err != nil {
				t.Fatal(err)
			}
			request.ContentLength = int64(len(body))
			request.Header.Set("Content-Type", contentType)
			request.Header.Set("Origin", a.Config.PublicBaseURL)
			request.Header.Set("X-CSRF-Token", csrf)
			request.Header.Set("Idempotency-Key", RandID())
			request.AddCookie(cookie)
			client := &http.Client{Timeout: 3 * time.Second}
			response, err := client.Do(request)
			if path == "/timeout-control" {
				if err == nil {
					defer response.Body.Close()
					if response.StatusCode == 201 {
						t.Fatal("ordinary request lost the server deadline")
					}
				}
				return
			}
			if err != nil {
				t.Fatal("slow image upload was cut off by the ordinary request deadline:", err)
			}
			defer response.Body.Close()
			if response.StatusCode != 201 {
				data, _ := io.ReadAll(response.Body)
				t.Fatalf("slow upload failed: %d %s", response.StatusCode, data)
			}
		})
	}
}

type uploadReadFailure struct{ err error }

func (r uploadReadFailure) Read([]byte) (int, error) { return 0, r.err }

func TestMediaUploadTimeoutResponseAndCleanup(t *testing.T) {
	a := testApp(t)
	a.RegisterMedia()
	_, cookie, csrf := testIdentity(t, a, "owner")
	body, contentType := uploadTimeoutBody(t)
	for _, prefix := range []int{0, bytes.Index(body, []byte("\r\n\r\n")) + 4 + 10} {
		reader := io.MultiReader(bytes.NewReader(body[:prefix]), uploadReadFailure{os.ErrDeadlineExceeded})
		request := httptest.NewRequest("POST", "/api/v1/media", reader)
		request.Header.Set("Content-Type", contentType)
		request.Header.Set("Origin", a.Config.PublicBaseURL)
		request.Header.Set("X-CSRF-Token", csrf)
		request.Header.Set("Idempotency-Key", RandID())
		request.AddCookie(cookie)
		response := httptest.NewRecorder()
		a.Handler().ServeHTTP(response, request)
		if response.Code != 408 || !bytes.Contains(response.Body.Bytes(), []byte(`"code":"UPLOAD_TIMEOUT"`)) {
			t.Fatalf("timeout must be actionable, not a storage failure: %d %s", response.Code, response.Body.String())
		}
		entries, err := os.ReadDir(a.mediaPath(""))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatal("partial image survived upload timeout")
		}
	}
	var count int
	if err := a.Pool.QueryRow(context.Background(), "SELECT count(*) FROM media_assets").Scan(&count); err != nil || count != 0 {
		t.Fatal("timeout created media metadata", count, err)
	}
}
