package app

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// LegacyPackage is an explicit offline interchange contract, not a MySQL dump
// parser. Exporters must distinguish saved draft from last actually published
// source. Passwords, tokens, OAuth states, sessions and SMTP secrets are forbidden.
type LegacyPackage struct {
	Version        int                     `json:"version"`
	BatchID        string                  `json:"batch_id"`
	SourceSystem   string                  `json:"source_system"`
	Users          []LegacyUser            `json:"users"`
	Media          []LegacyMedia           `json:"media"`
	Categories     []LegacyTaxonomy        `json:"categories"`
	Tags           []LegacyTaxonomy        `json:"tags"`
	Posts          []LegacyPost            `json:"posts"`
	Comments       []LegacyComment         `json:"comments"`
	PostLikes      []LegacyPostRelation    `json:"post_likes"`
	CommentLikes   []LegacyCommentRelation `json:"comment_likes"`
	Bookmarks      []LegacyPostRelation    `json:"bookmarks"`
	Follows        []LegacyFollow          `json:"follows"`
	History        []LegacyPostRelation    `json:"history"`
	LegacyCounters []LegacyCounter         `json:"legacy_counters"`
	SiteProfile    *LegacySiteProfile      `json:"site_profile,omitempty"`
	Settings       *LegacySettings         `json:"settings,omitempty"`
}
type LegacyUser struct {
	ID            string  `json:"id"`
	Username      string  `json:"username"`
	DisplayName   string  `json:"display_name"`
	Email         *string `json:"email"`
	Phone         string  `json:"phone"`
	Bio           string  `json:"bio"`
	Role          string  `json:"role"`
	Disabled      bool    `json:"disabled"`
	GitHubSubject string  `json:"github_subject"`
	AvatarMediaID *string `json:"avatar_media_id"`
	CreatedAtMS   int64   `json:"created_at_ms"`
	UpdatedAtMS   int64   `json:"updated_at_ms"`
}
type LegacyMedia struct {
	ID           string   `json:"id"`
	OwnerID      string   `json:"owner_id"`
	Path         string   `json:"path"`
	OriginalName string   `json:"original_name"`
	MIMEType     string   `json:"mime_type"`
	SHA256       string   `json:"sha256"`
	LegacyPaths  []string `json:"legacy_paths"`
	CreatedAtMS  int64    `json:"created_at_ms"`
}
type LegacyTaxonomy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Archived    bool   `json:"archived"`
	CreatedAtMS int64  `json:"created_at_ms"`
}
type LegacyContent struct {
	Title       string              `json:"title"`
	Summary     string              `json:"summary"`
	Markdown    string              `json:"markdown"`
	CategoryID  *string             `json:"category_id"`
	TagIDs      []string            `json:"tag_ids"`
	MediaIDs    []string            `json:"media_ids"`
	Cover       CoverConfig         `json:"cover"`
	DateDisplay *ArticleDateDisplay `json:"date_display,omitempty"`
}
type LegacyPost struct {
	ID              string         `json:"id"`
	AuthorID        string         `json:"author_id"`
	Draft           LegacyContent  `json:"draft"`
	Publication     *LegacyContent `json:"publication"`
	PublishedAtMS   *int64         `json:"published_at_ms"`
	DeletedAtMS     *int64         `json:"deleted_at_ms"`
	CreatedAtMS     int64          `json:"created_at_ms"`
	UpdatedAtMS     int64          `json:"updated_at_ms"`
	LegacyReadCount int64          `json:"legacy_read_count"`
}
type LegacyComment struct {
	ID          string `json:"id"`
	PostID      string `json:"post_id"`
	UserID      string `json:"user_id"`
	Body        string `json:"body"`
	State       string `json:"state"`
	CreatedAtMS int64  `json:"created_at_ms"`
	UpdatedAtMS int64  `json:"updated_at_ms"`
}
type LegacyPostRelation struct {
	PostID string `json:"post_id"`
	UserID string `json:"user_id"`
	AtMS   int64  `json:"at_ms"`
}
type LegacyCommentRelation struct {
	CommentID string `json:"comment_id"`
	UserID    string `json:"user_id"`
	AtMS      int64  `json:"at_ms"`
}
type LegacyFollow struct {
	FollowerID string `json:"follower_id"`
	FollowedID string `json:"followed_id"`
	AtMS       int64  `json:"at_ms"`
}
type LegacyCounter struct {
	SourceKey string `json:"source_key"`
	Day       string `json:"day"`
	Count     int64  `json:"count"`
}
type LegacySocial struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}
type LegacySiteProfile struct {
	DisplayName    string         `json:"display_name"`
	Tagline        string         `json:"tagline"`
	AboutMarkdown  string         `json:"about_markdown"`
	AvatarMediaID  *string        `json:"avatar_media_id"`
	SocialLinks    []LegacySocial `json:"social_links"`
	FooterText     string         `json:"footer_text"`
	ICPNumber      string         `json:"icp_number"`
	SEODescription string         `json:"seo_description"`
}
type LegacySettings struct {
	RegistrationEnabled bool `json:"registration_enabled"`
	CommentsEnabled     bool `json:"comments_enabled"`
	CollectionsEnabled  bool `json:"collections_enabled"`
	FollowsEnabled      bool `json:"follows_enabled"`
	AnalyticsEnabled    bool `json:"analytics_enabled"`
}
type ImportReport struct {
	DryRun        bool                         `json:"dry_run"`
	Committed     bool                         `json:"committed"`
	BatchID       string                       `json:"batch_id"`
	SourceSystem  string                       `json:"source_system"`
	PackageSHA256 string                       `json:"package_sha256"`
	Counts        map[string]int               `json:"counts"`
	IDMappings    map[string]map[string]string `json:"id_mappings"`
	Warnings      []string                     `json:"warnings"`
	JournalPath   string                       `json:"journal_path,omitempty"`
}
type preparedLegacyMedia struct {
	Input                       LegacyMedia
	ID, Key, Source, Hash, MIME string
	Size                        int64
}
type preparedLegacyContent struct {
	Draft    draftInput
	Rendered Rendered
}
type preparedLegacyPost struct {
	Input       LegacyPost
	Draft       preparedLegacyContent
	Publication *preparedLegacyContent
}
type preparedLegacy struct {
	Packet LegacyPackage
	Report ImportReport
	Media  []preparedLegacyMedia
	Posts  []preparedLegacyPost
}

func legacyAt(v int64) int64 {
	if v <= 0 {
		return 1
	}
	return v
}
func packageFile(path string) (string, string, error) {
	s, e := os.Stat(path)
	if e != nil {
		return "", "", e
	}
	if s.IsDir() {
		path = filepath.Join(path, "manifest.json")
	}
	path, e = filepath.Abs(path)
	if e != nil {
		return "", "", e
	}
	return path, filepath.Dir(path), nil
}
func packageAsset(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return "", fmt.Errorf("media path must be relative to package")
	}
	clean := filepath.Clean(relative)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("media path escapes package")
	}
	candidate := filepath.Join(root, clean)
	resolved, e := filepath.EvalSymlinks(candidate)
	if e != nil {
		return "", e
	}
	rootReal, e := filepath.EvalSymlinks(root)
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(rootReal, resolved)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("media symlink escapes package")
	}
	if candidate != resolved {
		return "", fmt.Errorf("media symlinks are not accepted")
	}
	return candidate, nil
}
func uniqueLegacyID(seen map[string]bool, id, kind string) error {
	if !positiveID(id) || seen[id] {
		return fmt.Errorf("duplicate or invalid %s id %q", kind, id)
	}
	seen[id] = true
	return nil
}
func prepareLegacy(path string) (preparedLegacy, error) {
	p := preparedLegacy{}
	file, root, e := packageFile(path)
	if e != nil {
		return p, e
	}
	f, e := os.Open(file)
	if e != nil {
		return p, e
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 128<<20))
	d.DisallowUnknownFields()
	if e = d.Decode(&p.Packet); e != nil {
		return p, fmt.Errorf("invalid legacy package contract: %w", e)
	}
	if e = d.Decode(new(any)); e != io.EOF {
		return p, fmt.Errorf("legacy package must contain one JSON object")
	}
	packet := p.Packet
	if packet.Version != 1 || len(packet.BatchID) < 1 || len(packet.BatchID) > 100 || len(packet.SourceSystem) < 1 || len(packet.SourceSystem) > 100 {
		return p, fmt.Errorf("package version=1, batch_id and source_system required")
	}
	sum, _, e := fileHash(file)
	if e != nil {
		return p, e
	}
	report := ImportReport{BatchID: packet.BatchID, SourceSystem: packet.SourceSystem, PackageSHA256: sum, Counts: map[string]int{}, IDMappings: map[string]map[string]string{}, Warnings: []string{"Offline JSON adapter validated; this is not an extraction or rehearsal against the real legacy MySQL database.", "Legacy read/visitor counters are isolated from Next analytics; historical sessions/passwords/tokens are never imported.", "Unlinked historical users cannot log in until an operator supplies an explicit GitHub numeric-subject mapping."}}
	for _, kind := range []string{"users", "media", "categories", "tags", "posts", "comments"} {
		report.IDMappings[kind] = map[string]string{}
	}
	users := map[string]bool{}
	names := map[string]bool{}
	subjects := map[string]bool{}
	unlinked := 0
	for _, u := range packet.Users {
		if e = uniqueLegacyID(users, u.ID, "user"); e != nil {
			return p, e
		}
		name := strings.ToLower(u.Username)
		if !usernamePattern.MatchString(name) || names[name] || strings.TrimSpace(u.DisplayName) == "" || utf8.RuneCountInString(u.DisplayName) > 100 || utf8.RuneCountInString(u.Bio) > 2000 || len(u.Phone) > 40 {
			return p, fmt.Errorf("invalid or duplicate legacy user profile %s", u.ID)
		}
		names[name] = true
		if u.Role != "owner" && u.Role != "author" && u.Role != "reader" {
			return p, fmt.Errorf("explicit owner/author/reader mapping required for user %s", u.ID)
		}
		if u.GitHubSubject != "" {
			if !positiveID(u.GitHubSubject) || subjects[u.GitHubSubject] {
				return p, fmt.Errorf("GitHub subject must be a unique positive numeric ID")
			}
			subjects[u.GitHubSubject] = true
		} else {
			unlinked++
			if u.Role == "owner" && !u.Disabled {
				return p, fmt.Errorf("active owner %s requires an explicit GitHub numeric subject to avoid lockout", u.ID)
			}
		}
		report.IDMappings["users"][u.ID] = u.ID
	}
	report.Counts["unlinked_users"] = unlinked
	categories := map[string]bool{}
	tags := map[string]bool{}
	for _, group := range []struct {
		Name  string
		Items []LegacyTaxonomy
		Seen  map[string]bool
	}{{"categories", packet.Categories, categories}, {"tags", packet.Tags, tags}} {
		labels := map[string]bool{}
		for _, t := range group.Items {
			if e = uniqueLegacyID(group.Seen, t.ID, group.Name); e != nil {
				return p, e
			}
			if strings.TrimSpace(t.Name) == "" || utf8.RuneCountInString(t.Name) > 100 || labels[t.Name] || utf8.RuneCountInString(t.Description) > 1000 {
				return p, fmt.Errorf("invalid taxonomy %s/%s", group.Name, t.ID)
			}
			labels[t.Name] = true
			report.IDMappings[group.Name][t.ID] = t.ID
		}
	}
	mediaIDs := map[string]bool{}
	aliases := map[string]string{}
	for _, m := range packet.Media {
		if m.ID == "" || mediaIDs[m.ID] || !users[m.OwnerID] {
			return p, fmt.Errorf("invalid media identity/owner %s", m.ID)
		}
		mediaIDs[m.ID] = true
		src, e := packageAsset(root, m.Path)
		if e != nil {
			return p, e
		}
		hash, size, e := fileHash(src)
		if e != nil || size <= 0 || size > 10<<20 {
			return p, fmt.Errorf("media %s must be a regular image of 1..10MiB", m.ID)
		}
		if m.SHA256 != "" && m.SHA256 != hash {
			return p, fmt.Errorf("legacy media %s checksum mismatch", m.ID)
		}
		imageFile, e := os.Open(src)
		if e != nil {
			return p, e
		}
		header := make([]byte, 512)
		n, _ := imageFile.Read(header)
		mime := http.DetectContentType(header[:n])
		imageFile.Seek(0, 0)
		cfg, _, imageErr := image.DecodeConfig(imageFile)
		imageFile.Close()
		if imageErr != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 40_000_000 || (mime != "image/png" && mime != "image/jpeg" && mime != "image/gif" && mime != "image/webp") {
			return p, fmt.Errorf("legacy media %s must be a supported safe raster image", m.ID)
		}
		if m.MIMEType != "" && m.MIMEType != mime {
			return p, fmt.Errorf("legacy media %s MIME mismatch", m.ID)
		}
		id := hashString("legacy-media:" + packet.SourceSystem + ":" + m.ID)[:48]
		report.IDMappings["media"][m.ID] = id
		p.Media = append(p.Media, preparedLegacyMedia{Input: m, ID: id, Key: RandID(), Source: src, Hash: hash, MIME: mime, Size: size})
		for _, alias := range m.LegacyPaths {
			u, e := url.Parse(alias)
			if e != nil || u.Scheme != "" || u.Host != "" || u.Path != alias || !strings.HasPrefix(alias, "/") || strings.HasPrefix(alias, "//") || strings.ContainsAny(alias, "\\\r\n") || strings.Contains(alias, "..") {
				return p, fmt.Errorf("legacy media alias must be an internal absolute path")
			}
			if _, ok := aliases[alias]; ok {
				return p, fmt.Errorf("duplicate legacy alias")
			}
			aliases[alias] = "/media/" + id
		}
	}
	for _, u := range packet.Users {
		if u.AvatarMediaID != nil && !mediaIDs[*u.AvatarMediaID] {
			return p, fmt.Errorf("user avatar references unknown media")
		}
	}
	orderedAliases := sortedKeys(aliases)
	sort.SliceStable(orderedAliases, func(i, j int) bool { return len(orderedAliases[i]) > len(orderedAliases[j]) })
	replacement := []string{}
	for _, old := range orderedAliases {
		replacement = append(replacement, old, aliases[old])
	}
	rewrite := strings.NewReplacer(replacement...)
	prepareContent := func(c LegacyContent) (preparedLegacyContent, error) {
		q := preparedLegacyContent{}
		q.Draft = draftInput{Title: c.Title, Summary: c.Summary, Markdown: rewrite.Replace(c.Markdown), CategoryID: c.CategoryID, TagIDs: c.TagIDs, Cover: c.Cover, DateDisplay: c.DateDisplay, UnknownEditedAt: true}
		if c.Cover.Mode == "upload" {
			mapped, ok := report.IDMappings["media"][c.Cover.MediaID]
			if !ok {
				return q, fmt.Errorf("unknown cover media reference")
			}
			q.Draft.Cover.MediaID = mapped
		}
		if strings.TrimSpace(c.Title) == "" {
			return q, fmt.Errorf("import revision title cannot be empty")
		}
		if c.CategoryID != nil && !categories[*c.CategoryID] {
			return q, fmt.Errorf("unknown category")
		}
		for _, id := range c.TagIDs {
			if !tags[id] {
				return q, fmt.Errorf("unknown tag")
			}
		}
		if c.MediaIDs != nil {
			q.Draft.MediaIDs = []string{}
			for _, id := range c.MediaIDs {
				mapped, ok := report.IDMappings["media"][id]
				if !ok {
					return q, fmt.Errorf("unknown media reference")
				}
				q.Draft.MediaIDs = append(q.Draft.MediaIDs, mapped)
			}
		}
		r, e := validateDraft(&q.Draft)
		q.Rendered = r
		if e == nil {
			valid := map[string]bool{}
			for _, mapped := range report.IDMappings["media"] {
				valid[mapped] = true
			}
			for _, id := range r.MediaIDs {
				if !valid[id] {
					return q, fmt.Errorf("canonical image reference is outside legacy package")
				}
			}
		}
		return q, e
	}
	posts := map[string]bool{}
	for _, post := range packet.Posts {
		if e = uniqueLegacyID(posts, post.ID, "post"); e != nil {
			return p, e
		}
		if !users[post.AuthorID] || post.LegacyReadCount < 0 {
			return p, fmt.Errorf("invalid post author/counter")
		}
		prepared := preparedLegacyPost{Input: post}
		prepared.Draft, e = prepareContent(post.Draft)
		if e != nil {
			return p, fmt.Errorf("post %s draft: %w", post.ID, e)
		}
		if post.Publication != nil {
			if post.PublishedAtMS == nil || *post.PublishedAtMS <= 0 {
				return p, fmt.Errorf("post %s publication requires an explicit historical published_at_ms", post.ID)
			}
			pub, e := prepareContent(*post.Publication)
			if e != nil {
				return p, fmt.Errorf("post %s publication: %w", post.ID, e)
			}
			prepared.Publication = &pub
		}
		p.Posts = append(p.Posts, prepared)
		report.IDMappings["posts"][post.ID] = post.ID
	}
	comments := map[string]bool{}
	for _, c := range packet.Comments {
		if e = uniqueLegacyID(comments, c.ID, "comment"); e != nil {
			return p, e
		}
		if !users[c.UserID] || !posts[c.PostID] || utf8.RuneCountInString(c.Body) < 1 || utf8.RuneCountInString(c.Body) > 5000 || (c.State != "visible" && c.State != "hidden" && c.State != "deleted") {
			return p, fmt.Errorf("invalid comment %s", c.ID)
		}
		report.IDMappings["comments"][c.ID] = c.ID
	}
	for _, group := range []struct {
		Name  string
		Items []LegacyPostRelation
	}{{"post_likes", packet.PostLikes}, {"bookmarks", packet.Bookmarks}, {"history", packet.History}} {
		seen := map[string]bool{}
		for _, r := range group.Items {
			k := r.UserID + "/" + r.PostID
			if !users[r.UserID] || !posts[r.PostID] || seen[k] {
				return p, fmt.Errorf("invalid or duplicate %s relationship", group.Name)
			}
			seen[k] = true
		}
	}
	seen := map[string]bool{}
	for _, r := range packet.CommentLikes {
		k := r.UserID + "/" + r.CommentID
		if !users[r.UserID] || !comments[r.CommentID] || seen[k] {
			return p, fmt.Errorf("invalid or duplicate comment like")
		}
		seen[k] = true
	}
	seen = map[string]bool{}
	for _, r := range packet.Follows {
		k := r.FollowerID + "/" + r.FollowedID
		if !users[r.FollowerID] || !users[r.FollowedID] || r.FollowerID == r.FollowedID || seen[k] {
			return p, fmt.Errorf("invalid or duplicate follow")
		}
		seen[k] = true
	}
	seen = map[string]bool{}
	for _, r := range packet.LegacyCounters {
		if r.SourceKey == "" || seen[r.SourceKey] || seen["day:"+r.Day] || r.Count < 0 || !validLegacyDay(r.Day) {
			return p, fmt.Errorf("invalid historical daily visitor estimate")
		}
		seen[r.SourceKey] = true
		seen["day:"+r.Day] = true
	}
	if s := packet.SiteProfile; s != nil {
		if strings.TrimSpace(s.DisplayName) == "" || utf8.RuneCountInString(s.DisplayName) > 100 || len(s.AboutMarkdown) > 1<<20 || len(s.SocialLinks) > 20 {
			return p, fmt.Errorf("invalid site profile")
		}
		if s.AvatarMediaID != nil && !mediaIDs[*s.AvatarMediaID] {
			return p, fmt.Errorf("unknown site avatar")
		}
		for _, social := range s.SocialLinks {
			u, e := url.Parse(social.URL)
			if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
				return p, fmt.Errorf("invalid site social link")
			}
		}
	}
	counts := map[string]int{"users": len(packet.Users), "media": len(packet.Media), "categories": len(packet.Categories), "tags": len(packet.Tags), "posts": len(packet.Posts), "comments": len(packet.Comments), "post_likes": len(packet.PostLikes), "comment_likes": len(packet.CommentLikes), "bookmarks": len(packet.Bookmarks), "follows": len(packet.Follows), "history": len(packet.History), "legacy_counters": len(packet.LegacyCounters)}
	for k, v := range counts {
		report.Counts[k] = v
	}
	p.Report = report
	return p, nil
}
func validLegacyDay(s string) bool {
	if len(s) != 10 {
		return false
	}
	_, e := timeParseDay(s)
	return e == nil
}
func timeParseDay(s string) (int64, error) {
	t, e := time.Parse("2006-01-02", s)
	return t.UnixMilli(), e
}

// ImportLegacy only targets an empty business database. Its media journal records
// copied objects before the DB commit. On ordinary failure those files are removed;
// after a process/power crash operators inspect the journal and DB before cleanup.
// There is no claim of filesystem/DB distributed atomicity or automatic purge.
func (a *App) ImportLegacy(ctx context.Context, path string, dryRun bool) (report ImportReport, err error) {
	prepared, e := prepareLegacy(path)
	if e != nil {
		return report, e
	}
	report = prepared.Report
	report.DryRun = dryRun
	tx, e := a.maintenance(ctx)
	if e != nil {
		return report, e
	}
	defer tx.Rollback(context.Background())
	var existing int
	if e = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM users)+(SELECT count(*) FROM posts)+(SELECT count(*) FROM media_assets)+(SELECT count(*) FROM categories)+(SELECT count(*) FROM tags)+(SELECT count(*) FROM comments)+(SELECT count(*) FROM legacy_counters)`).Scan(&existing); e != nil {
		return report, e
	}
	if existing != 0 {
		return report, fmt.Errorf("legacy import requires an empty business database; use a fresh deployment or explicit rollback")
	}
	if dryRun {
		return report, nil
	}
	journalDir := filepath.Join(a.Config.MediaDir, "imports")
	if e = os.MkdirAll(journalDir, 0700); e != nil {
		return report, e
	}
	journal := filepath.Join(journalDir, "import-"+RandID()+".json")
	report.JournalPath = journal
	objects := []BackupObject{}
	for _, m := range prepared.Media {
		objects = append(objects, BackupObject{ID: m.ID, StorageKey: m.Key, SHA256: m.Hash, Bytes: m.Size})
	}
	if e = writePrivateJSON(journal, map[string]any{"version": 1, "batch_id": report.BatchID, "package_sha256": report.PackageSHA256, "state": "pending_database_commit", "objects": objects}); e != nil {
		return report, e
	}
	if e = syncDir(journalDir); e != nil {
		return report, e
	}
	copied := []string{}
	committed := false
	defer func() {
		if !committed {
			for _, file := range copied {
				_ = os.Remove(file)
			}
		}
	}()
	for _, m := range prepared.Media {
		dest := filepath.Join(a.Config.MediaDir, "objects", m.Key)
		if e = copyVerified(m.Source, dest, m.Hash, m.Size); e != nil {
			return report, e
		}
		copied = append(copied, dest)
	}
	if len(copied) > 0 {
		if e = syncDir(filepath.Join(a.Config.MediaDir, "objects")); e != nil {
			return report, e
		}
	}
	now := NowMillis()
	packet := prepared.Packet
	for _, u := range packet.Users {
		var disabled *int64
		if u.Disabled {
			at := legacyAt(u.UpdatedAtMS)
			disabled = &at
		}
		_, e = tx.Exec(ctx, "INSERT INTO users(id,username,display_name,email,phone,bio,role,disabled_at_ms,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)", u.ID, strings.ToLower(u.Username), u.DisplayName, u.Email, u.Phone, u.Bio, u.Role, disabled, legacyAt(u.CreatedAtMS), legacyAt(u.UpdatedAtMS))
		if e != nil {
			return report, e
		}
		if u.GitHubSubject != "" {
			_, e = tx.Exec(ctx, "INSERT INTO oauth_accounts(provider,issuer,subject,user_id,provider_login,created_at_ms,updated_at_ms) VALUES('github',$1,$2,$3,$4,$5,$5)", githubIssuer, u.GitHubSubject, u.ID, u.Username, now)
			if e != nil {
				return report, e
			}
		}
	}
	for _, m := range prepared.Media {
		_, e = tx.Exec(ctx, "INSERT INTO media_assets(id,owner_user_id,storage_key,original_name,mime_type,byte_size,sha256,state,created_at_ms,updated_at_ms,ready_at_ms) VALUES($1,$2,$3,$4,$5,$6,$7,'ready',$8,$9,$9)", m.ID, m.Input.OwnerID, m.Key, filepath.Base(m.Input.OriginalName), m.MIME, m.Size, m.Hash, legacyAt(m.Input.CreatedAtMS), now)
		if e != nil {
			return report, e
		}
		for _, alias := range m.Input.LegacyPaths {
			if _, e = tx.Exec(ctx, "INSERT INTO media_aliases(legacy_path,media_id) VALUES($1,$2)", alias, m.ID); e != nil {
				return report, e
			}
		}
	}
	for _, u := range packet.Users {
		if u.AvatarMediaID != nil {
			if _, e = tx.Exec(ctx, "UPDATE users SET avatar_media_id=$2 WHERE id=$1", u.ID, report.IDMappings["media"][*u.AvatarMediaID]); e != nil {
				return report, e
			}
		}
	}
	for _, group := range []struct {
		Name  string
		Items []LegacyTaxonomy
	}{{"categories", packet.Categories}, {"tags", packet.Tags}} {
		for _, t := range group.Items {
			_, e = tx.Exec(ctx, "INSERT INTO "+group.Name+"(id,name,description,created_at_ms,updated_at_ms,archived_at_ms) VALUES($1,$2,$3,$4::bigint,$4::bigint,CASE WHEN $5::boolean THEN $4::bigint ELSE NULL END)", t.ID, t.Name, t.Description, legacyAt(t.CreatedAtMS), t.Archived)
			if e != nil {
				return report, e
			}
		}
	}
	if _, e = tx.Exec(ctx, `SELECT setval(pg_get_serial_sequence('tags','id'),GREATEST(COALESCE((SELECT MAX(id) FROM tags),0),1),(SELECT EXISTS(SELECT 1 FROM tags))); SELECT painter_sync_unified_tags()`); e != nil {
		return report, e
	}
	for _, post := range prepared.Posts {
		if e = contentCanonicalize(ctx, tx, &post.Draft.Draft); e != nil {
			return report, e
		}
		if post.Publication != nil {
			if e = contentCanonicalize(ctx, tx, &post.Publication.Draft); e != nil {
				return report, e
			}
		}
		source := post.Draft
		if post.Publication != nil {
			source = *post.Publication
		}
		old := post.Input
		_, e = tx.Exec(ctx, "INSERT INTO posts(id,author_user_id,client_draft_id,first_published_at_ms,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$4,$5,$6)", old.ID, old.AuthorID, "legacy:"+old.ID, old.PublishedAtMS, legacyAt(old.CreatedAtMS), legacyAt(old.UpdatedAtMS))
		if e != nil {
			return report, e
		}
		if e = legacySaveDraft(ctx, tx, old.ID, old.AuthorID, source.Draft, 1, legacyAt(old.UpdatedAtMS)); e != nil {
			return report, e
		}
		rid, e := contentMakeSnapshot(ctx, tx, Principal{ID: old.AuthorID, Role: "owner"}, old.ID, "import", source.Draft, source.Rendered, 1)
		if e != nil {
			return report, e
		}
		if post.Publication != nil && old.DeletedAtMS == nil {
			if _, e = tx.Exec(ctx, "INSERT INTO post_publications(post_id,revision_id,published_by,published_at_ms) VALUES($1,$2,$3,$4)", old.ID, rid, old.AuthorID, *old.PublishedAtMS); e != nil {
				return report, e
			}
		}
		if post.Publication != nil && draftContentHash(source.Draft) != draftContentHash(post.Draft.Draft) {
			if _, e = tx.Exec(ctx, "UPDATE posts SET version=version+1 WHERE id=$1", old.ID); e != nil {
				return report, e
			}
			if e = legacySaveDraft(ctx, tx, old.ID, old.AuthorID, post.Draft.Draft, 2, legacyAt(old.UpdatedAtMS)); e != nil {
				return report, e
			}
			if _, e = contentMakeSnapshot(ctx, tx, Principal{ID: old.AuthorID, Role: "owner"}, old.ID, "import", post.Draft.Draft, post.Draft.Rendered, 2); e != nil {
				return report, e
			}
		}
		if old.DeletedAtMS != nil {
			if _, e = tx.Exec(ctx, "UPDATE posts SET deleted_at_ms=$2,version=version+1 WHERE id=$1", old.ID, *old.DeletedAtMS); e != nil {
				return report, e
			}
		}
		if _, e = tx.Exec(ctx, "INSERT INTO legacy_counters(source_system,source_key,metric,post_id,count_value,import_batch,imported_at_ms) VALUES($1,$2,'article_read_count',$3,$4,$5,$6)", packet.SourceSystem, old.ID, old.ID, old.LegacyReadCount, packet.BatchID, now); e != nil {
			return report, e
		}
	}
	for _, c := range packet.Comments {
		if _, e = tx.Exec(ctx, "INSERT INTO comments(id,post_id,user_id,body,state,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$4,$5,$6,$7)", c.ID, c.PostID, c.UserID, c.Body, c.State, legacyAt(c.CreatedAtMS), legacyAt(c.UpdatedAtMS)); e != nil {
			return report, e
		}
	}
	for _, group := range []struct {
		Name  string
		Items []LegacyPostRelation
	}{{"post_likes", packet.PostLikes}, {"bookmarks", packet.Bookmarks}, {"reading_history", packet.History}} {
		for _, r := range group.Items {
			timeColumn := "created_at_ms"
			if group.Name == "reading_history" {
				timeColumn = "last_read_at_ms"
			}
			if _, e = tx.Exec(ctx, "INSERT INTO "+group.Name+"(post_id,user_id,"+timeColumn+") VALUES($1,$2,$3)", r.PostID, r.UserID, legacyAt(r.AtMS)); e != nil {
				return report, e
			}
		}
	}
	for _, r := range packet.CommentLikes {
		if _, e = tx.Exec(ctx, "INSERT INTO comment_likes(comment_id,user_id,created_at_ms) VALUES($1,$2,$3)", r.CommentID, r.UserID, legacyAt(r.AtMS)); e != nil {
			return report, e
		}
	}
	for _, r := range packet.Follows {
		if _, e = tx.Exec(ctx, "INSERT INTO follows(follower_user_id,followed_user_id,created_at_ms) VALUES($1,$2,$3)", r.FollowerID, r.FollowedID, legacyAt(r.AtMS)); e != nil {
			return report, e
		}
	}
	for _, group := range []struct {
		Name  string
		Items []LegacyTaxonomy
	}{{"categories", packet.Categories}, {"tags", packet.Tags}} {
		for _, t := range group.Items {
			if t.Archived {
				if _, e = tx.Exec(ctx, "UPDATE "+group.Name+" SET archived_at_ms=$2 WHERE id=$1", t.ID, now); e != nil {
					return report, e
				}
			}
		}
	}
	for _, r := range packet.LegacyCounters {
		if _, e = tx.Exec(ctx, "INSERT INTO legacy_counters(source_system,source_key,metric,day_key,count_value,import_batch,imported_at_ms) VALUES($1,$2,'daily_visitor_estimate',$3::date,$4,$5,$6)", packet.SourceSystem, r.SourceKey, r.Day, r.Count, packet.BatchID, now); e != nil {
			return report, e
		}
	}
	if s := packet.SiteProfile; s != nil {
		var avatar *string
		if s.AvatarMediaID != nil {
			v := report.IDMappings["media"][*s.AvatarMediaID]
			avatar = &v
		}
		social := s.SocialLinks
		if social == nil {
			social = []LegacySocial{}
		}
		raw, _ := json.Marshal(social)
		if _, e = tx.Exec(ctx, "UPDATE site_profile SET version=version+1,display_name=$1,tagline=$2,about_markdown=$3,avatar_media_id=$4,social_links_json=$5,footer_text=$6,icp_number=$7,seo_description=$8,updated_at_ms=$9 WHERE id=1", s.DisplayName, s.Tagline, s.AboutMarkdown, avatar, raw, s.FooterText, s.ICPNumber, s.SEODescription, now); e != nil {
			return report, e
		}
	}
	if s := packet.Settings; s != nil {
		if _, e = tx.Exec(ctx, "UPDATE site_settings SET version=version+1,registration_enabled=$1,comments_enabled=$2,collections_enabled=$3,follows_enabled=$4,analytics_enabled=$5,updated_at_ms=$6 WHERE id=1", s.RegistrationEnabled, s.CommentsEnabled, s.CollectionsEnabled, s.FollowsEnabled, s.AnalyticsEnabled, now); e != nil {
			return report, e
		}
	}
	for _, table := range []string{"users", "categories", "tags", "posts", "comments"} {
		if _, e = tx.Exec(ctx, "SELECT setval(pg_get_serial_sequence('"+table+"','id'),GREATEST(COALESCE((SELECT MAX(id) FROM "+table+"),0),1),(SELECT EXISTS(SELECT 1 FROM "+table+")))"); e != nil {
			return report, e
		}
	}
	if _, e = tx.Exec(ctx, "INSERT INTO audit_logs(action,target_type,target_id,request_id,reason,created_at_ms) VALUES('system.bootstrap','system',$1,$2,'offline legacy JSON import; no credential migration',$3)", packet.BatchID, RandID(), now); e != nil {
		return report, e
	}
	if e = tx.Commit(ctx); e != nil {
		return report, e
	}
	committed = true
	report.Committed = true
	// The pending journal is retained if a crash occurs between commit and this
	// separate completion marker. Inspect database IDs before removing any object.
	if e = writePrivateJSON(journal+".committed", report); e != nil {
		report.Warnings = append(report.Warnings, "Database committed, but completion journal could not be written; inspect pending journal before cleanup.")
	}
	return report, nil
}
func legacySaveDraft(ctx context.Context, tx pgx.Tx, id, author string, d draftInput, version, at int64) error {
	_, e := tx.Exec(ctx, "INSERT INTO post_drafts(post_id,draft_version,title,summary,markdown,category_id,updated_by,updated_at_ms,cover,date_display) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(post_id) DO UPDATE SET draft_version=EXCLUDED.draft_version,title=EXCLUDED.title,summary=EXCLUDED.summary,markdown=EXCLUDED.markdown,category_id=EXCLUDED.category_id,updated_by=EXCLUDED.updated_by,updated_at_ms=EXCLUDED.updated_at_ms,cover=EXCLUDED.cover,date_display=EXCLUDED.date_display", id, version, d.Title, d.Summary, d.Markdown, d.CategoryID, author, at, coverJSON(d.Cover), dateDisplayJSON(d.DateDisplay))
	if e != nil {
		return e
	}
	return contentSetRelations(ctx, tx, id, d)
}
