// Package testutil provides builder functions that create model instances
// with sensible defaults for use in tests. Each builder accepts functional
// options to override individual fields.
package testutil

import (
	"database/sql"
	"time"

	"github.com/c-premus/documcp/internal/model"
)

// nullString returns a valid sql.NullString.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

// nullInt64 returns a valid sql.NullInt64.
func nullInt64(n int64) sql.NullInt64 {
	return sql.NullInt64{Int64: n, Valid: true}
}

// nullTime returns a valid sql.NullTime for the given time.
func nullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}

//nolint:godot // ---------------------------------------------------------------------------
// Document.
//nolint:godot // ---------------------------------------------------------------------------

// DocumentOption configures a Document created by NewDocument.
type DocumentOption func(*model.Document)

// NewDocument returns a Document with sensible defaults. Pass DocumentOption
// functions to override specific fields.
func NewDocument(opts ...DocumentOption) *model.Document {
	now := nullTime(time.Now())
	d := &model.Document{
		ID:        1,
		UUID:      "test-doc-uuid",
		Title:     "Test Document",
		FileType:  "pdf",
		FilePath:  "/tmp/test-document.pdf",
		FileSize:  1024,
		MIMEType:  "application/pdf",
		IsPublic:  false,
		Status:    "completed",
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// WithDocumentID sets the document ID on the builder.
func WithDocumentID(id int64) DocumentOption {
	return func(d *model.Document) { d.ID = id }
}

// WithDocumentUUID sets the document UUID on the builder.
func WithDocumentUUID(uuid string) DocumentOption {
	return func(d *model.Document) { d.UUID = uuid }
}

// WithDocumentTitle sets the document title on the builder.
func WithDocumentTitle(title string) DocumentOption {
	return func(d *model.Document) { d.Title = title }
}

// WithDocumentDescription sets the document description on the builder.
func WithDocumentDescription(desc string) DocumentOption {
	return func(d *model.Document) { d.Description = nullString(desc) }
}

// WithDocumentFileType sets the document file type on the builder.
func WithDocumentFileType(ft string) DocumentOption {
	return func(d *model.Document) { d.FileType = ft }
}

// WithDocumentFilePath sets the document file path on the builder.
func WithDocumentFilePath(fp string) DocumentOption {
	return func(d *model.Document) { d.FilePath = fp }
}

// WithDocumentFileSize sets the document file size on the builder.
func WithDocumentFileSize(size int64) DocumentOption {
	return func(d *model.Document) { d.FileSize = size }
}

// WithDocumentMIMEType sets the document MIME type on the builder.
func WithDocumentMIMEType(mime string) DocumentOption {
	return func(d *model.Document) { d.MIMEType = mime }
}

// WithDocumentContent sets the document content on the builder.
func WithDocumentContent(content string) DocumentOption {
	return func(d *model.Document) { d.Content = nullString(content) }
}

// WithDocumentUserID sets the document user ID on the builder.
func WithDocumentUserID(uid int64) DocumentOption {
	return func(d *model.Document) { d.UserID = nullInt64(uid) }
}

// WithDocumentIsPublic sets the document public visibility on the builder.
func WithDocumentIsPublic(public bool) DocumentOption {
	return func(d *model.Document) { d.IsPublic = public }
}

// WithDocumentStatus sets the document status on the builder.
func WithDocumentStatus(status string) DocumentOption {
	return func(d *model.Document) { d.Status = status }
}

//nolint:godot // ---------------------------------------------------------------------------
// User.
//nolint:godot // ---------------------------------------------------------------------------

// UserOption configures a User created by NewUser.
type UserOption func(*model.User)

// NewUser returns a User with sensible defaults.
func NewUser(opts ...UserOption) *model.User {
	now := nullTime(time.Now())
	u := &model.User{
		ID:        1,
		Name:      "Test User",
		Email:     "test@example.com",
		IsAdmin:   false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// WithUserID sets the user ID on the builder.
func WithUserID(id int64) UserOption {
	return func(u *model.User) { u.ID = id }
}

// WithUserName sets the user name on the builder.
func WithUserName(name string) UserOption {
	return func(u *model.User) { u.Name = name }
}

// WithUserEmail sets the user email on the builder.
func WithUserEmail(email string) UserOption {
	return func(u *model.User) { u.Email = email }
}

// WithUserIsAdmin sets the user admin flag on the builder.
func WithUserIsAdmin(admin bool) UserOption {
	return func(u *model.User) { u.IsAdmin = admin }
}

// WithUserOIDCSub sets the user OIDC subject on the builder.
func WithUserOIDCSub(sub string) UserOption {
	return func(u *model.User) { u.OIDCSub = nullString(sub) }
}

// WithUserOIDCProvider sets the user OIDC provider on the builder.
func WithUserOIDCProvider(provider string) UserOption {
	return func(u *model.User) { u.OIDCProvider = nullString(provider) }
}

// WithUserPassword sets the user password on the builder.
func WithUserPassword(pw string) UserOption {
	return func(u *model.User) { u.Password = nullString(pw) }
}

//nolint:godot // ---------------------------------------------------------------------------
// OAuthClient.
//nolint:godot // ---------------------------------------------------------------------------

// OAuthClientOption configures an OAuthClient created by NewOAuthClient.
type OAuthClientOption func(*model.OAuthClient)

// NewOAuthClient returns an OAuthClient with sensible defaults.
func NewOAuthClient(opts ...OAuthClientOption) *model.OAuthClient {
	now := nullTime(time.Now())
	c := &model.OAuthClient{
		ID:                      1,
		ClientID:                "test-client-id",
		ClientName:              "Test Client",
		RedirectURIs:            `["http://localhost:8080/callback"]`,
		GrantTypes:              `["authorization_code"]`,
		ResponseTypes:           `["code"]`,
		TokenEndpointAuthMethod: "client_secret_basic",
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithOAuthClientID sets the OAuth client primary key ID on the builder.
func WithOAuthClientID(id int64) OAuthClientOption {
	return func(c *model.OAuthClient) { c.ID = id }
}

// WithOAuthClientClientID sets the OAuth client identifier on the builder.
func WithOAuthClientClientID(clientID string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.ClientID = clientID }
}

// WithOAuthClientName sets the OAuth client name on the builder.
func WithOAuthClientName(name string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.ClientName = name }
}

// WithOAuthClientSecret sets the OAuth client secret on the builder.
func WithOAuthClientSecret(secret string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.ClientSecret = nullString(secret) }
}

// WithOAuthClientRedirectURIs sets the OAuth client redirect URIs on the builder.
func WithOAuthClientRedirectURIs(uris string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.RedirectURIs = uris }
}

// WithOAuthClientGrantTypes sets the OAuth client grant types on the builder.
func WithOAuthClientGrantTypes(types string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.GrantTypes = types }
}

// WithOAuthClientScope sets the OAuth client scope on the builder.
func WithOAuthClientScope(scope string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.Scope = nullString(scope) }
}

// WithOAuthClientUserID sets the OAuth client user ID on the builder.
func WithOAuthClientUserID(uid int64) OAuthClientOption {
	return func(c *model.OAuthClient) { c.UserID = nullInt64(uid) }
}

//nolint:godot // ---------------------------------------------------------------------------
// ExternalService.
//nolint:godot // ---------------------------------------------------------------------------

// ExternalServiceOption configures an ExternalService created by NewExternalService.
type ExternalServiceOption func(*model.ExternalService)

// NewExternalService returns an ExternalService with sensible defaults.
func NewExternalService(opts ...ExternalServiceOption) *model.ExternalService {
	now := nullTime(time.Now())
	es := &model.ExternalService{
		ID:        1,
		UUID:      "test-extservice-uuid",
		Name:      "Test Service",
		Slug:      "test-service",
		Type:      "kiwix",
		BaseURL:   "https://example.com",
		Priority:  100,
		Status:    "active",
		IsEnabled: true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, opt := range opts {
		opt(es)
	}
	return es
}

// WithExternalServiceID sets the external service ID on the builder.
func WithExternalServiceID(id int64) ExternalServiceOption {
	return func(es *model.ExternalService) { es.ID = id }
}

// WithExternalServiceUUID sets the external service UUID on the builder.
func WithExternalServiceUUID(uuid string) ExternalServiceOption {
	return func(es *model.ExternalService) { es.UUID = uuid }
}

// WithExternalServiceName sets the external service name on the builder.
func WithExternalServiceName(name string) ExternalServiceOption {
	return func(es *model.ExternalService) { es.Name = name }
}

// WithExternalServiceSlug sets the external service slug on the builder.
func WithExternalServiceSlug(slug string) ExternalServiceOption {
	return func(es *model.ExternalService) { es.Slug = slug }
}

// WithExternalServiceType sets the external service type on the builder.
func WithExternalServiceType(t string) ExternalServiceOption {
	return func(es *model.ExternalService) { es.Type = t }
}

// WithExternalServiceBaseURL sets the external service base URL on the builder.
func WithExternalServiceBaseURL(url string) ExternalServiceOption {
	return func(es *model.ExternalService) { es.BaseURL = url }
}

// WithExternalServiceStatus sets the external service status on the builder.
func WithExternalServiceStatus(status string) ExternalServiceOption {
	return func(es *model.ExternalService) { es.Status = status }
}

// WithExternalServiceIsEnabled sets the external service enabled flag on the builder.
func WithExternalServiceIsEnabled(enabled bool) ExternalServiceOption {
	return func(es *model.ExternalService) { es.IsEnabled = enabled }
}

// WithExternalServicePriority sets the external service priority on the builder.
func WithExternalServicePriority(p int) ExternalServiceOption {
	return func(es *model.ExternalService) { es.Priority = p }
}

//nolint:godot // ---------------------------------------------------------------------------
// ZimArchive.
//nolint:godot // ---------------------------------------------------------------------------

// ZimArchiveOption configures a ZimArchive created by NewZimArchive.
type ZimArchiveOption func(*model.ZimArchive)

// NewZimArchive returns a ZimArchive with sensible defaults.
func NewZimArchive(opts ...ZimArchiveOption) *model.ZimArchive {
	now := nullTime(time.Now())
	za := &model.ZimArchive{
		ID:           1,
		UUID:         "test-zim-uuid",
		Name:         "Test ZIM Archive",
		Slug:         "test-zim-archive",
		Title:        "Test ZIM",
		Language:     "en",
		ArticleCount: 100,
		MediaCount:   10,
		FileSize:     1048576,
		IsEnabled:    true,
		IsSearchable: true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	for _, opt := range opts {
		opt(za)
	}
	return za
}

// WithZimArchiveID sets the ZIM archive ID on the builder.
func WithZimArchiveID(id int64) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.ID = id }
}

// WithZimArchiveUUID sets the ZIM archive UUID on the builder.
func WithZimArchiveUUID(uuid string) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.UUID = uuid }
}

// WithZimArchiveName sets the ZIM archive name on the builder.
func WithZimArchiveName(name string) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.Name = name }
}

// WithZimArchiveSlug sets the ZIM archive slug on the builder.
func WithZimArchiveSlug(slug string) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.Slug = slug }
}

// WithZimArchiveTitle sets the ZIM archive title on the builder.
func WithZimArchiveTitle(title string) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.Title = title }
}

// WithZimArchiveLanguage sets the ZIM archive language on the builder.
func WithZimArchiveLanguage(lang string) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.Language = lang }
}

// WithZimArchiveExternalServiceID sets the ZIM archive external service ID on the builder.
func WithZimArchiveExternalServiceID(id int64) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.ExternalServiceID = nullInt64(id) }
}

// WithZimArchiveIsEnabled sets the ZIM archive enabled flag on the builder.
func WithZimArchiveIsEnabled(enabled bool) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.IsEnabled = enabled }
}

// WithZimArchiveIsSearchable sets the ZIM archive searchable flag on the builder.
func WithZimArchiveIsSearchable(searchable bool) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.IsSearchable = searchable }
}

//nolint:godot // ---------------------------------------------------------------------------
// GitTemplate.
//nolint:godot // ---------------------------------------------------------------------------

// GitTemplateOption configures a GitTemplate created by NewGitTemplate.
type GitTemplateOption func(*model.GitTemplate)

// NewGitTemplate returns a GitTemplate with sensible defaults.
func NewGitTemplate(opts ...GitTemplateOption) *model.GitTemplate {
	now := nullTime(time.Now())
	gt := &model.GitTemplate{
		ID:            1,
		UUID:          "test-template-uuid",
		Name:          "Test Template",
		Slug:          "test-template",
		RepositoryURL: "https://github.com/example/repo.git",
		Branch:        "main",
		IsPublic:      true,
		IsEnabled:     true,
		Status:        "synced",
		FileCount:     5,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	for _, opt := range opts {
		opt(gt)
	}
	return gt
}

// WithGitTemplateID sets the git template ID on the builder.
func WithGitTemplateID(id int64) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.ID = id }
}

// WithGitTemplateUUID sets the git template UUID on the builder.
func WithGitTemplateUUID(uuid string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.UUID = uuid }
}

// WithGitTemplateName sets the git template name on the builder.
func WithGitTemplateName(name string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Name = name }
}

// WithGitTemplateSlug sets the git template slug on the builder.
func WithGitTemplateSlug(slug string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Slug = slug }
}

// WithGitTemplateDescription sets the git template description on the builder.
func WithGitTemplateDescription(desc string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Description = nullString(desc) }
}

// WithGitTemplateRepositoryURL sets the git template repository URL on the builder.
func WithGitTemplateRepositoryURL(url string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.RepositoryURL = url }
}

// WithGitTemplateBranch sets the git template branch on the builder.
func WithGitTemplateBranch(branch string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Branch = branch }
}

// WithGitTemplateUserID sets the git template user ID on the builder.
func WithGitTemplateUserID(uid int64) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.UserID = nullInt64(uid) }
}

// WithGitTemplateIsPublic sets the git template public visibility on the builder.
func WithGitTemplateIsPublic(public bool) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.IsPublic = public }
}

// WithGitTemplateIsEnabled sets the git template enabled flag on the builder.
func WithGitTemplateIsEnabled(enabled bool) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.IsEnabled = enabled }
}

// WithGitTemplateStatus sets the git template status on the builder.
func WithGitTemplateStatus(status string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Status = status }
}

//nolint:godot // ---------------------------------------------------------------------------
// SearchQuery.
//nolint:godot // ---------------------------------------------------------------------------

// SearchQueryOption configures a SearchQuery created by NewSearchQuery.
type SearchQueryOption func(*model.SearchQuery)

// NewSearchQuery returns a SearchQuery with sensible defaults.
func NewSearchQuery(opts ...SearchQueryOption) *model.SearchQuery {
	sq := &model.SearchQuery{
		Query:        "test search",
		ResultsCount: 10,
	}
	for _, opt := range opts {
		opt(sq)
	}
	return sq
}

// WithSearchQueryUserID sets the search query user ID on the builder.
func WithSearchQueryUserID(uid int64) SearchQueryOption {
	return func(sq *model.SearchQuery) { sq.UserID = nullInt64(uid) }
}

// WithSearchQueryQuery sets the search query text on the builder.
func WithSearchQueryQuery(q string) SearchQueryOption {
	return func(sq *model.SearchQuery) { sq.Query = q }
}

// WithSearchQueryResultsCount sets the search query results count on the builder.
func WithSearchQueryResultsCount(n int) SearchQueryOption {
	return func(sq *model.SearchQuery) { sq.ResultsCount = n }
}

// WithSearchQueryFilters sets the search query filters on the builder.
func WithSearchQueryFilters(filters string) SearchQueryOption {
	return func(sq *model.SearchQuery) { sq.Filters = nullString(filters) }
}
