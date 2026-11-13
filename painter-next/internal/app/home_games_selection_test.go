package app

import (
	"reflect"
	"testing"
)

func TestHomeV39UploadedSteamCoverIsTheSelectedPublicPoster(t *testing.T) {
	for _, source := range []string{"manual", "steam_sync"} {
		t.Run(source, func(t *testing.T) {
			item := map[string]any{
				"kind": "steam", "platform": "steam", "provider_id": "730", "source": source,
				"media_id": "owner-selected-cover", "image_url": "/media/owner-selected-cover",
				"hero_url": "https://cdn.akamai.steamstatic.com/steam/apps/730/custom-hero.jpg",
				"logo_url": "https://cdn.akamai.steamstatic.com/steam/apps/730/custom-logo.png",
				"version":  int64(4), "visible": true, "local_override": true,
			}
			homePublicItem(item)
			if item["poster_url"] != "/media/owner-selected-cover" {
				t.Fatalf("selected upload was replaced by an automatic Steam poster: %v", item["poster_url"])
			}
			if item["hero_url"] != "https://cdn.akamai.steamstatic.com/steam/apps/730/custom-hero.jpg" || item["logo_url"] != "https://cdn.akamai.steamstatic.com/steam/apps/730/custom-logo.png" {
				t.Fatal("independently configured hero or logo was changed")
			}
			if item["version"] != nil || item["visible"] != nil || item["local_override"] != nil {
				t.Fatal("private editing fields leaked into the public projection")
			}
		})
	}
}

func TestHomeV39SelectedCoverVisibilityAndRemovalKeepWriteGuards(t *testing.T) {
	a := homeTestApp(t)
	a.RegisterMedia()
	_, owner, csrf := testIdentity(t, a, "owner")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	homeGameEnable(t, a, owner, csrf, 1, true, 5)
	media := momentImageUpload(t, a, owner, csrf, 39)
	in := homeTestInput("steam")
	in["media_id"], in["image_url"] = media, ""
	created := homeTestItem(t, a, owner, csrf, in)
	id := created["id"].(string)
	endpoint := "/api/v1/admin/home/items/" + id
	public := func() map[string]any {
		return momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/games?platform=steam", nil, 0, ""), 200)
	}
	items := public()["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["poster_url"] != "/media/"+media {
		t.Fatal("the public game wall ignored its selected ready upload")
	}
	momentImageRead(t, a, nil, media, 200)
	in["visible"] = false
	momentResponse(t, momentCall(t, a, nil, "", "PUT", endpoint, in, 1, RandID()), 401)
	momentResponse(t, momentCall(t, a, reader, readerCSRF, "PUT", endpoint, in, 1, RandID()), 403)
	momentResponse(t, momentCall(t, a, owner, "invalid", "PUT", endpoint, in, 1, RandID()), 403)
	key := RandID()
	hidden := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", endpoint, in, 1, key), 200)
	replayed := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", endpoint, in, 1, key), 200)
	if !reflect.DeepEqual(hidden, replayed) || hidden["media_id"] != media || hidden["visible"] != false || public()["total"] != float64(0) {
		t.Fatal("visibility lost its selected cover, idempotency or public scope")
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", endpoint, in, 1, RandID()), 409)
	// This is the validated payload sent after removing a saved Steam upload.
	// It detaches the image reference and restores official art without deletion.
	in["media_id"], in["image_url"], in["visible"] = "", "https://cdn.akamai.steamstatic.com/steam/apps/730/header.jpg", true
	restored := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", endpoint, in, 2, RandID()), 200)
	items = public()["items"].([]any)
	if restored["media_id"] != "" || restored["image_url"] != in["image_url"] || len(items) != 1 || items[0].(map[string]any)["poster_url"] != "https://cdn.akamai.steamstatic.com/steam/apps/730/library_600x900.jpg" {
		t.Fatal("detaching the selected upload did not restore normal automatic artwork")
	}
	momentImageRead(t, a, owner, media, 200)
	momentImageRead(t, a, nil, media, 404)
}
