package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// MomentTextLimit counts Unicode code points, including spaces and newlines.
// Keep the moments migration and the browser's counter in sync with this rule.
const MomentTextLimit = 500
const MomentImageLimit = 9
const momentAltLimit = 200
const momentPageLimit = 50
const momentBodyLimit = 64 << 10

type momentImage struct {
	MediaID string `json:"media_id"`
	Alt     string `json:"alt"`
}
type momentInput struct {
	Text   string
	Images []momentImage // nil means omitted; an explicit [] stays non-nil.
}

func momentImagesJSON(scope string) string {
	return `COALESCE((SELECT jsonb_agg(jsonb_build_object('media_id',mi.media_id,'url','/media/'||mi.media_id,'alt',mi.alt) ORDER BY mi.position)
 FROM moment_images mi WHERE mi.moment_id=m.id AND mi.scope='` + scope + `'),'[]'::jsonb)`
}

const momentAuthorJSON = `jsonb_build_object('id',u.id::text,'username',u.username,'display_name',u.display_name,
 'avatar_url',CASE WHEN u.avatar_media_id IS NULL THEN '' ELSE '/media/'||u.avatar_media_id END)`

var publicMomentJSON = `jsonb_build_object('id',m.id::text,'text',m.text,'images',` + momentImagesJSON("public") + `,'author',` + momentAuthorJSON + `,
 'first_published_at',` + contentTimeSQL("m.first_published_at_ms") + `,'published_at',` + contentTimeSQL("m.published_at_ms") + `)`

const publicMomentFrom = ` FROM public_moments m JOIN users u ON u.id=m.author_user_id `

var privateMomentJSON = `jsonb_build_object('id',m.id::text,'version',m.version,'text',m.draft_text,'public_text',m.public_text,
 'images',` + momentImagesJSON("draft") + `,'public_images',` + momentImagesJSON("public") + `,
 'author',` + momentAuthorJSON + `,'published',m.public_text IS NOT NULL AND m.deleted_at_ms IS NULL,
 'status',CASE WHEN m.deleted_at_ms IS NOT NULL THEN 'deleted' WHEN m.public_text IS NOT NULL THEN 'published' ELSE 'draft' END,
 'has_unpublished_changes',m.public_text IS DISTINCT FROM m.draft_text OR ` + momentImagesJSON("draft") + ` IS DISTINCT FROM ` + momentImagesJSON("public") + `,
 'created_at',` + contentTimeSQL("m.created_at_ms") + `,'updated_at',` + contentTimeSQL("m.updated_at_ms") + `,
 'first_published_at',` + contentTimeSQL("m.first_published_at_ms") + `,'published_at',` + contentTimeSQL("m.published_at_ms") + `,
 'deleted_at',` + contentTimeSQL("m.deleted_at_ms") + `,'max_text_length',` + strconv.Itoa(MomentTextLimit) + `)`

const privateMomentFrom = ` FROM moments m JOIN users u ON u.id=m.author_user_id `

func (a *App) RegisterMoments() {
	a.Handle("GET /api/v1/moments", func(w http.ResponseWriter, r *http.Request) error {
		value, err := a.PublicMoments(r.Context(), r.URL.Query())
		if err == nil {
			JSON(w, 200, value)
		}
		return err
	})
	a.Handle("GET /api/v1/moments/{id}", func(w http.ResponseWriter, r *http.Request) error {
		value, err := a.PublicMoment(r.Context(), r.PathValue("id"))
		if err == nil {
			JSON(w, 200, value)
		}
		return err
	})
	a.Handle("GET /api/v1/me/moments", a.myMoments)
	a.Handle("GET /api/v1/studio/moments/{id}", a.studioMoment)
	a.Handle("POST /api/v1/moments", a.createMoment)
	a.Handle("PUT /api/v1/studio/moments/{id}/draft", a.saveMoment)
	a.Handle("POST /api/v1/studio/moments/{id}/publish", func(w http.ResponseWriter, r *http.Request) error {
		return a.changeMoment(w, r, "publish")
	})
	a.Handle("DELETE /api/v1/studio/moments/{id}/publication", func(w http.ResponseWriter, r *http.Request) error {
		return a.changeMoment(w, r, "unpublish")
	})
	a.Handle("DELETE /api/v1/studio/moments/{id}", func(w http.ResponseWriter, r *http.Request) error {
		return a.changeMoment(w, r, "delete")
	})
}

func validateMomentText(text string) error {
	if err := validateMomentTextField(text); err != nil {
		return err
	}
	return validateMomentNonempty(text, 0)
}

func validateMomentTextField(text string) error {
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) > MomentTextLimit {
		return E(400, "VALIDATION_FAILED", fmt.Sprintf("动态最多%d个字符，空格与换行计入字数", MomentTextLimit))
	}
	if strings.ContainsFunc(text, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' }) {
		return E(400, "VALIDATION_FAILED", "动态不能包含控制字符")
	}
	return nil
}

func validateMomentNonempty(text string, images int) error {
	if strings.TrimSpace(text) == "" && images == 0 {
		return E(400, "VALIDATION_FAILED", "请填写动态内容或添加图片")
	}
	return nil
}

// Bound decoding before the shared body cache is populated. Even escaped emoji
// at the full code-point limit fit comfortably in this request budget.
func readMomentBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, momentBodyLimit)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, E(413, "PAYLOAD_TOO_LARGE", "动态请求最多64 KiB")
		}
		return nil, E(400, "INVALID_JSON", "无法读取动态请求")
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	return raw, nil
}

func decodeMoment(w http.ResponseWriter, r *http.Request) (momentInput, error) {
	var input momentInput
	if _, err := readMomentBody(w, r); err != nil {
		return input, err
	}
	var in struct {
		Text   json.RawMessage `json:"text"`
		Images json.RawMessage `json:"images"`
	}
	if err := Decode(r, &in); err != nil {
		return input, err
	}
	if in.Text != nil {
		if bytes.Equal(bytes.TrimSpace(in.Text), []byte("null")) || json.Unmarshal(in.Text, &input.Text) != nil {
			return input, E(400, "INVALID_JSON", "动态文字须为字符串")
		}
	}
	if err := validateMomentTextField(input.Text); err != nil {
		return input, err
	}
	if in.Images != nil {
		if bytes.Equal(bytes.TrimSpace(in.Images), []byte("null")) {
			return input, E(400, "VALIDATION_FAILED", "图片须为数组；移除全部图片请传空数组")
		}
		decoder := json.NewDecoder(bytes.NewReader(in.Images))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input.Images); err != nil {
			return input, E(400, "INVALID_JSON", "动态图片字段无效")
		}
		if len(input.Images) > MomentImageLimit {
			return input, E(400, "VALIDATION_FAILED", "每条动态最多9张图片")
		}
		seen := map[string]bool{}
		for _, image := range input.Images {
			if len(image.MediaID) == 0 || len(image.MediaID) > 128 || strings.ContainsFunc(image.MediaID, func(r rune) bool {
				return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
			}) || seen[image.MediaID] {
				return input, E(400, "VALIDATION_FAILED", "图片ID无效或重复")
			}
			if !utf8.ValidString(image.Alt) || utf8.RuneCountInString(image.Alt) > momentAltLimit || strings.ContainsRune(image.Alt, '\x00') {
				return input, E(400, "VALIDATION_FAILED", "图片说明最多200个字符")
			}
			seen[image.MediaID] = true
		}
	}
	return input, nil
}

func momentCommandBody(w http.ResponseWriter, r *http.Request) error {
	raw, err := readMomentBody(w, r)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var in struct{}
	return Decode(r, &in)
}

func momentLimit(q url.Values) (int, error) {
	limit := 20
	if raw := q.Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return 0, E(400, "VALIDATION_FAILED", "动态每页数量须为正整数，最多50项")
		}
		limit = min(value, momentPageLimit)
	}
	return limit, nil
}

func (a *App) momentPage(cursor, scope string) (contentListPage, error) {
	var page contentListPage
	if len(cursor) > 2048 {
		return page, E(400, "VALIDATION_FAILED", "动态分页游标无效")
	}
	if err := a.decodePage(cursor, &page); err != nil {
		return page, err
	}
	if page.Scope != scope || !positiveID(page.ID) || page.Timestamp < 0 {
		return page, E(400, "VALIDATION_FAILED", "分页条件已改变，请重新加载")
	}
	return page, nil
}

func (a *App) momentCollection(items []map[string]any, limit int, scope string, public bool) map[string]any {
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	next := ""
	if more {
		last := items[len(items)-1]
		page := contentListPage{Scope: scope, ID: last["id"].(string)}
		if public {
			// JSON numbers cannot carry arbitrary int64 precision. The SQL-only
			// cursor timestamp stays a decimal string until it is signed.
			page.Timestamp, _ = strconv.ParseInt(last["_cursor_ms"].(string), 10, 64)
		}
		next = a.encodePage(page)
	}
	for _, item := range items {
		delete(item, "_cursor_ms")
		if !public {
			a.momentLimits(item)
		}
	}
	value := map[string]any{"items": items, "next_cursor": next, "has_more": more, "limit": limit, "max_limit": momentPageLimit}
	a.momentLimits(value)
	return value
}

func (a *App) momentLimits(value map[string]any) {
	limit := a.Config.UploadLimit
	if limit <= 0 {
		limit = 10 << 20
	}
	value["max_text_length"] = MomentTextLimit
	value["max_images"] = MomentImageLimit
	value["max_image_bytes"] = limit
}

// PublicMoments is also the SSR entry point. Neither a cookie nor a scope query
// can add drafts: the database view itself contains only active snapshots.
func (a *App) PublicMoments(ctx context.Context, q url.Values) (map[string]any, error) {
	limit, err := momentLimit(q)
	if err != nil {
		return nil, err
	}
	author := q.Get("author_id")
	if author != "" && !positiveID(author) {
		return nil, E(400, "VALIDATION_FAILED", "作者ID无效")
	}
	where := " WHERE true "
	args := []any{}
	if author != "" {
		args = append(args, author)
		where += " AND m.author_user_id=$1::bigint"
	}
	scope := hashString(strings.Join([]string{"public-moments", author, strconv.Itoa(limit)}, "\x00"))
	if cursor := q.Get("cursor"); cursor != "" {
		page, err := a.momentPage(cursor, scope)
		if err != nil {
			return nil, err
		}
		args = append(args, page.Timestamp, page.ID)
		where += fmt.Sprintf(" AND (m.first_published_at_ms,m.id)<($%d::bigint,$%d::bigint)", len(args)-1, len(args))
	}
	args = append(args, limit+1)
	sql := "SELECT " + publicMomentJSON + ` || jsonb_build_object('_cursor_ms',m.first_published_at_ms::text)` + publicMomentFrom + where + fmt.Sprintf(" ORDER BY m.first_published_at_ms DESC,m.id DESC LIMIT $%d", len(args))
	items, err := contentObjects(ctx, a.Pool, sql, args...)
	if err != nil {
		return nil, err
	}
	return a.momentCollection(items, limit, scope, true), nil
}

func (a *App) PublicMoment(ctx context.Context, id string) (map[string]any, error) {
	if !positiveID(id) {
		return nil, NotFound()
	}
	return contentObject(ctx, a.Pool, "SELECT "+publicMomentJSON+publicMomentFrom+" WHERE m.id=$1", id)
}

func (a *App) privateMoment(ctx context.Context, db contentQuerier, p Principal, id string) (map[string]any, error) {
	if !positiveID(id) {
		return nil, NotFound()
	}
	value, err := contentObject(ctx, db, "SELECT "+privateMomentJSON+privateMomentFrom+" WHERE m.id=$1 AND (m.author_user_id=$2 OR $3)", id, p.ID, p.Role == "owner")
	if err == nil {
		a.momentLimits(value)
	}
	return value, err
}

func (a *App) studioMoment(w http.ResponseWriter, r *http.Request) error {
	p, err := a.Require(r, "author", "owner")
	if err != nil {
		return err
	}
	value, err := a.privateMoment(r.Context(), a.Pool, p, r.PathValue("id"))
	if err == nil {
		w.Header().Set("Cache-Control", "private, no-store")
		JSON(w, 200, value)
	}
	return err
}

func (a *App) myMoments(w http.ResponseWriter, r *http.Request) error {
	p, err := a.Require(r, "author", "owner")
	if err != nil {
		return err
	}
	q := r.URL.Query()
	scopeName := q.Get("scope")
	if scopeName == "" {
		scopeName = "mine"
	}
	if scopeName != "mine" && scopeName != "site" {
		return E(400, "VALIDATION_FAILED", "动态范围须为mine或site")
	}
	if scopeName == "site" && p.Role != "owner" {
		return E(403, "PERMISSION_DENIED", "需要站长权限")
	}
	limit, err := momentLimit(q)
	if err != nil {
		return err
	}
	status := q.Get("status")
	if status == "" {
		status = "all"
	}
	where, args := " WHERE true ", []any{}
	if scopeName == "mine" {
		args = append(args, p.ID)
		where += " AND m.author_user_id=$1"
	}
	switch status {
	case "all":
	case "draft":
		where += " AND m.deleted_at_ms IS NULL AND m.public_text IS NULL"
	case "published":
		where += " AND m.deleted_at_ms IS NULL AND m.public_text IS NOT NULL"
	case "deleted":
		where += " AND m.deleted_at_ms IS NOT NULL"
	default:
		return E(400, "VALIDATION_FAILED", "动态状态无效")
	}
	scope := hashString(strings.Join([]string{"studio-moments", p.ID, scopeName, status, strconv.Itoa(limit)}, "\x00"))
	if cursor := q.Get("cursor"); cursor != "" {
		page, err := a.momentPage(cursor, scope)
		if err != nil {
			return err
		}
		args = append(args, page.ID)
		where += fmt.Sprintf(" AND m.id<$%d::bigint", len(args))
	}
	args = append(args, limit+1)
	items, err := contentObjects(r.Context(), a.Pool, "SELECT "+privateMomentJSON+privateMomentFrom+where+fmt.Sprintf(" ORDER BY m.id DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, a.momentCollection(items, limit, scope, false))
	return nil
}

func (a *App) momentAuthorize(r *http.Request) error {
	if _, err := a.Require(r, "author", "owner"); err != nil {
		return err
	}
	return a.checkCSRF(r)
}

// Write checks roles before replaying its ledger. For foreign resources that
// check must remain owner-only, even if a former owner is now an author. The
// schema keeps author identity immutable; the locked callback rechecks access
// for ordinary mutations. This pre-read never selects private text/version.
func (a *App) momentMutationRoles(r *http.Request) ([]string, error) {
	p, err := a.Require(r, "author", "owner")
	if err != nil {
		return nil, err
	}
	if err = a.checkCSRF(r); err != nil {
		return nil, err
	}
	id := r.PathValue("id")
	if !positiveID(id) {
		return nil, NotFound()
	}
	var author string
	err = a.Pool.QueryRow(r.Context(), "SELECT author_user_id::text FROM moments WHERE id=$1", id).Scan(&author)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, NotFound()
	}
	if err != nil {
		return nil, err
	}
	if author != p.ID {
		if p.Role != "owner" {
			return nil, NotFound()
		}
		return []string{"owner"}, nil
	}
	return []string{"author", "owner"}, nil
}

// Write has already locked policy/session/actor. The content row is next in
// that same lock order, and ownership is checked before exposing its version.
func momentLock(ctx context.Context, tx pgx.Tx, p Principal, id string, expected int64) error {
	if !positiveID(id) {
		return NotFound()
	}
	var author string
	var actual int64
	var deleted bool
	err := tx.QueryRow(ctx, "SELECT author_user_id::text,version,deleted_at_ms IS NOT NULL FROM moments WHERE id=$1 FOR UPDATE", id).Scan(&author, &actual, &deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return NotFound()
	}
	if err != nil {
		return err
	}
	if (author != p.ID && p.Role != "owner") || deleted {
		return NotFound()
	}
	if actual != expected {
		return E(409, "VERSION_CONFLICT", "动态已变化，请保留本地内容并重新读取版本")
	}
	return nil
}

func (a *App) momentAck(r *http.Request, tx pgx.Tx, p Principal, id, action string) (any, error) {
	if err := audit(r.Context(), tx, p, action, "moment", id, ""); err != nil {
		return nil, err
	}
	value, err := a.privateMoment(r.Context(), tx, p, id)
	if err == nil {
		value["ack_mutation_id"] = r.Header.Get("Idempotency-Key")
	}
	return value, err
}

func momentImages(ctx context.Context, tx pgx.Tx, id, scope string) ([]momentImage, error) {
	rows, err := tx.Query(ctx, "SELECT media_id,alt FROM moment_images WHERE moment_id=$1 AND scope=$2 ORDER BY position", id, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	images := []momentImage{}
	for rows.Next() {
		var image momentImage
		if err := rows.Scan(&image.MediaID, &image.Alt); err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, rows.Err()
}

// Lock media in one stable order after the moment row. Deletion and GC take
// the same media lock and check references in a fresh statement afterwards.
func momentLockImages(ctx context.Context, tx pgx.Tx, p Principal, id string, images []momentImage) error {
	ids := make([]string, 0, len(images))
	for _, image := range images {
		ids = append(ids, image.MediaID)
	}
	sort.Strings(ids)
	for _, media := range ids {
		var owner, state, mime string
		err := tx.QueryRow(ctx, "SELECT owner_user_id::text,state,mime_type FROM media_assets WHERE id=$1 FOR UPDATE", media).Scan(&owner, &state, &mime)
		if errors.Is(err, pgx.ErrNoRows) {
			return E(409, "MEDIA_NOT_READY", "图片未完成上传或没有引用权限")
		}
		if err != nil {
			return err
		}
		allowed := owner == p.ID
		if !allowed {
			// Access to this moment was checked under its lock; this only
			// permits retention of an already attached shared image. An owner
			// cannot newly attach arbitrary other users' private uploads.
			if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM moment_images WHERE moment_id=$1 AND media_id=$2)", id, media).Scan(&allowed); err != nil {
				return err
			}
		}
		if !allowed || state != "ready" || (mime != "image/png" && mime != "image/jpeg" && mime != "image/gif" && mime != "image/webp") {
			return E(409, "MEDIA_NOT_READY", "图片未完成上传或没有引用权限")
		}
	}
	return nil
}

func momentSetImages(ctx context.Context, tx pgx.Tx, id, scope string, images []momentImage) error {
	if _, err := tx.Exec(ctx, "DELETE FROM moment_images WHERE moment_id=$1 AND scope=$2", id, scope); err != nil {
		return err
	}
	for position, image := range images {
		if _, err := tx.Exec(ctx, "INSERT INTO moment_images(moment_id,scope,position,media_id,alt) VALUES($1,$2,$3,$4,$5)", id, scope, position, image.MediaID, image.Alt); err != nil {
			return err
		}
	}
	return nil
}

// Publication already owns a transaction and media locks. Read metadata from
// that connection: borrowing Pool here could deadlock a full small pool when
// several publishers each wait for an additional connection.
func (a *App) verifyMomentImageFiles(ctx context.Context, tx pgx.Tx, images []momentImage) error {
	for _, image := range images {
		var key, expected, state string
		var size int64
		if err := tx.QueryRow(ctx, "SELECT storage_key,COALESCE(sha256,''),state,byte_size FROM media_assets WHERE id=$1", image.MediaID).Scan(&key, &expected, &state, &size); err != nil {
			return err
		}
		if state != "ready" || size < 1 || size == 1<<63-1 {
			return E(409, "MEDIA_NOT_READY", "图片当前不可用")
		}
		file, err := os.Open(a.mediaPath(key))
		if err != nil {
			return E(503, "STORAGE_UNAVAILABLE", "图片文件缺失，请恢复后再发布")
		}
		hash := sha256.New()
		n, err := io.Copy(hash, io.LimitReader(file, size+1))
		file.Close()
		if err != nil || n != size || hex.EncodeToString(hash.Sum(nil)) != expected {
			return E(503, "STORAGE_UNAVAILABLE", "图片完整性检查未通过")
		}
	}
	return nil
}

func (a *App) createMoment(w http.ResponseWriter, r *http.Request) error {
	if err := a.momentAuthorize(r); err != nil {
		return err
	}
	in, err := decodeMoment(w, r)
	if err != nil {
		return err
	}
	if err := validateMomentNonempty(in.Text, len(in.Images)); err != nil {
		return err
	}
	value, err := a.Write(r, []string{"author", "owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		var id string
		if err := tx.QueryRow(r.Context(), "INSERT INTO moments(author_user_id,draft_text,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$3) RETURNING id::text", p.ID, in.Text, NowMillis()).Scan(&id); err != nil {
			return nil, err
		}
		if err := momentLockImages(r.Context(), tx, p, id, in.Images); err != nil {
			return nil, err
		}
		if err := momentSetImages(r.Context(), tx, id, "draft", in.Images); err != nil {
			return nil, err
		}
		return a.momentAck(r, tx, p, id, "moment.create")
	})
	if err == nil {
		JSON(w, 201, value)
	}
	return err
}

func (a *App) saveMoment(w http.ResponseWriter, r *http.Request) error {
	roles, err := a.momentMutationRoles(r)
	if err != nil {
		return err
	}
	expected, err := IfMatch(r)
	if err != nil {
		return err
	}
	in, err := decodeMoment(w, r)
	if err != nil {
		return err
	}
	id := r.PathValue("id")
	value, err := a.Write(r, roles, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		if err := momentLock(r.Context(), tx, p, id, expected); err != nil {
			return nil, err
		}
		images := in.Images
		if images == nil {
			var err error
			images, err = momentImages(r.Context(), tx, id, "draft")
			if err != nil {
				return nil, err
			}
		}
		if err := validateMomentNonempty(in.Text, len(images)); err != nil {
			return nil, err
		}
		if err := momentLockImages(r.Context(), tx, p, id, images); err != nil {
			return nil, err
		}
		if in.Images != nil {
			if err := momentSetImages(r.Context(), tx, id, "draft", images); err != nil {
				return nil, err
			}
		}
		if _, err := tx.Exec(r.Context(), "UPDATE moments SET draft_text=$2,version=version+1,updated_at_ms=GREATEST(updated_at_ms,$3) WHERE id=$1", id, in.Text, NowMillis()); err != nil {
			return nil, err
		}
		return a.momentAck(r, tx, p, id, "moment.save")
	})
	if err == nil {
		JSON(w, 200, value)
	}
	return err
}

func (a *App) changeMoment(w http.ResponseWriter, r *http.Request, action string) error {
	roles, err := a.momentMutationRoles(r)
	if err != nil {
		return err
	}
	expected, err := IfMatch(r)
	if err != nil {
		return err
	}
	if err = momentCommandBody(w, r); err != nil {
		return err
	}
	id := r.PathValue("id")
	value, err := a.Write(r, roles, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		if err := momentLock(r.Context(), tx, p, id, expected); err != nil {
			return nil, err
		}
		set := ""
		switch action {
		case "publish":
			images, err := momentImages(r.Context(), tx, id, "draft")
			if err != nil {
				return nil, err
			}
			if err := momentLockImages(r.Context(), tx, p, id, images); err != nil {
				return nil, err
			}
			if err := a.verifyMomentImageFiles(r.Context(), tx, images); err != nil {
				return nil, err
			}
			if err := momentSetImages(r.Context(), tx, id, "public", images); err != nil {
				return nil, err
			}
			set = "public_text=draft_text,first_published_at_ms=COALESCE(first_published_at_ms,$2),published_at_ms=GREATEST(COALESCE(published_at_ms,0),$2)"
		case "unpublish":
			if err := momentSetImages(r.Context(), tx, id, "public", nil); err != nil {
				return nil, err
			}
			set = "public_text=NULL"
		case "delete":
			if err := momentSetImages(r.Context(), tx, id, "public", nil); err != nil {
				return nil, err
			}
			set = "public_text=NULL,deleted_at_ms=GREATEST(updated_at_ms,$2)"
		default:
			return nil, E(400, "VALIDATION_FAILED", "动态操作无效")
		}
		if _, err := tx.Exec(r.Context(), "UPDATE moments SET "+set+",version=version+1,updated_at_ms=GREATEST(updated_at_ms,$2) WHERE id=$1", id, NowMillis()); err != nil {
			return nil, err
		}
		return a.momentAck(r, tx, p, id, "moment."+action)
	})
	if err == nil {
		JSON(w, 200, value)
	}
	return err
}
