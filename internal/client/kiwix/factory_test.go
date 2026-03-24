package kiwix

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// mockServiceFinder is a test double for the ServiceFinder interface.
type mockServiceFinder struct {
	services []model.ExternalService
	err      error
	calls    atomic.Int32
}

func (m *mockServiceFinder) FindEnabledByType(_ context.Context, _ string) ([]model.ExternalService, error) {
	m.calls.Add(1)
	return m.services, m.err
}

func TestNewClientFactory(t *testing.T) {
	finder := &mockServiceFinder{}
	cfg := ClientConfig{
		HTTPTimeout:        10 * time.Second,
		HealthCheckTimeout: 5 * time.Second,
		CacheTTL:           1 * time.Hour,
		SSRFDialerTimeout:  5 * time.Second,
	}
	logger := slog.Default()

	factory := NewClientFactory(finder, cfg, logger)

	if factory == nil {
		t.Fatal("expected non-nil factory")
	}
	if factory.repo != finder {
		t.Error("factory.repo not set correctly")
	}
	if factory.logger != logger {
		t.Error("factory.logger not set correctly")
	}
	if factory.client != nil {
		t.Error("factory.client should be nil initially")
	}
}

func TestClientFactory_Get_Success(t *testing.T) {
	finder := &mockServiceFinder{
		services: []model.ExternalService{
			{BaseURL: "http://example.com:8080"},
		},
	}
	cfg := ClientConfig{
		HTTPTimeout:        10 * time.Second,
		HealthCheckTimeout: 5 * time.Second,
		CacheTTL:           1 * time.Hour,
		SSRFDialerTimeout:  5 * time.Second,
	}

	factory := NewClientFactory(finder, cfg, slog.Default())

	client, err := factory.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.baseURL != "http://example.com:8080" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "http://example.com:8080")
	}
}

func TestClientFactory_Get_Cached(t *testing.T) {
	finder := &mockServiceFinder{
		services: []model.ExternalService{
			{BaseURL: "http://example.com:8080"},
		},
	}
	cfg := ClientConfig{
		HTTPTimeout:        10 * time.Second,
		HealthCheckTimeout: 5 * time.Second,
		CacheTTL:           1 * time.Hour,
		SSRFDialerTimeout:  5 * time.Second,
	}

	factory := NewClientFactory(finder, cfg, slog.Default())

	client1, err := factory.Get(context.Background())
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}

	client2, err := factory.Get(context.Background())
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}

	if client1 != client2 {
		t.Error("expected same client instance on second call")
	}
	if finder.calls.Load() != 1 {
		t.Errorf("expected finder called once, got %d", finder.calls.Load())
	}
}

func TestClientFactory_Get_NoService(t *testing.T) {
	finder := &mockServiceFinder{
		services: []model.ExternalService{},
	}
	cfg := ClientConfig{
		HTTPTimeout:        10 * time.Second,
		HealthCheckTimeout: 5 * time.Second,
		CacheTTL:           1 * time.Hour,
		SSRFDialerTimeout:  5 * time.Second,
	}

	factory := NewClientFactory(finder, cfg, slog.Default())

	_, err := factory.Get(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "no kiwix service configured" {
		t.Errorf("error = %q, want %q", err.Error(), "no kiwix service configured")
	}
}

func TestClientFactory_Get_FinderError(t *testing.T) {
	finder := &mockServiceFinder{
		err: errors.New("database connection failed"),
	}
	cfg := ClientConfig{
		HTTPTimeout:        10 * time.Second,
		HealthCheckTimeout: 5 * time.Second,
		CacheTTL:           1 * time.Hour,
		SSRFDialerTimeout:  5 * time.Second,
	}

	factory := NewClientFactory(finder, cfg, slog.Default())

	_, err := factory.Get(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, finder.err) {
		t.Errorf("error should wrap original: %v", err)
	}
}

func TestClientFactory_Invalidate(t *testing.T) {
	finder := &mockServiceFinder{
		services: []model.ExternalService{
			{BaseURL: "http://example.com:8080"},
		},
	}
	cfg := ClientConfig{
		HTTPTimeout:        10 * time.Second,
		HealthCheckTimeout: 5 * time.Second,
		CacheTTL:           1 * time.Hour,
		SSRFDialerTimeout:  5 * time.Second,
	}

	factory := NewClientFactory(finder, cfg, slog.Default())

	// First Get populates cache.
	_, err := factory.Get(context.Background())
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if finder.calls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", finder.calls.Load())
	}

	// Invalidate clears cache.
	factory.Invalidate()

	// Next Get should call finder again.
	_, err = factory.Get(context.Background())
	if err != nil {
		t.Fatalf("second Get after Invalidate: %v", err)
	}
	if finder.calls.Load() != 2 {
		t.Errorf("expected 2 finder calls after invalidate, got %d", finder.calls.Load())
	}
}

func TestClientFactory_Get_Concurrent(t *testing.T) {
	finder := &mockServiceFinder{
		services: []model.ExternalService{
			{BaseURL: "http://example.com:8080"},
		},
	}
	cfg := ClientConfig{
		HTTPTimeout:        10 * time.Second,
		HealthCheckTimeout: 5 * time.Second,
		CacheTTL:           1 * time.Hour,
		SSRFDialerTimeout:  5 * time.Second,
	}

	factory := NewClientFactory(finder, cfg, slog.Default())

	const goroutines = 10
	errs := make(chan error, goroutines)
	clients := make(chan *Client, goroutines)

	for range goroutines {
		go func() {
			c, err := factory.Get(context.Background())
			errs <- err
			clients <- c
		}()
	}

	var first *Client
	for range goroutines {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent Get returned error: %v", err)
		}
		c := <-clients
		if first == nil {
			first = c
		} else if c != first {
			t.Error("concurrent Get returned different client instances")
		}
	}
}

func TestClientFactory_Get_InvalidBaseURL(t *testing.T) {
	finder := &mockServiceFinder{
		services: []model.ExternalService{
			{BaseURL: ""},
		},
	}
	cfg := ClientConfig{
		HTTPTimeout:        10 * time.Second,
		HealthCheckTimeout: 5 * time.Second,
		CacheTTL:           1 * time.Hour,
		SSRFDialerTimeout:  5 * time.Second,
	}

	factory := NewClientFactory(finder, cfg, slog.Default())

	_, err := factory.Get(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid base URL, got nil")
	}
	if !strings.Contains(err.Error(), "creating kiwix client") {
		t.Errorf("error = %q, want it to contain 'creating kiwix client'", err)
	}
}
