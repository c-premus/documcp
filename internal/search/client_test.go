package search_test

import (
	"bytes"
	"log/slog"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/search"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	t.Run("returns non-nil client with host and key", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
		c := search.NewClient("http://localhost:7700", "master-key", logger)

		if c == nil {
			t.Fatal("NewClient returned nil")
		}
	})

	t.Run("returns non-nil client with empty key", func(t *testing.T) {
		t.Parallel()

		logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
		c := search.NewClient("http://localhost:7700", "", logger)

		if c == nil {
			t.Fatal("NewClient returned nil with empty key")
		}
	})

	t.Run("returns non-nil client with nil logger", func(t *testing.T) {
		t.Parallel()

		c := search.NewClient("http://localhost:7700", "key", nil)

		if c == nil {
			t.Fatal("NewClient returned nil with nil logger")
		}
	})
}

func TestClient_ServiceManager(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	c := search.NewClient("http://localhost:7700", "key", logger)

	sm := c.ServiceManager()
	if sm == nil {
		t.Fatal("ServiceManager() returned nil")
	}
}

func TestClient_Healthy_UnreachableHost(t *testing.T) {
	t.Parallel()

	// A client pointing at a non-existent host should report unhealthy.
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	c := search.NewClient("http://127.0.0.1:1", "key", logger)

	if c.Healthy() {
		t.Error("Healthy() = true for unreachable host, want false")
	}
}
