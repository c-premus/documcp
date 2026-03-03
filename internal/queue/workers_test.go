package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock implementations ---

type mockDocumentProcessor struct {
	calledWith int64
	err        error
}

func (m *mockDocumentProcessor) ProcessDocument(_ context.Context, docID int64) error {
	m.calledWith = docID
	return m.err
}

type mockDocumentIndexer struct {
	calledWith int64
	err        error
}

func (m *mockDocumentIndexer) IndexDocumentByID(_ context.Context, docID int64) error {
	m.calledWith = docID
	return m.err
}

// --- DocumentExtractWorker tests ---

func TestDocumentExtractWorker_Work(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		docID      int64
		processErr error
		wantErr    bool
	}{
		{
			name:  "success",
			docID: 42,
		},
		{
			name:       "processor_error",
			docID:      99,
			processErr: errors.New("extraction failed"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockDocumentProcessor{err: tt.processErr}
			worker := &DocumentExtractWorker{Pipeline: mock}

			job := &river.Job[DocumentExtractArgs]{
				JobRow: &rivertype.JobRow{ID: 1},
				Args:   DocumentExtractArgs{DocumentID: tt.docID, DocUUID: "test-uuid"},
			}

			err := worker.Work(context.Background(), job)

			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.processErr, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.docID, mock.calledWith, "ProcessDocument should receive the correct document ID")
		})
	}
}

func TestDocumentExtractWorker_NextRetry(t *testing.T) {
	t.Parallel()

	worker := &DocumentExtractWorker{}

	tests := []struct {
		name     string
		attempt  int
		wantDur  time.Duration
		tolerance time.Duration
	}{
		{"attempt_1_60s", 1, 60 * time.Second, 2 * time.Second},
		{"attempt_2_120s", 2, 120 * time.Second, 2 * time.Second},
		{"attempt_3_300s", 3, 300 * time.Second, 2 * time.Second},
		{"attempt_4_capped_at_300s", 4, 300 * time.Second, 2 * time.Second},
		{"attempt_100_capped_at_300s", 100, 300 * time.Second, 2 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			job := &river.Job[DocumentExtractArgs]{
				JobRow: &rivertype.JobRow{Attempt: tt.attempt},
				Args:   DocumentExtractArgs{},
			}

			before := time.Now().Add(tt.wantDur)
			result := worker.NextRetry(job)
			after := time.Now().Add(tt.wantDur)

			assert.True(t, result.After(before.Add(-tt.tolerance)),
				"retry time %v should be after %v (with tolerance)", result, before.Add(-tt.tolerance))
			assert.True(t, result.Before(after.Add(tt.tolerance)),
				"retry time %v should be before %v (with tolerance)", result, after.Add(tt.tolerance))
		})
	}
}

// --- DocumentIndexWorker tests ---

func TestDocumentIndexWorker_Work(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		docID    int64
		indexErr error
		wantErr  bool
	}{
		{
			name:  "success",
			docID: 55,
		},
		{
			name:     "indexer_error",
			docID:    77,
			indexErr: errors.New("indexing failed"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockDocumentIndexer{err: tt.indexErr}
			worker := &DocumentIndexWorker{Indexer: mock}

			job := &river.Job[DocumentIndexArgs]{
				JobRow: &rivertype.JobRow{ID: 2},
				Args:   DocumentIndexArgs{DocumentID: tt.docID, DocUUID: "idx-uuid"},
			}

			err := worker.Work(context.Background(), job)

			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.indexErr, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.docID, mock.calledWith, "IndexDocumentByID should receive the correct document ID")
		})
	}
}

func TestDocumentIndexWorker_NextRetry(t *testing.T) {
	t.Parallel()

	worker := &DocumentIndexWorker{}

	tests := []struct {
		name     string
		attempt  int
		wantDur  time.Duration
		tolerance time.Duration
	}{
		{"attempt_1_60s", 1, 60 * time.Second, 2 * time.Second},
		{"attempt_2_120s", 2, 120 * time.Second, 2 * time.Second},
		{"attempt_3_300s", 3, 300 * time.Second, 2 * time.Second},
		{"attempt_beyond_backoffs_capped", 10, 300 * time.Second, 2 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			job := &river.Job[DocumentIndexArgs]{
				JobRow: &rivertype.JobRow{Attempt: tt.attempt},
				Args:   DocumentIndexArgs{},
			}

			before := time.Now().Add(tt.wantDur)
			result := worker.NextRetry(job)
			after := time.Now().Add(tt.wantDur)

			assert.True(t, result.After(before.Add(-tt.tolerance)),
				"retry time should be approximately now + %v", tt.wantDur)
			assert.True(t, result.Before(after.Add(tt.tolerance)),
				"retry time should be approximately now + %v", tt.wantDur)
		})
	}
}

// --- ReindexAllWorker tests ---

func TestReindexAllWorker_Work(t *testing.T) {
	t.Parallel()

	mock := &mockDocumentIndexer{}
	worker := &ReindexAllWorker{Indexer: mock}

	job := &river.Job[ReindexAllArgs]{
		JobRow: &rivertype.JobRow{ID: 3},
		Args:   ReindexAllArgs{},
	}

	err := worker.Work(context.Background(), job)
	require.NoError(t, err, "ReindexAllWorker.Work should return nil (placeholder)")
}

// --- nextRetryFromBackoffs tests ---

func TestNextRetryFromBackoffs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		attempt int
		wantDur time.Duration
	}{
		{"attempt_0_clamps_to_first", 0, 60 * time.Second},
		{"attempt_1_first_backoff", 1, 60 * time.Second},
		{"attempt_2_second_backoff", 2, 120 * time.Second},
		{"attempt_3_third_backoff", 3, 300 * time.Second},
		{"attempt_4_clamped_to_last", 4, 300 * time.Second},
		{"attempt_negative_clamps_to_first", -1, 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			before := time.Now()
			result := nextRetryFromBackoffs(tt.attempt)
			after := time.Now()

			expectedLow := before.Add(tt.wantDur)
			expectedHigh := after.Add(tt.wantDur)

			assert.False(t, result.Before(expectedLow.Add(-time.Second)),
				"result %v too early, expected around %v", result, expectedLow)
			assert.False(t, result.After(expectedHigh.Add(time.Second)),
				"result %v too late, expected around %v", result, expectedHigh)
		})
	}
}
