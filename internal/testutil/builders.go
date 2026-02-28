// Package testutil provides builder functions that create model instances
// with sensible defaults for use in tests. Each builder accepts functional
// options to override individual fields.
package testutil

import (
	"database/sql"
	"time"

	"git.999.haus/chris/DocuMCP-go/internal/model"
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

// ---------------------------------------------------------------------------
// Document
// ---------------------------------------------------------------------------

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

func WithDocumentID(id int64) DocumentOption {
	return func(d *model.Document) { d.ID = id }
}

func WithDocumentUUID(uuid string) DocumentOption {
	return func(d *model.Document) { d.UUID = uuid }
}

func WithDocumentTitle(title string) DocumentOption {
	return func(d *model.Document) { d.Title = title }
}

func WithDocumentDescription(desc string) DocumentOption {
	return func(d *model.Document) { d.Description = nullString(desc) }
}

func WithDocumentFileType(ft string) DocumentOption {
	return func(d *model.Document) { d.FileType = ft }
}

func WithDocumentFilePath(fp string) DocumentOption {
	return func(d *model.Document) { d.FilePath = fp }
}

func WithDocumentFileSize(size int64) DocumentOption {
	return func(d *model.Document) { d.FileSize = size }
}

func WithDocumentMIMEType(mime string) DocumentOption {
	return func(d *model.Document) { d.MIMEType = mime }
}

func WithDocumentContent(content string) DocumentOption {
	return func(d *model.Document) { d.Content = nullString(content) }
}

func WithDocumentUserID(uid int64) DocumentOption {
	return func(d *model.Document) { d.UserID = nullInt64(uid) }
}

func WithDocumentIsPublic(public bool) DocumentOption {
	return func(d *model.Document) { d.IsPublic = public }
}

func WithDocumentStatus(status string) DocumentOption {
	return func(d *model.Document) { d.Status = status }
}

// ---------------------------------------------------------------------------
// User
// ---------------------------------------------------------------------------

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

func WithUserID(id int64) UserOption {
	return func(u *model.User) { u.ID = id }
}

func WithUserName(name string) UserOption {
	return func(u *model.User) { u.Name = name }
}

func WithUserEmail(email string) UserOption {
	return func(u *model.User) { u.Email = email }
}

func WithUserIsAdmin(admin bool) UserOption {
	return func(u *model.User) { u.IsAdmin = admin }
}

func WithUserOIDCSub(sub string) UserOption {
	return func(u *model.User) { u.OIDCSub = nullString(sub) }
}

func WithUserOIDCProvider(provider string) UserOption {
	return func(u *model.User) { u.OIDCProvider = nullString(provider) }
}

func WithUserPassword(pw string) UserOption {
	return func(u *model.User) { u.Password = nullString(pw) }
}

// ---------------------------------------------------------------------------
// OAuthClient
// ---------------------------------------------------------------------------

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
		IsActive:                true,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func WithOAuthClientID(id int64) OAuthClientOption {
	return func(c *model.OAuthClient) { c.ID = id }
}

func WithOAuthClientClientID(clientID string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.ClientID = clientID }
}

func WithOAuthClientName(name string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.ClientName = name }
}

func WithOAuthClientSecret(secret string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.ClientSecret = nullString(secret) }
}

func WithOAuthClientRedirectURIs(uris string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.RedirectURIs = uris }
}

func WithOAuthClientGrantTypes(types string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.GrantTypes = types }
}

func WithOAuthClientScope(scope string) OAuthClientOption {
	return func(c *model.OAuthClient) { c.Scope = nullString(scope) }
}

func WithOAuthClientUserID(uid int64) OAuthClientOption {
	return func(c *model.OAuthClient) { c.UserID = nullInt64(uid) }
}

func WithOAuthClientIsActive(active bool) OAuthClientOption {
	return func(c *model.OAuthClient) { c.IsActive = active }
}

// ---------------------------------------------------------------------------
// ExternalService
// ---------------------------------------------------------------------------

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

func WithExternalServiceID(id int64) ExternalServiceOption {
	return func(es *model.ExternalService) { es.ID = id }
}

func WithExternalServiceUUID(uuid string) ExternalServiceOption {
	return func(es *model.ExternalService) { es.UUID = uuid }
}

func WithExternalServiceName(name string) ExternalServiceOption {
	return func(es *model.ExternalService) { es.Name = name }
}

func WithExternalServiceSlug(slug string) ExternalServiceOption {
	return func(es *model.ExternalService) { es.Slug = slug }
}

func WithExternalServiceType(t string) ExternalServiceOption {
	return func(es *model.ExternalService) { es.Type = t }
}

func WithExternalServiceBaseURL(url string) ExternalServiceOption {
	return func(es *model.ExternalService) { es.BaseURL = url }
}

func WithExternalServiceStatus(status string) ExternalServiceOption {
	return func(es *model.ExternalService) { es.Status = status }
}

func WithExternalServiceIsEnabled(enabled bool) ExternalServiceOption {
	return func(es *model.ExternalService) { es.IsEnabled = enabled }
}

func WithExternalServicePriority(p int) ExternalServiceOption {
	return func(es *model.ExternalService) { es.Priority = p }
}

// ---------------------------------------------------------------------------
// ZimArchive
// ---------------------------------------------------------------------------

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

func WithZimArchiveID(id int64) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.ID = id }
}

func WithZimArchiveUUID(uuid string) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.UUID = uuid }
}

func WithZimArchiveName(name string) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.Name = name }
}

func WithZimArchiveSlug(slug string) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.Slug = slug }
}

func WithZimArchiveTitle(title string) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.Title = title }
}

func WithZimArchiveLanguage(lang string) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.Language = lang }
}

func WithZimArchiveExternalServiceID(id int64) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.ExternalServiceID = nullInt64(id) }
}

func WithZimArchiveIsEnabled(enabled bool) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.IsEnabled = enabled }
}

func WithZimArchiveIsSearchable(searchable bool) ZimArchiveOption {
	return func(za *model.ZimArchive) { za.IsSearchable = searchable }
}

// ---------------------------------------------------------------------------
// ConfluenceSpace
// ---------------------------------------------------------------------------

// ConfluenceSpaceOption configures a ConfluenceSpace created by NewConfluenceSpace.
type ConfluenceSpaceOption func(*model.ConfluenceSpace)

// NewConfluenceSpace returns a ConfluenceSpace with sensible defaults.
func NewConfluenceSpace(opts ...ConfluenceSpaceOption) *model.ConfluenceSpace {
	now := nullTime(time.Now())
	cs := &model.ConfluenceSpace{
		ID:           1,
		UUID:         "test-confluence-uuid",
		ConfluenceID: "12345",
		Key:          "TST",
		Name:         "Test Space",
		Type:         "global",
		Status:       "current",
		IsEnabled:    true,
		IsSearchable: true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	for _, opt := range opts {
		opt(cs)
	}
	return cs
}

func WithConfluenceSpaceID(id int64) ConfluenceSpaceOption {
	return func(cs *model.ConfluenceSpace) { cs.ID = id }
}

func WithConfluenceSpaceUUID(uuid string) ConfluenceSpaceOption {
	return func(cs *model.ConfluenceSpace) { cs.UUID = uuid }
}

func WithConfluenceSpaceKey(key string) ConfluenceSpaceOption {
	return func(cs *model.ConfluenceSpace) { cs.Key = key }
}

func WithConfluenceSpaceName(name string) ConfluenceSpaceOption {
	return func(cs *model.ConfluenceSpace) { cs.Name = name }
}

func WithConfluenceSpaceStatus(status string) ConfluenceSpaceOption {
	return func(cs *model.ConfluenceSpace) { cs.Status = status }
}

func WithConfluenceSpaceExternalServiceID(id int64) ConfluenceSpaceOption {
	return func(cs *model.ConfluenceSpace) { cs.ExternalServiceID = nullInt64(id) }
}

func WithConfluenceSpaceIsEnabled(enabled bool) ConfluenceSpaceOption {
	return func(cs *model.ConfluenceSpace) { cs.IsEnabled = enabled }
}

func WithConfluenceSpaceIsSearchable(searchable bool) ConfluenceSpaceOption {
	return func(cs *model.ConfluenceSpace) { cs.IsSearchable = searchable }
}

// ---------------------------------------------------------------------------
// GitTemplate
// ---------------------------------------------------------------------------

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

func WithGitTemplateID(id int64) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.ID = id }
}

func WithGitTemplateUUID(uuid string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.UUID = uuid }
}

func WithGitTemplateName(name string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Name = name }
}

func WithGitTemplateSlug(slug string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Slug = slug }
}

func WithGitTemplateDescription(desc string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Description = nullString(desc) }
}

func WithGitTemplateRepositoryURL(url string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.RepositoryURL = url }
}

func WithGitTemplateBranch(branch string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Branch = branch }
}

func WithGitTemplateUserID(uid int64) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.UserID = nullInt64(uid) }
}

func WithGitTemplateIsPublic(public bool) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.IsPublic = public }
}

func WithGitTemplateIsEnabled(enabled bool) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.IsEnabled = enabled }
}

func WithGitTemplateStatus(status string) GitTemplateOption {
	return func(gt *model.GitTemplate) { gt.Status = status }
}

// ---------------------------------------------------------------------------
// SearchQuery
// ---------------------------------------------------------------------------

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

func WithSearchQueryUserID(uid int64) SearchQueryOption {
	return func(sq *model.SearchQuery) { sq.UserID = nullInt64(uid) }
}

func WithSearchQueryQuery(q string) SearchQueryOption {
	return func(sq *model.SearchQuery) { sq.Query = q }
}

func WithSearchQueryResultsCount(n int) SearchQueryOption {
	return func(sq *model.SearchQuery) { sq.ResultsCount = n }
}

func WithSearchQueryFilters(filters string) SearchQueryOption {
	return func(sq *model.SearchQuery) { sq.Filters = nullString(filters) }
}
