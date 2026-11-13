package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func psnTestApp(t *testing.T) *App {
	t.Helper()
	a := homeTestApp(t)
	for _, endpoint := range []struct {
		method, path string
		fn           func(http.ResponseWriter, *http.Request) error
	}{{"GET", "/api/v1/admin/home/psn", a.homePSNGet}, {"PUT", "/api/v1/admin/home/psn", a.homePSNPut}, {"POST", "/api/v1/admin/home/psn/sync", a.homePSNSync}} {
		pattern := endpoint.method + " " + endpoint.path
		if _, existing := a.Mux.Handler(httptest.NewRequest(endpoint.method, endpoint.path, nil)); existing != pattern {
			a.Handle(pattern, endpoint.fn)
		}
	}
	return a
}

func psnTestConnect(t *testing.T, a *App) map[string]any {
	t.Helper()
	psnFixture(t, a, psnFixtureConnect)
	value, err := a.ConnectHomePSNLocal(context.Background(), strings.Repeat("X", 64), "Fixture_Player", true)
	if err != nil {
		t.Fatal("local connect", err)
	}
	return value
}

func TestPSNConnectionAuthorizationEncryptionAndQueueIdempotency(t *testing.T) {
	a := psnTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	_, author, authorCSRF := testIdentity(t, a, "author")
	path := "/api/v1/admin/home/psn"
	momentResponse(t, momentCall(t, a, nil, "", "GET", path, nil, 0, ""), 401)
	momentResponse(t, momentCall(t, a, author, "", "GET", path, nil, 0, ""), 403)
	momentResponse(t, momentCall(t, a, author, authorCSRF, "PUT", path, map[string]any{"auto_sync": false}, 1, RandID()), 403)
	momentResponse(t, momentCall(t, a, owner, "invalid", "PUT", path, map[string]any{"auto_sync": false}, 1, RandID()), 403)
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, map[string]any{"auto_sync": false}, 0, RandID()), 428)
	var calls atomic.Int32
	psnFixture(t, a, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); psnFixtureConnect(w, r) })
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, map[string]any{"npsso": strings.Repeat("X", 64), "auto_sync": true}, 1, RandID()), 409)
	if calls.Load() != 0 {
		t.Fatal("plain LAN HTTP submitted NPSSO to provider")
	}
	value, err := a.ConnectHomePSNLocal(context.Background(), strings.Repeat("X", 64), "Fixture_Player", true)
	if err != nil || value["connected"] != true || value["status"] != "queued" || value["version"] != int64(2) || calls.Load() != 4 {
		t.Fatal("safe local connect", value, calls.Load(), err)
	}
	connection, err := psnRead(context.Background(), a.Pool, false)
	if err != nil || bytes.Contains(connection.Cipher, []byte("fixture-refresh")) || bytes.Contains(connection.Cipher, []byte(strings.Repeat("X", 64))) {
		t.Fatal("credential plaintext persisted", err)
	}
	decoded, err := a.psnDecode(connection)
	if err != nil || decoded.Refresh != "fixture-refresh" {
		t.Fatal("encrypted token unreadable", err)
	}
	private := momentResponse(t, momentCall(t, a, owner, "", "GET", path, nil, 0, ""), 200)
	encoded := psnMarshal(t, private)
	for _, forbidden := range []string{"fixture-refresh", "fixture-access", strings.Repeat("X", 64), "ciphertext", "access_token", "refresh_token"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatal("secret leaked into private DTO")
		}
	}
	key := RandID()
	queued := momentResponse(t, momentCall(t, a, owner, csrf, "POST", path+"/sync", map[string]any{}, 2, key), 202)
	replay := momentResponse(t, momentCall(t, a, owner, csrf, "POST", path+"/sync", map[string]any{}, 2, key), 202)
	if queued["version"] != float64(3) || psnMarshal(t, queued) != psnMarshal(t, replay) || calls.Load() != 4 {
		t.Fatal("queue was not fast/idempotent", queued, replay, calls.Load())
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "POST", path+"/sync", map[string]any{}, 2, RandID()), 409)
	disconnected := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, map[string]any{"disconnect": true, "auto_sync": false}, 3, RandID()), 200)
	if disconnected["connected"] != false || disconnected["auto_sync"] != false || disconnected["status"] != "idle" {
		t.Fatal("disconnect state", disconnected)
	}
	connection, err = psnRead(context.Background(), a.Pool, false)
	if err != nil || len(connection.Cipher) != 0 || connection.Requested || connection.Lease != "" {
		t.Fatal("disconnect retained active credentials/job", err)
	}
}

func TestPSNWorkerUpsertAndCredentialCAS(t *testing.T) {
	a := psnTestApp(t)
	_, owner, csrf := testIdentity(t, a, "owner")
	psnTestConnect(t, a)
	steam := homeTestItem(t, a, owner, csrf, homeTestInput("steam"))
	ctx := context.Background()
	lease, err := a.psnClaim(ctx)
	if err != nil || lease.Lease == "" {
		t.Fatal("claim", err)
	}
	games := []psnGame{
		{ProviderID: "CUSA00001_00", Title: "Same title", Alt: "Same title", Platforms: []string{"PS4"}, ImageURL: "https://image.api.playstation.com/a.png", Position: 0, PlaytimeMinutes: psnInt(60)},
		{ProviderID: "PPSA00001_00", Title: "Same title", Alt: "Same title", Platforms: []string{"PS5"}, Position: 1},
	}
	if err = a.psnApply(ctx, lease, games); err != nil {
		t.Fatal("initial PSN upsert", err)
	}
	if _, err = a.Pool.Exec(ctx, `UPDATE homepage_items SET title='My own title',image_url='https://image.api.playstation.com/local.png',local_override=true,visible=false,position=17 WHERE source='psn_sync' AND provider_id='CUSA00001_00'`); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Pool.Exec(ctx, `UPDATE homepage_game_connections SET sync_requested=true,next_sync_at_ms=0 WHERE provider='psn'`); err != nil {
		t.Fatal(err)
	}
	lease, err = a.psnClaim(ctx)
	if err != nil || lease.Lease == "" {
		t.Fatal("second claim", err)
	}
	// Another homepage edit must not invalidate a rotating PSN refresh token.
	if _, err = a.Pool.Exec(ctx, `UPDATE homepage_settings SET version=version+1 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	fresh := psnGoodTokens()
	fresh.Access, fresh.Refresh = "new-access", "new-refresh"
	oldCredentialVersion := lease.CredentialVersion
	if err = a.psnPersistTokens(ctx, &lease, fresh); err != nil || lease.CredentialVersion != oldCredentialVersion+1 {
		t.Fatal("independent credential CAS", err)
	}
	stored, err := psnRead(ctx, a.Pool, false)
	decoded, decodeErr := a.psnDecode(stored)
	if err != nil || decodeErr != nil || decoded.Refresh != "new-refresh" {
		t.Fatal("rotation not committed immediately", err, decodeErr)
	}
	games[0].Title, games[0].ImageURL, games[0].PlaytimeMinutes = "Remote renamed", "https://image.api.playstation.com/new.png", psnInt(120)
	games[1] = psnGame{ProviderID: "PPSA00002_00", Title: "New title", Alt: "New title", Platforms: []string{"PS5"}, Position: 1, PlaytimeMinutes: psnInt(0)}
	if err = a.psnApply(ctx, lease, games); err != nil {
		t.Fatal("subsequent PSN upsert", err)
	}
	var title, image, platform string
	var visible bool
	var position, minutes int64
	if err = a.Pool.QueryRow(ctx, `SELECT title,image_url,visible,position,playtime_minutes,platform FROM homepage_items WHERE source='psn_sync' AND provider_id='CUSA00001_00'`).Scan(&title, &image, &visible, &position, &minutes, &platform); err != nil || title != "My own title" || image != "https://image.api.playstation.com/local.png" || visible || position != 17 || minutes != 120 || platform != "psn" {
		t.Fatal("sync overwrote local editing decisions", title, image, visible, position, minutes, err)
	}
	var psnCount, steamCount int
	if err = a.Pool.QueryRow(ctx, `SELECT count(*) FROM homepage_items WHERE source='psn_sync'`).Scan(&psnCount); err != nil || psnCount != 3 {
		t.Fatal("missing titles were deleted or identity merged", psnCount, err)
	}
	if err = a.Pool.QueryRow(ctx, `SELECT count(*) FROM homepage_items WHERE id=$1 AND platform='steam' AND provider_id='730'`, steam["id"]).Scan(&steamCount); err != nil || steamCount != 1 {
		t.Fatal("PSN changed Steam identity", steamCount, err)
	}
	// A disconnected or replaced connection must reject stale worker rotation.
	if _, err = a.Pool.Exec(ctx, `UPDATE homepage_game_connections SET sync_requested=true,next_sync_at_ms=0 WHERE provider='psn'`); err != nil {
		t.Fatal(err)
	}
	lease, err = a.psnClaim(ctx)
	if err != nil || lease.Lease == "" {
		t.Fatal("final claim", err)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", "/api/v1/admin/home/psn", map[string]any{"disconnect": true, "auto_sync": false}, lease.Version, RandID()), 200)
	if err = a.psnPersistTokens(ctx, &lease, fresh); err == nil {
		t.Fatal("stale lease resurrected disconnected credential")
	}
	stored, err = psnRead(ctx, a.Pool, false)
	if err != nil || len(stored.Cipher) != 0 {
		t.Fatal("disconnect lost to stale rotation", err)
	}
	if err = a.Pool.QueryRow(ctx, `SELECT count(*) FROM homepage_items WHERE source='psn_sync'`).Scan(&psnCount); err != nil || psnCount != 3 {
		t.Fatal("disconnect removed game snapshot", err)
	}
}

func TestPSNConnectionHTTPSExchangeReplayAndExpectedIdentity(t *testing.T) {
	a := psnTestApp(t)
	a.Config.PublicBaseURL = "https://painter.test"
	_, owner, csrf := testIdentity(t, a, "owner")
	var calls atomic.Int32
	psnFixture(t, a, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); psnFixtureConnect(w, r) })
	path := "/api/v1/admin/home/psn"
	input := map[string]any{"npsso": strings.Repeat("X", 64), "online_id": "Fixture_Player", "auto_sync": false}
	key := RandID()
	connected := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, input, 1, key), 200)
	replay := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, input, 1, key), 200)
	if connected["connected"] != true || connected["auto_sync"] != false || connected["status"] != "queued" || calls.Load() != 4 || psnMarshal(t, connected) != psnMarshal(t, replay) {
		t.Fatal("HTTPS exchange replay repeated credentials or lost initial sync", connected, replay, calls.Load())
	}
	before, err := psnRead(context.Background(), a.Pool, false)
	if err != nil {
		t.Fatal(err)
	}
	input["online_id"] = "Different_Player"
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, input, 2, RandID()), 409)
	after, err := psnRead(context.Background(), a.Pool, false)
	if err != nil || before.Version != after.Version || before.CredentialVersion != after.CredentialVersion || !bytes.Equal(before.Cipher, after.Cipher) || after.Lease != "" || after.OnlineID != "Fixture_Player" {
		t.Fatal("mismatched account replaced current credentials", err)
	}
	var audits int
	if err = a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE action='home.psn_connect'`).Scan(&audits); err != nil || audits != 1 {
		t.Fatal("idempotency or mismatch duplicated connect audit", audits, err)
	}
}

func TestPSNWorkerRefreshSurvivesFailedFullFetch(t *testing.T) {
	a := psnTestApp(t)
	psnTestConnect(t, a)
	ctx := context.Background()
	tokens := psnGoodTokens()
	tokens.AccessExpires = NowMillis()
	raw, _ := json.Marshal(tokens)
	cipher, err := a.encrypt(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Pool.Exec(ctx, `UPDATE homepage_game_connections SET token_ciphertext=$1 WHERE provider='psn'`, cipher); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	psnFixture(t, a, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if strings.HasSuffix(r.URL.Path, "/token") {
			fmt.Fprint(w, `{"access_token":"rotated-access","refresh_token":"rotated-refresh","token_type":"Bearer","expires_in":3600,"refresh_token_expires_in":2592000}`)
			return
		}
		if r.URL.Query().Get("offset") == "0" {
			// The rotated token has to be in the DB before the first games page.
			stored, err := psnRead(ctx, a.Pool, false)
			decoded, decodeErr := a.psnDecode(stored)
			if err != nil || decodeErr != nil || decoded.Refresh != "rotated-refresh" {
				t.Error("rotation waited until all pages completed")
			}
			fmt.Fprint(w, `{"titles":[{"titleId":"CUSA00001_00","name":"First","category":"ps4_game"}],"totalItemCount":2,"nextOffset":1}`)
			return
		}
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":"rotated-refresh must not leak"}`)
	})
	a.runHomePSNOnce(ctx)
	stored, err := psnRead(ctx, a.Pool, false)
	decoded, decodeErr := a.psnDecode(stored)
	if err != nil || decodeErr != nil || decoded.Refresh != "rotated-refresh" || stored.CredentialVersion != 3 || stored.LastError == "" || stored.Requested || stored.Lease != "" || stored.NextSync < NowMillis()+14*time.Minute.Milliseconds() || requests.Load() != 3 {
		t.Fatal("failed pagination lost rotation/backoff/lease", stored.CredentialVersion, stored.LastError, requests.Load(), err, decodeErr)
	}
	var count int
	if err = a.Pool.QueryRow(ctx, `SELECT count(*) FROM homepage_items WHERE source='psn_sync'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("incomplete pages were applied", count, err)
	}
}

func TestPSNApplyLocksSettingsBeforeConnectionAndRechecksGlobalCapacity(t *testing.T) {
	a := psnTestApp(t)
	psnTestConnect(t, a)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	lease, err := a.psnClaim(ctx)
	if err != nil || lease.Lease == "" {
		t.Fatal("claim", err)
	}
	// Model a concurrent manual/Steam writer holding the shared capacity row.
	manual, err := a.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer manual.Rollback(context.Background())
	if _, err = homeRead(ctx, manual, true); err != nil {
		t.Fatal(err)
	}
	blocker := int64(manual.Conn().PgConn().PID())
	result := make(chan error, 1)
	go func() {
		result <- a.psnApply(ctx, lease, []psnGame{{ProviderID: "PPSA00999_00", Title: "New title", Alt: "New title", Platforms: []string{"PS5"}}})
	}()
	// Observe the real backend waiting on homepage_settings, rather than
	// relying on scheduling sleeps. It must not own the connection row yet.
	for {
		var waiting bool
		err = a.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%FROM homepage_settings WHERE id=1 FOR UPDATE%' AND $1::integer=ANY(pg_blocking_pids(pid)))`, blocker).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-result:
			t.Fatal("PSN apply did not wait for the global settings lock", err)
		case <-ctx.Done():
			t.Fatal("settings lock wait was not observed", ctx.Err())
		case <-time.After(5 * time.Millisecond):
		}
	}
	probe, err := a.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = probe.Exec(ctx, `SELECT provider FROM homepage_game_connections WHERE provider='psn' FOR UPDATE NOWAIT`)
	probe.Rollback(context.Background())
	if err != nil {
		t.Fatal("PSN apply locked connection before waiting for settings", err)
	}
	// Fill capacity while still holding that same global writer lock. Once it
	// wakes, PSN must see this committed count and refuse its one extra title.
	_, err = manual.Exec(ctx, `INSERT INTO homepage_items(kind,title,alt,image_url,provider_id,source,created_at_ms,updated_at_ms) SELECT 'steam','Capacity fixture','Capacity fixture','https://shared.akamai.steamstatic.com/header.jpg',n::text,'manual',$1,$1 FROM generate_series(1,5000) n`, NowMillis())
	if err != nil {
		t.Fatal(err)
	}
	if err = manual.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-result:
		var api *APIError
		if !errors.As(err, &api) || api.Code != "HOME_ITEM_LIMIT" {
			t.Fatal("capacity was checked before concurrent writer committed", err)
		}
	case <-ctx.Done():
		t.Fatal("apply failed to finish after settings writer committed", ctx.Err())
	}
	var total, psn int
	if err = a.Pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE source='psn_sync') FROM homepage_items`).Scan(&total, &psn); err != nil || total != 5000 || psn != 0 {
		t.Fatal("global item capacity exceeded", total, psn, err)
	}
	// The apply failure must leave the connection usable for the worker's
	// ordinary release/backoff path; no item/media lock order was inverted.
	stored, err := psnRead(ctx, a.Pool, false)
	if err != nil || stored.Lease != lease.Lease {
		t.Fatal("capacity conflict changed connection state", err)
	}
}

func TestPSNConnectionSingleAccountSnapshotGuard(t *testing.T) {
	a := psnTestApp(t)
	a.Config.PublicBaseURL = "https://painter.test"
	_, owner, csrf := testIdentity(t, a, "owner")
	psnTestConnect(t, a)
	ctx := context.Background()
	lease, err := a.psnClaim(ctx)
	if err != nil || lease.Lease == "" {
		t.Fatal("claim", err)
	}
	if err = a.psnApply(ctx, lease, []psnGame{{ProviderID: "PPSA00001_00", Title: "Saved game", Alt: "Saved game", Platforms: []string{"PS5"}, PlaytimeMinutes: psnInt(60)}}); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/admin/home/psn"
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, map[string]any{"disconnect": true, "auto_sync": false}, 2, RandID()), 200)
	setIdentity := func(account, online string) {
		psnFixture(t, a, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/api/v1/devices/accounts/me":
				fmt.Fprintf(w, `{"accountId":%q}`, account)
			case strings.HasPrefix(r.URL.Path, "/api/userProfile/v1/internal/users/"):
				fmt.Fprintf(w, `{"onlineId":%q}`, online)
			default:
				psnFixtureConnect(w, r)
			}
		})
	}
	otherAccount := "9876543210987654321"
	setIdentity(otherAccount, "Other_Player")
	input := map[string]any{"npsso": strings.Repeat("X", 64), "online_id": "Other_Player", "auto_sync": false}
	rejected := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, input, 3, RandID()), 409)
	if !strings.Contains(psnMarshal(t, rejected), "PSN_ACCOUNT_CHANGE_REQUIRES_CLEANUP") {
		t.Fatal("account change was not clearly distinguished", rejected)
	}
	if _, err = a.ConnectHomePSNLocal(ctx, strings.Repeat("X", 64), "Other_Player", false); err == nil {
		t.Fatal("local helper mixed another account into saved snapshot")
	}
	stored, err := psnRead(ctx, a.Pool, false)
	if err != nil || stored.AccountID != "12345678901234567890" || stored.Version != 3 || len(stored.Cipher) != 0 || stored.Lease != "" {
		t.Fatal("rejected rebind changed disconnected identity", stored.AccountID, err)
	}
	var count int
	if err = a.Pool.QueryRow(ctx, `SELECT count(*) FROM homepage_items WHERE source='psn_sync' AND provider_id='PPSA00001_00' AND playtime_minutes=60`).Scan(&count); err != nil || count != 1 {
		t.Fatal("rebind modified saved snapshot", err)
	}
	// Online IDs can be renamed: the immutable account ID, not its label,
	// decides whether the existing wall may be retained on reconnect.
	setIdentity("12345678901234567890", "Renamed_Player")
	renamed, err := a.ConnectHomePSNLocal(ctx, strings.Repeat("X", 64), "Renamed_Player", false)
	if err != nil || renamed["online_id"] != "Renamed_Player" || renamed["version"] != int64(4) {
		t.Fatal("same account Online ID rename rejected", renamed, err)
	}
	momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, map[string]any{"disconnect": true, "auto_sync": false}, 4, RandID()), 200)
	// A deliberate operator cleanup leaves no account-owned snapshot to mix.
	// Keep its own maintenance transaction in the same settings/connection order.
	err = a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		if _, err := psnLock(ctx, tx); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM homepage_items WHERE source='psn_sync'`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	setIdentity(otherAccount, "Other_Player")
	rebound := momentResponse(t, momentCall(t, a, owner, csrf, "PUT", path, input, 5, RandID()), 200)
	if rebound["account_id"] != otherAccount || rebound["game_count"] != float64(0) || rebound["status"] != "queued" {
		t.Fatal("empty snapshot could not rebind safely", rebound)
	}
}
