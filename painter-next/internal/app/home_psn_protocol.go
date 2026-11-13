package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const psnOAuthBase = "https://ca.account.sony.com/api/authz/v3/oauth"
const psnClientID = "09515159-7237-4370-9b40-3806e67c0891"
const psnScope = "psn:mobile.v2.core psn:clientapp"
const psnRedirect = "com.scee.psxandroid.scecompcall://redirect"

// Published PS App protocol constant, not a Painter or user credential.
const psnClientAuthorization = "Basic MDk1MTUxNTktNzIzNy00MzcwLTliNDAtMzgwNmU2N2MwODkxOnVjUGprYTV0bnRCMktxc1A="
const psnReauthMessage = "PSN 连接已失效，请重新连接；已有游戏保持不变"

type psnTokens struct {
	Type           string `json:"type"`
	Access         string `json:"access_token"`
	Refresh        string `json:"refresh_token"`
	AccessExpires  int64  `json:"access_expires_at_ms"`
	RefreshExpires int64  `json:"refresh_expires_at_ms"`
}

type psnGame struct {
	ProviderID      string   `json:"provider_id"`
	Title           string   `json:"title"`
	Alt             string   `json:"alt"`
	URL             string   `json:"url"`
	ImageURL        string   `json:"image_url"`
	HeroURL         string   `json:"hero_url"`
	LogoURL         string   `json:"logo_url"`
	Platforms       []string `json:"platforms"`
	Position        int      `json:"position"`
	PlaytimeMinutes *int64   `json:"playtime_minutes"`
}

type psnFailure struct {
	Code    string
	Message string
	Status  int
	RetryAt int64
}

func (e *psnFailure) Error() string { return e.Code + ": " + e.Message }
func psnUnavailable() error {
	return &psnFailure{Code: "PSN_UPSTREAM_UNAVAILABLE", Message: "PSN 暂不可用；已有游戏保持不变", Status: 502}
}
func psnReauth() error {
	return &psnFailure{Code: "PSN_REAUTH_REQUIRED", Message: psnReauthMessage, Status: 409}
}
func psnSafeError(err error) error {
	var failure *psnFailure
	if errors.As(err, &failure) {
		return E(failure.Status, failure.Code, failure.Message)
	}
	return E(502, "PSN_UPSTREAM_UNAVAILABLE", "PSN 暂不可用；已有游戏保持不变")
}

var psnPacing struct {
	sync.Mutex
	next time.Time
}

type psnClient struct {
	http   *http.Client
	before func(context.Context) error
}

func (a *App) psnClient(before func(context.Context) error) *psnClient {
	client := &http.Client{Transport: homeProviderTransport, Timeout: 8 * time.Second}
	if a.Config.Environment == "test" && a.Config.HTTPClient != nil && a.Config.HTTPClient.Transport != nil {
		client.Transport = a.Config.HTTPClient.Transport
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &psnClient{http: client, before: func(ctx context.Context) error {
		if before != nil {
			if err := before(ctx); err != nil {
				return err
			}
		}
		if a.Config.Environment != "test" {
			psnPacing.Lock()
			at := time.Now()
			if psnPacing.next.After(at) {
				at = psnPacing.next
			}
			psnPacing.next = at.Add(3 * time.Second)
			psnPacing.Unlock()
			timer := time.NewTimer(time.Until(at))
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return psnUnavailable()
			case <-timer.C:
			}
		}
		if before != nil {
			return before(ctx)
		}
		return nil
	}}
}

func psnEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return false
	}
	switch u.Host {
	case "ca.account.sony.com":
		return u.Path == "/api/authz/v3/oauth/authorize" || u.Path == "/api/authz/v3/oauth/token"
	case "dms.api.playstation.com":
		return u.Path == "/api/v1/devices/accounts/me"
	case "m.np.playstation.com":
		for _, route := range []struct{ prefix, suffix string }{{"/api/gamelist/v2/users/", "/titles"}, {"/api/userProfile/v1/internal/users/", "/profiles"}} {
			if strings.HasPrefix(u.Path, route.prefix) && strings.HasSuffix(u.Path, route.suffix) {
				id := strings.TrimSuffix(strings.TrimPrefix(u.Path, route.prefix), route.suffix)
				return id == "me" || homeDigits(id, 1, 20)
			}
		}
	}
	return false
}

func (c *psnClient) request(ctx context.Context, method, target, body string, headers http.Header) ([]byte, string, error) {
	if !psnEndpoint(target) {
		return nil, "", psnUnavailable()
	}
	if c.before != nil {
		if err := c.before(ctx); err != nil {
			return nil, "", err
		}
	}
	select {
	case homeProviderSlots <- struct{}{}:
		defer func() { <-homeProviderSlots }()
	case <-ctx.Done():
		return nil, "", psnUnavailable()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, strings.NewReader(body))
	if err != nil {
		return nil, "", psnUnavailable()
	}
	req.Header = headers.Clone()
	req.Header.Set("Accept", "application/json")
	if method == http.MethodGet && req.Header.Get("Authorization") != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, "", psnUnavailable()
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 && response.StatusCode < 400 && strings.HasPrefix(target, psnOAuthBase+"/authorize?") {
		return nil, response.Header.Get("Location"), nil
	}
	if response.StatusCode != http.StatusOK {
		switch response.StatusCode {
		case 400:
			if !strings.HasPrefix(target, psnOAuthBase+"/") {
				return nil, "", psnUnavailable()
			}
			fallthrough
		case 401:
			return nil, "", &psnFailure{Code: "PSN_UNAUTHORIZED", Message: psnReauthMessage, Status: 409}
		case 403:
			return nil, "", &psnFailure{Code: "PSN_PRIVACY_OR_FORBIDDEN", Message: "PSN 游戏不可见或无访问权限；已有游戏保持不变", Status: 409}
		case 404:
			return nil, "", &psnFailure{Code: "PSN_ACCOUNT_UNAVAILABLE", Message: "PSN 账号或游戏数据暂不可见；已有游戏保持不变", Status: 409}
		case 429:
			at := time.Now().Add(15 * time.Minute)
			if seconds, err := strconv.ParseInt(response.Header.Get("Retry-After"), 10, 32); err == nil && seconds > 0 {
				at = time.Now().Add(time.Duration(seconds) * time.Second)
			} else if parsed, err := http.ParseTime(response.Header.Get("Retry-After")); err == nil && parsed.After(time.Now()) {
				at = parsed
			}
			return nil, "", &psnFailure{Code: "PSN_RATE_LIMITED", Message: "PSN 请求较频繁，请稍后重试；已有游戏保持不变", Status: 429, RetryAt: at.UnixMilli()}
		default:
			return nil, "", psnUnavailable()
		}
	}
	const max = 8 << 20
	raw, err := io.ReadAll(io.LimitReader(response.Body, max+1))
	if err != nil || len(raw) > max {
		return nil, "", psnUnavailable()
	}
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(raw, &envelope) != nil || (len(envelope.Error) > 0 && string(envelope.Error) != "null") {
		return nil, "", psnUnavailable()
	}
	return raw, "", nil
}

func psnSecret(value string) bool {
	return value != "" && len(value) <= 16384 && !strings.ContainsFunc(value, func(r rune) bool { return r < 33 || r > 126 })
}
func psnNPSSO(value string) bool {
	return len(value) == 64 && !strings.ContainsFunc(value, func(r rune) bool {
		return r < 0x21 || r > 0x7e || strings.ContainsRune("\";,\\", r)
	})
}

func (c *psnClient) exchange(ctx context.Context, npsso string) (psnTokens, error) {
	if !psnNPSSO(npsso) {
		return psnTokens{}, E(400, "VALIDATION_FAILED", "请输入64字符的有效 PSN 连接凭证")
	}
	query := url.Values{"access_type": {"offline"}, "client_id": {psnClientID}, "redirect_uri": {psnRedirect}, "response_type": {"code"}, "scope": {psnScope}}
	_, location, err := c.request(ctx, "GET", psnOAuthBase+"/authorize?"+query.Encode(), "", http.Header{"Cookie": {"npsso=" + npsso}})
	if err != nil {
		return psnTokens{}, err
	}
	redirect, err := url.Parse(location)
	if err != nil || redirect.Scheme != "com.scee.psxandroid.scecompcall" || redirect.Host != "redirect" || redirect.User != nil || redirect.Fragment != "" || (redirect.Path != "" && redirect.Path != "/") {
		return psnTokens{}, psnReauth()
	}
	values, err := url.ParseQuery(redirect.RawQuery)
	if err != nil || len(values["code"]) != 1 || values.Get("error") != "" || values.Get("error_code") != "" || !psnSecret(values.Get("code")) {
		return psnTokens{}, psnReauth()
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {values.Get("code")}, "redirect_uri": {psnRedirect}, "token_format": {"jwt"}}
	return c.token(ctx, form, psnTokens{})
}

func (c *psnClient) token(ctx context.Context, form url.Values, old psnTokens) (psnTokens, error) {
	raw, _, err := c.request(ctx, "POST", psnOAuthBase+"/token", form.Encode(), http.Header{"Authorization": {psnClientAuthorization}, "Content-Type": {"application/x-www-form-urlencoded"}})
	if err != nil {
		var failure *psnFailure
		if errors.As(err, &failure) && failure.Code == "PSN_UNAUTHORIZED" {
			return psnTokens{}, psnReauth()
		}
		return psnTokens{}, err
	}
	var response struct {
		Access         string `json:"access_token"`
		Refresh        string `json:"refresh_token"`
		Expires        int64  `json:"expires_in"`
		RefreshExpires int64  `json:"refresh_token_expires_in"`
		TokenType      string `json:"token_type"`
	}
	if json.Unmarshal(raw, &response) != nil || !psnSecret(response.Access) || !strings.EqualFold(response.TokenType, "Bearer") || response.Expires <= 0 || response.Expires > 7*24*3600 || response.RefreshExpires < 0 || response.RefreshExpires > 2*366*24*3600 {
		return psnTokens{}, psnUnavailable()
	}
	result := psnTokens{Type: "psn_tokens_v1", Access: response.Access, Refresh: response.Refresh, AccessExpires: NowMillis() + response.Expires*1000, RefreshExpires: old.RefreshExpires}
	if result.Refresh == "" {
		result.Refresh = old.Refresh
	}
	if response.RefreshExpires > 0 {
		result.RefreshExpires = NowMillis() + response.RefreshExpires*1000
	}
	if !psnSecret(result.Refresh) || result.RefreshExpires <= NowMillis() {
		return psnTokens{}, psnReauth()
	}
	return result, nil
}

func (c *psnClient) refresh(ctx context.Context, old psnTokens) (psnTokens, error) {
	if !psnSecret(old.Refresh) || old.RefreshExpires <= NowMillis() {
		return psnTokens{}, psnReauth()
	}
	return c.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {old.Refresh}, "token_format": {"jwt"}, "scope": {psnScope}}, old)
}

func (c *psnClient) identity(ctx context.Context, tokens psnTokens) (string, string, error) {
	headers := http.Header{"Authorization": {"Bearer " + tokens.Access}}
	raw, _, err := c.request(ctx, "GET", "https://dms.api.playstation.com/api/v1/devices/accounts/me", "", headers)
	if err != nil {
		return "", "", err
	}
	var account struct {
		ID string `json:"accountId"`
	}
	if json.Unmarshal(raw, &account) != nil || !homeDigits(account.ID, 1, 20) {
		return "", "", psnUnavailable()
	}
	raw, _, err = c.request(ctx, "GET", "https://m.np.playstation.com/api/userProfile/v1/internal/users/"+account.ID+"/profiles", "", headers)
	if err != nil {
		return "", "", err
	}
	var profile struct {
		OnlineID string `json:"onlineId"`
	}
	if json.Unmarshal(raw, &profile) != nil || profile.OnlineID == "" || homeText(profile.OnlineID, 80, false) != nil {
		return "", "", psnUnavailable()
	}
	return account.ID, profile.OnlineID, nil
}

var psnDuration = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)
var psnIdentifier = regexp.MustCompile(`^[A-Za-z0-9:_-]{1,64}$`)

func psnMinutes(raw string) *int64 {
	parts := psnDuration.FindStringSubmatch(raw)
	if parts == nil || parts[1]+parts[2]+parts[3] == "" {
		return nil
	}
	seconds := int64(0)
	for index, multiplier := range []int64{3600, 60, 1} {
		if parts[index+1] == "" {
			continue
		}
		value, err := strconv.ParseInt(parts[index+1], 10, 64)
		if err != nil || value > 52704000*60/multiplier || seconds > 52704000*60-value*multiplier {
			return nil
		}
		seconds += value * multiplier
	}
	minutes := seconds / 60
	return &minutes
}

func psnImage(raw string) string {
	if !homeRemoteImage("psn", raw) {
		return ""
	}
	return raw
}

func (c *psnClient) games(ctx context.Context, accountID string, tokens *psnTokens, persist func(psnTokens) error) ([]psnGame, error) {
	if !homeDigits(accountID, 1, 20) {
		return nil, psnUnavailable()
	}
	refresh := func() error {
		fresh, err := c.refresh(ctx, *tokens)
		if err != nil {
			return err
		}
		if err = persist(fresh); err != nil {
			return err
		}
		*tokens = fresh
		return nil
	}
	if tokens.AccessExpires <= NowMillis()+60000 {
		if err := refresh(); err != nil {
			return nil, err
		}
	}
	items := make([]psnGame, 0)
	seen := map[string]bool{}
	visited := map[int]bool{}
	offset := 0
	for page := 0; page < 25; page++ {
		if visited[offset] {
			return nil, psnUnavailable()
		}
		visited[offset] = true
		query := url.Values{"limit": {"200"}, "offset": {strconv.Itoa(offset)}, "categories": {"ps4_game,ps5_native_game"}}
		target := "https://m.np.playstation.com/api/gamelist/v2/users/" + accountID + "/titles?" + query.Encode()
		raw, _, err := c.request(ctx, "GET", target, "", http.Header{"Authorization": {"Bearer " + tokens.Access}})
		var failure *psnFailure
		if errors.As(err, &failure) && failure.Code == "PSN_UNAUTHORIZED" {
			if err = refresh(); err != nil {
				return nil, err
			}
			raw, _, err = c.request(ctx, "GET", target, "", http.Header{"Authorization": {"Bearer " + tokens.Access}})
			if errors.As(err, &failure) && failure.Code == "PSN_UNAUTHORIZED" {
				return nil, psnReauth()
			}
		}
		if err != nil {
			return nil, err
		}
		var response struct {
			Titles *[]struct {
				ID         string `json:"titleId"`
				Name       string `json:"name"`
				Localized  string `json:"localizedName"`
				Image      string `json:"imageUrl"`
				LocalImage string `json:"localizedImageUrl"`
				Category   string `json:"category"`
				Duration   string `json:"playDuration"`
				Concept    struct {
					ID    json.Number `json:"id"`
					Media struct {
						Images []struct {
							URL  string `json:"url"`
							Type string `json:"type"`
						} `json:"images"`
					} `json:"media"`
				} `json:"concept"`
			} `json:"titles"`
			Total *int `json:"totalItemCount"`
			Next  *int `json:"nextOffset"`
		}
		if json.Unmarshal(raw, &response) != nil || response.Titles == nil || response.Total == nil || *response.Total < 0 {
			return nil, psnUnavailable()
		}
		if *response.Total > homeItemLimit {
			return nil, &psnFailure{Code: "PSN_ITEM_LIMIT", Message: "PSN 游戏超过5000项，未修改已有内容", Status: 409}
		}
		if len(*response.Titles) == 0 {
			if len(items) == 0 {
				return nil, &psnFailure{Code: "PSN_EMPTY_OR_PRIVATE", Message: "PSN 未返回可见游戏；已有游戏保持不变", Status: 409}
			}
			if len(items) == *response.Total {
				return items, nil
			}
			return nil, psnUnavailable()
		}
		for _, title := range *response.Titles {
			if !psnIdentifier.MatchString(title.ID) || seen[title.ID] {
				return nil, psnUnavailable()
			}
			seen[title.ID] = true
			platform := ""
			switch title.Category {
			case "ps4_game":
				platform = "PS4"
			case "ps5_native_game":
				platform = "PS5"
			default:
				return nil, psnUnavailable()
			}
			name := strings.TrimSpace(title.Localized)
			if name == "" {
				name = strings.TrimSpace(title.Name)
			}
			name = strings.Map(func(r rune) rune {
				if unicode.IsControl(r) {
					return -1
				}
				return r
			}, name)
			if name == "" {
				return nil, psnUnavailable()
			}
			runes := []rune(name)
			if len(runes) > 80 {
				runes = runes[:80]
			}
			name = string(runes)
			image := psnImage(title.LocalImage)
			if image == "" {
				image = psnImage(title.Image)
			}
			art := map[string]string{}
			for _, media := range title.Concept.Media.Images {
				if value := psnImage(media.URL); value != "" && art[media.Type] == "" {
					art[media.Type] = value
				}
			}
			if art["PORTRAIT_BANNER"] != "" {
				image = art["PORTRAIT_BANNER"]
			}
			hero := ""
			for _, kind := range []string{"SIXTEEN_BY_NINE_BANNER", "GAMEHUB_COVER_ART", "BACKGROUND_LAYER_ART", "FOUR_BY_THREE_BANNER"} {
				if art[kind] != "" {
					hero = art[kind]
					break
				}
			}
			link := ""
			if homeDigits(string(title.Concept.ID), 1, 20) {
				link = "https://store.playstation.com/en-us/concept/" + string(title.Concept.ID)
			}
			items = append(items, psnGame{ProviderID: title.ID, Title: name, Alt: name, URL: link, ImageURL: image, HeroURL: hero, LogoURL: art["LOGO"], Platforms: []string{platform}, Position: len(items), PlaytimeMinutes: psnMinutes(title.Duration)})
			if len(items) > homeItemLimit {
				return nil, psnUnavailable()
			}
		}
		if len(items) == *response.Total {
			return items, nil
		}
		if len(items) > *response.Total || response.Next == nil || *response.Next <= offset || *response.Next > homeItemLimit {
			return nil, psnUnavailable()
		}
		offset = *response.Next
	}
	return nil, psnUnavailable()
}
