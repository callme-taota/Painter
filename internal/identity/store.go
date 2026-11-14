package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

type Queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Hooks struct {
	Encrypt        func([]byte) ([]byte, error)
	Decrypt        func([]byte) ([]byte, error)
	SessionAllowed func(context.Context, Queryer, string, string, *time.Time) error
}
type Store struct {
	Pool  *pgxpool.Pool
	Hooks Hooks
}

func New(pool *pgxpool.Pool, h Hooks) *Store { return &Store{Pool: pool, Hooks: h} }
func invalidGrant() error                    { return oidc.ErrInvalidGrant() }

func (s *Store) GetClient(ctx context.Context, id string) (*Client, error) {
	c := new(Client)
	var raw []byte
	err := s.Pool.QueryRow(ctx, "SELECT id,name,kind,redirect_uris,disabled_at_ms,created_at_ms FROM identity_clients WHERE id=$1", id).Scan(&c.ID, &c.Name, &c.Kind, &raw, &c.DisabledAt, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	if c.DisabledAt != nil {
		return nil, oidc.ErrInvalidClient()
	}
	err = json.Unmarshal(raw, &c.Redirects)
	return c, err
}
func (s *Store) GetClientByClientID(ctx context.Context, id string) (op.Client, error) {
	return s.GetClient(ctx, id)
}
func (s *Store) AuthorizeClientIDSecret(ctx context.Context, id, secret string) error {
	var stored string
	err := s.Pool.QueryRow(ctx, "SELECT secret_hash FROM identity_clients WHERE id=$1 AND kind='web' AND disabled_at_ms IS NULL", id).Scan(&stored)
	if err != nil || len(secret) < 32 || !hmac.Equal([]byte(stored), []byte(Hash(secret))) {
		return oidc.ErrInvalidClient()
	}
	return nil
}

func (s *Store) CreateAuthRequest(ctx context.Context, in *oidc.AuthRequest, _ string) (op.AuthRequest, error) {
	c, err := s.GetClient(ctx, in.ClientID)
	if err != nil {
		return nil, err
	}
	if !c.RedirectAllowed(in.RedirectURI) {
		return nil, oidc.ErrInvalidRequestRedirectURI()
	}
	if in.ResponseType != oidc.ResponseTypeCode || in.ResponseMode != "" && in.ResponseMode != oidc.ResponseModeQuery {
		return nil, oidc.ErrInvalidRequest().WithDescription("only code response_type with query response_mode is supported")
	}
	challenge, err := base64.RawURLEncoding.DecodeString(in.CodeChallenge)
	if err != nil || len(challenge) != 32 || in.CodeChallengeMethod != oidc.CodeChallengeMethodS256 {
		return nil, oidc.ErrInvalidRequest().WithDescription("PKCE S256 is required for every client")
	}
	if len(in.State) > 1024 || len(in.Nonce) > 1024 || len(in.Scopes) > 3 || !slices.Contains(in.Scopes, "openid") {
		return nil, oidc.ErrInvalidScope()
	}
	for _, scope := range in.Scopes {
		if !c.IsScopeAllowed(scope) {
			return nil, oidc.ErrInvalidScope()
		}
	}
	for _, prompt := range in.Prompt {
		if prompt == oidc.PromptNone {
			return nil, oidc.ErrInteractionRequired().WithDescription("explicit consent is required")
		}
		if prompt != oidc.PromptConsent && prompt != oidc.PromptLogin {
			return nil, oidc.ErrInvalidRequest().WithDescription("unsupported prompt")
		}
		if prompt == oidc.PromptLogin {
			in.MaxAge = oidc.NewMaxAge(0)
		}
	}
	req := &Request{ID: Random(), ClientID: in.ClientID, Redirect: in.RedirectURI, State: in.State, Nonce: in.Nonce, Scopes: in.Scopes, Challenge: in.CodeChallenge, CreatedAt: time.Now().UTC(), MaxAge: in.MaxAge}
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SET LOCAL lock_timeout='2s'"); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(7284923151125)"); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "DELETE FROM identity_auth_requests WHERE id IN (SELECT id FROM identity_auth_requests WHERE expires_at_ms<$1 LIMIT 100)", time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	var active int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM identity_auth_requests WHERE expires_at_ms>$1", time.Now().UnixMilli()).Scan(&active); err != nil {
		return nil, err
	}
	if active >= 1000 {
		return nil, oidc.ErrServerError().WithDescription("authorization capacity reached; retry later")
	}
	_, err = tx.Exec(ctx, "INSERT INTO identity_auth_requests(id,client_id,request_json,created_at_ms,expires_at_ms) VALUES($1,$2,$3,$4,$5)", req.ID, req.ClientID, raw, req.CreatedAt.UnixMilli(), req.CreatedAt.Add(10*time.Minute).UnixMilli())
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err == nil {
		// Bounded opportunistic cleanup; refresh tombstones remain until their grant's
		// absolute expiry so a replay can still revoke the complete token family.
		_, _ = s.Pool.Exec(ctx, "DELETE FROM identity_grants WHERE id IN (SELECT id FROM identity_grants WHERE expires_at_ms<$1 LIMIT 100)", time.Now().UnixMilli())
	}
	return req, err
}
func (s *Store) loadRequest(ctx context.Context, query string, arg any) (*Request, error) {
	var raw []byte
	err := s.Pool.QueryRow(ctx, query, arg, time.Now().UnixMilli()).Scan(&raw)
	if err != nil {
		return nil, invalidGrant()
	}
	r := new(Request)
	err = json.Unmarshal(raw, r)
	return r, err
}
func (s *Store) RequestByID(ctx context.Context, id string) (*Request, error) {
	return s.loadRequest(ctx, "SELECT request_json FROM identity_auth_requests WHERE id=$1 AND expires_at_ms>$2", id)
}
func (s *Store) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	return s.RequestByID(ctx, id)
}
func (s *Store) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	return s.loadRequest(ctx, "SELECT request_json FROM identity_auth_requests WHERE code_hash=$1 AND code_used_at_ms IS NULL AND expires_at_ms>$2", Hash(code))
}
func (s *Store) SaveAuthCode(ctx context.Context, id, code string) error {
	tag, err := s.Pool.Exec(ctx, "UPDATE identity_auth_requests SET code_hash=$2,expires_at_ms=LEAST(expires_at_ms,$3) WHERE id=$1 AND user_id IS NOT NULL AND code_hash IS NULL AND expires_at_ms>$4", id, Hash(code), time.Now().Add(time.Minute).UnixMilli(), time.Now().UnixMilli())
	if err == nil && tag.RowsAffected() != 1 {
		return invalidGrant()
	}
	return err
}
func (s *Store) DeleteAuthRequest(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, "DELETE FROM identity_auth_requests WHERE id=$1", id)
	return err
}

// CompleteTx is called by the host's same-origin CSRF transaction. Session,
// client, and policy locks remain held until that transaction commits.
func (s *Store) CompleteTx(ctx context.Context, tx pgx.Tx, req *Request) error {
	if err := s.guard(ctx, tx, req); err != nil {
		return err
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, "UPDATE identity_auth_requests SET request_json=$2,user_id=$3,session_id=$4 WHERE id=$1 AND user_id IS NULL AND code_hash IS NULL AND expires_at_ms>$5", req.ID, raw, req.Subject, req.SessionID, time.Now().UnixMilli())
	if err == nil && tag.RowsAffected() != 1 {
		return invalidGrant()
	}
	return err
}

func (s *Store) guard(ctx context.Context, tx pgx.Tx, r *Request) error {
	var id string
	if _, err := tx.Exec(ctx, "SELECT id FROM policy_barrier WHERE id=1 FOR SHARE"); err != nil {
		return err
	}
	err := tx.QueryRow(ctx, "SELECT s.management_id FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.management_id=$1 AND u.id=$2 AND s.revoked_at_ms IS NULL AND s.expires_at_ms>$3 AND u.disabled_at_ms IS NULL FOR SHARE OF s,u", r.SessionID, r.Subject, time.Now().UnixMilli()).Scan(&id)
	if err != nil {
		return invalidGrant()
	}
	if err = s.Hooks.SessionAllowed(ctx, tx, r.Subject, r.SessionID, r.MFAAt); err != nil {
		return invalidGrant()
	}
	err = tx.QueryRow(ctx, "SELECT id FROM identity_clients WHERE id=$1 AND disabled_at_ms IS NULL FOR SHARE", r.ClientID).Scan(&id)
	if err != nil {
		return invalidGrant()
	}
	return nil
}
func (s *Store) issue(ctx context.Context, request op.TokenRequest, current string, wantRefresh bool) (string, string, time.Time, error) {
	r, ok := request.(*Request)
	if refresh, yes := request.(*RefreshRequest); yes {
		r = refresh.Request
		ok = true
	}
	if !ok || !r.Complete {
		return "", "", time.Time{}, invalidGrant()
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", "", time.Time{}, err
	}
	defer tx.Rollback(context.Background())
	if err = s.guard(ctx, tx, r); err != nil {
		return "", "", time.Time{}, err
	}
	now := time.Now().UTC()
	expiry := now.Add(AccessLifetime)
	grantID := Random()
	grantExpiry := now.Add(GrantLifetime).UnixMilli()
	if current == "" {
		tag, e := tx.Exec(ctx, "UPDATE identity_auth_requests SET code_used_at_ms=$2 WHERE id=$1 AND code_hash IS NOT NULL AND code_used_at_ms IS NULL AND expires_at_ms>$2", r.ID, now.UnixMilli())
		if e != nil {
			return "", "", time.Time{}, e
		}
		if tag.RowsAffected() != 1 {
			return "", "", time.Time{}, invalidGrant()
		}
		_, err = tx.Exec(ctx, "INSERT INTO identity_grants(id,client_id,user_id,session_id,created_at_ms,expires_at_ms) VALUES($1,$2,$3,$4,$5,$6)", grantID, r.ClientID, r.Subject, r.SessionID, now.UnixMilli(), grantExpiry)
	} else {
		var used, revoked *int64
		var tokenExpiry int64
		err = tx.QueryRow(ctx, "SELECT t.grant_id,t.used_at_ms,g.revoked_at_ms,t.expires_at_ms,g.expires_at_ms FROM identity_tokens t JOIN identity_grants g ON g.id=t.grant_id WHERE t.id=$1 AND t.kind='refresh' AND g.client_id=$2 AND g.user_id=$3 FOR UPDATE OF t,g", Hash(current), r.ClientID, r.Subject).Scan(&grantID, &used, &revoked, &tokenExpiry, &grantExpiry)
		if err != nil {
			return "", "", time.Time{}, invalidGrant()
		}
		if used != nil {
			_, err = tx.Exec(ctx, "UPDATE identity_grants SET revoked_at_ms=$2 WHERE id=$1", grantID, now.UnixMilli())
			if err == nil {
				err = tx.Commit(ctx)
			}
			if err != nil {
				return "", "", time.Time{}, err
			}
			return "", "", time.Time{}, invalidGrant()
		}
		if revoked != nil || tokenExpiry <= now.UnixMilli() || grantExpiry <= now.UnixMilli() {
			return "", "", time.Time{}, invalidGrant()
		}
		_, err = tx.Exec(ctx, "UPDATE identity_tokens SET used_at_ms=$2 WHERE id=$1", Hash(current), now.UnixMilli())
	}
	if err != nil {
		return "", "", time.Time{}, err
	}
	accessID := Random()
	raw, err := json.Marshal(r)
	if err != nil {
		return "", "", time.Time{}, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO identity_tokens(id,grant_id,kind,request_json,created_at_ms,expires_at_ms) VALUES($1,$2,'access',$3,$4,$5)", accessID, grantID, raw, now.UnixMilli(), expiry.UnixMilli())
	if err != nil {
		return "", "", time.Time{}, err
	}
	refresh := ""
	if wantRefresh {
		refresh = Random()
		_, err = tx.Exec(ctx, "INSERT INTO identity_tokens(id,grant_id,kind,request_json,created_at_ms,expires_at_ms) VALUES($1,$2,'refresh',$3,$4,$5)", Hash(refresh), grantID, raw, now.UnixMilli(), grantExpiry)
		if err != nil {
			return "", "", time.Time{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", time.Time{}, err
	}
	return accessID, refresh, expiry, nil
}
func (s *Store) CreateAccessToken(ctx context.Context, r op.TokenRequest) (string, time.Time, error) {
	id, _, expiry, err := s.issue(ctx, r, "", false)
	return id, expiry, err
}
func (s *Store) CreateAccessAndRefreshTokens(ctx context.Context, r op.TokenRequest, current string) (string, string, time.Time, error) {
	return s.issue(ctx, r, current, true)
}
func (s *Store) TokenRequestByRefreshToken(ctx context.Context, token string) (op.RefreshTokenRequest, error) {
	var raw []byte
	var used *int64
	var grant string
	err := s.Pool.QueryRow(ctx, "SELECT t.request_json,t.used_at_ms,g.id FROM identity_tokens t JOIN identity_grants g ON g.id=t.grant_id WHERE t.id=$1 AND t.kind='refresh' AND t.revoked_at_ms IS NULL AND g.revoked_at_ms IS NULL AND t.expires_at_ms>$2 AND g.expires_at_ms>$2", Hash(token), time.Now().UnixMilli()).Scan(&raw, &used, &grant)
	if err != nil {
		return nil, op.ErrInvalidRefreshToken
	}
	if used != nil {
		_, err = s.Pool.Exec(ctx, "UPDATE identity_grants SET revoked_at_ms=$2 WHERE id=$1", grant, time.Now().UnixMilli())
		if err != nil {
			return nil, err
		}
		return nil, op.ErrInvalidRefreshToken
	}
	r := new(Request)
	if err = json.Unmarshal(raw, r); err != nil {
		return nil, err
	}
	if err = s.Hooks.SessionAllowed(ctx, s.Pool, r.Subject, r.SessionID, r.MFAAt); err != nil {
		return nil, op.ErrInvalidRefreshToken
	}
	return &RefreshRequest{Request: r}, nil
}
func (s *Store) TerminateSession(ctx context.Context, userID, clientID string) error {
	_, err := s.Pool.Exec(ctx, "UPDATE identity_grants SET revoked_at_ms=$3 WHERE user_id=$1 AND client_id=$2 AND revoked_at_ms IS NULL", userID, clientID, time.Now().UnixMilli())
	return err
}
func (s *Store) GetRefreshTokenInfo(ctx context.Context, clientID, token string) (string, string, error) {
	var user string
	err := s.Pool.QueryRow(ctx, "SELECT g.user_id::text FROM identity_tokens t JOIN identity_grants g ON g.id=t.grant_id WHERE t.id=$1 AND t.kind='refresh' AND g.client_id=$2", Hash(token), clientID).Scan(&user)
	if err != nil {
		return "", "", op.ErrInvalidRefreshToken
	}
	return user, Hash(token), nil
}
func (s *Store) RevokeToken(ctx context.Context, token, userID, clientID string) *oidc.Error {
	_, err := s.Pool.Exec(ctx, "UPDATE identity_grants SET revoked_at_ms=$3 WHERE client_id=$2 AND id IN (SELECT grant_id FROM identity_tokens WHERE id=$1 OR id=$4)", token, clientID, time.Now().UnixMilli(), Hash(token))
	if err != nil {
		return oidc.ErrServerError()
	}
	return nil
}

func (s *Store) tokenRequest(ctx context.Context, id string) (*Request, int64, error) {
	var raw []byte
	var expiry int64
	err := s.Pool.QueryRow(ctx, "SELECT t.request_json,t.expires_at_ms FROM identity_tokens t JOIN identity_grants g ON g.id=t.grant_id JOIN identity_clients c ON c.id=g.client_id WHERE t.id=$1 AND t.kind='access' AND t.revoked_at_ms IS NULL AND t.expires_at_ms>$2 AND g.revoked_at_ms IS NULL AND g.expires_at_ms>$2 AND c.disabled_at_ms IS NULL", id, time.Now().UnixMilli()).Scan(&raw, &expiry)
	if err != nil {
		return nil, 0, invalidGrant()
	}
	r := new(Request)
	if err = json.Unmarshal(raw, r); err != nil {
		return nil, 0, err
	}
	if err = s.Hooks.SessionAllowed(ctx, s.Pool, r.Subject, r.SessionID, r.MFAAt); err != nil {
		return nil, 0, invalidGrant()
	}
	return r, expiry, nil
}
func (s *Store) setUserinfo(ctx context.Context, info *oidc.UserInfo, r *Request, scopes []string) error {
	if err := s.Hooks.SessionAllowed(ctx, s.Pool, r.Subject, r.SessionID, r.MFAAt); err != nil {
		return invalidGrant()
	}
	info.Subject = r.Subject
	if slices.Contains(scopes, "profile") {
		return s.Pool.QueryRow(ctx, "SELECT username,display_name FROM users WHERE id=$1 AND disabled_at_ms IS NULL", r.Subject).Scan(&info.PreferredUsername, &info.Name)
	}
	return nil
}
func (s *Store) SetUserinfoFromScopes(context.Context, *oidc.UserInfo, string, string, []string) error {
	return nil
}
func (s *Store) SetUserinfoFromRequest(ctx context.Context, info *oidc.UserInfo, in op.IDTokenRequest, scopes []string) error {
	r, ok := in.(*Request)
	if refresh, yes := in.(*RefreshRequest); yes {
		r = refresh.Request
		ok = true
	}
	if !ok {
		return invalidGrant()
	}
	return s.setUserinfo(ctx, info, r, scopes)
}
func (s *Store) SetUserinfoFromToken(ctx context.Context, info *oidc.UserInfo, id, subject, origin string) error {
	r, _, err := s.tokenRequest(ctx, id)
	if err != nil || r.Subject != subject {
		return invalidGrant()
	}
	return s.setUserinfo(ctx, info, r, r.Scopes)
}
func (s *Store) SetIntrospectionFromToken(ctx context.Context, out *oidc.IntrospectionResponse, id, subject, clientID string) error {
	r, expiry, err := s.tokenRequest(ctx, id)
	if err != nil || r.Subject != subject || r.ClientID != clientID {
		return invalidGrant()
	}
	info := new(oidc.UserInfo)
	if err = s.setUserinfo(ctx, info, r, r.Scopes); err != nil {
		return err
	}
	out.SetUserInfo(info)
	out.Expiration = oidc.FromTime(time.UnixMilli(expiry))
	out.Scope = r.Scopes
	out.ClientID = r.ClientID
	return nil
}
func (s *Store) GetPrivateClaimsFromScopes(context.Context, string, string, []string) (map[string]any, error) {
	return nil, nil
}
func (s *Store) GetKeyByIDAndClientID(context.Context, string, string) (*jose.JSONWebKey, error) {
	return nil, oidc.ErrInvalidClient()
}
func (s *Store) ValidateJWTProfileScopes(context.Context, string, []string) ([]string, error) {
	return nil, oidc.ErrUnsupportedGrantType()
}
func (s *Store) Health(ctx context.Context) error { return s.Pool.Ping(ctx) }

type signingKey struct {
	id      string
	private *rsa.PrivateKey
}

func (k *signingKey) ID() string                                  { return k.id }
func (k *signingKey) SignatureAlgorithm() jose.SignatureAlgorithm { return jose.RS256 }
func (k *signingKey) Key() any                                    { return k.private }

type publicKey struct{ *signingKey }

func (k *publicKey) Algorithm() jose.SignatureAlgorithm { return jose.RS256 }
func (k *publicKey) Use() string                        { return "sig" }
func (k *publicKey) Key() any                           { return &k.private.PublicKey }
func (s *Store) EnsureSigningKey(ctx context.Context) error {
	var n int
	if err := s.Pool.QueryRow(ctx, "SELECT count(*) FROM identity_signing_keys").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		_, err := s.SigningKey(ctx)
		return err
	}
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	ciphertext, err := s.Hooks.Encrypt(x509.MarshalPKCS1PrivateKey(private))
	if err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(7284923151124)"); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO identity_signing_keys(id,private_key_ciphertext,created_at_ms) SELECT $1,$2,$3 WHERE NOT EXISTS(SELECT 1 FROM identity_signing_keys)", Random(), ciphertext, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) readKey(id string, ciphertext []byte) (*signingKey, error) {
	plain, err := s.Hooks.Decrypt(ciphertext)
	if err != nil {
		return nil, errors.New("identity signing key cannot be decrypted; restore the original deployment encryption key")
	}
	key, err := x509.ParsePKCS1PrivateKey(plain)
	if err != nil {
		return nil, err
	}
	return &signingKey{id: id, private: key}, nil
}
func (s *Store) SigningKey(ctx context.Context) (op.SigningKey, error) {
	var id string
	var ciphertext []byte
	err := s.Pool.QueryRow(ctx, "SELECT id,private_key_ciphertext FROM identity_signing_keys ORDER BY created_at_ms DESC,id DESC LIMIT 1").Scan(&id, &ciphertext)
	if err != nil {
		return nil, err
	}
	return s.readKey(id, ciphertext)
}
func (s *Store) SignatureAlgorithms(context.Context) ([]jose.SignatureAlgorithm, error) {
	return []jose.SignatureAlgorithm{jose.RS256}, nil
}
func (s *Store) KeySet(ctx context.Context) ([]op.Key, error) {
	rows, err := s.Pool.Query(ctx, "SELECT id,private_key_ciphertext FROM identity_signing_keys ORDER BY created_at_ms DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []op.Key{}
	for rows.Next() {
		var id string
		var cipher []byte
		if err = rows.Scan(&id, &cipher); err != nil {
			return nil, err
		}
		key, e := s.readKey(id, cipher)
		if e != nil {
			return nil, e
		}
		keys = append(keys, &publicKey{key})
	}
	return keys, rows.Err()
}

var _ op.Storage = (*Store)(nil)
var _ op.CanSetUserinfoFromRequest = (*Store)(nil)
