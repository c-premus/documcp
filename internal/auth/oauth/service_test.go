package oauth

import (
	"log/slog"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/config"
)

// ---------------------------------------------------------------------------
// TestClientTouchTimeout
// ---------------------------------------------------------------------------

func TestClientTouchTimeout(t *testing.T) {
	t.Parallel()

	t.Run("returns configured duration", func(t *testing.T) {
		t.Parallel()

		cfg := config.OAuthConfig{
			ClientTouchTimeout: 30 * time.Second,
		}
		svc, err := NewService(nil, cfg, "https://app.example.com", slog.Default(), testHMACKeys())
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}

		got := svc.ClientTouchTimeout()
		if got != 30*time.Second {
			t.Errorf("ClientTouchTimeout() = %v, want %v", got, 30*time.Second)
		}
	})

	t.Run("returns zero when not configured", func(t *testing.T) {
		t.Parallel()

		cfg := config.OAuthConfig{}
		svc, err := NewService(nil, cfg, "https://app.example.com", slog.Default(), testHMACKeys())
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}

		got := svc.ClientTouchTimeout()
		if got != 0 {
			t.Errorf("ClientTouchTimeout() = %v, want 0", got)
		}
	})

	t.Run("returns large duration", func(t *testing.T) {
		t.Parallel()

		cfg := config.OAuthConfig{
			ClientTouchTimeout: 5 * time.Minute,
		}
		svc, err := NewService(nil, cfg, "https://app.example.com", slog.Default(), testHMACKeys())
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}

		got := svc.ClientTouchTimeout()
		if got != 5*time.Minute {
			t.Errorf("ClientTouchTimeout() = %v, want %v", got, 5*time.Minute)
		}
	})
}

// ---------------------------------------------------------------------------
// clientTouchDebouncer (code-quality M4)
// ---------------------------------------------------------------------------

func TestClientTouchDebouncer(t *testing.T) {
	t.Parallel()

	t.Run("first call passes, repeat within ttl skips", func(t *testing.T) {
		t.Parallel()

		d := newClientTouchDebouncer(time.Minute)
		now := time.Unix(1_700_000_000, 0)

		if !d.shouldTouch(42, now) {
			t.Fatal("first call should fire")
		}
		if d.shouldTouch(42, now.Add(10*time.Second)) {
			t.Error("repeat call within ttl should be debounced")
		}
	})

	t.Run("call after ttl window passes again", func(t *testing.T) {
		t.Parallel()

		d := newClientTouchDebouncer(time.Minute)
		now := time.Unix(1_700_000_000, 0)

		if !d.shouldTouch(42, now) {
			t.Fatal("first call should fire")
		}
		if !d.shouldTouch(42, now.Add(2*time.Minute)) {
			t.Error("call past ttl should fire again")
		}
	})

	t.Run("distinct clients are tracked independently", func(t *testing.T) {
		t.Parallel()

		d := newClientTouchDebouncer(time.Minute)
		now := time.Unix(1_700_000_000, 0)

		if !d.shouldTouch(1, now) {
			t.Error("client 1 first call should fire")
		}
		if !d.shouldTouch(2, now) {
			t.Error("client 2 first call should fire even though client 1 just touched")
		}
		if d.shouldTouch(1, now.Add(time.Second)) {
			t.Error("client 1 repeat should be debounced")
		}
	})

	t.Run("zero ttl disables debouncing", func(t *testing.T) {
		t.Parallel()

		d := newClientTouchDebouncer(0)
		now := time.Unix(1_700_000_000, 0)

		for i := range 3 {
			if !d.shouldTouch(7, now.Add(time.Duration(i)*time.Nanosecond)) {
				t.Errorf("call %d should fire when ttl is zero", i)
			}
		}
	})
}
