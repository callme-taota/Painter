package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	_ "golang.org/x/image/webp"
)

func (a *App) RegisterMedia() {
	a.Handle("POST /api/v1/media", a.mediaUpload)
	a.Handle("GET /api/v1/me/media", a.mediaList)
	a.Handle("GET /api/v1/media/{id}/status", a.mediaStatus)
	a.Handle("DELETE /api/v1/media/{id}", a.mediaDelete)
	a.Handle("GET /media/{id}", a.mediaRead)
}
func (a *App) mediaPath(key string) string { return filepath.Join(a.Config.MediaDir, "objects", key) }
func mediaRefsSQL() string {
	return `EXISTS(SELECT 1 FROM post_draft_media_refs WHERE media_id=m.id) OR EXISTS(SELECT 1 FROM post_revision_media_refs WHERE media_id=m.id) OR EXISTS(SELECT 1 FROM users WHERE avatar_media_id=m.id) OR EXISTS(SELECT 1 FROM site_profile WHERE avatar_media_id=m.id) OR EXISTS(SELECT 1 FROM author_page_profiles WHERE background_media_id=m.id) OR EXISTS(SELECT 1 FROM moment_images WHERE media_id=m.id) OR EXISTS(SELECT 1 FROM homepage_items WHERE media_id=m.id)`
}
func (a *App) mediaUpload(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.Require(r); e != nil {
		return e
	}
	if e := a.checkCSRF(r); e != nil {
		return e
	}
	limit := a.Config.UploadLimit
	if limit <= 0 {
		limit = 10 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit+(1<<20))
	mr, e := r.MultipartReader()
	if e != nil {
		return E(400, "VALIDATION_FAILED", "需要multipart文件")
	}
	if e = os.MkdirAll(filepath.Join(a.Config.MediaDir, "objects"), 0700); e != nil {
		return E(503, "STORAGE_UNAVAILABLE", "媒体目录不可写")
	}
	temp, e := os.CreateTemp(filepath.Join(a.Config.MediaDir, "objects"), ".upload-*")
	if e != nil {
		return E(503, "STORAGE_UNAVAILABLE", "媒体目录不可写")
	}
	tempname := temp.Name()
	defer os.Remove(tempname)
	defer temp.Close()
	var name string
	var size int64
	h := sha256.New()
	found := false
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return E(400, "VALIDATION_FAILED", "无效文件请求")
		}
		if part.FileName() == "" {
			if _, err = io.Copy(io.Discard, io.LimitReader(part, 4097)); err != nil {
				return err
			}
			part.Close()
			continue
		}
		if found {
			return E(400, "VALIDATION_FAILED", "每次只能上传一个文件")
		}
		found = true
		name = filepath.Base(part.FileName())
		if len(name) > 255 {
			return E(400, "VALIDATION_FAILED", "文件名过长")
		}
		size, err = io.Copy(io.MultiWriter(temp, h), io.LimitReader(part, limit+1))
		part.Close()
		if err != nil {
			return E(503, "STORAGE_UNAVAILABLE", "上传写入失败")
		}
		if size > limit {
			return E(413, "PAYLOAD_TOO_LARGE", "图片超过大小限制")
		}
	}
	if !found || size == 0 {
		return E(400, "VALIDATION_FAILED", "缺少图片文件")
	}
	if _, e = temp.Seek(0, 0); e != nil {
		return e
	}
	head := make([]byte, 512)
	n, _ := temp.Read(head)
	mime := http.DetectContentType(head[:n])
	if mime != "image/png" && mime != "image/jpeg" && mime != "image/gif" && mime != "image/webp" {
		return E(400, "VALIDATION_FAILED", "只支持PNG、JPEG、GIF和WebP图片")
	}
	if _, e = temp.Seek(0, 0); e != nil {
		return e
	}
	cfg, _, e := image.DecodeConfig(temp)
	if e != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 40_000_000 {
		return E(400, "VALIDATION_FAILED", "图片损坏或像素过大")
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if e = temp.Sync(); e != nil {
		return E(503, "STORAGE_UNAVAILABLE", "无法持久化媒体")
	}
	if e = temp.Close(); e != nil {
		return e
	}
	// Stable request bytes make multipart retries independent of boundary encoding.
	body, _ := json.Marshal(map[string]any{"filename": name, "mime_type": mime, "byte_size": size, "sha256": sum})
	*r = *r.WithContext(context.WithValue(r.Context(), bodyKey, body))
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	data, e := a.Write(r, nil, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		id := RandID()
		key := RandID()
		now := NowMillis()
		_, e := tx.Exec(r.Context(), `INSERT INTO media_assets(id,owner_user_id,storage_key,original_name,mime_type,byte_size,sha256,state,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$4,$5,$6,$7,'staging',$8,$8)`, id, p.ID, key, name, mime, size, sum, now)
		return map[string]any{"media_id": id}, e
	})
	if e != nil {
		return e
	}
	m, ok := data.(map[string]any)
	if !ok {
		return E(503, "STORAGE_UNAVAILABLE", "上传状态不可用")
	}
	id, _ := m["media_id"].(string)
	var key, state, expected string
	if e = a.Pool.QueryRow(r.Context(), `SELECT storage_key,state,sha256 FROM media_assets WHERE id=$1`, id).Scan(&key, &state, &expected); e != nil {
		return e
	}
	if expected != sum {
		return E(409, "IDEMPOTENCY_KEY_REUSED", "上传内容已改变")
	}
	if state == "staging" {
		if e = os.Rename(tempname, a.mediaPath(key)); e != nil {
			return E(503, "STORAGE_UNAVAILABLE", "媒体落盘失败，可重试原上传")
		}
		dir, err := os.Open(filepath.Dir(a.mediaPath(key)))
		if err != nil {
			return E(503, "STORAGE_UNAVAILABLE", "媒体目录不可用")
		}
		err = dir.Sync()
		dir.Close()
		if err != nil {
			return E(503, "STORAGE_UNAVAILABLE", "无法持久化媒体目录")
		}
		tx, err := a.Pool.Begin(r.Context())
		if err != nil {
			return err
		}
		defer tx.Rollback(r.Context())
		if _, err = tx.Exec(r.Context(), `SELECT id FROM policy_barrier WHERE id=1 FOR SHARE`); err != nil {
			return err
		}
		var current string
		if err = tx.QueryRow(r.Context(), `SELECT state FROM media_assets WHERE id=$1 FOR UPDATE`, id).Scan(&current); err != nil {
			return err
		}
		if current == "staging" {
			_, err = tx.Exec(r.Context(), `UPDATE media_assets SET state='ready',ready_at_ms=$2,updated_at_ms=$2 WHERE id=$1`, id, NowMillis())
			if err != nil {
				return err
			}
		}
		if err = tx.Commit(r.Context()); err != nil {
			return err
		}
		state = current
		if current == "staging" {
			state = "ready"
		}
	}
	if state != "ready" {
		return E(409, "MEDIA_NOT_READY", "媒体当前不可用")
	}
	JSON(w, 201, map[string]any{"media_id": id, "state": state, "private_preview_url": "/media/" + id})
	return nil
}
func (a *App) mediaList(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r)
	if e != nil {
		return e
	}
	scope := "media:" + p.ID
	c, e := a.interactionCursor(r, scope)
	if e != nil {
		return e
	}
	rows, e := a.Pool.Query(r.Context(), `SELECT m.id,m.original_name,m.mime_type,m.byte_size,m.state,m.created_at_ms,(`+mediaRefsSQL()+`) FROM media_assets m WHERE m.owner_user_id=$1 AND m.state<>'deleted' AND ($2::bigint=0 OR (m.created_at_ms,m.id)<($2,$3)) ORDER BY m.created_at_ms DESC,m.id DESC LIMIT $4`, p.ID, c.Time, c.ID, Limit(r)+1)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []map[string]any{}
	next, last := "", ""
	var lastms int64
	for rows.Next() {
		var id, name, mime, state string
		var size, ms int64
		var refs bool
		if e = rows.Scan(&id, &name, &mime, &size, &state, &ms, &refs); e != nil {
			return e
		}
		if len(items) == Limit(r) {
			next = a.interactionNext(scope, last, lastms)
			break
		}
		items = append(items, map[string]any{"media_id": id, "original_name": name, "mime_type": mime, "byte_size": size, "state": state, "created_at": interactionTime(ms), "in_use": refs, "private_preview_url": "/media/" + id})
		last = id
		lastms = ms
	}
	if e = rows.Err(); e != nil {
		return e
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, interactionPage(items, next))
	return nil
}
func (a *App) mediaStatus(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r)
	if e != nil {
		return e
	}
	var id, name, mime, state string
	var size, ms int64
	var refs bool
	e = a.Pool.QueryRow(r.Context(), `SELECT m.id,m.original_name,m.mime_type,m.byte_size,m.state,m.created_at_ms,(`+mediaRefsSQL()+`) FROM media_assets m WHERE m.id=$1 AND (m.owner_user_id=$2 OR $3='owner')`, r.PathValue("id"), p.ID, p.Role).Scan(&id, &name, &mime, &size, &state, &ms, &refs)
	if e == pgx.ErrNoRows {
		return NotFound()
	}
	if e != nil {
		return e
	}
	w.Header().Set("Cache-Control", "private, no-store")
	JSON(w, 200, map[string]any{"media_id": id, "original_name": name, "mime_type": mime, "byte_size": size, "state": state, "in_use": refs, "created_at": interactionTime(ms), "private_preview_url": "/media/" + id})
	return nil
}
func (a *App) mediaDelete(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	_, e := a.Write(r, nil, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		var owner, state string
		if e := tx.QueryRow(r.Context(), `SELECT owner_user_id::text,state FROM media_assets WHERE id=$1 FOR UPDATE`, id).Scan(&owner, &state); e == pgx.ErrNoRows {
			return nil, NotFound()
		} else if e != nil {
			return nil, e
		}
		if p.ID != owner && p.Role != "owner" {
			return nil, NotFound()
		}
		if state == "deleted" || state == "deleting" {
			return nil, nil
		}
		var refs bool
		if e := tx.QueryRow(r.Context(), `SELECT (`+mediaRefsSQL()+`) FROM media_assets m WHERE id=$1`, id).Scan(&refs); e != nil {
			return nil, e
		}
		if refs {
			return nil, E(409, "IN_USE", "媒体仍被文章、动态、首页、头像或作者主页引用")
		}
		if _, e := tx.Exec(r.Context(), `UPDATE media_assets SET state='deleting',updated_at_ms=$2 WHERE id=$1`, id, NowMillis()); e != nil {
			return nil, e
		}
		return nil, interactionAudit(tx, r, p, "media.delete", "media", id, "")
	})
	if e != nil {
		return e
	}
	if e = a.mediaFinishDelete(r.Context(), id); e != nil {
		return e
	}
	w.WriteHeader(204)
	return nil
}
func (a *App) mediaFinishDelete(ctx context.Context, id string) error {
	var key, state string
	e := a.Pool.QueryRow(ctx, `SELECT storage_key,state FROM media_assets WHERE id=$1`, id).Scan(&key, &state)
	if e != nil {
		return e
	}
	if state == "deleted" {
		return nil
	}
	if state != "deleting" {
		return nil
	}
	e = os.Remove(a.mediaPath(key))
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return E(503, "STORAGE_UNAVAILABLE", "删除媒体失败，将由后台重试")
	}
	return a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE media_assets SET state='deleted',updated_at_ms=$2 WHERE id=$1 AND state='deleting'`, id, NowMillis())
		return err
	})
}
func (a *App) mediaRead(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	p, _ := a.Principal(r)
	uid := p.ID
	if uid == "" {
		uid = "0"
	}
	var key, mime string
	var created int64
	var permitted bool
	e := a.Pool.QueryRow(r.Context(), `SELECT m.storage_key,m.mime_type,m.created_at_ms, (EXISTS(SELECT 1 FROM post_revision_media_refs x JOIN public_posts pp ON pp.revision_id=x.revision_id WHERE x.media_id=m.id) OR EXISTS(SELECT 1 FROM moment_images x JOIN public_moments mm ON mm.id=x.moment_id WHERE x.media_id=m.id AND x.scope='public') OR EXISTS(SELECT 1 FROM homepage_items hi CROSS JOIN homepage_settings hs WHERE hi.media_id=m.id AND hi.visible AND hs.id=1 AND (hs.config->'sections'->hi.kind->>'enabled')::boolean AND (hi.kind='steam' OR (hs.config->'sections'->hi.kind->>'limit')::integer>0)) OR EXISTS(SELECT 1 FROM users WHERE avatar_media_id=m.id AND disabled_at_ms IS NULL) OR EXISTS(SELECT 1 FROM site_profile WHERE avatar_media_id=m.id) OR EXISTS(SELECT 1 FROM author_page_profiles ap JOIN users au ON au.id=ap.user_id WHERE ap.background_media_id=m.id AND au.disabled_at_ms IS NULL) OR m.owner_user_id=$2 OR $3='owner' OR EXISTS(SELECT 1 FROM moment_images x JOIN moments mm ON mm.id=x.moment_id WHERE x.media_id=m.id AND mm.author_user_id=$2) OR EXISTS(SELECT 1 FROM post_draft_media_refs x JOIN posts p ON p.id=x.post_id WHERE x.media_id=m.id AND p.author_user_id=$2) OR EXISTS(SELECT 1 FROM post_revision_media_refs x JOIN post_revisions rr ON rr.id=x.revision_id JOIN posts p ON p.id=rr.post_id WHERE x.media_id=m.id AND p.author_user_id=$2)) FROM media_assets m WHERE m.id=$1 AND m.state='ready'`, id, uid, p.Role).Scan(&key, &mime, &created, &permitted)
	if e == pgx.ErrNoRows {
		return NotFound()
	}
	if e != nil {
		return e
	}
	if !permitted {
		return NotFound()
	}
	f, e := os.Open(a.mediaPath(key))
	if errors.Is(e, os.ErrNotExist) {
		return E(503, "STORAGE_UNAVAILABLE", "媒体文件暂不可用")
	}
	if e != nil {
		return e
	}
	defer f.Close()
	stat, e := f.Stat()
	if e != nil {
		return e
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "image", time.UnixMilli(created), io.NewSectionReader(f, 0, stat.Size()))
	return nil
}

// ReconcileMedia is intentionally conservative: every historical revision protects its media.
func (a *App) ReconcileMedia(ctx context.Context) error {
	rows, e := a.Pool.Query(ctx, `SELECT id FROM media_assets WHERE state='deleting' OR (state='staging' AND created_at_ms<$2) OR (state IN('ready','failed') AND created_at_ms<$1) ORDER BY id LIMIT 50`, NowMillis()-7*24*60*60*1000, NowMillis()-60*60*1000)
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	rows.Close()
	if e = rows.Err(); e != nil {
		return e
	}
	for _, id := range ids {
		if recovered, e := a.mediaRecoverStaging(ctx, id); e != nil {
			return e
		} else if recovered {
			continue
		}
		tx, err := a.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `SELECT id FROM policy_barrier WHERE id=1 FOR SHARE`); err != nil {
			tx.Rollback(ctx)
			return err
		}
		var state string
		err = tx.QueryRow(ctx, `SELECT state FROM media_assets WHERE id=$1 FOR UPDATE`, id).Scan(&state)
		if err == nil && state != "deleted" && state != "deleting" {
			var refs bool
			err = tx.QueryRow(ctx, `SELECT (`+mediaRefsSQL()+`) FROM media_assets m WHERE id=$1`, id).Scan(&refs)
			if err == nil && !refs {
				_, err = tx.Exec(ctx, `UPDATE media_assets SET state='deleting',updated_at_ms=$2 WHERE id=$1`, id, NowMillis())
				state = "deleting"
			}
		}
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		if state == "deleting" {
			if err = a.mediaFinishDelete(ctx, id); err != nil {
				return err
			}
		}
	}
	return nil
}

// Recover a process crash after rename but before the ready commit. File I/O is
// outside transactions; the locked state is checked again before committing.
func (a *App) mediaRecoverStaging(ctx context.Context, id string) (bool, error) {
	var key, state string
	var expected *string
	var size int64
	if e := a.Pool.QueryRow(ctx, `SELECT storage_key,state,sha256,byte_size FROM media_assets WHERE id=$1`, id).Scan(&key, &state, &expected, &size); e != nil {
		return false, e
	}
	if state != "staging" {
		return false, nil
	}
	verified := false
	f, e := os.Open(a.mediaPath(key))
	if e == nil {
		h := sha256.New()
		n, readerr := io.Copy(h, io.LimitReader(f, size+1))
		syncerr := f.Sync()
		f.Close()
		verified = readerr == nil && syncerr == nil && n == size && expected != nil && hex.EncodeToString(h.Sum(nil)) == *expected
		if verified {
			d, err := os.Open(filepath.Dir(a.mediaPath(key)))
			if err != nil {
				return true, err
			}
			err = d.Sync()
			d.Close()
			if err != nil {
				return true, err
			}
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return true, e
	}
	e = a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		var current string
		if err := tx.QueryRow(ctx, `SELECT state FROM media_assets WHERE id=$1 FOR UPDATE`, id).Scan(&current); err != nil {
			return err
		}
		if current != "staging" {
			return nil
		}
		target := "failed"
		var ready *int64
		if verified {
			target = "ready"
			now := NowMillis()
			ready = &now
		}
		_, err := tx.Exec(ctx, `UPDATE media_assets SET state=$2,ready_at_ms=$3,updated_at_ms=$4 WHERE id=$1`, id, target, ready, NowMillis())
		return err
	})
	return true, e
}
