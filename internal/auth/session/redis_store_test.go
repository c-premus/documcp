package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/sessions"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStore boots an in-memory redis and returns the Store wired to it.
func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	opts := &sessions.Options{Path: "/", HttpOnly: true, MaxAge: 3600}
	store := New(client, 0, opts,
		[]byte("0123456789abcdef0123456789abcdef"), // 32-byte hash key
		[]byte("fedcba9876543210fedcba9876543210"), // 32-byte block key
	)
	return store, mr
}

func TestStore_SaveAndLoadRoundTrip(t *testing.T) {
	store, _ := newTestStore(t)

	// Initial request — no cookie yet.
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()

	session, err := store.Get(req, "documcp_session")
	require.NoError(t, err)
	assert.True(t, session.IsNew, "fresh session should be new")

	session.Values[UserIDValueKey] = int64(42)
	session.Values[LoginAtValueKey] = int64(1700000000)
	require.NoError(t, store.Save(req, w, session))
	assert.NotEmpty(t, session.ID, "Save should assign an ID")

	// Replay the cookie.
	cookie := w.Result().Cookies()[0]
	req2 := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req2.AddCookie(cookie)

	loaded, err := store.Get(req2, "documcp_session")
	require.NoError(t, err)
	assert.False(t, loaded.IsNew, "loaded session should not be new")
	assert.Equal(t, session.ID, loaded.ID)
	assert.Equal(t, int64(42), loaded.Values[UserIDValueKey])
	assert.Equal(t, int64(1700000000), loaded.Values[LoginAtValueKey])
}

func TestStore_MissingRedisRowTreatedAsFreshSession(t *testing.T) {
	store, mr := newTestStore(t)

	// Save a session and then delete its Redis row from under us.
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	session, err := store.Get(req, "documcp_session")
	require.NoError(t, err)
	session.Values[UserIDValueKey] = int64(1)
	require.NoError(t, store.Save(req, w, session))

	mr.FlushAll()

	// Next request with the cookie must treat the session as fresh.
	cookie := w.Result().Cookies()[0]
	req2 := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req2.AddCookie(cookie)

	loaded, err := store.Get(req2, "documcp_session")
	require.NoError(t, err)
	assert.True(t, loaded.IsNew, "missing redis row should produce a fresh session")
	assert.Empty(t, loaded.Values)
}

func TestStore_SaveWithNegativeMaxAgeDeletes(t *testing.T) {
	store, mr := newTestStore(t)

	// Create a session, then delete it via MaxAge=-1 (gorilla logout idiom).
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	session, _ := store.Get(req, "documcp_session")
	session.Values[UserIDValueKey] = int64(99)
	require.NoError(t, store.Save(req, w, session))

	sessionID := session.ID
	assert.True(t, mr.Exists(sessionKey(sessionID)), "session key must exist after save")
	assert.True(t, mr.Exists(userIndexKey(99)), "user index must exist after save")

	session.Options.MaxAge = -1
	w2 := httptest.NewRecorder()
	require.NoError(t, store.Save(req, w2, session))
	assert.False(t, mr.Exists(sessionKey(sessionID)), "session key must be gone after logout save")
	// Cookie must be expired client-side.
	cookie := w2.Result().Cookies()[0]
	assert.Equal(t, -1, cookie.MaxAge)
}

func TestStore_RevokeUserSessions(t *testing.T) {
	store, _ := newTestStore(t)

	// Simulate three sessions for user 7, one for user 8.
	create := func(userID int64) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		w := httptest.NewRecorder()
		session, _ := store.Get(req, "documcp_session")
		session.Values[UserIDValueKey] = userID
		require.NoError(t, store.Save(req, w, session))
	}
	create(7)
	create(7)
	create(7)
	create(8)

	// User 7's sessions must all exist.
	ids, err := store.ListUserSessions(context.Background(), 7)
	require.NoError(t, err)
	assert.Len(t, ids, 3)

	// Revoke them.
	removed, err := store.RevokeUserSessions(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, 3, removed)

	// The index is gone and the sessions cannot be loaded anymore.
	after, err := store.ListUserSessions(context.Background(), 7)
	require.NoError(t, err)
	assert.Empty(t, after)

	// User 8 is untouched.
	other, err := store.ListUserSessions(context.Background(), 8)
	require.NoError(t, err)
	assert.Len(t, other, 1)
}

func TestStore_RevokeSingleSession(t *testing.T) {
	store, mr := newTestStore(t)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	session, _ := store.Get(req, "documcp_session")
	session.Values[UserIDValueKey] = int64(42)
	require.NoError(t, store.Save(req, w, session))

	require.NoError(t, store.RevokeSession(context.Background(), session.ID))
	assert.False(t, mr.Exists(sessionKey(session.ID)))
	// The user index loses this id.
	members, _ := store.ListUserSessions(context.Background(), 42)
	assert.Empty(t, members)
}

func TestStore_RevokeSessionMissingIsNoOp(t *testing.T) {
	store, _ := newTestStore(t)
	err := store.RevokeSession(context.Background(), "unknown-session-id")
	assert.NoError(t, err)
}

func TestStore_LoadPreservesTTL(t *testing.T) {
	store, mr := newTestStore(t)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	session, _ := store.Get(req, "documcp_session")
	session.Values[UserIDValueKey] = int64(5)
	session.Options.MaxAge = 60 // seconds
	require.NoError(t, store.Save(req, w, session))

	ttl := mr.TTL(sessionKey(session.ID))
	assert.InDelta(t, 60*time.Second, ttl, float64(2*time.Second), "session key should honor MaxAge TTL")
}

func TestStore_CookieContainsOnlyID(t *testing.T) {
	store, _ := newTestStore(t)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	session, _ := store.Get(req, "documcp_session")
	// Any large-ish payload that would have blown past Safari's 4KB cookie
	// limit under the old CookieStore. With the Redis store the cookie only
	// carries the signed session ID.
	session.Values[UserIDValueKey] = int64(1)
	session.Values["payload"] = make([]byte, 8*1024)
	require.NoError(t, store.Save(req, w, session))

	cookie := w.Result().Cookies()[0]
	assert.Less(t, len(cookie.Value), 512, "cookie should hold only a signed session id, not the payload")
}

func TestStore_RevokeUserSessionsZeroUserID(t *testing.T) {
	store, _ := newTestStore(t)
	removed, err := store.RevokeUserSessions(context.Background(), 0)
	assert.NoError(t, err)
	assert.Zero(t, removed)
}
