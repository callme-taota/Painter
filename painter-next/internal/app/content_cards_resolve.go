package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	xhtml "golang.org/x/net/html"
)

// No provider credentials are required or attached to card requests.
type GitHubRepositoryMetadata struct {
	Owner, Name, FullName, Description, URL, AvatarURL, Language string
	Stars, Forks                                                 *int64
	UpdatedAt                                                    string
}

func contentCardUnavailable() error {
	return E(502, "CARD_PROVIDER_UNAVAILABLE", "未能读取公开网页或供应商元数据，请检查链接后重试；也可填写网页参考标题")
}

var contentCardReservedNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"), netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"), netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"),
}

func contentCardPublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range contentCardReservedNetworks {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func contentCardPublicDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, e := net.SplitHostPort(address)
	if e != nil {
		return nil, contentCardUnavailable()
	}
	ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if e != nil || len(ips) == 0 {
		return nil, contentCardUnavailable()
	}
	for _, ip := range ips {
		if !contentCardPublicIP(ip) {
			return nil, contentCardUnavailable()
		}
	}
	dialer := net.Dialer{Timeout: 4 * time.Second}
	for _, ip := range ips {
		if conn, e := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port)); e == nil {
			return conn, nil
		}
	}
	return nil, contentCardUnavailable()
}

var contentCardTransport = func() *http.Transport {
	t := homeProviderTransport.Clone()
	t.DialContext = contentCardPublicDial
	return t
}()

// Fixed provider hosts do not redirect. Generic references permit at most two
// public HTTPS redirects; every hop goes through the same DNS-pinning dialer.
func (a *App) contentCardGET(ctx context.Context, target, mode string, max int64) ([]byte, string, error) {
	u, e := contentCardPublicURL(target)
	if e != nil {
		return nil, "", e
	}
	allowed := func(u *url.URL) bool {
		if mode == "github" {
			return u.Host == "api.github.com"
		}
		if mode == "steam" {
			return u.Host == "store.steampowered.com"
		}
		if mode == "apple_lookup" {
			return u.Host == "itunes.apple.com"
		}
		if mode == "apple_page" {
			return u.Host == "music.apple.com"
		}
		return mode == "link"
	}
	if !allowed(u) {
		return nil, "", contentCardInvalid("供应商网址不匹配")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	select {
	case homeProviderSlots <- struct{}{}:
		defer func() { <-homeProviderSlots }()
	case <-ctx.Done():
		return nil, "", contentCardUnavailable()
	}
	client := http.Client{Transport: contentCardTransport, Timeout: 8 * time.Second}
	if a.Config.Environment == "test" && a.Config.HTTPClient != nil && a.Config.HTTPClient.Transport != nil {
		client = *a.Config.HTTPClient
		client.Timeout = 8 * time.Second
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if mode != "link" && mode != "apple_page" || len(via) > 2 {
			return http.ErrUseLastResponse
		}
		u, e := contentCardPublicURL(req.URL.String())
		if e != nil || !allowed(u) {
			return http.ErrUseLastResponse
		}
		if mode == "apple_page" {
			kind := "album"
			if strings.Contains(target, "/playlist/") {
				kind = "playlist"
			}
			oldID, oldCountry, _, oldErr := homeMusicParseURL(target, kind)
			id, country, _, newErr := homeMusicParseURL(req.URL.String(), kind)
			if oldErr != nil || newErr != nil || id != oldID || country != oldCountry {
				return http.ErrUseLastResponse
			}
		}
		return nil
	}
	req, e := http.NewRequestWithContext(ctx, "GET", target, nil)
	if e != nil {
		return nil, "", contentCardUnavailable()
	}
	req.Header.Set("User-Agent", "Painter-Reference/1.0")
	if mode == "link" || mode == "apple_page" {
		req.Header.Set("Accept", "text/html")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	if mode == "github" {
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	response, e := client.Do(req)
	if e != nil {
		return nil, "", contentCardUnavailable()
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, "", contentCardUnavailable()
	}
	ct := strings.ToLower(response.Header.Get("Content-Type"))
	if ct != "" && (mode == "link" || mode == "apple_page") && !strings.HasPrefix(ct, "text/html") && !strings.HasPrefix(ct, "application/xhtml+xml") {
		return nil, "", contentCardUnavailable()
	}
	if response.ContentLength > max {
		return nil, "", contentCardUnavailable()
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, max+1))
	if e != nil || int64(len(raw)) > max {
		return nil, "", contentCardUnavailable()
	}
	resolved := target
	if response.Request != nil && response.Request.URL != nil {
		resolved = response.Request.URL.String()
	}
	return raw, resolved, nil
}

func (a *App) fetchPublicGitHubRepository(ctx context.Context, reference string) (GitHubRepositoryMetadata, error) {
	var result GitHubRepositoryMetadata
	owner, name, e := parseGitHubRepository(reference)
	if e != nil {
		return result, e
	}
	raw, _, e := a.contentCardGET(ctx, "https://api.github.com/repos/"+owner+"/"+name, "github", 2<<20)
	if e != nil {
		return result, e
	}
	var repository struct {
		Name        string `json:"name"`
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		URL         string `json:"html_url"`
		Private     bool   `json:"private"`
		Stars       *int64 `json:"stargazers_count"`
		Forks       *int64 `json:"forks_count"`
		Language    string `json:"language"`
		UpdatedAt   string `json:"updated_at"`
		Owner       struct {
			Login  string `json:"login"`
			Avatar string `json:"avatar_url"`
		} `json:"owner"`
	}
	if json.Unmarshal(raw, &repository) != nil || repository.Private || !strings.EqualFold(repository.FullName, owner+"/"+name) || !strings.EqualFold(repository.Name, name) || !strings.EqualFold(repository.Owner.Login, owner) {
		return result, contentCardUnavailable()
	}
	result = GitHubRepositoryMetadata{Owner: repository.Owner.Login, Name: repository.Name, FullName: repository.FullName, Description: homeMusicText(repository.Description, 1000), URL: repository.URL, AvatarURL: repository.Owner.Avatar, Language: homeMusicText(repository.Language, 100), Stars: repository.Stars, Forks: repository.Forks, UpdatedAt: repository.UpdatedAt}
	check := contentCard{V: 1, Type: "github", URL: result.URL, Title: result.FullName, Description: result.Description, Subtitle: result.Language, ImageURL: result.AvatarURL, Stats: &contentCardStats{Stars: result.Stars, Forks: result.Forks}}
	if _, e := validateContentCard(check); e != nil {
		return GitHubRepositoryMetadata{}, contentCardUnavailable()
	}
	returnedOwner, returnedName, e := parseGitHubRepository(result.URL)
	if e != nil || !strings.EqualFold(returnedOwner, owner) || !strings.EqualFold(returnedName, name) {
		return GitHubRepositoryMetadata{}, contentCardUnavailable()
	}
	return result, nil
}

func contentCardHTMLMetadata(raw []byte) (title, description, image string) {
	z := xhtml.NewTokenizer(bytes.NewReader(raw))
	var pageTitle strings.Builder
	inTitle := false
	for {
		kind := z.Next()
		if kind == xhtml.ErrorToken {
			break
		}
		if kind == xhtml.TextToken && inTitle {
			pageTitle.Write(z.Text())
		}
		if kind != xhtml.StartTagToken && kind != xhtml.SelfClosingTagToken && kind != xhtml.EndTagToken {
			continue
		}
		token := z.Token()
		if token.Data == "title" {
			inTitle = kind != xhtml.EndTagToken
		}
		if token.Data != "meta" || kind == xhtml.EndTagToken {
			continue
		}
		name, value := "", ""
		for _, attr := range token.Attr {
			if attr.Key == "name" || attr.Key == "property" {
				name = strings.ToLower(attr.Val)
			}
			if attr.Key == "content" {
				value = attr.Val
			}
		}
		switch name {
		case "og:title":
			if title == "" {
				title = value
			}
		case "description", "og:description":
			if description == "" {
				description = value
			}
		case "og:image":
			if image == "" {
				image = value
			}
		}
	}
	if title == "" {
		title = pageTitle.String()
	}
	return homeMusicText(title, 200), homeMusicText(description, 1000), image
}

type contentCardResolveInput struct {
	Type        string `json:"type"`
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	RefID       string `json:"ref_id,omitempty"`
}

func (a *App) contentCardResolve(ctx context.Context, in contentCardResolveInput) (contentCard, error) {
	if contentCardIsInternal(in.Type) {
		id, e := contentCardInputReference(in)
		if e != nil {
			return contentCard{}, e
		}
		c := contentCard{V: 1, Type: in.Type, URL: contentCardInternalURL(in.Type, id), RefID: id}
		if _, e = a.PublicContentCard(ctx, in.Type, id); e != nil {
			return c, e
		}
		return c, nil
	}
	if in.RefID != "" {
		return contentCard{}, contentCardInvalid("外部卡片不能包含站内引用编号")
	}
	c := contentCard{V: 1, Type: in.Type, URL: in.URL, MetadataSource: "provider", ResolvedAt: time.Now().UTC().Format(time.RFC3339)}
	switch in.Type {
	case "github":
		r, e := a.fetchPublicGitHubRepository(ctx, in.URL)
		if e != nil {
			return c, e
		}
		c.URL, c.Title, c.Description, c.Subtitle, c.ImageURL = r.URL, r.FullName, r.Description, r.Language, r.AvatarURL
		if r.Stars != nil || r.Forks != nil {
			c.Stats = &contentCardStats{Stars: r.Stars, Forks: r.Forks}
		}
	case "steam":
		id, _, e := homeParseImport("steam", in.URL)
		if e != nil {
			return c, contentCardInvalid("请输入 Steam 游戏商店链接")
		}
		raw, _, e := a.contentCardGET(ctx, "https://store.steampowered.com/api/appdetails?appids="+id+"&cc=us&l=schinese", "steam", 2<<20)
		if e != nil {
			return c, e
		}
		var response map[string]struct {
			Success bool `json:"success"`
			Data    struct {
				Name        string `json:"name"`
				Image       string `json:"header_image"`
				ID          int64  `json:"steam_appid"`
				Description string `json:"short_description"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &response) != nil {
			return c, contentCardUnavailable()
		}
		game, ok := response[id]
		if !ok || !game.Success || strconv.FormatInt(game.Data.ID, 10) != id {
			return c, contentCardUnavailable()
		}
		c.URL, c.Title, c.ImageURL, c.Description = "https://store.steampowered.com/app/"+id+"/", homeMusicText(game.Data.Name, 200), game.Data.Image, homeMusicText(game.Data.Description, 1000)
	case "apple_music":
		id, country, canonical, e := homeMusicParseURL(in.URL, "album")
		if e != nil {
			_, _, canonical, e = homeMusicParseURL(in.URL, "playlist")
			if e != nil {
				return c, contentCardInvalid("请输入 Apple Music 专辑或歌单链接")
			}
			raw, resolved, e := a.contentCardGET(ctx, canonical, "apple_page", homeMusicHTMLLimit)
			if e != nil {
				return c, e
			}
			c.URL, c.Title, c.Description, c.ImageURL = resolved, "", "", ""
			c.Title, c.Description, c.ImageURL = contentCardHTMLMetadata(raw)
		} else {
			raw, _, e := a.contentCardGET(ctx, "https://itunes.apple.com/lookup?id="+id+"&entity=album&country="+country, "apple_lookup", 2<<20)
			if e != nil {
				return c, e
			}
			var response struct {
				Results []struct {
					Wrapper string `json:"wrapperType"`
					Type    string `json:"collectionType"`
					ID      int64  `json:"collectionId"`
					Title   string `json:"collectionName"`
					Artist  string `json:"artistName"`
					Image   string `json:"artworkUrl100"`
					URL     string `json:"collectionViewUrl"`
				} `json:"results"`
			}
			if json.Unmarshal(raw, &response) != nil {
				return c, contentCardUnavailable()
			}
			for _, album := range response.Results {
				if album.Wrapper == "collection" && album.Type == "Album" && strconv.FormatInt(album.ID, 10) == id {
					c.Title, c.Subtitle, c.ImageURL = homeMusicText(album.Title, 200), homeMusicText(album.Artist, 200), album.Image
					c.URL = canonical
					break
				}
			}
		}
	case "link":
		if _, e := contentCardPublicURL(in.URL); e != nil {
			return c, e
		}
		if in.Title != "" {
			c.Title, c.Description, c.MetadataSource = in.Title, in.Description, "author"
		} else {
			raw, resolved, e := a.contentCardGET(ctx, in.URL, "link", 1<<20)
			if e != nil {
				return c, e
			}
			c.URL = resolved
			c.Title, c.Description, _ = contentCardHTMLMetadata(raw)
		}
	default:
		return c, contentCardInvalid("不支持此卡片类型")
	}
	if _, e := validateContentCard(c); e != nil {
		return c, e
	}
	return c, nil
}

func (a *App) contentCardRate(id string) bool {
	a.rateMu.Lock()
	defer a.rateMu.Unlock()
	if a.rates == nil {
		a.rates = map[string]rateBucket{}
	}
	now := NowMillis()
	key := "card-resolve:" + id
	b := a.rates[key]
	if b.Reset <= now {
		b = rateBucket{Reset: now + 60000}
	}
	if b.Count >= 20 {
		return false
	}
	if len(a.rates) >= 10000 {
		for key, value := range a.rates {
			if value.Reset <= now {
				delete(a.rates, key)
			}
		}
		if _, ok := a.rates[key]; !ok && len(a.rates) >= 10000 {
			return false
		}
	}
	b.Count++
	a.rates[key] = b
	return true
}

func (a *App) resolveContentCard(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "author", "owner"); e != nil {
		return e
	}
	if e := a.checkCSRF(r); e != nil {
		return e
	}
	r.Body = http.MaxBytesReader(w, r.Body, contentCardLimit)
	var in contentCardResolveInput
	if e := Decode(r, &in); e != nil {
		return e
	}
	if _, known := contentCardRegistry[in.Type]; !known {
		return contentCardInvalid("不支持此卡片类型")
	}
	if !contentCardText(in.Title, 200, false) || !contentCardText(in.Description, 1000, false) {
		return contentCardInvalid("标题或简介超长或含控制字符")
	}
	_, e := a.Write(r, []string{"author", "owner"}, false, false, func(_ pgx.Tx, p Principal) (any, error) {
		if !a.contentCardRate(p.ID) {
			return nil, E(429, "CARD_RATE_LIMIT", "每分钟最多解析20个内容卡片，请稍后重试")
		}
		return nil, nil
	})
	if e != nil {
		return e
	}
	c, e := a.contentCardResolve(r.Context(), in)
	if e != nil {
		return e
	}
	result, e := a.Write(r, []string{"author", "owner"}, false, false, func(tx pgx.Tx, _ Principal) (any, error) {
		value := map[string]any{"card": c, "markdown": contentCardMarkdown(c)}
		if contentCardIsInternal(c.Type) {
			references, e := contentCardLookup(r.Context(), tx, []contentCard{c})
			if e != nil {
				return nil, e
			}
			reference, ok := references[c.Type+":"+c.RefID]
			if !ok {
				return nil, NotFound()
			}
			value["reference"] = reference
		}
		if c.Type == "link" {
			value["inline_markdown"] = contentCardInlineMarkdown(c)
		}
		return value, nil
	})
	if e == nil {
		JSON(w, 200, result)
	}
	return e
}
