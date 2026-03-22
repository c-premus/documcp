package search

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

// ---------------------------------------------------------------------------
// Client tests
// ---------------------------------------------------------------------------

func TestConfigureIndexes_Success(t *testing.T) {
	t.Parallel()

	var createdUIDs []string
	var settingsUIDs []string

	idx := &mockIndexManager{
		updateSettingsWithContextFn: func(_ context.Context, _ *meilisearch.Settings) (*meilisearch.TaskInfo, error) {
			return &meilisearch.TaskInfo{TaskUID: 10}, nil
		},
	}
	sm := &mockServiceManager{
		createIndexWithContextFn: func(_ context.Context, config *meilisearch.IndexConfig) (*meilisearch.TaskInfo, error) {
			createdUIDs = append(createdUIDs, config.Uid)
			return &meilisearch.TaskInfo{TaskUID: 1}, nil
		},
		indexFn: func(uid string) meilisearch.IndexManager {
			settingsUIDs = append(settingsUIDs, uid)
			return idx
		},
	}

	c := newTestClient(sm, testLogger())
	err := c.ConfigureIndexes(context.Background())
	if err != nil {
		t.Fatalf("ConfigureIndexes() unexpected error: %v", err)
	}

	wantUIDs := []string{IndexDocuments, IndexZimArchives, IndexGitTemplates}
	if len(createdUIDs) != len(wantUIDs) {
		t.Fatalf("created %d indexes, want %d", len(createdUIDs), len(wantUIDs))
	}
	for i, uid := range wantUIDs {
		if createdUIDs[i] != uid {
			t.Errorf("created index %d = %q, want %q", i, createdUIDs[i], uid)
		}
	}
}

func TestConfigureIndexes_CreateIndexError_ContinuesWithSettings(t *testing.T) {
	t.Parallel()

	settingsCalled := 0
	idx := &mockIndexManager{
		updateSettingsWithContextFn: func(_ context.Context, _ *meilisearch.Settings) (*meilisearch.TaskInfo, error) {
			settingsCalled++
			return &meilisearch.TaskInfo{TaskUID: 10}, nil
		},
	}
	sm := &mockServiceManager{
		createIndexWithContextFn: func(_ context.Context, _ *meilisearch.IndexConfig) (*meilisearch.TaskInfo, error) {
			return nil, errors.New("index already exists")
		},
		indexFn: func(_ string) meilisearch.IndexManager {
			return idx
		},
	}

	c := newTestClient(sm, testLogger())
	err := c.ConfigureIndexes(context.Background())
	if err != nil {
		t.Fatalf("ConfigureIndexes() unexpected error: %v", err)
	}

	if settingsCalled != 3 {
		t.Errorf("UpdateSettings called %d times, want 3", settingsCalled)
	}
}

func TestConfigureIndexes_UpdateSettingsError_ReturnsError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("settings update failed")
	idx := &mockIndexManager{
		updateSettingsWithContextFn: func(_ context.Context, _ *meilisearch.Settings) (*meilisearch.TaskInfo, error) {
			return nil, wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager {
			return idx
		},
	}

	c := newTestClient(sm, testLogger())
	err := c.ConfigureIndexes(context.Background())
	if err == nil {
		t.Fatal("ConfigureIndexes() expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestConfigureIndexes_SettingsErrorMessageContainsIndexUID(t *testing.T) {
	t.Parallel()

	idx := &mockIndexManager{
		updateSettingsWithContextFn: func(_ context.Context, _ *meilisearch.Settings) (*meilisearch.TaskInfo, error) {
			return nil, errors.New("boom")
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager {
			return idx
		},
	}

	c := newTestClient(sm, testLogger())
	err := c.ConfigureIndexes(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got == "" {
		t.Error("error message is empty")
	}
}

func TestConfigureIndexes_VerifiesPrimaryKeys(t *testing.T) {
	t.Parallel()

	var createdConfigs []*meilisearch.IndexConfig
	idx := &mockIndexManager{}
	sm := &mockServiceManager{
		createIndexWithContextFn: func(_ context.Context, config *meilisearch.IndexConfig) (*meilisearch.TaskInfo, error) {
			createdConfigs = append(createdConfigs, config)
			return &meilisearch.TaskInfo{TaskUID: 1}, nil
		},
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	c := newTestClient(sm, testLogger())
	err := c.ConfigureIndexes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, cfg := range createdConfigs {
		if cfg.PrimaryKey != "uuid" {
			t.Errorf("index %q primary key = %q, want %q", cfg.Uid, cfg.PrimaryKey, "uuid")
		}
	}
}

func TestHealthy_WithMock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		healthy bool
	}{
		{"healthy returns true", true},
		{"unhealthy returns false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sm := &mockServiceManager{
				isHealthyFn: func() bool { return tt.healthy },
			}
			c := newTestClient(sm, testLogger())
			if got := c.Healthy(); got != tt.healthy {
				t.Errorf("Healthy() = %v, want %v", got, tt.healthy)
			}
		})
	}
}

func TestServiceManager_WithMock(t *testing.T) {
	t.Parallel()

	sm := &mockServiceManager{}
	c := newTestClient(sm, testLogger())
	if c.ServiceManager() == nil {
		t.Error("ServiceManager() returned nil")
	}
}

// ---------------------------------------------------------------------------
// Indexer tests
// ---------------------------------------------------------------------------

func TestIndexDocument_Success(t *testing.T) {
	t.Parallel()

	var capturedIndex string
	idx := &mockIndexManager{
		addDocumentsWithContextFn: func(_ context.Context, _ any, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			return &meilisearch.TaskInfo{TaskUID: 42}, nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(uid string) meilisearch.IndexManager {
			capturedIndex = uid
			return idx
		},
	}

	ix := newTestIndexer(sm)
	err := ix.IndexDocument(context.Background(), DocumentRecord{UUID: "doc-1", Title: "Test"})
	if err != nil {
		t.Fatalf("IndexDocument() unexpected error: %v", err)
	}
	if capturedIndex != IndexDocuments {
		t.Errorf("used index %q, want %q", capturedIndex, IndexDocuments)
	}
}

func TestIndexDocument_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("connection refused")
	idx := &mockIndexManager{
		addDocumentsWithContextFn: func(_ context.Context, _ any, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			return nil, wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.IndexDocument(context.Background(), DocumentRecord{UUID: "doc-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestDeleteDocument_Success(t *testing.T) {
	t.Parallel()

	var capturedID string
	idx := &mockIndexManager{
		deleteDocumentWithContextFn: func(_ context.Context, id string, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			capturedID = id
			return &meilisearch.TaskInfo{TaskUID: 5}, nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.DeleteDocument(context.Background(), "uuid-abc")
	if err != nil {
		t.Fatalf("DeleteDocument() unexpected error: %v", err)
	}
	if capturedID != "uuid-abc" {
		t.Errorf("deleted id = %q, want %q", capturedID, "uuid-abc")
	}
}

func TestDeleteDocument_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("not found")
	idx := &mockIndexManager{
		deleteDocumentWithContextFn: func(_ context.Context, _ string, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			return nil, wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.DeleteDocument(context.Background(), "uuid-abc")
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestSoftDeleteDocument_Success(t *testing.T) {
	t.Parallel()

	var capturedDocs any
	idx := &mockIndexManager{
		addDocumentsWithContextFn: func(_ context.Context, docs any, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			capturedDocs = docs
			return &meilisearch.TaskInfo{TaskUID: 7}, nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.SoftDeleteDocument(context.Background(), "uuid-soft")
	if err != nil {
		t.Fatalf("SoftDeleteDocument() unexpected error: %v", err)
	}

	records, ok := capturedDocs.([]map[string]any)
	if !ok || len(records) != 1 {
		t.Fatalf("expected 1 record, got %T", capturedDocs)
	}
	if records[0]["__soft_deleted"] != true {
		t.Error("__soft_deleted should be true")
	}
	if records[0]["uuid"] != "uuid-soft" {
		t.Errorf("uuid = %v, want %q", records[0]["uuid"], "uuid-soft")
	}
}

func TestSoftDeleteDocument_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fail")
	idx := &mockIndexManager{
		addDocumentsWithContextFn: func(_ context.Context, _ any, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			return nil, wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.SoftDeleteDocument(context.Background(), "uuid-soft")
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestIndexBatch_EmptySlice_ReturnsNil(t *testing.T) {
	t.Parallel()

	addCalled := false
	idx := &mockIndexManager{
		addDocumentsWithContextFn: func(_ context.Context, _ any, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			addCalled = true
			return &meilisearch.TaskInfo{}, nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.IndexBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("IndexBatch(nil) unexpected error: %v", err)
	}
	if addCalled {
		t.Error("AddDocuments should not be called for empty batch")
	}
}

func TestIndexBatch_Success(t *testing.T) {
	t.Parallel()

	idx := &mockIndexManager{}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	docs := []DocumentRecord{
		{UUID: "a", Title: "A"},
		{UUID: "b", Title: "B"},
	}
	err := ix.IndexBatch(context.Background(), docs)
	if err != nil {
		t.Fatalf("IndexBatch() unexpected error: %v", err)
	}
}

func TestIndexBatch_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("batch fail")
	idx := &mockIndexManager{
		addDocumentsWithContextFn: func(_ context.Context, _ any, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			return nil, wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.IndexBatch(context.Background(), []DocumentRecord{{UUID: "x"}})
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestWaitForTask_Success(t *testing.T) {
	t.Parallel()

	var capturedUID int64
	sm := &mockServiceManager{
		waitForTaskWithContextFn: func(_ context.Context, uid int64, _ time.Duration) (*meilisearch.Task, error) {
			capturedUID = uid
			return &meilisearch.Task{}, nil
		},
	}

	ix := newTestIndexer(sm)
	err := ix.WaitForTask(context.Background(), 123)
	if err != nil {
		t.Fatalf("WaitForTask() unexpected error: %v", err)
	}
	if capturedUID != 123 {
		t.Errorf("waited for task %d, want 123", capturedUID)
	}
}

func TestWaitForTask_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("timeout")
	sm := &mockServiceManager{
		waitForTaskWithContextFn: func(_ context.Context, _ int64, _ time.Duration) (*meilisearch.Task, error) {
			return nil, wantErr
		},
	}

	ix := newTestIndexer(sm)
	err := ix.WaitForTask(context.Background(), 99)
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestListIndexedDocumentUUIDs_SinglePage(t *testing.T) {
	t.Parallel()

	idx := &mockIndexManager{
		getDocumentsWithContextFn: func(_ context.Context, _ *meilisearch.DocumentsQuery, resp *meilisearch.DocumentsResult) error {
			resp.Total = 2
			resp.Results = meilisearch.Hits{
				{"uuid": json.RawMessage(`"uuid-1"`)},
				{"uuid": json.RawMessage(`"uuid-2"`)},
			}
			return nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	uuids, err := ix.ListIndexedDocumentUUIDs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(uuids) != 2 {
		t.Fatalf("got %d uuids, want 2", len(uuids))
	}
	if !uuids["uuid-1"] || !uuids["uuid-2"] {
		t.Errorf("missing expected uuids: %v", uuids)
	}
}

func TestListIndexedDocumentUUIDs_MultiplePages(t *testing.T) {
	t.Parallel()

	callCount := 0
	idx := &mockIndexManager{
		getDocumentsWithContextFn: func(_ context.Context, param *meilisearch.DocumentsQuery, resp *meilisearch.DocumentsResult) error {
			callCount++
			resp.Total = 3
			if param.Offset == 0 {
				resp.Results = meilisearch.Hits{
					{"uuid": json.RawMessage(`"a"`)},
					{"uuid": json.RawMessage(`"b"`)},
				}
			} else {
				resp.Results = meilisearch.Hits{
					{"uuid": json.RawMessage(`"c"`)},
				}
			}
			return nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	uuids, err := ix.ListIndexedDocumentUUIDs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(uuids) != 3 {
		t.Fatalf("got %d uuids, want 3", len(uuids))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestListIndexedDocumentUUIDs_EmptyIndex(t *testing.T) {
	t.Parallel()

	idx := &mockIndexManager{
		getDocumentsWithContextFn: func(_ context.Context, _ *meilisearch.DocumentsQuery, resp *meilisearch.DocumentsResult) error {
			resp.Total = 0
			resp.Results = nil
			return nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	uuids, err := ix.ListIndexedDocumentUUIDs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(uuids) != 0 {
		t.Errorf("got %d uuids, want 0", len(uuids))
	}
}

func TestListIndexedDocumentUUIDs_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("network error")
	idx := &mockIndexManager{
		getDocumentsWithContextFn: func(_ context.Context, _ *meilisearch.DocumentsQuery, _ *meilisearch.DocumentsResult) error {
			return wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	_, err := ix.ListIndexedDocumentUUIDs(context.Background())
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestListIndexedDocumentUUIDs_MalformedUUID_Skipped(t *testing.T) {
	t.Parallel()

	idx := &mockIndexManager{
		getDocumentsWithContextFn: func(_ context.Context, _ *meilisearch.DocumentsQuery, resp *meilisearch.DocumentsResult) error {
			resp.Total = 2
			resp.Results = meilisearch.Hits{
				{"uuid": json.RawMessage(`"valid-uuid"`)},
				{"uuid": json.RawMessage(`123`)},
			}
			return nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	uuids, err := ix.ListIndexedDocumentUUIDs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(uuids) != 1 {
		t.Fatalf("got %d uuids, want 1 (malformed should be skipped)", len(uuids))
	}
	if !uuids["valid-uuid"] {
		t.Error("expected valid-uuid in result")
	}
}

func TestListIndexedDocumentUUIDs_MissingUUIDKey_Skipped(t *testing.T) {
	t.Parallel()

	idx := &mockIndexManager{
		getDocumentsWithContextFn: func(_ context.Context, _ *meilisearch.DocumentsQuery, resp *meilisearch.DocumentsResult) error {
			resp.Total = 1
			resp.Results = meilisearch.Hits{
				{"title": json.RawMessage(`"no uuid here"`)},
			}
			return nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	uuids, err := ix.ListIndexedDocumentUUIDs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(uuids) != 0 {
		t.Errorf("got %d uuids, want 0 (no uuid key)", len(uuids))
	}
}

// ---------------------------------------------------------------------------
// ZIM Archive operations
// ---------------------------------------------------------------------------

func TestIndexZimArchive_Success(t *testing.T) {
	t.Parallel()

	var capturedIndex string
	idx := &mockIndexManager{}
	sm := &mockServiceManager{
		indexFn: func(uid string) meilisearch.IndexManager {
			capturedIndex = uid
			return idx
		},
	}

	ix := newTestIndexer(sm)
	err := ix.IndexZimArchive(context.Background(), ZimArchiveRecord{UUID: "zim-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedIndex != IndexZimArchives {
		t.Errorf("used index %q, want %q", capturedIndex, IndexZimArchives)
	}
}

func TestIndexZimArchive_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fail")
	idx := &mockIndexManager{
		addDocumentsWithContextFn: func(_ context.Context, _ any, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			return nil, wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.IndexZimArchive(context.Background(), ZimArchiveRecord{UUID: "zim-1"})
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestDeleteZimArchive_Success(t *testing.T) {
	t.Parallel()

	idx := &mockIndexManager{}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.DeleteZimArchive(context.Background(), "zim-uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteZimArchive_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fail")
	idx := &mockIndexManager{
		deleteDocumentWithContextFn: func(_ context.Context, _ string, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			return nil, wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.DeleteZimArchive(context.Background(), "zim-uuid")
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestIndexZimArchiveBatch_EmptySlice(t *testing.T) {
	t.Parallel()

	addCalled := false
	idx := &mockIndexManager{
		addDocumentsWithContextFn: func(_ context.Context, _ any, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			addCalled = true
			return &meilisearch.TaskInfo{}, nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.IndexZimArchiveBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addCalled {
		t.Error("AddDocuments should not be called for empty batch")
	}
}

func TestIndexZimArchiveBatch_Success(t *testing.T) {
	t.Parallel()

	idx := &mockIndexManager{}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.IndexZimArchiveBatch(context.Background(), []ZimArchiveRecord{{UUID: "z1"}, {UUID: "z2"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIndexZimArchiveBatch_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("batch fail")
	idx := &mockIndexManager{
		addDocumentsWithContextFn: func(_ context.Context, _ any, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			return nil, wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.IndexZimArchiveBatch(context.Background(), []ZimArchiveRecord{{UUID: "z1"}})
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

// ---------------------------------------------------------------------------
// Git Template operations
// ---------------------------------------------------------------------------

func TestIndexGitTemplate_Success(t *testing.T) {
	t.Parallel()

	var capturedIndex string
	idx := &mockIndexManager{}
	sm := &mockServiceManager{
		indexFn: func(uid string) meilisearch.IndexManager {
			capturedIndex = uid
			return idx
		},
	}

	ix := newTestIndexer(sm)
	err := ix.IndexGitTemplate(context.Background(), GitTemplateRecord{UUID: "git-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedIndex != IndexGitTemplates {
		t.Errorf("used index %q, want %q", capturedIndex, IndexGitTemplates)
	}
}

func TestIndexGitTemplate_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fail")
	idx := &mockIndexManager{
		addDocumentsWithContextFn: func(_ context.Context, _ any, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			return nil, wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.IndexGitTemplate(context.Background(), GitTemplateRecord{UUID: "git-1"})
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestDeleteGitTemplate_Success(t *testing.T) {
	t.Parallel()

	idx := &mockIndexManager{}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.DeleteGitTemplate(context.Background(), "git-uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteGitTemplate_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fail")
	idx := &mockIndexManager{
		deleteDocumentWithContextFn: func(_ context.Context, _ string, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			return nil, wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.DeleteGitTemplate(context.Background(), "git-uuid")
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestSoftDeleteGitTemplate_Success(t *testing.T) {
	t.Parallel()

	var capturedDocs any
	idx := &mockIndexManager{
		addDocumentsWithContextFn: func(_ context.Context, docs any, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			capturedDocs = docs
			return &meilisearch.TaskInfo{TaskUID: 1}, nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.SoftDeleteGitTemplate(context.Background(), "git-uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	records, ok := capturedDocs.([]map[string]any)
	if !ok || len(records) != 1 {
		t.Fatalf("expected 1 record, got %T", capturedDocs)
	}
	if records[0]["__soft_deleted"] != true {
		t.Error("__soft_deleted should be true")
	}
}

func TestSoftDeleteGitTemplate_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("fail")
	idx := &mockIndexManager{
		addDocumentsWithContextFn: func(_ context.Context, _ any, _ *meilisearch.DocumentOptions) (*meilisearch.TaskInfo, error) {
			return nil, wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	ix := newTestIndexer(sm)
	err := ix.SoftDeleteGitTemplate(context.Background(), "git-uuid")
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestIndexer_Searcher_ReturnsNonNil(t *testing.T) {
	t.Parallel()

	sm := &mockServiceManager{}
	ix := newTestIndexer(sm)
	s := ix.Searcher()
	if s == nil {
		t.Fatal("Searcher() returned nil")
	}
}

// ---------------------------------------------------------------------------
// Searcher tests
// ---------------------------------------------------------------------------

func TestSearch_Success(t *testing.T) {
	t.Parallel()

	var capturedQuery string
	var capturedReq *meilisearch.SearchRequest
	idx := &mockIndexManager{
		searchWithContextFn: func(_ context.Context, query string, req *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
			capturedQuery = query
			capturedReq = req
			return &meilisearch.SearchResponse{EstimatedTotalHits: 5}, nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	c := newTestClient(sm, testLogger())
	s := NewSearcher(c, testLogger())

	resp, err := s.Search(context.Background(), SearchParams{
		Query:    "golang",
		IndexUID: IndexDocuments,
		Limit:    10,
		Offset:   5,
		Sort:     []string{"created_at:desc"},
		Filters:  "status = 'published'",
	})
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if resp.EstimatedTotalHits != 5 {
		t.Errorf("EstimatedTotalHits = %d, want 5", resp.EstimatedTotalHits)
	}
	if capturedQuery != "golang" {
		t.Errorf("query = %q, want %q", capturedQuery, "golang")
	}
	if capturedReq.Limit != 10 {
		t.Errorf("limit = %d, want 10", capturedReq.Limit)
	}
	if capturedReq.Offset != 5 {
		t.Errorf("offset = %d, want 5", capturedReq.Offset)
	}
	if capturedReq.Filter != "status = 'published'" {
		t.Errorf("filter = %v, want %q", capturedReq.Filter, "status = 'published'")
	}
}

func TestSearch_DefaultLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		limit     int64
		wantLimit int64
	}{
		{"zero uses default 20", 0, 20},
		{"negative uses default 20", -5, 20},
		{"positive is preserved", 50, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedLimit int64
			idx := &mockIndexManager{
				searchWithContextFn: func(_ context.Context, _ string, req *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
					capturedLimit = req.Limit
					return &meilisearch.SearchResponse{}, nil
				},
			}
			sm := &mockServiceManager{
				indexFn: func(_ string) meilisearch.IndexManager { return idx },
			}

			c := newTestClient(sm, testLogger())
			s := NewSearcher(c, testLogger())
			_, err := s.Search(context.Background(), SearchParams{
				Query:    "test",
				IndexUID: IndexDocuments,
				Limit:    tt.limit,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if capturedLimit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", capturedLimit, tt.wantLimit)
			}
		})
	}
}

func TestSearch_HighlightSettings(t *testing.T) {
	t.Parallel()

	var capturedReq *meilisearch.SearchRequest
	idx := &mockIndexManager{
		searchWithContextFn: func(_ context.Context, _ string, req *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
			capturedReq = req
			return &meilisearch.SearchResponse{}, nil
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	c := newTestClient(sm, testLogger())
	s := NewSearcher(c, testLogger())
	_, err := s.Search(context.Background(), SearchParams{Query: "test", IndexUID: IndexDocuments})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedReq.HighlightPreTag != "<em>" {
		t.Errorf("HighlightPreTag = %q, want %q", capturedReq.HighlightPreTag, "<em>")
	}
	if capturedReq.HighlightPostTag != "</em>" {
		t.Errorf("HighlightPostTag = %q, want %q", capturedReq.HighlightPostTag, "</em>")
	}
	if len(capturedReq.AttributesToHighlight) != 2 {
		t.Errorf("AttributesToHighlight len = %d, want 2", len(capturedReq.AttributesToHighlight))
	}
}

func TestSearch_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("search failed")
	idx := &mockIndexManager{
		searchWithContextFn: func(_ context.Context, _ string, _ *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
			return nil, wantErr
		},
	}
	sm := &mockServiceManager{
		indexFn: func(_ string) meilisearch.IndexManager { return idx },
	}

	c := newTestClient(sm, testLogger())
	s := NewSearcher(c, testLogger())
	_, err := s.Search(context.Background(), SearchParams{Query: "test", IndexUID: IndexDocuments})
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestFederatedSearch_DefaultIndexes(t *testing.T) {
	t.Parallel()

	var capturedQueries []*meilisearch.SearchRequest
	sm := &mockServiceManager{
		multiSearchWithContextFn: func(_ context.Context, req *meilisearch.MultiSearchRequest) (*meilisearch.MultiSearchResponse, error) {
			capturedQueries = req.Queries
			return &meilisearch.MultiSearchResponse{}, nil
		},
	}

	c := newTestClient(sm, testLogger())
	s := NewSearcher(c, testLogger())

	_, err := s.FederatedSearch(context.Background(), FederatedSearchParams{Query: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedQueries) != 3 {
		t.Fatalf("expected 3 queries (all indexes), got %d", len(capturedQueries))
	}

	wantUIDs := []string{IndexDocuments, IndexZimArchives, IndexGitTemplates}
	for i, want := range wantUIDs {
		if capturedQueries[i].IndexUID != want {
			t.Errorf("query[%d].IndexUID = %q, want %q", i, capturedQueries[i].IndexUID, want)
		}
	}
}

func TestFederatedSearch_ExplicitIndexes(t *testing.T) {
	t.Parallel()

	var capturedQueries []*meilisearch.SearchRequest
	sm := &mockServiceManager{
		multiSearchWithContextFn: func(_ context.Context, req *meilisearch.MultiSearchRequest) (*meilisearch.MultiSearchResponse, error) {
			capturedQueries = req.Queries
			return &meilisearch.MultiSearchResponse{}, nil
		},
	}

	c := newTestClient(sm, testLogger())
	s := NewSearcher(c, testLogger())

	_, err := s.FederatedSearch(context.Background(), FederatedSearchParams{
		Query:   "test",
		Indexes: []string{IndexDocuments, IndexZimArchives},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedQueries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(capturedQueries))
	}
}

func TestFederatedSearch_DefaultLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		limit     int64
		wantLimit int64
	}{
		{"zero uses default 20", 0, 20},
		{"negative uses default 20", -1, 20},
		{"positive is preserved", 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedLimit int64
			sm := &mockServiceManager{
				multiSearchWithContextFn: func(_ context.Context, req *meilisearch.MultiSearchRequest) (*meilisearch.MultiSearchResponse, error) {
					capturedLimit = req.Federation.Limit
					return &meilisearch.MultiSearchResponse{}, nil
				},
			}

			c := newTestClient(sm, testLogger())
			s := NewSearcher(c, testLogger())
			_, err := s.FederatedSearch(context.Background(), FederatedSearchParams{
				Query: "test",
				Limit: tt.limit,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if capturedLimit != tt.wantLimit {
				t.Errorf("limit = %d, want %d", capturedLimit, tt.wantLimit)
			}
		})
	}
}

func TestFederatedSearch_SoftDeleteFilters(t *testing.T) {
	t.Parallel()

	var capturedQueries []*meilisearch.SearchRequest
	sm := &mockServiceManager{
		multiSearchWithContextFn: func(_ context.Context, req *meilisearch.MultiSearchRequest) (*meilisearch.MultiSearchResponse, error) {
			capturedQueries = req.Queries
			return &meilisearch.MultiSearchResponse{}, nil
		},
	}

	c := newTestClient(sm, testLogger())
	s := NewSearcher(c, testLogger())

	_, err := s.FederatedSearch(context.Background(), FederatedSearchParams{Query: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantFilters := []string{
		"__soft_deleted = false", // documents
		"",                       // zim_archives (no soft delete)
		"__soft_deleted = false", // git_templates
	}

	for i, want := range wantFilters {
		got, _ := capturedQueries[i].Filter.(string)
		if got != want {
			t.Errorf("query[%d] filter = %q, want %q", i, got, want)
		}
	}
}

func TestFederatedSearch_Error(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("multi search failed")
	sm := &mockServiceManager{
		multiSearchWithContextFn: func(_ context.Context, _ *meilisearch.MultiSearchRequest) (*meilisearch.MultiSearchResponse, error) {
			return nil, wantErr
		},
	}

	c := newTestClient(sm, testLogger())
	s := NewSearcher(c, testLogger())
	_, err := s.FederatedSearch(context.Background(), FederatedSearchParams{Query: "test"})
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestFederatedSearch_OffsetPassedThrough(t *testing.T) {
	t.Parallel()

	var capturedOffset int64
	sm := &mockServiceManager{
		multiSearchWithContextFn: func(_ context.Context, req *meilisearch.MultiSearchRequest) (*meilisearch.MultiSearchResponse, error) {
			capturedOffset = req.Federation.Offset
			return &meilisearch.MultiSearchResponse{}, nil
		},
	}

	c := newTestClient(sm, testLogger())
	s := NewSearcher(c, testLogger())
	_, err := s.FederatedSearch(context.Background(), FederatedSearchParams{
		Query:  "test",
		Offset: 42,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedOffset != 42 {
		t.Errorf("offset = %d, want 42", capturedOffset)
	}
}

func TestFederatedSearch_QueryPassedToAllSubQueries(t *testing.T) {
	t.Parallel()

	var capturedQueries []*meilisearch.SearchRequest
	sm := &mockServiceManager{
		multiSearchWithContextFn: func(_ context.Context, req *meilisearch.MultiSearchRequest) (*meilisearch.MultiSearchResponse, error) {
			capturedQueries = req.Queries
			return &meilisearch.MultiSearchResponse{}, nil
		},
	}

	c := newTestClient(sm, testLogger())
	s := NewSearcher(c, testLogger())
	_, err := s.FederatedSearch(context.Background(), FederatedSearchParams{
		Query:   "my search term",
		Indexes: []string{IndexDocuments, IndexZimArchives},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, q := range capturedQueries {
		if q.Query != "my search term" {
			t.Errorf("query[%d].Query = %q, want %q", i, q.Query, "my search term")
		}
	}
}
