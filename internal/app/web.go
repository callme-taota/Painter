package app

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"painter-next/web"
)

type webPage struct {
	Kind, Title, Description, Canonical, JS, Next, TopicParam, ReturnTo, Error, SearchID, PreviewImage, ViewerID string
	ScriptNonce                                                                                                  string
	Site, Post, User, Featured, FriendSettings                                                                   map[string]any
	Items, Categories, Tags, Topics, Related                                                                     []map[string]any
	Series                                                                                                       []map[string]any
	CSS                                                                                                          []string
	Query                                                                                                        url.Values
	TOC                                                                                                          []*ArticleTOCNode
	HTML                                                                                                         template.HTML
	JSONLD                                                                                                       template.JS
	SearchActive, NoIndex, Registration, LoginError, Creator, Authenticated                                      bool
	ReadTime, Status, QueryLength, ResultCount                                                                   int
	HomeConfig                                                                                                   *HomeConfig
	HomeSections                                                                                                 []homeWebSection
	HomeFirst                                                                                                    string
	HomeHasIntro                                                                                                 bool
}

const siteFaviconPath = "images/painter-favicon-d4d342f8b170.ico"

func (p webPage) DocumentTitle() string {
	site := strings.TrimSpace(webString(p.Site["display_name"]))
	if site == "" {
		site = "Painter"
	}
	title := strings.TrimSpace(p.Title)
	if title == "" || strings.EqualFold(title, site) {
		return site
	}
	return title + " · " + site
}

func (p webPage) FaviconURL() string { return "/assets/" + siteFaviconPath }

func webString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func webDate(v any) string {
	s := webString(v)
	t, e := time.Parse(time.RFC3339Nano, s)
	if e != nil {
		return s
	}
	return t.In(time.FixedZone("CST", 8*3600)).Format("2006年1月2日")
}
func webDateTime(v any) string {
	t, e := time.Parse(time.RFC3339Nano, webString(v))
	if e != nil {
		return ""
	}
	return t.In(time.FixedZone("CST", 8*3600)).Format("2006年1月2日 15:04")
}
func webInitial(v any) string {
	r := []rune(webString(v))
	if len(r) > 0 {
		return string(r[0])
	}
	return "P"
}
func webMaps(v any) []map[string]any {
	if a, ok := v.([]map[string]any); ok {
		return a
	}
	out := []map[string]any{}
	if a, ok := v.([]any); ok {
		for _, r := range a {
			if m, ok := r.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}
func (a *App) webSite(ctx context.Context) (map[string]any, error) {
	var name, tagline, about, footer, icp, seo, introName string
	var socials, roles []byte
	var avatar *string
	err := a.Pool.QueryRow(ctx, `SELECT display_name,tagline,about_markdown,footer_text,icp_number,seo_description,social_links_json,avatar_media_id,intro_name,intro_roles_json FROM site_profile WHERE id=1`).Scan(&name, &tagline, &about, &footer, &icp, &seo, &socials, &avatar, &introName, &roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]any{"display_name": "Painter Next", "tagline": "记录思考，分享值得停下来读的文字。", "footer_text": "以文字，连接想法。"}, nil
	}
	if err != nil {
		return nil, err
	}
	var links []map[string]any
	_ = json.Unmarshal(socials, &links)
	var introRoles []string
	_ = json.Unmarshal(roles, &introRoles)
	avatarURL := ""
	if avatar != nil {
		avatarURL = "/media/" + *avatar
	}
	rss, controls, err := a.siteExperience(ctx)
	if err != nil {
		return nil, err
	}
	friends, err := a.FriendSettings(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"avatar_url": avatarURL, "display_name": name, "tagline": tagline, "intro_name": introName, "intro_roles": introRoles, "about_markdown": about, "footer_text": footer, "icp_number": icp, "seo_description": seo, "social_links": links, "rss_enabled": rss, "floating_controls": controls, "friend_links_enabled": friends["enabled"]}, nil
}
func (a *App) webTaxonomy(ctx context.Context, kind string) ([]map[string]any, error) {
	if kind != "tags" {
		return nil, NotFound()
	}
	count := `(SELECT count(DISTINCT p.post_id) FROM public_posts p JOIN effective_revision_tags rt ON rt.revision_id=p.revision_id WHERE rt.tag_id=t.id)`
	rows, e := a.Pool.Query(ctx, `SELECT t.id::text,t.name,t.description,`+count+` FROM tags t WHERE t.archived_at_ms IS NULL AND NOT EXISTS(SELECT 1 FROM tag_aliases ta WHERE ta.tag_id=t.id AND ta.canonical_tag_id<>t.id) ORDER BY t.name,t.id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, desc string
		var count int64
		if e = rows.Scan(&id, &name, &desc, &count); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "name": name, "description": desc, "article_count": count})
	}
	return out, rows.Err()
}
func webNext(r *http.Request, result map[string]any) string {
	next := webString(result["next_cursor"])
	if next == "" {
		return ""
	}
	q := r.URL.Query()
	q.Set("cursor", next)
	return r.URL.Path + "?" + q.Encode()
}
func (a *App) RegisterWeb() {
	templates := template.Must(template.New("page").Funcs(template.FuncMap{"str": webString, "hasTag": func(q url.Values, id any) bool {
		for _, v := range q["tag_id"] {
			if v == webString(id) {
				return true
			}
		}
		return false
	}, "date": webDate, "dateTime": webDateTime, "gameHours": gameHours, "phrases": func(v any) []string { return strings.SplitAfter(webString(v), "，") }, "initial": webInitial, "rank": func(i int) int { return i + 1 }, "linkHost": func(v any) string {
		u, _ := url.Parse(webString(v))
		if u == nil {
			return ""
		}
		return strings.TrimPrefix(u.Host, "www.")
	}}).ParseFS(web.FS, "templates/*.html"))
	render := func(w http.ResponseWriter, r *http.Request, p webPage) {
		p.ScriptNonce = requestScriptNonce(r.Context())
		if p.Kind == "home" || p.Kind == "login" || p.Kind == "register" {
			installed, e := a.installationComplete(r.Context(), a.Pool)
			if e != nil {
				Error(w, e)
				return
			}
			if !installed {
				http.Redirect(w, r, "/setup", http.StatusSeeOther)
				return
			}
		}
		if p.Site == nil {
			site, e := a.webSite(r.Context())
			if e != nil {
				site = map[string]any{"display_name": "Painter Next"}
				if p.Status == 0 {
					p.Status = 503
					p.Kind = "error"
					p.Title = "暂时无法打开页面"
					p.Error = "服务暂不可用，请稍后重试。"
				}
			}
			p.Site = site
		}
		if p.Query == nil {
			p.Query = r.URL.Query()
		}
		if p.Kind != "studio" && p.Kind != "setup" {
			if actor, err := a.Principal(r); err == nil {
				p.Authenticated = true
				p.ViewerID = actor.ID
				p.Creator = actor.Role == "owner" || actor.Role == "author"
			}
		}
		if p.Title == "" {
			p.Title = webString(p.Site["display_name"])
		}
		if p.Description == "" {
			p.Description = webString(p.Site["seo_description"])
			if p.Description == "" {
				p.Description = webString(p.Site["tagline"])
			}
		}
		p.Canonical = strings.TrimRight(a.Config.PublicBaseURL, "/") + r.URL.Path
		if p.Kind == "search" || (p.Kind == "author" && r.URL.Query().Get("q") != "") {
			p.Canonical += "?q=" + url.QueryEscape(r.URL.Query().Get("q"))
		}
		entry := "src/public.ts"
		if p.Kind == "studio" {
			entry = "src/studio.ts"
		}
		if p.Kind == "setup" {
			entry = "src/setup.ts"
		}
		p.JS, p.CSS = web.Assets(entry)
		if p.Status == 0 {
			p.Status = 200
		}
		var buffer bytes.Buffer
		if e := templates.ExecuteTemplate(&buffer, "page", p); e != nil {
			http.Error(w, "页面暂不可用", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(p.Status)
		_, _ = w.Write(buffer.Bytes())
	}
	failure := func(w http.ResponseWriter, r *http.Request, e error) {
		code := 503
		title, message := "暂时无法打开页面", "请稍后重试，或返回文章列表。"
		var apiErr *APIError
		if errors.Is(e, pgx.ErrNoRows) {
			code = 404
		}
		if errors.As(e, &apiErr) {
			code = apiErr.Status
			message = apiErr.Message
		}
		if code == 404 {
			title = "这篇内容还没有公开"
			message = "它可能还在写作中，或作者已撤回。"
		}
		render(w, r, webPage{Kind: "error", Status: code, Title: title, Error: message, NoIndex: true})
	}
	a.Mux.Handle("GET /assets/", http.StripPrefix("/assets/", assetHTTPHandler(web.AssetFS())))
	a.Mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeFileFS(w, r, web.AssetFS(), siteFaviconPath)
	})
	a.Mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		site, e := a.webSite(r.Context())
		if e != nil {
			failure(w, r, e)
			return
		}
		p, e := a.homepagePage(r.Context(), site)
		if e != nil {
			failure(w, r, e)
			return
		}
		render(w, r, p)
	})
	a.Mux.HandleFunc("GET /posts", func(w http.ResponseWriter, r *http.Request) {
		data, e := a.PublicPosts(r.Context(), r.URL.Query())
		if e != nil {
			failure(w, r, e)
			return
		}
		tags, e := a.webTaxonomy(r.Context(), "tags")
		if e != nil {
			failure(w, r, e)
			return
		}
		render(w, r, webPage{Kind: "posts", Title: "所有文章", Items: webMaps(data["items"]), Tags: tags, Next: webNext(r, data)})
	})
	a.Mux.HandleFunc("GET /series", func(w http.ResponseWriter, r *http.Request) {
		data, err := a.PublicSeries(r.Context(), r.URL.Query())
		if err != nil {
			failure(w, r, err)
			return
		}
		render(w, r, webPage{Kind: "series-list", Title: "专栏", Description: "把文章写成连续的章节，按自己的节奏读完一个主题。", Items: webMaps(data["items"]), Next: webNext(r, data)})
	})
	a.Mux.HandleFunc("GET /series/{id}", func(w http.ResponseWriter, r *http.Request) {
		data, err := a.PublicSeriesDetail(r.Context(), r.PathValue("id"))
		if err != nil {
			failure(w, r, err)
			return
		}
		render(w, r, webPage{Kind: "series", Title: webString(data["title"]), Description: webString(data["description"]), Post: data, Items: webMaps(data["chapters"]), PreviewImage: webString(data["cover_url"])})
	})
	a.Mux.HandleFunc("GET /games", func(w http.ResponseWriter, r *http.Request) {
		data, e := a.PublicGames(r.Context(), r.URL.Query())
		if e != nil {
			failure(w, r, e)
			return
		}
		total, _ := strconv.Atoi(webString(data["total"]))
		render(w, r, webPage{Kind: "games", Title: "游戏墙", Description: "那些走进去的世界，和留在其中的时间。", Items: webMaps(data["items"]), ResultCount: total, Next: webNext(r, data)})
	})
	a.Mux.HandleFunc("GET /posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		post, e := a.PublicPost(r.Context(), r.PathValue("id"))
		if e != nil {
			failure(w, r, e)
			return
		}
		toc := []TOCEntry{}
		b, _ := json.Marshal(post["toc"])
		_ = json.Unmarshal(b, &toc)
		text := webString(post["plain_text"])
		minutes := (utf8.RuneCountInString(text) + 399) / 400
		if minutes < 1 {
			minutes = 1
		}
		schema := map[string]any{"@context": "https://schema.org", "@type": "BlogPosting", "headline": post["title"], "description": post["summary"], "mainEntityOfPage": a.Config.PublicBaseURL + "/posts/" + r.PathValue("id"), "author": post["author"]}
		display, _ := post["date_display"].(map[string]any)
		if display["published"] == true {
			schema["datePublished"] = post["published_at"]
		}
		if display["created"] == true {
			schema["dateCreated"] = post["created_at"]
		}
		if display["updated"] == true && post["updated_at"] != nil {
			schema["dateModified"] = post["updated_at"]
		}
		ld, _ := json.Marshal(schema)
		related, e := a.RelatedPosts(r.Context(), r.PathValue("id"))
		if e != nil {
			failure(w, r, e)
			return
		}
		series, e := a.PostSeries(r.Context(), r.PathValue("id"))
		if e != nil {
			failure(w, r, e)
			return
		}
		render(w, r, webPage{Kind: "article", Title: webString(post["title"]), Description: webString(post["summary"]), Post: post, Related: related, Series: series, TOC: articleTOCTree(toc), ReadTime: minutes, HTML: template.HTML(webString(post["sanitized_html"])), JSONLD: template.JS(ld)})
	})
	a.Mux.HandleFunc("GET /tags", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/search", http.StatusMovedPermanently)
	})
	a.Mux.HandleFunc("GET /categories", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tags", http.StatusMovedPermanently)
	})
	a.Mux.HandleFunc("GET /categories/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !positiveID(r.PathValue("id")) {
			failure(w, r, NotFound())
			return
		}
		var tag string
		if e := a.Pool.QueryRow(r.Context(), `SELECT tag_id::text FROM category_tag_map WHERE category_id=$1`, r.PathValue("id")).Scan(&tag); e != nil {
			failure(w, r, e)
			return
		}
		http.Redirect(w, r, "/search?tag_id="+url.QueryEscape(tag), http.StatusMovedPermanently)
	})
	a.Mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !positiveID(id) {
			failure(w, r, NotFound())
			return
		}
		pageSettings, e := a.AuthorPage(r.Context(), id)
		if e != nil {
			failure(w, r, e)
			return
		}
		u, e := a.userDTO(r.Context(), id, false)
		if e != nil {
			failure(w, r, e)
			return
		}
		u["page"] = pageSettings
		var creator bool
		var joined int64
		var followers, following int64
		if e := a.Pool.QueryRow(r.Context(), `SELECT role IN ('owner','author'),created_at_ms,(SELECT count(*) FROM follows WHERE followed_user_id=u.id),(SELECT count(*) FROM follows WHERE follower_user_id=u.id) FROM users u WHERE id=$1 AND disabled_at_ms IS NULL`, id).Scan(&creator, &joined, &followers, &following); e != nil {
			failure(w, r, e)
			return
		}
		u["creator"], u["joined_at"], u["follower_count"], u["following_count"] = creator, TimeString(joined), followers, following
		if !creator {
			render(w, r, webPage{Kind: "author", Title: webString(u["display_name"]), User: u})
			return
		}
		data, e := a.AuthorPosts(r.Context(), id, r.URL.Query())
		p := webPage{Kind: "author", Title: webString(u["display_name"]), User: u, NoIndex: r.URL.Query().Get("q") != ""}
		if e != nil {
			var ae *APIError
			p.Error = "暂时无法载入文章，请稍后重试。"
			if errors.As(e, &ae) {
				p.Error = ae.Message
				p.Status = ae.Status
			}
		} else {
			p.Items = webMaps(data["items"])
			p.Next = webNext(r, data)
		}
		render(w, r, p)
	})
	a.Mux.HandleFunc("GET /search", func(w http.ResponseWriter, r *http.Request) {
		p := webPage{Kind: "search", Title: "搜索", NoIndex: true}
		query := r.URL.Query()
		if len(query["tag_id"]) > 0 || query.Get("author_id") != "" || query.Get("category_id") != "" {
			p.Query = query
			p.Query.Set("type", "post")
		}
		if r.URL.Query().Get("q") != "" || len(r.URL.Query()["tag_id"]) > 0 || r.URL.Query().Get("author_id") != "" {
			p.SearchActive = true
			data, e := a.SearchContent(r.Context(), r.URL.Query())
			if e != nil {
				p.Error = "搜索暂不可用，请缩短关键词或稍后重试。"
				var ae *APIError
				if errors.As(e, &ae) {
					p.Error = ae.Message
					p.Status = ae.Status
				}
			} else {
				p.Items = webMaps(data["items"])
				p.Next = webNext(r, data)
				p.SearchID = webString(data["search_instance_id"])
				p.QueryLength = utf8.RuneCountInString(webString(data["query"]))
				p.ResultCount = len(p.Items)
			}
		}
		p.Tags, _ = a.webTaxonomy(r.Context(), "tags")
		render(w, r, p)
	})
	a.Mux.HandleFunc("GET /about", func(w http.ResponseWriter, r *http.Request) {
		site, e := a.webSite(r.Context())
		if e != nil {
			failure(w, r, e)
			return
		}
		rendered, e := RenderMarkdown(webString(site["about_markdown"]))
		if e != nil {
			failure(w, r, e)
			return
		}
		expanded, e := a.ExpandInternalContentCards(r.Context(), rendered.HTML)
		if e != nil {
			failure(w, r, e)
			return
		}
		render(w, r, webPage{Kind: "about", Title: "关于", Site: site, HTML: template.HTML(expanded)})
	})
	authPage := func(w http.ResponseWriter, r *http.Request) {
		if a.redirectAuthenticationOrigin(w, r) {
			return
		}
		var registration bool
		_ = a.Pool.QueryRow(r.Context(), "SELECT registration_enabled AND external_login_enabled FROM site_settings WHERE id=1").Scan(&registration)
		returnTo := "/me"
		if r.URL.Query().Get("return_to") != "" {
			returnTo = internalReturn(r.URL.Query().Get("return_to"))
		}
		kind, title := "login", "登录"
		if r.URL.Path == "/register" {
			kind, title = "register", "注册"
		}
		render(w, r, webPage{Kind: kind, Title: title, NoIndex: true, Registration: registration, ReturnTo: returnTo, LoginError: r.URL.Query().Get("error") != ""})
	}
	a.Mux.HandleFunc("GET /phrases/{id}", func(w http.ResponseWriter, r *http.Request) {
		item, e := a.PublicContentCard(r.Context(), "phrase", r.PathValue("id"))
		if e != nil {
			failure(w, r, e)
			return
		}
		render(w, r, webPage{Kind: "phrases", Title: "片语", Description: webString(item["text"]), Items: []map[string]any{item}})
	})
	a.Mux.HandleFunc("GET /phrases", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("limit") == "" {
			query.Set("limit", "20")
		}
		data, e := a.PublicPhrases(r.Context(), query)
		if e != nil {
			failure(w, r, e)
			return
		}
		items := webMaps(data["items"])
		for i := range items {
			if i+1 < len(items) {
				items[i]["next_id"] = items[i+1]["id"]
			}
		}
		render(w, r, webPage{Kind: "phrases", Title: "片语", Description: "文章之外，留下的只言片语。", Items: items, Next: webNext(r, data)})
	})
	a.Mux.HandleFunc("GET /moments", func(w http.ResponseWriter, r *http.Request) {
		data, e := a.PublicMoments(r.Context(), r.URL.Query())
		if e != nil {
			failure(w, r, e)
			return
		}
		render(w, r, webPage{Kind: "moments", Title: "动态", Description: "记录此刻的想法与日常。", Items: webMaps(data["items"]), Next: webNext(r, data), NoIndex: r.URL.Query().Get("author_id") != "" || r.URL.Query().Get("cursor") != ""})
	})
	a.Mux.HandleFunc("GET /moments/{id}", func(w http.ResponseWriter, r *http.Request) {
		item, e := a.PublicMoment(r.Context(), r.PathValue("id"))
		if e != nil {
			failure(w, r, e)
			return
		}
		author, _ := item["author"].(map[string]any)
		description := []rune(webString(item["text"]))
		images := webMaps(item["images"])
		previewImage := ""
		if len(images) > 0 {
			previewImage = a.Config.PublicBaseURL + webString(images[0]["url"])
			if strings.TrimSpace(string(description)) == "" {
				description = []rune(fmt.Sprintf("%s分享了%d张图片。", webString(author["display_name"]), len(images)))
			}
		}
		if len(description) > 160 {
			description = description[:160]
		}
		render(w, r, webPage{Kind: "moment", Title: webString(author["display_name"]) + "的动态", Description: string(description), Post: item, PreviewImage: previewImage})
	})
	a.Mux.HandleFunc("GET /friends", func(w http.ResponseWriter, r *http.Request) {
		data, e := a.PublicFriends(r.Context())
		if e != nil {
			failure(w, r, e)
			return
		}
		settings := data["settings"].(map[string]any)
		render(w, r, webPage{Kind: "friends", Title: webString(settings["title"]), Description: webString(settings["description"]), FriendSettings: settings, Items: webMaps(data["items"])})
	})
	a.Mux.HandleFunc("GET /setup", func(w http.ResponseWriter, r *http.Request) {
		render(w, r, webPage{Kind: "setup", Title: "安装 Painter", NoIndex: true})
	})
	a.Mux.HandleFunc("GET /login", authPage)
	a.Mux.HandleFunc("GET /register", authPage)
	shell := func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimRight(r.URL.Path, "/")
		known := map[string]bool{"/me": true, "/me/bookmarks": true, "/me/following": true, "/me/followers": true, "/me/history": true, "/me/sessions": true, "/me/media": true, "/studio": true, "/studio/posts": true, "/studio/posts/new": true, "/studio/moments": true, "/studio/series": true, "/studio/home": true, "/studio/settings": true, "/studio/phrases": true, "/studio/friends": true, "/studio/taxonomy": true, "/studio/members": true, "/studio/comments": true, "/studio/analytics": true, "/studio/operations": true}
		if !known[path] && !(strings.HasPrefix(path, "/studio/posts/") && positiveID(strings.TrimPrefix(path, "/studio/posts/"))) {
			failure(w, r, NotFound())
			return
		}
		p, e := a.Principal(r)
		if e != nil {
			http.Redirect(w, r, "/login?return_to="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		if strings.HasPrefix(path, "/studio") {
			allowed := p.Role == "owner"
			if strings.HasPrefix(path, "/studio/posts") || path == "/studio/analytics" || path == "/studio/moments" || path == "/studio/series" {
				allowed = p.Role == "owner" || p.Role == "author"
			}
			if !allowed {
				render(w, r, webPage{Kind: "error", Status: 403, Title: "这里需要写作或管理权限", Error: "你的阅读、收藏和关注功能仍然可用。写作权限由站长分配。", NoIndex: true})
				return
			}
		}
		if path == "/studio" {
			http.Redirect(w, r, "/studio/posts", 303)
			return
		}
		render(w, r, webPage{Kind: "studio", Title: "我的空间", NoIndex: true})
	}
	for _, route := range []string{"GET /me", "GET /me/", "GET /studio", "GET /studio/"} {
		a.Mux.HandleFunc(route, shell)
	}
	a.Mux.HandleFunc("GET /rss.xml", func(w http.ResponseWriter, r *http.Request) {
		var enabled bool
		if e := a.Pool.QueryRow(r.Context(), `SELECT rss_enabled FROM site_settings WHERE id=1`).Scan(&enabled); e != nil {
			http.Error(w, "Feed unavailable", 503)
			return
		}
		if !enabled {
			http.NotFound(w, r)
			return
		}
		data, e := a.PublicPosts(r.Context(), url.Values{"limit": {"20"}})
		if e != nil {
			http.Error(w, "Feed unavailable", 503)
			return
		}
		site, e := a.webSite(r.Context())
		if e != nil {
			http.Error(w, "Feed unavailable", 503)
			return
		}
		type item struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			GUID        string `xml:"guid"`
			Date        string `xml:"pubDate"`
		}
		type channel struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			Items       []item `xml:"item"`
		}
		feed := struct {
			XMLName xml.Name `xml:"rss"`
			Version string   `xml:"version,attr"`
			Channel channel  `xml:"channel"`
		}{Version: "2.0", Channel: channel{Title: webString(site["display_name"]), Link: a.Config.PublicBaseURL, Description: webString(site["tagline"])}}
		for _, post := range webMaps(data["items"]) {
			link := a.Config.PublicBaseURL + "/posts/" + webString(post["id"])
			stamp, _ := time.Parse(time.RFC3339Nano, webString(post["published_at"]))
			feed.Channel.Items = append(feed.Channel.Items, item{Title: webString(post["title"]), Link: link, GUID: link, Description: webString(post["summary"]), Date: stamp.Format(time.RFC1123Z)})
		}
		b, e := xml.MarshalIndent(feed, "", "  ")
		if e != nil {
			http.Error(w, "Feed unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(append([]byte(xml.Header), b...))
	})
	a.Mux.HandleFunc("GET /sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		type loc struct {
			URL string `xml:"loc"`
		}
		out := struct {
			XMLName xml.Name `xml:"urlset"`
			NS      string   `xml:"xmlns,attr"`
			URLs    []loc    `xml:"url"`
		}{NS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
		for _, path := range []string{"/", "/posts", "/series", "/moments", "/about"} {
			out.URLs = append(out.URLs, loc{a.Config.PublicBaseURL + path})
		}
		var gamesEnabled bool
		if e := a.Pool.QueryRow(r.Context(), `SELECT (config->'sections'->'steam'->>'enabled')::boolean FROM homepage_settings WHERE id=1`).Scan(&gamesEnabled); e != nil {
			http.Error(w, "Sitemap unavailable", 503)
			return
		}
		if gamesEnabled {
			out.URLs = append(out.URLs, loc{a.Config.PublicBaseURL + "/games"})
		}
		friendSettings, e := a.FriendSettings(r.Context())
		if e != nil {
			http.Error(w, "Sitemap unavailable", 503)
			return
		}
		if friendSettings["enabled"] == true {
			out.URLs = append(out.URLs, loc{a.Config.PublicBaseURL + "/friends"})
		}
		q := url.Values{"limit": {"50"}}
		for pages := 0; pages < 1000; pages++ {
			data, e := a.PublicPosts(r.Context(), q)
			if e != nil {
				http.Error(w, "Sitemap unavailable", 503)
				return
			}
			for _, post := range webMaps(data["items"]) {
				out.URLs = append(out.URLs, loc{a.Config.PublicBaseURL + "/posts/" + webString(post["id"])})
			}
			next := webString(data["next_cursor"])
			if next == "" {
				break
			}
			q.Set("cursor", next)
		}
		q = url.Values{"limit": {"50"}}
		for pages := 0; pages < 1000; pages++ {
			data, e := a.PublicMoments(r.Context(), q)
			if e != nil {
				http.Error(w, "Sitemap unavailable", 503)
				return
			}
			for _, item := range webMaps(data["items"]) {
				out.URLs = append(out.URLs, loc{a.Config.PublicBaseURL + "/moments/" + webString(item["id"])})
			}
			next := webString(data["next_cursor"])
			if next == "" {
				break
			}
			q.Set("cursor", next)
		}
		q = url.Values{"limit": {"50"}}
		for pages := 0; pages < 1000; pages++ {
			data, err := a.PublicSeries(r.Context(), q)
			if err != nil {
				http.Error(w, "Sitemap unavailable", 503)
				return
			}
			for _, item := range webMaps(data["items"]) {
				out.URLs = append(out.URLs, loc{a.Config.PublicBaseURL + "/series/" + webString(item["id"])})
			}
			if next := webString(data["next_cursor"]); next != "" {
				q.Set("cursor", next)
			} else {
				break
			}
		}
		b, _ := xml.Marshal(out)
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(append([]byte(xml.Header), b...))
	})
	a.Mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprint(w, "User-agent: *\nDisallow: /studio\nDisallow: /me\nDisallow: /search\nSitemap: "+a.Config.PublicBaseURL+"/sitemap.xml\n")
	})
	a.Mux.HandleFunc("GET /article", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if !positiveID(id) {
			failure(w, r, NotFound())
			return
		}
		http.Redirect(w, r, "/posts/"+id, 301)
	})
	a.Mux.HandleFunc("GET /articleList", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/posts", 301) })
	a.Mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if !positiveID(id) {
			failure(w, r, NotFound())
			return
		}
		http.Redirect(w, r, "/users/"+id, 301)
	})
	a.Mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) { failure(w, r, NotFound()) })
}

var _ = strconv.Itoa
