package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
)

type floatingControls map[string][]string

func defaultFloatingControls() floatingControls {
	return floatingControls{
		"home":    {"theme", "top", "account"},
		"article": {"theme", "top", "bottom", "like", "account"},
		"other":   {"theme", "top", "account"},
	}
}

// Missing scenes inherit defaults; an explicit empty array hides that scene's
// controls. Keep configuration data-only: no selectors, URLs or executable code.
func normalizeFloatingControls(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) > 2048 {
		return nil, E(400, "VALIDATION_FAILED", "悬浮控件配置过长")
	}
	var scenes map[string]json.RawMessage
	d := json.NewDecoder(bytes.NewReader(raw))
	if e := d.Decode(&scenes); e != nil || scenes == nil {
		return nil, E(400, "VALIDATION_FAILED", "悬浮控件需要按页面场景配置")
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return nil, E(400, "VALIDATION_FAILED", "悬浮控件配置无效")
	}
	result := defaultFloatingControls()
	allowed := map[string]bool{"theme": true, "top": true, "bottom": true, "like": true, "account": true, "create": true}
	for scene, value := range scenes {
		if _, ok := result[scene]; !ok {
			return nil, E(400, "VALIDATION_FAILED", "悬浮控件场景无效")
		}
		var controls []string
		if e := json.Unmarshal(value, &controls); e != nil || controls == nil || len(controls) > 6 {
			return nil, E(400, "VALIDATION_FAILED", "请选择该场景的悬浮控件，留空数组可隐藏")
		}
		seen := map[string]bool{}
		for _, control := range controls {
			if !allowed[control] || seen[control] || (scene != "article" && control == "like") {
				return nil, E(400, "VALIDATION_FAILED", "悬浮控件不存在、重复或不适用于该场景")
			}
			seen[control] = true
		}
		result[scene] = controls
	}
	b, e := json.Marshal(result)
	return b, e
}

func (a *App) siteExperience(ctx context.Context) (bool, floatingControls, error) {
	var rss bool
	var raw []byte
	e := a.Pool.QueryRow(ctx, `SELECT rss_enabled,floating_controls FROM site_settings WHERE id=1`).Scan(&rss, &raw)
	if e != nil {
		return false, nil, e
	}
	var controls floatingControls
	if e = json.Unmarshal(raw, &controls); e != nil {
		return false, nil, e
	}
	return rss, controls, nil
}
