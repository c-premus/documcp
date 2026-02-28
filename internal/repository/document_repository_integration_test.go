//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/testutil"
)

func TestDocumentRepository_CreateAndFind(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testDB, discardLogger())

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
	repo := NewDocumentRepository(testDB, discardLogger())

	tests := []struct {
		name      string
		uuid      string
		newTitle  string
		newStatus string
	}{
		{
			name:      "update title and status",
			uuid:      testUUID("update-title-status"),
			newTitle:  "Updated Title",
			newStatus: "processing",
		},
		{
			name:      "update to error status",
			uuid:      testUUID("update-error-status"),
			newTitle:  "Error Doc",
			newStatus: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := testutil.NewDocument(
				testutil.WithDocumentID(0),
				testutil.WithDocumentUUID(tt.uuid),
				testutil.WithDocumentTitle("Original Title"),
				testutil.WithDocumentStatus("completed"),
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
	repo := NewDocumentRepository(testDB, discardLogger())

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
	repo := NewDocumentRepository(testDB, discardLogger())

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
	repo := NewDocumentRepository(testDB, discardLogger())

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
	repo := NewDocumentRepository(testDB, discardLogger())

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
			var found model.DocumentVersion
			err := testDB.GetContext(ctx, &found,
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

func boolPtr(b bool) *bool { return &b }

func TestDocumentRepository_List(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testDB, discardLogger())

	// Create 4 documents: 3 active, 1 soft-deleted.
	doc1 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-doc-001")),
		testutil.WithDocumentTitle("Alpha Report"),
		testutil.WithDocumentFileType("pdf"),
		testutil.WithDocumentStatus("completed"),
		testutil.WithDocumentIsPublic(true),
	)
	doc2 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-doc-002")),
		testutil.WithDocumentTitle("Beta Report"),
		testutil.WithDocumentFileType("pdf"),
		testutil.WithDocumentStatus("processing"),
		testutil.WithDocumentIsPublic(false),
	)
	doc3 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-doc-003")),
		testutil.WithDocumentTitle("Gamma Notes"),
		testutil.WithDocumentFileType("markdown"),
		testutil.WithDocumentStatus("completed"),
		testutil.WithDocumentIsPublic(true),
	)
	doc4 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("list-doc-004")),
		testutil.WithDocumentTitle("Deleted Doc"),
		testutil.WithDocumentFileType("pdf"),
		testutil.WithDocumentStatus("completed"),
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
		result, err := repo.List(ctx, DocumentListParams{Status: "completed"})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if result.Total != 2 {
			t.Errorf("Total = %d, want 2", result.Total)
		}
	})

	t.Run("filter by is_public", func(t *testing.T) {
		result, err := repo.List(ctx, DocumentListParams{IsPublic: boolPtr(true)})
		if err != nil {
			t.Fatalf("List() error: %v", err)
		}
		if result.Total != 2 {
			t.Errorf("Total = %d, want 2", result.Total)
		}
	})

	t.Run("filter by user_id", func(t *testing.T) {
		// Create a user and assign doc1 to them.
		oauthRepo := NewOAuthRepository(testDB, discardLogger())
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
			Status:   "completed",
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
	repo := NewDocumentRepository(testDB, discardLogger())

	// Create 4 documents: 2 pending, 1 completed, 1 pending (soft-deleted).
	pending1 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("status-pending-001")),
		testutil.WithDocumentTitle("Pending One"),
		testutil.WithDocumentStatus("pending"),
	)
	pending2 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("status-pending-002")),
		testutil.WithDocumentTitle("Pending Two"),
		testutil.WithDocumentStatus("pending"),
	)
	completed1 := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("status-completed-001")),
		testutil.WithDocumentTitle("Completed One"),
		testutil.WithDocumentStatus("completed"),
	)
	pendingDeleted := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("status-pending-deleted")),
		testutil.WithDocumentTitle("Pending Deleted"),
		testutil.WithDocumentStatus("pending"),
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
		docs, err := repo.FindByStatus(ctx, "pending", 10)
		if err != nil {
			t.Fatalf("FindByStatus() error: %v", err)
		}
		if len(docs) != 2 {
			t.Errorf("len(docs) = %d, want 2", len(docs))
		}
	})

	t.Run("finds completed", func(t *testing.T) {
		docs, err := repo.FindByStatus(ctx, "completed", 10)
		if err != nil {
			t.Fatalf("FindByStatus() error: %v", err)
		}
		if len(docs) != 1 {
			t.Errorf("len(docs) = %d, want 1", len(docs))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		docs, err := repo.FindByStatus(ctx, "pending", 1)
		if err != nil {
			t.Fatalf("FindByStatus() error: %v", err)
		}
		if len(docs) != 1 {
			t.Errorf("len(docs) = %d, want 1", len(docs))
		}
	})

	t.Run("no matches", func(t *testing.T) {
		docs, err := repo.FindByStatus(ctx, "error", 10)
		if err != nil {
			t.Fatalf("FindByStatus() error: %v", err)
		}
		if len(docs) != 0 {
			t.Errorf("len(docs) = %d, want 0", len(docs))
		}
	})
}
