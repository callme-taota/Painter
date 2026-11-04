package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

const homeProjectLimit = 100
const homeProjectColumns = `id::text,source,repo_url,title_override,description_override,url_override,language_override,metadata,position,visible,version,fetched_at_ms,attempted_at_ms,last_error,created_at_ms,updated_at_ms`

type homeProjectInput struct {
	Source      string  `json:"source"`
	RepoURL     string  `json:"repo_url"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	URL         *string `json:"url"`
	Language    *string `json:"language"`
	Position    int     `json:"position"`
	Visible     bool    `json:"visible"`
}

func homeProjectReference(raw string) (string, error) {
	invalid := func() (string, error) {
		return "", E(400, "VALIDATION_FAILED", "请输入公开 GitHub 仓库的 owner/repo 或完整 HTTPS 地址")
	}
	if strings.TrimSpace(raw) != raw || len(raw) > 300 || strings.ContainsAny(raw, "\\%?#") {
		return invalid()
	}
	path := raw
	if strings.Contains(raw, "://") {
		u, e := url.Parse(raw)
		if e != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return invalid()
		}
		path = strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), "/")
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 || len(parts[0]) < 1 || len(parts[0]) > 39 || len(parts[1]) < 1 || len(parts[1]) > 100 || parts[0][0] == '-' || strings.HasSuffix(parts[0], "-") || strings.HasPrefix(parts[1], ".") || strings.HasSuffix(parts[1], ".git") {
		return invalid()
	}
	for i, p := range parts {
		for _, r := range p {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || i == 1 && (r == '_' || r == '.')) {
				return invalid()
			}
		}
	}
	return "https://github.com/" + parts[0] + "/" + parts[1], nil
}
func validateHomeProject(in *homeProjectInput) error {
	if in.Source != "manual" && in.Source != "github" || in.Position < 0 || in.Position > 1000000 {
		return E(400, "VALIDATION_FAILED", "项目来源或排序无效")
	}
	if in.Source == "github" {
		v, e := homeProjectReference(in.RepoURL)
		if e != nil {
			return e
		}
		in.RepoURL = v
	} else if in.RepoURL != "" {
		return E(400, "VALIDATION_FAILED", "手动项目不应包含 GitHub 仓库引用")
	}
	for _, v := range []struct {
		s     *string
		n     int
		multi bool
	}{{in.Title, 80, false}, {in.Description, 600, true}, {in.Language, 80, false}} {
		if v.s != nil {
			if e := homeText(*v.s, v.n, v.multi); e != nil {
				return e
			}
		}
	}
	if in.URL != nil {
		if e := homeURL(*in.URL); e != nil {
			return e
		}
	}
	if in.Source == "manual" && (in.Title == nil || strings.TrimSpace(*in.Title) == "" || in.URL == nil || *in.URL == "") {
		return E(400, "VALIDATION_FAILED", "手动项目需要标题和目标链接")
	}
	return nil
}
func scanHomeProject(row interface{ Scan(...any) error }) (map[string]any, error) {
	var id, source, repo, lastError string
	var title, description, target, language *string
	var raw []byte
	var position int
	var visible bool
	var version, created, updated int64
	var fetched, attempted *int64
	if e := row.Scan(&id, &source, &repo, &title, &description, &target, &language, &raw, &position, &visible, &version, &fetched, &attempted, &lastError, &created, &updated); e != nil {
		return nil, e
	}
	metadata := map[string]any{}
	if e := json.Unmarshal(raw, &metadata); e != nil {
		return nil, e
	}
	choose := func(override *string, key string) string {
		if override != nil {
			return *override
		}
		v, _ := metadata[key].(string)
		return v
	}
	name := choose(title, "title")
	if name == "" && source == "github" {
		name = strings.TrimPrefix(repo, "https://github.com/")
	}
	link := choose(target, "url")
	if link == "" {
		link = repo
	}
	out := map[string]any{"id": id, "source": source, "repo_url": repo, "title": name, "description": choose(description, "description"), "url": link, "language": choose(language, "language"), "full_name": metadata["full_name"], "avatar_url": metadata["avatar_url"], "stars": metadata["stars"], "forks": metadata["forks"], "repo_updated_at": metadata["updated_at"], "position": position, "visible": visible, "version": version, "fetched_at": nil, "last_error": lastError, "created_at": TimeString(created), "updated_at": TimeString(updated), "title_override": title, "description_override": description, "url_override": target, "language_override": language}
	if fetched != nil {
		out["fetched_at"] = TimeString(*fetched)
	}
	return out, nil
}
func homeProjectPublic(item map[string]any) {
	for _, k := range []string{"version", "position", "visible", "last_error", "created_at", "title_override", "description_override", "url_override", "language_override"} {
		delete(item, k)
	}
}
func homePublicProjects(ctx context.Context, q homeRowsQuery, c HomeConfig) ([]map[string]any, error) {
	out := []map[string]any{}
	s := c.Sections["projects"]
	if !s.Enabled || s.Limit == 0 {
		return out, nil
	}
	rows, e := q.Query(ctx, `SELECT `+homeProjectColumns+` FROM homepage_projects WHERE visible ORDER BY position,id LIMIT $1`, min(s.Limit, 24))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		item, e := scanHomeProject(rows)
		if e != nil {
			return nil, e
		}
		homeProjectPublic(item)
		out = append(out, item)
	}
	return out, rows.Err()
}
func (a *App) PublicHomeProjects(ctx context.Context, c HomeConfig) ([]map[string]any, error) {
	return homePublicProjects(ctx, a.Pool, c)
}
func (a *App) registerHomeProjects() {
	a.Handle("GET /api/v1/admin/home/projects", a.homeProjects)
	a.Handle("POST /api/v1/admin/home/projects", a.homeProjectCreate)
	a.Handle("PUT /api/v1/admin/home/projects/{id}", a.homeProjectUpdate)
	a.Handle("DELETE /api/v1/admin/home/projects/{id}", a.homeProjectDelete)
	a.Handle("POST /api/v1/admin/home/projects/{id}/refresh", a.homeProjectRefresh)
}
func (a *App) homeProjects(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r, "owner"); e != nil {
		return e
	}
	rows, e := a.Pool.Query(r.Context(), `SELECT `+homeProjectColumns+` FROM homepage_projects ORDER BY position,id LIMIT $1`, homeProjectLimit)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		v, e := scanHomeProject(rows)
		if e != nil {
			return e
		}
		items = append(items, v)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	JSON(w, 200, map[string]any{"items": items, "max_items": homeProjectLimit})
	return nil
}
func (a *App) homeProjectCreate(w http.ResponseWriter, r *http.Request) error {
	if e := homeAuthorize(a, r); e != nil {
		return e
	}
	var in homeProjectInput
	if e := decodeHome(w, r, &in); e != nil {
		return e
	}
	if e := validateHomeProject(&in); e != nil {
		return e
	}
	value, e := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		if _, e := homeRead(r.Context(), tx, true); e != nil {
			return nil, e
		}
		var n int
		if e := tx.QueryRow(r.Context(), `SELECT count(*) FROM homepage_projects`).Scan(&n); e != nil {
			return nil, e
		}
		if n >= homeProjectLimit {
			return nil, E(409, "HOME_PROJECT_LIMIT", "首页项目最多100项")
		}
		now := NowMillis()
		item, e := scanHomeProject(tx.QueryRow(r.Context(), `INSERT INTO homepage_projects(source,repo_url,title_override,description_override,url_override,language_override,position,visible,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9) RETURNING `+homeProjectColumns, in.Source, in.RepoURL, in.Title, in.Description, in.URL, in.Language, in.Position, in.Visible, now))
		if e != nil {
			return nil, e
		}
		return item, audit(r.Context(), tx, p, "home.item_create", "site", item["id"].(string), "")
	})
	if e == nil {
		JSON(w, 201, value)
	}
	return e
}
func (a *App) homeProjectUpdate(w http.ResponseWriter, r *http.Request) error {
	if e := homeAuthorize(a, r); e != nil {
		return e
	}
	id := r.PathValue("id")
	if !positiveID(id) {
		return NotFound()
	}
	version, e := IfMatch(r)
	if e != nil {
		return e
	}
	var in homeProjectInput
	if e = decodeHome(w, r, &in); e != nil {
		return e
	}
	if e = validateHomeProject(&in); e != nil {
		return e
	}
	value, e := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		old, e := scanHomeProject(tx.QueryRow(r.Context(), `SELECT `+homeProjectColumns+` FROM homepage_projects WHERE id=$1 FOR UPDATE`, id))
		if e != nil {
			return nil, e
		}
		if old["version"] != version {
			return nil, E(409, "VERSION_CONFLICT", "项目已变化，请载入最新内容")
		}
		item, e := scanHomeProject(tx.QueryRow(r.Context(), `UPDATE homepage_projects SET source=$2,repo_url=$3,title_override=$4,description_override=$5,url_override=$6,language_override=$7,position=$8,visible=$9,version=version+1,updated_at_ms=$10,metadata=CASE WHEN source=$2 AND repo_url=$3 THEN metadata ELSE '{}'::jsonb END,fetched_at_ms=CASE WHEN source=$2 AND repo_url=$3 THEN fetched_at_ms ELSE NULL END,attempted_at_ms=CASE WHEN source=$2 AND repo_url=$3 THEN attempted_at_ms ELSE NULL END,last_error='' WHERE id=$1 RETURNING `+homeProjectColumns, id, in.Source, in.RepoURL, in.Title, in.Description, in.URL, in.Language, in.Position, in.Visible, NowMillis()))
		if e != nil {
			return nil, e
		}
		return item, audit(r.Context(), tx, p, "home.item_update", "site", id, "")
	})
	if e == nil {
		JSON(w, 200, value)
	}
	return e
}
func (a *App) homeProjectDelete(w http.ResponseWriter, r *http.Request) error {
	if e := homeAuthorize(a, r); e != nil {
		return e
	}
	id := r.PathValue("id")
	if !positiveID(id) {
		return NotFound()
	}
	version, e := IfMatch(r)
	if e != nil {
		return e
	}
	value, e := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		var actual int64
		if e := tx.QueryRow(r.Context(), `SELECT version FROM homepage_projects WHERE id=$1 FOR UPDATE`, id).Scan(&actual); e != nil {
			return nil, e
		}
		if actual != version {
			return nil, E(409, "VERSION_CONFLICT", "项目已变化，请载入最新内容")
		}
		if _, e := tx.Exec(r.Context(), `DELETE FROM homepage_projects WHERE id=$1`, id); e != nil {
			return nil, e
		}
		return map[string]any{"deleted": true}, audit(r.Context(), tx, p, "home.item_delete", "site", id, "")
	})
	if e == nil {
		JSON(w, 200, value)
	}
	return e
}
func (a *App) homeProjectRefresh(w http.ResponseWriter, r *http.Request) error {
	if e := homeAuthorize(a, r); e != nil {
		return e
	}
	id := r.PathValue("id")
	if !positiveID(id) {
		return NotFound()
	}
	version, e := IfMatch(r)
	if e != nil {
		return e
	}
	value, e := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		item, e := scanHomeProject(tx.QueryRow(r.Context(), `SELECT `+homeProjectColumns+` FROM homepage_projects WHERE id=$1 FOR UPDATE`, id))
		if e != nil {
			return nil, e
		}
		if item["version"] != version {
			return nil, E(409, "VERSION_CONFLICT", "项目已变化，请载入最新内容")
		}
		if item["source"] != "github" {
			return nil, E(400, "VALIDATION_FAILED", "手动项目无需从 GitHub 刷新")
		}
		now := NowMillis()
		var attempted *int64
		if e = tx.QueryRow(r.Context(), `SELECT attempted_at_ms FROM homepage_projects WHERE id=$1`, id).Scan(&attempted); e != nil {
			return nil, e
		}
		if attempted != nil && now-*attempted < 30000 {
			return nil, E(429, "RATE_LIMITED", "此项目刚刚读取过，请30秒后再试")
		}
		var requests int
		e = tx.QueryRow(r.Context(), `INSERT INTO homepage_project_rates(owner_id,window_at_ms,requests) VALUES($1,$2,1) ON CONFLICT(owner_id) DO UPDATE SET window_at_ms=CASE WHEN homepage_project_rates.window_at_ms<=$2-60000 THEN $2 ELSE homepage_project_rates.window_at_ms END,requests=CASE WHEN homepage_project_rates.window_at_ms<=$2-60000 THEN 1 ELSE homepage_project_rates.requests+1 END WHERE homepage_project_rates.window_at_ms<=$2-60000 OR homepage_project_rates.requests<10 RETURNING requests`, p.ID, now).Scan(&requests)
		if e == pgx.ErrNoRows {
			return nil, E(429, "RATE_LIMITED", "GitHub 读取较频繁，请稍后再试")
		}
		if e != nil {
			return nil, e
		}
		meta, fetchError := a.fetchPublicGitHubRepository(r.Context(), item["repo_url"].(string))
		lastError := ""
		var raw []byte
		if fetchError != nil {
			lastError = "GitHub 信息读取失败，保留上次已保存的资料；可稍后重试。"
		} else {
			raw, _ = json.Marshal(map[string]any{"title": meta.Name, "description": meta.Description, "url": meta.URL, "full_name": meta.FullName, "avatar_url": meta.AvatarURL, "language": meta.Language, "stars": meta.Stars, "forks": meta.Forks, "updated_at": meta.UpdatedAt})
		}
		item, e = scanHomeProject(tx.QueryRow(r.Context(), `UPDATE homepage_projects SET metadata=CASE WHEN $2::bytea IS NULL THEN metadata ELSE convert_from($2,'UTF8')::jsonb END,fetched_at_ms=CASE WHEN $2::bytea IS NULL THEN fetched_at_ms ELSE $3 END,attempted_at_ms=$3,last_error=$4,version=version+1,updated_at_ms=$3 WHERE id=$1 RETURNING `+homeProjectColumns, id, raw, now, lastError))
		if e != nil {
			return nil, e
		}
		return item, audit(r.Context(), tx, p, "home.item_update", "site", id, "")
	})
	if e == nil {
		JSON(w, 200, value)
	}
	return e
}
