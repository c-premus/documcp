// Package admin provides HTTP handlers for the templ+htmx admin UI.
package admin

import (
	"context"

	"git.999.haus/chris/DocuMCP-go/internal/model"
	"git.999.haus/chris/DocuMCP-go/internal/repository"
	"git.999.haus/chris/DocuMCP-go/internal/service"
)

// DocumentRepo defines the document repository methods used by the admin handler.
type DocumentRepo interface {
	Count(ctx context.Context) (int, error)
	List(ctx context.Context, params repository.DocumentListParams) (*repository.DocumentListResult, error)
	FindByUUID(ctx context.Context, uuid string) (*model.Document, error)
	TagsForDocument(ctx context.Context, documentID int64) ([]model.DocumentTag, error)
	SoftDelete(ctx context.Context, id int64) error
}

// OAuthRepo defines the OAuth repository methods used by the admin handler.
type OAuthRepo interface {
	CountUsers(ctx context.Context) (int, error)
	CountClients(ctx context.Context) (int, error)
	ListUsers(ctx context.Context, query string, limit, offset int) ([]model.User, int, error)
	FindUserByID(ctx context.Context, id int64) (*model.User, error)
	ToggleAdmin(ctx context.Context, userID int64) error
	ListClients(ctx context.Context, query string, limit, offset int) ([]model.OAuthClient, int, error)
	FindClientByID(ctx context.Context, id int64) (*model.OAuthClient, error)
	CreateClient(ctx context.Context, client *model.OAuthClient) error
	DeactivateClient(ctx context.Context, clientID int64) error
}

// ExternalServiceRepo defines the external service repository methods used by the admin handler.
type ExternalServiceRepo interface {
	List(ctx context.Context, serviceType, status string, limit, offset int) ([]model.ExternalService, int, error)
	FindByUUID(ctx context.Context, uuid string) (*model.ExternalService, error)
	Create(ctx context.Context, svc *model.ExternalService) error
	Delete(ctx context.Context, id int64) error
}

// ZimArchiveRepo defines the ZIM archive repository methods used by the admin handler.
type ZimArchiveRepo interface {
	Count(ctx context.Context) (int, error)
	ListAll(ctx context.Context, query string, limit int) ([]model.ZimArchive, error)
	FindByUUID(ctx context.Context, uuid string) (*model.ZimArchive, error)
	ToggleEnabled(ctx context.Context, id int64) error
	ToggleSearchable(ctx context.Context, id int64) error
}

// ConfluenceSpaceRepo defines the Confluence space repository methods used by the admin handler.
type ConfluenceSpaceRepo interface {
	Count(ctx context.Context) (int, error)
	ListAll(ctx context.Context, query string, limit int) ([]model.ConfluenceSpace, error)
	FindByUUID(ctx context.Context, uuid string) (*model.ConfluenceSpace, error)
	ToggleEnabled(ctx context.Context, id int64) error
	ToggleSearchable(ctx context.Context, id int64) error
}

// GitTemplateRepo defines the git template repository methods used by the admin handler.
type GitTemplateRepo interface {
	Count(ctx context.Context) (int, error)
	ListAll(ctx context.Context, query string, limit int) ([]model.GitTemplate, error)
	FindByUUID(ctx context.Context, uuid string) (*model.GitTemplate, error)
	FilesForTemplate(ctx context.Context, templateID int64) ([]model.GitTemplateFile, error)
	Create(ctx context.Context, tmpl *model.GitTemplate) error
	SoftDelete(ctx context.Context, id int64) error
}

// DocumentUploader defines the document pipeline methods used by the admin handler.
type DocumentUploader interface {
	Upload(ctx context.Context, params service.UploadDocumentParams) (*model.Document, error)
}

// ExternalServiceHealthChecker defines the external service health check methods
// used by the admin handler.
type ExternalServiceHealthChecker interface {
	CheckHealth(ctx context.Context, svcUUID string, checker service.HealthChecker) (*model.ExternalService, error)
}
