package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"

	"golang.org/x/crypto/hkdf"

	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/queue"
	"github.com/c-premus/documcp/internal/repository"
)

// newLogger creates a structured logger appropriate for the environment.
func newLogger(env string, debug bool, w io.Writer) *slog.Logger {
	var level slog.Level
	if debug {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	var handler slog.Handler
	if env == "production" || env == "staging" {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	}

	return slog.New(handler)
}

// deriveKey uses HKDF-SHA256 to derive a subkey from a master secret.
// This ensures different keys for different purposes (e.g. CSRF vs sessions).
func deriveKey(secret []byte, salt, info string, length int) ([]byte, error) {
	r := hkdf.New(sha256.New, secret, []byte(salt), []byte(info))
	key := make([]byte, length)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("deriving key for %s: %w", info, err)
	}
	return key, nil
}

// docStatusAdapter adapts DocumentRepository.FindByStatus to queue.DocumentStatusFinder.
type docStatusAdapter struct {
	repo interface {
		FindByStatus(ctx context.Context, status string, limit int) ([]model.Document, error)
	}
}

// FindByStatus returns jobs whose associated documents have the given processing status.
func (a *docStatusAdapter) FindByStatus(ctx context.Context, status string) ([]queue.StuckDocument, error) {
	docs, err := a.repo.FindByStatus(ctx, status, 1000)
	if err != nil {
		return nil, err
	}
	result := make([]queue.StuckDocument, len(docs))
	for i := range docs {
		result[i] = queue.StuckDocument{ID: docs[i].ID, UUID: docs[i].UUID}
	}
	return result, nil
}

// newDocStatusAdapter creates a docStatusAdapter for the given repository.
func newDocStatusAdapter(repo *repository.DocumentRepository) *docStatusAdapter {
	return &docStatusAdapter{repo: repo}
}
