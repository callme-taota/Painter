package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

func intString(v int64) string { return strconv.FormatInt(v, 10) }
func positiveID(s string) bool { v, e := strconv.ParseInt(s, 10, 64); return e == nil && v > 0 }

func (a *App) userDTO(ctx context.Context, id string, private bool) (map[string]any, error) {
	var username, display, bio, avatar, role, phone string
	var email, media *string
	var version, created, updated int64
	e := a.Pool.QueryRow(ctx, "SELECT username,display_name,bio,avatar_url,role,phone,email,avatar_media_id,version,created_at_ms,updated_at_ms FROM users WHERE id=$1", id).Scan(&username, &display, &bio, &avatar, &role, &phone, &email, &media, &version, &created, &updated)
	if e != nil {
		return nil, e
	}
	if media != nil {
		avatar = "/media/" + *media
	}
	out := map[string]any{"id": id, "username": username, "display_name": display, "bio": bio, "avatar_url": avatar}
	if private {
		out["role"] = role
		out["phone"] = phone
		out["email"] = email
		out["avatar_media_id"] = media
		out["version"] = version
		out["created_at"] = TimeString(created)
		out["updated_at"] = TimeString(updated)
	}
	return out, nil
}
func (a *App) meGet(w http.ResponseWriter, r *http.Request) error {
	p, e := a.Require(r)
	if e != nil {
		return e
	}
	out, e := a.userDTO(r.Context(), p.ID, true)
	if e != nil {
		return e
	}
	w.Header().Set("ETag", `"`+intString(out["version"].(int64))+`"`)
	JSON(w, 200, out)
	return nil
}
func (a *App) userGet(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	if !positiveID(id) {
		return NotFound()
	}
	out, e := a.userDTO(r.Context(), id, false)
	if e != nil {
		return e
	}
	var posts, followers, following int64
	e = a.Pool.QueryRow(r.Context(), "SELECT (SELECT count(*) FROM public_posts WHERE author_user_id=$1),(SELECT count(*) FROM follows WHERE followed_user_id=$1),(SELECT count(*) FROM follows WHERE follower_user_id=$1)", id).Scan(&posts, &followers, &following)
	if e != nil {
		return e
	}
	out["post_count"] = posts
	out["follower_count"] = followers
	out["following_count"] = following
	JSON(w, 200, out)
	return nil
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{1,63}$`)

type profilePatch struct {
	Username      *string         `json:"username"`
	DisplayName   *string         `json:"display_name"`
	Bio           *string         `json:"bio"`
	Email         *string         `json:"email"`
	Phone         *string         `json:"phone"`
	AvatarMediaID json.RawMessage `json:"avatar_media_id"`
}

func (a *App) mePatch(w http.ResponseWriter, r *http.Request) error {
	version, e := IfMatch(r)
	if e != nil {
		return e
	}
	var in profilePatch
	if e = Decode(r, &in); e != nil {
		return e
	}
	if in.Username != nil && !usernamePattern.MatchString(*in.Username) {
		return E(400, "VALIDATION_FAILED", "用户名须为2至64位字母、数字、下划线或连字符")
	}
	if in.DisplayName != nil && (strings.TrimSpace(*in.DisplayName) == "" || utf8.RuneCountInString(*in.DisplayName) > 100) {
		return E(400, "VALIDATION_FAILED", "昵称须为1至100字符")
	}
	if in.Bio != nil && utf8.RuneCountInString(*in.Bio) > 2000 {
		return E(400, "VALIDATION_FAILED", "简介最多2000字符")
	}
	if in.Phone != nil && len(*in.Phone) > 40 {
		return E(400, "VALIDATION_FAILED", "联系电话过长")
	}
	if in.Email != nil && *in.Email != "" {
		v, err := mail.ParseAddress(*in.Email)
		if err != nil || v.Address != *in.Email || len(*in.Email) > 254 {
			return E(400, "VALIDATION_FAILED", "邮箱格式无效")
		}
	}
	// Profile is the only ordinary command modifying its actor row. Take UPDATE
	// at the initial user lock instead of upgrading two simultaneous SHARE locks.
	*r = *r.WithContext(context.WithValue(r.Context(), updateActorKey, true))
	value, e := a.Write(r, nil, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		var user, display, bio, phone, avatar string
		var email, media *string
		var actual int64
		if err := tx.QueryRow(r.Context(), "SELECT username,display_name,bio,phone,email,avatar_media_id,avatar_url,version FROM users WHERE id=$1", p.ID).Scan(&user, &display, &bio, &phone, &email, &media, &avatar, &actual); err != nil {
			return nil, err
		}
		if actual != version {
			return nil, E(409, "VERSION_CONFLICT", "资料已变化，请刷新后重试")
		}
		if in.Username != nil {
			user = strings.ToLower(*in.Username)
		}
		if in.DisplayName != nil {
			display = strings.TrimSpace(*in.DisplayName)
		}
		if in.Bio != nil {
			bio = *in.Bio
		}
		if in.Email != nil {
			if *in.Email == "" {
				email = nil
			} else {
				email = in.Email
			}
		}
		if in.Phone != nil {
			phone = *in.Phone
		}
		if len(in.AvatarMediaID) > 0 {
			var m *string
			if err := json.Unmarshal(in.AvatarMediaID, &m); err != nil {
				return nil, E(400, "VALIDATION_FAILED", "头像ID无效")
			}
			media = m
			avatar = ""
			if media != nil {
				var owner, state string
				if err := tx.QueryRow(r.Context(), "SELECT owner_user_id::text,state FROM media_assets WHERE id=$1 FOR UPDATE", *media).Scan(&owner, &state); err != nil {
					return nil, err
				}
				if state != "ready" || owner != p.ID {
					return nil, E(400, "VALIDATION_FAILED", "头像必须是自己已上传的媒体")
				}
			}
		}
		now := NowMillis()
		_, err := tx.Exec(r.Context(), "UPDATE users SET username=$2,display_name=$3,bio=$4,email=$5,phone=$6,avatar_media_id=$7,avatar_url=$8,version=version+1,updated_at_ms=$9 WHERE id=$1 AND version=$10", p.ID, user, display, bio, email, phone, media, avatar, now, version)
		if err != nil {
			return nil, err
		}
		if err = audit(r.Context(), tx, p, "user.update", "user", p.ID, ""); err != nil {
			return nil, err
		}
		if media != nil {
			avatar = "/media/" + *media
		}
		var created int64
		if err = tx.QueryRow(r.Context(), "SELECT created_at_ms FROM users WHERE id=$1", p.ID).Scan(&created); err != nil {
			return nil, err
		}
		return map[string]any{"id": p.ID, "username": user, "display_name": display, "bio": bio, "email": email, "phone": phone, "avatar_media_id": media, "avatar_url": avatar, "role": p.Role, "version": version + 1, "created_at": TimeString(created), "updated_at": TimeString(now)}, nil
	})
	if e != nil {
		return e
	}
	JSON(w, 200, value)
	return nil
}
