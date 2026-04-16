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
		svc := NewService(nil, cfg, "https://app.example.com", slog.Default(), nil)

		got := svc.ClientTouchTimeout()
		if got != 30*time.Second {
			t.Errorf("ClientTouchTimeout() = %v, want %v", got, 30*time.Second)
		}
	})

	t.Run("returns zero when not configured", func(t *testing.T) {
		t.Parallel()

		cfg := config.OAuthConfig{}
		svc := NewService(nil, cfg, "https://app.example.com", slog.Default(), nil)

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
		svc := NewService(nil, cfg, "https://app.example.com", slog.Default(), nil)

		got := svc.ClientTouchTimeout()
		if got != 5*time.Minute {
			t.Errorf("ClientTouchTimeout() = %v, want %v", got, 5*time.Minute)
		}
	})
}
