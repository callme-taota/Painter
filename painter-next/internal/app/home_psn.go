package app

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type psnConnection struct {
	Version, CredentialVersion int64
	OnlineID, AccountID        string
	Cipher                     []byte
	Auto, Requested            bool
	LastSync                   *int64
	LastError                  string
	NextSync                   int64
	Lease                      string
	Until                      *int64
	Failures                   int
}

func psnRead(ctx context.Context, q homeQuery, lock bool) (psnConnection, error) {
	var connection psnConnection
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	err := q.QueryRow(ctx, `SELECT version,credential_version,online_id,account_id,token_ciphertext,auto_sync,sync_requested,last_sync_at_ms,last_error,next_sync_at_ms,COALESCE(lease_token,''),lease_until_ms,sync_failures FROM homepage_game_connections WHERE provider='psn'`+suffix).Scan(&connection.Version, &connection.CredentialVersion, &connection.OnlineID, &connection.AccountID, &connection.Cipher, &connection.Auto, &connection.Requested, &connection.LastSync, &connection.LastError, &connection.NextSync, &connection.Lease, &connection.Until, &connection.Failures)
	return connection, err
}

func psnLock(ctx context.Context, tx pgx.Tx) (psnConnection, error) {
	// All homepage writers serialize capacity and item/media changes through
	// this settings row. Always acquire it before the private PSN connection;
	// credential-only updates deliberately do not compare its version.
	if _, err := homeRead(ctx, tx, true); err != nil {
		return psnConnection{}, err
	}
	return psnRead(ctx, tx, true)
}

func (a *App) psnDecode(connection psnConnection) (psnTokens, error) {
	var tokens psnTokens
	raw, err := a.decrypt(connection.Cipher)
	if err != nil || json.Unmarshal(raw, &tokens) != nil || tokens.Type != "psn_tokens_v1" || !psnSecret(tokens.Access) || !psnSecret(tokens.Refresh) || tokens.AccessExpires <= 0 || tokens.RefreshExpires <= 0 {
		return psnTokens{}, psnReauth()
	}
	return tokens, nil
}

func (a *App) psnDTO(ctx context.Context, q homeQuery, connection psnConnection) (map[string]any, error) {
	count := 0
	if err := q.QueryRow(ctx, `SELECT count(*) FROM homepage_items WHERE source='psn_sync' AND platform='psn'`).Scan(&count); err != nil {
		return nil, err
	}
	connected := len(connection.Cipher) > 0 && connection.AccountID != ""
	status := "idle"
	lastError := connection.LastError
	if connected {
		tokens, err := a.psnDecode(connection)
		if err != nil || tokens.RefreshExpires <= NowMillis() || lastError == psnReauthMessage {
			status = "reauth_required"
			lastError = psnReauthMessage
		}
	}
	if status != "reauth_required" {
		switch {
		case connection.Lease != "" && connection.Until != nil && *connection.Until > NowMillis():
			status = "syncing"
		case connection.Requested:
			status = "queued"
		case lastError != "":
			status = "error"
		}
	}
	var lastSync any
	if connection.LastSync != nil {
		lastSync = TimeString(*connection.LastSync)
	}
	var nextSync any
	if connection.NextSync > 0 {
		nextSync = TimeString(connection.NextSync)
	}
	return map[string]any{"version": connection.Version, "connected": connected, "online_id": connection.OnlineID, "account_id": connection.AccountID, "auto_sync": connection.Auto, "status": status, "last_sync_at": lastSync, "last_error": lastError, "next_sync_at": nextSync, "game_count": count}, nil
}

func psnVersionError() error {
	return E(409, "VERSION_CONFLICT", "PSN 配置已变化，请重新加载后重试")
}
func psnBusy() error {
	return E(409, "PSN_SYNC_BUSY", "PSN 连接或同步正在进行，请稍后查看")
}

// Call only after psnLock: the settings lock serializes this snapshot check
// with every manual/provider item writer. A wall stores one PSN account's
// snapshot; absent an item-level account key, never silently mix another one.
func psnCheckAccount(ctx context.Context, q homeQuery, current psnConnection, account string) error {
	if current.AccountID == "" || current.AccountID == account {
		return nil
	}
	var count int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM homepage_items WHERE source='psn_sync'`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return E(409, "PSN_ACCOUNT_CHANGE_REQUIRES_CLEANUP", "PSN 账号与已保存的游戏墙不匹配，请先清理 PSN 同步快照或由管理员维护后再换绑")
	}
	return nil
}

func (a *App) homePSNGet(w http.ResponseWriter, r *http.Request) error {
	if _, err := a.Require(r, "owner"); err != nil {
		return err
	}
	tx, err := a.Pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(r.Context(), `SELECT id FROM policy_barrier WHERE id=1 FOR SHARE`); err != nil {
		return err
	}
	p, err := a.txPrincipal(r.Context(), tx, r, "SHARE")
	if err != nil {
		return err
	}
	if p.Role != "owner" {
		return E(403, "PERMISSION_DENIED", "没有执行此操作的权限")
	}
	connection, err := psnRead(r.Context(), tx, false)
	if err != nil {
		return err
	}
	value, err := a.psnDTO(r.Context(), tx, connection)
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	JSON(w, 200, value)
	return nil
}

func psnSecureInput(r *http.Request, base string) bool {
	if r.TLS != nil {
		return true
	}
	if configured, err := url.Parse(base); err == nil && configured.Scheme == "https" && configured.Host != "" {
		origin, err := url.Parse(r.Header.Get("Origin"))
		if err == nil && origin.Scheme == "https" && origin.Host == configured.Host && origin.User == nil && origin.Path == "" && origin.RawQuery == "" && origin.Fragment == "" {
			return true
		}
	}
	host := r.Host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return false
	}
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	remote := net.ParseIP(peer)
	return remote != nil && remote.IsLoopback()
}

func (a *App) homePSNPut(w http.ResponseWriter, r *http.Request) error {
	if err := homeAuthorize(a, r); err != nil {
		return err
	}
	version, err := IfMatch(r)
	if err != nil {
		return err
	}
	var input struct {
		NPSSO      *string `json:"npsso"`
		Auto       bool    `json:"auto_sync"`
		Disconnect bool    `json:"disconnect"`
		OnlineID   *string `json:"online_id"`
	}
	if err = decodeHome(w, r, &input); err != nil {
		return err
	}
	if input.OnlineID != nil && (homeText(*input.OnlineID, 80, false) != nil || strings.TrimSpace(*input.OnlineID) != *input.OnlineID) {
		return E(400, "VALIDATION_FAILED", "PSN Online ID 格式无效")
	}
	if input.Disconnect && (input.NPSSO != nil || input.Auto) {
		return E(400, "VALIDATION_FAILED", "断开连接时不能同时提供凭证或开启自动同步")
	}
	if input.NPSSO != nil && !psnNPSSO(*input.NPSSO) {
		return E(400, "VALIDATION_FAILED", "请输入64字符的有效 PSN 连接凭证")
	}
	if input.NPSSO != nil && !psnSecureInput(r, a.Config.PublicBaseURL) {
		return E(409, "PSN_SECURE_CONNECTION_REQUIRED", "请通过 HTTPS 或服务器本机安全入口连接 PSN，不要在普通局域网 HTTP 中提交凭证")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if value, found, err := a.Replay(r, "owner"); err != nil {
		return err
	} else if found {
		JSON(w, 200, value)
		return nil
	}
	if input.NPSSO == nil {
		value, err := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
			connection, err := psnLock(ctx, tx)
			if err != nil {
				return nil, err
			}
			if connection.Version != version {
				return nil, psnVersionError()
			}
			if !input.Disconnect && input.OnlineID != nil && connection.AccountID != "" && !strings.EqualFold(*input.OnlineID, connection.OnlineID) {
				return nil, E(400, "PSN_ACCOUNT_MISMATCH", "更换 PSN 账号需要重新连接并确认账号")
			}
			if input.Auto {
				tokens, err := a.psnDecode(connection)
				if err != nil || tokens.RefreshExpires <= NowMillis() {
					return nil, psnReauth()
				}
			}
			action := "home.psn_update"
			if input.Disconnect {
				_, err = tx.Exec(ctx, `UPDATE homepage_game_connections SET version=version+1,credential_version=credential_version+1,token_ciphertext=NULL,auto_sync=false,sync_requested=false,last_error='',next_sync_at_ms=0,lease_token=NULL,lease_until_ms=NULL,sync_failures=0 WHERE provider='psn'`)
				action = "home.psn_disconnect"
			} else {
				if connection.Lease != "" && connection.Until != nil && *connection.Until > NowMillis() {
					return nil, psnBusy()
				}
				online := connection.OnlineID
				if input.OnlineID != nil {
					online = *input.OnlineID
				}
				_, err = tx.Exec(ctx, `UPDATE homepage_game_connections SET version=version+1,online_id=$1,auto_sync=$2 WHERE provider='psn'`, online, input.Auto)
			}
			if err != nil {
				return nil, err
			}
			if err = audit(ctx, tx, p, action, "site", "homepage", ""); err != nil {
				return nil, err
			}
			connection, err = psnRead(ctx, tx, false)
			if err != nil {
				return nil, err
			}
			return a.psnDTO(ctx, tx, connection)
		})
		if err == nil {
			JSON(w, 200, value)
		}
		return err
	}
	claim, err := a.Write(r, []string{"owner"}, false, false, func(tx pgx.Tx, p Principal) (any, error) {
		connection, err := psnLock(ctx, tx)
		if err != nil {
			return nil, err
		}
		if connection.Version != version {
			return nil, psnVersionError()
		}
		if connection.Lease != "" && connection.Until != nil && *connection.Until > NowMillis() {
			return nil, psnBusy()
		}
		if connection.NextSync > NowMillis() && strings.HasPrefix(connection.LastError, "PSN 请求较频繁") {
			return nil, E(429, "PSN_RATE_LIMITED", connection.LastError)
		}
		if err = homeRateTx(ctx, tx, p.ID, "psn"); err != nil {
			return nil, err
		}
		connection.Lease = RandID()
		until := NowMillis() + 180000
		connection.Until = &until
		_, err = tx.Exec(ctx, `UPDATE homepage_game_connections SET lease_token=$1,lease_until_ms=$2 WHERE provider='psn'`, connection.Lease, until)
		return connection, err
	})
	if err != nil {
		return err
	}
	lease := claim.(psnConnection)
	client := a.psnClient(func(checkctx context.Context) error { return a.psnLeaseAlive(checkctx, lease) })
	tokens, err := client.exchange(ctx, *input.NPSSO)
	if err != nil {
		a.psnConnectRelease(lease, err)
		return psnSafeError(err)
	}
	account, online, err := client.identity(ctx, tokens)
	if err != nil {
		a.psnConnectRelease(lease, err)
		return psnSafeError(err)
	}
	expected := lease.OnlineID
	if input.OnlineID != nil {
		expected = *input.OnlineID
	}
	if expected != "" && !strings.EqualFold(expected, online) {
		a.psnConnectRelease(lease, nil)
		return E(409, "PSN_ACCOUNT_MISMATCH", "凭证所属 PSN 账号与填写的 Online ID 不一致，未修改已有连接")
	}
	raw, _ := json.Marshal(tokens)
	cipher, err := a.encrypt(raw)
	if err != nil {
		a.psnConnectRelease(lease, nil)
		return E(503, "HOME_KEY_UNAVAILABLE", "PSN 凭证无法加密，请检查服务器加密配置")
	}
	value, err := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		current, err := psnLock(ctx, tx)
		if err != nil {
			return nil, err
		}
		if !psnOwnsLease(current, lease) {
			return nil, psnVersionError()
		}
		if err = psnCheckAccount(ctx, tx, current, account); err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `UPDATE homepage_game_connections SET version=version+1,credential_version=credential_version+1,online_id=$1,account_id=$2,token_ciphertext=$3,auto_sync=$4,sync_requested=true,last_error='',next_sync_at_ms=0,lease_token=NULL,lease_until_ms=NULL,sync_failures=0 WHERE provider='psn'`, online, account, cipher, input.Auto)
		if err != nil {
			return nil, err
		}
		if err = audit(ctx, tx, p, "home.psn_connect", "site", "homepage", ""); err != nil {
			return nil, err
		}
		current, err = psnRead(ctx, tx, false)
		if err != nil {
			return nil, err
		}
		return a.psnDTO(ctx, tx, current)
	})
	if err != nil {
		a.psnConnectRelease(lease, nil)
		return err
	}
	JSON(w, 200, value)
	return nil
}

func psnOwnsLease(current, lease psnConnection) bool {
	return current.Version == lease.Version && current.Lease == lease.Lease && current.Lease != "" && current.Until != nil && *current.Until > NowMillis()
}

func (a *App) psnLeaseAlive(ctx context.Context, lease psnConnection) error {
	var valid bool
	err := a.Pool.QueryRow(ctx, `SELECT version=$1 AND lease_token=$2 AND lease_until_ms>$3 FROM homepage_game_connections WHERE provider='psn'`, lease.Version, lease.Lease, NowMillis()).Scan(&valid)
	if err != nil || !valid {
		return psnVersionError()
	}
	return nil
}

func (a *App) psnConnectRelease(lease psnConnection, failure error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_ = a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		if _, err := homeRead(ctx, tx, true); err != nil {
			return err
		}
		message := ""
		next := int64(0)
		var upstream *psnFailure
		if errors.As(failure, &upstream) {
			message = upstream.Message
			if upstream.Code == "PSN_RATE_LIMITED" {
				next = max(upstream.RetryAt, NowMillis()+15*time.Minute.Milliseconds())
			}
		}
		_, err := tx.Exec(ctx, `UPDATE homepage_game_connections SET lease_token=NULL,lease_until_ms=NULL,last_error=$3,next_sync_at_ms=CASE WHEN $4::bigint>0 THEN $4 ELSE next_sync_at_ms END WHERE provider='psn' AND version=$1 AND lease_token=$2`, lease.Version, lease.Lease, message, next)
		return err
	})
}

// ConnectHomePSNLocal is an operator-only entry point for a server CLI. It must
// never be exposed as an unauthenticated HTTP endpoint. NPSSO is exchanged once
// in memory; only encrypted, verified OAuth tokens are written to PostgreSQL.
func (a *App) ConnectHomePSNLocal(ctx context.Context, npsso, onlineID string, auto bool) (map[string]any, error) {
	if !psnNPSSO(npsso) {
		return nil, E(400, "VALIDATION_FAILED", "请输入64字符的有效 PSN 连接凭证")
	}
	if homeText(onlineID, 80, false) != nil || strings.TrimSpace(onlineID) != onlineID {
		return nil, E(400, "VALIDATION_FAILED", "PSN Online ID 格式无效")
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	var lease psnConnection
	err := a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		connection, err := psnLock(ctx, tx)
		if err != nil {
			return err
		}
		if connection.Lease != "" && connection.Until != nil && *connection.Until > NowMillis() {
			return psnBusy()
		}
		if connection.NextSync > NowMillis() && strings.HasPrefix(connection.LastError, "PSN 请求较频繁") {
			return E(429, "PSN_RATE_LIMITED", connection.LastError)
		}
		connection.Lease = RandID()
		until := NowMillis() + 180000
		connection.Until = &until
		_, err = tx.Exec(ctx, `UPDATE homepage_game_connections SET lease_token=$1,lease_until_ms=$2 WHERE provider='psn'`, connection.Lease, until)
		lease = connection
		return err
	})
	if err != nil {
		return nil, err
	}
	client := a.psnClient(func(checkctx context.Context) error { return a.psnLeaseAlive(checkctx, lease) })
	tokens, err := client.exchange(ctx, npsso)
	if err != nil {
		a.psnConnectRelease(lease, err)
		return nil, psnSafeError(err)
	}
	account, online, err := client.identity(ctx, tokens)
	if err != nil {
		a.psnConnectRelease(lease, err)
		return nil, psnSafeError(err)
	}
	expected := onlineID
	if expected == "" {
		expected = lease.OnlineID
	}
	if expected != "" && !strings.EqualFold(expected, online) {
		a.psnConnectRelease(lease, nil)
		return nil, E(409, "PSN_ACCOUNT_MISMATCH", "凭证所属 PSN 账号与填写的 Online ID 不一致，未修改已有连接")
	}
	raw, _ := json.Marshal(tokens)
	cipher, err := a.encrypt(raw)
	if err != nil {
		a.psnConnectRelease(lease, nil)
		return nil, E(503, "HOME_KEY_UNAVAILABLE", "PSN 凭证无法加密，请检查服务器加密配置")
	}
	var result map[string]any
	err = a.moduleWriteTx(ctx, func(tx pgx.Tx) error {
		current, err := psnLock(ctx, tx)
		if err != nil {
			return err
		}
		if !psnOwnsLease(current, lease) {
			return psnVersionError()
		}
		if err = psnCheckAccount(ctx, tx, current, account); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE homepage_game_connections SET version=version+1,credential_version=credential_version+1,online_id=$1,account_id=$2,token_ciphertext=$3,auto_sync=$4,sync_requested=true,last_error='',next_sync_at_ms=0,lease_token=NULL,lease_until_ms=NULL,sync_failures=0 WHERE provider='psn'`, online, account, cipher, auto); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(actor_user_id,action,target_type,target_id,request_id,reason,created_at_ms) VALUES(NULL,'home.psn_connect','site','homepage',$1,'服务器本机连接 PSN',$2)`, requestID(ctx), NowMillis()); err != nil {
			return err
		}
		current, err = psnRead(ctx, tx, false)
		if err == nil {
			result, err = a.psnDTO(ctx, tx, current)
		}
		return err
	})
	if err != nil {
		a.psnConnectRelease(lease, nil)
		return nil, err
	}
	return result, nil
}

func (a *App) homePSNSync(w http.ResponseWriter, r *http.Request) error {
	if err := homeAuthorize(a, r); err != nil {
		return err
	}
	version, err := IfMatch(r)
	if err != nil {
		return err
	}
	if err = momentCommandBody(w, r); err != nil {
		return err
	}
	if value, found, err := a.Replay(r, "owner"); err != nil {
		return err
	} else if found {
		JSON(w, 202, value)
		return nil
	}
	value, err := a.Write(r, []string{"owner"}, false, true, func(tx pgx.Tx, p Principal) (any, error) {
		connection, err := psnLock(r.Context(), tx)
		if err != nil {
			return nil, err
		}
		if connection.Version != version {
			return nil, psnVersionError()
		}
		tokens, err := a.psnDecode(connection)
		if err != nil || tokens.RefreshExpires <= NowMillis() {
			return nil, psnReauth()
		}
		if connection.Lease != "" && connection.Until != nil && *connection.Until > NowMillis() {
			return nil, psnBusy()
		}
		if connection.NextSync > NowMillis() && strings.HasPrefix(connection.LastError, "PSN 请求较频繁") {
			return nil, E(429, "PSN_RATE_LIMITED", connection.LastError)
		}
		if err = homeRateTx(r.Context(), tx, p.ID, "psn"); err != nil {
			return nil, err
		}
		if _, err = tx.Exec(r.Context(), `UPDATE homepage_game_connections SET version=version+1,sync_requested=true,last_error='',next_sync_at_ms=0 WHERE provider='psn'`); err != nil {
			return nil, err
		}
		if err = audit(r.Context(), tx, p, "home.psn_sync", "site", "homepage", "排队同步 PSN 游戏"); err != nil {
			return nil, err
		}
		connection, err = psnRead(r.Context(), tx, false)
		if err != nil {
			return nil, err
		}
		return a.psnDTO(r.Context(), tx, connection)
	})
	if err == nil {
		JSON(w, 202, value)
	}
	return err
}
