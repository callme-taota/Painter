package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"golang.org/x/net/html"
)

const homeMusicTrackLimit = 300
const homeMusicHTMLLimit = 4 << 20

type homeMusicSnapshot struct {
	SourceURL   string          `json:"source_url"`
	ResolvedURL string          `json:"resolved_url"`
	PlaylistID  string          `json:"playlist_id"`
	Title       string          `json:"title"`
	TrackCount  int             `json:"track_count"`
	AlbumCount  int             `json:"album_count"`
	Partial     bool            `json:"partial"`
	Items       []homeItemInput `json:"items"`
}

type homeMusicDescriptor struct {
	Kind        string `json:"kind"`
	URL         string `json:"url"`
	Identifiers struct {
		ID string `json:"storeAdamID"`
	} `json:"identifiers"`
	Locale struct {
		Storefront string `json:"storefront"`
	} `json:"locale"`
}
type homeMusicArtwork struct {
	Dictionary struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"dictionary"`
}
type homeMusicPageItem struct {
	Title      string              `json:"title"`
	Artist     string              `json:"artistName"`
	TrackCount int                 `json:"trackCount"`
	Descriptor homeMusicDescriptor `json:"contentDescriptor"`
	Artwork    homeMusicArtwork    `json:"artwork"`
	Albums     []struct {
		Title string `json:"title"`
		Segue struct {
			Destination struct {
				Descriptor homeMusicDescriptor `json:"contentDescriptor"`
			} `json:"destination"`
		} `json:"segue"`
	} `json:"tertiaryLinks"`
}
type homeMusicPage struct {
	Data []struct {
		Intent struct {
			Kind       string              `json:"$kind"`
			Descriptor homeMusicDescriptor `json:"contentDescriptor"`
		} `json:"intent"`
		Data struct {
			Canonical string          `json:"canonicalURL"`
			Next      json.RawMessage `json:"nextIntent"`
			Sections  []struct {
				ID    string              `json:"id"`
				Kind  string              `json:"itemKind"`
				Items []homeMusicPageItem `json:"items"`
			} `json:"sections"`
		} `json:"data"`
	} `json:"data"`
}

func homeMusicUnavailable() error {
	return E(502, "HOME_MUSIC_UNAVAILABLE", "未能读取这张公开 Apple Music 歌单；请检查公开状态或稍后重试，已有内容已保留")
}
func homeMusicID(id string) bool {
	if len(id) < 4 || len(id) > 64 || !strings.HasPrefix(id, "pl.") {
		return false
	}
	for _, r := range id[3:] {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

// Only exact Apple content URLs are accepted. Queries are discarded; neither
// links from HTML nor pagination descriptors become arbitrary fetch targets.
func homeMusicParseURL(raw, kind string) (id, country, canonical string, err error) {
	invalid := func() (string, string, string, error) {
		return "", "", "", E(400, "VALIDATION_FAILED", "请输入完整的公开 Apple Music 歌单链接")
	}
	if len(raw) > 2048 || raw != strings.TrimSpace(raw) || strings.ContainsRune(raw, '\\') {
		return invalid()
	}
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host != "music.apple.com" || u.User != nil || u.Port() != "" || u.Fragment != "" || strings.ContainsRune(u.Path, '\\') || strings.ContainsFunc(u.Path, unicode.IsControl) {
		return invalid()
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 3 || len(parts) > 4 || len(parts[0]) != 2 || parts[1] != kind || strings.Contains(u.Path, "//") || (len(parts) == 4 && (parts[2] == "." || parts[2] == "..")) {
		return invalid()
	}
	country = strings.ToLower(parts[0])
	for _, r := range country {
		if r < 'a' || r > 'z' {
			return invalid()
		}
	}
	id = parts[len(parts)-1]
	if kind == "playlist" && !homeMusicID(id) || kind == "album" && !homeDigits(id, 1, 20) || kind != "playlist" && kind != "album" {
		return invalid()
	}
	u.RawQuery = ""
	u.ForceQuery = false
	return id, country, u.String(), nil
}
func homeMusicCountry(raw, country string) string {
	u, _ := url.Parse(raw)
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	parts[0] = country
	u.Path = "/" + strings.Join(parts, "/")
	u.RawPath = ""
	return u.String()
}

// The caller supplies one overall eight-second deadline, including any regional
// retry. Redirects may only add/change the slug of this exact content identity.
func (a *App) homeMusicPageGET(ctx context.Context, target, kind, id, country string) ([]byte, error) {
	gotID, gotCountry, _, e := homeMusicParseURL(target, kind)
	if e != nil || gotID != id || gotCountry != country {
		return nil, homeMusicUnavailable()
	}
	select {
	case homeProviderSlots <- struct{}{}:
		defer func() { <-homeProviderSlots }()
	case <-ctx.Done():
		return nil, homeMusicUnavailable()
	}
	client := http.Client{Transport: homeProviderTransport, Timeout: 8 * time.Second}
	if a.Config.Environment == "test" && a.Config.HTTPClient != nil && a.Config.HTTPClient.Transport != nil {
		client = *a.Config.HTTPClient
		client.Timeout = 8 * time.Second
	}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		redirectID, redirectCountry, _, e := homeMusicParseURL(request.URL.String(), kind)
		if len(via) > 2 || e != nil || redirectID != id || redirectCountry != country || request.URL.RawQuery != "" {
			return http.ErrUseLastResponse
		}
		return nil
	}
	req, e := http.NewRequestWithContext(ctx, "GET", target, nil)
	if e != nil {
		return nil, homeMusicUnavailable()
	}
	req.Header.Set("Accept", "text/html")
	response, e := client.Do(req)
	if e != nil {
		return nil, homeMusicUnavailable()
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, homeMusicUnavailable()
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "" && !strings.HasPrefix(strings.ToLower(contentType), "text/html") {
		return nil, homeMusicUnavailable()
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, homeMusicHTMLLimit+1))
	if e != nil || len(raw) > homeMusicHTMLLimit {
		return nil, homeMusicUnavailable()
	}
	return raw, nil
}
func homeMusicSerialized(raw []byte) ([]byte, error) {
	z := html.NewTokenizer(bytes.NewReader(raw))
	var result []byte
	for {
		token := z.Next()
		if token == html.ErrorToken {
			if z.Err() != io.EOF || result == nil {
				return nil, homeMusicUnavailable()
			}
			return result, nil
		}
		if token != html.StartTagToken {
			continue
		}
		tag := z.Token()
		if tag.Data != "script" {
			continue
		}
		id, contentType := "", ""
		for _, attribute := range tag.Attr {
			if attribute.Key == "id" {
				id = attribute.Val
			}
			if attribute.Key == "type" {
				contentType = attribute.Val
			}
		}
		if id != "serialized-server-data" || contentType != "application/json" {
			continue
		}
		if result != nil || z.Next() != html.TextToken {
			return nil, homeMusicUnavailable()
		}
		result = append([]byte(nil), z.Text()...)
	}
}
func homeMusicText(value string, limit int) string {
	value = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value))
	runes := []rune(value)
	if len(runes) > limit {
		value = string(runes[:limit])
	}
	return value
}

// Apple returns an artwork URL template and maximum dimensions. Expand only
// those explicit placeholders, never rewrite an iTunes artworkUrl100 address.
// https://developer.apple.com/documentation/applemusicapi/artwork
func homeMusicArtworkURL(art homeMusicArtwork) string {
	value := art.Dictionary.URL
	if strings.Count(value, "{w}") == 1 && strings.Count(value, "{h}") == 1 {
		size := min(632, art.Dictionary.Width, art.Dictionary.Height)
		if size < 1 {
			return ""
		}
		value = strings.ReplaceAll(value, "{w}", strconv.Itoa(size))
		value = strings.ReplaceAll(value, "{h}", strconv.Itoa(size))
		value = strings.ReplaceAll(value, "{f}", "jpg")
	}
	if strings.ContainsAny(value, "{}") || !homeRemoteImage("music", value) {
		return ""
	}
	return value
}
func homeMusicParsePage(raw []byte, source, resolved, id, country string) (homeMusicSnapshot, error) {
	var result homeMusicSnapshot
	serialized, e := homeMusicSerialized(raw)
	if e != nil {
		return result, e
	}
	var page homeMusicPage
	if json.Unmarshal(serialized, &page) != nil || len(page.Data) != 1 {
		return result, homeMusicUnavailable()
	}
	data := page.Data[0]
	canonicalID, canonicalCountry, canonical, e := homeMusicParseURL(data.Data.Canonical, "playlist")
	if e != nil || canonicalID != id || canonicalCountry != country || data.Intent.Kind != "PlaylistDetailPageIntent" || data.Intent.Descriptor.Kind != "playlist" || data.Intent.Descriptor.Identifiers.ID != id || data.Intent.Descriptor.Locale.Storefront != country {
		return result, homeMusicUnavailable()
	}
	result = homeMusicSnapshot{SourceURL: source, ResolvedURL: canonical, PlaylistID: id, Items: []homeItemInput{}}
	var tracks []homeMusicPageItem
	var header *homeMusicPageItem
	for _, section := range data.Data.Sections {
		if section.Kind == "containerDetailHeaderLockup" && len(section.Items) == 1 && section.Items[0].Descriptor.Kind == "playlist" && section.Items[0].Descriptor.Identifiers.ID == id {
			if header != nil {
				return result, homeMusicUnavailable()
			}
			value := section.Items[0]
			header = &value
		}
		if section.Kind == "trackLockup" && section.ID == "track-list - "+id {
			if tracks != nil {
				return result, homeMusicUnavailable()
			}
			tracks = section.Items
		}
	}
	if header == nil || strings.TrimSpace(header.Title) == "" || len(tracks) == 0 || len(tracks) > homeMusicTrackLimit {
		return result, homeMusicUnavailable()
	}
	result.Title = homeMusicText(header.Title, 200)
	result.TrackCount = len(tracks)
	result.Partial = len(data.Data.Next) != 0 && string(data.Data.Next) != "null" || header.TrackCount != len(tracks)
	seenTracks, seenAlbums := map[string]bool{}, map[string]bool{}
	for _, track := range tracks {
		trackID := track.Descriptor.Identifiers.ID
		if track.Descriptor.Kind != "song" || !homeDigits(trackID, 1, 20) || seenTracks[trackID] {
			result.Partial = true
			continue
		}
		seenTracks[trackID] = true
		var item homeItemInput
		for _, album := range track.Albums {
			descriptor := album.Segue.Destination.Descriptor
			albumID, albumCountry, albumURL, e := homeMusicParseURL(descriptor.URL, "album")
			if descriptor.Kind != "album" || e != nil || albumID != descriptor.Identifiers.ID || albumCountry != country || strings.TrimSpace(album.Title) == "" {
				continue
			}
			item = homeItemInput{Kind: "music", ProviderID: albumID, Title: homeMusicText(album.Title, 80), Subtitle: homeMusicText(track.Artist, 600), Alt: homeMusicText(album.Title, 200), URL: albumURL, ImageURL: homeMusicArtworkURL(track.Artwork), Position: len(result.Items), Visible: true}
			break
		}
		if item.ProviderID == "" || item.ImageURL == "" || validateHomeItem(item) != nil {
			result.Partial = true
			continue
		}
		if !seenAlbums[item.ProviderID] {
			seenAlbums[item.ProviderID] = true
			result.Items = append(result.Items, item)
		}
	}
	result.AlbumCount = len(result.Items)
	if result.AlbumCount == 0 {
		return result, homeMusicUnavailable()
	}
	return result, nil
}
func (a *App) homeMusicFetch(ctx context.Context, source string) (homeMusicSnapshot, error) {
	id, country, source, e := homeMusicParseURL(source, "playlist")
	if e != nil {
		return homeMusicSnapshot{}, e
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	countries := []string{country}
	if country != "cn" {
		countries = append(countries, "cn")
	}
	for _, candidate := range countries {
		target := homeMusicCountry(source, candidate)
		raw, e := a.homeMusicPageGET(ctx, target, "playlist", id, candidate)
		if e == nil {
			if snapshot, e := homeMusicParsePage(raw, source, target, id, candidate); e == nil {
				return snapshot, nil
			}
		}
	}
	return homeMusicSnapshot{}, homeMusicUnavailable()
}

// Keep the iTunes lookup as the single-album fallback. Only a verified album
// page's own returned artwork template may enhance its small artworkUrl100.
func (a *App) homeMusicEnhanceAlbum(ctx context.Context, item homeItemInput) homeItemInput {
	if !strings.Contains(item.ImageURL, "/image/thumb/Music") {
		return item
	}
	id, country, source, e := homeMusicParseURL(item.URL, "album")
	if e != nil || id != item.ProviderID {
		return item
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	countries := []string{country}
	if country != "cn" {
		countries = append(countries, "cn")
	}
	for _, candidate := range countries {
		raw, e := a.homeMusicPageGET(ctx, homeMusicCountry(source, candidate), "album", id, candidate)
		if e != nil {
			continue
		}
		serialized, e := homeMusicSerialized(raw)
		var page homeMusicPage
		if e != nil || json.Unmarshal(serialized, &page) != nil || len(page.Data) != 1 {
			continue
		}
		data := page.Data[0]
		canonicalID, canonicalCountry, _, e := homeMusicParseURL(data.Data.Canonical, "album")
		if e != nil || canonicalID != id || canonicalCountry != candidate || data.Intent.Kind != "AlbumDetailPageIntent" || data.Intent.Descriptor.Kind != "album" || data.Intent.Descriptor.Identifiers.ID != id || data.Intent.Descriptor.Locale.Storefront != candidate {
			continue
		}
		for _, section := range data.Data.Sections {
			for _, header := range section.Items {
				if section.Kind == "containerDetailHeaderLockup" && header.Descriptor.Kind == "album" && header.Descriptor.Identifiers.ID == id {
					if image := homeMusicArtworkURL(header.Artwork); image != "" {
						item.ImageURL = image
						return item
					}
				}
			}
		}
	}
	return item
}
func homeMusicValidateSnapshot(snapshot homeMusicSnapshot) bool {
	id, _, source, e := homeMusicParseURL(snapshot.SourceURL, "playlist")
	resolvedID, _, _, resolvedError := homeMusicParseURL(snapshot.ResolvedURL, "playlist")
	if e != nil || resolvedError != nil || source != snapshot.SourceURL || id != snapshot.PlaylistID || resolvedID != id || homeText(snapshot.Title, 200, true) != nil || snapshot.TrackCount < 1 || snapshot.TrackCount > homeMusicTrackLimit || snapshot.AlbumCount != len(snapshot.Items) || len(snapshot.Items) < 1 || len(snapshot.Items) > snapshot.TrackCount {
		return false
	}
	seen := map[string]bool{}
	for _, item := range snapshot.Items {
		albumID, _, _, e := homeMusicParseURL(item.URL, "album")
		if item.Kind != "music" || item.MediaID != "" || item.ProviderID != albumID || e != nil || seen[albumID] || validateHomeItem(item) != nil {
			return false
		}
		seen[albumID] = true
	}
	return true
}

type homeMusicStatus struct {
	Version                             int64
	Source, Resolved, PlaylistID, Title string
	Tracks, Albums                      int
	Partial                             bool
	LastSuccess, LastAttempt            *int64
	AttemptSource, LastError            string
}

func homeMusicRead(ctx context.Context, q homeQuery, lock bool) (homeMusicStatus, error) {
	var status homeMusicStatus
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	e := q.QueryRow(ctx, `SELECT version,source_url,resolved_url,playlist_id,title,track_count,album_count,partial,last_success_at_ms,last_attempt_at_ms,last_attempt_source_url,last_error FROM homepage_music_playlist WHERE id=1`+suffix).Scan(&status.Version, &status.Source, &status.Resolved, &status.PlaylistID, &status.Title, &status.Tracks, &status.Albums, &status.Partial, &status.LastSuccess, &status.LastAttempt, &status.AttemptSource, &status.LastError)
	return status, e
}
func homeMusicStatusDTO(status homeMusicStatus, private bool) map[string]any {
	value := map[string]any{"source_url": status.Source, "resolved_url": status.Resolved, "playlist_id": status.PlaylistID, "title": status.Title, "track_count": status.Tracks, "album_count": status.Albums, "partial": status.Partial, "page_count": 1, "read_limit": homeMusicTrackLimit, "last_success_at": nil}
	if status.LastSuccess != nil {
		value["last_success_at"] = TimeString(*status.LastSuccess)
	}
	if private {
		value["version"], value["last_error"], value["last_attempt_source_url"], value["last_attempt_at"] = status.Version, status.LastError, status.AttemptSource, nil
		if status.LastAttempt != nil {
			value["last_attempt_at"] = TimeString(*status.LastAttempt)
		}
	}
	return value
}
func (a *App) PublicMusicPlaylist(ctx context.Context) (map[string]any, error) {
	status, e := homeMusicRead(ctx, a.Pool, false)
	if e != nil || status.LastSuccess == nil {
		return nil, e
	}
	return homeMusicStatusDTO(status, false), nil
}
func (a *App) homeMusicGet(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	status, e := homeMusicRead(r.Context(), a.Pool, false)
	if e == nil {
		JSON(w, 200, homeMusicStatusDTO(status, true))
	}
	return e
}
func (a *App) homeMusicFailure(ctx context.Context, token string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	_ = a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE homepage_music_playlist SET last_error='公开歌单读取未完成；已有内容和上次成功快照已保留，请检查公开状态或稍后重试',attempt_token=NULL WHERE id=1 AND attempt_token=$1`, token)
		return e
	})
}
func (a *App) homeMusicPreview(w http.ResponseWriter, r *http.Request) error {
	if e := homeAuthorize(a, r); e != nil {
		return e
	}
	var input struct {
		Input   string `json:"input"`
		Refresh bool   `json:"refresh"`
	}
	if e := decodeHome(w, r, &input); e != nil {
		return e
	}
	id, country, source, e := homeMusicParseURL(input.Input, "playlist")
	if e != nil {
		return e
	}
	if replay, found, e := a.Replay(r, "owner"); e != nil {
		return e
	} else if found {
		JSON(w, 200, replay)
		return nil
	}
	owner, e := a.Require(r, "owner")
	if e != nil {
		return e
	}
	var snapshot homeMusicSnapshot
	var cachedRaw []byte
	cached := false
	if !input.Refresh {
		e = a.Pool.QueryRow(r.Context(), `SELECT metadata FROM homepage_import_cache WHERE kind='music' AND provider_id=$1 AND country=$2 AND expires_at_ms>$3`, "playlist:"+id, country, NowMillis()).Scan(&cachedRaw)
		if e == nil {
			if json.Unmarshal(cachedRaw, &snapshot) != nil || !homeMusicValidateSnapshot(snapshot) || snapshot.PlaylistID != id {
				return homeMusicUnavailable()
			}
			// The same public ID may be shared under a different slug.
			snapshot.SourceURL = source
			cached = true
		} else if e != pgx.ErrNoRows {
			return e
		}
	}
	token := RandID()
	_, e = a.Write(r, []string{"owner"}, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		if _, e := homeRead(r.Context(), tx, true); e != nil {
			return nil, e
		}
		if !cached {
			if e := homeRateTx(r.Context(), tx, p.ID, "music"); e != nil {
				return nil, e
			}
		}
		_, e := tx.Exec(r.Context(), `UPDATE homepage_music_playlist SET last_attempt_at_ms=$1,last_attempt_source_url=$2,attempt_token=$3 WHERE id=1`, NowMillis(), source, token)
		return nil, e
	})
	if e != nil {
		return e
	}
	if !cached {
		snapshot, e = a.homeMusicFetch(r.Context(), source)
		if e != nil {
			a.homeMusicFailure(r.Context(), token)
			return e
		}
	}
	value, e := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		if _, e := homeRead(r.Context(), tx, true); e != nil {
			return nil, e
		}
		if p.ID != owner.ID {
			return nil, homeMusicUnavailable()
		}
		now := NowMillis()
		raw, _ := json.Marshal(snapshot)
		if !cached {
			if _, e := tx.Exec(r.Context(), `DELETE FROM homepage_import_cache WHERE expires_at_ms<=$1`, now); e != nil {
				return nil, e
			}
			// Playlist projections are much larger than a single album's metadata.
			if _, e := tx.Exec(r.Context(), `DELETE FROM homepage_import_cache WHERE kind='music' AND (provider_id,country) IN(SELECT provider_id,country FROM homepage_import_cache WHERE kind='music' AND provider_id LIKE 'playlist:%' ORDER BY expires_at_ms DESC,provider_id,country OFFSET 31)`); e != nil {
				return nil, e
			}
			if _, e := tx.Exec(r.Context(), `DELETE FROM homepage_import_cache WHERE (kind,provider_id,country) IN(SELECT kind,provider_id,country FROM homepage_import_cache ORDER BY expires_at_ms DESC,kind,provider_id,country OFFSET 4999)`); e != nil {
				return nil, e
			}
			if _, e := tx.Exec(r.Context(), `INSERT INTO homepage_import_cache(kind,provider_id,country,metadata,expires_at_ms) VALUES('music',$1,$2,$3,$4) ON CONFLICT(kind,provider_id,country) DO UPDATE SET metadata=EXCLUDED.metadata,expires_at_ms=EXCLUDED.expires_at_ms`, "playlist:"+id, country, raw, now+24*time.Hour.Milliseconds()); e != nil {
				return nil, e
			}
		}
		if _, e := tx.Exec(r.Context(), `DELETE FROM homepage_music_previews WHERE expires_at_ms<=$1 OR id IN(SELECT id FROM homepage_music_previews WHERE owner_id=$2 ORDER BY created_at_ms DESC,id OFFSET 31) OR id IN(SELECT id FROM homepage_music_previews ORDER BY created_at_ms DESC,id OFFSET 127)`, now, p.ID); e != nil {
			return nil, e
		}
		previewID, expires := RandID(), now+30*time.Minute.Milliseconds()
		if _, e := tx.Exec(r.Context(), `INSERT INTO homepage_music_previews(id,owner_id,snapshot,created_at_ms,expires_at_ms) VALUES($1,$2,$3,$4,$5)`, previewID, p.ID, raw, now, expires); e != nil {
			return nil, e
		}
		if _, e := tx.Exec(r.Context(), `UPDATE homepage_music_playlist SET last_error='',attempt_token=NULL WHERE id=1 AND attempt_token=$1`, token); e != nil {
			return nil, e
		}
		items := make([]map[string]any, 0, min(24, len(snapshot.Items)))
		for _, item := range snapshot.Items[:min(24, len(snapshot.Items))] {
			items = append(items, homeMetadata(item))
		}
		notice := "已读取公开页面；确认后才会保存到本地唱片库。"
		if snapshot.Partial {
			notice = "仅读取公开页面当前前300首以内的歌曲，未确认完整歌单；按专辑去重后导入，已有条目会保留。"
		}
		return map[string]any{"preview_id": previewID, "source_url": snapshot.SourceURL, "resolved_url": snapshot.ResolvedURL, "playlist_id": snapshot.PlaylistID, "title": snapshot.Title, "track_count": snapshot.TrackCount, "album_count": snapshot.AlbumCount, "partial": snapshot.Partial, "page_count": 1, "read_limit": homeMusicTrackLimit, "items": items, "preview_count": len(items), "expires_at": TimeString(expires), "notice": notice}, nil
	})
	if e != nil {
		a.homeMusicFailure(r.Context(), token)
		return e
	}
	JSON(w, 200, value)
	return nil
}
func (a *App) homeMusicImport(w http.ResponseWriter, r *http.Request) error {
	if e := homeAuthorize(a, r); e != nil {
		return e
	}
	version, e := IfMatch(r)
	if e != nil {
		return e
	}
	var input struct {
		PreviewID string `json:"preview_id"`
	}
	if e := decodeHome(w, r, &input); e != nil {
		return e
	}
	if len(input.PreviewID) != 48 || strings.ContainsFunc(input.PreviewID, func(r rune) bool { return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') }) {
		return E(400, "VALIDATION_FAILED", "需要有效的歌单预览")
	}
	value, e := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		// Same order as all home mutations: settings -> source -> items -> media.
		if _, e := homeRead(r.Context(), tx, true); e != nil {
			return nil, e
		}
		status, e := homeMusicRead(r.Context(), tx, true)
		if e != nil {
			return nil, e
		}
		if status.Version != version {
			return nil, homeVersionError()
		}
		var raw []byte
		if e := tx.QueryRow(r.Context(), `SELECT snapshot FROM homepage_music_previews WHERE id=$1 AND owner_id=$2 AND expires_at_ms>$3`, input.PreviewID, p.ID, NowMillis()).Scan(&raw); e == pgx.ErrNoRows {
			return nil, E(409, "HOME_MUSIC_PREVIEW_EXPIRED", "歌单预览已过期或不可用，请重新读取；已有条目已保留")
		} else if e != nil {
			return nil, e
		}
		var snapshot homeMusicSnapshot
		if json.Unmarshal(raw, &snapshot) != nil || !homeMusicValidateSnapshot(snapshot) {
			return nil, homeMusicUnavailable()
		}
		if _, e := tx.Exec(r.Context(), `SELECT id FROM homepage_items WHERE kind='music' ORDER BY id FOR UPDATE`); e != nil {
			return nil, e
		}
		if _, e := tx.Exec(r.Context(), `SELECT m.id FROM media_assets m WHERE EXISTS(SELECT 1 FROM homepage_items h WHERE h.media_id=m.id AND h.kind='music') ORDER BY m.id FOR UPDATE`); e != nil {
			return nil, e
		}
		var total, maxPosition int
		if e := tx.QueryRow(r.Context(), `SELECT count(*),COALESCE(max(position),-1) FROM homepage_items`).Scan(&total, &maxPosition); e != nil {
			return nil, e
		}
		newCount, preserved := 0, 0
		now := NowMillis()
		for _, item := range snapshot.Items {
			var manual bool
			if e := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM homepage_items WHERE kind='music' AND source='manual' AND provider_id=$1)`, item.ProviderID).Scan(&manual); e != nil {
				return nil, e
			}
			if manual {
				preserved++
				continue
			}
			var existing string
			var overridden bool
			e := tx.QueryRow(r.Context(), `SELECT id::text,(local_override OR media_id IS NOT NULL) FROM homepage_items WHERE kind='music' AND source='music_playlist' AND provider_id=$1`, item.ProviderID).Scan(&existing, &overridden)
			if e != nil && e != pgx.ErrNoRows {
				return nil, e
			}
			if e == pgx.ErrNoRows {
				if total+newCount >= homeItemLimit {
					return nil, E(409, "HOME_ITEM_LIMIT", "导入后将超过首页本地库5000项限制，已有内容未修改")
				}
				maxPosition = min(maxPosition+1, 1000000)
				_, e = tx.Exec(r.Context(), `INSERT INTO homepage_items(kind,title,subtitle,alt,url,image_url,provider_id,source,position,visible,created_at_ms,updated_at_ms) VALUES('music',$1,$2,$3,$4,$5,$6,'music_playlist',$7,true,$8,$8)`, item.Title, item.Subtitle, item.Alt, item.URL, item.ImageURL, item.ProviderID, maxPosition, now)
				newCount++
			} else if overridden {
				preserved++
			} else {
				// Position, visibility, alt and local media are never overwritten.
				_, e = tx.Exec(r.Context(), `UPDATE homepage_items SET title=$2,subtitle=$3,url=$4,image_url=$5,version=version+1,updated_at_ms=$6 WHERE id=$1`, existing, item.Title, item.Subtitle, item.URL, item.ImageURL, now)
			}
			if e != nil {
				return nil, e
			}
		}
		_, e = tx.Exec(r.Context(), `UPDATE homepage_music_playlist SET version=version+1,source_url=$1,resolved_url=$2,playlist_id=$3,title=$4,track_count=$5,album_count=$6,partial=$7,last_success_at_ms=$8,last_attempt_at_ms=$8,last_attempt_source_url=$1,last_error='',attempt_token=NULL WHERE id=1`, snapshot.SourceURL, snapshot.ResolvedURL, snapshot.PlaylistID, snapshot.Title, snapshot.TrackCount, snapshot.AlbumCount, snapshot.Partial, now)
		if e != nil {
			return nil, e
		}
		if e := audit(r.Context(), tx, p, "home.music_import", "site", "homepage", ""); e != nil {
			return nil, e
		}
		status, e = homeMusicRead(r.Context(), tx, false)
		if e != nil {
			return nil, e
		}
		result := homeMusicStatusDTO(status, true)
		result["imported_count"], result["new_count"], result["preserved_count"] = len(snapshot.Items)-preserved, newCount, preserved
		return result, nil
	})
	if e == nil {
		JSON(w, 200, value)
	}
	return e
}
