package app

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RSS reads the same immutable publication projection as the public archive.
// Revalidate the current switch and content before honoring a reader's ETag;
// a previously cached response must not keep withdrawn content available here.
func (a *App) rssFeed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var enabled bool
	var title, tagline, seo string
	if err := a.Pool.QueryRow(r.Context(), `SELECT s.rss_enabled,p.display_name,p.tagline,p.seo_description
		FROM site_settings s JOIN site_profile p ON p.id=s.id WHERE s.id=1`).Scan(&enabled, &title, &tagline, &seo); err != nil {
		rssFailure(w, r, http.StatusServiceUnavailable)
		return
	}
	if !enabled {
		rssFailure(w, r, http.StatusNotFound)
		return
	}
	data, err := a.PublicPosts(r.Context(), url.Values{"limit": {"20"}})
	if err != nil {
		rssFailure(w, r, http.StatusServiceUnavailable)
		return
	}
	type item struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		Description string `xml:"description"`
		GUID        string `xml:"guid"`
		Date        string `xml:"pubDate"`
	}
	type selfLink struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
		Type string `xml:"type,attr"`
	}
	type channel struct {
		Title       string   `xml:"title"`
		Link        string   `xml:"link"`
		Description string   `xml:"description"`
		Self        selfLink `xml:"atom:link"`
		Items       []item   `xml:"item"`
	}
	description := strings.TrimSpace(tagline)
	if description == "" {
		description = strings.TrimSpace(seo)
	}
	if description == "" {
		description = title
	}
	feed := struct {
		XMLName xml.Name `xml:"rss"`
		Version string   `xml:"version,attr"`
		AtomNS  string   `xml:"xmlns:atom,attr"`
		Channel channel  `xml:"channel"`
	}{Version: "2.0", AtomNS: "http://www.w3.org/2005/Atom", Channel: channel{
		Title: title, Link: a.Config.PublicBaseURL, Description: description,
		Self: selfLink{Href: a.Config.PublicBaseURL + "/rss.xml", Rel: "self", Type: "application/rss+xml"},
	}}
	for _, post := range webMaps(data["items"]) {
		link := a.Config.PublicBaseURL + "/posts/" + webString(post["id"])
		stamp, err := time.Parse(time.RFC3339Nano, webString(post["published_at"]))
		if err != nil {
			rssFailure(w, r, http.StatusServiceUnavailable)
			return
		}
		feed.Channel.Items = append(feed.Channel.Items, item{Title: webString(post["title"]), Link: link, GUID: link,
			Description: webString(post["summary"]), Date: stamp.UTC().Format(time.RFC1123Z)})
	}
	b, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		rssFailure(w, r, http.StatusServiceUnavailable)
		return
	}
	body := append([]byte(xml.Header), b...)
	etag := `"` + hashString(string(body)) + `"`
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if rssETagMatches(strings.Join(r.Header.Values("If-None-Match"), ","), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(body)
}

func rssFailure(w http.ResponseWriter, r *http.Request, status int) {
	message := "Feed unavailable\n"
	if status == http.StatusNotFound {
		message = "404 page not found\n"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(message)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(message))
	}
}

// If-None-Match uses weak comparison for GET/HEAD. Parse quoted opaque tags
// rather than splitting on commas, which can legally appear inside a tag.
func rssETagMatches(raw, current string) bool {
	raw = strings.Trim(raw, " \t")
	if raw == "*" {
		return true
	}
	matched := false
	for raw != "" {
		if strings.HasPrefix(raw, "W/") {
			raw = raw[2:]
		}
		if len(raw) < 2 || raw[0] != '"' {
			return false
		}
		end := strings.IndexByte(raw[1:], '"') + 1
		if end == 0 {
			return false
		}
		for _, b := range []byte(raw[1:end]) {
			if b < 0x21 || b == 0x7f {
				return false
			}
		}
		matched = matched || raw[:end+1] == current
		raw = strings.Trim(raw[end+1:], " \t")
		if raw == "" {
			return matched
		}
		if raw[0] != ',' {
			return false
		}
		raw = strings.Trim(raw[1:], " \t")
		if raw == "" {
			return false
		}
	}
	return false
}
