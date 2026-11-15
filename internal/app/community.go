package app

import (
	"context"
	"net/url"
)

// communityPage discovers current public content across the site. Personal
// showcase/provider settings never enter this aggregation.
func (a *App) communityPage(ctx context.Context, query url.Values) (webPage, error) {
	// A discovery continuation can never become a private or author-filtered
	// feed through extra URL parameters; its public list size is fixed.
	q := url.Values{"limit": {"12"}}
	if cursor := query.Get("cursor"); cursor != "" {
		q.Set("cursor", cursor)
	}
	posts, err := a.PublicPosts(ctx, q)
	if err != nil {
		return webPage{}, err
	}
	series, err := a.PublicSeries(ctx, url.Values{"limit": {"3"}})
	if err != nil {
		return webPage{}, err
	}
	authors, err := a.communityAuthors(ctx)
	if err != nil {
		return webPage{}, err
	}
	p := webPage{Kind: "discover", Title: "站内", Items: webMaps(posts["items"]), Series: webMaps(series["items"]), Authors: authors}
	if cursor := webString(posts["next_cursor"]); cursor != "" {
		next := url.Values{"cursor": {cursor}}
		if locale := canonicalLocale(query.Get("lang")); locale != "" {
			next.Set("lang", locale)
		}
		p.Next = "/discover?" + next.Encode()
	}
	return p, nil
}

// Public creator discovery is based only on currently public objects, not
// membership, draft edits, follower counts or invented popularity. A series has
// no separate publication timestamp: its current public metadata update is an
// activity, so this field deliberately says activity rather than publication.
func (a *App) communityAuthors(ctx context.Context) ([]map[string]any, error) {
	return contentObjects(ctx, a.Pool, `WITH public_activity AS (
 SELECT author_user_id,max(published_at_ms) AS stamp FROM public_posts GROUP BY author_user_id
 UNION ALL
 SELECT author_user_id,max(published_at_ms) FROM public_moments GROUP BY author_user_id
 UNION ALL
 SELECT author_user_id,max(updated_at_ms) FROM series WHERE status='published' AND deleted_at_ms IS NULL GROUP BY author_user_id
), latest AS (
 SELECT author_user_id,max(stamp) AS stamp FROM public_activity GROUP BY author_user_id
)
 SELECT jsonb_build_object('id',u.id::text,'username',u.username,'display_name',u.display_name,'bio',u.bio,
 'avatar_url',CASE WHEN u.avatar_media_id IS NULL THEN u.avatar_url ELSE '/media/'||u.avatar_media_id END,
 'url','/users/'||u.id,'last_public_activity_at',`+contentTimeSQL("latest.stamp")+`)
 FROM latest JOIN users u ON u.id=latest.author_user_id
 WHERE u.disabled_at_ms IS NULL AND u.role IN ('owner','author')
 ORDER BY latest.stamp DESC,u.id DESC LIMIT 8`)
}

// authorContentQuery is shared by data reads, canonical URLs and continuation
// links so ignored parameters never masquerade as active profile filters.
func authorContentQuery(tab string, query url.Values) (url.Values, error) {
	if tab == "" {
		tab = "posts"
	}
	keys := []string{"limit", "cursor"}
	switch tab {
	case "posts":
		keys = append(keys, "q", "sort")
	case "moments":
		// Moments have no text search/sort controls. Drop stray article filters.
	case "series":
		keys = append(keys, "q", "state")
	default:
		return nil, E(400, "VALIDATION_FAILED", "请选择文章、动态或专栏")
	}
	q := make(url.Values, len(keys)+2)
	for _, key := range keys {
		if value := query.Get(key); value != "" {
			q.Set(key, value)
		}
	}
	q.Set("tab", tab)
	if locale := canonicalLocale(query.Get("lang")); locale != "" {
		q.Set("lang", locale)
	}
	return q, nil
}

// AuthorContent accepts only filters supported by the chosen public tab. The
// route identity wins over any supplied author_id. Native domain cursors bind
// the resulting author/filter/limit and cannot cross authors or content tabs.
// Profile existence/visibility is checked by the calling public profile route.
func (a *App) AuthorContent(ctx context.Context, id, tab string, query url.Values) (map[string]any, error) {
	if !positiveID(id) {
		return nil, NotFound()
	}
	q, err := authorContentQuery(tab, query)
	if err != nil {
		return nil, err
	}
	q.Set("author_id", id)
	switch q.Get("tab") {
	case "posts":
		return a.AuthorPosts(ctx, id, q)
	case "moments":
		return a.PublicMoments(ctx, q)
	default:
		return a.PublicSeries(ctx, q)
	}
}
