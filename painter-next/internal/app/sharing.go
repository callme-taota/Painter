package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"painter-next/web"
)

// Shares identify live public resources, never a revision or permission grant.
// The signature prevents arbitrary redirects; public visibility is checked on
// every request. Rotating EncryptionKey intentionally invalidates old links.
type shareReference struct{ Kind, ID string }
type shareDocument struct {
	Kind, ID, Title, Summary, Author, CoverURL, SiteName, Canonical, Favicon string
	CSS                                                                      []string
	Embed                                                                    bool
}
type shareSeriesResolver func(context.Context, string) (map[string]any, error)

var sharingTemplates struct {
	sync.Once
	template *template.Template
}

func shareKindCode(kind string) string {
	return map[string]string{"post": "p", "moment": "m", "phrase": "q", "series": "c"}[kind]
}

func (a *App) shareSignature(payload string) string {
	h := hmac.New(sha256.New, a.Config.EncryptionKey)
	_, _ = h.Write([]byte("painter:public-share:v1:" + payload))
	// 96 bits of a keyed digest keep a short link compact without making it a
	// bearer capability. It contains no user input, destination or private data.
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil)[:12])
}

func (a *App) shareToken(ref shareReference) string {
	id, err := strconv.ParseInt(ref.ID, 10, 64)
	code := shareKindCode(ref.Kind)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != ref.ID || code == "" {
		return ""
	}
	payload := "1" + code + strconv.FormatInt(id, 36)
	return payload + "." + a.shareSignature(payload)
}

func (a *App) parseShareToken(token string) (shareReference, error) {
	ref := shareReference{}
	if len(token) < 20 || len(token) > 34 {
		return ref, NotFound()
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(parts[0]) < 3 || parts[0][0] != '1' || !hmac.Equal([]byte(parts[1]), []byte(a.shareSignature(parts[0]))) {
		return ref, NotFound()
	}
	for _, kind := range []string{"post", "moment", "phrase", "series"} {
		if shareKindCode(kind) == parts[0][1:2] {
			ref.Kind = kind
		}
	}
	id, err := strconv.ParseInt(parts[0][2:], 36, 64)
	if ref.Kind == "" || err != nil || id < 1 || strconv.FormatInt(id, 36) != parts[0][2:] {
		return shareReference{}, NotFound()
	}
	ref.ID = strconv.FormatInt(id, 10)
	return ref, nil
}

func shareExcerpt(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return text
}

// Metadata-only projections avoid fetching/rendering article bodies, generating
// read receipts or expanding card pools just to open a share menu.
func (a *App) publicShareDocument(ctx context.Context, ref shareReference, resolveSeries shareSeriesResolver) (shareDocument, error) {
	doc := shareDocument{Kind: ref.Kind, ID: ref.ID, Favicon: "/assets/" + siteFaviconPath}
	if !contentCardReferenceID(ref.ID) || shareKindCode(ref.Kind) == "" {
		return doc, NotFound()
	}
	var value map[string]any
	var err error
	switch ref.Kind {
	case "post":
		value, err = contentObject(ctx, a.Pool, `SELECT jsonb_build_object('title',p.title,'summary',p.summary,'author',u.display_name,'site_name',s.display_name,
 'cover_url',CASE WHEN r.cover->>'mode'='none' THEN '' ELSE '/api/v1/posts/'||p.post_id||'/cover?revision='||p.revision_id END)
 FROM public_posts p JOIN users u ON u.id=p.author_user_id JOIN post_revisions r ON r.id=p.revision_id CROSS JOIN site_profile s WHERE s.id=1 AND p.post_id=$1`, ref.ID)
	case "moment":
		value, err = contentObject(ctx, a.Pool, `SELECT jsonb_build_object('title','动态','summary',m.text,'author',u.display_name,'site_name',s.display_name,
 'cover_url',COALESCE((SELECT '/media/'||i.media_id FROM moment_images i WHERE i.moment_id=m.id AND i.scope='public' ORDER BY i.position LIMIT 1),''))
 FROM public_moments m JOIN users u ON u.id=m.author_user_id CROSS JOIN site_profile s WHERE s.id=1 AND m.id=$1`, ref.ID)
	case "phrase":
		value, err = contentObject(ctx, a.Pool, `SELECT jsonb_build_object('title',CASE WHEN p.kind='word' THEN '词语' ELSE '片语' END,'summary',p.text,'author',COALESCE(NULLIF(p.source,''),s.display_name),'site_name',s.display_name)
 FROM phrases p CROSS JOIN site_profile s WHERE s.id=1 AND p.published=true AND p.id=$1`, ref.ID)
	case "series":
		if resolveSeries == nil {
			return doc, NotFound()
		}
		value, err = resolveSeries(ctx, ref.ID)
	}
	if err != nil {
		return doc, err
	}
	if value == nil {
		return doc, NotFound()
	}
	doc.Title = shareExcerpt(webString(value["title"]), 120)
	doc.Summary = webString(value["summary"])
	if doc.Summary == "" {
		doc.Summary = webString(value["description"])
	}
	doc.Summary = shareExcerpt(doc.Summary, 180)
	doc.Author = shareExcerpt(webString(value["author"]), 80)
	if author, ok := value["author"].(map[string]any); ok {
		doc.Author = shareExcerpt(webString(author["display_name"]), 80)
	}
	doc.SiteName = shareExcerpt(webString(value["site_name"]), 80)
	if doc.SiteName == "" {
		if err = a.Pool.QueryRow(ctx, "SELECT display_name FROM site_profile WHERE id=1").Scan(&doc.SiteName); err != nil {
			return doc, err
		}
	}
	cover := webString(value["cover_url"])
	// All share imagery is already-public site media or a public cover route.
	// Never allow a stored URL to become a script URL or third-party tracker.
	if strings.HasPrefix(cover, "/media/") || (ref.Kind == "post" && strings.HasPrefix(cover, "/api/v1/posts/")) {
		doc.CoverURL = strings.TrimRight(a.Config.PublicBaseURL, "/") + cover
	}
	paths := map[string]string{"post": "/posts/", "moment": "/moments/", "phrase": "/phrases/", "series": "/series/"}
	doc.Canonical = strings.TrimRight(a.Config.PublicBaseURL, "/") + paths[ref.Kind] + ref.ID
	return doc, nil
}

func shareEmbedHTML(doc shareDocument, embedURL string) string {
	title := html.EscapeString(doc.Title + " · " + doc.SiteName)
	return `<iframe src="` + html.EscapeString(embedURL) + `" title="` + title + `" width="640" height="360" style="display:block;width:100%;max-width:640px;border:0;border-radius:12px" loading="lazy" sandbox="allow-popups allow-popups-to-escape-sandbox" referrerpolicy="no-referrer"></iframe>` + "\n" + `<p><a href="` + html.EscapeString(doc.Canonical) + `" target="_blank" rel="noopener noreferrer">` + title + `</a></p>`
}

// RegisterSharing accepts the series module's public-only metadata reader.
// No database rows or background jobs are created by sharing.
func (a *App) RegisterSharing(series ...func(context.Context, string) (map[string]any, error)) {
	var resolveSeries shareSeriesResolver
	if len(series) > 0 {
		resolveSeries = series[0]
	}
	sharingTemplates.Do(func() {
		sharingTemplates.template = template.Must(template.New("sharing").ParseFS(web.FS, "templates/sharing.html"))
	})
	a.Handle("GET /api/v1/share", func(w http.ResponseWriter, r *http.Request) error {
		ref := shareReference{Kind: r.URL.Query().Get("type"), ID: r.URL.Query().Get("id")}
		if shareKindCode(ref.Kind) == "" {
			return E(400, "VALIDATION_FAILED", "请选择文章、动态、片语或专栏")
		}
		doc, err := a.publicShareDocument(r.Context(), ref, resolveSeries)
		if err != nil {
			return err
		}
		base := strings.TrimRight(a.Config.PublicBaseURL, "/")
		token := a.shareToken(ref)
		embedURL := base + "/embed/" + token
		w.Header().Set("Cache-Control", "private, no-store")
		JSON(w, 200, map[string]any{"type": ref.Kind, "id": ref.ID, "title": doc.Title, "short_url": base + "/s/" + token, "share_url": base + "/share/" + token, "embed_url": embedURL, "canonical_url": doc.Canonical, "embed_html": shareEmbedHTML(doc, embedURL)})
		return nil
	})
	a.Handle("GET /s/{token}", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "private, no-store")
		ref, err := a.parseShareToken(r.PathValue("token"))
		if err != nil {
			return err
		}
		doc, err := a.publicShareDocument(r.Context(), ref, resolveSeries)
		if err != nil {
			return err
		}
		http.Redirect(w, r, doc.Canonical, http.StatusFound)
		return nil
	})
	for _, mode := range []string{"share", "embed"} {
		a.Handle("GET /"+mode+"/{token}", func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("Cache-Control", "private, no-store")
			ref, err := a.parseShareToken(r.PathValue("token"))
			if err != nil {
				return err
			}
			doc, err := a.publicShareDocument(r.Context(), ref, resolveSeries)
			if err != nil {
				return err
			}
			doc.Embed = mode == "embed"
			// The preview has no JavaScript and needs only its small standalone
			// stylesheet. Keep source-only/test builds usable before assets exist.
			if stylesheet, _ := web.Assets("src/sharing.css"); strings.HasSuffix(stylesheet, ".css") {
				doc.CSS = []string{stylesheet}
			} else {
				_, doc.CSS = web.Assets("src/public.ts")
			}
			var buffer bytes.Buffer
			if err = sharingTemplates.template.ExecuteTemplate(&buffer, "sharing", doc); err != nil {
				return fmt.Errorf("render share document: %w", err)
			}
			if doc.Embed {
				// Only this deliberately script-free document can be framed. No
				// authentication, forms, cookies or parent-window access is used.
				w.Header().Del("X-Frame-Options")
				w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' https: data:; style-src 'self'; script-src 'none'; base-uri 'none'; object-src 'none'; form-action 'none'; frame-ancestors *; sandbox allow-popups allow-popups-to-escape-sandbox")
			}
			w.Header().Set("X-Robots-Tag", "noindex, follow")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Referrer-Policy", "no-referrer")
			_, _ = w.Write(buffer.Bytes())
			return nil
		})
	}
}
