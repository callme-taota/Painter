package app

import "context"

// RelatedPosts uses only current public revision tags. Private edits cannot
// change recommendations until publication; a withdrawn source yields no list.
// Shared tags rank first, then recent publications fill the remaining places.
func (a *App) RelatedPosts(ctx context.Context, id string) ([]map[string]any, error) {
	if contentID(id) != nil {
		return []map[string]any{}, nil
	}
	return contentObjects(ctx, a.Pool, `WITH source AS (
	 SELECT revision_id FROM public_posts WHERE post_id=$1
	), source_tags AS (
	 SELECT rt.tag_id FROM source s JOIN effective_revision_tags rt ON rt.revision_id=s.revision_id
	), ranked AS (
	 SELECT p.post_id,p.published_at_ms,
	  (SELECT count(*) FROM effective_revision_tags rt JOIN source_tags st ON st.tag_id=rt.tag_id
	   WHERE rt.revision_id=p.revision_id) AS shared_tags
	 FROM public_posts p WHERE p.post_id<>$1 AND EXISTS(SELECT 1 FROM source)
	 ORDER BY shared_tags DESC,p.published_at_ms DESC,p.post_id DESC LIMIT 3
	) SELECT `+publicPostJSON+publicPostFrom+` JOIN ranked ON ranked.post_id=v.post_id
	 ORDER BY ranked.shared_tags DESC,ranked.published_at_ms DESC,ranked.post_id DESC`, id)
}
