package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) psnClaim(ctx context.Context) (psnConnection, error) {
	var lease psnConnection
	err := a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		connection, err := psnLock(ctx, tx)
		if err != nil {
			return err
		}
		if (!connection.Requested && !connection.Auto) || connection.NextSync > NowMillis() || len(connection.Cipher) == 0 || connection.AccountID == "" || connection.LastError == psnReauthMessage {
			return nil
		}
		if connection.Lease != "" && connection.Until != nil && *connection.Until > NowMillis() {
			return nil
		}
		connection.Lease = RandID()
		until := NowMillis() + 180000
		connection.Until = &until
		_, err = tx.Exec(ctx, `UPDATE homepage_game_connections SET lease_token=$1,lease_until_ms=$2 WHERE provider='psn'`, connection.Lease, until)
		lease = connection
		return err
	})
	return lease, err
}

func (a *App) psnPersistTokens(ctx context.Context, lease *psnConnection, tokens psnTokens) error {
	raw, _ := json.Marshal(tokens)
	cipher, err := a.encrypt(raw)
	if err != nil {
		return E(503, "HOME_KEY_UNAVAILABLE", "PSN 凭证无法加密，请检查服务器加密配置")
	}
	err = a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		current, err := psnLock(ctx, tx)
		if err != nil {
			return err
		}
		if !psnOwnsLease(current, *lease) || current.CredentialVersion != lease.CredentialVersion {
			return psnVersionError()
		}
		_, err = tx.Exec(ctx, `UPDATE homepage_game_connections SET credential_version=credential_version+1,token_ciphertext=$1 WHERE provider='psn'`, cipher)
		return err
	})
	if err == nil {
		lease.CredentialVersion++
		lease.Cipher = cipher
	}
	return err
}

func (a *App) psnApply(ctx context.Context, lease psnConnection, items []psnGame) error {
	return a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		// psnLock acquires homepage_settings FOR UPDATE, then the connection.
		// Keep this global capacity lock through the item and sorted-media locks
		// and the final upsert, matching Steam/manual homepage write ordering.
		current, err := psnLock(ctx, tx)
		if err != nil {
			return err
		}
		if !psnOwnsLease(current, lease) || current.CredentialVersion != lease.CredentialVersion || current.AccountID != lease.AccountID {
			return psnVersionError()
		}
		if len(items) == 0 || len(items) > homeItemLimit {
			return psnUnavailable()
		}
		var existing int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM homepage_items`).Scan(&existing); err != nil {
			return err
		}
		raw, _ := json.Marshal(items)
		var added int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM jsonb_to_recordset($1::jsonb) AS x(provider_id text) WHERE NOT EXISTS(SELECT 1 FROM homepage_items h WHERE h.kind='steam' AND h.source='psn_sync' AND h.provider_id=x.provider_id)`, raw).Scan(&added); err != nil {
			return err
		}
		if existing+added > homeItemLimit {
			return E(409, "HOME_ITEM_LIMIT", "游戏与其它首页条目合计超过5000项，未修改已有内容")
		}
		if _, err = tx.Exec(ctx, `SELECT id FROM homepage_items WHERE source='psn_sync' ORDER BY id FOR UPDATE`); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `SELECT m.id FROM media_assets m WHERE EXISTS(SELECT 1 FROM homepage_items h WHERE h.media_id=m.id AND h.source='psn_sync') ORDER BY m.id FOR UPDATE`); err != nil {
			return err
		}
		// Full-fetch, upsert-only: privacy omissions or a removed upstream title
		// never erase saved games. Existing visibility/order/manual art are owned
		// by the site, not by the provider's next response.
		_, err = tx.Exec(ctx, `INSERT INTO homepage_items(kind,platform,platforms,title,alt,url,image_url,provider_id,source,position,visible,created_at_ms,updated_at_ms,hero_url,logo_url,playtime_minutes)
 SELECT 'steam','psn',platforms,title,alt,url,image_url,provider_id,'psn_sync',position,true,$2,$2,hero_url,logo_url,playtime_minutes FROM jsonb_to_recordset($1::jsonb) AS x(platforms text[],title text,alt text,url text,image_url text,provider_id text,position int,hero_url text,logo_url text,playtime_minutes bigint)
 ON CONFLICT(kind,source,provider_id) WHERE source='psn_sync' DO UPDATE SET
 title=CASE WHEN homepage_items.local_override THEN homepage_items.title ELSE EXCLUDED.title END,
 alt=CASE WHEN homepage_items.local_override THEN homepage_items.alt ELSE EXCLUDED.alt END,
 url=CASE WHEN homepage_items.local_override THEN homepage_items.url ELSE EXCLUDED.url END,
 image_url=CASE WHEN homepage_items.local_override THEN homepage_items.image_url ELSE COALESCE(NULLIF(EXCLUDED.image_url,''),homepage_items.image_url) END,
 hero_url=CASE WHEN homepage_items.local_override THEN homepage_items.hero_url ELSE COALESCE(NULLIF(EXCLUDED.hero_url,''),homepage_items.hero_url) END,
 logo_url=CASE WHEN homepage_items.local_override THEN homepage_items.logo_url ELSE COALESCE(NULLIF(EXCLUDED.logo_url,''),homepage_items.logo_url) END,
 platforms=EXCLUDED.platforms,playtime_minutes=EXCLUDED.playtime_minutes,version=homepage_items.version+1,updated_at_ms=EXCLUDED.updated_at_ms`, raw, NowMillis())
		if err != nil {
			return err
		}
		now := NowMillis()
		if _, err = tx.Exec(ctx, `UPDATE homepage_game_connections SET sync_requested=false,last_sync_at_ms=$1,last_error='',next_sync_at_ms=$2,lease_token=NULL,lease_until_ms=NULL,sync_failures=0 WHERE provider='psn'`, now, now+6*time.Hour.Milliseconds()); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_user_id,action,target_type,target_id,request_id,reason,created_at_ms) VALUES(NULL,'home.psn_sync','site','homepage',$1,'PSN 游戏快照同步完成',$2)`, requestID(ctx), now)
		return err
	})
}

func (a *App) psnFailed(lease psnConnection, failure error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	message := "PSN 同步未完成；已有游戏保持不变"
	next := NowMillis() + int64(15*time.Minute/time.Millisecond)
	var upstream *psnFailure
	if errors.As(failure, &upstream) {
		message = upstream.Message
		if upstream.Code == "PSN_UNAUTHORIZED" {
			message = psnReauthMessage
		}
		if upstream.RetryAt > next {
			next = upstream.RetryAt
		}
	}
	_ = a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		connection, err := psnLock(ctx, tx)
		if err != nil {
			return err
		}
		if connection.Version != lease.Version || connection.Lease != lease.Lease {
			return nil
		}
		delay := 15 * time.Minute * time.Duration(int64(1)<<min(connection.Failures, 4))
		if delay > 6*time.Hour {
			delay = 6 * time.Hour
		}
		if !errors.As(failure, &upstream) || upstream.Code != "PSN_RATE_LIMITED" {
			next = NowMillis() + delay.Milliseconds()
		}
		_, err = tx.Exec(ctx, `UPDATE homepage_game_connections SET sync_requested=false,last_error=$1,next_sync_at_ms=$2,sync_failures=LEAST(sync_failures+1,20),lease_token=NULL,lease_until_ms=NULL WHERE provider='psn'`, message, next)
		return err
	})
}

func (a *App) runHomePSNOnce(ctx context.Context) {
	lease, err := a.psnClaim(ctx)
	if err != nil || lease.Lease == "" {
		return
	}
	tokens, err := a.psnDecode(lease)
	if err != nil {
		a.psnFailed(lease, err)
		return
	}
	client := a.psnClient(func(checkctx context.Context) error { return a.psnLeaseAlive(checkctx, lease) })
	items, err := client.games(ctx, lease.AccountID, &tokens, func(fresh psnTokens) error { return a.psnPersistTokens(ctx, &lease, fresh) })
	if err == nil {
		err = a.psnApply(ctx, lease, items)
	}
	if err != nil {
		a.psnFailed(lease, err)
	}
}

// RunHomePSN has its own lifecycle and deadline. It must not be invoked inside
// the analytics worker's shared 20-second context or a synchronous HTTP write.
func (a *App) RunHomePSN(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		job, cancel := context.WithTimeout(ctx, 120*time.Second)
		a.runHomePSNOnce(job)
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
