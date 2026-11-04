package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func localRegisteredEvent(ctx context.Context, tx pgx.Tx, id string, now int64) error {
	eventID := RandID()
	envelope, err := json.Marshal(map[string]any{"event_id": eventID, "schema_version": 1, "event_type": "user.registered.v1", "aggregate_type": "user", "aggregate_id": id, "aggregate_version": 1, "actor_id": id, "occurred_at_ms": now, "payload": map[string]any{"provider": "local"}})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO outbox(id,topic,aggregate_id,dedupe_key,payload_json,available_at_ms,created_at_ms) VALUES($1,'user.registered.v1',$2,$3,$4,$5,$5)", eventID, id, "user.registered/"+id, envelope, now)
	return err
}

// CreateLocalUser is an explicit offline operation for an OS/database operator.
// It never changes an existing user's role or installs a default credential.
// The maintenance gate serializes owner creation with management and backups.
func (a *App) CreateLocalUser(ctx context.Context, username, display, password, role string) (map[string]any, error) {
	username, err := normalizeLocalUsername(username)
	if err != nil {
		return nil, err
	}
	display, err = localDisplayName(display)
	if err != nil {
		return nil, err
	}
	if !hasRole(role, []string{"owner", "author", "reader"}) {
		return nil, fmt.Errorf("role must be owner, author or reader")
	}
	encoded, err := hashPassword(ctx, password)
	if err != nil {
		return nil, err
	}
	tx, err := a.maintenance(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	var owners int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM users WHERE role='owner' AND disabled_at_ms IS NULL").Scan(&owners); err != nil {
		return nil, err
	}
	now := NowMillis()
	var id string
	if err = tx.QueryRow(ctx, "INSERT INTO users(username,display_name,role,created_at_ms,updated_at_ms) VALUES($1,$2,$3,$4,$4) RETURNING id::text", username, display, role, now).Scan(&id); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO local_accounts(user_id,password_hash,password_changed_at_ms,created_at_ms) VALUES($1,$2,$3,$3)", id, encoded, now); err != nil {
		return nil, err
	}
	if err = localRegisteredEvent(ctx, tx, id, now); err != nil {
		return nil, err
	}
	if role == "owner" {
		if err = markInstallation(ctx, tx, id, now); err != nil {
			return nil, err
		}
	}
	action := "user.update"
	if role == "owner" && owners == 0 {
		action = "system.bootstrap"
	}
	if _, err = tx.Exec(ctx, "INSERT INTO audit_logs(actor_user_id,action,target_type,target_id,request_id,reason,created_at_ms) VALUES(NULL,$1,'user',$2,$3,$4,$5)", action, id, requestID(ctx), "offline create-local-user; role="+role, now); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "username": username, "role": role, "created": true}, nil
}

// ResetLocalPassword deliberately has no HTTP counterpart. It may attach local
// credentials to an existing OAuth user when a trusted offline operator chooses
// that exact username. Disabled users stay disabled; every old session is revoked.
func (a *App) ResetLocalPassword(ctx context.Context, username, password string) (map[string]any, error) {
	username, err := normalizeLocalUsername(username)
	if err != nil {
		return nil, err
	}
	encoded, err := hashPassword(ctx, password)
	if err != nil {
		return nil, err
	}
	tx, err := a.maintenance(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	var id string
	var disabled *int64
	if err = tx.QueryRow(ctx, "SELECT id::text,disabled_at_ms FROM users WHERE lower(username)=$1 FOR UPDATE", username).Scan(&id, &disabled); err != nil {
		return nil, err
	}
	if disabled != nil {
		return nil, fmt.Errorf("disabled account cannot receive a password reset")
	}
	now := NowMillis()
	if _, err = tx.Exec(ctx, "INSERT INTO local_accounts(user_id,password_hash,password_changed_at_ms,created_at_ms) VALUES($1,$2,$3,$3) ON CONFLICT(user_id) DO UPDATE SET password_hash=EXCLUDED.password_hash,password_changed_at_ms=EXCLUDED.password_changed_at_ms", id, encoded, now); err != nil {
		return nil, err
	}
	tag, err := tx.Exec(ctx, "UPDATE sessions SET revoked_at_ms=$2 WHERE user_id=$1 AND revoked_at_ms IS NULL", id, now)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "UPDATE users SET version=version+1,updated_at_ms=$2 WHERE id=$1", id, now); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO audit_logs(actor_user_id,action,target_type,target_id,request_id,reason,created_at_ms) VALUES(NULL,'user.update','user',$1,$2,'offline local password reset; all sessions revoked',$3)", id, requestID(ctx), now); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "username": username, "password_reset": true, "sessions_revoked": tag.RowsAffected()}, nil
}
