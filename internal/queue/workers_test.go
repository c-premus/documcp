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

	"git.999.haus/chris/DocuMCP-go/internal/model"
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
		name      string
		attempt   int
		wantDur   time.Duration
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
		name      string
		attempt   int
		wantDur   time.Duration
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

// --- Mock DocumentLister ---

type mockDocumentLister struct {
	docs map[string][]model.Document
	err  error
}

func (m *mockDocumentLister) FindByStatus(_ context.Context, status string, _ int) ([]model.Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.docs[status], nil
}

// --- ReindexAllWorker tests ---

func TestReindexAllWorker_Work(t *testing.T) {
	t.Parallel()

	makeJob := func() *river.Job[ReindexAllArgs] {
		return &river.Job[ReindexAllArgs]{
			JobRow: &rivertype.JobRow{ID: 3},
			Args:   ReindexAllArgs{},
		}
	}

	t.Run("not_configured_missing_lister", func(t *testing.T) {
		t.Parallel()
		worker := &ReindexAllWorker{Indexer: &mockDocumentIndexer{}}
		err := worker.Work(context.Background(), makeJob())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("not_configured_missing_indexer", func(t *testing.T) {
		t.Parallel()
		worker := &ReindexAllWorker{Lister: &mockDocumentLister{}}
		err := worker.Work(context.Background(), makeJob())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("success_no_documents", func(t *testing.T) {
		t.Parallel()
		worker := &ReindexAllWorker{
			Indexer: &mockDocumentIndexer{},
			Lister:  &mockDocumentLister{docs: map[string][]model.Document{}},
		}
		err := worker.Work(context.Background(), makeJob())
		require.NoError(t, err)
	})

	t.Run("success_indexes_all_documents", func(t *testing.T) {
		t.Parallel()
		indexer := &mockDocumentIndexer{}
		lister := &mockDocumentLister{
			docs: map[string][]model.Document{
				"indexed":   {{ID: 1}, {ID: 2}},
				"processed": {{ID: 3}},
			},
		}
		worker := &ReindexAllWorker{Indexer: indexer, Lister: lister}
		err := worker.Work(context.Background(), makeJob())
		require.NoError(t, err)
	})

	t.Run("partial_failure_returns_error", func(t *testing.T) {
		t.Parallel()
		indexer := &mockDocumentIndexer{err: errors.New("search down")}
		lister := &mockDocumentLister{
			docs: map[string][]model.Document{
				"indexed": {{ID: 1}},
			},
		}
		worker := &ReindexAllWorker{Indexer: indexer, Lister: lister}
		err := worker.Work(context.Background(), makeJob())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "1 failures")
	})

	t.Run("canceled_context", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		lister := &mockDocumentLister{
			docs: map[string][]model.Document{
				"indexed": {{ID: 1}},
			},
		}
		worker := &ReindexAllWorker{Indexer: &mockDocumentIndexer{}, Lister: lister}
		err := worker.Work(ctx, makeJob())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "canceled")
	})

	t.Run("lister_error_continues", func(t *testing.T) {
		t.Parallel()
		lister := &mockDocumentLister{err: errors.New("db error")}
		worker := &ReindexAllWorker{Indexer: &mockDocumentIndexer{}, Lister: lister}
		// All statuses fail to list, but worker completes with 0 docs.
		err := worker.Work(context.Background(), makeJob())
		require.NoError(t, err)
	})
}

// --- DocumentExtractWorker nil pipeline test ---

func TestDocumentExtractWorker_Work_nilPipeline(t *testing.T) {
	t.Parallel()

	worker := &DocumentExtractWorker{Pipeline: nil}

	job := &river.Job[DocumentExtractArgs]{
		JobRow: &rivertype.JobRow{ID: 1},
		Args:   DocumentExtractArgs{DocumentID: 1, DocUUID: "uuid"},
	}

	err := worker.Work(context.Background(), job)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pipeline is nil")
}

// --- DocumentExtractWorker success records metrics ---

func TestDocumentExtractWorker_Work_successRecordsMetrics(t *testing.T) {
	t.Parallel()

	mock := &mockDocumentProcessor{}
	metrics := newTestMetrics()
	worker := &DocumentExtractWorker{Pipeline: mock, Metrics: metrics}

	job := &river.Job[DocumentExtractArgs]{
		JobRow: &rivertype.JobRow{ID: 1, Queue: "high", Kind: "document_extract"},
		Args:   DocumentExtractArgs{DocumentID: 10, DocUUID: "test-uuid"},
	}

	err := worker.Work(context.Background(), job)
	require.NoError(t, err)
	assert.Equal(t, int64(10), mock.calledWith)

	// Verify completed counter was incremented.
	counter, metricErr := metrics.QueueJobsCompleted.GetMetricWithLabelValues("high", "document_extract")
	require.NoError(t, metricErr)
	require.NotNil(t, counter)
}

// --- DocumentIndexWorker nil indexer test ---

func TestDocumentIndexWorker_Work_nilIndexer(t *testing.T) {
	t.Parallel()

	worker := &DocumentIndexWorker{Indexer: nil}

	job := &river.Job[DocumentIndexArgs]{
		JobRow: &rivertype.JobRow{ID: 1},
		Args:   DocumentIndexArgs{DocumentID: 1, DocUUID: "uuid"},
	}

	err := worker.Work(context.Background(), job)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "indexer is nil")
}

// --- DocumentIndexWorker success records metrics ---

func TestDocumentIndexWorker_Work_successRecordsMetrics(t *testing.T) {
	t.Parallel()

	mock := &mockDocumentIndexer{}
	metrics := newTestMetrics()
	worker := &DocumentIndexWorker{Indexer: mock, Metrics: metrics}

	job := &river.Job[DocumentIndexArgs]{
		JobRow: &rivertype.JobRow{ID: 2, Queue: "default", Kind: "document_index"},
		Args:   DocumentIndexArgs{DocumentID: 55, DocUUID: "idx-uuid"},
	}

	err := worker.Work(context.Background(), job)
	require.NoError(t, err)
	assert.Equal(t, int64(55), mock.calledWith)

	// Verify completed counter was incremented.
	counter, metricErr := metrics.QueueJobsCompleted.GetMetricWithLabelValues("default", "document_index")
	require.NoError(t, metricErr)
	require.NotNil(t, counter)
}

// --- recordJobCompleted tests ---

func TestRecordJobCompleted(t *testing.T) {
	t.Parallel()

	t.Run("nil metrics does not panic", func(t *testing.T) {
		t.Parallel()
		assert.NotPanics(t, func() {
			recordJobCompleted(nil, "high", "document_extract", 100*time.Millisecond)
		})
	})

	t.Run("non-nil metrics increments counter and observes duration", func(t *testing.T) {
		t.Parallel()
		metrics := newTestMetrics()
		recordJobCompleted(metrics, "high", "document_extract", 150*time.Millisecond)

		// Counter was incremented (no panic means it exists).
		counter, err := metrics.QueueJobsCompleted.GetMetricWithLabelValues("high", "document_extract")
		require.NoError(t, err)
		require.NotNil(t, counter)

		// Duration was observed.
		hist, err := metrics.QueueJobDuration.GetMetricWithLabelValues("high", "document_extract")
		require.NoError(t, err)
		require.NotNil(t, hist)
	})
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
