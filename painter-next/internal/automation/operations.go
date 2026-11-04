package automation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type operationArgs struct {
	ID           string                     `json:"id,omitempty"`
	Status       string                     `json:"status,omitempty"`
	Limit        int                        `json:"limit,omitempty"`
	Cursor       string                     `json:"cursor,omitempty"`
	Version      int64                      `json:"version,omitempty"`
	DraftVersion int64                      `json:"draft_version,omitempty"`
	Key          string                     `json:"idempotency_key,omitempty"`
	Draft        map[string]json.RawMessage `json:"draft,omitempty"`
	Name         string                     `json:"name,omitempty"`
	Description  string                     `json:"description,omitempty"`
}

func numericID(id string) bool {
	n, err := strconv.ParseInt(id, 10, 64)
	return err == nil && n > 0 && strconv.FormatInt(n, 10) == id
}

var draftFields = map[string]bool{"title": true, "summary": true, "markdown": true, "tag_ids": true, "media_ids": true, "cover": true, "date_display": true}

func (c *Client) execute(ctx context.Context, name string, args operationArgs, allowWrite bool) (any, error) {
	write := name == "post_create" || name == "post_save" || name == "post_publish" || name == "post_withdraw" || name == "tag_create"
	if write && !allowWrite {
		return nil, &APIError{Code: "WRITE_DISABLED"}
	}
	if write && !validKey(args.Key) {
		return nil, errors.New("idempotency_key is required (8..128 characters)")
	}
	if name == "post_get" || name == "post_status" || name == "post_save" || name == "post_publish" || name == "post_withdraw" {
		if !numericID(args.ID) {
			return nil, errors.New("id must be a positive integer string")
		}
	}
	if name == "post_save" || name == "post_publish" || name == "post_withdraw" {
		if args.Version < 1 {
			return nil, errors.New("version is required; read the current article first")
		}
	}
	if name == "post_save" || name == "post_publish" {
		if args.DraftVersion < 1 {
			return nil, errors.New("draft_version is required; read the current article first")
		}
	}
	version := strconv.FormatInt(args.Version, 10)
	path := "/studio/posts/" + args.ID
	switch name {
	case "posts_list":
		if args.Status != "" && args.Status != "all" && args.Status != "draft" && args.Status != "published" && args.Status != "trashed" {
			return nil, errors.New("invalid status filter")
		}
		if args.Limit == 0 {
			args.Limit = 20
		}
		if args.Limit < 1 || args.Limit > 100 || len(args.Cursor) > 4096 {
			return nil, errors.New("invalid pagination options")
		}
		q := url.Values{"scope": {"site"}, "status": {args.Status}, "limit": {strconv.Itoa(args.Limit)}}
		if args.Cursor != "" {
			q.Set("cursor", args.Cursor)
		}
		return c.ownerRequest(ctx, http.MethodGet, "/me/posts?"+q.Encode(), "", "", nil)
	case "post_get", "post_status":
		raw, err := c.ownerRequest(ctx, http.MethodGet, path, "", "", nil)
		if err != nil || name == "post_get" {
			return raw, err
		}
		var post map[string]json.RawMessage
		if json.Unmarshal(raw, &post) != nil {
			return nil, &APIError{Code: "INVALID_JSON_RESPONSE"}
		}
		out := map[string]json.RawMessage{}
		for _, k := range []string{"id", "title", "status", "version", "draft_version", "publication", "created_at", "updated_at", "last_edited_at"} {
			if v, ok := post[k]; ok {
				out[k] = v
			}
		}
		return out, nil
	case "tags_list":
		if args.Limit == 0 {
			args.Limit = 20
		}
		if args.Limit < 1 || args.Limit > 100 || len(args.Cursor) > 4096 {
			return nil, errors.New("invalid pagination options")
		}
		q := url.Values{"limit": {strconv.Itoa(args.Limit)}}
		if args.Cursor != "" {
			q.Set("cursor", args.Cursor)
		}
		return c.ownerRequest(ctx, http.MethodGet, "/admin/tags?"+q.Encode(), "", "", nil)
	case "post_create", "post_save":
		if len(args.Draft) == 0 {
			return nil, errors.New("draft must be a nonempty object")
		}
		for k, v := range args.Draft {
			if !draftFields[k] || len(v) == 0 || string(v) == "null" {
				return nil, errors.New("draft contains unsupported fields or null values")
			}
		}
		body := map[string]json.RawMessage{}
		if name == "post_save" {
			// A full draft snapshot makes a retry byte-for-byte reproducible.
			// Never refetch/merge after a failed mutation: that could transform the
			// same idempotency key into a different request or overwrite new edits.
			for k := range draftFields {
				if _, ok := args.Draft[k]; !ok {
					return nil, errors.New("save requires all draft fields; read and preserve the current article first")
				}
			}
			body["expected_draft_version"] = json.RawMessage(strconv.FormatInt(args.DraftVersion, 10))
		} else {
			// One stable client ID for each explicit mutation key also protects a
			// create retry after the HTTP idempotency retention window.
			sum := sha256.Sum256([]byte(args.Key))
			id, _ := json.Marshal("cli-" + hex.EncodeToString(sum[:]))
			body["client_draft_id"] = id
		}
		for k, v := range args.Draft {
			body[k] = v
		}
		if name == "post_create" {
			return c.ownerRequest(ctx, http.MethodPost, "/posts", "", args.Key, body)
		}
		return c.ownerRequest(ctx, http.MethodPut, path+"/draft", version, args.Key, body)
	case "post_publish":
		return c.ownerRequest(ctx, http.MethodPost, path+"/publish", version, args.Key, map[string]int64{"expected_draft_version": args.DraftVersion})
	case "post_withdraw":
		return c.ownerRequest(ctx, http.MethodDelete, path+"/publication", version, args.Key, nil)
	case "tag_create":
		if args.Name == "" {
			return nil, errors.New("name is required")
		}
		return c.ownerRequest(ctx, http.MethodPost, "/admin/tags", "", args.Key, map[string]string{"name": args.Name, "description": args.Description})
	default:
		return nil, fmt.Errorf("unknown operation")
	}
}
