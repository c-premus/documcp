package queue

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c-premus/documcp/internal/model"
)

// --- Mock implementations for recovery ---

type mockJobInserter struct {
	inserted []insertedJob
	err      error
}

type insertedJob struct {
	args river.JobArgs
	opts *river.InsertOpts
}

func (m *mockJobInserter) Insert(_ context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	m.inserted = append(m.inserted, insertedJob{args: args, opts: opts})
	if m.err != nil {
		return nil, m.err
	}
	return &rivertype.JobInsertResult{
		Job: &rivertype.JobRow{ID: int64(len(m.inserted))},
	}, nil
}

type mockDocumentStatusFinder struct {
	results map[model.DocumentStatus][]StuckDocument
	err     error
}

func (m *mockDocumentStatusFinder) FindByStatus(_ context.Context, status model.DocumentStatus) ([]StuckDocument, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results[status], nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// --- RecoverStuckDocuments tests ---

func TestRecoverStuckDocuments_uploatedDispatchtesExtract(t *testing.T) {
	t.Parallel()

	inserter := &mockJobInserter{}
	finder := &mockDocumentStatusFinder{
		results: map[model.DocumentStatus][]StuckDocument{
			model.DocumentStatusUploaded: {
				{ID: 10, UUID: "uuid-10"},
				{ID: 20, UUID: "uuid-20"},
			},
			model.DocumentStatus("extracted"): {},
		},
	}

	RecoverStuckDocuments(context.Background(), inserter, finder, discardLogger())

	require.Len(t, inserter.inserted, 2)

	// Verify first inserted job is DocumentExtractArgs.
	args0, ok := inserter.inserted[0].args.(DocumentExtractArgs)
	require.True(t, ok, "expected DocumentExtractArgs")
	assert.Equal(t, int64(10), args0.DocumentID)
	assert.Equal(t, "uuid-10", args0.DocUUID)

	args1, ok := inserter.inserted[1].args.(DocumentExtractArgs)
	require.True(t, ok, "expected DocumentExtractArgs")
	assert.Equal(t, int64(20), args1.DocumentID)
	assert.Equal(t, "uuid-20", args1.DocUUID)
}

func TestRecoverStuckDocuments_extractedDispatchesExtract(t *testing.T) {
	t.Parallel()

	inserter := &mockJobInserter{}
	finder := &mockDocumentStatusFinder{
		results: map[model.DocumentStatus][]StuckDocument{
			model.DocumentStatusUploaded: {},
			model.DocumentStatus("extracted"): {
				{ID: 30, UUID: "uuid-30"},
			},
		},
	}

	RecoverStuckDocuments(context.Background(), inserter, finder, discardLogger())

	require.Len(t, inserter.inserted, 1)

	args, ok := inserter.inserted[0].args.(DocumentExtractArgs)
	require.True(t, ok, "expected DocumentExtractArgs")
	assert.Equal(t, int64(30), args.DocumentID)
	assert.Equal(t, "uuid-30", args.DocUUID)
}

func TestRecoverStuckDocuments_bothStatuses(t *testing.T) {
	t.Parallel()

	inserter := &mockJobInserter{}
	finder := &mockDocumentStatusFinder{
		results: map[model.DocumentStatus][]StuckDocument{
			model.DocumentStatusUploaded:  {{ID: 1, UUID: "u1"}},
			model.DocumentStatus("extracted"): {{ID: 2, UUID: "u2"}},
		},
	}

	RecoverStuckDocuments(context.Background(), inserter, finder, discardLogger())

	require.Len(t, inserter.inserted, 2)

	// First should be extract (from uploaded).
	_, isExtract := inserter.inserted[0].args.(DocumentExtractArgs)
	assert.True(t, isExtract, "first job should be DocumentExtractArgs")

	// Second should be extract (from extracted).
	_, isExtract2 := inserter.inserted[1].args.(DocumentExtractArgs)
	assert.True(t, isExtract2, "second job should be DocumentExtractArgs")
}

func TestRecoverStuckDocuments_nilInserter(t *testing.T) {
	t.Parallel()

	finder := &mockDocumentStatusFinder{
		results: map[model.DocumentStatus][]StuckDocument{
			model.DocumentStatusUploaded: {{ID: 1, UUID: "u1"}},
		},
	}

	// Should return immediately without panicking.
	assert.NotPanics(t, func() {
		RecoverStuckDocuments(context.Background(), nil, finder, discardLogger())
	})
}

func TestRecoverStuckDocuments_nilFinder(t *testing.T) {
	t.Parallel()

	inserter := &mockJobInserter{}

	// Should return immediately without panicking.
	assert.NotPanics(t, func() {
		RecoverStuckDocuments(context.Background(), inserter, nil, discardLogger())
	})

	assert.Empty(t, inserter.inserted)
}

func TestRecoverStuckDocuments_bothNil(t *testing.T) {
	t.Parallel()

	assert.NotPanics(t, func() {
		RecoverStuckDocuments(context.Background(), nil, nil, discardLogger())
	})
}

func TestRecoverStuckDocuments_finderError(t *testing.T) {
	t.Parallel()

	inserter := &mockJobInserter{}
	finder := &mockDocumentStatusFinder{
		err: errors.New("database connection failed"),
	}

	// Should not panic; errors are logged, not returned.
	assert.NotPanics(t, func() {
		RecoverStuckDocuments(context.Background(), inserter, finder, discardLogger())
	})

	assert.Empty(t, inserter.inserted, "no jobs should be inserted when finder errors")
}

func TestRecoverStuckDocuments_insertError(t *testing.T) {
	t.Parallel()

	inserter := &mockJobInserter{err: errors.New("insert failed")}
	finder := &mockDocumentStatusFinder{
		results: map[model.DocumentStatus][]StuckDocument{
			model.DocumentStatusUploaded:  {{ID: 1, UUID: "u1"}},
			model.DocumentStatus("extracted"): {{ID: 2, UUID: "u2"}},
		},
	}

	// Should not panic; insert errors are logged individually.
	assert.NotPanics(t, func() {
		RecoverStuckDocuments(context.Background(), inserter, finder, discardLogger())
	})

	// Insert was attempted for both, even though it failed.
	assert.Len(t, inserter.inserted, 2)
}

func TestRecoverStuckDocuments_noStuckDocuments(t *testing.T) {
	t.Parallel()

	inserter := &mockJobInserter{}
	finder := &mockDocumentStatusFinder{
		results: map[model.DocumentStatus][]StuckDocument{
			model.DocumentStatusUploaded:  {},
			model.DocumentStatus("extracted"): {},
		},
	}

	RecoverStuckDocuments(context.Background(), inserter, finder, discardLogger())

	assert.Empty(t, inserter.inserted)
}

func TestRecoverStuckDocuments_nilOptsPassedToInsert(t *testing.T) {
	t.Parallel()

	inserter := &mockJobInserter{}
	finder := &mockDocumentStatusFinder{
		results: map[model.DocumentStatus][]StuckDocument{
			model.DocumentStatusUploaded:  {{ID: 5, UUID: "u5"}},
			model.DocumentStatus("extracted"): {{ID: 6, UUID: "u6"}},
		},
	}

	RecoverStuckDocuments(context.Background(), inserter, finder, discardLogger())

	for _, ij := range inserter.inserted {
		assert.Nil(t, ij.opts, "InsertOpts should be nil (uses job defaults)")
	}
}
