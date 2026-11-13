package app

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

func TestGameHoursDistinguishesUnknownAndZero(t *testing.T) {
	for _, sample := range []struct {
		value any
		want  string
	}{{nil, "时长未提供"}, {int64(0), "0 小时"}, {int64(1), "不足 0.1 小时"}, {int64(6), "0.1 小时"}, {int64(90), "1.5 小时"}, {float64(120), "2.0 小时"}, {int64(-1), "时长未提供"}} {
		if got := gameHours(sample.value); got != sample.want {
			t.Fatalf("%v: got %q, want %q", sample.value, got, sample.want)
		}
	}
}

func TestGameShowcaseSSRCapFullLibraryAndHandoff(t *testing.T) {
	a, calls := homeSSRApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	for i := 0; i < 7; i++ {
		in := homeTestInput("steam")
		in["title"] = "Game " + string(rune('A'+i))
		in["position"] = i
		in["visible"] = i != 6
		in["hero_url"] = "https://cdn.akamai.steamstatic.com/steam/apps/620/library_hero.jpg"
		in["logo_url"] = "https://cdn.akamai.steamstatic.com/steam/apps/620/logo.png"
		item := homeTestItem(t, a, owner, csrf, in)
		if i > 0 {
			_, err := a.Pool.Exec(context.Background(), "UPDATE homepage_items SET playtime_minutes=$2 WHERE id=$1", item["id"], (i-1)*90)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	album := homeTestInput("music")
	album["title"] = "A saved record"
	album["url"] = "https://music.apple.com/us/album/12345"
	album["image_url"] = "https://is1-ssl.mzstatic.com/image/thumb/a/632x632bb.jpg"
	homeTestItem(t, a, owner, csrf, album)
	homeSSRSave(t, a, owner, csrf, func(c map[string]any) {
		for _, key := range []string{"steam", "music"} {
			s := c["sections"].(map[string]any)[key].(map[string]any)
			s["enabled"] = true
			s["limit"] = 24
		}
	})
	doc := homeSSRDocument(t, a, nil)
	frames := friendWebClass(doc, "home-game-frame")
	if len(frames) != 5 {
		t.Fatal("homepage must render only five games", len(frames))
	}
	if text := friendWebText(doc); strings.Contains(text, "Game F") || strings.Contains(text, "Game G") || !strings.Contains(text, "0 小时") || !strings.Contains(text, "1.5 小时") || !strings.Contains(text, "时长未提供") {
		t.Fatal("homepage snapshot or real-time distinction lost", text)
	}
	if len(friendWebClass(doc, "home-collection-journey")) != 1 || len(friendWebClass(doc, "home-record-tile")) != 1 {
		t.Fatal("adjacent games and music lost their handoff")
	}
	if links := friendWebClass(doc, "game-library-link"); len(links) != 1 || friendWebAttr(links[0], "href") != "/games" {
		t.Fatal("missing complete game library destination")
	}
	if images := friendWebClass(doc, "game-backdrop"); len(images) != 5 || !strings.Contains(friendWebAttr(images[0], "src"), "library_hero.jpg") {
		t.Fatal("Steam hero artwork missing")
	}
	w := httptest.NewRecorder()
	a.Mux.ServeHTTP(w, httptest.NewRequest("GET", "/games?limit=2", nil))
	if w.Code != 200 {
		t.Fatal("complete library unavailable", w.Code, w.Body.String())
	}
	library, err := nethtml.Parse(strings.NewReader(w.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(friendWebClass(library, "game-library-card")) != 2 || !strings.Contains(friendWebText(library), "6 条游戏记录") || !strings.Contains(w.Body.String(), "cursor=") {
		t.Fatal("full library must paginate all visible games")
	}
	homeSSRSave(t, a, owner, csrf, func(c map[string]any) { c["sections"].(map[string]any)["steam"].(map[string]any)["limit"] = 0 })
	doc = homeSSRDocument(t, a, nil)
	if len(friendWebClass(doc, "home-game-frame")) != 0 || len(friendWebClass(doc, "home-record-tile")) != 1 {
		t.Fatal("zero homepage games must retain a standalone music wall")
	}
	w = httptest.NewRecorder()
	a.Mux.ServeHTTP(w, httptest.NewRequest("GET", "/games", nil))
	if w.Code != 200 {
		t.Fatal("homepage zero must not remove the full library")
	}
	homeSSRSave(t, a, owner, csrf, func(c map[string]any) { c["sections"].(map[string]any)["steam"].(map[string]any)["enabled"] = false })
	w = httptest.NewRecorder()
	a.Mux.ServeHTTP(w, httptest.NewRequest("GET", "/games", nil))
	if w.Code != 404 {
		t.Fatal("disabled game wall should not leak its library")
	}
	if calls.Load() != 0 {
		t.Fatal("visitor rendering made a provider request")
	}
}
