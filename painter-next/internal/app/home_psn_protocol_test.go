package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Real TLS fixture transport; production endpoint validation remains enabled.
// No test ever contacts Sony or uses a real account credential.
func psnFixture(t *testing.T, a *App, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	target, _ := url.Parse(server.URL)
	transport := server.Client().Transport
	a.Config.Environment = "test"
	a.Config.HTTPClient = &http.Client{Transport: homeRoundTripper(func(request *http.Request) (*http.Response, error) {
		copy := request.Clone(request.Context())
		copy.URL.Scheme, copy.URL.Host = target.Scheme, target.Host
		copy.Host = request.URL.Host
		return transport.RoundTrip(copy)
	})}
}

func psnGoodTokens() psnTokens {
	return psnTokens{Type: "psn_tokens_v1", Access: "fixture-access", Refresh: "fixture-refresh", AccessExpires: NowMillis() + time.Hour.Milliseconds(), RefreshExpires: NowMillis() + 30*24*time.Hour.Milliseconds()}
}

func TestPSNProtocolExchangeIdentityAndNoRedirectFollowing(t *testing.T) {
	a := &App{}
	var calls atomic.Int32
	psnFixture(t, a, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Path {
		case "/api/authz/v3/oauth/authorize":
			if r.Host != "ca.account.sony.com" || r.Method != "GET" || r.Header.Get("Cookie") != "npsso="+strings.Repeat("X", 64) {
				t.Error("authorization request protocol mismatch")
			}
			want := url.Values{"access_type": {"offline"}, "client_id": {psnClientID}, "redirect_uri": {psnRedirect}, "response_type": {"code"}, "scope": {psnScope}}
			if !reflect.DeepEqual(r.URL.Query(), want) {
				t.Error("authorization query mismatch")
			}
			w.Header().Set("Location", psnRedirect+"?code=fixture-code")
			w.WriteHeader(302)
		case "/api/authz/v3/oauth/token":
			if r.Method != "POST" || r.Header.Get("Authorization") != psnClientAuthorization || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.ParseForm() != nil || r.PostForm.Get("code") != "fixture-code" || r.PostForm.Get("grant_type") != "authorization_code" || r.PostForm.Get("redirect_uri") != psnRedirect || r.PostForm.Get("token_format") != "jwt" {
				t.Error("token request protocol mismatch")
			}
			fmt.Fprint(w, `{"access_token":"fixture-access","refresh_token":"fixture-refresh","token_type":"Bearer","expires_in":3600,"refresh_token_expires_in":2592000}`)
		case "/api/v1/devices/accounts/me":
			if r.Host != "dms.api.playstation.com" || r.Header.Get("Authorization") != "Bearer fixture-access" {
				t.Error("identity authentication mismatch")
			}
			fmt.Fprint(w, `{"accountId":"12345678901234567890","devices":[{"ignored":"private"}]}`)
		case "/api/userProfile/v1/internal/users/12345678901234567890/profiles":
			fmt.Fprint(w, `{"onlineId":"Fixture_Player","ignored":"private"}`)
		default:
			t.Error("unexpected endpoint or redirect followed", r.URL.Path)
			w.WriteHeader(500)
		}
	})
	client := a.psnClient(nil)
	start := NowMillis()
	tokens, err := client.exchange(context.Background(), strings.Repeat("X", 64))
	if err != nil || tokens.Type != "psn_tokens_v1" || tokens.AccessExpires < start+3599000 || tokens.RefreshExpires < start+2591999000 {
		t.Fatal("exchange or absolute expiries", err)
	}
	account, online, err := client.identity(context.Background(), tokens)
	if err != nil || account != "12345678901234567890" || online != "Fixture_Player" || calls.Load() != 4 {
		t.Fatal("identity or redirect handling", account, online, calls.Load(), err)
	}
}

func TestPSNProtocolRejectsRedirectAndRedactsProviderErrors(t *testing.T) {
	a := &App{}
	for _, location := range []string{"https://evil.example/?code=secret", psnRedirect + "?code=a&code=b", psnRedirect + "?error=secret", psnRedirect + "?code=a&error_code=bad", psnRedirect + "/bad?code=a"} {
		psnFixture(t, a, func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Location", location); w.WriteHeader(302) })
		if _, err := a.psnClient(nil).exchange(context.Background(), strings.Repeat("X", 64)); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("unsafe authorization redirect accepted or error leaked", err)
		}
	}
	for _, tc := range []struct {
		status int
		code   string
	}{{400, "PSN_UPSTREAM_UNAVAILABLE"}, {401, "PSN_UNAUTHORIZED"}, {403, "PSN_PRIVACY_OR_FORBIDDEN"}, {404, "PSN_ACCOUNT_UNAVAILABLE"}, {429, "PSN_RATE_LIMITED"}, {500, "PSN_UPSTREAM_UNAVAILABLE"}, {200, "PSN_UPSTREAM_UNAVAILABLE"}} {
		psnFixture(t, a, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(tc.status)
			fmt.Fprint(w, `{"error":{"message":"fixture-credential-must-stay-private"}}`)
		})
		_, _, err := a.psnClient(nil).request(context.Background(), "GET", "https://m.np.playstation.com/api/gamelist/v2/users/123/titles", "", http.Header{})
		var failure *psnFailure
		if !errors.As(err, &failure) || failure.Code != tc.code || strings.Contains(err.Error(), "fixture-credential") {
			t.Fatal("unsafe error mapping", tc.status, err)
		}
		if tc.status == 429 && failure.RetryAt < NowMillis()+3599000 {
			t.Fatal("Retry-After not honored")
		}
	}
}

func TestPSNProtocolRefreshPersistedBeforeFailedPagination(t *testing.T) {
	a := &App{}
	tokens := psnGoodTokens()
	tokens.AccessExpires = NowMillis()
	previousRefreshExpiry := tokens.RefreshExpires
	persisted := false
	psnFixture(t, a, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			if r.ParseForm() != nil || r.PostForm.Get("grant_type") != "refresh_token" || r.PostForm.Get("scope") != psnScope || r.PostForm.Get("refresh_token") != "fixture-refresh" {
				t.Error("refresh form mismatch")
			}
			fmt.Fprint(w, `{"access_token":"new-access","token_type":"Bearer","expires_in":3600}`)
			return
		}
		if !persisted || r.Header.Get("Authorization") != "Bearer new-access" {
			t.Error("games were requested before rotated tokens were committed")
		}
		w.WriteHeader(503)
	})
	items, err := a.psnClient(nil).games(context.Background(), "123", &tokens, func(fresh psnTokens) error {
		persisted = true
		if fresh.Refresh != "fixture-refresh" || fresh.RefreshExpires != previousRefreshExpiry {
			t.Error("omitted refresh token or absolute expiry was not preserved")
		}
		return nil
	})
	if err == nil || items != nil || !persisted || tokens.Access != "new-access" {
		t.Fatal("refresh persistence/page failure contract", persisted, err)
	}
}

func TestPSNProtocolCompletePaginationAndRealMedia(t *testing.T) {
	a := &App{}
	var pages atomic.Int32
	psnFixture(t, a, func(w http.ResponseWriter, r *http.Request) {
		pages.Add(1)
		if r.URL.Query().Get("limit") != "200" || r.URL.Query().Get("categories") != "ps4_game,ps5_native_game" {
			t.Error("games query mismatch")
		}
		if r.URL.Query().Get("offset") == "0" {
			fmt.Fprint(w, `{"titles":[{"titleId":"CUSA01433_00","name":"First","category":"ps4_game","imageUrl":"https://image.api.playstation.com/icon.png","concept":{"id":100001,"media":{"images":[{"type":"PORTRAIT_BANNER","url":"https://image.api.playstation.com/portrait.png"},{"type":"SIXTEEN_BY_NINE_BANNER","url":"https://image.api.playstation.com/hero.png"},{"type":"LOGO","url":"https://evil.example/logo.png"}]}}}],"totalItemCount":2,"nextOffset":1}`)
		} else {
			fmt.Fprint(w, `{"titles":[{"titleId":"PPSA00002_00","name":"Second","localizedName":"第二个","category":"ps5_native_game","playDuration":"PT0S","imageUrl":"https://evil.example/icon.png"}],"totalItemCount":2}`)
		}
	})
	tokens := psnGoodTokens()
	items, err := a.psnClient(nil).games(context.Background(), "123", &tokens, func(psnTokens) error { t.Error("unexpected refresh"); return nil })
	if err != nil || len(items) != 2 || pages.Load() != 2 {
		t.Fatal("short first page treated as complete", len(items), pages.Load(), err)
	}
	if items[0].ProviderID != "CUSA01433_00" || items[0].Platforms[0] != "PS4" || items[0].ImageURL != "https://image.api.playstation.com/portrait.png" || items[0].HeroURL != "https://image.api.playstation.com/hero.png" || items[0].LogoURL != "" || items[0].URL != "https://store.playstation.com/en-us/concept/100001" || items[0].PlaytimeMinutes != nil {
		t.Fatal("real PSN media, identity or missing-hours contract", items[0])
	}
	if items[1].Platforms[0] != "PS5" || items[1].Title != "第二个" || items[1].ImageURL != "" || items[1].PlaytimeMinutes == nil || *items[1].PlaytimeMinutes != 0 {
		t.Fatal("null/zero hours or empty-cover contract", items[1])
	}
}

func TestPSNProtocolRejectsIncompleteOrRepeatedGamePages(t *testing.T) {
	a := &App{}
	for _, raw := range []string{
		`{"titles":[],"totalItemCount":0}`,
		`{"titles":[],"totalItemCount":5001}`,
		`{"titles":[{"titleId":"A","name":"A","category":"ps4_game"}],"totalItemCount":2}`,
		`{"titles":[{"titleId":"A","name":"A","category":"ps4_game"}],"totalItemCount":2,"nextOffset":0}`,
		`{"titles":[{"titleId":"A","name":"A","category":"ps4_game"},{"titleId":"A","name":"A","category":"ps5_native_game"}],"totalItemCount":2}`,
		`{"titles":[{"titleId":"A","name":"A","category":"unknown"}],"totalItemCount":1}`,
	} {
		psnFixture(t, a, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, raw) })
		tokens := psnGoodTokens()
		if items, err := a.psnClient(nil).games(context.Background(), "123", &tokens, func(psnTokens) error { return nil }); err == nil || items != nil {
			t.Fatal("incomplete/invalid response produced snapshot")
		}
	}
}

func TestPSNProtocol401RefreshesOnce(t *testing.T) {
	a := &App{}
	var pages, refreshes atomic.Int32
	psnFixture(t, a, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			refreshes.Add(1)
			fmt.Fprint(w, `{"access_token":"rotated-access","refresh_token":"rotated-refresh","token_type":"Bearer","expires_in":3600,"refresh_token_expires_in":2592000}`)
			return
		}
		pages.Add(1)
		w.WriteHeader(401)
	})
	tokens := psnGoodTokens()
	persisted := false
	_, err := a.psnClient(nil).games(context.Background(), "123", &tokens, func(psnTokens) error { persisted = true; return nil })
	var failure *psnFailure
	if !errors.As(err, &failure) || failure.Code != "PSN_REAUTH_REQUIRED" || pages.Load() != 2 || refreshes.Load() != 1 || !persisted {
		t.Fatal("401 retry was unbounded or lost rotation", pages.Load(), refreshes.Load(), persisted, err)
	}
}

func TestPSNValidationDurationImagesAndSecureInput(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want *int64
	}{{"", nil}, {"PT", nil}, {"P1D", nil}, {"PT1.5H", nil}, {"-PT1H", nil}, {"PT999999999999999999999H", nil}, {"PT0S", psnInt(0)}, {"PT228H56M33S", psnInt(13736)}, {"PT59S", psnInt(0)}} {
		if got := psnMinutes(tc.raw); !reflect.DeepEqual(got, tc.want) {
			t.Error("duration", tc.raw, got, tc.want)
		}
	}
	for _, raw := range []string{"https://image.api.playstation.com/a.png", "https://image.api.np.km.playstation.net/a.png"} {
		if psnImage(raw) != raw {
			t.Error("official artwork rejected")
		}
	}
	for _, raw := range []string{"http://image.api.playstation.com/a.png", "https://image.api.playstation.com.evil.example/a.png", "https://image.api.playstation.com:443/a.png", "https://x@y@image.api.playstation.com/a.png", "https://tmdb.np.dl.playstation.net/a.png", "https://image.api.playstation.com/a.png#secret"} {
		if psnImage(raw) != "" {
			t.Error("unapproved artwork host/URL accepted")
		}
	}
	for _, raw := range []string{strings.Repeat("X", 63), strings.Repeat("X", 63) + ";", strings.Repeat("X", 63) + " ", strings.Repeat("X", 63) + "\n"} {
		if psnNPSSO(raw) {
			t.Error("unsafe cookie value accepted")
		}
	}
	for _, tc := range []struct {
		base, origin, host, peer string
		tls, want                bool
	}{
		{"http://painter.test", "http://painter.test", "painter.test", "192.168.1.2:1", false, false},
		{"https://blog.example", "", "blog.example", "192.168.1.2:1", false, false},
		{"https://blog.example", "http://blog.example", "blog.example", "192.168.1.2:1", false, false},
		{"https://blog.example", "https://evil.example", "blog.example", "192.168.1.2:1", false, false},
		{"https://blog.example", "https://blog.example", "blog.example", "192.168.1.2:1", false, true},
		{"http://localhost", "http://localhost", "localhost:18080", "127.0.0.1:1", false, true},
		{"http://localhost", "http://localhost", "localhost:18080", "192.168.1.2:1", false, false},
		{"http://painter.test", "http://painter.test", "painter.test", "192.168.1.2:1", true, true},
	} {
		r := httptest.NewRequest("PUT", "http://"+tc.host+"/api/v1/admin/home/psn", nil)
		r.Host, r.RemoteAddr = tc.host, tc.peer
		r.Header.Set("Origin", tc.origin)
		if tc.tls {
			r.TLS = &tls.ConnectionState{}
		}
		if got := psnSecureInput(r, tc.base); got != tc.want {
			t.Errorf("secure input %+v got %v", tc, got)
		}
	}
}

func psnInt(value int64) *int64 { return &value }

func psnFixtureConnect(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/authz/v3/oauth/authorize":
		w.Header().Set("Location", psnRedirect+"?code=fixture-code")
		w.WriteHeader(302)
	case "/api/authz/v3/oauth/token":
		fmt.Fprint(w, `{"access_token":"fixture-access","refresh_token":"fixture-refresh","token_type":"Bearer","expires_in":3600,"refresh_token_expires_in":2592000}`)
	case "/api/v1/devices/accounts/me":
		fmt.Fprint(w, `{"accountId":"12345678901234567890"}`)
	case "/api/userProfile/v1/internal/users/12345678901234567890/profiles":
		fmt.Fprint(w, `{"onlineId":"Fixture_Player"}`)
	default:
		w.WriteHeader(500)
	}
}

func psnMarshal(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
