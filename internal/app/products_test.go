package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func productTestInput(slug string) map[string]any {
	return map[string]any{"slug": slug, "name": "独立产品", "summary": "不依赖代码仓库的产品", "markdown": "## 产品能力\n\n写下自己的内容。", "icon_media_id": nil, "cover_media_id": nil, "gallery_media_ids": []string{}, "platforms": []string{"Web", "macOS"}, "state": "development", "published": false, "featured": true, "position": 10, "links": map[string]any{"main": map[string]any{"type": "web", "url": "https://example.org/app"}, "docs_url": "", "repositories": []map[string]any{}}}
}
func productTestCreate(t *testing.T, a *App, cookie *http.Cookie, csrf string, in map[string]any) map[string]any {
	t.Helper()
	return momentResponse(t, momentCall(t, a, cookie, csrf, "POST", "/api/v1/admin/products", in, 0, RandID()), 201)
}

func TestProductValidationRejectsUnsafeDestinationsAndInvalidMetadata(t *testing.T) {
	for _, target := range []string{"javascript:alert(1)", "data:text/html,x", "//evil.test/a", "/%2fexample.org", "https://a:b@example.org", "https://example.org/%0d%0a", "/bad\\path", "https://example.org/a b", "file:///etc/passwd"} {
		in := productTestInput("valid-product")
		in["links"].(map[string]any)["main"].(map[string]any)["url"] = target
		body, _ := json.Marshal(in)
		r := httpRequestProduct(string(body))
		if _, _, err := decodeProduct(r); err == nil {
			t.Fatal("unsafe destination accepted", target)
		}
	}
	for _, mutate := range []func(map[string]any){
		func(v map[string]any) { v["slug"] = "Bad Slug" }, func(v map[string]any) { v["name"] = " " }, func(v map[string]any) { v["state"] = "released" },
		func(v map[string]any) { v["platforms"] = []string{"Web", "web"} }, func(v map[string]any) { v["position"] = -1 },
		func(v map[string]any) { v["icon_media_id"] = "invalid" }, func(v map[string]any) { v["markdown"] = "![external](https://example.org/private.png)" },
		func(v map[string]any) {
			v["links"].(map[string]any)["repositories"] = []map[string]any{{"label": "code", "url": "/repository"}}
		},
	} {
		in := productTestInput("valid-product")
		mutate(in)
		body, _ := json.Marshal(in)
		if _, _, err := decodeProduct(httpRequestProduct(string(body))); err == nil {
			t.Fatal("invalid product accepted", in)
		}
	}
	for _, target := range []string{"/", "/posts", "https://example.org/app?x=1", "http://192.168.1.2/app"} {
		in := productTestInput("valid-product")
		in["links"].(map[string]any)["main"].(map[string]any)["url"] = target
		body, _ := json.Marshal(in)
		if _, _, err := decodeProduct(httpRequestProduct(string(body))); err != nil {
			t.Fatal("safe destination rejected", target, err)
		}
	}
}
func httpRequestProduct(body string) *http.Request {
	r, _ := http.NewRequest("POST", "http://painter.test/api/v1/admin/products", strings.NewReader(body))
	return r
}

func TestProductsOwnerCASIdempotencyPublicPrivacyAndRealCounts(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	_, owner, csrf := testIdentity(t, a, "owner")
	_, reader, readerCSRF := testIdentity(t, a, "reader")
	_, author, authorCSRF := testIdentity(t, a, "author")
	for _, actor := range []struct {
		cookie *http.Cookie
		csrf   string
		status int
	}{{nil, "", 401}, {reader, readerCSRF, 403}, {author, authorCSRF, 403}} {
		momentResponse(t, momentCall(t, a, actor.cookie, actor.csrf, "GET", "/api/v1/admin/products", nil, 0, ""), actor.status)
		momentResponse(t, momentCall(t, a, actor.cookie, actor.csrf, "POST", "/api/v1/admin/products", productTestInput("private-product"), 0, RandID()), actor.status)
	}
	seed, err := a.PublicProducts(ctx, url.Values{})
	if err != nil || seed["total"] != float64(1) || seed["available_count"] != float64(1) {
		t.Fatal("verified seed missing", seed, err)
	}
	items := webMaps(seed["items"])
	if len(items) != 1 || items[0]["slug"] != "painter" || items[0]["icon_url"] != "/assets/images/painter-logo.png" {
		t.Fatal("seed identity missing", items)
	}
	in := productTestInput("independent-product")
	in["name"] = "秘密产品草稿"
	in["summary"] = "私有草稿摘要"
	in["markdown"] = "私有草稿正文"
	momentResponse(t, momentCall(t, a, owner, "invalid", "POST", "/api/v1/admin/products", in, 0, RandID()), 403)
	key := RandID()
	created := momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/products", in, 0, key), 201)
	if created["icon_url"] != "" || created["cover_url"] != "" {
		t.Fatal("ordinary product inherited builtin assets", created)
	}
	replay := momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/products", in, 0, key), 201)
	if !reflect.DeepEqual(created, replay) {
		t.Fatal("idempotent create changed", created, replay)
	}
	id := created["id"].(string)
	path := "/api/v1/admin/products/" + id
	for _, q := range []string{"", "?published=true", "?scope=site&status=draft"} {
		res := momentCall(t, a, owner, csrf, "GET", "/api/v1/products"+q, nil, 0, "")
		momentResponse(t, res, 200)
		if strings.Contains(res.Body.String(), "秘密") || strings.Contains(res.Body.String(), "私有") {
			t.Fatal("draft leaked in public list", res.Body.String())
		}
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "GET", "/api/v1/products/independent-product", nil, 0, ""), 404)
	in["published"] = true
	in["name"] = "Independent"
	in["summary"] = "A genuine product"
	in["markdown"] = "## Public\n\n[unsafe](javascript:alert(1))\n\n<script>alert(1)</script>"
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, in, 0, RandID()), 428)
	updated := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, in, 1, RandID()), 200)
	if updated["version"] != float64(2) {
		t.Fatal("version failed", updated)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, in, 1, RandID()), 409)
	public, err := a.PublicProduct(ctx, "independent-product")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"version", "published", "markdown", "icon_media_id", "cover_media_id", "gallery_media_ids"} {
		if _, ok := public[field]; ok {
			t.Fatal("private field leaked", field, public)
		}
	}
	if strings.Contains(webString(public["sanitized_html"]), "javascript:") || strings.Contains(webString(public["sanitized_html"]), "<script") {
		t.Fatal("unsafe product HTML", public)
	}
	listed, err := a.PublicProducts(ctx, url.Values{"platform": {"macOS"}, "state": {"development"}})
	if err != nil || listed["total"] != float64(2) || listed["result_count"] != float64(1) || listed["development_count"] != float64(1) || len(webMaps(listed["items"])) != 1 {
		t.Fatal("counts or filters wrong", listed, err)
	}
	listed, err = a.PublicProducts(ctx, url.Values{"q": {"%"}})
	if err != nil || listed["result_count"] != float64(0) {
		t.Fatal("search wildcard was not escaped", listed, err)
	}
	in["name"] = "Renamed Product"
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, in, 2, RandID()), 200)
	in["slug"] = "new-product-path"
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, in, 3, RandID()), 400)
	in["slug"] = "independent-product"
	public, err = a.PublicProduct(ctx, "independent-product")
	if err != nil || public["name"] != "Renamed Product" {
		t.Fatal("renaming broke stable public path", public, err)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", path, nil, 2, RandID()), 409)
	deleteKey := RandID()
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", path, nil, 3, deleteKey), 200)
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", path, nil, 3, deleteKey), 200)
	if _, err = a.PublicProduct(ctx, "independent-product"); err == nil {
		t.Fatal("deleted product public")
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/products", in, 0, RandID()), 409)
	var audits int
	if err = a.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE target_type='product' AND target_id=$1`, id).Scan(&audits); err != nil || audits != 4 {
		t.Fatal("unexpected audit count", audits, err)
	}
}

func TestProductImagesOwnershipVisibilitySharedReferencesAndGC(t *testing.T) {
	a := testApp(t)
	a.RegisterMedia()
	ctx := context.Background()
	_, owner, csrf := testIdentity(t, a, "owner")
	_, other, otherCSRF := testIdentity(t, a, "author")
	upload := momentResponse(t, mediaTestUpload(t, a, owner, csrf, RandID(), 41), 201)
	media := upload["media_id"].(string)
	foreign := momentResponse(t, mediaTestUpload(t, a, other, otherCSRF, RandID(), 42), 201)["media_id"].(string)
	seed, err := a.PublicProduct(ctx, "painter")
	if err != nil || seed["icon_url"] != builtinPainterIconURL || seed["cover_url"] != builtinPainterCoverURL {
		t.Fatal("curated product defaults missing", seed, err)
	}
	if _, err = a.Pool.Exec(ctx, `UPDATE products SET builtin_key=NULL,version=version+1 WHERE builtin_key='painter'`); err == nil {
		t.Fatal("database allowed builtin identity mutation")
	}
	seedInput := productTestInput("painter")
	seedInput["published"], seedInput["icon_media_id"], seedInput["cover_media_id"] = true, media, media
	seedPath := "/api/v1/admin/products/" + seed["id"].(string)
	customized := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", seedPath, seedInput, 1, RandID()), 200)
	if customized["icon_url"] != "/media/"+media || customized["cover_url"] != "/media/"+media {
		t.Fatal("uploaded media did not override builtin defaults", customized)
	}
	seedInput["icon_media_id"], seedInput["cover_media_id"] = nil, nil
	restored := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", seedPath, seedInput, 2, RandID()), 200)
	if restored["icon_url"] != builtinPainterIconURL || restored["cover_url"] != builtinPainterCoverURL {
		t.Fatal("cleared uploads did not restore builtin defaults", restored)
	}
	in := productTestInput("image-product")
	in["icon_media_id"] = foreign
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", "/api/v1/admin/products", in, 0, RandID()), 400)
	in["icon_media_id"] = media
	in["cover_media_id"] = media
	in["gallery_media_ids"] = []string{media}
	in["markdown"] = "![产品截图](/media/" + media + ")"
	created := productTestCreate(t, a, owner, csrf, in)
	path := "/api/v1/admin/products/" + created["id"].(string)
	if testRequest(a, "GET", "/media/"+media, "", nil, "").Code != 404 {
		t.Fatal("draft image public")
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", "/api/v1/media/"+media, nil, 0, RandID()), 409)
	if _, err := a.Pool.Exec(ctx, `UPDATE media_assets SET created_at_ms=$2 WHERE id=$1`, media, NowMillis()-8*24*60*60*1000); err != nil {
		t.Fatal(err)
	}
	if err := a.ReconcileMedia(ctx); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := a.Pool.QueryRow(ctx, `SELECT state FROM media_assets WHERE id=$1`, media).Scan(&state); err != nil || state != "ready" {
		t.Fatal("GC removed draft product media", state, err)
	}
	in["published"] = true
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, in, 1, RandID()), 200)
	if testRequest(a, "GET", "/media/"+media, "", nil, "").Code != 200 {
		t.Fatal("public product image not served")
	}
	second := productTestInput("shared-image-product")
	second["icon_media_id"] = media
	shared := productTestCreate(t, a, owner, csrf, second)
	sharedPath := "/api/v1/admin/products/" + shared["id"].(string)
	in["published"] = false
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, in, 2, RandID()), 200)
	if testRequest(a, "GET", "/media/"+media, "", nil, "").Code != 404 {
		t.Fatal("withdrawal did not revoke public product image")
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", path, nil, 3, RandID()), 200)
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", "/api/v1/media/"+media, nil, 0, RandID()), 409)
	if _, err := a.Pool.Exec(ctx, `UPDATE media_assets SET state='deleting' WHERE id=$1`, media); err == nil {
		t.Fatal("DB allowed deleting product-referenced media")
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", sharedPath, nil, 1, RandID()), 200)
	deleted := momentCall(t, a, owner, csrf, "DELETE", "/api/v1/media/"+media, nil, 0, RandID())
	if deleted.Code != http.StatusNoContent {
		t.Fatal("unreferenced media could not be deleted", deleted.Code, deleted.Body.String())
	}
}

func TestProductsSSRCanonicalSitemapAndDraftOmission(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterWeb()
	_, owner, csrf := testIdentity(t, a, "owner")
	secret := productTestInput("secret-product")
	secret["name"] = "NeverPublicProductName"
	productTestCreate(t, a, owner, csrf, secret)
	for _, path := range []string{"/products", "/products/painter", "/products?product=painter"} {
		res := testRequest(a, "GET", path, "", nil, "")
		if res.Code != 200 || !strings.Contains(res.Body.String(), "Painter") || strings.Contains(res.Body.String(), "NeverPublicProductName") {
			t.Fatal("public product SSR failed", path, res.Code, res.Body.String())
		}
	}
	for _, path := range []string{"/products/secret-product", "/products?product=secret-product", "/products/nonexistent"} {
		res := testRequest(a, "GET", path, "", owner, "")
		if res.Code != 404 || strings.Contains(res.Body.String(), "NeverPublicProductName") {
			t.Fatal("draft product SSR leaked", path, res.Code)
		}
	}
	res := testRequest(a, "GET", "/sitemap.xml", "", nil, "")
	if res.Code != 200 || !strings.Contains(res.Body.String(), "/products/painter") || strings.Contains(res.Body.String(), "secret-product") {
		t.Fatal("product sitemap wrong", res.Code, res.Body.String())
	}
	res = testRequest(a, "GET", "/products?product=painter", "", nil, "")
	if !strings.Contains(res.Body.String(), `rel="canonical" href="http://painter.test/products/painter"`) {
		t.Fatal("selected product canonical missing", res.Body.String())
	}
	res = testRequest(a, "GET", "/studio/products", "", owner, "")
	if res.Code != 200 {
		t.Fatal("owner product workbench route failed", res.Code)
	}
}

func TestProductsFeaturedPriorityUpdatesPublicCatalogWithoutPromotingDrafts(t *testing.T) {
	a := testApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	regular := productTestInput("ordinary-product")
	regular["published"], regular["featured"], regular["position"] = true, false, 0
	productTestCreate(t, a, owner, csrf, regular)
	featured := productTestInput("recommended-product")
	featured["published"], featured["featured"], featured["position"] = true, true, 900
	promoted := productTestCreate(t, a, owner, csrf, featured)
	draft := productTestInput("recommended-draft")
	draft["published"], draft["featured"], draft["position"] = false, true, 0
	productTestCreate(t, a, owner, csrf, draft)
	assertOrder := func(want []string) {
		t.Helper()
		catalog := momentResponse(t, momentCall(t, a, nil, "", "GET", "/api/v1/products", nil, 0, ""), 200)
		got := []string{}
		for _, item := range webMaps(catalog["items"]) {
			got = append(got, webString(item["slug"]))
		}
		if !reflect.DeepEqual(got, want) || catalog["total"] != float64(len(want)) {
			t.Fatal("recommendation did not control the public catalog or promoted a draft", got, want, catalog["total"])
		}
	}
	// Recommendation takes priority even when an ordinary product has a lower
	// numeric position; the existing recommended seed retains its lower position.
	assertOrder([]string{"painter", "recommended-product", "ordinary-product"})
	featured["featured"] = false
	path := "/api/v1/admin/products/" + promoted["id"].(string)
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, featured, 1, RandID()), 200)
	assertOrder([]string{"painter", "ordinary-product", "recommended-product"})
	momentResponse(t, momentCall(t, a, owner, csrf, "DELETE", path, nil, 2, RandID()), 200)
	assertOrder([]string{"painter", "ordinary-product"})
}
