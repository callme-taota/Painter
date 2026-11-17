package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWebMomentCoverRatioBoundaries(t *testing.T) {
	for _, test := range []struct {
		width, height int
		shape         string
		ratio, mobile float64
	}{
		{499, 1000, "long", 2.0 / 3, 2.0 / 3},
		{500, 1000, "portrait", 2.0 / 3, 2.0 / 3},
		{800, 1000, "portrait", .8, .8},
		{899, 1000, "portrait", .899, .899},
		{900, 1000, "square", 1, 1},
		{1100, 1000, "square", 1, 1},
		{1101, 1000, "landscape", 1.101, 1.101},
		{1600, 1000, "landscape", 1.6, 1.6},
		{2000, 1000, "landscape", 2, 1.6},
		{2001, 1000, "panorama", 2, 1.6},
	} {
		t.Run(strconv.Itoa(test.width)+"x"+strconv.Itoa(test.height), func(t *testing.T) {
			got := webMomentCoverDimensions(test.width, test.height)
			if got.shape != test.shape || math.Abs(got.ratio-test.ratio) > 1e-12 || math.Abs(got.mobile-test.mobile) > 1e-12 {
				t.Fatalf("unexpected cover: %#v", got)
			}
		})
	}
	for _, dimensions := range [][2]int{{0, 100}, {100, 0}, {-1, 100}, {40001, 1000}, {int(^uint(0) >> 1), 3}} {
		if got := webMomentCoverDimensions(dimensions[0], dimensions[1]); got != webMomentCoverFallback {
			t.Fatal("invalid dimensions did not preserve fixed placeholder", dimensions, got)
		}
	}
}

func webMomentCoverTestJPEG(t *testing.T) []byte {
	t.Helper()
	var imageBytes bytes.Buffer
	if err := jpeg.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 40, 20)), nil); err != nil {
		t.Fatal(err)
	}
	return imageBytes.Bytes()
}

func webMomentCoverTestEXIF(order binary.ByteOrder, orientation uint16) []byte {
	tiff := make([]byte, 26)
	copy(tiff, "II")
	if order == binary.BigEndian {
		copy(tiff, "MM")
	}
	order.PutUint16(tiff[2:], 42)
	order.PutUint32(tiff[4:], 8)
	order.PutUint16(tiff[8:], 1)
	order.PutUint16(tiff[10:], 0x0112)
	order.PutUint16(tiff[12:], 3)
	order.PutUint32(tiff[14:], 1)
	order.PutUint16(tiff[18:], orientation)
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xff, 0xe1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(payload)+2))
	return append(segment, payload...)
}

func webMomentCoverTestFile(t *testing.T, a *App, key string, data []byte) webMomentCoverKey {
	t.Helper()
	path := filepath.Join(a.Config.MediaDir, "objects", key)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return webMomentCoverKey{key, hashString(string(data)), int64(len(data))}
}

func TestWebMomentCoverJPEGOrientationAndBoundedHeader(t *testing.T) {
	a := &App{Config: Config{MediaDir: t.TempDir()}}
	original := webMomentCoverTestJPEG(t)
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for orientation := uint16(1); orientation <= 8; orientation++ {
			data := append(append(append([]byte{}, original[:2]...), webMomentCoverTestEXIF(order, orientation)...), original[2:]...)
			key := webMomentCoverTestFile(t, a, RandID(), data)
			got, ok := a.webMomentCoverFile(key)
			want := webMomentCoverDimensions(40, 20)
			if orientation >= 5 {
				want = webMomentCoverDimensions(20, 40)
			}
			if !ok || got != want {
				t.Fatal("EXIF display orientation ignored", order, orientation, got, want, ok)
			}
		}
	}
	plain := webMomentCoverTestFile(t, a, "plain.jpeg", original)
	if got, ok := a.webMomentCoverFile(plain); !ok || got != webMomentCoverDimensions(40, 20) {
		t.Fatal("ordinary JPEG without EXIF rejected", got, ok)
	}
	// Place valid dimensions before the bound, but EXIF and SOS after it. Merely
	// decoding SOF must not incorrectly classify this portrait as a landscape.
	jfif := []byte{0xff, 0xe0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0}
	original = append(append(append([]byte{}, original[:2]...), jfif...), original[2:]...)
	sos := bytes.Index(original, []byte{0xff, 0xda})
	var padding []byte
	for range 5 {
		segment := make([]byte, 65002)
		segment[0], segment[1] = 0xff, 0xe2
		binary.BigEndian.PutUint16(segment[2:], 65000)
		padding = append(padding, segment...)
	}
	data := append(append(append(append([]byte{}, original[:sos]...), padding...), webMomentCoverTestEXIF(binary.LittleEndian, 6)...), original[sos:]...)
	if _, _, err := image.DecodeConfig(bytes.NewReader(data[:webMomentCoverHeaderLimit])); err != nil {
		t.Fatal("fixture must expose dimensions before truncated EXIF marker stream", err)
	}
	key := webMomentCoverTestFile(t, a, "oversized-header.jpeg", data)
	if got, ok := a.webMomentCoverFile(key); ok || got != webMomentCoverFallback {
		t.Fatal("incomplete bounded header did not retain placeholder", got, ok)
	}
}

func TestWebMomentCoverMalformedEXIFAndUnsafeFiles(t *testing.T) {
	a := &App{Config: Config{MediaDir: t.TempDir()}}
	original := webMomentCoverTestJPEG(t)
	exif := webMomentCoverTestEXIF(binary.LittleEndian, 6)
	for _, data := range [][]byte{
		nil, []byte("not an image"), original[:30],
		append(append([]byte{}, original[:2]...), exif[:12]...),
	} {
		key := webMomentCoverTestFile(t, a, RandID(), data)
		if got, ok := a.webMomentCoverFile(key); ok || got != webMomentCoverFallback {
			t.Fatal("malformed image did not retain placeholder", got, ok)
		}
	}
	for _, damage := range []func([]byte){
		func(data []byte) { binary.LittleEndian.PutUint32(data[14:], ^uint32(0)) }, // IFD outside TIFF.
		func(data []byte) { binary.LittleEndian.PutUint16(data[18:], ^uint16(0)) }, // Entry count outside TIFF.
		func(data []byte) { binary.LittleEndian.PutUint16(data[22:], 4) },          // Wrong orientation type.
		func(data []byte) { binary.LittleEndian.PutUint16(data[28:], 9) },          // Invalid orientation value.
	} {
		segment := append([]byte{}, exif...)
		damage(segment)
		data := append(append(append([]byte{}, original[:2]...), segment...), original[2:]...)
		key := webMomentCoverTestFile(t, a, RandID(), data)
		if got, ok := a.webMomentCoverFile(key); ok || got != webMomentCoverFallback {
			t.Fatal("unsafe EXIF offset/type/value accepted", got, ok)
		}
	}
	valid := webMomentCoverTestFile(t, a, "valid.jpeg", original)
	for _, storage := range []string{"", ".", "../valid.jpeg", "/valid.jpeg", "x/../valid.jpeg", "x\\valid.jpeg", "bad\x00key"} {
		key := valid
		key.storage = storage
		if _, ok := a.webMomentCoverFile(key); ok {
			t.Fatal("unsafe stored path accepted", storage)
		}
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.jpeg"), original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(a.Config.MediaDir, "objects", "escape")); err != nil {
		t.Fatal(err)
	}
	valid.storage = "escape/outside.jpeg"
	if _, ok := a.webMomentCoverFile(valid); ok {
		t.Fatal("directory symlink escaped media objects")
	}
}

func TestWebMomentCoverCacheBoundAndFailureExpiry(t *testing.T) {
	var cache webMomentCoverCache
	now := time.Now()
	first := webMomentCoverKey{"first", "hash", 123}
	cache.put(first, webMomentCoverDimensions(1, 1), true, now)
	if _, ok := cache.get(first, now.Add(time.Hour)); !ok {
		t.Fatal("immutable metadata unexpectedly expired")
	}
	failed := webMomentCoverKey{"failed", "hash", 123}
	cache.put(failed, webMomentCoverFallback, false, now)
	if _, ok := cache.get(failed, now.Add(webMomentCoverFailureTTL-time.Nanosecond)); !ok {
		t.Fatal("failed read was not cached")
	}
	if _, ok := cache.get(failed, now.Add(webMomentCoverFailureTTL)); ok {
		t.Fatal("failed read did not expire for recovery")
	}
	for i := range webMomentCoverCacheLimit {
		cache.put(webMomentCoverKey{strconv.Itoa(i), "hash", 123}, webMomentCoverFallback, true, now)
	}
	if len(cache.entries) != webMomentCoverCacheLimit {
		t.Fatal("metadata cache exceeded bound", len(cache.entries))
	}
	if _, ok := cache.get(first, now); ok {
		t.Fatal("least recently used metadata was not evicted")
	}
	if _, ok := cache.get(webMomentCoverKey{"511", "changed-hash", 123}, now); ok {
		t.Fatal("different immutable media identity reused metadata")
	}
}

func TestWebMomentCoverFallbackPreservesDTO(t *testing.T) {
	image := map[string]any{"media_id": "not-a-path", "url": "/media/not-a-path", "alt": "cover"}
	items := []map[string]any{
		{"id": "1", "images": []any{image}},
		{"id": "2", "images": []map[string]any{image}},
		{"id": "3", "images": []any{}, "text": "text only"},
	}
	decorated := (&App{}).webMomentCoverItems(context.Background(), items)
	for i := range 2 {
		if decorated[i]["cover_shape"] != "unknown" || decorated[i]["cover_ratio"] != .8 || decorated[i]["cover_mobile_ratio"] != .8 {
			t.Fatal("unavailable metadata did not reserve stable space", decorated[i])
		}
		if _, exists := items[i]["cover_shape"]; exists {
			t.Fatal("fallback decoration mutated original item", items[i])
		}
	}
	if len(image) != 3 || !reflect.DeepEqual(decorated[2], items[2]) {
		t.Fatal("fallback decoration changed image or text-only DTOs")
	}
}

func TestWebMomentCoverDecorationClonesAndKeepsAPIPublicShape(t *testing.T) {
	a := momentWebApp(t)
	a.RegisterMedia()
	principal, author, csrf := testIdentity(t, a, "author")
	// Seed an immutable ready asset with known landscape dimensions.
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 180, 100))); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	publicImage := momentImage{RandID(), "public"}
	key := webMomentCoverTestFile(t, a, RandID(), imageBytes.Bytes())
	if _, err := a.Pool.Exec(ctx, `INSERT INTO media_assets(id,owner_user_id,storage_key,original_name,mime_type,byte_size,sha256,state,created_at_ms,updated_at_ms,ready_at_ms)
 VALUES($1,$2,$3,'landscape.png','image/png',$4,$5,'ready',1,1,1)`, publicImage.MediaID, principal.ID, key.storage, key.size, key.hash); err != nil {
		t.Fatal(err)
	}
	privateImage := momentImage{momentImageUpload(t, a, author, csrf, 2), "private"}
	item := momentImageFixture(t, a, author, csrf, "public text", []momentImage{publicImage})
	id := item["id"].(string)
	momentResponse(t, momentCall(t, a, author, csrf, "POST", "/api/v1/studio/moments/"+id+"/publish", nil, 1, RandID()), 200)
	momentImageFixture(t, a, author, csrf, "private text", []momentImage{privateImage})
	public, err := a.PublicMoment(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	private := map[string]any{"images": []map[string]any{{"media_id": privateImage.MediaID, "url": "/media/" + privateImage.MediaID, "alt": "private"}}}
	items := []map[string]any{public, public, private, {"text": "text only"}}
	decorated := a.webMomentCoverItems(ctx, items)
	for _, item := range decorated[:2] {
		if item["cover_shape"] != "landscape" || item["cover_ratio"] != 1.8 || item["cover_mobile_ratio"] != 1.6 {
			t.Fatal("public first-image dimensions not decorated", item)
		}
	}
	if decorated[2]["cover_shape"] != "unknown" || decorated[2]["cover_ratio"] != .8 {
		t.Fatal("unpublished media was used for SSR enhancement", decorated[2])
	}
	if _, exists := decorated[3]["cover_shape"]; exists {
		t.Fatal("text-only moment acquired invented image metadata")
	}
	for _, item := range items {
		if _, exists := item["cover_shape"]; exists {
			t.Fatal("HTML decoration mutated a public DTO")
		}
	}
	if !reflect.DeepEqual(public["images"], decorated[0]["images"]) {
		t.Fatal("HTML decoration changed image DTOs")
	}
	response := testRequest(a, "GET", "/api/v1/moments/"+id, "", nil, "")
	if response.Code != 200 || strings.Contains(response.Body.String(), "cover_ratio") || strings.Contains(response.Body.String(), "cover_shape") {
		t.Fatal("HTML metadata leaked into the public JSON API", response.Code, response.Body.String())
	}
}
