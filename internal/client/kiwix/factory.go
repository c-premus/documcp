package kiwix

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/c-premus/documcp/internal/model"
)

// ServiceFinder retrieves enabled external services by type.
type ServiceFinder interface {
	FindEnabledByType(ctx context.Context, serviceType string) ([]model.ExternalService, error)
}

// ClientFactory creates Kiwix clients on demand from the database.
// It caches the client until Invalidate() is called.
type ClientFactory struct {
	mu     sync.Mutex
	client *Client

	repo   ServiceFinder
	config ClientConfig // template config (timeouts, cache TTL — BaseURL filled from DB)
	logger *slog.Logger
}

// NewClientFactory creates a factory that lazily initializes a Kiwix client
// from the first enabled kiwix external service in the database.
func NewClientFactory(repo ServiceFinder, config ClientConfig, logger *slog.Logger) *ClientFactory {
	return &ClientFactory{
		repo:   repo,
		config: config,
		logger: logger,
	}
}

// Get returns a cached Kiwix client, creating one on first call by querying
// the database for an enabled kiwix external service.
func (f *ClientFactory) Get(ctx context.Context) (*Client, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.client != nil {
		return f.client, nil
	}

	services, err := f.repo.FindEnabledByType(ctx, "kiwix")
	if err != nil {
		return nil, fmt.Errorf("looking up kiwix services: %w", err)
	}
	if len(services) == 0 {
		return nil, errors.New("no kiwix service configured")
	}

	cfg := f.config
	cfg.BaseURL = services[0].BaseURL

	client, err := NewClient(cfg, f.logger)
	if err != nil {
		return nil, fmt.Errorf("creating kiwix client: %w", err)
	}

	f.client = client
	f.logger.Info("Kiwix client initialized from database", "base_url", services[0].BaseURL)
	return client, nil
}

// Invalidate clears the cached client, forcing re-initialization on next Get().
func (f *ClientFactory) Invalidate() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.client = nil
}
