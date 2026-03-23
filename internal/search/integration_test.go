//go:build integration

package search

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcmeili "github.com/testcontainers/testcontainers-go/modules/meilisearch"
)

var (
	testMeiliHost string
	testMeiliKey  = "testMasterKey123"
)

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("docker"); err != nil {
		log.Printf("skipping integration tests: docker not found in PATH")
		os.Exit(0)
	}

	ctx := context.Background()

	container, err := tcmeili.Run(ctx,
		"getmeili/meilisearch:v1.12",
		tcmeili.WithMasterKey(testMeiliKey),
	)
	if err != nil {
		log.Printf("skipping integration tests: starting meilisearch container: %v", err)
		os.Exit(0)
	}

	defer func() {
		if err := container.Terminate(ctx); err != nil {
			log.Printf("terminating meilisearch container: %v", err)
		}
	}()

	host, err := container.Host(ctx)
	if err != nil {
		log.Fatalf("getting container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "7700/tcp")
	if err != nil {
		log.Fatalf("getting container port: %v", err)
	}
	testMeiliHost = fmt.Sprintf("http://%s:%s", host, port.Port())

	os.Exit(m.Run())
}

func integrationLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func integrationClient() *Client {
	return NewClient(testMeiliHost, testMeiliKey, integrationLogger())
}

// waitForAllTasks waits for all enqueued Meilisearch tasks to complete.
func waitForAllTasks(t *testing.T, client *Client) {
	t.Helper()
	ctx := context.Background()
	sm := client.ServiceManager()

	for {
		resp, err := sm.GetTasksWithContext(ctx, &meilisearch.TasksQuery{
			Statuses: []meilisearch.TaskStatus{meilisearch.TaskStatusEnqueued, meilisearch.TaskStatusProcessing},
		})
		require.NoError(t, err)
		if resp.Total == 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// clearAllIndexes deletes all documents from all 4 indexes and waits for completion.
func clearAllIndexes(t *testing.T, client *Client) {
	t.Helper()
	ctx := context.Background()
	sm := client.ServiceManager()

	for _, uid := range []string{IndexDocuments, IndexZimArchives, IndexGitTemplates} {
		idx := sm.Index(uid)
		_, _ = idx.DeleteAllDocumentsWithContext(ctx, nil)
	}
	waitForAllTasks(t, client)
}

func TestHealthy_Integration(t *testing.T) {
	client := integrationClient()
	assert.True(t, client.Healthy())
}

func TestConfigureIndexes_Integration(t *testing.T) {
	client := integrationClient()
	ctx := context.Background()

	err := client.ConfigureIndexes(ctx)
	require.NoError(t, err)
	waitForAllTasks(t, client)

	sm := client.ServiceManager()

	t.Run("all indexes exist", func(t *testing.T) {
		for _, uid := range []string{IndexDocuments, IndexZimArchives, IndexGitTemplates} {
			idx := sm.Index(uid)
			info, err := idx.FetchInfoWithContext(ctx)
			require.NoError(t, err, "index %s should exist", uid)
			assert.Equal(t, "uuid", info.PrimaryKey)
		}
	})

	t.Run("documents index settings applied", func(t *testing.T) {
		idx := sm.Index(IndexDocuments)
		settings, err := idx.GetSettingsWithContext(ctx)
		require.NoError(t, err)

		assert.Contains(t, settings.FilterableAttributes, "__soft_deleted")
		assert.Contains(t, settings.FilterableAttributes, "file_type")
		assert.Contains(t, settings.SearchableAttributes, "title")
		assert.Contains(t, settings.SearchableAttributes, "content")
		assert.Contains(t, settings.SortableAttributes, "created_at")
	})

	t.Run("idempotent", func(t *testing.T) {
		err := client.ConfigureIndexes(ctx)
		require.NoError(t, err)
	})
}

func TestIndexAndSearchDocuments(t *testing.T) {
	client := integrationClient()
	ctx := context.Background()
	require.NoError(t, client.ConfigureIndexes(ctx))
	waitForAllTasks(t, client)
	clearAllIndexes(t, client)

	indexer := NewIndexer(client, integrationLogger())
	searcher := NewSearcher(client, integrationLogger())

	docs := []DocumentRecord{
		{UUID: "doc-001", Title: "Go Programming Guide", Content: "Learn Go programming", FileType: "pdf", Status: "indexed", IsPublic: true},
		{UUID: "doc-002", Title: "Python Cookbook", Content: "Python recipes", FileType: "pdf", Status: "indexed", IsPublic: true},
		{UUID: "doc-003", Title: "Go Testing Patterns", Content: "Testing in Go", FileType: "markdown", Status: "indexed", IsPublic: true},
	}

	for _, doc := range docs {
		require.NoError(t, indexer.IndexDocument(ctx, doc))
	}
	waitForAllTasks(t, client)

	t.Run("search returns matching documents", func(t *testing.T) {
		resp, err := searcher.Search(ctx, SearchParams{
			Query:    "Go",
			IndexUID: IndexDocuments,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(2), resp.EstimatedTotalHits)
	})

	t.Run("search with no matches", func(t *testing.T) {
		resp, err := searcher.Search(ctx, SearchParams{
			Query:    "Kubernetes",
			IndexUID: IndexDocuments,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(0), resp.EstimatedTotalHits)
	})

	t.Run("search with filter", func(t *testing.T) {
		resp, err := searcher.Search(ctx, SearchParams{
			Query:    "Go",
			IndexUID: IndexDocuments,
			Filters:  "file_type = pdf",
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), resp.EstimatedTotalHits)
	})
}

func TestSoftDeleteDocument_Integration(t *testing.T) {
	client := integrationClient()
	ctx := context.Background()
	require.NoError(t, client.ConfigureIndexes(ctx))
	waitForAllTasks(t, client)
	clearAllIndexes(t, client)

	indexer := NewIndexer(client, integrationLogger())
	searcher := NewSearcher(client, integrationLogger())

	doc := DocumentRecord{
		UUID: "soft-del-001", Title: "Deletable Doc", Content: "Will be soft-deleted",
		FileType: "pdf", Status: "indexed", IsPublic: true, SoftDeleted: false,
	}
	require.NoError(t, indexer.IndexDocument(ctx, doc))
	waitForAllTasks(t, client)

	// Verify it's searchable with soft-delete filter.
	resp, err := searcher.Search(ctx, SearchParams{
		Query: "Deletable", IndexUID: IndexDocuments,
		Filters: "__soft_deleted = false",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.EstimatedTotalHits)

	// Soft-delete it.
	require.NoError(t, indexer.SoftDeleteDocument(ctx, "soft-del-001"))
	waitForAllTasks(t, client)

	// Should be hidden with soft-delete filter.
	resp, err = searcher.Search(ctx, SearchParams{
		Query: "Deletable", IndexUID: IndexDocuments,
		Filters: "__soft_deleted = false",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.EstimatedTotalHits)

	// Should still be visible when filtering for soft-deleted records.
	resp, err = searcher.Search(ctx, SearchParams{
		Query: "", IndexUID: IndexDocuments,
		Filters: "__soft_deleted = true",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.EstimatedTotalHits)
}

func TestDeleteDocument_Integration(t *testing.T) {
	client := integrationClient()
	ctx := context.Background()
	require.NoError(t, client.ConfigureIndexes(ctx))
	waitForAllTasks(t, client)
	clearAllIndexes(t, client)

	indexer := NewIndexer(client, integrationLogger())
	searcher := NewSearcher(client, integrationLogger())

	doc := DocumentRecord{
		UUID: "hard-del-001", Title: "Hard Delete Doc", Content: "Will be hard-deleted",
		FileType: "pdf", Status: "indexed", IsPublic: true,
	}
	require.NoError(t, indexer.IndexDocument(ctx, doc))
	waitForAllTasks(t, client)

	require.NoError(t, indexer.DeleteDocument(ctx, "hard-del-001"))
	waitForAllTasks(t, client)

	resp, err := searcher.Search(ctx, SearchParams{
		Query: "Hard Delete", IndexUID: IndexDocuments,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.EstimatedTotalHits)
}

func TestIndexBatch_Integration(t *testing.T) {
	client := integrationClient()
	ctx := context.Background()
	require.NoError(t, client.ConfigureIndexes(ctx))
	waitForAllTasks(t, client)
	clearAllIndexes(t, client)

	indexer := NewIndexer(client, integrationLogger())
	searcher := NewSearcher(client, integrationLogger())

	batch := make([]DocumentRecord, 5)
	for i := range batch {
		batch[i] = DocumentRecord{
			UUID:     fmt.Sprintf("batch-%03d", i+1),
			Title:    fmt.Sprintf("Batch Document %d", i+1),
			Content:  "batch content",
			FileType: "pdf",
			Status:   "indexed",
			IsPublic: true,
		}
	}

	require.NoError(t, indexer.IndexBatch(ctx, batch))
	waitForAllTasks(t, client)

	resp, err := searcher.Search(ctx, SearchParams{
		Query: "Batch Document", IndexUID: IndexDocuments, Limit: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5), resp.EstimatedTotalHits)
}

func TestFederatedSearch_Integration(t *testing.T) {
	client := integrationClient()
	ctx := context.Background()
	require.NoError(t, client.ConfigureIndexes(ctx))
	waitForAllTasks(t, client)
	clearAllIndexes(t, client)

	indexer := NewIndexer(client, integrationLogger())
	searcher := NewSearcher(client, integrationLogger())

	// Index one record in each of the 3 indexes.
	require.NoError(t, indexer.IndexDocument(ctx, DocumentRecord{
		UUID: "fed-doc-001", Title: "Federated Document", Content: "federated test",
		FileType: "pdf", Status: "indexed", IsPublic: true,
	}))
	require.NoError(t, indexer.IndexZimArchive(ctx, ZimArchiveRecord{
		UUID: "fed-zim-001", Name: "federated-zim", Title: "Federated ZIM Archive",
		Language: "en", Category: "wikipedia",
	}))
	require.NoError(t, indexer.IndexGitTemplate(ctx, GitTemplateRecord{
		UUID: "fed-git-001", Name: "Federated Git Template", Slug: "federated-git",
		Status: "synced", IsPublic: true,
	}))
	waitForAllTasks(t, client)

	t.Run("searches all indexes", func(t *testing.T) {
		resp, err := searcher.FederatedSearch(ctx, FederatedSearchParams{
			Query: "Federated",
			Limit: 20,
		})
		require.NoError(t, err)
		// All 3 records should match "Federated".
		assert.GreaterOrEqual(t, resp.Hits.Len(), 3)
	})

	t.Run("searches specific indexes", func(t *testing.T) {
		resp, err := searcher.FederatedSearch(ctx, FederatedSearchParams{
			Query:   "Federated",
			Indexes: []string{IndexDocuments, IndexZimArchives},
			Limit:   20,
		})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.Hits.Len(), 2)
	})
}

func TestListIndexedDocumentUUIDs_Integration(t *testing.T) {
	client := integrationClient()
	ctx := context.Background()
	require.NoError(t, client.ConfigureIndexes(ctx))
	waitForAllTasks(t, client)
	clearAllIndexes(t, client)

	indexer := NewIndexer(client, integrationLogger())

	expectedUUIDs := []string{"list-uuid-001", "list-uuid-002", "list-uuid-003"}
	for _, uuid := range expectedUUIDs {
		require.NoError(t, indexer.IndexDocument(ctx, DocumentRecord{
			UUID: uuid, Title: "Doc " + uuid, FileType: "pdf", Status: "indexed", IsPublic: true,
		}))
	}
	waitForAllTasks(t, client)

	uuids, err := indexer.ListIndexedDocumentUUIDs(ctx)
	require.NoError(t, err)
	assert.Len(t, uuids, 3)
	for _, uuid := range expectedUUIDs {
		assert.True(t, uuids[uuid], "expected UUID %s in result set", uuid)
	}
}

func TestIndexZimArchive_Integration(t *testing.T) {
	client := integrationClient()
	ctx := context.Background()
	require.NoError(t, client.ConfigureIndexes(ctx))
	waitForAllTasks(t, client)
	clearAllIndexes(t, client)

	indexer := NewIndexer(client, integrationLogger())
	searcher := NewSearcher(client, integrationLogger())

	rec := ZimArchiveRecord{
		UUID: "zim-001", Name: "wikipedia-en", Title: "English Wikipedia",
		Description: "Full English Wikipedia archive", Language: "en",
		Category: "wikipedia", ArticleCount: 6000000,
	}
	require.NoError(t, indexer.IndexZimArchive(ctx, rec))
	waitForAllTasks(t, client)

	resp, err := searcher.Search(ctx, SearchParams{
		Query: "Wikipedia", IndexUID: IndexZimArchives,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.EstimatedTotalHits)
}


func TestIndexGitTemplate_Integration(t *testing.T) {
	client := integrationClient()
	ctx := context.Background()
	require.NoError(t, client.ConfigureIndexes(ctx))
	waitForAllTasks(t, client)
	clearAllIndexes(t, client)

	indexer := NewIndexer(client, integrationLogger())
	searcher := NewSearcher(client, integrationLogger())

	rec := GitTemplateRecord{
		UUID: "git-001", Name: "Go Microservice", Slug: "go-microservice",
		Description: "A Go microservice template", Category: "backend",
		Status: "synced", IsPublic: true, SoftDeleted: false,
	}
	require.NoError(t, indexer.IndexGitTemplate(ctx, rec))
	waitForAllTasks(t, client)

	// Searchable.
	resp, err := searcher.Search(ctx, SearchParams{
		Query: "Microservice", IndexUID: IndexGitTemplates,
		Filters: "__soft_deleted = false",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.EstimatedTotalHits)

	// Soft-delete.
	require.NoError(t, indexer.SoftDeleteGitTemplate(ctx, "git-001"))
	waitForAllTasks(t, client)

	resp, err = searcher.Search(ctx, SearchParams{
		Query: "Microservice", IndexUID: IndexGitTemplates,
		Filters: "__soft_deleted = false",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.EstimatedTotalHits)
}
