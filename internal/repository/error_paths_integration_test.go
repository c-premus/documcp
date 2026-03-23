//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"git.999.haus/chris/DocuMCP-go/internal/model"
)

// TestErrorPaths_CanceledContext verifies that every repository method
// returns a wrapped error when the context is canceled. This exercises the
// error-return branches that are unreachable with a healthy database.
func TestErrorPaths_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	logger := discardLogger()

	t.Run("DocumentRepository", func(t *testing.T) {
		repo := NewDocumentRepository(testDB, logger)

		doc := &model.Document{UUID: testUUID("err-doc-001"), Title: "Err", FileType: "pdf", FilePath: "/tmp/x", MIMEType: "application/pdf", Status: "pending"}
		assert.Error(t, repo.Create(ctx, doc))
		assert.Error(t, repo.Update(ctx, doc))
		assert.Error(t, repo.SoftDelete(ctx, 1))

		_, err := repo.FindByUUID(ctx, "x")
		assert.Error(t, err)
		_, err = repo.FindByID(ctx, 1)
		assert.Error(t, err)
		_, err = repo.Count(ctx)
		assert.Error(t, err)
		_, err = repo.TagsForDocument(ctx, 1)
		assert.Error(t, err)

		assert.Error(t, repo.ReplaceTags(ctx, 1, []string{"a"}))
		assert.Error(t, repo.CreateVersion(ctx, &model.DocumentVersion{DocumentID: 1, Version: 1, FilePath: "/tmp/x"}))

		_, err = repo.List(ctx, DocumentListParams{})
		assert.Error(t, err)
		_, err = repo.FindByStatus(ctx, "pending", 10)
		assert.Error(t, err)
	})

	t.Run("ExternalServiceRepository", func(t *testing.T) {
		repo := NewExternalServiceRepository(testDB, logger)

		svc := &model.ExternalService{UUID: testUUID("err-svc-001"), Name: "Err", Slug: "err", Type: "kiwix", BaseURL: "https://x.com", Status: "unknown"}
		assert.Error(t, repo.Create(ctx, svc))
		assert.Error(t, repo.Update(ctx, svc))
		assert.Error(t, repo.Delete(ctx, 1))
		assert.Error(t, repo.UpdateHealthStatus(ctx, 1, "healthy", 0, ""))

		_, err := repo.FindByUUID(ctx, "x")
		assert.Error(t, err)
		_, err = repo.FindBySlug(ctx, "x")
		assert.Error(t, err)
		_, err = repo.FindEnabledByType(ctx, "kiwix")
		assert.Error(t, err)
		_, _, err = repo.List(ctx, "", "", 10, 0)
		assert.Error(t, err)
	})

	t.Run("SearchQueryRepository", func(t *testing.T) {
		repo := NewSearchQueryRepository(testDB, logger)

		sq := &model.SearchQuery{Query: "test", ResultsCount: 1}
		assert.Error(t, repo.Create(ctx, sq))
	})

	t.Run("ZimArchiveRepository", func(t *testing.T) {
		repo := NewZimArchiveRepository(testDB, logger)

		assert.Error(t, repo.UpsertFromCatalog(ctx, 1, ZimArchiveUpsert{Name: "X", Title: "X"}))
		assert.Error(t, repo.ToggleEnabled(ctx, 1))
		assert.Error(t, repo.ToggleSearchable(ctx, 1))

		_, err := repo.FindByName(ctx, "x")
		assert.Error(t, err)
		_, err = repo.FindByUUID(ctx, "x")
		assert.Error(t, err)
		_, err = repo.List(ctx, "", "", "", 10, 0)
		assert.Error(t, err)
		_, err = repo.ListAll(ctx, "", 10)
		assert.Error(t, err)
		_, err = repo.Count(ctx)
		assert.Error(t, err)
		_, err = repo.DisableOrphaned(ctx, 1, nil)
		assert.Error(t, err)
		_, err = repo.DisableOrphaned(ctx, 1, []string{"a"})
		assert.Error(t, err)
	})

	t.Run("GitTemplateRepository", func(t *testing.T) {
		repo := NewGitTemplateRepository(testDB, logger, nil)

		tmpl := &model.GitTemplate{UUID: testUUID("err-tmpl-001"), Name: "Err", Slug: "err", RepositoryURL: "https://x.com", Branch: "main", Status: "pending"}
		assert.Error(t, repo.Create(ctx, tmpl))
		assert.Error(t, repo.Update(ctx, tmpl))
		assert.Error(t, repo.SoftDelete(ctx, 1))
		assert.Error(t, repo.UpdateSyncStatus(ctx, 1, "synced", "abc", 1, 100, ""))
		assert.Error(t, repo.ReplaceFiles(ctx, 1, []GitTemplateFileInsert{{Path: "a", Filename: "a"}}))

		_, err := repo.FindByUUID(ctx, "x")
		assert.Error(t, err)
		_, err = repo.FindBySlug(ctx, "x")
		assert.Error(t, err)
		_, err = repo.List(ctx, "", 10, 0)
		assert.Error(t, err)
		_, err = repo.ListAll(ctx, "", 10)
		assert.Error(t, err)
		_, err = repo.Count(ctx)
		assert.Error(t, err)
		_, err = repo.FilesForTemplate(ctx, 1)
		assert.Error(t, err)
		_, err = repo.FindFileByPath(ctx, 1, "x")
		assert.Error(t, err)
		_, err = repo.Search(ctx, "x", "", 10)
		assert.Error(t, err)
	})

	t.Run("OAuthRepository", func(t *testing.T) {
		repo := NewOAuthRepository(testDB, logger)

		assert.Error(t, repo.CreateClient(ctx, &model.OAuthClient{ClientID: "x", ClientName: "x", RedirectURIs: "[]", GrantTypes: "[]", ResponseTypes: "[]", TokenEndpointAuthMethod: "none"}))
		assert.Error(t, repo.CreateUser(ctx, &model.User{Name: "x", Email: "x@x.com"}))
		assert.Error(t, repo.UpdateUser(ctx, &model.User{ID: 1}))
		assert.Error(t, repo.ToggleAdmin(ctx, 1))
		assert.Error(t, repo.DeactivateClient(ctx, 1))
		assert.Error(t, repo.RevokeAuthorizationCode(ctx, 1))
		assert.Error(t, repo.RevokeAccessToken(ctx, 1))
		assert.Error(t, repo.RevokeRefreshToken(ctx, 1))
		assert.Error(t, repo.RevokeRefreshTokenByAccessTokenID(ctx, 1))
		assert.Error(t, repo.CreateAccessToken(ctx, &model.OAuthAccessToken{Token: "x", ClientID: 1}))
		assert.Error(t, repo.CreateRefreshToken(ctx, &model.OAuthRefreshToken{Token: "x", AccessTokenID: 1}))
		assert.Error(t, repo.CreateAuthorizationCode(ctx, &model.OAuthAuthorizationCode{Code: "x", ClientID: 1, RedirectURI: "http://x"}))
		assert.Error(t, repo.CreateDeviceCode(ctx, &model.OAuthDeviceCode{DeviceCode: "x", UserCode: "X", ClientID: 1, VerificationURI: "http://x", Status: "pending"}))
		assert.Error(t, repo.UpdateDeviceCodeStatus(ctx, 1, "denied", nil))
		assert.Error(t, repo.UpdateDeviceCodeLastPolled(ctx, 1, 5))

		_, err := repo.FindClientByClientID(ctx, "x")
		assert.Error(t, err)
		_, err = repo.FindClientByID(ctx, 1)
		assert.Error(t, err)
		_, err = repo.FindAccessTokenByToken(ctx, "x")
		assert.Error(t, err)
		_, err = repo.FindAccessTokenByID(ctx, 1)
		assert.Error(t, err)
		_, err = repo.FindRefreshTokenByToken(ctx, "x")
		assert.Error(t, err)
		_, err = repo.FindAuthorizationCodeByCode(ctx, "x")
		assert.Error(t, err)
		_, err = repo.FindDeviceCodeByDeviceCode(ctx, "x")
		assert.Error(t, err)
		_, err = repo.FindDeviceCodeByUserCode(ctx, "x")
		assert.Error(t, err)
		_, err = repo.FindUserByID(ctx, 1)
		assert.Error(t, err)
		_, err = repo.FindUserByEmail(ctx, "x")
		assert.Error(t, err)
		_, err = repo.FindUserByOIDCSub(ctx, "x")
		assert.Error(t, err)
		_, _, err = repo.ListUsers(ctx, "", 10, 0)
		assert.Error(t, err)
		_, _, err = repo.ListClients(ctx, "", 10, 0)
		assert.Error(t, err)
		_, err = repo.CountUsers(ctx)
		assert.Error(t, err)
		_, err = repo.CountClients(ctx)
		assert.Error(t, err)
	})
}
