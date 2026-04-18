//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c-premus/documcp/internal/testutil"
)

// TestDocumentRepository_MetadataContributesToSearchVector is a regression
// guard for migration 000019, which extends the STORED
// documents.search_vector generated column with JSONB-path extraction of
// metadata.title / creator / description and metadata.subjects (as a
// JSONB array, via the jsonb_typeof + ::text pattern inherited from
// migration 000016).
//
// Each subtest seeds a document whose chapter/body content does NOT
// contain the query token — the match must come from the metadata JSONB
// column alone, proving the generated column really indexes it.
func TestDocumentRepository_MetadataContributesToSearchVector(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	repo := NewDocumentRepository(testPool, testutil.DiscardLogger())

	metadata := map[string]any{
		"title":       "Pride and Prejudice",
		"creator":     "Jane Austen",
		"subjects":    []string{"classic", "romance"},
		"description": "A novel of manners",
	}
	metaJSON, err := json.Marshal(metadata)
	require.NoError(t, err, "marshaling metadata fixture")

	doc := testutil.NewDocument(
		testutil.WithDocumentID(0),
		testutil.WithDocumentUUID(testUUID("fts-metadata-jsonb")),
		testutil.WithDocumentTitle("Unrelated Body Title"),
		testutil.WithDocumentFileType("epub"),
		testutil.WithDocumentContent("chapter one had nothing special to note"),
	)
	doc.Metadata = metaJSON

	require.NoError(t, repo.Create(ctx, doc), "creating document with JSONB metadata")

	tests := []struct {
		name  string
		token string
	}{
		{
			// metadata ->> 'creator' path
			name:  "creator token from metadata",
			token: "Austen",
		},
		{
			// jsonb_typeof(metadata -> 'subjects') = 'array' path
			name:  "subject token from metadata.subjects JSONB array",
			token: "romance",
		},
		{
			// metadata ->> 'description' path
			name:  "description token from metadata",
			token: "manners",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hitUUID string
			err := testPool.QueryRow(ctx,
				`SELECT uuid FROM documents
				 WHERE search_vector @@ websearch_to_tsquery('documcp_english', $1)`,
				tt.token,
			).Scan(&hitUUID)
			require.NoError(t, err, "metadata-only FTS query should return the seeded row for %q", tt.token)
			assert.Equal(t, doc.UUID, hitUUID)
		})
	}
}
