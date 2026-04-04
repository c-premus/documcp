package testutil

import (
	"time"

	"github.com/c-premus/documcp/internal/model"
)

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
		Status:    model.ExternalServiceStatusUnknown,
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
func WithExternalServiceStatus(status model.ExternalServiceStatus) ExternalServiceOption {
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
