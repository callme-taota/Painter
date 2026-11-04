package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var homeProviderSlots = make(chan struct{}, 4)
var homeProviderTransport = &http.Transport{Proxy: nil, MaxConnsPerHost: 4, MaxIdleConnsPerHost: 2, TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 6 * time.Second, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, DialContext: homePublicDial}

func homePublicDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, e := net.SplitHostPort(address)
	if e != nil {
		return nil, homeUpstreamError()
	}
	ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if e != nil || len(ips) == 0 {
		return nil, homeUpstreamError()
	}
	for _, ip := range ips {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return nil, homeUpstreamError()
		}
	}
	dialer := net.Dialer{Timeout: 4 * time.Second}
	for _, ip := range ips {
		conn, e := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if e == nil {
			return conn, nil
		}
	}
	return nil, homeUpstreamError()
}
func homeRemoteImage(kind, raw string) bool {
	if _, e := friendURL(raw, true); e != nil {
		return false
	}
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Port() != "" || u.RawQuery != "" && len(u.RawQuery) > 512 {
		return false
	}
	h := strings.ToLower(u.Hostname())
	if kind == "music" {
		return h == "mzstatic.com" || strings.HasSuffix(h, ".mzstatic.com")
	}
	if kind == "steam" {
		return h == "steamstatic.com" || strings.HasSuffix(h, ".steamstatic.com")
	}
	return false
}
func (a *App) homeProviderGET(ctx context.Context, target, key string, max int64) ([]byte, error) {
	u, e := url.Parse(target)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || !(u.Host == "store.steampowered.com" || u.Host == "api.steampowered.com" || u.Host == "itunes.apple.com") {
		return nil, homeUpstreamError()
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	select {
	case homeProviderSlots <- struct{}{}:
		defer func() { <-homeProviderSlots }()
	case <-ctx.Done():
		return nil, homeUpstreamError()
	}
	req, e := http.NewRequestWithContext(ctx, "GET", target, nil)
	if e != nil {
		return nil, homeUpstreamError()
	}
	req.Header.Set("Accept", "application/json")
	if key != "" {
		req.Header.Set("x-webapi-key", key)
	}
	client := http.Client{Transport: homeProviderTransport}
	if a.Config.Environment == "test" && a.Config.HTTPClient != nil && a.Config.HTTPClient.Transport != nil {
		client = *a.Config.HTTPClient
	}
	client.Timeout = 8 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, e := client.Do(req)
	if e != nil {
		return nil, homeUpstreamError()
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, homeUpstreamError()
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, max+1))
	if int64(len(raw)) > max {
		return nil, E(502, "HOME_PROVIDER_RESPONSE_TOO_LARGE", "供应商响应超过大小限制，未修改已有内容")
	}
	if e != nil {
		return nil, homeUpstreamError()
	}
	return raw, nil
}
func homeParseImport(kind, input string) (id, country string, err error) {
	invalid := func() (string, string, error) {
		return "", "", E(400, "VALIDATION_FAILED", "请输入 Steam App ID/商店链接或 Apple Music 专辑链接")
	}
	country = "us"
	if kind == "steam" && homeDigits(input, 1, 10) {
		return input, country, nil
	}
	if len(input) > 2048 || strings.TrimSpace(input) != input {
		return invalid()
	}
	u, e := url.Parse(input)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" || strings.ContainsRune(input, '\\') {
		return invalid()
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch kind {
	case "steam":
		if u.Host != "store.steampowered.com" || len(parts) < 2 || len(parts) > 3 || parts[0] != "app" || !homeDigits(parts[1], 1, 10) {
			return invalid()
		}
		id = parts[1]
	case "music":
		if u.Host != "music.apple.com" || len(parts) < 3 || len(parts) > 4 || len(parts[0]) != 2 || parts[1] != "album" || !homeDigits(parts[len(parts)-1], 1, 20) {
			return invalid()
		}
		country = strings.ToLower(parts[0])
		for _, r := range country {
			if r < 'a' || r > 'z' {
				return invalid()
			}
		}
		id = parts[len(parts)-1]
	default:
		return invalid()
	}
	return id, country, nil
}
func homeMetadata(in homeItemInput) map[string]any {
	return map[string]any{"kind": in.Kind, "title": in.Title, "subtitle": in.Subtitle, "alt": in.Alt, "url": in.URL, "image_url": in.ImageURL, "hero_url": in.HeroURL, "logo_url": in.LogoURL, "playtime_minutes": in.PlaytimeMinutes, "media_id": "", "provider_id": in.ProviderID, "source": "manual", "position": 0, "visible": true}
}
func (a *App) homeFetchImport(ctx context.Context, kind, id, country string) (homeItemInput, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	in := homeItemInput{Kind: kind, ProviderID: id, Visible: true}
	if kind == "steam" {
		raw, e := a.homeProviderGET(ctx, "https://store.steampowered.com/api/appdetails?appids="+id+"&cc=us&l=schinese", "", 2<<20)
		if e != nil {
			return in, e
		}
		var response map[string]struct {
			Success bool `json:"success"`
			Data    struct {
				Name  string `json:"name"`
				Image string `json:"header_image"`
				ID    int64  `json:"steam_appid"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &response) != nil {
			return in, homeUpstreamError()
		}
		game, ok := response[id]
		if !ok || !game.Success || strconv.FormatInt(game.Data.ID, 10) != id || strings.TrimSpace(game.Data.Name) == "" {
			return in, homeUpstreamError()
		}
		in.Title = game.Data.Name
		in.Alt = in.Title
		in.ImageURL = game.Data.Image
		in.URL = "https://store.steampowered.com/app/" + id + "/"
		in = a.enhanceSteamImport(ctx, in)
	} else {
		raw, e := a.homeProviderGET(ctx, "https://itunes.apple.com/lookup?id="+id+"&entity=album&country="+country, "", 2<<20)
		if e != nil {
			return in, e
		}
		var response struct {
			Results []struct {
				Wrapper string `json:"wrapperType"`
				Type    string `json:"collectionType"`
				ID      int64  `json:"collectionId"`
				Name    string `json:"collectionName"`
				Artist  string `json:"artistName"`
				URL     string `json:"collectionViewUrl"`
				Image   string `json:"artworkUrl100"`
			} `json:"results"`
		}
		if json.Unmarshal(raw, &response) != nil {
			return in, homeUpstreamError()
		}
		found := false
		for _, album := range response.Results {
			if album.Wrapper != "collection" || album.Type != "Album" || strconv.FormatInt(album.ID, 10) != id {
				continue
			}
			u, e := url.Parse(album.URL)
			if e != nil || u.Scheme != "https" || u.Host != "music.apple.com" || u.User != nil {
				continue
			}
			in.Title = album.Name
			in.Subtitle = album.Artist
			in.URL = album.URL
			in.ImageURL = album.Image
			in.Alt = album.Name
			found = true
			break
		}
		if !found || strings.TrimSpace(in.Title) == "" {
			return in, homeUpstreamError()
		}
		in = a.homeMusicEnhanceAlbum(ctx, in)
	}
	if validateHomeItem(in) != nil {
		return in, homeUpstreamError()
	}
	return in, nil
}
func (a *App) homeImportRate(ctx context.Context, owner, provider string) error {
	return a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		return homeRateTx(ctx, tx, owner, provider)
	})
}
func homeRateTx(ctx context.Context, tx pgx.Tx, owner, provider string) error {
	var count int
	now := NowMillis()
	e := tx.QueryRow(ctx, `INSERT INTO homepage_provider_rates(owner_id,provider,window_at_ms,requests) VALUES($1,$2,$3,1) ON CONFLICT(owner_id,provider) DO UPDATE SET window_at_ms=CASE WHEN homepage_provider_rates.window_at_ms<=$3-60000 THEN $3 ELSE homepage_provider_rates.window_at_ms END,requests=CASE WHEN homepage_provider_rates.window_at_ms<=$3-60000 THEN 1 ELSE homepage_provider_rates.requests+1 END WHERE homepage_provider_rates.window_at_ms<=$3-60000 OR homepage_provider_rates.requests<20 RETURNING requests`, owner, provider, now).Scan(&count)
	if e == pgx.ErrNoRows {
		return E(429, "HOME_PROVIDER_RATE_LIMIT", "每个供应商每分钟最多20次请求，请稍后重试")
	}
	return e
}
func (a *App) homeImport(w http.ResponseWriter, r *http.Request) error {
	if e := homeAuthorize(a, r); e != nil {
		return e
	}
	var in struct {
		Kind  string `json:"kind"`
		Input string `json:"input"`
	}
	if e := decodeHome(w, r, &in); e != nil {
		return e
	}
	id, country, e := homeParseImport(in.Kind, in.Input)
	if e != nil {
		return e
	}
	replay, found, e := a.Replay(r, "owner")
	if e != nil {
		return e
	}
	if found {
		JSON(w, 200, replay)
		return nil
	}
	p, e := a.Require(r, "owner")
	if e != nil {
		return e
	}
	var raw []byte
	e = a.Pool.QueryRow(r.Context(), `SELECT metadata FROM homepage_import_cache WHERE kind=$1 AND provider_id=$2 AND country=$3 AND expires_at_ms>$4`, in.Kind, id, country, NowMillis()).Scan(&raw)
	var item homeItemInput
	cached := e == nil
	if e == nil {
		if json.Unmarshal(raw, &item) != nil || validateHomeItem(item) != nil {
			return homeUpstreamError()
		}
	} else if e == pgx.ErrNoRows {
		if e = a.homeImportRate(r.Context(), p.ID, in.Kind); e != nil {
			return e
		}
		item, e = a.homeFetchImport(r.Context(), in.Kind, id, country)
		if e != nil {
			return e
		}
	} else {
		return e
	}
	value, e := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		if _, e := homeRead(r.Context(), tx, true); e != nil {
			return nil, e
		}
		if !cached {
			if _, e := tx.Exec(r.Context(), `DELETE FROM homepage_import_cache WHERE expires_at_ms<=$1`, NowMillis()); e != nil {
				return nil, e
			}
			if _, e := tx.Exec(r.Context(), `DELETE FROM homepage_import_cache WHERE (kind,provider_id,country) IN(SELECT kind,provider_id,country FROM homepage_import_cache ORDER BY expires_at_ms DESC,kind,provider_id,country OFFSET 4999)`); e != nil {
				return nil, e
			}
			raw, _ := json.Marshal(item)
			_, e := tx.Exec(r.Context(), `INSERT INTO homepage_import_cache(kind,provider_id,country,metadata,expires_at_ms) VALUES($1,$2,$3,$4,$5) ON CONFLICT(kind,provider_id,country) DO UPDATE SET metadata=EXCLUDED.metadata,expires_at_ms=EXCLUDED.expires_at_ms`, in.Kind, id, country, raw, NowMillis()+24*time.Hour.Milliseconds())
			if e != nil {
				return nil, e
			}
		}
		return homeMetadata(item), nil
	})
	if e == nil {
		JSON(w, 200, value)
	}
	return e
}
