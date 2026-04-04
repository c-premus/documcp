//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/c-premus/documcp/internal/database"
	"github.com/c-premus/documcp/internal/model"
	"github.com/c-premus/documcp/internal/testutil"
)

func TestDocumentRepository_CreateAndFind(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	tests := []struct {
		name     string
		uuid     string
		title    string
		fileType string
		isPublic bool
		content  string
	}{
		{
			name:     "basic document",
			uuid:     testUUID("create-find-basic"),
			title:    "Basic Doc",
			fileType: "pdf",
			isPublic: false,
			content:  "hello world",
		},
		{
			name:     "public document",
			uuid:     testUUID("create-find-public"),
			title:    "Public Doc",
			fileType: "markdown",
			isPublic: true,
			content:  "public content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := testutil.NewDocument(
				testutil.WithDocumentID(0),
				testutil.WithDocumentUUID(tt.uuid),
				testutil.WithDocumentTitle(tt.title),
				testutil.WithDocumentFileType(tt.fileType),
				testutil.WithDocumentIsPublic(tt.isPublic),
				testutil.WithDocumentContent(tt.content),
			)

			if err := repo.Create(ctx, doc); err != nil {
				t.Fatalf("Create() error: %v", err)
			}
			if doc.ID == 0 {
				t.Fatal("Create() did not set ID on document")
			}

			// FindByUUID
			found, err := repo.FindByUUID(ctx, tt.uuid)
			if err != nil {
				t.Fatalf("FindByUUID() error: %v", err)
			}
			if found.UUID != tt.uuid {
				t.Errorf("FindByUUID() UUID = %q, want %q", found.UUID, tt.uuid)
			}
			if found.Title != tt.title {
				t.Errorf("FindByUUID() Title = %q, want %q", found.Title, tt.title)
			}
			if found.FileType != tt.fileType {
				t.Errorf("FindByUUID() FileType = %q, want %q", found.FileType, tt.fileType)
			}
			if found.IsPublic != tt.isPublic {
				t.Errorf("FindByUUID() IsPublic = %v, want %v", found.IsPublic, tt.isPublic)
			}
			if !found.Content.Valid || found.Content.String != tt.content {
				t.Errorf("FindByUUID() Content = %q, want %q", found.Content.String, tt.content)
			}
			if !found.CreatedAt.Valid {
				t.Error("FindByUUID() CreatedAt is not set")
			}
			if !found.UpdatedAt.Valid {
				t.Error("FindByUUID() UpdatedAt is not set")
			}

			// FindByID
			foundByID, err := repo.FindByID(ctx, doc.ID)
			if err != nil {
				t.Fatalf("FindByID() error: %v", err)
			}
			if foundByID.UUID != tt.uuid {
				t.Errorf("FindByID() UUID = %q, want %q", foundByID.UUID, tt.uuid)
			}
			if foundByID.ID != doc.ID {
				t.Errorf("FindByID() ID = %d, want %d", foundByID.ID, doc.ID)
			}
		})
	}
}

func TestDocumentRepository_Update(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	tests := []struct {
		name      string
		uuid      string
		newTitle  string
		newStatus model.DocumentStatus
	}{
		{
			name:      "update title and status",
			uuid:      testUUID("update-title-status"),
			newTitle:  "Updated Title",
			newStatus: model.DocumentStatus("processing"),
		},
		{
			name:      "update to error status",
			uuid:      testUUID("update-error-status"),
			newTitle:  "Error Doc",
			newStatus: model.DocumentStatus("error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := testutil.NewDocument(
				testutil.WithDocumentID(0),
				testutil.WithDocumentUUID(tt.uuid),
				testutil.WithDocumentTitle("Original Title"),
				testutil.WithDocumentStatus(model.DocumentStatusIndexed),
			)
			if err := repo.Create(ctx, doc); err != nil {
				t.Fatalf("Create() error: %v", err)
			}

			doc.Title = tt.newTitle
			doc.Status = tt.newStatus

			if err := repo.Update(ctx, doc); err != nil {
				t.Fatalf("Update() error: %v", err)
			}

			found, err := repo.FindByUUID(ctx, tt.uuid)
			if err != nil {
				t.Fatalf("FindByUUID() after Update error: %v", err)
			}
			if found.Title != tt.newTitle {
				t.Errorf("Title after Update = %q, want %q", found.Title, tt.newTitle)
			}
			if found.Status != tt.newStatus {
				t.Errorf("Status after Update = %q, want %q", found.Status, tt.newStatus)
			}
		})
	}
}

func TestDocumentRepository_SoftDelete(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	doc := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("soft-delete-test")),
		testutil.WithDocumentTitle("To Be Deleted"),
	)
	if err := repo.Create(ctx, doc); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	countBefore, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() before delete error: %v", err)
	}
	if countBefore < 1 {
		t.Fatalf("Count() before delete = %d, want >= 1", countBefore)
	}

	if err := repo.SoftDelete(ctx, doc.ID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	// FindByUUID should fail for soft-deleted documents.
	_, err = repo.FindByUUID(ctx, testUUID("soft-delete-test"))
	if err == nil {
		t.Fatal("FindByUUID() after SoftDelete expected error, got nil")
	}

	// FindByID should also fail for soft-deleted documents.
	_, err = repo.FindByID(ctx, doc.ID)
	if err == nil {
		t.Fatal("FindByID() after SoftDelete expected error, got nil")
	}

	countAfter, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() after delete error: %v", err)
	}
	if countAfter != countBefore-1 {
		t.Errorf("Count() after SoftDelete = %d, want %d", countAfter, countBefore-1)
	}
}

func TestDocumentRepository_Count(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	// Count should be zero on a clean database.
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() on empty table error: %v", err)
	}
	if count != 0 {
		t.Fatalf("Count() on empty table = %d, want 0", count)
	}

	tests := []struct {
		name      string
		insertN   int
		wantCount int
	}{
		{name: "single document", insertN: 1, wantCount: 1},
		{name: "three documents", insertN: 3, wantCount: 4}, // cumulative: 1 + 3
	}

	inserted := 0
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range tt.insertN {
				inserted++
				doc := testutil.NewDocument(
					testutil.WithDocumentID(0),
					testutil.WithDocumentUUID(testUUID(fmt.Sprintf("count-test-%d", inserted))),
					testutil.WithDocumentTitle(fmt.Sprintf("Count Doc %d", inserted)),
				)
				if err := repo.Create(ctx, doc); err != nil {
					t.Fatalf("Create() doc %d error: %v", i, err)
				}
			}

			got, err := repo.Count(ctx)
			if err != nil {
				t.Fatalf("Count() error: %v", err)
			}
			if got != tt.wantCount {
				t.Errorf("Count() = %d, want %d", got, tt.wantCount)
			}
		})
	}
}

func TestDocumentRepository_Tags(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	doc := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("tags-test")),
		testutil.WithDocumentTitle("Tagged Document"),
	)
	if err := repo.Create(ctx, doc); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	t.Run("initial tags empty", func(t *testing.T) {
		tags, err := repo.TagsForDocument(ctx, doc.ID)
		if err != nil {
			t.Fatalf("TagsForDocument() error: %v", err)
		}
		if len(tags) != 0 {
			t.Errorf("TagsForDocument() returned %d tags, want 0", len(tags))
		}
	})

	t.Run("replace with first set", func(t *testing.T) {
		firstTags := []string{"alpha", "beta", "gamma"}
		if err := repo.ReplaceTags(ctx, doc.ID, firstTags); err != nil {
			t.Fatalf("ReplaceTags() error: %v", err)
		}

		tags, err := repo.TagsForDocument(ctx, doc.ID)
		if err != nil {
			t.Fatalf("TagsForDocument() error: %v", err)
		}
		if len(tags) != len(firstTags) {
			t.Fatalf("TagsForDocument() returned %d tags, want %d", len(tags), len(firstTags))
		}

		// Tags are ordered by tag name from the query.
		for i, want := range firstTags {
			if tags[i].Tag != want {
				t.Errorf("tag[%d] = %q, want %q", i, tags[i].Tag, want)
			}
			if tags[i].DocumentID != doc.ID {
				t.Errorf("tag[%d].DocumentID = %d, want %d", i, tags[i].DocumentID, doc.ID)
			}
		}
	})

	t.Run("replace with second set", func(t *testing.T) {
		secondTags := []string{"delta", "epsilon"}
		if err := repo.ReplaceTags(ctx, doc.ID, secondTags); err != nil {
			t.Fatalf("ReplaceTags() error: %v", err)
		}

		tags, err := repo.TagsForDocument(ctx, doc.ID)
		if err != nil {
			t.Fatalf("TagsForDocument() error: %v", err)
		}
		if len(tags) != len(secondTags) {
			t.Fatalf("TagsForDocument() returned %d tags, want %d", len(tags), len(secondTags))
		}
		for i, want := range secondTags {
			if tags[i].Tag != want {
				t.Errorf("tag[%d] = %q, want %q", i, tags[i].Tag, want)
			}
		}
	})

	t.Run("replace with empty set", func(t *testing.T) {
		if err := repo.ReplaceTags(ctx, doc.ID, []string{}); err != nil {
			t.Fatalf("ReplaceTags() with empty slice error: %v", err)
		}

		tags, err := repo.TagsForDocument(ctx, doc.ID)
		if err != nil {
			t.Fatalf("TagsForDocument() error: %v", err)
		}
		if len(tags) != 0 {
			t.Errorf("TagsForDocument() after clearing returned %d tags, want 0", len(tags))
		}
	})
}

func TestDocumentRepository_CreateVersion(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	doc := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("version-test")),
		testutil.WithDocumentTitle("Versioned Document"),
	)
	if err := repo.Create(ctx, doc); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	tests := []struct {
		name     string
		version  int
		filePath string
		content  string
		metadata string
	}{
		{
			name:     "first version",
			version:  1,
			filePath: "/tmp/v1/document.pdf",
			content:  "version 1 content",
			metadata: `{"author":"alice"}`,
		},
		{
			name:     "second version",
			version:  2,
			filePath: "/tmp/v2/document.pdf",
			content:  "version 2 content",
			metadata: `{"author":"bob"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver := &model.DocumentVersion{
				DocumentID: doc.ID,
				Version:    tt.version,
				FilePath:   tt.filePath,
				Content:    sql.NullString{String: tt.content, Valid: true},
				Metadata:   sql.NullString{String: tt.metadata, Valid: true},
			}

			if err := repo.CreateVersion(ctx, ver); err != nil {
				t.Fatalf("CreateVersion() error: %v", err)
			}
			if ver.ID == 0 {
				t.Fatal("CreateVersion() did not set ID on version")
			}

			// Verify the version was persisted by querying directly.
			found, err := database.Get[model.DocumentVersion](ctx, testPool,
				`SELECT * FROM document_versions WHERE id = $1`, ver.ID)
			if err != nil {
				t.Fatalf("querying created version: %v", err)
			}
			if found.DocumentID != doc.ID {
				t.Errorf("DocumentID = %d, want %d", found.DocumentID, doc.ID)
			}
			if found.Version != tt.version {
				t.Errorf("Version = %d, want %d", found.Version, tt.version)
			}
			if found.FilePath != tt.filePath {
				t.Errorf("FilePath = %q, want %q", found.FilePath, tt.filePath)
			}
			if !found.Content.Valid || found.Content.String != tt.content {
				t.Errorf("Content = %q, want %q", found.Content.String, tt.content)
			}
			if !found.Metadata.Valid || found.Metadata.String != tt.metadata {
				t.Errorf("Metadata = %q, want %q", found.Metadata.String, tt.metadata)
			}
			if !found.CreatedAt.Valid {
				t.Error("CreatedAt is not set")
			}
		})
	}
}

func TestDocumentRepository_ListAllUUIDs(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	t.Run("empty table returns empty slice", func(t *testing.T) {
		uuids, err := repo.ListAllUUIDs(ctx)
		if err != nil {
			t.Fatalf("ListAllUUIDs() error: %v", err)
		}
		if len(uuids) != 0 {
			t.Errorf("ListAllUUIDs() returned %d uuids, want 0", len(uuids))
		}
	})

	// Insert 3 documents: 2 active, 1 soft-deleted.
	doc1 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-uuids-001")),
		testutil.WithDocumentTitle("UUID Doc One"),
	)
	doc2 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-uuids-002")),
		testutil.WithDocumentTitle("UUID Doc Two"),
	)
	doc3 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-uuids-003")),
		testutil.WithDocumentTitle("UUID Doc Three (Deleted)"),
	)

	for _, doc := range []*model.Document{doc1, doc2, doc3} {
		if err := repo.Create(ctx, doc); err != nil {
			t.Fatalf("Create(%s) error: %v", doc.Title, err)
		}
	}

	// Soft-delete the third document.
	if err := repo.SoftDelete(ctx, doc3.ID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	t.Run("returns all UUIDs including soft-deleted", func(t *testing.T) {
		uuids, err := repo.ListAllUUIDs(ctx)
		if err != nil {
			t.Fatalf("ListAllUUIDs() error: %v", err)
		}
		if len(uuids) != 3 {
			t.Fatalf("ListAllUUIDs() returned %d uuids, want 3", len(uuids))
		}

		// Verify all expected UUIDs are present.
		uuidSet := make(map[string]bool)
		for _, u := range uuids {
			uuidSet[u] = true
		}
		for _, want := range []string{testUUID("list-uuids-001"), testUUID("list-uuids-002"), testUUID("list-uuids-003")} {
			if !uuidSet[want] {
				t.Errorf("ListAllUUIDs() missing expected UUID %q", want)
			}
		}
	})
}

func TestDocumentRepository_ListActiveFilePaths(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	// Insert documents with varying file path states.
	docWithPath := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("active-path-001")),
		testutil.WithDocumentTitle("Doc With Path"),
		testutil.WithDocumentFilePath("/tmp/documents/file1.pdf"),
	)
	docWithPath2 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("active-path-002")),
		testutil.WithDocumentTitle("Doc With Path 2"),
		testutil.WithDocumentFilePath("/tmp/documents/file2.pdf"),
	)
	docNoPath := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("active-path-003")),
		testutil.WithDocumentTitle("Doc Without Path"),
		testutil.WithDocumentFilePath(""),
	)
	docDeletedWithPath := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("active-path-004")),
		testutil.WithDocumentTitle("Deleted Doc With Path"),
		testutil.WithDocumentFilePath("/tmp/documents/deleted.pdf"),
	)

	for _, doc := range []*model.Document{docWithPath, docWithPath2, docNoPath, docDeletedWithPath} {
		if err := repo.Create(ctx, doc); err != nil {
			t.Fatalf("Create(%s) error: %v", doc.Title, err)
		}
	}

	// Soft-delete one document that has a file path.
	if err := repo.SoftDelete(ctx, docDeletedWithPath.ID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	paths, err := repo.ListActiveFilePaths(ctx)
	if err != nil {
		t.Fatalf("ListActiveFilePaths() error: %v", err)
	}

	// Should return only non-deleted documents with non-empty file paths.
	if len(paths) != 2 {
		t.Fatalf("ListActiveFilePaths() returned %d paths, want 2", len(paths))
	}

	// Verify the returned paths match expected documents.
	pathSet := make(map[string]string) // uuid -> file_path
	for _, p := range paths {
		pathSet[p.UUID] = p.FilePath
	}

	if fp, ok := pathSet[testUUID("active-path-001")]; !ok || fp != "/tmp/documents/file1.pdf" {
		t.Errorf("missing or wrong path for doc1: got %q", fp)
	}
	if fp, ok := pathSet[testUUID("active-path-002")]; !ok || fp != "/tmp/documents/file2.pdf" {
		t.Errorf("missing or wrong path for doc2: got %q", fp)
	}

	// Deleted doc and no-path doc should not appear.
	if _, ok := pathSet[testUUID("active-path-003")]; ok {
		t.Error("ListActiveFilePaths() should not include document with empty file path")
	}
	if _, ok := pathSet[testUUID("active-path-004")]; ok {
		t.Error("ListActiveFilePaths() should not include soft-deleted document")
	}
}

func TestDocumentRepository_PurgeSoftDeleted(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	// 1. Create documents: 1 active, 2 soft-deleted (one old, one recent).
	activeDoc := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("purge-active")),
		testutil.WithDocumentTitle("Active Document"),
		testutil.WithDocumentFilePath("/tmp/active.pdf"),
	)
	oldDeletedDoc := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("purge-old-deleted")),
		testutil.WithDocumentTitle("Old Deleted Document"),
		testutil.WithDocumentFilePath("/tmp/old-deleted.pdf"),
	)
	recentDeletedDoc := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("purge-recent-deleted")),
		testutil.WithDocumentTitle("Recent Deleted Document"),
		testutil.WithDocumentFilePath("/tmp/recent-deleted.pdf"),
	)

	for _, doc := range []*model.Document{activeDoc, oldDeletedDoc, recentDeletedDoc} {
		if err := repo.Create(ctx, doc); err != nil {
			t.Fatalf("Create(%s) error: %v", doc.Title, err)
		}
	}

	// Add tags and a version to the old soft-deleted document.
	if err := repo.ReplaceTags(ctx, oldDeletedDoc.ID, []string{"purge-tag-a", "purge-tag-b"}); err != nil {
		t.Fatalf("ReplaceTags() error: %v", err)
	}
	ver := &model.DocumentVersion{
		DocumentID: oldDeletedDoc.ID,
		Version:    1,
		FilePath:   "/tmp/v1/old-deleted.pdf",
		Content:    sql.NullString{String: "version 1 content", Valid: true},
	}
	if err := repo.CreateVersion(ctx, ver); err != nil {
		t.Fatalf("CreateVersion() error: %v", err)
	}

	// Soft-delete both documents.
	if err := repo.SoftDelete(ctx, oldDeletedDoc.ID); err != nil {
		t.Fatalf("SoftDelete(oldDeletedDoc) error: %v", err)
	}
	if err := repo.SoftDelete(ctx, recentDeletedDoc.ID); err != nil {
		t.Fatalf("SoftDelete(recentDeletedDoc) error: %v", err)
	}

	// Backdate the old deleted doc's deleted_at to 48 hours ago.
	_, err := testPool.Exec(ctx,
		`UPDATE documents SET deleted_at = $1 WHERE id = $2`,
		time.Now().Add(-48*time.Hour), oldDeletedDoc.ID)
	if err != nil {
		t.Fatalf("backdating deleted_at: %v", err)
	}

	// Purge documents soft-deleted more than 24 hours ago.
	purged, err := repo.PurgeSoftDeleted(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("PurgeSoftDeleted() error: %v", err)
	}

	// Only the old deleted document should be purged.
	if len(purged) != 1 {
		t.Fatalf("PurgeSoftDeleted() returned %d paths, want 1", len(purged))
	}
	if purged[0].UUID != testUUID("purge-old-deleted") {
		t.Errorf("purged UUID = %q, want %q", purged[0].UUID, testUUID("purge-old-deleted"))
	}
	if purged[0].FilePath != "/tmp/old-deleted.pdf" {
		t.Errorf("purged FilePath = %q, want %q", purged[0].FilePath, "/tmp/old-deleted.pdf")
	}

	// Verify the old deleted document's tags were removed.
	var tagCount int
	err = testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM document_tags WHERE document_id = $1`, oldDeletedDoc.ID).Scan(&tagCount)
	if err != nil {
		t.Fatalf("counting tags: %v", err)
	}
	if tagCount != 0 {
		t.Errorf("tag count for purged doc = %d, want 0", tagCount)
	}

	// Verify the old deleted document's versions were removed.
	var versionCount int
	err = testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM document_versions WHERE document_id = $1`, oldDeletedDoc.ID).Scan(&versionCount)
	if err != nil {
		t.Fatalf("counting versions: %v", err)
	}
	if versionCount != 0 {
		t.Errorf("version count for purged doc = %d, want 0", versionCount)
	}

	// Verify the old deleted document itself is gone.
	var docCount int
	err = testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM documents WHERE id = $1`, oldDeletedDoc.ID).Scan(&docCount)
	if err != nil {
		t.Fatalf("counting purged document: %v", err)
	}
	if docCount != 0 {
		t.Errorf("purged document still exists, want 0")
	}

	// Active document should still exist.
	found, err := repo.FindByUUID(ctx, testUUID("purge-active"))
	if err != nil {
		t.Fatalf("FindByUUID(active) error: %v", err)
	}
	if found.Title != "Active Document" {
		t.Errorf("active doc title = %q, want %q", found.Title, "Active Document")
	}

	// Recently deleted document should still exist (in database, not via FindByUUID since soft-deleted).
	var recentCount int
	err = testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM documents WHERE id = $1`, recentDeletedDoc.ID).Scan(&recentCount)
	if err != nil {
		t.Fatalf("counting recent deleted document: %v", err)
	}
	if recentCount != 1 {
		t.Errorf("recently deleted document count = %d, want 1", recentCount)
	}
}

func TestDocumentRepository_SuggestTitles(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	// Create a mix of public and private documents.
	docs := []struct {
		uuid     string
		title    string
		isPublic bool
	}{
		{testUUID("suggest-001"), "Docker Getting Started", true},
		{testUUID("suggest-002"), "Docker Advanced Topics", true},
		{testUUID("suggest-003"), "Docker Security", false}, // private
		{testUUID("suggest-004"), "Golang Basics", true},
		{testUUID("suggest-005"), "Document Management", true},
	}

	for _, d := range docs {
		doc := testutil.NewDocument(
			testutil.WithDocumentID(0),
			testutil.WithDocumentUUID(d.uuid),
			testutil.WithDocumentTitle(d.title),
			testutil.WithDocumentIsPublic(d.isPublic),
		)
		if err := repo.Create(ctx, doc); err != nil {
			t.Fatalf("Create(%s) error: %v", d.title, err)
		}
	}

	// Soft-delete one public document.
	var deleteID int64
	err := testPool.QueryRow(ctx,
		`SELECT id FROM documents WHERE uuid = $1`, testUUID("suggest-005")).Scan(&deleteID)
	if err != nil {
		t.Fatalf("finding document for soft delete: %v", err)
	}
	if err := repo.SoftDelete(ctx, deleteID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	t.Run("returns public non-deleted matches ordered by title", func(t *testing.T) {
		suggestions, err := repo.SuggestTitles(ctx, "Doc", 5)
		if err != nil {
			t.Fatalf("SuggestTitles() error: %v", err)
		}
		// "Docker Getting Started", "Docker Advanced Topics" match (public, not deleted).
		// "Docker Security" is private.
		// "Document Management" is soft-deleted.
		if len(suggestions) != 2 {
			t.Fatalf("SuggestTitles() returned %d, want 2", len(suggestions))
		}
		// Ordered by title: "Docker Advanced Topics" before "Docker Getting Started".
		if suggestions[0].Title != "Docker Advanced Topics" {
			t.Errorf("first suggestion = %q, want %q", suggestions[0].Title, "Docker Advanced Topics")
		}
		if suggestions[1].Title != "Docker Getting Started" {
			t.Errorf("second suggestion = %q, want %q", suggestions[1].Title, "Docker Getting Started")
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		suggestions, err := repo.SuggestTitles(ctx, "doc", 5)
		if err != nil {
			t.Fatalf("SuggestTitles() error: %v", err)
		}
		// ILIKE should match "Docker..." titles case-insensitively.
		if len(suggestions) != 2 {
			t.Fatalf("SuggestTitles() returned %d, want 2", len(suggestions))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		suggestions, err := repo.SuggestTitles(ctx, "Doc", 1)
		if err != nil {
			t.Fatalf("SuggestTitles() error: %v", err)
		}
		if len(suggestions) != 1 {
			t.Fatalf("SuggestTitles() returned %d, want 1", len(suggestions))
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		suggestions, err := repo.SuggestTitles(ctx, "Kubernetes", 5)
		if err != nil {
			t.Fatalf("SuggestTitles() error: %v", err)
		}
		if len(suggestions) != 0 {
			t.Errorf("SuggestTitles() returned %d, want 0", len(suggestions))
		}
	})

	t.Run("different prefix matches different results", func(t *testing.T) {
		suggestions, err := repo.SuggestTitles(ctx, "Go", 5)
		if err != nil {
			t.Fatalf("SuggestTitles() error: %v", err)
		}
		if len(suggestions) != 1 {
			t.Fatalf("SuggestTitles() returned %d, want 1", len(suggestions))
		}
		if suggestions[0].Title != "Golang Basics" {
			t.Errorf("suggestion = %q, want %q", suggestions[0].Title, "Golang Basics")
		}
	})
}

func TestDocumentRepository_List(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	// Create 4 documents: 3 active, 1 soft-deleted.
	doc1 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-doc-001")),
		testutil.WithDocumentTitle("Alpha Report"),
		testutil.WithDocumentFileType("pdf"),
		testutil.WithDocumentStatus(model.DocumentStatusIndexed),
		testutil.WithDocumentIsPublic(true),
	)
	doc2 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-doc-002")),
		testutil.WithDocumentTitle("Beta Report"),
		testutil.WithDocumentFileType("pdf"),
		testutil.WithDocumentStatus(model.DocumentStatus("processing")),
		testutil.WithDocumentIsPublic(false),
	)
	doc3 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-doc-003")),
		testutil.WithDocumentTitle("Gamma Notes"),
		testutil.WithDocumentFileType("markdown"),
		testutil.WithDocumentStatus(model.DocumentStatusIndexed),
		testutil.WithDocumentIsPublic(true),
	)
	doc4 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-doc-004")),
		testutil.WithDocumentTitle("Deleted Doc"),
		testutil.WithDocumentFileType("pdf"),
		testutil.WithDocumentStatus(model.DocumentStatusIndexed),
	)

	for _, doc := range []*model.Document{doc1, doc2, doc3, doc4} {
		if err := repo.Create(ctx, doc); err != nil {
			t.Fatalf("Create(%s) error: %v", doc.Title, err)
		}
	}

	// Soft-delete doc4 so it is excluded from queries.
	if err := repo.SoftDelete(ctx, doc4.ID); err != nil {
		t.Fatalf("SoftDelete(doc4) error: %v", err)
	}

	t.Run("no filters", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if result.Total != 3 {
			t.Errorf("Total = %d, want 3", result.Total)
		}
		if len(result.Documents) != 3 {
			t.Errorf("len(Documents) = %d, want 3", len(result.Documents))
		}
	})

	t.Run("filter by file_type", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{FileType: "pdf"})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if result.Total != 2 {
			t.Errorf("Total = %d, want 2", result.Total)
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{Status: model.DocumentStatusIndexed})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if result.Total != 2 {
			t.Errorf("Total = %d, want 2", result.Total)
		}
	})

	t.Run("filter by is_public", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{IsPublic: new(true)})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if result.Total != 2 {
			t.Errorf("Total = %d, want 2", result.Total)
		}
	})

	t.Run("filter by user_id", func(t *testing.T) {
		// Create a user and assign doc1 to them.
		oauthRepo := NewOAuthRepository(testPool, discardLogger())
		user := testutil.NewUser(
			testutil.WithUserID(0),
			testutil.WithUserEmail("list-user@example.com"),
		)
		if err := oauthRepo.CreateUser(ctx, user); err != nil {
			t.Fatalf("CreateUser() error: %v", err)
		}
		doc1.UserID = sql.NullInt64{Int64: user.ID, Valid: true}
		if err := repo.Update(ctx, doc1); err != nil {
			t.Fatalf("Update() error: %v", err)
		}

		uid := user.ID
		result, err := repo.List(ctx, DocumentListParams{UserID: &uid})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if result.Total != 1 {
			t.Errorf("Total = %d, want 1", result.Total)
		}
	})

	t.Run("filter by query", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{Query: "Report"})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if result.Total != 2 {
			t.Errorf("Total = %d, want 2", result.Total)
		}
	})

	t.Run("combined filters", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{
			FileType: "pdf",
			Status:   model.DocumentStatusIndexed,
		})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if result.Total != 1 {
			t.Errorf("Total = %d, want 1", result.Total)
		}
	})

	t.Run("pagination limit", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{Limit: 2})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if result.Total != 3 {
			t.Errorf("Total = %d, want 3", result.Total)
		}
		if len(result.Documents) != 2 {
			t.Errorf("len(Documents) = %d, want 2", len(result.Documents))
		}
	})

	t.Run("pagination offset", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{Limit: 2, Offset: 2})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if result.Total != 3 {
			t.Errorf("Total = %d, want 3", result.Total)
		}
		if len(result.Documents) != 1 {
			t.Errorf("len(Documents) = %d, want 1", len(result.Documents))
		}
	})

	t.Run("order by title asc", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{
			OrderBy:  "title",
			OrderDir: "asc",
		})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if len(result.Documents) == 0 {
			t.Fatal("expected at least one document")
		}
		if result.Documents[0].Title != "Alpha Report" {
			t.Errorf("first doc title = %q, want %q", result.Documents[0].Title, "Alpha Report")
		}
	})

	t.Run("order by updated_at asc", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{
			OrderBy:  "updated_at",
			OrderDir: "asc",
		})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if len(result.Documents) == 0 {
			t.Fatal("expected at least one document")
		}
	})

	t.Run("order by title desc", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{
			OrderBy:  "title",
			OrderDir: "desc",
		})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if len(result.Documents) == 0 {
			t.Fatal("expected at least one document")
		}
		if got := result.Documents[0].Title; got != "Gamma Notes" {
			t.Errorf("first doc title = %q, want prefix %q", got, "Gamma Notes")
		}
	})

	t.Run("default limit caps at 20", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{Limit: 0})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if result.Total != 3 {
			t.Errorf("Total = %d, want 3", result.Total)
		}
		if len(result.Documents) != 3 {
			t.Errorf("len(Documents) = %d, want 3", len(result.Documents))
		}
	})
}

func TestDocumentRepository_FindByStatus(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	// Create 4 documents: 2 pending, 1 completed, 1 pending (soft-deleted).
	pending1 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("status-pending-001")),
		testutil.WithDocumentTitle("Pending One"),
		testutil.WithDocumentStatus(model.DocumentStatusPending),
	)
	pending2 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("status-pending-002")),
		testutil.WithDocumentTitle("Pending Two"),
		testutil.WithDocumentStatus(model.DocumentStatusPending),
	)
	completed1 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("status-completed-001")),
		testutil.WithDocumentTitle("Completed One"),
		testutil.WithDocumentStatus(model.DocumentStatusIndexed),
	)
	pendingDeleted := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("status-pending-deleted")),
		testutil.WithDocumentTitle("Pending Deleted"),
		testutil.WithDocumentStatus(model.DocumentStatusPending),
	)

	for _, doc := range []*model.Document{pending1, pending2, completed1, pendingDeleted} {
		if err := repo.Create(ctx, doc); err != nil {
			t.Fatalf("Create(%s) error: %v", doc.Title, err)
		}
	}

	if err := repo.SoftDelete(ctx, pendingDeleted.ID); err != nil {
		t.Fatalf("SoftDelete(pendingDeleted) error: %v", err)
	}

	t.Run("finds pending", func(t *testing.T) {
		docs, err := repo.FindByStatus(ctx, model.DocumentStatusPending, 10)
		if err != nil {
			t.Fatalf("FindByStatus() error: %v", err)
		}
		if len(docs) != 2 {
			t.Errorf("len(docs) = %d, want 2", len(docs))
		}
	})

	t.Run("finds completed", func(t *testing.T) {
		docs, err := repo.FindByStatus(ctx, model.DocumentStatusIndexed, 10)
		if err != nil {
			t.Fatalf("FindByStatus() error: %v", err)
		}
		if len(docs) != 1 {
			t.Errorf("len(docs) = %d, want 1", len(docs))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		docs, err := repo.FindByStatus(ctx, model.DocumentStatusPending, 1)
		if err != nil {
			t.Fatalf("FindByStatus() error: %v", err)
		}
		if len(docs) != 1 {
			t.Errorf("len(docs) = %d, want 1", len(docs))
		}
	})

	t.Run("no matches", func(t *testing.T) {
		docs, err := repo.FindByStatus(ctx, model.DocumentStatus("error"), 10)
		if err != nil {
			t.Fatalf("FindByStatus() error: %v", err)
		}
		if len(docs) != 0 {
			t.Errorf("len(docs) = %d, want 0", len(docs))
		}
	})
}

func TestDocumentRepository_FindByUUIDIncludingDeleted(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	// Create a document and soft-delete it.
	doc := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("incl-deleted-001")),
		testutil.WithDocumentTitle("Soft Deleted Doc"),
	)
	if err := repo.Create(ctx, doc); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if err := repo.SoftDelete(ctx, doc.ID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	t.Run("finds soft-deleted document", func(t *testing.T) {
		found, err := repo.FindByUUIDIncludingDeleted(ctx, testUUID("incl-deleted-001"))
		if err != nil {
			t.Fatalf("FindByUUIDIncludingDeleted() error: %v", err)
		}
		if !found.DeletedAt.Valid {
			t.Error("FindByUUIDIncludingDeleted() DeletedAt.Valid = false, want true")
		}
	})

	t.Run("finds active document", func(t *testing.T) {
		activeDoc := testutil.NewDocument(
			testutil.WithDocumentID(0),
			testutil.WithDocumentUUID(testUUID("incl-deleted-002")),
			testutil.WithDocumentTitle("Active Doc"),
		)
		if err := repo.Create(ctx, activeDoc); err != nil {
			t.Fatalf("Create() error: %v", err)
		}

		found, err := repo.FindByUUIDIncludingDeleted(ctx, testUUID("incl-deleted-002"))
		if err != nil {
			t.Fatalf("FindByUUIDIncludingDeleted() error: %v", err)
		}
		if found.DeletedAt.Valid {
			t.Error("FindByUUIDIncludingDeleted() DeletedAt.Valid = true, want false")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.FindByUUIDIncludingDeleted(ctx, testUUID("incl-deleted-nonexistent"))
		if err == nil {
			t.Fatal("FindByUUIDIncludingDeleted() expected error for nonexistent UUID, got nil")
		}
	})
}

func TestDocumentRepository_Restore(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	// Create a document and soft-delete it.
	doc := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("restore-001")),
		testutil.WithDocumentTitle("Restorable Doc"),
	)
	if err := repo.Create(ctx, doc); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if err := repo.SoftDelete(ctx, doc.ID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	// Confirm FindByUUID fails while soft-deleted.
	_, err := repo.FindByUUID(ctx, testUUID("restore-001"))
	if err == nil {
		t.Fatal("FindByUUID() expected error for soft-deleted doc, got nil")
	}

	// Restore the document.
	if err := repo.Restore(ctx, doc.ID); err != nil {
		t.Fatalf("Restore() error: %v", err)
	}

	// FindByUUID should now succeed.
	found, err := repo.FindByUUID(ctx, testUUID("restore-001"))
	if err != nil {
		t.Fatalf("FindByUUID() after Restore error: %v", err)
	}
	if found.DeletedAt.Valid {
		t.Error("DeletedAt.Valid = true after Restore, want false")
	}

	t.Run("restore already-active doc is no-op", func(t *testing.T) {
		if err := repo.Restore(ctx, doc.ID); err != nil {
			t.Fatalf("Restore() on active doc error: %v", err)
		}

		found, err := repo.FindByUUID(ctx, testUUID("restore-001"))
		if err != nil {
			t.Fatalf("FindByUUID() after second Restore error: %v", err)
		}
		if found.DeletedAt.Valid {
			t.Error("DeletedAt.Valid = true after second Restore, want false")
		}
	})
}

func TestDocumentRepository_PurgeSingle(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	// Create a document with tags and a version.
	doc := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("purge-single-001")),
		testutil.WithDocumentTitle("Purgeable Doc"),
		testutil.WithDocumentFilePath("/tmp/purge-single.pdf"),
	)
	if err := repo.Create(ctx, doc); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := repo.ReplaceTags(ctx, doc.ID, []string{"tag-a", "tag-b"}); err != nil {
		t.Fatalf("ReplaceTags() error: %v", err)
	}

	ver := &model.DocumentVersion{
		DocumentID: doc.ID,
		Version:    1,
		FilePath:   "/tmp/v1/purge-single.pdf",
		Content:    sql.NullString{String: "version 1", Valid: true},
	}
	if err := repo.CreateVersion(ctx, ver); err != nil {
		t.Fatalf("CreateVersion() error: %v", err)
	}

	// Soft-delete before purging.
	if err := repo.SoftDelete(ctx, doc.ID); err != nil {
		t.Fatalf("SoftDelete() error: %v", err)
	}

	// Purge the document.
	filePath, err := repo.PurgeSingle(ctx, doc.ID)
	if err != nil {
		t.Fatalf("PurgeSingle() error: %v", err)
	}
	if filePath != "/tmp/purge-single.pdf" {
		t.Errorf("PurgeSingle() filePath = %q, want %q", filePath, "/tmp/purge-single.pdf")
	}

	// Verify document is gone.
	var docCount int
	err = testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM documents WHERE id = $1`, doc.ID).Scan(&docCount)
	if err != nil {
		t.Fatalf("counting purged document: %v", err)
	}
	if docCount != 0 {
		t.Errorf("document count after purge = %d, want 0", docCount)
	}

	// Verify tags are gone.
	var tagCount int
	err = testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM document_tags WHERE document_id = $1`, doc.ID).Scan(&tagCount)
	if err != nil {
		t.Fatalf("counting purged tags: %v", err)
	}
	if tagCount != 0 {
		t.Errorf("tag count after purge = %d, want 0", tagCount)
	}

	// Verify versions are gone.
	var versionCount int
	err = testPool.QueryRow(ctx,
		`SELECT COUNT(*) FROM document_versions WHERE document_id = $1`, doc.ID).Scan(&versionCount)
	if err != nil {
		t.Fatalf("counting purged versions: %v", err)
	}
	if versionCount != 0 {
		t.Errorf("version count after purge = %d, want 0", versionCount)
	}
}

func TestDocumentRepository_ListDeleted(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, discardLogger())

	// Create 4 docs: 2 active, 2 soft-deleted.
	active1 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-del-active-001")),
		testutil.WithDocumentTitle("Active One"),
	)
	active2 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-del-active-002")),
		testutil.WithDocumentTitle("Active Two"),
	)
	deleted1 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-del-deleted-001")),
		testutil.WithDocumentTitle("Deleted One"),
	)
	deleted2 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-del-deleted-002")),
		testutil.WithDocumentTitle("Deleted Two"),
	)

	for _, doc := range []*model.Document{active1, active2, deleted1, deleted2} {
		if err := repo.Create(ctx, doc); err != nil {
			t.Fatalf("Create(%s) error: %v", doc.Title, err)
		}
	}

	if err := repo.SoftDelete(ctx, deleted1.ID); err != nil {
		t.Fatalf("SoftDelete(deleted1) error: %v", err)
	}
	if err := repo.SoftDelete(ctx, deleted2.ID); err != nil {
		t.Fatalf("SoftDelete(deleted2) error: %v", err)
	}

	t.Run("returns only soft-deleted", func(t *testing.T) {
		docs, total, err := repo.ListDeleted(ctx, 10, 0, nil)
		if err != nil {
			t.Fatalf("ListDeleted() error: %v", err)
		}
		if len(docs) != 2 {
			t.Errorf("ListDeleted() len = %d, want 2", len(docs))
		}
		if total != 2 {
			t.Errorf("ListDeleted() total = %d, want 2", total)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		docs, total, err := repo.ListDeleted(ctx, 1, 0, nil)
		if err != nil {
			t.Fatalf("ListDeleted(1,0) error: %v", err)
		}
		if len(docs) != 1 {
			t.Errorf("ListDeleted(1,0) len = %d, want 1", len(docs))
		}
		if total != 2 {
			t.Errorf("ListDeleted(1,0) total = %d, want 2", total)
		}

		docs2, _, err := repo.ListDeleted(ctx, 1, 1, nil)
		if err != nil {
			t.Fatalf("ListDeleted(1,1) error: %v", err)
		}
		if len(docs2) != 1 {
			t.Errorf("ListDeleted(1,1) len = %d, want 1", len(docs2))
		}
	})

	t.Run("empty when no deleted", func(t *testing.T) {
		truncateAll(t)

		for _, doc := range []*model.Document{
			testutil.NewDocument(
				testutil.WithDocumentID(0),
				testutil.WithDocumentUUID(testUUID("list-del-none-001")),
				testutil.WithDocumentTitle("Still Active One"),
			),
			testutil.NewDocument(
				testutil.WithDocumentID(0),
				testutil.WithDocumentUUID(testUUID("list-del-none-002")),
				testutil.WithDocumentTitle("Still Active Two"),
			),
		} {
			if err := repo.Create(ctx, doc); err != nil {
				t.Fatalf("Create(%s) error: %v", doc.Title, err)
			}
		}

		docs, total, err := repo.ListDeleted(ctx, 10, 0, nil)
		if err != nil {
			t.Fatalf("ListDeleted() error: %v", err)
		}
		if len(docs) != 0 {
			t.Errorf("ListDeleted() len = %d, want 0", len(docs))
		}
		if total != 0 {
			t.Errorf("ListDeleted() total = %d, want 0", total)
		}
	})
}
