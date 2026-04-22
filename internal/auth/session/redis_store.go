// Package session holds the Redis-backed gorilla/sessions Store used by
// OIDC, OAuth, and the admin session middleware. The cookie contains only a
// signed session ID; the session payload lives in Redis, which lets the
// server enumerate and revoke sessions that are currently in flight (security
// L finding: no concurrent session control).
package session

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/redis/go-redis/v9"
)

const (
	sessionKeyPrefix = "session:"
	userIndexPrefix  = "user-sessions:"
)

// LoginAtValueKey is the session-Values key under which the OIDC callback
// writes the login timestamp (unix seconds). Duplicated from
// authmiddleware.LoginAtSessionKey to avoid an import cycle.
const LoginAtValueKey = "login_at"

// UserIDValueKey is the session-Values key carrying the authenticated user's
// numeric id. Matches the historical gorilla/sessions CookieStore layout.
const UserIDValueKey = "user_id"

// ErrSessionTooLarge is returned when the encoded session payload exceeds
// maxPayloadBytes — a defense against pathologic growth of session.Values
// that would make Redis memory unbounded.
var ErrSessionTooLarge = errors.New("session payload too large")

const maxPayloadBytes = 64 << 10 // 64 KiB

// Store is a gorilla/sessions.Store backed by Redis. The HTTP cookie contains
// a securecookie-signed session ID; the Values map is gob-encoded into Redis
// under `session:<id>` with a TTL equal to the session MaxAge.
//
// For revocation, sessions that carry a user_id are additionally tracked in
// `user-sessions:<user_id>` (a Redis Set of session IDs). RevokeSession and
// RevokeUserSessions operate on those indexes.
type Store struct {
	client  *redis.Client
	codecs  []securecookie.Codec
	options *sessions.Options
	ttl     time.Duration
}

// New creates a Redis-backed session Store. keyPairs match gorilla's
// CookieStore constructor: each pair is a hash key + block key for signing
// (and optionally encrypting) the session ID cookie. TTL is applied to every
// Redis key; pass 0 to fall back to opts.MaxAge.
func New(client *redis.Client, ttl time.Duration, opts *sessions.Options, keyPairs ...[]byte) *Store {
	if opts == nil {
		opts = &sessions.Options{Path: "/"}
	}
	if ttl <= 0 && opts.MaxAge > 0 {
		ttl = time.Duration(opts.MaxAge) * time.Second
	}
	s := &Store{
		client:  client,
		codecs:  securecookie.CodecsFromPairs(keyPairs...),
		options: opts,
		ttl:     ttl,
	}
	for _, codec := range s.codecs {
		if sc, ok := codec.(*securecookie.SecureCookie); ok {
			sc.MaxAge(opts.MaxAge)
		}
	}
	return s
}

// Get returns the session for the given name, loading from Redis when the
// request carries a cookie with a known session ID. Missing or invalid
// sessions come back IsNew=true with an empty Values map — same contract as
// gorilla's CookieStore.
func (s *Store) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

// New implements sessions.Store.New — always returns a fresh Session, optionally
// populated from the request's cookie. Errors from securecookie decoding are
// returned but do not prevent a usable (IsNew=true) session from being
// returned, matching CookieStore behavior.
func (s *Store) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	opts := *s.options
	session.Options = &opts
	session.IsNew = true

	// Every error path below collapses to "return a fresh session" — missing
	// cookie, signature failure, empty payload, expired Redis row. gorilla's
	// CookieStore has the same behavior; the middleware layer decides whether
	// to challenge the caller for auth based on IsNew.
	cookie, cookieErr := r.Cookie(name)
	if cookieErr != nil {
		return session, nil //nolint:nilerr // cookie-less requests are expected
	}

	var sessionID string
	if decodeErr := securecookie.DecodeMulti(name, cookie.Value, &sessionID, s.codecs...); decodeErr != nil {
		return session, nil //nolint:nilerr // signature mismatch → treat as fresh
	}
	if sessionID == "" {
		return session, nil
	}

	loaded, loadErr := s.loadValues(r.Context(), sessionID)
	if loadErr != nil {
		return session, nil //nolint:nilerr // missing / expired redis row → fresh
	}

	session.ID = sessionID
	session.Values = loaded
	session.IsNew = false
	return session, nil
}

// Save persists the session's Values to Redis and writes a signed cookie
// carrying the session ID. When session.Options.MaxAge is < 0 the session is
// deleted from Redis and the cookie is expired client-side — this is how
// gorilla signals logout.
func (s *Store) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	ctx := r.Context()

	if session.Options != nil && session.Options.MaxAge < 0 {
		if session.ID != "" {
			if err := s.deleteSession(ctx, session.ID); err != nil {
				return fmt.Errorf("deleting session: %w", err)
			}
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
		return nil
	}

	if session.ID == "" {
		id, err := newSessionID()
		if err != nil {
			return fmt.Errorf("generating session id: %w", err)
		}
		session.ID = id
	}

	encoded, err := encodeValues(session.Values)
	if err != nil {
		return fmt.Errorf("encoding session: %w", err)
	}
	if len(encoded) > maxPayloadBytes {
		return ErrSessionTooLarge
	}

	ttl := s.ttl
	if session.Options != nil && session.Options.MaxAge > 0 {
		ttl = time.Duration(session.Options.MaxAge) * time.Second
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, sessionKey(session.ID), encoded, ttl)
	if userID, ok := userIDFromValues(session.Values); ok {
		pipe.SAdd(ctx, userIndexKey(userID), session.ID)
		pipe.Expire(ctx, userIndexKey(userID), ttl+time.Hour)
	}
	if _, pipeErr := pipe.Exec(ctx); pipeErr != nil {
		return fmt.Errorf("persisting session: %w", pipeErr)
	}

	signed, err := securecookie.EncodeMulti(session.Name(), session.ID, s.codecs...)
	if err != nil {
		return fmt.Errorf("signing session cookie: %w", err)
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), signed, session.Options))
	return nil
}

// RevokeSession deletes a session by ID, removing it from the user index too.
// Safe to call for unknown session IDs (returns nil).
func (s *Store) RevokeSession(ctx context.Context, sessionID string) error {
	return s.deleteSession(ctx, sessionID)
}

// RevokeUserSessions deletes every session currently attached to userID and
// returns the count removed. Used when an admin is demoted or a user is
// deleted — sessions that otherwise would keep working until their cookie
// expired are dropped immediately.
func (s *Store) RevokeUserSessions(ctx context.Context, userID int64) (int, error) {
	if userID == 0 {
		return 0, nil
	}
	ids, err := s.client.SMembers(ctx, userIndexKey(userID)).Result()
	if err != nil {
		return 0, fmt.Errorf("listing user sessions: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	pipe := s.client.Pipeline()
	for _, id := range ids {
		pipe.Del(ctx, sessionKey(id))
	}
	pipe.Del(ctx, userIndexKey(userID))
	if _, pipeErr := pipe.Exec(ctx); pipeErr != nil {
		return 0, fmt.Errorf("revoking user sessions: %w", pipeErr)
	}
	return len(ids), nil
}

// ListUserSessions returns the session IDs attached to userID. Empty result
// for userID == 0 or when no sessions are active. Callers combine this with
// Peek/Inspect to render an admin UI — not implemented in this pass.
func (s *Store) ListUserSessions(ctx context.Context, userID int64) ([]string, error) {
	if userID == 0 {
		return nil, nil
	}
	ids, err := s.client.SMembers(ctx, userIndexKey(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("listing user sessions: %w", err)
	}
	return ids, nil
}

// deleteSession removes a session row and detaches it from any user index.
// Unknown ids are a no-op error-free path.
func (s *Store) deleteSession(ctx context.Context, sessionID string) error {
	// Read the user_id from the session payload so we can clean up the index,
	// then drop the main key. A missing row is harmless.
	values, loadErr := s.loadValues(ctx, sessionID)
	pipe := s.client.Pipeline()
	pipe.Del(ctx, sessionKey(sessionID))
	if loadErr == nil {
		if userID, ok := userIDFromValues(values); ok {
			pipe.SRem(ctx, userIndexKey(userID), sessionID)
		}
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

// loadValues fetches the stored payload and gob-decodes it into a
// map[interface{}]interface{} matching session.Values.
func (s *Store) loadValues(ctx context.Context, sessionID string) (map[any]any, error) {
	raw, err := s.client.Get(ctx, sessionKey(sessionID)).Bytes()
	if err != nil {
		return nil, err
	}
	return decodeValues(raw)
}

// newSessionID returns a base32-encoded 32-byte random identifier. Matches
// gorilla's ID length; url-safe characters only.
func newSessionID() (string, error) {
	raw := securecookie.GenerateRandomKey(32)
	if raw == nil {
		return "", errors.New("generating random session id")
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(raw), "="), nil
}

func sessionKey(id string) string { return sessionKeyPrefix + id }

func userIndexKey(id int64) string { return fmt.Sprintf("%s%d", userIndexPrefix, id) }

// encodeValues gob-encodes the session Values map for Redis.
func encodeValues(values map[any]any) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&values); err != nil {
		return nil, fmt.Errorf("gob encoding session values: %w", err)
	}
	return buf.Bytes(), nil
}

// decodeValues gob-decodes a Redis-stored session payload.
func decodeValues(b []byte) (map[any]any, error) {
	var values map[any]any
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&values); err != nil {
		return nil, fmt.Errorf("gob decoding session values: %w", err)
	}
	return values, nil
}

// userIDFromValues extracts the authenticated user's numeric id from the
// session Values map. Returns (0, false) when absent or zero-valued.
func userIDFromValues(values map[any]any) (int64, bool) {
	raw, ok := values[UserIDValueKey]
	if !ok {
		return 0, false
	}
	userID, ok := raw.(int64)
	if !ok || userID == 0 {
		return 0, false
	}
	return userID, true
}
