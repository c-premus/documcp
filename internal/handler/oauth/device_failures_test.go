package oauthhandler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c-premus/documcp/internal/auth/oauth"
	"github.com/c-premus/documcp/internal/model"
)

func newLimiterWithMiniredis(t *testing.T, limit int) *oauth.DeviceFailureLimiter {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return oauth.NewDeviceFailureLimiter(client, limit, time.Hour)
}

// TestDeviceVerificationSubmit_BruteForceBlocking closes security L6: the
// session-scoped counter was trivially defeated by clearing the session
// cookie. With the Redis-backed limiter keyed on user_id, a user who hits the
// threshold stays blocked even with a fresh session.
func TestDeviceVerificationSubmit_BruteForceBlocking(t *testing.T) {
	t.Parallel()

	t.Run("blocks after threshold is reached within the window", func(t *testing.T) {
		t.Parallel()
		limiter := newLimiterWithMiniredis(t, 3)
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return nil, errors.New("not found")
			},
		}
		h, store := newHandlerWithRepoConfigAndLimiter(repo, defaultOAuthConfig(), limiter)
		store.session.Values["user_id"] = int64(42)

		// 3 failed lookups burn the budget.
		for i := range 3 {
			req := httptest.NewRequest(http.MethodPost, "/oauth/device",
				strings.NewReader("user_code=ABCD-EFGH"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			h.DeviceVerificationSubmit(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Contains(t, rr.Body.String(), "Invalid or expired",
				"attempt %d: expected failure render", i+1)
		}

		// 4th attempt is blocked by the limiter.
		req := httptest.NewRequest(http.MethodPost, "/oauth/device",
			strings.NewReader("user_code=ABCD-EFGH"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		h.DeviceVerificationSubmit(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Too many failed attempts")
	})

	t.Run("fresh session does not reset the counter (the bug)", func(t *testing.T) {
		t.Parallel()
		limiter := newLimiterWithMiniredis(t, 3)
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return nil, errors.New("not found")
			},
		}

		// Session A — burn 3 failures for user 42.
		hA, storeA := newHandlerWithRepoConfigAndLimiter(repo, defaultOAuthConfig(), limiter)
		storeA.session.Values["user_id"] = int64(42)
		for range 3 {
			req := httptest.NewRequest(http.MethodPost, "/oauth/device",
				strings.NewReader("user_code=ABCD-EFGH"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			hA.DeviceVerificationSubmit(rr, req)
		}

		// Session B — same user_id, fresh session store, shared limiter.
		hB, storeB := newHandlerWithRepoConfigAndLimiter(repo, defaultOAuthConfig(), limiter)
		storeB.session.Values["user_id"] = int64(42)

		req := httptest.NewRequest(http.MethodPost, "/oauth/device",
			strings.NewReader("user_code=ABCD-EFGH"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		hB.DeviceVerificationSubmit(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Too many failed attempts",
			"same user should stay blocked even with a fresh session cookie")
	})

	t.Run("empty user_code also increments the counter", func(t *testing.T) {
		t.Parallel()
		limiter := newLimiterWithMiniredis(t, 2)
		h, store := newHandlerWithRepoConfigAndLimiter(&mockOAuthRepo{}, defaultOAuthConfig(), limiter)
		store.session.Values["user_id"] = int64(42)

		for range 2 {
			req := httptest.NewRequest(http.MethodPost, "/oauth/device",
				strings.NewReader("user_code="))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			h.DeviceVerificationSubmit(rr, req)
			assert.Contains(t, rr.Body.String(), "Invalid or expired")
		}

		req := httptest.NewRequest(http.MethodPost, "/oauth/device",
			strings.NewReader("user_code="))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		h.DeviceVerificationSubmit(rr, req)
		assert.Contains(t, rr.Body.String(), "Too many failed attempts")
	})

	t.Run("per-user isolation — different users have independent counters", func(t *testing.T) {
		t.Parallel()
		limiter := newLimiterWithMiniredis(t, 3)
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return nil, errors.New("not found")
			},
		}

		// User 42 burns its budget.
		h42, store42 := newHandlerWithRepoConfigAndLimiter(repo, defaultOAuthConfig(), limiter)
		store42.session.Values["user_id"] = int64(42)
		for range 3 {
			req := httptest.NewRequest(http.MethodPost, "/oauth/device",
				strings.NewReader("user_code=ABCD-EFGH"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			h42.DeviceVerificationSubmit(rr, req)
		}

		// User 99 — uses the same shared limiter but hasn't failed yet.
		h99, store99 := newHandlerWithRepoConfigAndLimiter(repo, defaultOAuthConfig(), limiter)
		store99.session.Values["user_id"] = int64(99)

		req := httptest.NewRequest(http.MethodPost, "/oauth/device",
			strings.NewReader("user_code=ABCD-EFGH"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		h99.DeviceVerificationSubmit(rr, req)
		assert.Contains(t, rr.Body.String(), "Invalid or expired",
			"user 99 should get the failure render, not the block")
		assert.NotContains(t, rr.Body.String(), "Too many failed attempts",
			"user 99 should not be blocked by user 42's failures")
	})

	t.Run("successful lookup clears the counter", func(t *testing.T) {
		t.Parallel()
		limiter := newLimiterWithMiniredis(t, 3)

		// Repo returns "not found" twice, then a valid pending code, then "not found" again.
		var calls int
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, userCode string) (*model.OAuthDeviceCode, error) {
				calls++
				if calls == 3 {
					return &model.OAuthDeviceCode{
						ID:        1,
						ClientID:  10,
						UserCode:  userCode,
						Status:    model.DeviceCodeStatusPending,
						ExpiresAt: time.Now().Add(15 * time.Minute),
					}, nil
				}
				return nil, errors.New("not found")
			},
			FindClientByIDFunc: func(_ context.Context, _ int64) (*model.OAuthClient, error) {
				return &model.OAuthClient{ID: 10, ClientName: "MyApp"}, nil
			},
		}
		h, store := newHandlerWithRepoConfigAndLimiter(repo, defaultOAuthConfig(), limiter)
		store.session.Values["user_id"] = int64(42)

		// Two typos, then the real code, then two more typos — should NOT be blocked,
		// because the successful lookup cleared the earlier failures.
		for i, body := range []string{
			"user_code=AAAA-AAAA",
			"user_code=BBBB-BBBB",
			"user_code=ABCD-EFGH", // success
			"user_code=CCCC-CCCC",
			"user_code=DDDD-DDDD",
		} {
			req := httptest.NewRequest(http.MethodPost, "/oauth/device",
				strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			h.DeviceVerificationSubmit(rr, req)
			require.Equal(t, http.StatusOK, rr.Code, "attempt %d", i+1)
			assert.NotContains(t, rr.Body.String(), "Too many failed attempts",
				"attempt %d: should not be blocked — success cleared the counter", i+1)
		}
	})

	t.Run("disabled limiter permits everything", func(t *testing.T) {
		t.Parallel()
		// Explicitly disabled limiter (zero limit).
		disabled := oauth.NewDeviceFailureLimiter(nil, 0, 0)
		repo := &mockOAuthRepo{
			FindDeviceCodeByUserCodeFunc: func(_ context.Context, _ string) (*model.OAuthDeviceCode, error) {
				return nil, errors.New("not found")
			},
		}
		h, store := newHandlerWithRepoConfigAndLimiter(repo, defaultOAuthConfig(), disabled)
		store.session.Values["user_id"] = int64(42)

		// 20 failed attempts — not blocked.
		for i := range 20 {
			req := httptest.NewRequest(http.MethodPost, "/oauth/device",
				strings.NewReader("user_code=ABCD-EFGH"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			h.DeviceVerificationSubmit(rr, req)
			assert.NotContains(t, rr.Body.String(), "Too many failed attempts",
				"disabled limiter should permit attempt %d", i+1)
		}
	})
}
