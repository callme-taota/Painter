package identity

import "testing"

func TestRedirectPolicy(t *testing.T) {
	for _, tc := range []struct {
		kind, uri string
		ok        bool
	}{
		{"web", "https://client.example/auth/callback", true},
		{"web", "http://client.example/auth/callback", false},
		{"spa", "https://client.example/*", false},
		{"spa", "https://client.example/auth#token", false},
		{"web", "https://user:pass@client.example/auth", false},
		{"native", "http://127.0.0.1/callback", true},
		{"native", "http://[::1]/callback", true},
		{"native", "http://localhost/callback", false},
		{"native", "http://192.168.2.1/callback", false},
		{"native", "cc.taota.app://callback/login", true},
		{"native", "myapp://callback/login", false},
		{"wrong", "https://client.example/callback", false},
	} {
		t.Run(tc.kind+tc.uri, func(t *testing.T) {
			err := ValidateRedirects(tc.kind, []string{tc.uri})
			if (err == nil) != tc.ok {
				t.Fatalf("allowed=%v, want %v: %v", err == nil, tc.ok, err)
			}
		})
	}
	c := &Client{Kind: "native", Redirects: []string{"http://127.0.0.1/callback?fixed=1"}}
	for _, uri := range []string{"http://127.0.0.1:13245/callback?fixed=1", "http://127.0.0.1/callback?fixed=1"} {
		if !c.RedirectAllowed(uri) {
			t.Errorf("legitimate loopback: %s", uri)
		}
	}
	for _, uri := range []string{"http://127.0.0.2:13245/callback?fixed=1", "http://[::1]:13245/callback?fixed=1", "http://localhost:13245/callback?fixed=1", "https://127.0.0.1:13245/callback?fixed=1", "http://127.0.0.1:13245/other?fixed=1", "http://127.0.0.1:13245/callback?fixed=2", "http://127.0.0.1:13245/callback?fixed=1#token"} {
		if c.RedirectAllowed(uri) {
			t.Errorf("unsafe loopback mismatch: %s", uri)
		}
	}
	web := &Client{Kind: "web", Redirects: []string{"https://client.example/callback"}}
	if web.RedirectAllowed("https://client.example:443/callback") {
		t.Error("web redirect must compare exactly")
	}
}
