package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const webMomentCoverHeaderLimit = 256 << 10
const webMomentCoverCacheLimit = 512
const webMomentCoverFailureTTL = 30 * time.Second

type webMomentCover struct {
	shape         string
	ratio, mobile float64
}

var webMomentCoverFallback = webMomentCover{"unknown", .8, .8}

type webMomentCoverKey struct {
	storage, hash string
	size          int64
}

type webMomentCoverEntry struct {
	cover   webMomentCover
	expires time.Time // Successful immutable media metadata never expires.
	used    uint64
}

type webMomentCoverCache struct {
	mu      sync.Mutex
	entries map[webMomentCoverKey]webMomentCoverEntry
	clock   uint64
}

func (c *webMomentCoverCache) get(key webMomentCoverKey, now time.Time) (webMomentCover, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return webMomentCover{}, false
	}
	if !entry.expires.IsZero() && !now.Before(entry.expires) {
		delete(c.entries, key)
		return webMomentCover{}, false
	}
	c.clock++
	entry.used = c.clock
	c.entries[key] = entry
	return entry.cover, true
}

func (c *webMomentCoverCache) put(key webMomentCoverKey, cover webMomentCover, success bool, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[webMomentCoverKey]webMomentCoverEntry)
	}
	if _, exists := c.entries[key]; !exists && len(c.entries) >= webMomentCoverCacheLimit {
		var oldest webMomentCoverKey
		used := ^uint64(0)
		for candidate, entry := range c.entries {
			if entry.used < used {
				oldest, used = candidate, entry.used
			}
		}
		delete(c.entries, oldest)
	}
	c.clock++
	entry := webMomentCoverEntry{cover: cover, used: c.clock}
	if !success {
		entry.expires = now.Add(webMomentCoverFailureTTL)
	}
	c.entries[key] = entry
}

func webMomentCoverDimensions(width, height int) webMomentCover {
	if width < 1 || height < 1 || width > 40_000_000/height {
		return webMomentCoverFallback
	}
	ratio := float64(width) / float64(height)
	shape := "landscape"
	switch {
	case ratio < .5:
		shape, ratio = "long", 2.0/3
	case ratio < .9:
		shape, ratio = "portrait", math.Max(2.0/3, ratio)
	case ratio <= 1.1:
		shape, ratio = "square", 1
	case ratio > 2:
		shape, ratio = "panorama", 2
	}
	return webMomentCover{shape, ratio, math.Min(ratio, 1.6)}
}

// The decoration belongs only to the HTML view. Clone the items so shared API
// DTOs and any caller caches retain their original public shape.
func (a *App) webMomentCoverItems(ctx context.Context, items []map[string]any) []map[string]any {
	out := make([]map[string]any, len(items))
	first := make(map[string][]map[string]any)
	ids := make([]string, 0, len(items))
	for i, item := range items {
		copy := make(map[string]any, len(item))
		for key, value := range item {
			copy[key] = value
		}
		out[i] = copy
		images := webMaps(item["images"])
		if len(images) == 0 {
			continue
		}
		webMomentCoverDecorate(copy, webMomentCoverFallback)
		id, ok := images[0]["media_id"].(string)
		if !ok || id == "" {
			continue
		}
		if _, exists := first[id]; !exists {
			ids = append(ids, id)
		}
		first[id] = append(first[id], copy)
	}
	if len(ids) == 0 || a.Pool == nil {
		return out
	}
	// Only server-stored keys of still-public first images may reach the disk.
	// Batch metadata once; a failed enhancement retains stable 4:5 placeholders.
	rows, err := a.Pool.Query(ctx, `SELECT a.id,a.storage_key,a.state,a.byte_size,COALESCE(a.sha256,'')
 FROM media_assets a WHERE a.id=ANY($1::text[]) AND EXISTS(
 SELECT 1 FROM moment_images mi JOIN public_moments pm ON pm.id=mi.moment_id
 WHERE mi.media_id=a.id AND mi.scope='public' AND mi.position=0)`, ids)
	if err != nil {
		return out
	}
	type storedCover struct {
		id, state string
		key       webMomentCoverKey
	}
	stored := make([]storedCover, 0, len(ids))
	for rows.Next() {
		var value storedCover
		if err = rows.Scan(&value.id, &value.key.storage, &value.state, &value.key.size, &value.key.hash); err != nil {
			rows.Close()
			return out
		}
		stored = append(stored, value)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out
	}
	for _, value := range stored {
		if value.state != "ready" {
			continue
		}
		presentation := a.webMomentCoverRead(value.key)
		for _, cover := range first[value.id] {
			webMomentCoverDecorate(cover, presentation)
		}
	}
	return out
}

func webMomentCoverDecorate(item map[string]any, cover webMomentCover) {
	item["cover_shape"] = cover.shape
	item["cover_ratio"] = cover.ratio
	item["cover_mobile_ratio"] = cover.mobile
}

func (a *App) webMomentCoverRead(key webMomentCoverKey) webMomentCover {
	now := time.Now()
	if cover, ok := a.momentCovers.get(key, now); ok {
		return cover
	}
	cover, success := a.webMomentCoverFile(key)
	a.momentCovers.put(key, cover, success, now)
	return cover
}

func (a *App) webMomentCoverFile(key webMomentCoverKey) (webMomentCover, bool) {
	if key.size < 1 || !filepath.IsLocal(key.storage) || strings.ContainsAny(key.storage, "\\\x00") || filepath.Clean(key.storage) != key.storage {
		return webMomentCoverFallback, false
	}
	root, err := os.OpenRoot(filepath.Join(a.Config.MediaDir, "objects"))
	if err != nil {
		return webMomentCoverFallback, false
	}
	defer root.Close()
	// Reject links and special files before opening; os.Root also prevents an
	// intermediate directory symlink from escaping the objects directory.
	info, err := root.Lstat(key.storage)
	if err != nil || !info.Mode().IsRegular() || info.Size() != key.size {
		return webMomentCoverFallback, false
	}
	file, err := root.Open(key.storage)
	if err != nil {
		return webMomentCoverFallback, false
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != key.size {
		return webMomentCoverFallback, false
	}
	header, err := io.ReadAll(io.LimitReader(file, webMomentCoverHeaderLimit))
	if err != nil {
		return webMomentCoverFallback, false
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(header))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 40_000_000/cfg.Height {
		return webMomentCoverFallback, false
	}
	if format == "jpeg" {
		orientation, complete := webMomentJPEGOrientation(header)
		if !complete {
			return webMomentCoverFallback, false
		}
		if orientation >= 5 && orientation <= 8 {
			cfg.Width, cfg.Height = cfg.Height, cfg.Width
		}
	}
	return webMomentCoverDimensions(cfg.Width, cfg.Height), true
}

// Read the bounded JPEG marker stream, never the compressed pixels. TIFF
// offsets are checked before slicing. A truncated marker stream or malformed
// EXIF cannot establish display dimensions and keeps the fixed placeholder.
func webMomentJPEGOrientation(data []byte) (uint16, bool) {
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		return 1, false
	}
	orientation := uint16(1)
	for at := 2; at < len(data); {
		if data[at] != 0xff {
			return 1, false
		}
		for at < len(data) && data[at] == 0xff {
			at++
		}
		if at >= len(data) {
			return 1, false
		}
		marker := data[at]
		at++
		if marker == 0xd9 {
			return 1, false
		}
		if marker == 0x01 || marker >= 0xd0 && marker <= 0xd8 {
			continue
		}
		if len(data)-at < 2 {
			return 1, false
		}
		length := int(binary.BigEndian.Uint16(data[at:]))
		if length < 2 || length > len(data)-at {
			return 1, false
		}
		if marker == 0xda {
			return orientation, true
		}
		segment := data[at+2 : at+length]
		if marker == 0xe1 && bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
			var valid bool
			orientation, valid = webMomentTIFFOrientation(segment[6:])
			if !valid {
				return 1, false
			}
		}
		at += length
	}
	return 1, false
}

func webMomentTIFFOrientation(data []byte) (uint16, bool) {
	if len(data) < 8 {
		return 1, false
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1, false
	}
	if order.Uint16(data[2:]) != 42 {
		return 1, false
	}
	offset := uint64(order.Uint32(data[4:]))
	if offset < 8 || offset+2 > uint64(len(data)) {
		return 1, false
	}
	count := uint64(order.Uint16(data[offset:]))
	if offset+2+count*12 > uint64(len(data)) {
		return 1, false
	}
	for i := uint64(0); i < count; i++ {
		entry := data[offset+2+i*12:]
		if order.Uint16(entry) == 0x0112 {
			if order.Uint16(entry[2:]) != 3 || order.Uint32(entry[4:]) != 1 {
				return 1, false
			}
			orientation := order.Uint16(entry[8:])
			if orientation >= 1 && orientation <= 8 {
				return orientation, true
			}
			return 1, false
		}
	}
	return 1, true
}
