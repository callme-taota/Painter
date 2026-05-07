package proxy

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 8 * time.Second}}
}

func (c *Client) Forward(method, url, traceID string, headers http.Header, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = headers.Clone()
	req.Header.Set("X-Trace-ID", traceID)
	return c.http.Do(req)
}

func ReadBody(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
