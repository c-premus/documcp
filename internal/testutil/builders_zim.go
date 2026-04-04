package testutil

import (
	"time"

	"github.com/c-premus/documcp/internal/model"
)

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
