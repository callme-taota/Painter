package app

// Unlike external snapshots, internal cards contain identity only. Their text
// and photos are projected from the current public source at request time.
import (
	"context"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"net/http"
	"strconv"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func contentCardIsInternal(kind string) bool { return kind == "moment" || kind == "phrase" }
func contentCardReferenceID(id string) bool {
	n, e := strconv.ParseInt(id, 10, 64)
	return e == nil && n > 0 && strconv.FormatInt(n, 10) == id
}
func contentCardInternalURL(kind, id string) string {
	if kind == "moment" {
		return "/moments/" + id
	}
	return "/phrases/" + id
}
func contentCardInputReference(in contentCardResolveInput) (string, error) {
	id := in.RefID
	if id == "" {
		prefix := "/phrases/"
		if in.Type == "moment" {
			prefix = "/moments/"
		}
		if strings.HasPrefix(in.URL, prefix) {
			id = strings.TrimPrefix(in.URL, prefix)
		}
	}
	if !contentCardReferenceID(id) || (in.URL != "" && in.URL != contentCardInternalURL(in.Type, id)) || in.Title != "" || in.Description != "" {
		return "", contentCardInvalid("请选择已公开的站内内容，不能附带快照或外部链接")
	}
	return id, nil
}
func contentCardInternalPlaceholder(c contentCard) string {
	label := contentCardRegistry[c.Type].Label
	return `<a class="content-card content-card--` + c.Type + `" href="` + c.URL + `"><span class="content-card__body">` + contentCardSpan("content-card__title", label+"引用") + contentCardSpan("content-card__description", "读取时展示当前公开内容") + `</span></a>` + "\n"
}

// One bounded statement gives moments, phrases and their media one snapshot.
// Do not read moments.draft_text or include private phrase rows here.
func contentCardLookup(ctx context.Context, db contentQuerier, cards []contentCard) (map[string]map[string]any, error) {
	moments, phrases := []string{}, []string{}
	seen := map[string]bool{}
	for _, c := range cards {
		key := c.Type + ":" + c.RefID
		if seen[key] {
			continue
		}
		seen[key] = true
		if c.Type == "moment" {
			moments = append(moments, c.RefID)
		} else if c.Type == "phrase" {
			phrases = append(phrases, c.RefID)
		}
	}
	out := map[string]map[string]any{}
	if len(moments)+len(phrases) == 0 {
		return out, nil
	}
	rows, e := db.Query(ctx, `SELECT 'moment',m.id::text,`+publicMomentJSON+publicMomentFrom+` WHERE m.id=ANY($1::bigint[])
 UNION ALL SELECT 'phrase',p.id::text,jsonb_build_object('id',p.id::text,'text',p.text,'kind',p.kind,'source',p.source) FROM phrases p WHERE p.published=true AND p.id=ANY($2::bigint[])`, moments, phrases)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var kind, id string
		var raw []byte
		if e = rows.Scan(&kind, &id, &raw); e != nil {
			return nil, e
		}
		var value map[string]any
		if e = json.Unmarshal(raw, &value); e != nil {
			return nil, e
		}
		value["type"] = kind
		value["url"] = contentCardInternalURL(kind, id)
		out[kind+":"+id] = value
	}
	return out, rows.Err()
}

// PublicContentCard is shared by SSR and the editor's safe same-origin reader.
func (a *App) PublicContentCard(ctx context.Context, kind, id string) (map[string]any, error) {
	if !contentCardIsInternal(kind) || !contentCardReferenceID(id) {
		return nil, NotFound()
	}
	values, e := contentCardLookup(ctx, a.Pool, []contentCard{{Type: kind, RefID: id}})
	if e != nil {
		return nil, e
	}
	v, ok := values[kind+":"+id]
	if !ok {
		return nil, NotFound()
	}
	return v, nil
}
func (a *App) publicContentCard(w http.ResponseWriter, r *http.Request) error {
	kind, id := r.PathValue("kind"), r.PathValue("id")
	value, e := a.PublicContentCard(r.Context(), kind, id)
	if e != nil {
		return e
	}
	JSON(w, 200, map[string]any{"card": contentCard{V: 1, Type: kind, RefID: id, URL: contentCardInternalURL(kind, id)}, "reference": value})
	return nil
}

func contentCardString(value any) string      { s, _ := value.(string); return s }
func contentCardMap(value any) map[string]any { m, _ := value.(map[string]any); return m }
func contentCardInternalHTML(c contentCard, reference map[string]any) string {
	label := contentCardRegistry[c.Type].Label
	if reference == nil {
		return `<span class="content-card content-card--` + c.Type + ` content-card--unavailable" role="note"><span class="content-card__body">` + contentCardSpan("content-card__title", "此"+label+"暂不可用") + contentCardSpan("content-card__description", "内容可能已撤回或删除") + `</span></span>`
	}
	markup := `<a class="content-card content-card--` + c.Type + `" href="` + c.URL + `" data-internal-card-state="ready"><span class="content-card__body">`
	if c.Type == "phrase" {
		kind := "句子"
		if contentCardString(reference["kind"]) == "word" {
			kind = "词语"
		}
		markup += contentCardSpan("content-card__kind", kind) + contentCardSpan("content-card__quote", contentCardString(reference["text"]))
		if source := contentCardString(reference["source"]); source != "" {
			markup += contentCardSpan("content-card__attribution", source)
		}
	} else {
		author := contentCardMap(reference["author"])
		markup += `<span class="content-card__identity">`
		if avatar := contentCardString(author["avatar_url"]); mediaReference.MatchString(avatar) {
			markup += `<img class="content-card__avatar" src="` + avatar + `" alt="" loading="lazy" />`
		}
		markup += contentCardSpan("content-card__author", contentCardString(author["display_name"]))
		if at := contentCardString(reference["published_at"]); at != "" {
			if date, e := time.Parse(time.RFC3339, at); e == nil {
				markup += `<time class="content-card__date" datetime="` + stdhtml.EscapeString(at) + `">` + date.Format("2006-01-02") + `</time>`
			}
		}
		markup += `</span>` + contentCardSpan("content-card__text", contentCardString(reference["text"]))
		if images, ok := reference["images"].([]any); ok && len(images) > 0 {
			markup += `<span class="content-card__photos">`
			count := 0
			for _, image := range images {
				photo := contentCardMap(image)
				src := contentCardString(photo["url"])
				if !mediaReference.MatchString(src) {
					continue
				}
				if count < 3 {
					markup += `<img class="content-card__photo" src="` + src + `" alt="` + stdhtml.EscapeString(contentCardString(photo["alt"])) + `" loading="lazy" />`
				}
				count++
			}
			markup += `</span>`
			if count > 3 {
				markup += contentCardSpan("content-card__photo-count", fmt.Sprintf("共%d张图片", count))
			}
		}
	}
	return markup + contentCardSpan("content-card__destination", "查看"+label) + `</span></a>`
}

func contentCardAnchor(n *xhtml.Node) (contentCard, bool) {
	var c contentCard
	if n.Type != xhtml.ElementNode || n.Data != "a" {
		return c, false
	}
	class, href := "", ""
	for _, v := range n.Attr {
		if v.Key == "class" {
			class = v.Val
		}
		if v.Key == "href" {
			href = v.Val
		}
	}
	for _, kind := range []string{"moment", "phrase"} {
		has := false
		for _, token := range strings.Fields(class) {
			if token == "content-card--"+kind {
				has = true
			}
		}
		prefix := "/phrases/"
		if kind == "moment" {
			prefix = "/moments/"
		}
		if has && strings.HasPrefix(href, prefix) {
			id := strings.TrimPrefix(href, prefix)
			if contentCardReferenceID(id) {
				return contentCard{V: 1, Type: kind, RefID: id, URL: href}, true
			}
		}
	}
	return c, false
}

// ExpandInternalContentCards never modifies immutable stored HTML. Call it at
// every public/preview boundary, so withdrawal immediately removes source text
// and photos even from an article published earlier. No external I/O occurs.
func (a *App) ExpandInternalContentCards(ctx context.Context, markup string) (string, error) {
	return a.expandInternalContentCards(ctx, a.Pool, markup)
}

// Transactional callers must pass their current transaction so a request does
// not hold one pooled connection while waiting for a second one to expand cards.
func (a *App) expandInternalContentCards(ctx context.Context, db contentQuerier, markup string) (string, error) {
	if !strings.Contains(markup, "content-card--moment") && !strings.Contains(markup, "content-card--phrase") {
		return markup, nil
	}
	contextNode := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, e := xhtml.ParseFragment(strings.NewReader(markup), contextNode)
	if e != nil {
		return "", e
	}
	for _, node := range nodes {
		contextNode.AppendChild(node)
	}
	var cards []contentCard
	var targets []*xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if c, ok := contentCardAnchor(n); ok {
			cards = append(cards, c)
			targets = append(targets, n)
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(contextNode)
	if len(cards) > 128 {
		return "", contentCardInvalid("每篇文章最多128张卡片")
	}
	if len(cards) == 0 {
		return markup, nil
	}
	references, e := contentCardLookup(ctx, db, cards)
	if e != nil {
		return "", e
	}
	for i, target := range targets {
		c := cards[i]
		replacement, e := xhtml.ParseFragment(strings.NewReader(contentCardInternalHTML(c, references[c.Type+":"+c.RefID])), &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div})
		if e != nil {
			return "", e
		}
		for _, node := range replacement {
			target.Parent.InsertBefore(node, target)
		}
		target.Parent.RemoveChild(target)
	}
	var result strings.Builder
	for child := contextNode.FirstChild; child != nil; child = child.NextSibling {
		if e = xhtml.Render(&result, child); e != nil {
			return "", e
		}
	}
	return result.String(), nil
}
