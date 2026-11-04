package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func analyticsCall(t *testing.T, a *App, events []map[string]any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	b, e := json.Marshal(map[string]any{"events": events})
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("POST", "/api/v1/analytics/events", bytes.NewReader(b))
	r.Header.Set("Origin", a.Config.PublicBaseURL)
	r.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	return w
}
func analyticsTestEvent(kind, page string) map[string]any {
	return map[string]any{"event_id": RandID(), "schema_version": 1, "event_name": kind, "page_instance_id": page, "client_at_ms": NowMillis(), "props": map[string]any{}}
}
func TestAnalyticsDurableDedupUVHistoryAndWithdraw(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	a.RegisterAnalytics()
	a.RegisterInteractions()
	_, owner, oc := testIdentity(t, a, "owner")
	_, reader, rc := testIdentity(t, a, "reader")
	post, revision := interactionPublished(t, a, owner, oc)
	receipt := a.ReadReceipt(post, revision)
	root := analyticsTestEvent("page_view.v1", RandID())
	first := analyticsCall(t, a, []map[string]any{root})
	contentWant(t, first, 200)
	var visitor *http.Cookie
	for _, c := range first.Result().Cookies() {
		if c.Name == "painter_visitor" {
			visitor = c
		}
	}
	if visitor == nil {
		t.Fatal("no tracking cookie")
	}
	page := RandID()
	pv := analyticsTestEvent("page_view.v1", page)
	pv["post_id"] = post
	pv["revision_id"] = revision
	pv["read_receipt"] = receipt
	read := analyticsTestEvent("post_read.v1", page)
	read["post_id"] = post
	read["revision_id"] = revision
	read["read_receipt"] = receipt
	read["props"] = map[string]any{"duration_ms": 10000}
	got := contentWant(t, analyticsCall(t, a, []map[string]any{pv, read}, reader, visitor), 200)
	if got["accepted"] != float64(2) {
		t.Fatal(got)
	}
	pv["event_id"] = RandID()
	read["event_id"] = RandID()
	got = contentWant(t, analyticsCall(t, a, []map[string]any{pv, read}, reader, visitor), 200)
	if got["duplicate"] != float64(2) {
		t.Fatal("semantic duplicate counted", got)
	}
	if e := a.RollupAnalytics(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e := a.RollupAnalytics(context.Background()); e != nil {
		t.Fatal(e)
	}
	var siteUV, postUV, sitePV, postPV, reads int64
	if e := a.Pool.QueryRow(context.Background(), `SELECT unique_visitors,page_views FROM analytics_daily WHERE resource_key='site'`).Scan(&siteUV, &sitePV); e != nil {
		t.Fatal(e)
	}
	if e := a.Pool.QueryRow(context.Background(), `SELECT unique_visitors,page_views,post_reads FROM analytics_daily WHERE resource_key=$1`, "post:"+post).Scan(&postUV, &postPV, &reads); e != nil {
		t.Fatal(e)
	}
	if siteUV != 1 || postUV != 1 || sitePV != 2 || postPV != 1 || reads != 1 {
		t.Fatal(siteUV, postUV, sitePV, postPV, reads)
	}
	history := contentWant(t, contentCall(t, a, reader, rc, "GET", "/api/v1/me/history", nil, 0, ""), 200)
	if len(history["items"].([]any)) != 1 {
		t.Fatal("history not recorded", history)
	}
	bad := analyticsTestEvent("page_view.v1", RandID())
	bad["props"] = map[string]any{"secret": "must reject"}
	good := analyticsTestEvent("page_view.v1", RandID())
	w := analyticsCall(t, a, []map[string]any{good, bad}, visitor)
	if w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	var n int
	if e := a.Pool.QueryRow(context.Background(), `SELECT count(*) FROM analytics_events WHERE event_id=$1`, good["event_id"]).Scan(&n); e != nil || n != 0 {
		t.Fatal("invalid batch partially persisted", n, e)
	}
	unknown := analyticsTestEvent("page_view.v1", RandID())
	unknown["client_supplied_user_id"] = "forbidden"
	badResponse := analyticsCall(t, a, []map[string]any{good, unknown}, visitor)
	if badResponse.Code != 400 {
		t.Fatal(badResponse.Code, badResponse.Body.String())
	}
	details := jsonMap(t, badResponse)["error"].(map[string]any)["details"].(map[string]any)
	if details["index"] != float64(1) {
		t.Fatal("missing invalid event index", details)
	}
	contentWant(t, contentCall(t, a, owner, oc, "DELETE", "/api/v1/studio/posts/"+post+"/publication", nil, 2, ""), 200)
	withdrawn := analyticsTestEvent("post_read.v1", RandID())
	withdrawn["post_id"] = post
	withdrawn["revision_id"] = revision
	withdrawn["read_receipt"] = receipt
	withdrawn["props"] = map[string]any{"duration_ms": 10000}
	w = analyticsCall(t, a, []map[string]any{withdrawn}, visitor)
	if w.Code != 400 {
		t.Fatal("withdrawn read accepted", w.Code, w.Body.String())
	}
	history = contentWant(t, contentCall(t, a, reader, rc, "GET", "/api/v1/me/history", nil, 0, ""), 200)
	if len(history["items"].([]any)) != 0 {
		t.Fatal("withdrawn history leaked")
	}
}

func TestAnalyticsLateCommitAndAtomicRollup(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	insert := func(tx pgx.Tx, id string) int64 {
		var seq int64
		now := NowMillis()
		e := tx.QueryRow(ctx, `INSERT INTO analytics_events(event_id,semantic_key,kind,page_instance_id,received_at_ms,day_key,traffic_class,normalized_path) VALUES($1,$2,'page_view.v1',$3,$4,(to_timestamp($4::bigint/1000.0) AT TIME ZONE 'Asia/Shanghai')::date,'public','/') RETURNING seq`, id, analyticsHash(id), RandID(), now).Scan(&seq)
		if e != nil {
			t.Fatal(e)
		}
		return seq
	}
	low, e := a.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer low.Rollback(ctx)
	lowseq := insert(low, RandID())
	high, e := a.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	highseq := insert(high, RandID())
	if e = high.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if lowseq >= highseq {
		t.Fatal("sequence ordering assumption")
	}
	if e = a.RollupAnalytics(ctx); e != nil {
		t.Fatal(e)
	}
	if e = low.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = a.RollupAnalytics(ctx); e != nil {
		t.Fatal(e)
	}
	var pv int64
	if e = a.Pool.QueryRow(ctx, `SELECT page_views FROM analytics_daily WHERE resource_key='site'`).Scan(&pv); e != nil || pv != 2 {
		t.Fatal("late commit lost", pv, e)
	}
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	insert(tx, RandID())
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	_, e = a.Pool.Exec(ctx, `CREATE FUNCTION test_rollup_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected rollup failure'; END $$; CREATE TRIGGER test_rollup_fail BEFORE UPDATE ON analytics_daily FOR EACH ROW EXECUTE FUNCTION test_rollup_fail()`)
	if e != nil {
		t.Fatal(e)
	}
	if e = a.RollupAnalytics(ctx); e == nil {
		t.Fatal("injected failure succeeded")
	}
	var pending int
	if e = a.Pool.QueryRow(ctx, `SELECT count(*) FROM analytics_events WHERE aggregated_at_ms IS NULL`).Scan(&pending); e != nil || pending != 1 {
		t.Fatal("raw marked before delta committed", pending, e)
	}
	if _, e = a.Pool.Exec(ctx, `DROP TRIGGER test_rollup_fail ON analytics_daily`); e != nil {
		t.Fatal(e)
	}
	if e = a.RollupAnalytics(ctx); e != nil {
		t.Fatal(e)
	}
	if e = a.Pool.QueryRow(ctx, `SELECT page_views FROM analytics_daily WHERE resource_key='site'`).Scan(&pv); e != nil || pv != 3 {
		t.Fatal(pv, e)
	}
}
func TestOutboxLeaseAndDurableConsumption(t *testing.T) {
	a := testApp(t)
	a.RegisterContent()
	ctx := context.Background()
	_, cookie, csrf := testIdentity(t, a, "owner")
	interactionPublished(t, a, cookie, csrf)
	if e := a.ConsumeOutbox(ctx); e != nil {
		t.Fatal(e)
	}
	var created, published int64
	if e := a.Pool.QueryRow(ctx, `SELECT posts_created,posts_published FROM business_daily`).Scan(&created, &published); e != nil || created != 1 || published != 1 {
		t.Fatal(created, published, e)
	}
	if _, e := a.Pool.Exec(ctx, `UPDATE outbox SET state='pending' WHERE state='done'`); e != nil {
		t.Fatal(e)
	}
	if e := a.ConsumeOutbox(ctx); e != nil {
		t.Fatal(e)
	}
	if e := a.Pool.QueryRow(ctx, `SELECT posts_created,posts_published FROM business_daily`).Scan(&created, &published); e != nil || created != 1 || published != 1 {
		t.Fatal("redelivery counted twice", created, published, e)
	}
	var id, topic string
	var body []byte
	if e := a.Pool.QueryRow(ctx, `UPDATE outbox SET state='processing',lease_token='new-worker',lease_until_ms=$1 WHERE id=(SELECT id FROM outbox LIMIT 1) RETURNING id,topic,payload_json`, NowMillis()+30000).Scan(&id, &topic, &body); e != nil {
		t.Fatal(e)
	}
	if e := a.analyticsConsumeFact(ctx, analyticsTask{id: id, topic: topic, token: "stale-worker", payload: body}); e != nil {
		t.Fatal(e)
	}
	var token string
	if e := a.Pool.QueryRow(ctx, `SELECT lease_token FROM outbox WHERE id=$1`, id).Scan(&token); e != nil || token != "new-worker" {
		t.Fatal("stale worker confirmed lease", token, e)
	}
	exhausted := RandID()
	payload, _ := json.Marshal(map[string]any{"event_id": exhausted, "schema_version": 1, "event_type": "comment.created.v1", "occurred_at_ms": NowMillis()})
	if _, e := a.Pool.Exec(ctx, `INSERT INTO outbox(id,topic,aggregate_id,dedupe_key,payload_json,state,attempts,available_at_ms,lease_token,lease_until_ms,created_at_ms) VALUES($1,'comment.created.v1','test',$1,$2,'processing',5,0,'crashed',0,$3)`, exhausted, payload, NowMillis()); e != nil {
		t.Fatal(e)
	}
	if e := a.ConsumeOutbox(ctx); e != nil {
		t.Fatal(e)
	}
	var state string
	if e := a.Pool.QueryRow(ctx, `SELECT state FROM outbox WHERE id=$1`, exhausted).Scan(&state); e != nil || state != "dead" {
		t.Fatal("exhausted lease was retried indefinitely", state, e)
	}
}

func TestWorkerHonorsBackupMaintenanceGate(t *testing.T) {
	a := testApp(t)
	ctx := context.Background()
	tx, e := a.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT id FROM policy_barrier WHERE id=1 FOR UPDATE`); e != nil {
		t.Fatal(e)
	}
	deadline, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if e = a.RollupAnalytics(deadline); e == nil {
		t.Fatal("worker ignored active maintenance gate")
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = a.RollupAnalytics(ctx); e != nil {
		t.Fatal(e)
	}
}
